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

package group

import (
	"context"
	"errors"
	"testing"

	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	groupv1alpha1 "github.com/rossigee/provider-keycloak/apis/group/v1alpha1"
	realmv1alpha1 "github.com/rossigee/provider-keycloak/apis/realm/v1alpha1"
	"github.com/rossigee/provider-keycloak/internal/clients"
)

type mockGroupClient struct {
	searchGroupsFn func(ctx context.Context, realm, name string) ([]clients.GroupRepresentation, error)
	createGroupFn  func(ctx context.Context, realm string, g *clients.GroupRepresentation) (*clients.GroupRepresentation, error)
	updateGroupFn  func(ctx context.Context, realm string, g *clients.GroupRepresentation) error
	deleteGroupFn  func(ctx context.Context, realm, groupID string) error
}

func (m *mockGroupClient) SearchGroups(ctx context.Context, realm, name string) ([]clients.GroupRepresentation, error) {
	return m.searchGroupsFn(ctx, realm, name)
}
func (m *mockGroupClient) CreateGroup(ctx context.Context, realm string, g *clients.GroupRepresentation) (*clients.GroupRepresentation, error) {
	return m.createGroupFn(ctx, realm, g)
}
func (m *mockGroupClient) UpdateGroup(ctx context.Context, realm string, g *clients.GroupRepresentation) error {
	return m.updateGroupFn(ctx, realm, g)
}
func (m *mockGroupClient) DeleteGroup(ctx context.Context, realm, groupID string) error {
	return m.deleteGroupFn(ctx, realm, groupID)
}

func (m *mockGroupClient) GetRealm(ctx context.Context, realm string) (*clients.Realm, error) {
	return nil, nil
}
func (m *mockGroupClient) CreateRealm(ctx context.Context, r *clients.Realm) (*clients.Realm, error) {
	return nil, nil
}
func (m *mockGroupClient) UpdateRealm(ctx context.Context, r *clients.Realm) error { return nil }
func (m *mockGroupClient) DeleteRealm(ctx context.Context, realm string) error     { return nil }
func (m *mockGroupClient) GetClient(ctx context.Context, realm, clientID string) (*clients.ClientRepresentation, error) {
	return nil, nil
}
func (m *mockGroupClient) CreateClient(ctx context.Context, realm string, c *clients.ClientRepresentation) (*clients.ClientRepresentation, error) {
	return nil, nil
}
func (m *mockGroupClient) UpdateClient(ctx context.Context, realm string, c *clients.ClientRepresentation) error {
	return nil
}
func (m *mockGroupClient) DeleteClient(ctx context.Context, realm, clientID string) error { return nil }
func (m *mockGroupClient) ListClients(ctx context.Context, realm string) ([]clients.ClientRepresentation, error) {
	return nil, nil
}
func (m *mockGroupClient) GetUser(ctx context.Context, realm, username string) (*clients.UserRepresentation, error) {
	return nil, nil
}
func (m *mockGroupClient) CreateUser(ctx context.Context, realm string, u *clients.UserRepresentation) (*clients.UserRepresentation, error) {
	return nil, nil
}
func (m *mockGroupClient) UpdateUser(ctx context.Context, realm string, u *clients.UserRepresentation) error {
	return nil
}
func (m *mockGroupClient) DeleteUser(ctx context.Context, realm, userID string) error { return nil }
func (m *mockGroupClient) ListUsers(ctx context.Context, realm string) ([]clients.UserRepresentation, error) {
	return nil, nil
}
func (m *mockGroupClient) GetGroup(ctx context.Context, realm, groupID string) (*clients.GroupRepresentation, error) {
	return nil, nil
}
func (m *mockGroupClient) ListGroups(ctx context.Context, realm string) ([]clients.GroupRepresentation, error) {
	return nil, nil
}
func (m *mockGroupClient) GetClientSecret(ctx context.Context, realm, clientID string) (string, error) {
	return "", nil
}
func (m *mockGroupClient) GetUserGroups(ctx context.Context, realm, userID string) ([]clients.GroupRepresentation, error) {
	return nil, nil
}
func (m *mockGroupClient) AddUserToGroup(ctx context.Context, realm, userID, groupID string) error {
	return nil
}
func (m *mockGroupClient) RemoveUserFromGroup(ctx context.Context, realm, userID, groupID string) error {
	return nil
}
func (m *mockGroupClient) SearchUsers(ctx context.Context, realm, username string) ([]clients.UserRepresentation, error) {
	return nil, nil
}
func (m *mockGroupClient) GetRealmRole(ctx context.Context, realm, roleName string) (*clients.RoleRepresentation, error) {
	return nil, nil
}
func (m *mockGroupClient) CreateRealmRole(ctx context.Context, realm string, r *clients.RoleRepresentation) error {
	return nil
}
func (m *mockGroupClient) UpdateRealmRole(ctx context.Context, realm, roleName string, r *clients.RoleRepresentation) error {
	return nil
}
func (m *mockGroupClient) DeleteRealmRole(ctx context.Context, realm, roleName string) error {
	return nil
}
func (m *mockGroupClient) GetClientProtocolMapper(ctx context.Context, realm, clientID, mapperID string) (*clients.ProtocolMapperRepresentation, error) {
	return nil, nil
}
func (m *mockGroupClient) CreateClientProtocolMapper(ctx context.Context, realm, clientID string, p *clients.ProtocolMapperRepresentation) (string, error) {
	return "", nil
}
func (m *mockGroupClient) UpdateClientProtocolMapper(ctx context.Context, realm, clientID string, p *clients.ProtocolMapperRepresentation) error {
	return nil
}
func (m *mockGroupClient) DeleteClientProtocolMapper(ctx context.Context, realm, clientID, mapperID string) error {
	return nil
}
func (m *mockGroupClient) ListClientProtocolMappers(ctx context.Context, realm, clientID string) ([]clients.ProtocolMapperRepresentation, error) {
	return nil, nil
}

func newGroupCR(realmId, name string) *groupv1alpha1.Group {
	cr := &groupv1alpha1.Group{
		ObjectMeta: metav1.ObjectMeta{Name: "test-group", Namespace: "default"},
		Spec: groupv1alpha1.GroupSpec{
			ForProvider: groupv1alpha1.GroupParameters{
				Name: name,
			},
		},
	}
	if realmId != "" {
		cr.Spec.ForProvider.RealmId = &realmId
	}
	return cr
}

type wrongGroupMG = realmv1alpha1.Realm

func TestGroupObserve(t *testing.T) {
	tests := []struct {
		name       string
		mg         resource.Managed
		searchFn   func(ctx context.Context, realm, name string) ([]clients.GroupRepresentation, error)
		wantExists bool
		wantErrStr string
	}{
		{
			name:       "wrong managed type",
			mg:         &wrongGroupMG{},
			wantErrStr: errNotGroup,
		},
		{
			name:       "empty realmId",
			mg:         newGroupCR("", "testgroup"),
			wantErrStr: "realmId is required",
		},
		{
			name: "group not found",
			mg:   newGroupCR("testrealm", "testgroup"),
			searchFn: func(_ context.Context, _, _ string) ([]clients.GroupRepresentation, error) {
				return nil, nil
			},
			wantExists: false,
		},
		{
			name: "group found",
			mg:   newGroupCR("testrealm", "testgroup"),
			searchFn: func(_ context.Context, _, _ string) ([]clients.GroupRepresentation, error) {
				return []clients.GroupRepresentation{{ID: "group-id", Name: "testgroup"}}, nil
			},
			wantExists: true,
		},
		{
			name: "search error",
			mg:   newGroupCR("testrealm", "testgroup"),
			searchFn: func(_ context.Context, _, _ string) ([]clients.GroupRepresentation, error) {
				return nil, errors.New("search failed")
			},
			wantErrStr: "search failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &external{client: &mockGroupClient{searchGroupsFn: tt.searchFn}}
			obs, err := e.Observe(context.Background(), tt.mg)
			if tt.wantErrStr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErrStr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if obs.ResourceExists != tt.wantExists {
				t.Errorf("ResourceExists = %v, want %v", obs.ResourceExists, tt.wantExists)
			}
		})
	}
}

func TestGroupCreate(t *testing.T) {
	tests := []struct {
		name       string
		mg         resource.Managed
		createFn   func(ctx context.Context, realm string, g *clients.GroupRepresentation) (*clients.GroupRepresentation, error)
		wantErrStr string
	}{
		{
			name:       "wrong managed type",
			mg:         &wrongGroupMG{},
			wantErrStr: errNotGroup,
		},
		{
			name:       "empty realmId",
			mg:         newGroupCR("", "testgroup"),
			wantErrStr: "realmId is required",
		},
		{
			name: "create error",
			mg:   newGroupCR("testrealm", "testgroup"),
			createFn: func(_ context.Context, _ string, _ *clients.GroupRepresentation) (*clients.GroupRepresentation, error) {
				return nil, errors.New("conflict")
			},
			wantErrStr: "conflict",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &external{client: &mockGroupClient{createGroupFn: tt.createFn}}
			_, err := e.Create(context.Background(), tt.mg)
			if tt.wantErrStr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErrStr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
