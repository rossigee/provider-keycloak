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

package authz

import (
	"context"
	"errors"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
	authzv1beta1 "github.com/rossigee/provider-keycloak/apis/authz/v1beta1"
	rolev1beta1 "github.com/rossigee/provider-keycloak/apis/role/v1beta1"
	"github.com/rossigee/provider-keycloak/internal/clients"
	"github.com/rossigee/provider-keycloak/internal/controller/testhelpers"
)

var errBoom = errors.New("boom")

type authzStub struct {
	*testhelpers.BaseMockClient
	client      *clients.ClientRepresentation
	clientErr   error
	resources   []clients.AuthzResourceRepresentation
	listErr     error
	created     *clients.AuthzResourceRepresentation
	createErr   error
	deletedID   string
	deleteErr   error
	deleteCalls int
	listCalls   int
}

func (s *authzStub) GetClient(context.Context, string, string) (*clients.ClientRepresentation, error) {
	if s.clientErr != nil {
		return nil, s.clientErr
	}
	if s.client == nil {
		return &clients.ClientRepresentation{ID: "cid-uuid"}, nil
	}
	return s.client, nil
}

func (s *authzStub) ListAuthzResources(context.Context, string, string) ([]clients.AuthzResourceRepresentation, error) {
	s.listCalls++
	if s.listErr != nil {
		return nil, s.listErr
	}
	return s.resources, nil
}

func (s *authzStub) CreateAuthzResource(_ context.Context, _, _ string, rep *clients.AuthzResourceRepresentation) (string, error) {
	if s.createErr != nil {
		return "", s.createErr
	}
	s.created = rep
	return "res-1", nil
}

func (s *authzStub) DeleteAuthzResource(_ context.Context, _, _, id string) error {
	s.deleteCalls++
	if s.deleteErr != nil {
		return s.deleteErr
	}
	s.deletedID = id
	return nil
}

func newAuthzCR() *authzv1beta1.AuthzResource {
	return &authzv1beta1.AuthzResource{
		ObjectMeta: metav1.ObjectMeta{Name: "a", Namespace: "ns"},
		Spec: authzv1beta1.AuthzResourceSpec{
			ForProvider: authzv1beta1.AuthzResourceParameters{
				RealmId: "master", ClientId: "cid", Name: "orders",
				URIs: []string{"/orders"},
			},
		},
	}
}

func newExternal(s *authzStub) *external { return &external{client: s} }

func matchingResources() []clients.AuthzResourceRepresentation {
	return []clients.AuthzResourceRepresentation{
		{ID: "res-1", Name: "orders", URIs: []string{"/orders"}},
	}
}

// TestObserveReportsAbsentWhenParentClientGone covers the orphaned-parent case:
// the reconciler must be told the resource is gone so it can release the
// finalizer, not be handed an error it would retry forever.
func TestObserveReportsAbsentWhenParentClientGone(t *testing.T) {
	for _, msg := range []string{"HTTP 404: not found", "client not found"} {
		t.Run(msg, func(t *testing.T) {
			e := newExternal(&authzStub{clientErr: errors.New(msg)})

			obs, err := e.Observe(context.Background(), newAuthzCR())
			if err != nil {
				t.Fatalf("a missing parent client must be absence, not an error: %v", err)
			}
			if obs.ResourceExists {
				t.Fatal("a resource whose client is gone must be reported absent")
			}
		})
	}
}

func TestObserveReportsAbsentWhenNotListed(t *testing.T) {
	e := newExternal(&authzStub{resources: []clients.AuthzResourceRepresentation{
		{ID: "other", Name: "invoices"},
	}})

	obs, err := e.Observe(context.Background(), newAuthzCR())
	if err != nil {
		t.Fatalf("Observe failed: %v", err)
	}
	if obs.ResourceExists {
		t.Fatal("a resource Keycloak does not list must be reported absent")
	}
}

func TestObserveReportsUpToDate(t *testing.T) {
	e := newExternal(&authzStub{resources: matchingResources()})
	cr := newAuthzCR()

	obs, err := e.Observe(context.Background(), cr)
	if err != nil {
		t.Fatalf("Observe failed: %v", err)
	}
	if !obs.ResourceExists || !obs.ResourceUpToDate {
		t.Fatalf("a resource matching the spec must be present and up to date, got %+v", obs)
	}
	if c := cr.Status.GetCondition(xpv1.TypeReady); c.Status != "True" {
		t.Errorf("Ready condition = %+v, want True", c)
	}
}

func TestObserveDetectsDisplayNameDrift(t *testing.T) {
	display := "Orders API"
	e := newExternal(&authzStub{resources: []clients.AuthzResourceRepresentation{
		{ID: "res-1", Name: "orders", URIs: []string{"/orders"}, DisplayName: &display},
	}})
	cr := newAuthzCR()
	cr.Spec.ForProvider.DisplayName = &display

	obs, err := e.Observe(context.Background(), cr)
	if err != nil {
		t.Fatalf("Observe failed: %v", err)
	}
	if !obs.ResourceUpToDate {
		t.Error("a matching display name must be up to date")
	}

	other := "Something else"
	cr.Spec.ForProvider.DisplayName = &other
	obs, err = e.Observe(context.Background(), cr)
	if err != nil {
		t.Fatalf("Observe failed: %v", err)
	}
	if obs.ResourceUpToDate {
		t.Error("a changed display name must be reported out of date")
	}
}

func TestObservePropagatesClientError(t *testing.T) {
	e := newExternal(&authzStub{clientErr: errBoom})
	if _, err := e.Observe(context.Background(), newAuthzCR()); err == nil {
		t.Fatal("a transport failure must surface as an error, not as an absent resource")
	}
}

func TestObservePropagatesListError(t *testing.T) {
	e := newExternal(&authzStub{listErr: errBoom})
	if _, err := e.Observe(context.Background(), newAuthzCR()); err == nil {
		t.Fatal("a failed list must surface as an error")
	}
}

func TestObserveRejectsWrongType(t *testing.T) {
	e := newExternal(&authzStub{})
	if _, err := e.Observe(context.Background(), &rolev1beta1.Role{}); err == nil {
		t.Fatal("Observe must reject a managed resource of the wrong type")
	}
}

func TestCreateWritesResource(t *testing.T) {
	s := &authzStub{}
	e := newExternal(s)

	if _, err := e.Create(context.Background(), newAuthzCR()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if s.created == nil || s.created.Name != "orders" {
		t.Errorf("written resource = %+v, want the spec's name", s.created)
	}
	if s.created == nil || len(s.created.URIs) != 1 || s.created.URIs[0] != "/orders" {
		t.Errorf("written URIs = %+v, want [/orders]", s.created)
	}
}

func TestCreatePropagatesError(t *testing.T) {
	e := newExternal(&authzStub{createErr: errBoom})
	if _, err := e.Create(context.Background(), newAuthzCR()); err == nil {
		t.Fatal("Create must surface a write failure")
	}
}

func TestCreateRejectsWrongType(t *testing.T) {
	e := newExternal(&authzStub{})
	if _, err := e.Create(context.Background(), &rolev1beta1.Role{}); err == nil {
		t.Fatal("Create must reject a managed resource of the wrong type")
	}
}

func TestDeleteRemovesResource(t *testing.T) {
	s := &authzStub{resources: matchingResources()}
	e := newExternal(s)

	if _, err := e.Delete(context.Background(), newAuthzCR()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if s.deleteCalls != 1 || s.deletedID != "res-1" {
		t.Errorf("deleted id = %q after %d calls, want res-1 after 1", s.deletedID, s.deleteCalls)
	}
}

func TestDeleteIsANoOpWhenAlreadyGone(t *testing.T) {
	s := &authzStub{resources: nil}
	e := newExternal(s)

	if _, err := e.Delete(context.Background(), newAuthzCR()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if s.deleteCalls != 0 {
		t.Errorf("Delete called Keycloak %d times for a resource already gone", s.deleteCalls)
	}
}

func TestDeletePropagatesError(t *testing.T) {
	e := newExternal(&authzStub{resources: matchingResources(), deleteErr: errBoom})
	if _, err := e.Delete(context.Background(), newAuthzCR()); err == nil {
		t.Fatal("Delete must surface a failure, or the finalizer would be dropped early")
	}
}

func TestDeleteRejectsWrongType(t *testing.T) {
	e := newExternal(&authzStub{})
	if _, err := e.Delete(context.Background(), &rolev1beta1.Role{}); err == nil {
		t.Fatal("Delete must reject a managed resource of the wrong type")
	}
}
