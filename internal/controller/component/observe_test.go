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

package component

import (
	"context"
	"errors"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
	compv1beta1 "github.com/rossigee/provider-keycloak/apis/component/v1beta1"
	rolev1beta1 "github.com/rossigee/provider-keycloak/apis/role/v1beta1"
	"github.com/rossigee/provider-keycloak/internal/clients"
	"github.com/rossigee/provider-keycloak/internal/controller/testhelpers"
)

var errBoom = errors.New("boom")

type componentStub struct {
	*testhelpers.BaseMockClient
	comp      *clients.ComponentRepresentation
	getErr    error
	getCalls  int
	created   *clients.ComponentRepresentation
	createErr error
	deletedID string
	deleteErr error
}

func (s *componentStub) GetComponent(context.Context, string, string) (*clients.ComponentRepresentation, error) {
	s.getCalls++
	if s.getErr != nil {
		return nil, s.getErr
	}
	return s.comp, nil
}

func (s *componentStub) CreateComponent(_ context.Context, _ string, rep *clients.ComponentRepresentation) (string, error) {
	if s.createErr != nil {
		return "", s.createErr
	}
	s.created = rep
	return "comp-1", nil
}

func (s *componentStub) DeleteComponent(_ context.Context, _ string, id string) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	s.deletedID = id
	return nil
}

func newComponentCR() *compv1beta1.Component {
	return &compv1beta1.Component{
		ObjectMeta: metav1.ObjectMeta{Name: "c", Namespace: "ns"},
		Spec: compv1beta1.ComponentSpec{
			ForProvider: compv1beta1.ComponentParameters{
				RealmId:      "master",
				Name:         "my-ldap",
				ProviderType: "org.keycloak.storage.UserStorageProvider",
			},
		},
	}
}

func withID(cr *compv1beta1.Component, id string) *compv1beta1.Component {
	cr.Annotations = map[string]string{"keycloak.crossplane.io/component-id": id}
	return cr
}

func newExternal(s *componentStub) *external { return &external{client: s} }

// TestObserveReportsAbsentWithoutID covers a resource that has never been
// created. The reconciler relies on this to move to Create rather than treating
// the missing component as an error and retrying forever.
func TestObserveReportsAbsentWithoutID(t *testing.T) {
	s := &componentStub{comp: &clients.ComponentRepresentation{ID: "comp-1"}}
	e := newExternal(s)

	obs, err := e.Observe(context.Background(), newComponentCR())
	if err != nil {
		t.Fatalf("Observe failed: %v", err)
	}
	if obs.ResourceExists {
		t.Fatal("a component with no recorded ID has not been created and must be reported absent")
	}
	if s.getCalls != 0 {
		t.Errorf("Keycloak was queried %d times with no ID to look up", s.getCalls)
	}
}

func TestObserveReportsPresent(t *testing.T) {
	s := &componentStub{comp: &clients.ComponentRepresentation{ID: "comp-1"}}
	e := newExternal(s)
	cr := withID(newComponentCR(), "comp-1")

	obs, err := e.Observe(context.Background(), cr)
	if err != nil {
		t.Fatalf("Observe failed: %v", err)
	}
	if !obs.ResourceExists {
		t.Fatal("an existing component must be reported present")
	}
	if c := cr.Status.GetCondition(xpv1.TypeReady); c.Status != "True" {
		t.Errorf("Ready condition = %+v, want True", c)
	}
}

// TestObserveTreats404AsAbsent keeps a deleted-in-Keycloak component from
// wedging the reconciler on a retry loop.
func TestObserveTreats404AsAbsent(t *testing.T) {
	e := newExternal(&componentStub{getErr: errors.New("HTTP 404: not found")})

	obs, err := e.Observe(context.Background(), withID(newComponentCR(), "comp-1"))
	if err != nil {
		t.Fatalf("a 404 must be reported as absence, not an error: %v", err)
	}
	if obs.ResourceExists {
		t.Error("a component Keycloak no longer has must be reported absent")
	}
}

func TestObservePropagatesRealError(t *testing.T) {
	e := newExternal(&componentStub{getErr: errBoom})

	obs, err := e.Observe(context.Background(), withID(newComponentCR(), "comp-1"))
	if err == nil {
		t.Fatal("a transport failure must surface as an error, not as an absent component")
	}
	if obs.ResourceExists {
		t.Error("a transport failure must not report the component absent")
	}
}

func TestObserveRejectsWrongType(t *testing.T) {
	e := newExternal(&componentStub{})
	if _, err := e.Observe(context.Background(), &rolev1beta1.Role{}); err == nil {
		t.Fatal("Observe must reject a managed resource of the wrong type")
	}
}

func TestCreateRecordsAssignedID(t *testing.T) {
	s := &componentStub{}
	e := newExternal(s)
	cr := newComponentCR()

	if _, err := e.Create(context.Background(), cr); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if got := getComponentID(cr); got != "comp-1" {
		t.Errorf("recorded component ID = %q, want the ID Keycloak assigned", got)
	}
	if s.created == nil || s.created.Name != "my-ldap" || s.created.ProviderType == "" {
		t.Errorf("written representation = %+v, want the spec's name and provider type", s.created)
	}
}

func TestCreatePropagatesError(t *testing.T) {
	e := newExternal(&componentStub{createErr: errBoom})
	cr := newComponentCR()

	if _, err := e.Create(context.Background(), cr); err == nil {
		t.Fatal("Create must surface a write failure")
	}
	if got := getComponentID(cr); got != "" {
		t.Errorf("a failed create must not record an ID, got %q", got)
	}
}

func TestCreateRejectsWrongType(t *testing.T) {
	e := newExternal(&componentStub{})
	if _, err := e.Create(context.Background(), &rolev1beta1.Role{}); err == nil {
		t.Fatal("Create must reject a managed resource of the wrong type")
	}
}

// TestDeleteRemovesRecordedComponent checks the ID is actually passed through,
// since without it nothing is removed.
func TestDeleteRemovesRecordedComponent(t *testing.T) {
	s := &componentStub{}
	e := newExternal(s)
	cr := withID(newComponentCR(), "comp-1")

	if _, err := e.Delete(context.Background(), cr); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if s.deletedID != "comp-1" {
		t.Errorf("deleted ID = %q, want comp-1", s.deletedID)
	}
	if c := cr.Status.GetCondition(xpv1.TypeReady); c.Status != "False" || c.Reason != "Deleting" {
		t.Errorf("Deleting condition = %+v, want False/Deleting", c)
	}
}

func TestDeleteWithoutIDCallsNothing(t *testing.T) {
	s := &componentStub{}
	e := newExternal(s)

	if _, err := e.Delete(context.Background(), newComponentCR()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if s.deletedID != "" {
		t.Errorf("Delete called the API with ID %q when none was recorded", s.deletedID)
	}
}

func TestDeleteTolerates404(t *testing.T) {
	e := newExternal(&componentStub{deleteErr: errors.New("HTTP 404: gone")})
	if _, err := e.Delete(context.Background(), withID(newComponentCR(), "comp-1")); err != nil {
		t.Errorf("a component already gone must not fail deletion: %v", err)
	}
}

func TestDeletePropagatesRealError(t *testing.T) {
	e := newExternal(&componentStub{deleteErr: errBoom})
	if _, err := e.Delete(context.Background(), withID(newComponentCR(), "comp-1")); err == nil {
		t.Fatal("Delete must surface a real failure, or the finalizer would be dropped early")
	}
}

func TestDeleteRejectsWrongType(t *testing.T) {
	e := newExternal(&componentStub{})
	if _, err := e.Delete(context.Background(), &rolev1beta1.Role{}); err == nil {
		t.Fatal("Delete must reject a managed resource of the wrong type")
	}
}

func TestDisconnectSucceeds(t *testing.T) {
	e := newExternal(&componentStub{})
	if err := e.Disconnect(context.Background()); err != nil {
		t.Errorf("Disconnect = %v, want nil", err)
	}
}
