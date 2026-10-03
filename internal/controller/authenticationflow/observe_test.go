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

package authenticationflow

import (
	"context"
	"errors"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
	authenticationflowv1beta1 "github.com/rossigee/provider-keycloak/apis/authenticationflow/v1beta1"
	rolev1beta1 "github.com/rossigee/provider-keycloak/apis/role/v1beta1"
	"github.com/rossigee/provider-keycloak/internal/clients"
	"github.com/rossigee/provider-keycloak/internal/controller/testhelpers"
)

var errBoom = errors.New("boom")

type flowStub struct {
	*testhelpers.BaseMockClient
	flow        *clients.AuthenticationFlowRepresentation
	getErr      error
	getCalls    int
	created     *clients.AuthenticationFlowRepresentation
	createErr   error
	updatedFor  string
	deleteCalls int
	deleteErr   error
}

func (s *flowStub) GetAuthenticationFlow(context.Context, string, string) (*clients.AuthenticationFlowRepresentation, error) {
	s.getCalls++
	if s.getErr != nil {
		return nil, s.getErr
	}
	return s.flow, nil
}

func (s *flowStub) CreateAuthenticationFlow(_ context.Context, _ string, rep *clients.AuthenticationFlowRepresentation) (string, error) {
	if s.createErr != nil {
		return "", s.createErr
	}
	s.created = rep
	return "flow-1", nil
}

func (s *flowStub) UpdateAuthenticationFlow(_ context.Context, _, alias string, _ *clients.AuthenticationFlowRepresentation) error {
	s.updatedFor = alias
	return nil
}

func (s *flowStub) DeleteAuthenticationFlow(context.Context, string, string) error {
	s.deleteCalls++
	return s.deleteErr
}

func strPtr(s string) *string { return &s }
func boolPtr(b bool) *bool    { return &b }

func newFlowCR() *authenticationflowv1beta1.AuthenticationFlow {
	return &authenticationflowv1beta1.AuthenticationFlow{
		ObjectMeta: metav1.ObjectMeta{Name: "f", Namespace: "ns"},
		Spec: authenticationflowv1beta1.AuthenticationFlowSpec{
			ForProvider: authenticationflowv1beta1.AuthenticationFlowParameters{
				RealmId: "master", Alias: "my-flow", Description: strPtr("d"),
				ProviderId: "basic-flow", BuiltIn: boolPtr(false),
			},
		},
	}
}

func newExternal(s *flowStub) *external {
	scheme := runtime.NewScheme()
	_ = authenticationflowv1beta1.AddToScheme(scheme)
	return &external{
		client: s,
		kube:   fake.NewClientBuilder().WithScheme(scheme).WithObjects(newFlowCR()).Build(),
	}
}

func matchingFlow() *clients.AuthenticationFlowRepresentation {
	return &clients.AuthenticationFlowRepresentation{
		Alias: "my-flow", Description: "d", ProviderId: "basic-flow", BuiltIn: false,
	}
}

func TestObserveReportsUpToDate(t *testing.T) {
	s := &flowStub{flow: matchingFlow()}
	e := newExternal(s)
	cr := newFlowCR()

	obs, err := e.Observe(context.Background(), cr)
	if err != nil {
		t.Fatalf("Observe failed: %v", err)
	}
	if !obs.ResourceExists || !obs.ResourceUpToDate {
		t.Fatalf("a flow matching the spec must be present and up to date, got %+v", obs)
	}
	if c := cr.Status.GetCondition(xpv1.TypeReady); c.Status != "True" {
		t.Errorf("Ready condition = %+v, want True", c)
	}
}

func TestObserveDetectsDrift(t *testing.T) {
	cases := map[string]func(*clients.AuthenticationFlowRepresentation){
		"alias":       func(f *clients.AuthenticationFlowRepresentation) { f.Alias = "other" },
		"description": func(f *clients.AuthenticationFlowRepresentation) { f.Description = "changed" },
		"provider":    func(f *clients.AuthenticationFlowRepresentation) { f.ProviderId = "other-flow" },
		"built in":    func(f *clients.AuthenticationFlowRepresentation) { f.BuiltIn = true },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			flow := matchingFlow()
			mutate(flow)
			e := newExternal(&flowStub{flow: flow})

			obs, err := e.Observe(context.Background(), newFlowCR())
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
	e := newExternal(&flowStub{getErr: errBoom})
	if _, err := e.Observe(context.Background(), newFlowCR()); err == nil {
		t.Fatal("a failed lookup must surface as an error")
	}
}

// TestObserveReportsAbsentAfterDelete covers the deletecomplete short-circuit:
// once Delete has run, Observe must say gone or the reconciler re-runs Delete
// forever and the finalizer is never removed.
func TestObserveReportsAbsentAfterDelete(t *testing.T) {
	s := &flowStub{flow: matchingFlow()}
	e := newExternal(s)
	cr := newFlowCR()
	cr.Annotations = map[string]string{"keycloak.m.crossplane.io/delete-completed": "true"}

	obs, err := e.Observe(context.Background(), cr)
	if err != nil {
		t.Fatalf("Observe failed: %v", err)
	}
	if obs.ResourceExists {
		t.Fatal("a flow already released must be reported absent")
	}
	if s.getCalls != 0 {
		t.Errorf("Keycloak was queried %d times after the flow was released", s.getCalls)
	}
}

func TestObserveRejectsWrongType(t *testing.T) {
	e := newExternal(&flowStub{})
	if _, err := e.Observe(context.Background(), &rolev1beta1.Role{}); err == nil {
		t.Fatal("Observe must reject a managed resource of the wrong type")
	}
}

func TestCreateWritesFlow(t *testing.T) {
	s := &flowStub{}
	e := newExternal(s)

	if _, err := e.Create(context.Background(), newFlowCR()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if s.created == nil || s.created.Alias != "my-flow" {
		t.Errorf("written flow = %+v, want the spec's alias", s.created)
	}
}

func TestCreatePropagatesError(t *testing.T) {
	e := newExternal(&flowStub{createErr: errBoom})
	if _, err := e.Create(context.Background(), newFlowCR()); err == nil {
		t.Fatal("Create must surface a write failure")
	}
}

func TestCreateRejectsWrongType(t *testing.T) {
	e := newExternal(&flowStub{})
	if _, err := e.Create(context.Background(), &rolev1beta1.Role{}); err == nil {
		t.Fatal("Create must reject a managed resource of the wrong type")
	}
}

func TestUpdateTargetsAlias(t *testing.T) {
	s := &flowStub{}
	e := newExternal(s)

	if _, err := e.Update(context.Background(), newFlowCR()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if s.updatedFor != "my-flow" {
		t.Errorf("updated alias = %q, want my-flow", s.updatedFor)
	}
}

func TestUpdateRejectsWrongType(t *testing.T) {
	e := newExternal(&flowStub{})
	if _, err := e.Update(context.Background(), &rolev1beta1.Role{}); err == nil {
		t.Fatal("Update must reject a managed resource of the wrong type")
	}
}

func TestDeleteRemovesFlow(t *testing.T) {
	s := &flowStub{}
	e := newExternal(s)

	if _, err := e.Delete(context.Background(), newFlowCR()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if s.deleteCalls != 1 {
		t.Errorf("DeleteAuthenticationFlow called %d times, want 1", s.deleteCalls)
	}
}

// TestDeleteIsIdempotent pins the short-circuit: Delete must be safe to call on
// its own, because the reconciler can reach it again.
func TestDeleteIsIdempotent(t *testing.T) {
	s := &flowStub{}
	e := newExternal(s)
	cr := newFlowCR()
	cr.Annotations = map[string]string{"keycloak.m.crossplane.io/delete-completed": "true"}

	if _, err := e.Delete(context.Background(), cr); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if s.deleteCalls != 0 {
		t.Errorf("Delete called Keycloak %d times after completion was recorded", s.deleteCalls)
	}
}

func TestDeletePropagatesError(t *testing.T) {
	e := newExternal(&flowStub{deleteErr: errBoom})
	if _, err := e.Delete(context.Background(), newFlowCR()); err == nil {
		t.Fatal("Delete must surface a failure, or the finalizer would be dropped with the flow intact")
	}
}

func TestDeleteRejectsWrongType(t *testing.T) {
	e := newExternal(&flowStub{})
	if _, err := e.Delete(context.Background(), &rolev1beta1.Role{}); err == nil {
		t.Fatal("Delete must reject a managed resource of the wrong type")
	}
}
