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

package userfederation

import (
	"context"
	"errors"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
	rolev1beta1 "github.com/rossigee/provider-keycloak/apis/role/v1beta1"
	userfederationv1beta1 "github.com/rossigee/provider-keycloak/apis/userfederation/v1beta1"
	"github.com/rossigee/provider-keycloak/internal/clients"
	"github.com/rossigee/provider-keycloak/internal/controller/testhelpers"
)

var errBoom = errors.New("boom")

type fedStub struct {
	*testhelpers.BaseMockClient
	providers  []clients.UserFederationProviderRepresentation
	listErr    error
	created    *clients.UserFederationProviderRepresentation
	createErr  error
	deletedID  string
	deleteErr  error
	deleteCall int
}

func (s *fedStub) ListUserFederationProviders(context.Context, string) ([]clients.UserFederationProviderRepresentation, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	return s.providers, nil
}

func (s *fedStub) CreateUserFederationProvider(_ context.Context, _ string, rep *clients.UserFederationProviderRepresentation) (string, error) {
	if s.createErr != nil {
		return "", s.createErr
	}
	s.created = rep
	return "fed-1", nil
}

func (s *fedStub) DeleteUserFederationProvider(_ context.Context, _ string, id string) error {
	s.deleteCall++
	if s.deleteErr != nil {
		return s.deleteErr
	}
	s.deletedID = id
	return nil
}

func i32Ptr(i int32) *int32 { return &i }

func newFedCR() *userfederationv1beta1.UserFederationProvider {
	return &userfederationv1beta1.UserFederationProvider{
		ObjectMeta: metav1.ObjectMeta{Name: "f", Namespace: "ns"},
		Spec: userfederationv1beta1.UserFederationProviderSpec{
			ForProvider: userfederationv1beta1.UserFederationProviderParameters{
				RealmId: "master", Name: "ldap", ProviderName: "ldap",
				Priority: i32Ptr(1),
			},
		},
	}
}

func newExternal(s *fedStub) *external { return &external{client: s} }

func matchingFed() []clients.UserFederationProviderRepresentation {
	on := true
	return []clients.UserFederationProviderRepresentation{
		{ID: "fed-1", Name: "ldap", ProviderName: "ldap", Priority: 1, Enabled: &on},
	}
}

// TestObserveReportsAbsentWhenNotListed covers a provider Keycloak does not
// have, which is what moves the reconciler to Create.
func TestObserveReportsAbsentWhenNotListed(t *testing.T) {
	e := newExternal(&fedStub{providers: []clients.UserFederationProviderRepresentation{
		{ID: "other", Name: "kerberos", ProviderName: "kerberos"},
	}})

	obs, err := e.Observe(context.Background(), newFedCR())
	if err != nil {
		t.Fatalf("Observe failed: %v", err)
	}
	if obs.ResourceExists {
		t.Fatal("a provider Keycloak does not list must be reported absent")
	}
}

func TestObserveReportsUpToDate(t *testing.T) {
	e := newExternal(&fedStub{providers: matchingFed()})
	cr := newFedCR()

	obs, err := e.Observe(context.Background(), cr)
	if err != nil {
		t.Fatalf("Observe failed: %v", err)
	}
	if !obs.ResourceExists || !obs.ResourceUpToDate {
		t.Fatalf("a provider matching the spec must be present and up to date, got %+v", obs)
	}
	if c := cr.Status.GetCondition(xpv1.TypeReady); c.Status != "True" {
		t.Errorf("Ready condition = %+v, want True", c)
	}
}

func TestObserveDetectsDrift(t *testing.T) {
	off := false
	cases := map[string][]clients.UserFederationProviderRepresentation{
		"provider name": {{ID: "fed-1", Name: "ldap", ProviderName: "kerberos", Priority: 1, Enabled: &off}},
		"priority":      {{ID: "fed-1", Name: "ldap", ProviderName: "ldap", Priority: 9, Enabled: &off}},
	}

	for name, providers := range cases {
		t.Run(name, func(t *testing.T) {
			e := newExternal(&fedStub{providers: providers})
			obs, err := e.Observe(context.Background(), newFedCR())
			if err != nil {
				t.Fatalf("Observe failed: %v", err)
			}
			if obs.ResourceUpToDate {
				t.Errorf("a changed %s must be reported out of date", name)
			}
		})
	}
}

func TestObservePropagatesListError(t *testing.T) {
	e := newExternal(&fedStub{listErr: errBoom})
	if _, err := e.Observe(context.Background(), newFedCR()); err == nil {
		t.Fatal("a failed list must surface as an error, not as an absent provider")
	}
}

func TestObserveRejectsWrongType(t *testing.T) {
	e := newExternal(&fedStub{})
	if _, err := e.Observe(context.Background(), &rolev1beta1.Role{}); err == nil {
		t.Fatal("Observe must reject a managed resource of the wrong type")
	}
}

func TestCreateWritesProvider(t *testing.T) {
	s := &fedStub{}
	e := newExternal(s)

	if _, err := e.Create(context.Background(), newFedCR()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if s.created == nil || s.created.Name != "ldap" || s.created.ProviderName != "ldap" {
		t.Errorf("written provider = %+v, want the spec's name and provider type", s.created)
	}
	if s.created == nil || s.created.Priority != 1 {
		t.Errorf("written priority = %+v, want 1 from the spec", s.created)
	}
}

func TestCreatePropagatesError(t *testing.T) {
	e := newExternal(&fedStub{createErr: errBoom})
	if _, err := e.Create(context.Background(), newFedCR()); err == nil {
		t.Fatal("Create must surface a write failure")
	}
}

func TestCreateRejectsWrongType(t *testing.T) {
	e := newExternal(&fedStub{})
	if _, err := e.Create(context.Background(), &rolev1beta1.Role{}); err == nil {
		t.Fatal("Create must reject a managed resource of the wrong type")
	}
}

func TestDeleteRemovesListedProvider(t *testing.T) {
	s := &fedStub{providers: matchingFed()}
	e := newExternal(s)

	if _, err := e.Delete(context.Background(), newFedCR()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if s.deleteCall != 1 || s.deletedID != "fed-1" {
		t.Errorf("deleted id = %q after %d calls, want fed-1 after 1", s.deletedID, s.deleteCall)
	}
}

// TestDeleteIsANoOpWhenAlreadyGone covers a provider removed out of band: there
// is nothing to delete, and calling through anyway would be a pointless call.
func TestDeleteIsANoOpWhenAlreadyGone(t *testing.T) {
	s := &fedStub{providers: nil}
	e := newExternal(s)

	if _, err := e.Delete(context.Background(), newFedCR()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if s.deleteCall != 0 {
		t.Errorf("Delete called Keycloak %d times for a provider already gone", s.deleteCall)
	}
}

func TestDeletePropagatesListError(t *testing.T) {
	e := newExternal(&fedStub{listErr: errBoom})
	if _, err := e.Delete(context.Background(), newFedCR()); err == nil {
		t.Fatal("Delete must surface a failed lookup")
	}
}

func TestDeletePropagatesError(t *testing.T) {
	e := newExternal(&fedStub{providers: matchingFed(), deleteErr: errBoom})
	if _, err := e.Delete(context.Background(), newFedCR()); err == nil {
		t.Fatal("Delete must surface a failure, or the finalizer would be dropped early")
	}
}

func TestDeleteRejectsWrongType(t *testing.T) {
	e := newExternal(&fedStub{})
	if _, err := e.Delete(context.Background(), &rolev1beta1.Role{}); err == nil {
		t.Fatal("Delete must reject a managed resource of the wrong type")
	}
}
