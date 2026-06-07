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
	"github.com/rossigee/provider-keycloak/internal/controller/testhelpers"
	"context"
	"errors"
	"testing"

	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	realmv1alpha1 "github.com/rossigee/provider-keycloak/apis/realm/v1alpha1"
	userv1alpha1 "github.com/rossigee/provider-keycloak/apis/user/v1alpha1"
	"github.com/rossigee/provider-keycloak/internal/clients"
)

type mockUserClient struct {
	*testhelpers.BaseMockClient
	getUserFn    func(ctx context.Context, realm, username string) (*clients.UserRepresentation, error)
	createUserFn func(ctx context.Context, realm string, u *clients.UserRepresentation) (*clients.UserRepresentation, error)
	updateUserFn func(ctx context.Context, realm string, u *clients.UserRepresentation) error
	deleteUserFn func(ctx context.Context, realm, userID string) error
}

func (m *mockUserClient) GetUser(ctx context.Context, realm, username string) (*clients.UserRepresentation, error) {
	return m.getUserFn(ctx, realm, username)
}
func (m *mockUserClient) CreateUser(ctx context.Context, realm string, u *clients.UserRepresentation) (*clients.UserRepresentation, error) {
	return m.createUserFn(ctx, realm, u)
}
func (m *mockUserClient) UpdateUser(ctx context.Context, realm string, u *clients.UserRepresentation) error {
	return m.updateUserFn(ctx, realm, u)
}
func (m *mockUserClient) DeleteUser(ctx context.Context, realm, userID string) error {
	return m.deleteUserFn(ctx, realm, userID)
}

func (m *mockUserClient) GetRealm(ctx context.Context, realm string) (*clients.Realm, error) {
	return nil, nil
}
func (m *mockUserClient) CreateRealm(ctx context.Context, r *clients.Realm) (*clients.Realm, error) {
	return nil, nil
}
func (m *mockUserClient) UpdateRealm(ctx context.Context, r *clients.Realm) error { return nil }
func (m *mockUserClient) DeleteRealm(ctx context.Context, realm string) error     { return nil }
func (m *mockUserClient) GetClient(ctx context.Context, realm, clientID string) (*clients.ClientRepresentation, error) {
	return nil, nil
}
func (m *mockUserClient) CreateClient(ctx context.Context, realm string, c *clients.ClientRepresentation) (*clients.ClientRepresentation, error) {
	return nil, nil
}
func (m *mockUserClient) UpdateClient(ctx context.Context, realm string, c *clients.ClientRepresentation) error {
	return nil
}
func (m *mockUserClient) DeleteClient(ctx context.Context, realm, clientID string) error { return nil }
func (m *mockUserClient) ListClients(ctx context.Context, realm string) ([]clients.ClientRepresentation, error) {
	return nil, nil
}
func (m *mockUserClient) ListUsers(ctx context.Context, realm string) ([]clients.UserRepresentation, error) {
	return nil, nil
}
func (m *mockUserClient) GetGroup(ctx context.Context, realm, groupID string) (*clients.GroupRepresentation, error) {
	return nil, nil
}
func (m *mockUserClient) CreateGroup(ctx context.Context, realm string, g *clients.GroupRepresentation) (*clients.GroupRepresentation, error) {
	return nil, nil
}
func (m *mockUserClient) UpdateGroup(ctx context.Context, realm string, g *clients.GroupRepresentation) error {
	return nil
}
func (m *mockUserClient) DeleteGroup(ctx context.Context, realm, groupID string) error { return nil }
func (m *mockUserClient) ListGroups(ctx context.Context, realm string) ([]clients.GroupRepresentation, error) {
	return nil, nil
}
func (m *mockUserClient) SearchGroups(ctx context.Context, realm, name string) ([]clients.GroupRepresentation, error) {
	return nil, nil
}
func (m *mockUserClient) GetClientSecret(ctx context.Context, realm, clientID string) (string, error) {
	return "", nil
}
func (m *mockUserClient) GetUserGroups(ctx context.Context, realm, userID string) ([]clients.GroupRepresentation, error) {
	return nil, nil
}
func (m *mockUserClient) AddUserToGroup(ctx context.Context, realm, userID, groupID string) error {
	return nil
}
func (m *mockUserClient) RemoveUserFromGroup(ctx context.Context, realm, userID, groupID string) error {
	return nil
}
func (m *mockUserClient) SearchUsers(ctx context.Context, realm, username string) ([]clients.UserRepresentation, error) {
	return nil, nil
}
func (m *mockUserClient) GetRealmRole(ctx context.Context, realm, roleName string) (*clients.RoleRepresentation, error) {
	return nil, nil
}
func (m *mockUserClient) CreateRealmRole(ctx context.Context, realm string, r *clients.RoleRepresentation) error {
	return nil
}
func (m *mockUserClient) UpdateRealmRole(ctx context.Context, realm, roleName string, r *clients.RoleRepresentation) error {
	return nil
}
func (m *mockUserClient) DeleteRealmRole(ctx context.Context, realm, roleName string) error {
	return nil
}
func (m *mockUserClient) GetClientProtocolMapper(ctx context.Context, realm, clientID, mapperID string) (*clients.ProtocolMapperRepresentation, error) {
	return nil, nil
}
func (m *mockUserClient) CreateClientProtocolMapper(ctx context.Context, realm, clientID string, p *clients.ProtocolMapperRepresentation) (string, error) {
	return "", nil
}
func (m *mockUserClient) UpdateClientProtocolMapper(ctx context.Context, realm, clientID string, p *clients.ProtocolMapperRepresentation) error {
	return nil
}
func (m *mockUserClient) DeleteClientProtocolMapper(ctx context.Context, realm, clientID, mapperID string) error {
	return nil
}
func (m *mockUserClient) ListClientProtocolMappers(ctx context.Context, realm, clientID string) ([]clients.ProtocolMapperRepresentation, error) {
	return nil, nil
}
func (m *mockUserClient) GetUserFederationProvider(_ context.Context, _, _ string) (*clients.UserFederationProviderRepresentation, error) { return nil, nil }
func (m *mockUserClient) CreateUserFederationProvider(_ context.Context, _ string, _ *clients.UserFederationProviderRepresentation) (string, error) { return "", nil }
func (m *mockUserClient) UpdateUserFederationProvider(_ context.Context, _, _ string, _ *clients.UserFederationProviderRepresentation) error { return nil }
func (m *mockUserClient) DeleteUserFederationProvider(_ context.Context, _, _ string) error { return nil }
func (m *mockUserClient) ListUserFederationProviders(_ context.Context, _ string) ([]clients.UserFederationProviderRepresentation, error) { return nil, nil }
func (m *mockUserClient) GetRealmEventsConfig(_ context.Context, _ string) (*clients.RealmEventsConfigRepresentation, error) { return nil, nil }
func (m *mockUserClient) UpdateRealmEventsConfig(_ context.Context, _ string, _ *clients.RealmEventsConfigRepresentation) error { return nil }
func (m *mockUserClient) ImportRealm(_ context.Context, _ string, _ bool) error { return nil }
func (m *mockUserClient) GetAuthzResource(_ context.Context, _, _, _ string) (*clients.AuthzResourceRepresentation, error) { return nil, nil }
func (m *mockUserClient) CreateAuthzResource(_ context.Context, _, _ string, _ *clients.AuthzResourceRepresentation) (string, error) { return "", nil }
func (m *mockUserClient) UpdateAuthzResource(_ context.Context, _, _, _ string, _ *clients.AuthzResourceRepresentation) error { return nil }
func (m *mockUserClient) DeleteAuthzResource(_ context.Context, _, _, _ string) error { return nil }
func (m *mockUserClient) ListAuthzResources(_ context.Context, _, _ string) ([]clients.AuthzResourceRepresentation, error) { return nil, nil }
func (m *mockUserClient) GetClientCertificate(_ context.Context, _, _, _ string) (*clients.ClientCertificateRepresentation, error) { return nil, nil }
func (m *mockUserClient) GenerateClientCertificate(_ context.Context, _, _ string, _ string) (*clients.ClientCertificateRepresentation, error) { return nil, nil }
func (m *mockUserClient) ListClientCertificates(_ context.Context, _, _ string) ([]clients.ClientCertificateRepresentation, error) { return nil, nil }

func newUserCR(realmId, username string) *userv1alpha1.User {
	cr := &userv1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{Name: "test-user", Namespace: "default"},
		Spec: userv1alpha1.UserSpec{
			ForProvider: userv1alpha1.UserParameters{
				Username: username,
			},
		},
	}
	if realmId != "" {
		cr.Spec.ForProvider.RealmId = &realmId
	}
	return cr
}

type wrongUserMG = realmv1alpha1.Realm

func TestUserObserve(t *testing.T) {
	tests := []struct {
		name       string
		mg         resource.Managed
		getUserFn  func(ctx context.Context, realm, username string) (*clients.UserRepresentation, error)
		wantExists bool
		wantErrStr string
	}{
		{
			name:       "wrong managed type",
			mg:         &wrongUserMG{},
			wantErrStr: errNotUser,
		},
		{
			name:       "empty realmId",
			mg:         newUserCR("", "testuser"),
			wantErrStr: "realmId is required",
		},
		{
			name: "user not found",
			mg:   newUserCR("testrealm", "testuser"),
			getUserFn: func(_ context.Context, _, _ string) (*clients.UserRepresentation, error) {
				return nil, nil
			},
			wantExists: false,
		},
		{
			name: "user found",
			mg:   newUserCR("testrealm", "testuser"),
			getUserFn: func(_ context.Context, _, _ string) (*clients.UserRepresentation, error) {
				return &clients.UserRepresentation{ID: "user-id", Username: "testuser"}, nil
			},
			wantExists: true,
		},
		{
			name: "get user error with 404",
			mg:   newUserCR("testrealm", "testuser"),
			getUserFn: func(_ context.Context, _, _ string) (*clients.UserRepresentation, error) {
				return nil, errors.New("404 not found")
			},
			wantExists: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &external{client: &mockUserClient{getUserFn: tt.getUserFn}}
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

func TestUserCreate(t *testing.T) {
	tests := []struct {
		name       string
		mg         resource.Managed
		createFn   func(ctx context.Context, realm string, u *clients.UserRepresentation) (*clients.UserRepresentation, error)
		wantErrStr string
	}{
		{
			name:       "wrong managed type",
			mg:         &wrongUserMG{},
			wantErrStr: errNotUser,
		},
		{
			name:       "empty realmId",
			mg:         newUserCR("", "testuser"),
			wantErrStr: "realmId is required",
		},
		{
			name: "create error",
			mg:   newUserCR("testrealm", "testuser"),
			createFn: func(_ context.Context, _ string, _ *clients.UserRepresentation) (*clients.UserRepresentation, error) {
				return nil, errors.New("conflict")
			},
			wantErrStr: "conflict",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &external{client: &mockUserClient{createUserFn: tt.createFn}}
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
