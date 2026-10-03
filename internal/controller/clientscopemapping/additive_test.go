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

package clientscopemapping

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	csv1beta1 "github.com/rossigee/provider-keycloak/apis/scopes/v1beta1"
	"github.com/rossigee/provider-keycloak/internal/clients"
	"github.com/rossigee/provider-keycloak/internal/controller/testhelpers"
)

var errBoom = errors.New("boom")

type recordingClient struct {
	*testhelpers.BaseMockClient
	current []clients.RoleRepresentation
	added   [][]clients.RoleRepresentation
	removed [][]clients.RoleRepresentation
	addErr  error
}

func (m *recordingClient) ListClientScopeMappings(context.Context, string, string) ([]clients.RoleRepresentation, error) {
	return m.current, nil
}

func (m *recordingClient) AddClientScopeMappings(_ context.Context, _, _ string, scopes []clients.RoleRepresentation) error {
	if m.addErr != nil {
		return m.addErr
	}
	m.added = append(m.added, scopes)
	m.current = append(m.current, scopes...)
	return nil
}

func (m *recordingClient) RemoveClientScopeMappings(_ context.Context, _, _ string, scopes []clients.RoleRepresentation) error {
	m.removed = append(m.removed, scopes)
	return nil
}

func (m *recordingClient) removedNames() []string {
	var out []string
	for _, batch := range m.removed {
		for _, s := range batch {
			out = append(out, s.Name)
		}
	}
	sort.Strings(out)
	return out
}

func (m *recordingClient) addedNames() []string {
	var out []string
	for _, batch := range m.added {
		for _, s := range batch {
			out = append(out, s.Name)
		}
	}
	sort.Strings(out)
	return out
}

func scope(name string) csv1beta1.ScopeMapping { return csv1beta1.ScopeMapping{Name: name} }

func reps(names ...string) []clients.RoleRepresentation {
	out := make([]clients.RoleRepresentation, 0, len(names))
	for _, n := range names {
		out = append(out, clients.RoleRepresentation{Name: n})
	}
	return out
}

func newResource(declared, applied []csv1beta1.ScopeMapping) *csv1beta1.ClientScopeMapping {
	return &csv1beta1.ClientScopeMapping{
		ObjectMeta: metav1.ObjectMeta{Name: "sm", Namespace: "ns"},
		Spec: csv1beta1.ClientScopeMappingSpec{
			ForProvider: csv1beta1.ClientScopeMappingParameters{
				RealmId: "realm", ClientId: "client", Scopes: declared,
			},
		},
		Status: csv1beta1.ClientScopeMappingStatus{AppliedScopes: applied},
	}
}

func newExternal(c *recordingClient) *external {
	scheme := runtime.NewScheme()
	_ = csv1beta1.AddToScheme(scheme)
	obj := newResource(nil, nil)
	return &external{client: c, kube: fake.NewClientBuilder().WithScheme(scheme).WithObjects(obj).Build()}
}

func TestObserveTolerantOfSiblingEntries(t *testing.T) {
	c := &recordingClient{current: reps("openid", "email")}
	e := newExternal(c)
	cr := newResource([]csv1beta1.ScopeMapping{scope("openid")}, []csv1beta1.ScopeMapping{scope("openid")})

	obs, err := e.Observe(context.Background(), cr)
	if err != nil {
		t.Fatalf("Observe failed: %v", err)
	}
	if !obs.ResourceUpToDate {
		t.Fatal("a sibling's scope on the same client must not make this resource out of date")
	}
}

func TestObserveReAddsExternallyRemovedScope(t *testing.T) {
	c := &recordingClient{current: reps()}
	e := newExternal(c)
	cr := newResource([]csv1beta1.ScopeMapping{scope("openid")}, []csv1beta1.ScopeMapping{scope("openid")})

	obs, err := e.Observe(context.Background(), cr)
	if err != nil {
		t.Fatalf("Observe failed: %v", err)
	}
	if obs.ResourceUpToDate {
		t.Fatal("a declared scope missing from the client must be reported out of date so it is restored")
	}
}

func TestUpdateRemovesOnlyWhatItOwned(t *testing.T) {
	c := &recordingClient{current: reps("openid", "email", "stale")}
	e := newExternal(c)
	cr := newResource(
		[]csv1beta1.ScopeMapping{scope("openid")},
		[]csv1beta1.ScopeMapping{scope("openid"), scope("stale")},
	)

	if _, err := e.Update(context.Background(), cr); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if got := strings.Join(c.removedNames(), ","); got != "stale" {
		t.Fatalf("expected only the no-longer-declared owned scope to be removed, got %q", got)
	}
}

func TestUpdateAddsWithoutAdopting(t *testing.T) {
	c := &recordingClient{current: reps("email")}
	e := newExternal(c)
	cr := newResource([]csv1beta1.ScopeMapping{scope("openid")}, nil)

	if _, err := e.Update(context.Background(), cr); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if got := strings.Join(c.addedNames(), ","); got != "openid" {
		t.Fatalf("expected the declared scope to be added, got %q", got)
	}
	if len(c.removed) != 0 {
		t.Fatalf("a sibling's scope must never be removed, got %v", c.removedNames())
	}
}

func TestDeleteRemovesOnlyOwnedEntries(t *testing.T) {
	c := &recordingClient{current: reps("openid", "email")}
	e := newExternal(c)
	cr := newResource([]csv1beta1.ScopeMapping{scope("openid")}, []csv1beta1.ScopeMapping{scope("openid")})

	if _, err := e.Delete(context.Background(), cr); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if got := strings.Join(c.removedNames(), ","); got != "openid" {
		t.Fatalf("Delete must remove only this resource's own scope, got %q", got)
	}
}

func TestUpdateDoesNotAdoptOnFailedAdd(t *testing.T) {
	c := &recordingClient{current: reps(), addErr: errBoom}
	e := newExternal(c)
	cr := newResource([]csv1beta1.ScopeMapping{scope("openid")}, nil)

	if _, err := e.Update(context.Background(), cr); err == nil {
		t.Fatal("Update must surface the add failure")
	}
	if len(cr.Status.AppliedScopes) != 0 {
		t.Fatalf("a failed add must not be recorded as applied, got %v", cr.Status.AppliedScopes)
	}
}

func TestMigrationFromExhaustiveOwnership(t *testing.T) {
	c := &recordingClient{current: reps("openid", "email")}
	e := newExternal(c)
	stale := []csv1beta1.ScopeMapping{scope("openid"), scope("email")}
	cr := newResource([]csv1beta1.ScopeMapping{scope("openid")}, stale)

	// The dropped scope must still drive an update, or it would never be removed.
	obs, err := e.Observe(context.Background(), cr)
	if err != nil {
		t.Fatalf("Observe failed: %v", err)
	}
	if obs.ResourceUpToDate {
		t.Fatal("a scope this resource applied but no longer declares must be reported out of date, or Update never runs and it is never removed")
	}

	if _, err := e.Update(context.Background(), cr); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if got := strings.Join(c.removedNames(), ","); got != "email" {
		t.Fatalf("only the dropped scope may be removed, got %q", got)
	}
}

// TestReconcileFlowRemovesScopeDroppedFromSpec walks the flow the managed
// reconciler actually takes. A scope dropped from the spec is a subset of what
// the client still has, so a desired-only check would report the resource
// settled and the scope would never be taken off.
func TestReconcileFlowRemovesScopeDroppedFromSpec(t *testing.T) {
	c := &recordingClient{current: reps("openid", "email")}
	e := newExternal(c)
	cr := newResource([]csv1beta1.ScopeMapping{scope("openid")}, []csv1beta1.ScopeMapping{scope("openid")})

	obs, err := e.Observe(context.Background(), cr)
	if err != nil {
		t.Fatalf("Observe failed: %v", err)
	}
	if !obs.ResourceUpToDate {
		t.Fatal("a sibling's scope alone must not trigger an update")
	}
	if len(c.removed) != 0 {
		t.Fatalf("nothing may be removed, got %v", c.removedNames())
	}

	cr.Spec.ForProvider.Scopes = []csv1beta1.ScopeMapping{}
	cr.Status.AppliedScopes = []csv1beta1.ScopeMapping{scope("openid")}

	obs, err = e.Observe(context.Background(), cr)
	if err != nil {
		t.Fatalf("Observe failed: %v", err)
	}
	if obs.ResourceUpToDate {
		t.Fatal("dropping the last scope from the spec must report the resource out of date so it is released")
	}
	if _, err := e.Update(context.Background(), cr); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if got := strings.Join(c.removedNames(), ","); got != "openid" {
		t.Fatalf("the dropped scope must be released, got %q", got)
	}
}
