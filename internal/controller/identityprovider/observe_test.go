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

package identityprovider

import (
	"context"
	"errors"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
	identityproviderv1beta1 "github.com/rossigee/provider-keycloak/apis/identityprovider/v1beta1"
	rolev1beta1 "github.com/rossigee/provider-keycloak/apis/role/v1beta1"
	"github.com/rossigee/provider-keycloak/internal/clients"
	"github.com/rossigee/provider-keycloak/internal/controller/testhelpers"
)

var errBoom = errors.New("boom")

type idpStub struct {
	*testhelpers.BaseMockClient
	provider    *clients.IdentityProviderRepresentation
	getErr      error
	getCalls    int
	created     *clients.IdentityProviderRepresentation
	createErr   error
	updateErr   error
	updateAlias string
	deleteCalls int
	deleteErr   error
}

func (s *idpStub) GetIdentityProvider(context.Context, string, string) (*clients.IdentityProviderRepresentation, error) {
	s.getCalls++
	if s.getErr != nil {
		return nil, s.getErr
	}
	return s.provider, nil
}

func (s *idpStub) CreateIdentityProvider(_ context.Context, _ string, rep *clients.IdentityProviderRepresentation) (string, error) {
	if s.createErr != nil {
		return "", s.createErr
	}
	s.created = rep
	return "idp-1", nil
}

func (s *idpStub) UpdateIdentityProvider(_ context.Context, _, alias string, _ *clients.IdentityProviderRepresentation) error {
	s.updateAlias = alias
	return s.updateErr
}

func (s *idpStub) DeleteIdentityProvider(context.Context, string, string) error {
	s.deleteCalls++
	return s.deleteErr
}

func strPtr(s string) *string { return &s }
func boolPtr(b bool) *bool    { return &b }

func newIDPCR() *identityproviderv1beta1.IdentityProvider {
	return &identityproviderv1beta1.IdentityProvider{
		ObjectMeta: metav1.ObjectMeta{Name: "idp", Namespace: "ns"},
		Spec: identityproviderv1beta1.IdentityProviderSpec{
			ForProvider: identityproviderv1beta1.IdentityProviderParameters{
				RealmId: "master", Alias: "google", DisplayName: strPtr("Google"),
				ProviderId: "google", Enabled: boolPtr(true), TrustEmail: boolPtr(false),
				FirstBrokerLoginFlowAlias: strPtr("first broker flow"),
				Config:                    map[string]string{"clientId": "x", "clientSecret": "y"},
			},
		},
	}
}

func newExternal(s *idpStub) *external {
	scheme := runtime.NewScheme()
	_ = identityproviderv1beta1.AddToScheme(scheme)
	return &external{
		client: s,
		kube:   fake.NewClientBuilder().WithScheme(scheme).WithObjects(newIDPCR()).Build(),
	}
}

func matchingIDP() *clients.IdentityProviderRepresentation {
	return &clients.IdentityProviderRepresentation{
		Alias: "google", DisplayName: "Google", ProviderId: "google",
		Enabled: true, TrustEmail: false, FirstBrokerLoginFlowAlias: "first broker flow",
		Config: map[string]string{"clientId": "x", "clientSecret": "y"},
	}
}

func TestObserveReportsUpToDate(t *testing.T) {
	s := &idpStub{provider: matchingIDP()}
	e := newExternal(s)
	cr := newIDPCR()

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
	cases := map[string]func(*clients.IdentityProviderRepresentation){
		"alias":            func(p *clients.IdentityProviderRepresentation) { p.Alias = "other" },
		"display name":     func(p *clients.IdentityProviderRepresentation) { p.DisplayName = "changed" },
		"provider id":      func(p *clients.IdentityProviderRepresentation) { p.ProviderId = "oidc" },
		"enabled":          func(p *clients.IdentityProviderRepresentation) { p.Enabled = false },
		"trust email":      func(p *clients.IdentityProviderRepresentation) { p.TrustEmail = true },
		"first flow alias": func(p *clients.IdentityProviderRepresentation) { p.FirstBrokerLoginFlowAlias = "other" },
		"post flow alias":  func(p *clients.IdentityProviderRepresentation) { p.PostBrokerLoginFlowAlias = "set" },
		"config value":     func(p *clients.IdentityProviderRepresentation) { p.Config = map[string]string{"clientId": "z"} },
		"config key": func(p *clients.IdentityProviderRepresentation) {
			p.Config = map[string]string{"clientId": "x", "clientSecret": "y", "extra": "1"}
		},
	}

	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			provider := matchingIDP()
			mutate(provider)
			e := newExternal(&idpStub{provider: provider})

			obs, err := e.Observe(context.Background(), newIDPCR())
			if err != nil {
				t.Fatalf("Observe failed: %v", err)
			}
			if obs.ResourceUpToDate {
				t.Errorf("a changed %s must be reported out of date", name)
			}
		})
	}
}

func TestObserveReportsAbsentAfterDelete(t *testing.T) {
	s := &idpStub{provider: matchingIDP()}
	e := newExternal(s)
	cr := newIDPCR()
	cr.Annotations = map[string]string{"keycloak.m.crossplane.io/delete-completed": "true"}

	obs, err := e.Observe(context.Background(), cr)
	if err != nil {
		t.Fatalf("Observe failed: %v", err)
	}
	if obs.ResourceExists {
		t.Fatal("a provider already released must be reported absent")
	}
	if s.getCalls != 0 {
		t.Errorf("Keycloak was queried %d times after the provider was released", s.getCalls)
	}
}

func TestObserveRejectsWrongType(t *testing.T) {
	e := newExternal(&idpStub{})
	if _, err := e.Observe(context.Background(), &rolev1beta1.Role{}); err == nil {
		t.Fatal("Observe must reject a managed resource of the wrong type")
	}
}

func TestCreateWritesProvider(t *testing.T) {
	s := &idpStub{}
	e := newExternal(s)

	if _, err := e.Create(context.Background(), newIDPCR()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if s.created == nil || s.created.Alias != "google" || s.created.ProviderId != "google" {
		t.Errorf("written provider = %+v, want the spec's alias and provider id", s.created)
	}
	if s.created == nil || !s.created.Enabled {
		t.Errorf("written enabled = %+v, want true from the spec", s.created)
	}
}

func TestCreatePropagatesError(t *testing.T) {
	e := newExternal(&idpStub{createErr: errBoom})
	if _, err := e.Create(context.Background(), newIDPCR()); err == nil {
		t.Fatal("Create must surface a write failure")
	}
}

func TestCreateRejectsWrongType(t *testing.T) {
	e := newExternal(&idpStub{})
	if _, err := e.Create(context.Background(), &rolev1beta1.Role{}); err == nil {
		t.Fatal("Create must reject a managed resource of the wrong type")
	}
}

func TestUpdateTargetsAlias(t *testing.T) {
	s := &idpStub{}
	e := newExternal(s)

	if _, err := e.Update(context.Background(), newIDPCR()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if s.updateAlias != "google" {
		t.Errorf("updated alias = %q, want google", s.updateAlias)
	}
}

func TestUpdatePropagatesError(t *testing.T) {
	e := newExternal(&idpStub{updateErr: errBoom})
	if _, err := e.Update(context.Background(), newIDPCR()); err == nil {
		t.Fatal("Update must surface a write failure")
	}
}

func TestUpdateRejectsWrongType(t *testing.T) {
	e := newExternal(&idpStub{})
	if _, err := e.Update(context.Background(), &rolev1beta1.Role{}); err == nil {
		t.Fatal("Update must reject a managed resource of the wrong type")
	}
}

func TestDeleteRemovesProvider(t *testing.T) {
	s := &idpStub{}
	e := newExternal(s)

	if _, err := e.Delete(context.Background(), newIDPCR()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if s.deleteCalls != 1 {
		t.Errorf("DeleteIdentityProvider called %d times, want 1", s.deleteCalls)
	}
}

func TestDeleteIsIdempotent(t *testing.T) {
	s := &idpStub{}
	e := newExternal(s)
	cr := newIDPCR()
	cr.Annotations = map[string]string{"keycloak.m.crossplane.io/delete-completed": "true"}

	if _, err := e.Delete(context.Background(), cr); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if s.deleteCalls != 0 {
		t.Errorf("Delete called Keycloak %d times after completion was recorded", s.deleteCalls)
	}
}

func TestDeletePropagatesError(t *testing.T) {
	e := newExternal(&idpStub{deleteErr: errBoom})
	if _, err := e.Delete(context.Background(), newIDPCR()); err == nil {
		t.Fatal("Delete must surface a failure, or the finalizer would be dropped with the provider intact")
	}
}

func TestDeleteRejectsWrongType(t *testing.T) {
	e := newExternal(&idpStub{})
	if _, err := e.Delete(context.Background(), &rolev1beta1.Role{}); err == nil {
		t.Fatal("Delete must reject a managed resource of the wrong type")
	}
}
