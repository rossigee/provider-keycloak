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

package clientinitialaccess

import (
	"context"
	"errors"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
	ciav1beta1 "github.com/rossigee/provider-keycloak/apis/clientinitialaccess/v1beta1"
	rolev1beta1 "github.com/rossigee/provider-keycloak/apis/role/v1beta1"
	"github.com/rossigee/provider-keycloak/internal/clients"
	"github.com/rossigee/provider-keycloak/internal/controller/testhelpers"
)

var errBoom = errors.New("boom")

type ciaStub struct {
	*testhelpers.BaseMockClient
	tokens     []clients.ClientInitialAccessRepresentation
	listErr    error
	created    *clients.ClientInitialAccessRepresentation
	createErr  error
	createdFor [2]int32
	deleteID   string
	deleteErr  error
}

func (s *ciaStub) ListClientInitialAccess(context.Context, string) ([]clients.ClientInitialAccessRepresentation, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	return s.tokens, nil
}

func (s *ciaStub) CreateClientInitialAccess(_ context.Context, _ string, count, expiration int32) (*clients.ClientInitialAccessRepresentation, error) {
	if s.createErr != nil {
		return nil, s.createErr
	}
	s.createdFor = [2]int32{count, expiration}
	s.created = &clients.ClientInitialAccessRepresentation{ID: "cia-1", Token: "tok", RemainingCount: count}
	return s.created, nil
}

func (s *ciaStub) DeleteClientInitialAccess(_ context.Context, _ string, id string) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	s.deleteID = id
	return nil
}

func newCIACR() *ciav1beta1.ClientInitialAccess {
	return &ciav1beta1.ClientInitialAccess{
		ObjectMeta: metav1.ObjectMeta{Name: "cia", Namespace: "ns"},
		Spec: ciav1beta1.ClientInitialAccessSpec{
			ForProvider: ciav1beta1.ClientInitialAccessParameters{
				RealmId: "master", Count: 5, Expiration: 3600,
			},
		},
	}
}

func newExternal(s *ciaStub) *external { return &external{client: s} }

func withAccessID(cr *ciav1beta1.ClientInitialAccess, id string) *ciav1beta1.ClientInitialAccess {
	setAccessID(cr, id)
	return cr
}

// TestObserveReportsAbsentWithoutAccessID covers a token that was never issued.
// The reconciler relies on this to move to Create rather than treating the
// missing token as an error and retrying forever.
func TestObserveReportsAbsentWithoutAccessID(t *testing.T) {
	s := &ciaStub{tokens: []clients.ClientInitialAccessRepresentation{{ID: "cia-1"}}}
	e := newExternal(s)

	obs, err := e.Observe(context.Background(), newCIACR())
	if err != nil {
		t.Fatalf("Observe failed: %v", err)
	}
	if obs.ResourceExists {
		t.Fatal("a token with no recorded access ID has not been issued and must be reported absent")
	}
}

func TestObserveReportsPresentAndRecordsRemaining(t *testing.T) {
	s := &ciaStub{tokens: []clients.ClientInitialAccessRepresentation{
		{ID: "cia-1", RemainingCount: 3},
	}}
	e := newExternal(s)
	cr := withAccessID(newCIACR(), "cia-1")

	obs, err := e.Observe(context.Background(), cr)
	if err != nil {
		t.Fatalf("Observe failed: %v", err)
	}
	if !obs.ResourceExists {
		t.Fatal("an issued token must be reported present")
	}
	if cr.Status.RemainingCount != 3 {
		t.Errorf("remainingCount = %d, want 3 from Keycloak", cr.Status.RemainingCount)
	}
	if c := cr.Status.GetCondition(xpv1.TypeReady); c.Status != "True" {
		t.Errorf("Ready condition = %+v, want True", c)
	}
}

// TestObserveReportsAbsentWhenNotListed covers a token consumed and gone:
// the reconciler must be able to finalise it.
func TestObserveReportsAbsentWhenNotListed(t *testing.T) {
	e := newExternal(&ciaStub{tokens: []clients.ClientInitialAccessRepresentation{
		{ID: "other", RemainingCount: 1},
	}})

	obs, err := e.Observe(context.Background(), withAccessID(newCIACR(), "cia-1"))
	if err != nil {
		t.Fatalf("Observe failed: %v", err)
	}
	if obs.ResourceExists {
		t.Fatal("a token Keycloak no longer lists must be reported absent")
	}
}

func TestObservePropagatesListError(t *testing.T) {
	e := newExternal(&ciaStub{listErr: errBoom})
	if _, err := e.Observe(context.Background(), withAccessID(newCIACR(), "cia-1")); err == nil {
		t.Fatal("a failed list must surface as an error, not as an absent token")
	}
}

func TestObserveRejectsWrongType(t *testing.T) {
	e := newExternal(&ciaStub{})
	if _, err := e.Observe(context.Background(), &rolev1beta1.Role{}); err == nil {
		t.Fatal("Observe must reject a managed resource of the wrong type")
	}
}

func TestCreateRecordsTokenAndID(t *testing.T) {
	s := &ciaStub{}
	e := newExternal(s)
	cr := newCIACR()

	if _, err := e.Create(context.Background(), cr); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if getAccessID(cr) != "cia-1" {
		t.Errorf("recorded access ID = %q, want the ID Keycloak assigned", getAccessID(cr))
	}
	if cr.Status.Token != "tok" {
		t.Errorf("status token = %q, want the token Keycloak issued", cr.Status.Token)
	}
	if s.createdFor != [2]int32{5, 3600} {
		t.Errorf("created with count/expiration %v, want 5/3600 from the spec", s.createdFor)
	}
}

func TestCreatePropagatesError(t *testing.T) {
	e := newExternal(&ciaStub{createErr: errBoom})
	cr := newCIACR()

	if _, err := e.Create(context.Background(), cr); err == nil {
		t.Fatal("Create must surface a write failure")
	}
	if getAccessID(cr) != "" {
		t.Errorf("a failed create must not record an access ID, got %q", getAccessID(cr))
	}
}

func TestCreateRejectsWrongType(t *testing.T) {
	e := newExternal(&ciaStub{})
	if _, err := e.Create(context.Background(), &rolev1beta1.Role{}); err == nil {
		t.Fatal("Create must reject a managed resource of the wrong type")
	}
}

func TestDeleteRemovesRecordedToken(t *testing.T) {
	s := &ciaStub{}
	e := newExternal(s)

	if _, err := e.Delete(context.Background(), withAccessID(newCIACR(), "cia-1")); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if s.deleteID != "cia-1" {
		t.Errorf("deleted ID = %q, want cia-1", s.deleteID)
	}
}

func TestDeletePropagatesError(t *testing.T) {
	e := newExternal(&ciaStub{deleteErr: errBoom})
	if _, err := e.Delete(context.Background(), withAccessID(newCIACR(), "cia-1")); err == nil {
		t.Fatal("Delete must surface a failure, or the finalizer would be dropped early")
	}
}

func TestDeleteRejectsWrongType(t *testing.T) {
	e := newExternal(&ciaStub{})
	if _, err := e.Delete(context.Background(), &rolev1beta1.Role{}); err == nil {
		t.Fatal("Delete must reject a managed resource of the wrong type")
	}
}
