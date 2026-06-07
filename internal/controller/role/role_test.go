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

package role

import (
	"context"
	"errors"
	"testing"

	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	realmv1alpha1 "github.com/rossigee/provider-keycloak/apis/realm/v1alpha1"
	rolev1alpha1 "github.com/rossigee/provider-keycloak/apis/role/v1alpha1"
	"github.com/rossigee/provider-keycloak/internal/clients"
)

type mockRoleClient struct {
	getRealmRoleFn    func(ctx context.Context, realm, name string) (*clients.RoleRepresentation, error)
	createRealmRoleFn func(ctx context.Context, realm string, r *clients.RoleRepresentation) error
	updateRealmRoleFn func(ctx context.Context, realm, name string, r *clients.RoleRepresentation) error
	deleteRealmRoleFn func(ctx context.Context, realm, name string) error
}

func (m *mockRoleClient) GetRealmRole(ctx context.Context, realm, name string) (*clients.RoleRepresentation, error) {
	return m.getRealmRoleFn(ctx, realm, name)
}
func (m *mockRoleClient) CreateRealmRole(ctx context.Context, realm string, r *clients.RoleRepresentation) error {
	return m.createRealmRoleFn(ctx, realm, r)
}
func (m *mockRoleClient) UpdateRealmRole(ctx context.Context, realm, name string, r *clients.RoleRepresentation) error {
	return m.updateRealmRoleFn(ctx, realm, name, r)
}
func (m *mockRoleClient) DeleteRealmRole(ctx context.Context, realm, name string) error {
	return m.deleteRealmRoleFn(ctx, realm, name)
}

func (m *mockRoleClient) GetRealm(ctx context.Context, realm string) (*clients.Realm, error) {
	return nil, nil
}
func (m *mockRoleClient) CreateRealm(ctx context.Context, r *clients.Realm) (*clients.Realm, error) {
	return nil, nil
}
func (m *mockRoleClient) UpdateRealm(ctx context.Context, r *clients.Realm) error { return nil }
func (m *mockRoleClient) DeleteRealm(ctx context.Context, realm string) error     { return nil }
func (m *mockRoleClient) GetClient(ctx context.Context, realm, clientID string) (*clients.ClientRepresentation, error) {
	return nil, nil
}
func (m *mockRoleClient) CreateClient(ctx context.Context, realm string, c *clients.ClientRepresentation) (*clients.ClientRepresentation, error) {
	return nil, nil
}
func (m *mockRoleClient) UpdateClient(ctx context.Context, realm string, c *clients.ClientRepresentation) error {
	return nil
}
func (m *mockRoleClient) DeleteClient(ctx context.Context, realm, clientID string) error { return nil }
func (m *mockRoleClient) ListClients(ctx context.Context, realm string) ([]clients.ClientRepresentation, error) {
	return nil, nil
}
func (m *mockRoleClient) GetUser(ctx context.Context, realm, username string) (*clients.UserRepresentation, error) {
	return nil, nil
}
func (m *mockRoleClient) CreateUser(ctx context.Context, realm string, u *clients.UserRepresentation) (*clients.UserRepresentation, error) {
	return nil, nil
}
func (m *mockRoleClient) UpdateUser(ctx context.Context, realm string, u *clients.UserRepresentation) error {
	return nil
}
func (m *mockRoleClient) DeleteUser(ctx context.Context, realm, userID string) error { return nil }
func (m *mockRoleClient) ListUsers(ctx context.Context, realm string) ([]clients.UserRepresentation, error) {
	return nil, nil
}
func (m *mockRoleClient) GetGroup(ctx context.Context, realm, groupID string) (*clients.GroupRepresentation, error) {
	return nil, nil
}
func (m *mockRoleClient) CreateGroup(ctx context.Context, realm string, g *clients.GroupRepresentation) (*clients.GroupRepresentation, error) {
	return nil, nil
}
func (m *mockRoleClient) UpdateGroup(ctx context.Context, realm string, g *clients.GroupRepresentation) error {
	return nil
}
func (m *mockRoleClient) DeleteGroup(ctx context.Context, realm, groupID string) error { return nil }
func (m *mockRoleClient) ListGroups(ctx context.Context, realm string) ([]clients.GroupRepresentation, error) {
	return nil, nil
}
func (m *mockRoleClient) SearchGroups(ctx context.Context, realm, name string) ([]clients.GroupRepresentation, error) {
	return nil, nil
}
func (m *mockRoleClient) GetClientSecret(ctx context.Context, realm, clientID string) (string, error) {
	return "", nil
}
func (m *mockRoleClient) GetUserGroups(ctx context.Context, realm, userID string) ([]clients.GroupRepresentation, error) {
	return nil, nil
}
func (m *mockRoleClient) AddUserToGroup(ctx context.Context, realm, userID, groupID string) error {
	return nil
}
func (m *mockRoleClient) RemoveUserFromGroup(ctx context.Context, realm, userID, groupID string) error {
	return nil
}
func (m *mockRoleClient) SearchUsers(ctx context.Context, realm, username string) ([]clients.UserRepresentation, error) {
	return nil, nil
}
func (m *mockRoleClient) GetClientProtocolMapper(ctx context.Context, realm, clientID, mapperID string) (*clients.ProtocolMapperRepresentation, error) {
	return nil, nil
}
func (m *mockRoleClient) CreateClientProtocolMapper(ctx context.Context, realm, clientID string, p *clients.ProtocolMapperRepresentation) (string, error) {
	return "", nil
}
func (m *mockRoleClient) UpdateClientProtocolMapper(ctx context.Context, realm, clientID string, p *clients.ProtocolMapperRepresentation) error {
	return nil
}
func (m *mockRoleClient) DeleteClientProtocolMapper(ctx context.Context, realm, clientID, mapperID string) error {
	return nil
}
func (m *mockRoleClient) ListClientProtocolMappers(ctx context.Context, realm, clientID string) ([]clients.ProtocolMapperRepresentation, error) {
	return nil, nil
}
func (m *mockRoleClient) GetUserFederationProvider(_ context.Context, _, _ string) (*clients.UserFederationProviderRepresentation, error) { return nil, nil }
func (m *mockRoleClient) CreateUserFederationProvider(_ context.Context, _ string, _ *clients.UserFederationProviderRepresentation) (string, error) { return "", nil }
func (m *mockRoleClient) UpdateUserFederationProvider(_ context.Context, _, _ string, _ *clients.UserFederationProviderRepresentation) error { return nil }
func (m *mockRoleClient) DeleteUserFederationProvider(_ context.Context, _, _ string) error { return nil }
func (m *mockRoleClient) ListUserFederationProviders(_ context.Context, _ string) ([]clients.UserFederationProviderRepresentation, error) { return nil, nil }
func (m *mockRoleClient) GetRealmEventsConfig(_ context.Context, _ string) (*clients.RealmEventsConfigRepresentation, error) { return nil, nil }
func (m *mockRoleClient) UpdateRealmEventsConfig(_ context.Context, _ string, _ *clients.RealmEventsConfigRepresentation) error { return nil }
func (m *mockRoleClient) ImportRealm(_ context.Context, _ string, _ bool) error { return nil }
func (m *mockRoleClient) GetAuthzResource(_ context.Context, _, _, _ string) (*clients.AuthzResourceRepresentation, error) { return nil, nil }
func (m *mockRoleClient) CreateAuthzResource(_ context.Context, _, _ string, _ *clients.AuthzResourceRepresentation) (string, error) { return "", nil }
func (m *mockRoleClient) UpdateAuthzResource(_ context.Context, _, _, _ string, _ *clients.AuthzResourceRepresentation) error { return nil }
func (m *mockRoleClient) DeleteAuthzResource(_ context.Context, _, _, _ string) error { return nil }
func (m *mockRoleClient) ListAuthzResources(_ context.Context, _, _ string) ([]clients.AuthzResourceRepresentation, error) { return nil, nil }
func (m *mockRoleClient) GetClientCertificate(_ context.Context, _, _, _ string) (*clients.ClientCertificateRepresentation, error) { return nil, nil }
func (m *mockRoleClient) GenerateClientCertificate(_ context.Context, _, _ string, _ string) (*clients.ClientCertificateRepresentation, error) { return nil, nil }
func (m *mockRoleClient) ListClientCertificates(_ context.Context, _, _ string) ([]clients.ClientCertificateRepresentation, error) { return nil, nil }

func newRoleCR(realmId, name string) *rolev1alpha1.Role {
	cr := &rolev1alpha1.Role{
		ObjectMeta: metav1.ObjectMeta{Name: "test-role", Namespace: "default"},
		Spec: rolev1alpha1.RoleSpec{
			ForProvider: rolev1alpha1.RoleParameters{
				Name: name,
			},
		},
	}
	if realmId != "" {
		cr.Spec.ForProvider.RealmId = &realmId
	}
	return cr
}

type wrongRoleMG = realmv1alpha1.Realm

func TestRoleObserve(t *testing.T) {
	tests := []struct {
		name       string
		mg         resource.Managed
		getFn      func(ctx context.Context, realm, name string) (*clients.RoleRepresentation, error)
		wantExists bool
		wantErrStr string
	}{
		{
			name:       "wrong managed type",
			mg:         &wrongRoleMG{},
			wantErrStr: errNotRole,
		},
		{
			name:       "empty realmId",
			mg:         newRoleCR("", "testrole"),
			wantErrStr: "realmId is required",
		},
		{
			name: "role not found",
			mg:   newRoleCR("testrealm", "testrole"),
			getFn: func(_ context.Context, _, _ string) (*clients.RoleRepresentation, error) {
				return nil, nil
			},
			wantExists: false,
		},
		{
			name: "role found",
			mg:   newRoleCR("testrealm", "testrole"),
			getFn: func(_ context.Context, _, _ string) (*clients.RoleRepresentation, error) {
				return &clients.RoleRepresentation{Name: "testrole"}, nil
			},
			wantExists: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &external{client: &mockRoleClient{getRealmRoleFn: tt.getFn}}
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

func TestRoleCreate(t *testing.T) {
	tests := []struct {
		name       string
		mg         resource.Managed
		createFn   func(ctx context.Context, realm string, r *clients.RoleRepresentation) error
		wantErrStr string
	}{
		{
			name:       "wrong managed type",
			mg:         &wrongRoleMG{},
			wantErrStr: errNotRole,
		},
		{
			name:       "empty realmId",
			mg:         newRoleCR("", "testrole"),
			wantErrStr: "realmId is required",
		},
		{
			name: "create error",
			mg:   newRoleCR("testrealm", "testrole"),
			createFn: func(_ context.Context, _ string, _ *clients.RoleRepresentation) error {
				return errors.New("conflict")
			},
			wantErrStr: "conflict",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &external{client: &mockRoleClient{createRealmRoleFn: tt.createFn}}
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
