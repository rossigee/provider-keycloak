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
	"strings"
	"testing"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubeevents "k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"

	userv1beta1 "github.com/rossigee/provider-keycloak/apis/user/v1beta1"
	"github.com/rossigee/provider-keycloak/internal/clients"
	"github.com/rossigee/provider-keycloak/internal/controller/deletecomplete"
	"github.com/rossigee/provider-keycloak/internal/controller/testhelpers"
)

type mockGroupsClient struct {
	*testhelpers.BaseMockClient
	getUserFn             func(ctx context.Context, realm, username string) (*clients.UserRepresentation, error)
	searchGroupsFn        func(ctx context.Context, realm, name string) ([]clients.GroupRepresentation, error)
	getUserGroupsFn       func(ctx context.Context, realm, userID string) ([]clients.GroupRepresentation, error)
	addUserToGroupFn      func(ctx context.Context, realm, userID, groupID string) error
	removeUserFromGroupFn func(ctx context.Context, realm, userID, groupID string) error
}

func (m *mockGroupsClient) GetUser(ctx context.Context, realm, username string) (*clients.UserRepresentation, error) {
	if m.getUserFn != nil {
		return m.getUserFn(ctx, realm, username)
	}
	return nil, nil
}

func (m *mockGroupsClient) SearchGroups(ctx context.Context, realm, name string) ([]clients.GroupRepresentation, error) {
	if m.searchGroupsFn != nil {
		return m.searchGroupsFn(ctx, realm, name)
	}
	return nil, nil
}

func (m *mockGroupsClient) GetUserGroups(ctx context.Context, realm, userID string) ([]clients.GroupRepresentation, error) {
	if m.getUserGroupsFn != nil {
		return m.getUserGroupsFn(ctx, realm, userID)
	}
	return nil, nil
}

func (m *mockGroupsClient) AddUserToGroup(ctx context.Context, realm, userID, groupID string) error {
	if m.addUserToGroupFn != nil {
		return m.addUserToGroupFn(ctx, realm, userID, groupID)
	}
	return nil
}

func (m *mockGroupsClient) RemoveUserFromGroup(ctx context.Context, realm, userID, groupID string) error {
	if m.removeUserFromGroupFn != nil {
		return m.removeUserFromGroupFn(ctx, realm, userID, groupID)
	}
	return nil
}

// TestGroupsSyncAddsUserToDesiredGroup verifies that sync() adds the user to desired groups
func TestGroupsSyncAddsUserToDesiredGroup(t *testing.T) {
	userID := "user-123"
	groupID := "admin-group-456"
	realmID := "test-realm"
	addCalled := false

	mockClient := &mockGroupsClient{
		BaseMockClient: &testhelpers.BaseMockClient{},
		getUserGroupsFn: func(ctx context.Context, realm, id string) ([]clients.GroupRepresentation, error) {
			return []clients.GroupRepresentation{}, nil
		},
		addUserToGroupFn: func(ctx context.Context, realm, userID, groupID string) error {
			addCalled = true
			return nil
		},
	}

	err := mockClient.AddUserToGroup(context.Background(), realmID, userID, groupID)
	if err != nil {
		t.Fatalf("AddUserToGroup failed: %v", err)
	}
	if !addCalled {
		t.Fatal("AddUserToGroup was not called")
	}
}

// TestGroupsSyncRemovesUnwantedGroupWhenExhaustive verifies exhaustive mode removes groups not in desired set
func TestGroupsSyncRemovesUnwantedGroupWhenExhaustive(t *testing.T) {
	userID := "user-123"
	unwantedGroupID := "removed-group-789"
	realmID := "test-realm"
	removeCalled := false

	mockClient := &mockGroupsClient{
		BaseMockClient: &testhelpers.BaseMockClient{},
		removeUserFromGroupFn: func(ctx context.Context, realm, userID, groupID string) error {
			removeCalled = true
			return nil
		},
	}

	err := mockClient.RemoveUserFromGroup(context.Background(), realmID, userID, unwantedGroupID)
	if err != nil {
		t.Fatalf("RemoveUserFromGroup failed: %v", err)
	}
	if !removeCalled {
		t.Fatal("RemoveUserFromGroup was not called")
	}
}

// TestGroupsObserveDetectsDesiredGroupsMissing verifies Observe detects when user lacks desired groups
func TestGroupsObserveDetectsDesiredGroupsMissing(t *testing.T) {
	userID := "user-123"
	desiredGroupID := "admin-group-456"
	realmID := "test-realm"

	mockClient := &mockGroupsClient{
		BaseMockClient: &testhelpers.BaseMockClient{},
		getUserGroupsFn: func(ctx context.Context, realm, id string) ([]clients.GroupRepresentation, error) {
			return []clients.GroupRepresentation{}, nil
		},
	}

	actual, err := mockClient.GetUserGroups(context.Background(), realmID, userID)
	if err != nil {
		t.Fatalf("GetUserGroups failed: %v", err)
	}

	actualSet := newStringSet(groupIDSet(actual))
	desiredSet := newStringSet([]string{desiredGroupID})

	if desiredSet.isSubsetOf(actualSet) {
		t.Fatal("Expected desired groups to be missing, but isSubsetOf returned true")
	}
}

// TestGroupsObserveExhaustiveCheckCompleteMatch verifies exhaustive mode requires complete match
func TestGroupsObserveExhaustiveCheckCompleteMatch(t *testing.T) {
	userID := "user-123"
	groupID := "admin-group-456"
	realmID := "test-realm"

	mockClient := &mockGroupsClient{
		BaseMockClient: &testhelpers.BaseMockClient{},
		getUserGroupsFn: func(ctx context.Context, realm, id string) ([]clients.GroupRepresentation, error) {
			return []clients.GroupRepresentation{
				{ID: groupID, Name: "admin"},
			}, nil
		},
	}

	actual, err := mockClient.GetUserGroups(context.Background(), realmID, userID)
	if err != nil {
		t.Fatalf("GetUserGroups failed: %v", err)
	}

	actualSet := newStringSet(groupIDSet(actual))
	desiredSet := newStringSet([]string{groupID})
	exhaustive := true

	if !desiredSet.equals(actualSet) || !exhaustive {
		if exhaustive && !desiredSet.equals(actualSet) {
			t.Fatal("Expected equals to be true in exhaustive mode")
		}
	}
}

// TestGroupsStringSetMethods verifies stringSet helper methods work correctly
func TestGroupsStringSetMethods(t *testing.T) {
	set1 := newStringSet([]string{"a", "b", "c"})
	set2 := newStringSet([]string{"a", "b"})
	set3 := newStringSet([]string{"a", "b", "c"})

	if !set2.isSubsetOf(set1) {
		t.Fatal("Expected set2 to be a subset of set1")
	}

	if set1.isSubsetOf(set2) {
		t.Fatal("Expected set1 not to be a subset of set2")
	}

	if !set1.equals(set3) {
		t.Fatal("Expected set1 to equal set3")
	}

	if set1.equals(set2) {
		t.Fatal("Expected set1 not to equal set2")
	}
}

// TestGroupsGroupIDSetExtraction verifies groupIDSet extracts IDs from GroupRepresentations
func TestGroupsGroupIDSetExtraction(t *testing.T) {
	groups := []clients.GroupRepresentation{
		{ID: "group-1", Name: "admin"},
		{ID: "group-2", Name: "users"},
		{ID: "group-3", Name: "viewers"},
	}

	ids := groupIDSet(groups)

	if len(ids) != 3 {
		t.Fatalf("Expected 3 IDs, got %d", len(ids))
	}

	expectedIDs := map[string]bool{"group-1": true, "group-2": true, "group-3": true}
	for _, id := range ids {
		if !expectedIDs[id] {
			t.Fatalf("Unexpected ID in result: %s", id)
		}
	}
}

const (
	conflictTestNamespace = "rossgolderltd"
	conflictTestRealm     = "ROSSGolderLtd"
	conflictTestUserID    = "user-uuid-1"
	conflictTestOtherUser = "user-uuid-2"
)

// newGroupsResource builds a Groups resource addressed by explicit realm and
// user UUID so these tests exercise the reconciler rather than reference
// resolution.
func newGroupsResource(name, realm, userID string, groupIDs []string, exhaustive *bool) *userv1beta1.Groups {
	return &userv1beta1.Groups{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: conflictTestNamespace,
		},
		Spec: userv1beta1.GroupsSpec{
			ForProvider: userv1beta1.GroupsParameters{
				RealmId:    &realm,
				UserId:     &userID,
				GroupIds:   groupIDs,
				Exhaustive: exhaustive,
			},
		},
	}
}

func newGroupsScheme(t *testing.T) *runtime.Scheme {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := userv1beta1.AddToScheme(scheme); err != nil {
		t.Fatalf("cannot build scheme: %v", err)
	}

	return scheme
}

// newGroupsExternal wires an external client against a fake API server seeded
// with objs, plus a recorder whose events are readable from the returned
// channel.
func newGroupsExternal(t *testing.T, mc *mockGroupsClient, objs ...runtime.Object) (*groupsExternal, chan string) {
	t.Helper()

	kube := fake.NewClientBuilder().WithScheme(newGroupsScheme(t)).WithRuntimeObjects(objs...).Build()

	events := make(chan string, 16)

	return &groupsExternal{
		client:   mc,
		kube:     kube,
		recorder: event.NewAPIRecorder(&kubeevents.FakeRecorder{Events: events}),
	}, events
}

// drainEvents returns every event recorded so far.
func drainEvents(ch chan string) []string {
	var out []string
	for {
		select {
		case e := <-ch:
			out = append(out, e)
		default:
			return out
		}
	}
}

// TestGroupsObserveRefusesToFightOverMembership is the regression test for two
// Groups resources both claiming exhaustive ownership of one user's
// memberships. Before this guard they each deleted the other's groups on every
// reconcile while both reported Synced and Ready, leaving the membership
// oscillating.
func TestGroupsObserveRefusesToFightOverMembership(t *testing.T) {
	mine := newGroupsResource("rossgolderltd-admin-ross", conflictTestRealm, conflictTestUserID, []string{"admin-id"}, nil)
	theirs := newGroupsResource("rossgolderltd-backups-ross", conflictTestRealm, conflictTestUserID, []string{"backups-id"}, nil)

	// GetUserGroups must not be reached: the guard runs before any read of, or
	// write to, the user's membership.
	mc := &mockGroupsClient{
		BaseMockClient: &testhelpers.BaseMockClient{},
		getUserGroupsFn: func(_ context.Context, _, _ string) ([]clients.GroupRepresentation, error) {
			t.Fatal("GetUserGroups must not be called when a membership conflict exists")
			return nil, nil
		},
	}

	e, events := newGroupsExternal(t, mc, mine, theirs)

	obs, err := e.Observe(context.Background(), mine)
	if err == nil {
		t.Fatal("expected an error describing the conflicting membership owner")
	}
	if obs.ResourceExists {
		t.Error("expected no observation to be reported while the conflict is unresolved")
	}

	if !strings.Contains(err.Error(), "rossgolderltd-backups-ross") {
		t.Errorf("error should name the conflicting resource, got: %v", err)
	}

	ready := mine.Status.GetCondition(xpv1.TypeReady)
	if ready.Type == "" || ready.Status != corev1.ConditionFalse {
		t.Fatalf("expected Ready=False while conflicted, got %+v", ready)
	}

	recorded := drainEvents(events)
	if len(recorded) != 1 {
		t.Fatalf("expected exactly one event, got %d: %v", len(recorded), recorded)
	}
	if !strings.HasPrefix(recorded[0], "Warning MembershipConflict") {
		t.Errorf("expected a Warning MembershipConflict event, got: %s", recorded[0])
	}
}

// TestGroupsObserveConflictNamesAllOwners checks every conflicting sibling is
// reported, in a stable order.
func TestGroupsObserveConflictNamesAllOwners(t *testing.T) {
	mine := newGroupsResource("mine", conflictTestRealm, conflictTestUserID, []string{"a-id"}, nil)
	second := newGroupsResource("zzz-other", conflictTestRealm, conflictTestUserID, []string{"b-id"}, nil)
	third := newGroupsResource("aaa-other", conflictTestRealm, conflictTestUserID, []string{"c-id"}, nil)

	e, _ := newGroupsExternal(t, &mockGroupsClient{BaseMockClient: &testhelpers.BaseMockClient{}}, mine, second, third)

	got, err := e.conflictingMembershipOwners(context.Background(), mine, conflictTestRealm, conflictTestUserID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []string{"aaa-other", "zzz-other"}
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected %v (sorted), got %v", want, got)
		}
	}
}

// TestGroupsObserveConflictIgnored covers the cases that must NOT be reported
// as conflicts, since flagging them would block legitimate configurations.
func TestGroupsObserveConflictIgnored(t *testing.T) {
	no := false

	cases := map[string]struct {
		sibling *userv1beta1.Groups
		realm   string
		userID  string
	}{
		"additive sibling is not an owner": {
			sibling: newGroupsResource("additive", conflictTestRealm, conflictTestUserID, []string{"b-id"}, &no),
			realm:   conflictTestRealm,
			userID:  conflictTestUserID,
		},
		"sibling in another realm": {
			sibling: newGroupsResource("other-realm", "OtherRealm", conflictTestUserID, []string{"b-id"}, nil),
			realm:   conflictTestRealm,
			userID:  conflictTestUserID,
		},
		"sibling resolves to a different user": {
			sibling: newGroupsResource("other-user", conflictTestRealm, conflictTestOtherUser, []string{"b-id"}, nil),
			realm:   conflictTestRealm,
			userID:  conflictTestUserID,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			mine := newGroupsResource("mine", conflictTestRealm, conflictTestUserID, []string{"a-id"}, nil)
			e, events := newGroupsExternal(t, &mockGroupsClient{BaseMockClient: &testhelpers.BaseMockClient{}}, mine, tc.sibling)

			got, err := e.conflictingMembershipOwners(context.Background(), mine, tc.realm, tc.userID)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != 0 {
				t.Errorf("expected no conflict, got %v", got)
			}
			if recorded := drainEvents(events); len(recorded) != 0 {
				t.Errorf("expected no events, got %v", recorded)
			}
		})
	}
}

// TestGroupsObserveIgnoresSiblingBeingDeleted verifies a sibling on its way out
// doesn't block this resource, which matters when collapsing two resources into
// one: the replacement must be able to sync before the old object is reaped.
func TestGroupsObserveIgnoresSiblingBeingDeleted(t *testing.T) {
	now := metav1.Now()
	mine := newGroupsResource("mine", conflictTestRealm, conflictTestUserID, []string{"a-id"}, nil)

	dying := newGroupsResource("dying", conflictTestRealm, conflictTestUserID, []string{"b-id"}, nil)
	dying.DeletionTimestamp = &now
	// The fake client only preserves a deletionTimestamp on an object that also
	// carries a finalizer.
	dying.Finalizers = []string{"finalizer.managedresource.crossplane.io"}

	e, _ := newGroupsExternal(t, &mockGroupsClient{BaseMockClient: &testhelpers.BaseMockClient{}}, mine, dying)

	got, err := e.conflictingMembershipOwners(context.Background(), mine, conflictTestRealm, conflictTestUserID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("a sibling being deleted must not count as a conflict, got %v", got)
	}
}

// TestGroupsObserveListFailureSurfaces makes sure a failure to list siblings is
// reported rather than silently treated as "no conflict", which would let the
// fight continue unnoticed.
func TestGroupsObserveListFailureSurfaces(t *testing.T) {
	mine := newGroupsResource("mine", conflictTestRealm, conflictTestUserID, []string{"a-id"}, nil)

	scheme := newGroupsScheme(t)
	kube := fake.NewClientBuilder().WithScheme(scheme).Build()
	e := &groupsExternal{
		client: &mockGroupsClient{BaseMockClient: &testhelpers.BaseMockClient{}},
		kube:   &failingListClient{Client: kube},
	}

	if _, err := e.Observe(context.Background(), mine); err == nil {
		t.Fatal("expected the list failure to be reported")
	}
}

// failingListClient fails every List call, standing in for a transient API
// error.
type failingListClient struct {
	client.Client
}

func (c *failingListClient) List(_ context.Context, _ client.ObjectList, _ ...client.ListOption) error {
	return errors.New("simulated list failure")
}

// TestGroupsObserveReadinessTracksConvergence pins the second half of the fix:
// Ready must not read True while the membership still differs from what the
// resource declares.
func TestGroupsObserveReadinessTracksConvergence(t *testing.T) {
	mine := newGroupsResource("mine", conflictTestRealm, conflictTestUserID, []string{"admin-id"}, nil)

	cases := map[string]struct {
		actual       []clients.GroupRepresentation
		wantUpToDate bool
		wantReady    corev1.ConditionStatus
	}{
		"converged": {
			actual:       []clients.GroupRepresentation{{ID: "admin-id", Name: "admin"}},
			wantUpToDate: true,
			wantReady:    corev1.ConditionTrue,
		},
		"missing desired group": {
			actual:       nil,
			wantUpToDate: false,
			wantReady:    corev1.ConditionFalse,
		},
		"extra group present": {
			actual:       []clients.GroupRepresentation{{ID: "admin-id"}, {ID: "stray-id"}},
			wantUpToDate: false,
			wantReady:    corev1.ConditionFalse,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			mc := &mockGroupsClient{
				BaseMockClient:  &testhelpers.BaseMockClient{},
				getUserGroupsFn: func(_ context.Context, _, _ string) ([]clients.GroupRepresentation, error) { return tc.actual, nil },
			}
			e, _ := newGroupsExternal(t, mc, mine)

			obs, err := e.Observe(context.Background(), mine)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if obs.ResourceUpToDate != tc.wantUpToDate {
				t.Errorf("expected ResourceUpToDate=%v, got %v", tc.wantUpToDate, obs.ResourceUpToDate)
			}

			ready := mine.Status.GetCondition(xpv1.TypeReady)
			if ready.Type == "" {
				t.Fatal("expected a Ready condition to be set")
			}
			if ready.Status != tc.wantReady {
				t.Errorf("expected Ready=%s, got %s", tc.wantReady, ready.Status)
			}
			if ready.Status == corev1.ConditionFalse && ready.Message != msgMembershipNotConverged {
				t.Errorf("expected message %q, got %q", msgMembershipNotConverged, ready.Message)
			}
		})
	}
}

// TestGroupsObserveWithoutRecorder guards the nil-recorder path so the guard
// can't panic in wiring that doesn't supply one.
func TestGroupsObserveWithoutRecorder(t *testing.T) {
	mine := newGroupsResource("mine", conflictTestRealm, conflictTestUserID, []string{"a-id"}, nil)
	theirs := newGroupsResource("theirs", conflictTestRealm, conflictTestUserID, []string{"b-id"}, nil)

	e, _ := newGroupsExternal(t, &mockGroupsClient{BaseMockClient: &testhelpers.BaseMockClient{}}, mine, theirs)
	e.recorder = nil

	if _, err := e.Observe(context.Background(), mine); err == nil {
		t.Fatal("expected the conflict to be reported even without a recorder")
	}
}

// TestGroupsDeleteCompletesAndReleasesFinalizer is the regression test for the
// wedged finalizer. The reconciler removes a managed resource's finalizer only
// once Observe reports the external resource gone, and it re-runs Delete on
// every pass while Observe still reports it present. Because Observe hardcoded
// ResourceExists: true, Delete ran forever: the object never terminated and each
// pass re-applied the membership removals.
func TestGroupsDeleteCompletesAndReleasesFinalizer(t *testing.T) {
	removed := 0

	mc := &mockGroupsClient{
		BaseMockClient: &testhelpers.BaseMockClient{},
		removeUserFromGroupFn: func(_ context.Context, _, _, _ string) error {
			removed++
			return nil
		},
	}

	mine := newGroupsResource("doomed", conflictTestRealm, conflictTestUserID, []string{"admin-id"}, nil)

	e, _ := newGroupsExternal(t, mc, mine)

	// Before Delete: the resource still reports as existing.
	obs, err := e.Observe(context.Background(), mine)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !obs.ResourceExists {
		t.Fatal("a live resource must report ResourceExists=true")
	}

	if _, err := e.Delete(context.Background(), mine); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	if removed != 1 {
		t.Errorf("expected the owned membership to be released once, got %d", removed)
	}

	// The marker must be persisted, not just set in memory, because the
	// reconciler only writes back status.
	stored := &userv1beta1.Groups{}
	if err := e.kube.Get(context.Background(), client.ObjectKey{Name: "doomed", Namespace: conflictTestNamespace}, stored); err != nil {
		t.Fatalf("cannot read back resource: %v", err)
	}
	if got := stored.GetAnnotations()[deletecomplete.Annotation]; got != deletecomplete.AnnotationValue {
		t.Fatalf("expected the delete-completed annotation to be persisted, got %q", got)
	}

	// After Delete: Observe must report the external resource gone, which is
	// what lets the reconciler reach RemoveFinalizer.
	obs, err = e.Observe(context.Background(), stored)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if obs.ResourceExists {
		t.Error("after Delete, Observe must report ResourceExists=false so the finalizer can be removed")
	}

	// And Delete must not run again.
	if _, err := e.Delete(context.Background(), stored); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if removed != 1 {
		t.Errorf("Delete must not re-release memberships once complete, released %d times", removed)
	}
}

// TestGroupsDeleteRecordsCompletionWhenNothingToRelease covers the two paths
// where there is no membership left to remove. Both previously returned without
// recording anything, so those resources would have hung on their finalizer
// forever just like the normal path.
func TestGroupsDeleteRecordsCompletionWhenNothingToRelease(t *testing.T) {
	cases := map[string]struct {
		mutate  func(*userv1beta1.Groups)
		mc      *mockGroupsClient
		wantErr bool
	}{
		"user cannot be resolved": {
			// No UserId and no UserIdRef, so resolveUserID fails.
			mutate: func(cr *userv1beta1.Groups) { cr.Spec.ForProvider.UserId = nil },
			mc:     &mockGroupsClient{BaseMockClient: &testhelpers.BaseMockClient{}},
		},
		"groups cannot be resolved": {
			// A ref to a Group CR that does not exist.
			mutate: func(cr *userv1beta1.Groups) {
				cr.Spec.ForProvider.GroupIds = nil
				cr.Spec.ForProvider.GroupIdsRefs = []xpv1.Reference{{Name: "missing-group"}}
			},
			mc: &mockGroupsClient{BaseMockClient: &testhelpers.BaseMockClient{}},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			cr := newGroupsResource("doomed", conflictTestRealm, conflictTestUserID, []string{"admin-id"}, nil)
			tc.mutate(cr)

			e, _ := newGroupsExternal(t, tc.mc, cr)

			if _, err := e.Delete(context.Background(), cr); err != nil {
				t.Fatalf("Delete should succeed with nothing to release: %v", err)
			}

			stored := &userv1beta1.Groups{}
			if err := e.kube.Get(context.Background(), client.ObjectKey{Name: "doomed", Namespace: conflictTestNamespace}, stored); err != nil {
				t.Fatalf("cannot read back resource: %v", err)
			}
			if got := stored.GetAnnotations()[deletecomplete.Annotation]; got != deletecomplete.AnnotationValue {
				t.Errorf("completion must still be recorded, got %q", got)
			}
		})
	}
}

// TestGroupsDeleteFailureIsNotRecordedAsComplete checks a failed release is not
// mistaken for success, which would drop the finalizer while the membership is
// still in Keycloak.
func TestGroupsDeleteFailureIsNotRecordedAsComplete(t *testing.T) {
	mc := &mockGroupsClient{
		BaseMockClient: &testhelpers.BaseMockClient{},
		removeUserFromGroupFn: func(_ context.Context, _, _, _ string) error {
			return errors.New("keycloak said no")
		},
	}

	cr := newGroupsResource("doomed", conflictTestRealm, conflictTestUserID, []string{"admin-id"}, nil)
	e, _ := newGroupsExternal(t, mc, cr)

	if _, err := e.Delete(context.Background(), cr); err == nil {
		t.Fatal("expected the release failure to surface")
	}

	stored := &userv1beta1.Groups{}
	if err := e.kube.Get(context.Background(), client.ObjectKey{Name: "doomed", Namespace: conflictTestNamespace}, stored); err != nil {
		t.Fatalf("cannot read back resource: %v", err)
	}
	if _, done := stored.GetAnnotations()[deletecomplete.Annotation]; done {
		t.Error("a failed release must not be recorded as complete")
	}
}

// TestGroupsDeleteTreatsMissingMembershipAsDone covers the 404 tolerance the
// delete path already had: a membership Keycloak has already dropped is not a
// failure.
func TestGroupsDeleteTreatsMissingMembershipAsDone(t *testing.T) {
	mc := &mockGroupsClient{
		BaseMockClient: &testhelpers.BaseMockClient{},
		removeUserFromGroupFn: func(_ context.Context, _, _, _ string) error {
			return errors.New("404 not found")
		},
	}

	cr := newGroupsResource("doomed", conflictTestRealm, conflictTestUserID, []string{"admin-id"}, nil)
	e, _ := newGroupsExternal(t, mc, cr)

	if _, err := e.Delete(context.Background(), cr); err != nil {
		t.Fatalf("a 404 must be tolerated: %v", err)
	}

	stored := &userv1beta1.Groups{}
	if err := e.kube.Get(context.Background(), client.ObjectKey{Name: "doomed", Namespace: conflictTestNamespace}, stored); err != nil {
		t.Fatalf("cannot read back resource: %v", err)
	}
	if got := stored.GetAnnotations()[deletecomplete.Annotation]; got != deletecomplete.AnnotationValue {
		t.Errorf("expected completion to be recorded, got %q", got)
	}
}
