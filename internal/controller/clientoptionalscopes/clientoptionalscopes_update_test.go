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

package clientoptionalscopes

import (
	"context"
	"errors"
	"sort"
	"testing"

	realmv1beta1 "github.com/rossigee/provider-keycloak/apis/realm/v1beta1"
	"github.com/rossigee/provider-keycloak/internal/clients"
	"github.com/rossigee/provider-keycloak/internal/controller/testhelpers"
)

// recordingOptsStub records the scope assignments Update makes, so the tests can
// assert what actually reached the Keycloak API rather than just that no error
// came back.
type recordingOptsStub struct {
	*optsStub

	added, removed     []string
	addCalls, remCalls int
	addErr, remErr     error
}

func newRecordingOpts(current ...clients.ClientScopeRepresentation) *recordingOptsStub {
	s := newOptsStub()
	s.currentOptional = current
	return &recordingOptsStub{optsStub: s}
}

func (s *recordingOptsStub) AddClientOptionalScopes(_ context.Context, _, _ string, scopes []clients.ClientScopeRepresentation) error {
	s.addCalls++
	if s.addErr != nil {
		return s.addErr
	}
	for _, sc := range scopes {
		s.added = append(s.added, sc.Name)
	}
	return nil
}

func (s *recordingOptsStub) RemoveClientOptionalScopes(_ context.Context, _, _ string, scopes []clients.ClientScopeRepresentation) error {
	s.remCalls++
	if s.remErr != nil {
		return s.remErr
	}
	for _, sc := range scopes {
		s.removed = append(s.removed, sc.Name)
	}
	return nil
}

func names(v []string) []string {
	out := append([]string{}, v...)
	sort.Strings(out)
	return out
}

func equalNames(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// scopeDiff is the reconciliation primitive: it returns the entries of desired
// that are not already present in current. Update calls it twice, once in each
// direction, to work out what to add and what to remove.
func TestScopeDiff(t *testing.T) {
	groups := clients.ClientScopeRepresentation{ID: optgroupsID, Name: "groups"}
	profile := clients.ClientScopeRepresentation{ID: optprofileID, Name: "profile"}

	t.Run("returns desired entries missing from current", func(t *testing.T) {
		got := scopeDiff([]clients.ClientScopeRepresentation{groups, profile}, []clients.ClientScopeRepresentation{groups})
		if !equalNames(scopeNames(got), []string{"profile"}) {
			t.Errorf("scopeDiff = %v, want [profile]", scopeNames(got))
		}
	})

	t.Run("empty when every desired entry is present", func(t *testing.T) {
		got := scopeDiff([]clients.ClientScopeRepresentation{groups}, []clients.ClientScopeRepresentation{groups, profile})
		if len(got) != 0 {
			t.Errorf("scopeDiff = %v, want empty", scopeNames(got))
		}
	})

	t.Run("matches on id alone when the name is unset", func(t *testing.T) {
		byID := clients.ClientScopeRepresentation{ID: optgroupsID}
		got := scopeDiff([]clients.ClientScopeRepresentation{byID}, []clients.ClientScopeRepresentation{groups})
		if len(got) != 0 {
			t.Errorf("scopeDiff = %v, want empty: id alone should match", scopeNames(got))
		}
	})

	t.Run("matches on name alone when the id is unset", func(t *testing.T) {
		byName := clients.ClientScopeRepresentation{Name: "groups"}
		got := scopeDiff([]clients.ClientScopeRepresentation{byName}, []clients.ClientScopeRepresentation{groups})
		if len(got) != 0 {
			t.Errorf("scopeDiff = %v, want empty: name alone should match", scopeNames(got))
		}
	})

	// Matching is on id OR name, so a scope that shares a name with an assigned
	// scope but has a different id is treated as already assigned. That is
	// deliberate leniency - it avoids churning the client when Keycloak hands
	// back a scope whose id the spec does not name - but it also means a spec
	// asking for one id cannot correct the client onto another id that happens
	// to share the name. Pinned here so the behaviour is a decision rather than
	// an accident.
	t.Run("treats a same-named scope with a different id as already present", func(t *testing.T) {
		wanted := clients.ClientScopeRepresentation{ID: "different-id", Name: "groups"}
		got := scopeDiff([]clients.ClientScopeRepresentation{wanted}, []clients.ClientScopeRepresentation{groups})
		if len(got) != 0 {
			t.Errorf("scopeDiff = %v, want empty: name match should win over a differing id", scopeNames(got))
		}
	})

	t.Run("returns everything when current is empty", func(t *testing.T) {
		got := scopeDiff([]clients.ClientScopeRepresentation{groups, profile}, nil)
		if !equalNames(scopeNames(got), []string{"groups", "profile"}) {
			t.Errorf("scopeDiff = %v, want both", scopeNames(got))
		}
	})
}

func scopeNames(s []clients.ClientScopeRepresentation) []string {
	out := make([]string, 0, len(s))
	for _, v := range s {
		out = append(out, v.Name)
	}
	return out
}

// Update is the whole point of this resource: bring the client's optional scopes
// in line with the spec by adding what is missing and removing what is not.
func TestUpdateClientOptionalScopesReconcilesBothDirections(t *testing.T) {
	s := newRecordingOpts(
		clients.ClientScopeRepresentation{ID: optgroupsID, Name: "groups"},
		clients.ClientScopeRepresentation{ID: "stale-id", Name: "offline_access"},
	)

	if _, err := UpdateClientOptionalScopes(context.Background(), s, newOptCR([]string{"groups", "profile"})); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := names(s.added); !equalNames(got, []string{"profile"}) {
		t.Errorf("added = %v, want [profile]", got)
	}
	if got := names(s.removed); !equalNames(got, []string{"offline_access"}) {
		t.Errorf("removed = %v, want [offline_access]", got)
	}
}

// When the client already matches the spec, Update must not call the API at all.
// Re-adding an assigned scope is a no-op at best and a 409 at worst. The call
// counters matter here: asserting only that no scope names were recorded would
// also pass if Update called Add with an empty slice, which is a request the
// spec does not justify.
func TestUpdateClientOptionalScopesNoOpWhenInSync(t *testing.T) {
	s := newRecordingOpts(clients.ClientScopeRepresentation{ID: optgroupsID, Name: "groups"})

	if _, err := UpdateClientOptionalScopes(context.Background(), s, newOptCR([]string{"groups"})); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if s.addCalls != 0 {
		t.Errorf("AddClientOptionalScopes called %d time(s) with nothing to add", s.addCalls)
	}
	if s.remCalls != 0 {
		t.Errorf("RemoveClientOptionalScopes called %d time(s) with nothing to remove", s.remCalls)
	}
	if len(s.added) != 0 {
		t.Errorf("added = %v, want none", s.added)
	}
	if len(s.removed) != 0 {
		t.Errorf("removed = %v, want none", s.removed)
	}
}

func TestUpdateClientOptionalScopesErrors(t *testing.T) {
	boom := errors.New("boom")

	t.Run("unresolvable client", func(t *testing.T) {
		s := newRecordingOpts()
		s.clientByName = "someone-else"
		if _, err := UpdateClientOptionalScopes(context.Background(), s, newOptCR([]string{"groups"})); err == nil {
			t.Fatal("expected an error for an unknown client")
		}
	})

	t.Run("unresolvable scope", func(t *testing.T) {
		s := newRecordingOpts()
		if _, err := UpdateClientOptionalScopes(context.Background(), s, newOptCR([]string{"nonexistent"})); err == nil {
			t.Fatal("expected an error for an unknown scope")
		}
	})

	t.Run("list fails", func(t *testing.T) {
		s := newRecordingOpts()
		s.listErr = boom
		if _, err := UpdateClientOptionalScopes(context.Background(), s, newOptCR([]string{"groups"})); err == nil {
			t.Fatal("expected the list error")
		}
	})

	t.Run("add fails", func(t *testing.T) {
		s := newRecordingOpts(clients.ClientScopeRepresentation{ID: optgroupsID, Name: "groups"})
		s.addErr = boom
		if _, err := UpdateClientOptionalScopes(context.Background(), s, newOptCR([]string{"groups", "profile"})); err == nil {
			t.Fatal("expected the add error")
		}
	})

	t.Run("remove fails", func(t *testing.T) {
		s := newRecordingOpts(
			clients.ClientScopeRepresentation{ID: optgroupsID, Name: "groups"},
			clients.ClientScopeRepresentation{ID: "stale-id", Name: "offline_access"},
		)
		s.remErr = boom
		if _, err := UpdateClientOptionalScopes(context.Background(), s, newOptCR([]string{"groups"})); err == nil {
			t.Fatal("expected the remove error")
		}
	})
}

// Add happens before remove, so a failure to add must not have already stripped
// scopes off the client - that would drop privileges the spec still wants.
func TestUpdateClientOptionalScopesDoesNotRemoveWhenAddFails(t *testing.T) {
	boom := errors.New("boom")
	s := newRecordingOpts(
		clients.ClientScopeRepresentation{ID: optgroupsID, Name: "groups"},
		clients.ClientScopeRepresentation{ID: "stale-id", Name: "offline_access"},
	)
	s.addErr = boom

	if _, err := UpdateClientOptionalScopes(context.Background(), s, newOptCR([]string{"groups", "profile"})); err == nil {
		t.Fatal("expected the add error")
	}
	if len(s.removed) != 0 {
		t.Errorf("removed = %v, want none: removals must not run after a failed add", s.removed)
	}
}

// The external methods are thin type-assertion shims. The important part is that
// a wrong managed type is rejected rather than panicking.
func TestExternalRejectsWrongManagedType(t *testing.T) {
	e := &external{client: &testhelpers.BaseMockClient{}}
	ctx := context.Background()

	if _, err := e.Observe(ctx, &realmv1beta1.Realm{}); err == nil {
		t.Error("Observe accepted an unrelated managed type")
	}
	if _, err := e.Create(ctx, &realmv1beta1.Realm{}); err == nil {
		t.Error("Create accepted an unrelated managed type")
	}
	if _, err := e.Update(ctx, &realmv1beta1.Realm{}); err == nil {
		t.Error("Update accepted an unrelated managed type")
	}
	if _, err := e.Delete(ctx, &realmv1beta1.Realm{}); err == nil {
		t.Error("Delete accepted an unrelated managed type")
	}
	if err := e.Disconnect(ctx); err != nil {
		t.Errorf("Disconnect returned %v, want nil", err)
	}
}
