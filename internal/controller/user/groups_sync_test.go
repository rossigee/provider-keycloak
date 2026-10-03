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

package user

import (
	"context"
	"errors"
	"sort"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	userv1beta1 "github.com/rossigee/provider-keycloak/apis/user/v1beta1"
	"github.com/rossigee/provider-keycloak/internal/clients"
	"github.com/rossigee/provider-keycloak/internal/controller/testhelpers"
)

// These tests drive sync() through the real groupsExternal.
//
// The pre-existing TestGroupsSyncAddsUserToDesiredGroup and
// TestGroupsSyncRemovesUnwantedGroupWhenExhaustive named themselves after this
// behaviour but called mockClient.AddUserToGroup / RemoveUserFromGroup
// directly, then asserted the flag that the mock's own function field had just
// set. Nothing in sync() executed - those tests would still pass if sync() were
// deleted. They are replaced by the ones below, which reach the production
// reconciliation path.

type membershipCall struct{ userID, groupID string }

// membershipRecorder builds a mock that records membership changes, returning
// it alongside the recorded additions and removals.
func membershipRecorder(t *testing.T, actual []clients.GroupRepresentation, addErr, removeErr, getErr error) (*mockGroupsClient, *[]membershipCall, *[]membershipCall) {
	t.Helper()

	added := &[]membershipCall{}
	removed := &[]membershipCall{}

	m := &mockGroupsClient{
		BaseMockClient: &testhelpers.BaseMockClient{},
		getUserGroupsFn: func(_ context.Context, _, _ string) ([]clients.GroupRepresentation, error) {
			return actual, getErr
		},
		addUserToGroupFn: func(_ context.Context, _, userID, groupID string) error {
			if addErr != nil {
				return addErr
			}
			*added = append(*added, membershipCall{userID, groupID})
			return nil
		},
		removeUserFromGroupFn: func(_ context.Context, _, userID, groupID string) error {
			if removeErr != nil {
				return removeErr
			}
			*removed = append(*removed, membershipCall{userID, groupID})
			return nil
		},
	}
	return m, added, removed
}

func newSyncExternal(t *testing.T, m *mockGroupsClient) *groupsExternal {
	t.Helper()
	return &groupsExternal{
		client: m,
		kube:   fake.NewClientBuilder().WithScheme(runtime.NewScheme()).Build(),
	}
}

func groupsCR(realmID, userID string, groupIDs []string, exhaustive *bool) *userv1beta1.Groups {
	return &userv1beta1.Groups{
		ObjectMeta: metav1.ObjectMeta{Name: "g", Namespace: "ns"},
		Spec: userv1beta1.GroupsSpec{
			ForProvider: userv1beta1.GroupsParameters{
				RealmId:    &realmID,
				UserId:     &userID,
				GroupIds:   groupIDs,
				Exhaustive: exhaustive,
			},
		},
	}
}

func groupIDsOf(calls []membershipCall) []string {
	out := make([]string, 0, len(calls))
	for _, c := range calls {
		out = append(out, c.groupID)
	}
	sort.Strings(out)
	return out
}

func equalIDs(a, b []string) bool {
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

func boolPtr(b bool) *bool { return &b }

func TestSyncAddsUserToDesiredGroupsTheyAreNotYetIn(t *testing.T) {
	m, added, removed := membershipRecorder(t, nil, nil, nil, nil)
	e := newSyncExternal(t, m)

	if err := e.sync(context.Background(), groupsCR("realm", "u1", []string{"g1", "g2"}, boolPtr(true))); err != nil {
		t.Fatalf("sync failed: %v", err)
	}

	if got := groupIDsOf(*added); !equalIDs(got, []string{"g1", "g2"}) {
		t.Errorf("added = %v, want [g1 g2]", got)
	}
	if len(*removed) != 0 {
		t.Errorf("removed = %v, want none", groupIDsOf(*removed))
	}
}

// A user already in a desired group must not be re-added: Keycloak returns 409
// on a duplicate membership, which would otherwise fail every reconcile.
func TestSyncDoesNotReAddExistingMembership(t *testing.T) {
	m, added, _ := membershipRecorder(t, []clients.GroupRepresentation{{ID: "g1"}}, nil, nil, nil)
	e := newSyncExternal(t, m)

	if err := e.sync(context.Background(), groupsCR("realm", "u1", []string{"g1"}, boolPtr(true))); err != nil {
		t.Fatalf("sync failed: %v", err)
	}

	if len(*added) != 0 {
		t.Errorf("added = %v, want no re-add of an existing membership", groupIDsOf(*added))
	}
}

// The security-relevant case: exhaustive mode must revoke groups that are no
// longer desired. If this branch regressed, a user removed from a privileged
// group would silently keep access.
func TestSyncRemovesUndesiredGroupWhenExhaustive(t *testing.T) {
	m, added, removed := membershipRecorder(t,
		[]clients.GroupRepresentation{{ID: "g1"}, {ID: "g2"}}, nil, nil, nil)
	e := newSyncExternal(t, m)

	if err := e.sync(context.Background(), groupsCR("realm", "u1", []string{"g1"}, boolPtr(true))); err != nil {
		t.Fatalf("sync failed: %v", err)
	}

	if got := groupIDsOf(*removed); !equalIDs(got, []string{"g2"}) {
		t.Errorf("removed = %v, want [g2]", got)
	}
	if got := groupIDsOf(*added); len(got) != 0 {
		t.Errorf("added = %v, want none", got)
	}
}

// Exhaustive is a *bool that defaults to true, making the resource authoritative
// for the user's whole membership set. A regression in boolValue's default would
// silently turn "revoke what is not listed" into "only ever add", leaving stale
// privileges in place - the kind of failure that would not show up as an error.
func TestSyncDefaultsToExhaustiveWhenUnset(t *testing.T) {
	m, _, removed := membershipRecorder(t,
		[]clients.GroupRepresentation{{ID: "g1"}, {ID: "g2"}}, nil, nil, nil)
	e := newSyncExternal(t, m)

	if err := e.sync(context.Background(), groupsCR("realm", "u1", []string{"g1"}, nil)); err != nil {
		t.Fatalf("sync failed: %v", err)
	}

	if got := groupIDsOf(*removed); !equalIDs(got, []string{"g2"}) {
		t.Errorf("removed = %v, want [g2]; Exhaustive must default to authoritative", got)
	}
}

// Non-exhaustive mode is additive only. Multiple non-exhaustive resources can
// then coexist for one user, which is why exhaustive ownership is detected as a
// conflict instead.
func TestSyncKeepsUndesiredGroupWhenNotExhaustive(t *testing.T) {
	m, _, removed := membershipRecorder(t,
		[]clients.GroupRepresentation{{ID: "g1"}, {ID: "g2"}}, nil, nil, nil)
	e := newSyncExternal(t, m)

	if err := e.sync(context.Background(), groupsCR("realm", "u1", []string{"g1"}, boolPtr(false))); err != nil {
		t.Fatalf("sync failed: %v", err)
	}

	if len(*removed) != 0 {
		t.Errorf("removed = %v, want none when Exhaustive is false", groupIDsOf(*removed))
	}
}

func TestSyncSurfacesErrors(t *testing.T) {
	boom := errors.New("boom")

	cases := map[string]struct {
		actual                    []clients.GroupRepresentation
		getErr, addErr, removeErr error
		cr                        *userv1beta1.Groups
	}{
		"get user groups fails": {
			getErr: boom,
			cr:     groupsCR("realm", "u1", []string{"g1"}, boolPtr(true)),
		},
		"add fails": {
			addErr: boom,
			cr:     groupsCR("realm", "u1", []string{"g1"}, boolPtr(true)),
		},
		"remove fails": {
			// Needs a pre-existing unwanted membership, otherwise the removal
			// loop has nothing to iterate and the error is never reached.
			actual:    []clients.GroupRepresentation{{ID: "stale"}},
			removeErr: boom,
			cr:        groupsCR("realm", "u1", []string{"g1"}, boolPtr(true)),
		},
		"realm id missing": {
			cr: groupsCR("", "u1", []string{"g1"}, boolPtr(true)),
		},
		"user id missing": {
			cr: groupsCR("realm", "", []string{"g1"}, boolPtr(true)),
		},
		"no groups listed": {
			cr: groupsCR("realm", "u1", nil, boolPtr(true)),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			m, _, _ := membershipRecorder(t, tc.actual, tc.addErr, tc.removeErr, tc.getErr)
			e := newSyncExternal(t, m)

			if err := e.sync(context.Background(), tc.cr); err == nil {
				t.Fatal("sync succeeded, want an error")
			}
		})
	}
}

func TestSyncStopsOnAddFailureWithoutProceeding(t *testing.T) {
	boom := errors.New("boom")
	m, _, removed := membershipRecorder(t,
		[]clients.GroupRepresentation{{ID: "stale"}}, boom, nil, nil)
	e := newSyncExternal(t, m)

	if err := e.sync(context.Background(), groupsCR("realm", "u1", []string{"g1"}, boolPtr(true))); err == nil {
		t.Fatal("sync succeeded, want the add error")
	}
	if len(*removed) != 0 {
		t.Errorf("removed = %v, want none: sync must not continue past a failed add", groupIDsOf(*removed))
	}
}
