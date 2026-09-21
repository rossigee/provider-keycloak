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

	"github.com/rossigee/provider-keycloak/internal/controller/testhelpers"

	groupv1beta1 "github.com/rossigee/provider-keycloak/apis/group/v1beta1"
	userv1beta1 "github.com/rossigee/provider-keycloak/apis/user/v1beta1"
	"github.com/rossigee/provider-keycloak/internal/clients"
	"sigs.k8s.io/controller-runtime/pkg/client"
	testclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type mockGroupsClient struct {
	*testhelpers.BaseMockClient
	getUserFn       func(ctx context.Context, realm, username string) (*clients.User, error)
	searchGroupsFn  func(ctx context.Context, realm, name string) ([]*clients.Group, error)
	getUserGroupsFn func(ctx context.Context, realm, userID string) ([]*clients.Group, error)
	addUserToGroupFn func(ctx context.Context, realm, userID, groupID string) error
}

func (m *mockGroupsClient) GetUser(ctx context.Context, realm, username string) (*clients.User, error) {
	if m.getUserFn != nil {
		return m.getUserFn(ctx, realm, username)
	}
	return nil, nil
}

func (m *mockGroupsClient) SearchGroups(ctx context.Context, realm, name string) ([]*clients.Group, error) {
	if m.searchGroupsFn != nil {
		return m.searchGroupsFn(ctx, realm, name)
	}
	return nil, nil
}

func (m *mockGroupsClient) GetUserGroups(ctx context.Context, realm, userID string) ([]*clients.Group, error) {
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

// TestGroupsSyncAddsUserToGroup verifies that sync() actually calls AddUserToGroup
// This is a regression test for the bug where Observe returned success without
// verifying the user was actually added to the group in Keycloak
func TestGroupsSyncAddsUserToGroup(t *testing.T) {
	userID := "user-uuid-123"
	adminGroupID := "admin-group-uuid"
	realmID := "test-realm"

	addUserCalled := false

	mockKcClient := &mockGroupsClient{
		BaseMockClient: &testhelpers.BaseMockClient{},
		getUserFn: func(ctx context.Context, realm, username string) (*clients.User, error) {
			if username == "rossg" {
				return &clients.User{ID: userID}, nil
			}
			return nil, nil
		},
		searchGroupsFn: func(ctx context.Context, realm, name string) ([]*clients.Group, error) {
			if name == "admin" {
				return []*clients.Group{{ID: adminGroupID, Name: "admin"}}, nil
			}
			return nil, nil
		},
		getUserGroupsFn: func(ctx context.Context, realm, userID string) ([]*clients.Group, error) {
			// Return empty initially, then after AddUserToGroup is called, return the group
			if addUserCalled {
				return []*clients.Group{{ID: adminGroupID, Name: "admin"}}, nil
			}
			return []*clients.Group{}, nil
		},
		addUserToGroupFn: func(ctx context.Context, realm, userID, groupID string) error {
			addUserCalled = true
			return nil
		},
	}

	kubeClient := testclient.NewClientBuilder().Build()

	// Create User CR
	userCR := &userv1beta1.User{
		Spec: userv1beta1.UserSpec{
			ForProvider: userv1beta1.UserParameters{
				Username: "rossg",
				Enabled:  boolPtr(true),
			},
		},
	}
	kubeClient.Create(context.Background(), userCR)

	// Create Group CR
	groupCR := &groupv1beta1.Group{
		Spec: groupv1beta1.GroupSpec{
			ForProvider: groupv1beta1.GroupParameters{
				Name: "admin",
			},
		},
	}
	kubeClient.Create(context.Background(), groupCR)

	ext := &groupsExternal{
		client: mockKcClient,
		kube:   kubeClient,
	}

	// Create Groups CR that references the User and Group
	cr := &userv1beta1.Groups{
		Spec: userv1beta1.GroupsSpec{
			ForProvider: userv1beta1.GroupsParameters{
				RealmId: realmID,
				UserIdRef: &userv1beta1.UserReference{
					Name: userCR.Name,
				},
				GroupIdsRefs: []userv1beta1.GroupReference{
					{Name: groupCR.Name},
				},
				Exhaustive: boolPtr(false),
			},
		},
	}

	// Execute sync
	err := ext.sync(context.Background(), cr)

	// Verify
	if err != nil {
		t.Fatalf("sync() failed: %v", err)
	}

	if !addUserCalled {
		t.Fatal("AddUserToGroup was not called - user was never added to group")
	}

	// Verify Observe would now see the user in the group
	obs, err := ext.Observe(context.Background(), cr)
	if err != nil {
		t.Fatalf("Observe() failed: %v", err)
	}

	if !obs.ResourceUpToDate {
		t.Fatal("Observe returned ResourceUpToDate=false after sync added user to group")
	}
}
