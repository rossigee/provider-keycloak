/*
Copyright 2024 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package realmimpexp

import (
	"context"
	"errors"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
	realmimpexpv1beta1 "github.com/rossigee/provider-keycloak/apis/realmimpexp/v1beta1"
	rolev1beta1 "github.com/rossigee/provider-keycloak/apis/role/v1beta1"
	"github.com/rossigee/provider-keycloak/internal/controller/testhelpers"
)

var errBoom = errors.New("boom")

type importStub struct {
	*testhelpers.BaseMockClient
	importErr   error
	importCalls int
	gotJSON     string
	gotIfNotEx  bool
}

func (s *importStub) ImportRealm(_ context.Context, realmJSON string, ifNotExists bool) error {
	s.importCalls++
	s.gotJSON = realmJSON
	s.gotIfNotEx = ifNotExists
	return s.importErr
}

func newImportCR() *realmimpexpv1beta1.RealmImport {
	return &realmimpexpv1beta1.RealmImport{
		ObjectMeta: metav1.ObjectMeta{Name: "imp", Namespace: "ns"},
		Spec: realmimpexpv1beta1.RealmImportSpec{
			ForProvider: realmimpexpv1beta1.RealmImportParameters{
				RealmId: "master", RealmJSON: `{"realm":"master"}`,
			},
		},
	}
}

func newExternal(s *importStub) *external { return &external{client: s} }

// TestObserveReportsAbsentWhenDeleting is the behavioural counterpart to the
// structural guard in deletecomplete. A realm import has nothing to undo in
// Keycloak, so once deletion is requested Observe must report it absent or the
// reconciler re-runs Delete forever and the finalizer is never removed.
func TestObserveReportsAbsentWhenDeleting(t *testing.T) {
	e := newExternal(&importStub{})

	cr := newImportCR()
	now := metav1.NewTime(time.Now())
	cr.SetDeletionTimestamp(&now)

	obs, err := e.Observe(context.Background(), cr)
	if err != nil {
		t.Fatalf("Observe failed: %v", err)
	}
	if obs.ResourceExists {
		t.Fatal("a resource being deleted with a no-op Delete must be reported absent, " +
			"or the finalizer is never removed")
	}
}

func TestObserveReportsPresent(t *testing.T) {
	e := newExternal(&importStub{})
	cr := newImportCR()

	obs, err := e.Observe(context.Background(), cr)
	if err != nil {
		t.Fatalf("Observe failed: %v", err)
	}
	if !obs.ResourceExists {
		t.Fatal("a live realm import must be reported present")
	}
	if c := cr.Status.GetCondition(xpv1.TypeReady); c.Status != "True" {
		t.Errorf("Ready condition = %+v, want True", c)
	}
}

func TestObserveRejectsWrongType(t *testing.T) {
	e := newExternal(&importStub{})
	if _, err := e.Observe(context.Background(), &rolev1beta1.Role{}); err == nil {
		t.Fatal("Observe must reject a managed resource of the wrong type")
	}
}

func TestCreateImportsRealm(t *testing.T) {
	s := &importStub{}
	e := newExternal(s)
	cr := newImportCR()

	if _, err := e.Create(context.Background(), cr); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if s.importCalls != 1 {
		t.Fatalf("ImportRealm called %d times, want 1", s.importCalls)
	}
	if s.gotJSON != `{"realm":"master"}` {
		t.Errorf("imported JSON = %q, want the spec's RealmJSON", s.gotJSON)
	}
	if s.gotIfNotEx {
		t.Error("IfNotExists unset must be passed as false")
	}
}

func TestCreatePassesIfNotExists(t *testing.T) {
	yes := true
	s := &importStub{}
	e := newExternal(s)
	cr := newImportCR()
	cr.Spec.ForProvider.IfNotExists = &yes

	if _, err := e.Create(context.Background(), cr); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if !s.gotIfNotEx {
		t.Error("IfNotExists: true was not passed through to ImportRealm")
	}
}

// TestCreateMarksUnavailableOnFailure checks a failed import is reported as
// unavailable rather than left looking healthy.
func TestCreateMarksUnavailableOnFailure(t *testing.T) {
	s := &importStub{importErr: errBoom}
	e := newExternal(s)
	cr := newImportCR()

	if _, err := e.Create(context.Background(), cr); err == nil {
		t.Fatal("Create must surface an import failure")
	}
	c := cr.Status.GetCondition(xpv1.TypeReady)
	if c.Status != "False" {
		t.Errorf("Ready condition = %+v, want False after a failed import", c)
	}
}

func TestCreateRejectsWrongType(t *testing.T) {
	e := newExternal(&importStub{})
	if _, err := e.Create(context.Background(), &rolev1beta1.Role{}); err == nil {
		t.Fatal("Create must reject a managed resource of the wrong type")
	}
}

// TestUpdateAndDeleteAreNoOps records the property the finalizer fix relies on:
// a realm import cannot be undone, so neither may touch Keycloak.
func TestUpdateAndDeleteAreNoOps(t *testing.T) {
	s := &importStub{importErr: errBoom}
	e := newExternal(s)
	cr := newImportCR()

	if _, err := e.Update(context.Background(), cr); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if _, err := e.Delete(context.Background(), cr); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if s.importCalls != 0 {
		t.Errorf("Update/Delete called ImportRealm %d times; there is nothing to undo", s.importCalls)
	}
}

func TestDisconnectSucceeds(t *testing.T) {
	e := newExternal(&importStub{})
	if err := e.Disconnect(context.Background()); err != nil {
		t.Errorf("Disconnect = %v, want nil", err)
	}
}
