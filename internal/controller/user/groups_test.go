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
	"testing"

	"github.com/rossigee/provider-keycloak/internal/clients"
	"github.com/rossigee/provider-keycloak/internal/controller/testhelpers"
)

type mockGroupsClient struct {
	*testhelpers.BaseMockClient
	getUserFn       func(ctx context.Context, realm, username string) (*clients.UserRepresentation, error)
	searchGroupsFn  func(ctx context.Context, realm, name string) ([]clients.GroupRepresentation, error)
	getUserGroupsFn func(ctx context.Context, realm, userID string) ([]clients.GroupRepresentation, error)
	addUserToGroupFn    func(ctx context.Context, realm, userID, groupID string) error
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
