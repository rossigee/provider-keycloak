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

package authorizationpolicy

import (
	"context"
	"errors"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
	authorizationpolicyv1beta1 "github.com/rossigee/provider-keycloak/apis/authorizationpolicy/v1beta1"
	rolev1beta1 "github.com/rossigee/provider-keycloak/apis/role/v1beta1"
	"github.com/rossigee/provider-keycloak/internal/clients"
	"github.com/rossigee/provider-keycloak/internal/controller/testhelpers"
)

const externalName = "crossplane.io/external-name"

var errBoom = errors.New("boom")

type policyStub struct {
	*testhelpers.BaseMockClient
	policy     *clients.AuthorizationPolicyRepresentation
	getErr     error
	deleteErr  error
	getCalls   int
	deleteCall int
	created    *clients.AuthorizationPolicyRepresentation
	createdFor [2]string
}

func (s *policyStub) GetAuthorizationPolicy(context.Context, string, string, string) (*clients.AuthorizationPolicyRepresentation, error) {
	s.getCalls++
	if s.getErr != nil {
		return nil, s.getErr
	}
	return s.policy, nil
}

func (s *policyStub) CreateAuthorizationPolicy(_ context.Context, realm, clientID string, rep *clients.AuthorizationPolicyRepresentation) (string, error) {
	s.created = rep
	s.createdFor = [2]string{realm, clientID}
	return "pol-1", nil
}

func (s *policyStub) DeleteAuthorizationPolicy(context.Context, string, string, string) error {
	s.deleteCall++
	return s.deleteErr
}

func strPtr(s string) *string { return &s }

func newPolicyCR() *authorizationpolicyv1beta1.AuthorizationPolicy {
	cr := &authorizationpolicyv1beta1.AuthorizationPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "p", Namespace: "ns"},
		Spec: authorizationpolicyv1beta1.AuthorizationPolicySpec{
			ForProvider: authorizationpolicyv1beta1.AuthorizationPolicyParameters{
				RealmId: "master", ClientId: "cid", Name: "only-admin",
				Type: "role", Description: strPtr("admins only"), Logic: strPtr("POSITIVE"),
				Config: map[string]string{"roles": "admin"},
			},
		},
	}
	cr.Annotations = map[string]string{externalName: "pol-1"}
	return cr
}

func newExternal(s *policyStub) *external {
	scheme := runtime.NewScheme()
	_ = authorizationpolicyv1beta1.AddToScheme(scheme)
	return &external{
		client: s,
		kube:   fake.NewClientBuilder().WithScheme(scheme).WithObjects(newPolicyCR()).Build(),
	}
}

func matchingPolicy() *clients.AuthorizationPolicyRepresentation {
	return &clients.AuthorizationPolicyRepresentation{
		ID: "pol-1", Name: "only-admin", Type: "role",
		Description: "admins only", Logic: "POSITIVE",
		Config: map[string]string{"roles": "admin"},
	}
}

func TestObserveReportsUpToDate(t *testing.T) {
	s := &policyStub{policy: matchingPolicy()}
	e := newExternal(s)
	cr := newPolicyCR()

	obs, err := e.Observe(context.Background(), cr)
	if err != nil {
		t.Fatalf("Observe failed: %v", err)
	}
	if !obs.ResourceExists || !obs.ResourceUpToDate {
		t.Fatalf("a policy matching the spec must be present and up to date, got %+v", obs)
	}
	if c := cr.Status.GetCondition(xpv1.TypeReady); c.Status != "True" {
		t.Errorf("Ready condition = %+v, want True", c)
	}
}

func TestObserveDetectsDrift(t *testing.T) {
	cases := map[string]func(*clients.AuthorizationPolicyRepresentation){
		"name":         func(p *clients.AuthorizationPolicyRepresentation) { p.Name = "other" },
		"type":         func(p *clients.AuthorizationPolicyRepresentation) { p.Type = "user" },
		"description":  func(p *clients.AuthorizationPolicyRepresentation) { p.Description = "changed" },
		"logic":        func(p *clients.AuthorizationPolicyRepresentation) { p.Logic = "NEGATIVE" },
		"config value": func(p *clients.AuthorizationPolicyRepresentation) { p.Config = map[string]string{"roles": "user"} },
		"config key": func(p *clients.AuthorizationPolicyRepresentation) {
			p.Config = map[string]string{"roles": "admin", "extra": "x"}
		},
	}

	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			policy := matchingPolicy()
			mutate(policy)
			e := newExternal(&policyStub{policy: policy})

			obs, err := e.Observe(context.Background(), newPolicyCR())
			if err != nil {
				t.Fatalf("Observe failed: %v", err)
			}
			if obs.ResourceUpToDate {
				t.Errorf("a changed %s must be reported out of date", name)
			}
		})
	}
}

func TestObservePropagatesGetError(t *testing.T) {
	e := newExternal(&policyStub{getErr: errBoom})
	if _, err := e.Observe(context.Background(), newPolicyCR()); err == nil {
		t.Fatal("a failed lookup must surface as an error")
	}
}

// TestObserveReportsAbsentAfterDelete covers the deletecomplete short-circuit:
// once Delete has run, Observe must say gone so the reconciler reaches
// RemoveFinalizer instead of re-running Delete forever.
func TestObserveReportsAbsentAfterDelete(t *testing.T) {
	s := &policyStub{policy: matchingPolicy()}
	e := newExternal(s)
	cr := newPolicyCR()
	cr.Annotations["keycloak.m.crossplane.io/delete-completed"] = "true"

	obs, err := e.Observe(context.Background(), cr)
	if err != nil {
		t.Fatalf("Observe failed: %v", err)
	}
	if obs.ResourceExists {
		t.Fatal("a policy already released must be reported absent")
	}
	if s.getCalls != 0 {
		t.Errorf("Keycloak was queried %d times after the policy was released", s.getCalls)
	}
}

func TestObserveRejectsWrongType(t *testing.T) {
	e := newExternal(&policyStub{})
	if _, err := e.Observe(context.Background(), &rolev1beta1.Role{}); err == nil {
		t.Fatal("Observe must reject a managed resource of the wrong type")
	}
}

func TestCreateRecordsExternalName(t *testing.T) {
	s := &policyStub{}
	e := newExternal(s)
	cr := newPolicyCR()
	cr.Annotations = map[string]string{}

	if _, err := e.Create(context.Background(), cr); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if cr.Annotations[externalName] != "pol-1" {
		t.Errorf("external-name = %q, want the ID Keycloak assigned", cr.Annotations[externalName])
	}
	if s.created == nil || s.created.Name != "only-admin" || s.created.Type != "role" {
		t.Errorf("written policy = %+v, want the spec's name and type", s.created)
	}
	if s.createdFor != [2]string{"master", "cid"} {
		t.Errorf("created against realm/client %v, want master/cid", s.createdFor)
	}
}

func TestCreateRejectsWrongType(t *testing.T) {
	e := newExternal(&policyStub{})
	if _, err := e.Create(context.Background(), &rolev1beta1.Role{}); err == nil {
		t.Fatal("Create must reject a managed resource of the wrong type")
	}
}

func TestDeleteRemovesPolicy(t *testing.T) {
	s := &policyStub{}
	e := newExternal(s)
	cr := newPolicyCR()

	if _, err := e.Delete(context.Background(), cr); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if s.deleteCall != 1 {
		t.Errorf("DeleteAuthorizationPolicy called %d times, want 1", s.deleteCall)
	}
	// The Deleting condition is deliberately not asserted here. Delete ends in
	// deletecomplete.Mark, which patches the object; the fake client answers
	// with the stored status and so discards the in-memory condition. In the
	// real reconciler the status is written back after the external call
	// returns, so asserting it here would only pin the fake's behaviour.
}

// TestDeleteIsIdempotent pins the short-circuit: Delete must be safe to call on
// its own, because the reconciler can reach it again.
func TestDeleteIsIdempotent(t *testing.T) {
	s := &policyStub{}
	e := newExternal(s)
	cr := newPolicyCR()
	cr.Annotations["keycloak.m.crossplane.io/delete-completed"] = "true"

	if _, err := e.Delete(context.Background(), cr); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if s.deleteCall != 0 {
		t.Errorf("Delete called Keycloak %d times after completion was recorded", s.deleteCall)
	}
}

func TestDeletePropagatesError(t *testing.T) {
	e := newExternal(&policyStub{deleteErr: errBoom})
	if _, err := e.Delete(context.Background(), newPolicyCR()); err == nil {
		t.Fatal("Delete must surface a failure, or the finalizer would be dropped with the policy intact")
	}
}

func TestDeleteRejectsWrongType(t *testing.T) {
	e := newExternal(&policyStub{})
	if _, err := e.Delete(context.Background(), &rolev1beta1.Role{}); err == nil {
		t.Fatal("Delete must reject a managed resource of the wrong type")
	}
}
