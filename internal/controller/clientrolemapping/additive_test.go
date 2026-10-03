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

package clientrolemapping

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	crv1beta1 "github.com/rossigee/provider-keycloak/apis/rolemappings/v1beta1"
	"github.com/rossigee/provider-keycloak/internal/clients"
	"github.com/rossigee/provider-keycloak/internal/controller/testhelpers"
)

// recordingClient captures the exact entries the controller adds and removes,
// which is the only way to assert the set is additive rather than exhaustive.
type recordingClient struct {
	*testhelpers.BaseMockClient
	current []clients.RoleRepresentation
	added   [][]clients.RoleRepresentation
	removed [][]clients.RoleRepresentation
	listErr error
	addErr  error
	rmErr   error
}

func (m *recordingClient) ListUserClientRoleMappings(context.Context, string, string, string) ([]clients.RoleRepresentation, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.current, nil
}

func (m *recordingClient) AddUserClientRoleMappings(_ context.Context, _, _, _ string, roles []clients.RoleRepresentation) error {
	if m.addErr != nil {
		return m.addErr
	}
	m.added = append(m.added, roles)
	m.current = append(m.current, roles...)
	return nil
}

func (m *recordingClient) RemoveUserClientRoleMappings(_ context.Context, _, _, _ string, roles []clients.RoleRepresentation) error {
	if m.rmErr != nil {
		return m.rmErr
	}
	m.removed = append(m.removed, roles)
	return nil
}

func (m *recordingClient) removedNames() []string {
	var out []string
	for _, batch := range m.removed {
		for _, r := range batch {
			out = append(out, r.Name)
		}
	}
	sort.Strings(out)
	return out
}

func (m *recordingClient) addedNames() []string {
	var out []string
	for _, batch := range m.added {
		for _, r := range batch {
			out = append(out, r.Name)
		}
	}
	sort.Strings(out)
	return out
}

var errBoom = errors.New("boom")

func role(name string) crv1beta1.RoleMapping { return crv1beta1.RoleMapping{Name: name} }

func reps(names ...string) []clients.RoleRepresentation {
	out := make([]clients.RoleRepresentation, 0, len(names))
	for _, n := range names {
		out = append(out, clients.RoleRepresentation{Name: n})
	}
	return out
}

func newResource(declared []crv1beta1.RoleMapping, applied []crv1beta1.RoleMapping) *crv1beta1.ClientRoleMapping {
	return &crv1beta1.ClientRoleMapping{
		ObjectMeta: metav1.ObjectMeta{Name: "rm", Namespace: "ns"},
		Spec: crv1beta1.ClientRoleMappingSpec{
			ForProvider: crv1beta1.ClientRoleMappingParameters{
				RealmId: "realm", UserId: "user", ClientId: "client", Roles: declared,
			},
		},
		Status: crv1beta1.ClientRoleMappingStatus{AppliedRoles: applied},
	}
}

func newExternal(c *recordingClient) *external {
	scheme := runtime.NewScheme()
	_ = crv1beta1.AddToScheme(scheme)
	obj := newResource(nil, nil)
	return &external{client: c, kube: fake.NewClientBuilder().WithScheme(scheme).WithObjects(obj).Build()}
}

// TestObserveTolerantOfSiblingEntries is the core of the change: a second
// resource sharing the same user and client must not make the first one look
// out of date. Under the previous exhaustive comparison the lengths differed, so
// every sibling entry drove a permanent update loop.
func TestObserveTolerantOfSiblingEntries(t *testing.T) {
	c := &recordingClient{current: reps("admin", "backups")}
	e := newExternal(c)
	cr := newResource([]crv1beta1.RoleMapping{role("admin")}, []crv1beta1.RoleMapping{role("admin")})

	obs, err := e.Observe(context.Background(), cr)
	if err != nil {
		t.Fatalf("Observe failed: %v", err)
	}
	if !obs.ResourceExists {
		t.Fatal("resource must still exist")
	}
	if !obs.ResourceUpToDate {
		t.Fatal("a sibling's entry on the same user must not make this resource out of date")
	}
}

// TestObserveReAddsExternallyRemovedRole proves drift correction survives:
// ownership tracks what was applied, not merely what is declared.
func TestObserveReAddsExternallyRemovedRole(t *testing.T) {
	c := &recordingClient{current: reps()}
	e := newExternal(c)
	cr := newResource([]crv1beta1.RoleMapping{role("admin")}, []crv1beta1.RoleMapping{role("admin")})

	obs, err := e.Observe(context.Background(), cr)
	if err != nil {
		t.Fatalf("Observe failed: %v", err)
	}
	if obs.ResourceUpToDate {
		t.Fatal("a declared role missing from the user must be reported out of date so it is restored")
	}
	if len(cr.Status.AppliedRoles) != 0 {
		t.Fatalf("ownership must drop the role Keycloak no longer has, got %v", cr.Status.AppliedRoles)
	}
}

// TestUpdateRemovesOnlyWhatItOwned is the regression test for the destructive
// half of the old behaviour, where Update removed every role not declared -
// including a sibling's.
func TestUpdateRemovesOnlyWhatItOwned(t *testing.T) {
	c := &recordingClient{current: reps("admin", "backups", "stale")}
	e := newExternal(c)
	cr := newResource(
		[]crv1beta1.RoleMapping{role("admin")},
		[]crv1beta1.RoleMapping{role("admin"), role("stale")},
	)

	if _, err := e.Update(context.Background(), cr); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if got := strings.Join(c.removedNames(), ","); got != "stale" {
		t.Fatalf("expected only the no-longer-declared owned role to be removed, got %q", got)
	}
	if len(c.added) != 0 {
		t.Fatalf("nothing needed adding, got %v", c.addedNames())
	}
}

// TestUpdateAddsWithoutAdopting covers a resource that has applied nothing yet,
// which is the state after an upgrade from the exhaustive behaviour.
func TestUpdateAddsWithoutAdopting(t *testing.T) {
	c := &recordingClient{current: reps("backups")}
	e := newExternal(c)
	cr := newResource([]crv1beta1.RoleMapping{role("admin")}, nil)

	if _, err := e.Update(context.Background(), cr); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if got := strings.Join(c.addedNames(), ","); got != "admin" {
		t.Fatalf("expected the declared role to be added, got %q", got)
	}
	if len(c.removed) != 0 {
		t.Fatalf("a sibling's role must never be removed, got %v", c.removedNames())
	}
}

// TestDeleteRemovesOnlyOwnedEntries is the other destructive regression: Delete
// used to strip every role on the user.
func TestDeleteRemovesOnlyOwnedEntries(t *testing.T) {
	c := &recordingClient{current: reps("admin", "backups")}
	e := newExternal(c)
	cr := newResource([]crv1beta1.RoleMapping{role("admin")}, []crv1beta1.RoleMapping{role("admin")})

	if _, err := e.Delete(context.Background(), cr); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if got := strings.Join(c.removedNames(), ","); got != "admin" {
		t.Fatalf("Delete must remove only this resource's own role, got %q", got)
	}
}

// TestDeleteToleratesAbsentOwnedRole covers a role removed in Keycloak before
// the resource was deleted, which must not block termination.
func TestDeleteToleratesAbsentOwnedRole(t *testing.T) {
	c := &recordingClient{current: reps("backups")}
	e := newExternal(c)
	cr := newResource([]crv1beta1.RoleMapping{role("admin")}, []crv1beta1.RoleMapping{role("admin")})

	if _, err := e.Delete(context.Background(), cr); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if len(c.removed) != 0 {
		t.Fatalf("a role already gone must not be sent for removal, got %v", c.removedNames())
	}
}

// TestUpdateDoesNotAdoptOnFailedAdd keeps a failed write out of the owned set,
// so the resource cannot later remove something it never successfully applied.
func TestUpdateDoesNotAdoptOnFailedAdd(t *testing.T) {
	c := &recordingClient{current: reps(), addErr: errBoom}
	e := newExternal(c)
	cr := newResource([]crv1beta1.RoleMapping{role("admin")}, nil)

	if _, err := e.Update(context.Background(), cr); err == nil {
		t.Fatal("Update must surface the add failure")
	}
	if len(cr.Status.AppliedRoles) != 0 {
		t.Fatalf("a failed add must not be recorded as applied, got %v", cr.Status.AppliedRoles)
	}
}

// TestMigrationFromExhaustiveOwnership pins the upgrade path. Under the old
// behaviour status recorded every role on the user, so an entry the spec no
// longer declared must still be removed exactly once - and, just as before,
// nothing else.
func TestMigrationFromExhaustiveOwnership(t *testing.T) {
	c := &recordingClient{current: reps("admin", "backups")}
	e := newExternal(c)

	// Recorded by the exhaustive controller before the upgrade.
	stale := []crv1beta1.RoleMapping{role("admin"), role("backups")}
	cr := newResource([]crv1beta1.RoleMapping{role("admin")}, stale)

	// The dropped role must still drive an update, or it would never be removed.
	obs, err := e.Observe(context.Background(), cr)
	if err != nil {
		t.Fatalf("Observe failed: %v", err)
	}
	if obs.ResourceUpToDate {
		t.Fatal("a role this resource applied but no longer declares must be reported out of date, or Update never runs and it is never removed")
	}

	if _, err := e.Update(context.Background(), cr); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if got := strings.Join(c.removedNames(), ","); got != "backups" {
		t.Fatalf("only the dropped role may be removed, got %q", got)
	}

	// And it settles afterwards.
	obs, err = e.Observe(context.Background(), cr)
	if err != nil {
		t.Fatalf("Observe failed: %v", err)
	}
	if !obs.ResourceUpToDate {
		t.Fatal("resource must settle once the dropped role is gone")
	}
}

// TestReconcileFlowRemovesRoleDroppedFromSpec walks the flow the managed
// reconciler actually takes - Observe, then Update only when Observe reports
// the resource out of date. Calling Update directly would miss the case this
// guards: a role dropped from the spec is a subset of what the user still has,
// so a desired-only check reports the resource settled and the role is never
// taken off the user.
func TestReconcileFlowRemovesRoleDroppedFromSpec(t *testing.T) {
	c := &recordingClient{current: reps("admin", "backups")}
	e := newExternal(c)
	cr := newResource([]crv1beta1.RoleMapping{role("admin")}, []crv1beta1.RoleMapping{role("admin")})

	obs, err := e.Observe(context.Background(), cr)
	if err != nil {
		t.Fatalf("Observe failed: %v", err)
	}
	if !obs.ResourceUpToDate {
		t.Fatal("a sibling's role alone must not trigger an update")
	}
	if len(c.removed) != 0 {
		t.Fatalf("nothing may be removed, got %v", c.removedNames())
	}

	// Drop the role from the spec, as a user would.
	cr.Spec.ForProvider.Roles = []crv1beta1.RoleMapping{}
	cr.Status.AppliedRoles = []crv1beta1.RoleMapping{role("admin")}

	obs, err = e.Observe(context.Background(), cr)
	if err != nil {
		t.Fatalf("Observe failed: %v", err)
	}
	if obs.ResourceUpToDate {
		t.Fatal("dropping the last role from the spec must report the resource out of date so it is released")
	}
	if _, err := e.Update(context.Background(), cr); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if got := strings.Join(c.removedNames(), ","); got != "admin" {
		t.Fatalf("the dropped role must be released, got %q", got)
	}
}
