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

package realm

import (
	"context"
	"errors"
	"testing"

	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	realmv1alpha1 "github.com/rossigee/provider-keycloak/apis/realm/v1alpha1"
	userv1alpha1 "github.com/rossigee/provider-keycloak/apis/user/v1alpha1"
	"github.com/rossigee/provider-keycloak/internal/clients"
)

type mockRealmClient struct {
	getRealmFn    func(ctx context.Context, realm string) (*clients.Realm, error)
	createRealmFn func(ctx context.Context, r *clients.Realm) (*clients.Realm, error)
	updateRealmFn func(ctx context.Context, r *clients.Realm) error
	deleteRealmFn func(ctx context.Context, realm string) error
}

func (m *mockRealmClient) GetRealm(ctx context.Context, realm string) (*clients.Realm, error) {
	return m.getRealmFn(ctx, realm)
}
func (m *mockRealmClient) CreateRealm(ctx context.Context, r *clients.Realm) (*clients.Realm, error) {
	return m.createRealmFn(ctx, r)
}
func (m *mockRealmClient) UpdateRealm(ctx context.Context, r *clients.Realm) error {
	return m.updateRealmFn(ctx, r)
}
func (m *mockRealmClient) DeleteRealm(ctx context.Context, realm string) error {
	return m.deleteRealmFn(ctx, realm)
}

func (m *mockRealmClient) GetClient(ctx context.Context, realm, clientID string) (*clients.ClientRepresentation, error) {
	return nil, nil
}
func (m *mockRealmClient) CreateClient(ctx context.Context, realm string, c *clients.ClientRepresentation) (*clients.ClientRepresentation, error) {
	return nil, nil
}
func (m *mockRealmClient) UpdateClient(ctx context.Context, realm string, c *clients.ClientRepresentation) error {
	return nil
}
func (m *mockRealmClient) DeleteClient(ctx context.Context, realm, clientID string) error { return nil }
func (m *mockRealmClient) ListClients(ctx context.Context, realm string) ([]clients.ClientRepresentation, error) {
	return nil, nil
}
func (m *mockRealmClient) GetUser(ctx context.Context, realm, username string) (*clients.UserRepresentation, error) {
	return nil, nil
}
func (m *mockRealmClient) CreateUser(ctx context.Context, realm string, u *clients.UserRepresentation) (*clients.UserRepresentation, error) {
	return nil, nil
}
func (m *mockRealmClient) UpdateUser(ctx context.Context, realm string, u *clients.UserRepresentation) error {
	return nil
}
func (m *mockRealmClient) DeleteUser(ctx context.Context, realm, userID string) error { return nil }
func (m *mockRealmClient) ListUsers(ctx context.Context, realm string) ([]clients.UserRepresentation, error) {
	return nil, nil
}
func (m *mockRealmClient) GetGroup(ctx context.Context, realm, groupID string) (*clients.GroupRepresentation, error) {
	return nil, nil
}
func (m *mockRealmClient) CreateGroup(ctx context.Context, realm string, g *clients.GroupRepresentation) (*clients.GroupRepresentation, error) {
	return nil, nil
}
func (m *mockRealmClient) UpdateGroup(ctx context.Context, realm string, g *clients.GroupRepresentation) error {
	return nil
}
func (m *mockRealmClient) DeleteGroup(ctx context.Context, realm, groupID string) error { return nil }
func (m *mockRealmClient) ListGroups(ctx context.Context, realm string) ([]clients.GroupRepresentation, error) {
	return nil, nil
}
func (m *mockRealmClient) SearchGroups(ctx context.Context, realm, name string) ([]clients.GroupRepresentation, error) {
	return nil, nil
}
func (m *mockRealmClient) GetClientSecret(ctx context.Context, realm, clientID string) (string, error) {
	return "", nil
}
func (m *mockRealmClient) GetUserGroups(ctx context.Context, realm, userID string) ([]clients.GroupRepresentation, error) {
	return nil, nil
}
func (m *mockRealmClient) AddUserToGroup(ctx context.Context, realm, userID, groupID string) error {
	return nil
}
func (m *mockRealmClient) RemoveUserFromGroup(ctx context.Context, realm, userID, groupID string) error {
	return nil
}
func (m *mockRealmClient) SearchUsers(ctx context.Context, realm, username string) ([]clients.UserRepresentation, error) {
	return nil, nil
}
func (m *mockRealmClient) GetRealmRole(ctx context.Context, realm, roleName string) (*clients.RoleRepresentation, error) {
	return nil, nil
}
func (m *mockRealmClient) CreateRealmRole(ctx context.Context, realm string, r *clients.RoleRepresentation) error {
	return nil
}
func (m *mockRealmClient) UpdateRealmRole(ctx context.Context, realm, roleName string, r *clients.RoleRepresentation) error {
	return nil
}
func (m *mockRealmClient) DeleteRealmRole(ctx context.Context, realm, roleName string) error {
	return nil
}
func (m *mockRealmClient) GetClientProtocolMapper(ctx context.Context, realm, clientID, mapperID string) (*clients.ProtocolMapperRepresentation, error) {
	return nil, nil
}
func (m *mockRealmClient) CreateClientProtocolMapper(ctx context.Context, realm, clientID string, p *clients.ProtocolMapperRepresentation) (string, error) {
	return "", nil
}
func (m *mockRealmClient) UpdateClientProtocolMapper(ctx context.Context, realm, clientID string, p *clients.ProtocolMapperRepresentation) error {
	return nil
}
func (m *mockRealmClient) DeleteClientProtocolMapper(ctx context.Context, realm, clientID, mapperID string) error {
	return nil
}
func (m *mockRealmClient) ListClientProtocolMappers(ctx context.Context, realm, clientID string) ([]clients.ProtocolMapperRepresentation, error) {
	return nil, nil
}
func (m *mockRealmClient) GetUserFederationProvider(_ context.Context, _, _ string) (*clients.UserFederationProviderRepresentation, error) { return nil, nil }
func (m *mockRealmClient) CreateUserFederationProvider(_ context.Context, _ string, _ *clients.UserFederationProviderRepresentation) (string, error) { return "", nil }
func (m *mockRealmClient) UpdateUserFederationProvider(_ context.Context, _, _ string, _ *clients.UserFederationProviderRepresentation) error { return nil }
func (m *mockRealmClient) DeleteUserFederationProvider(_ context.Context, _, _ string) error { return nil }
func (m *mockRealmClient) ListUserFederationProviders(_ context.Context, _ string) ([]clients.UserFederationProviderRepresentation, error) { return nil, nil }
func (m *mockRealmClient) GetRealmEventsConfig(_ context.Context, _ string) (*clients.RealmEventsConfigRepresentation, error) { return nil, nil }
func (m *mockRealmClient) UpdateRealmEventsConfig(_ context.Context, _ string, _ *clients.RealmEventsConfigRepresentation) error { return nil }
func (m *mockRealmClient) ImportRealm(_ context.Context, _ string, _ bool) error { return nil }
func (m *mockRealmClient) GetAuthzResource(_ context.Context, _, _, _ string) (*clients.AuthzResourceRepresentation, error) { return nil, nil }
func (m *mockRealmClient) CreateAuthzResource(_ context.Context, _, _ string, _ *clients.AuthzResourceRepresentation) (string, error) { return "", nil }
func (m *mockRealmClient) UpdateAuthzResource(_ context.Context, _, _, _ string, _ *clients.AuthzResourceRepresentation) error { return nil }
func (m *mockRealmClient) DeleteAuthzResource(_ context.Context, _, _, _ string) error { return nil }
func (m *mockRealmClient) ListAuthzResources(_ context.Context, _, _ string) ([]clients.AuthzResourceRepresentation, error) { return nil, nil }
func (m *mockRealmClient) GetClientCertificate(_ context.Context, _, _, _ string) (*clients.ClientCertificateRepresentation, error) { return nil, nil }
func (m *mockRealmClient) GenerateClientCertificate(_ context.Context, _, _ string, _ string) (*clients.ClientCertificateRepresentation, error) { return nil, nil }
func (m *mockRealmClient) ListClientCertificates(_ context.Context, _, _ string) ([]clients.ClientCertificateRepresentation, error) { return nil, nil }

func newRealmCR(name string) *realmv1alpha1.Realm {
	return &realmv1alpha1.Realm{
		ObjectMeta: metav1.ObjectMeta{Name: "test-realm", Namespace: "default"},
		Spec: realmv1alpha1.RealmSpec{
			ForProvider: realmv1alpha1.RealmParameters{
				Realm: name,
			},
		},
	}
}

type wrongRealmMG = userv1alpha1.User

func TestRealmObserve(t *testing.T) {
	tests := []struct {
		name       string
		mg         resource.Managed
		getRealmFn func(ctx context.Context, realm string) (*clients.Realm, error)
		wantExists bool
		wantErrStr string
	}{
		{
			name:       "wrong managed type",
			mg:         &wrongRealmMG{},
			wantErrStr: errNotRealm,
		},
		{
			name: "realm not found",
			mg:   newRealmCR("test"),
			getRealmFn: func(_ context.Context, _ string) (*clients.Realm, error) {
				return nil, nil
			},
			wantExists: false,
		},
		{
			name: "realm found",
			mg:   newRealmCR("test"),
			getRealmFn: func(_ context.Context, _ string) (*clients.Realm, error) {
				return &clients.Realm{Realm: "test", Enabled: true}, nil
			},
			wantExists: true,
		},
		{
			name: "get realm error",
			mg:   newRealmCR("test"),
			getRealmFn: func(_ context.Context, _ string) (*clients.Realm, error) {
				return nil, errors.New("404 not found")
			},
			wantExists: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &external{client: &mockRealmClient{getRealmFn: tt.getRealmFn}}
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

func TestRealmCreate(t *testing.T) {
	tests := []struct {
		name        string
		mg          resource.Managed
		createFn    func(ctx context.Context, r *clients.Realm) (*clients.Realm, error)
		wantErrStr  string
		wantSuccess bool
	}{
		{
			name:       "wrong managed type",
			mg:         &wrongRealmMG{},
			wantErrStr: errNotRealm,
		},
		{
			name: "create error",
			mg:   newRealmCR("test"),
			createFn: func(_ context.Context, _ *clients.Realm) (*clients.Realm, error) {
				return nil, errors.New("conflict")
			},
			wantErrStr: "conflict",
		},
		{
			name: "successful create",
			mg:   newRealmCR("test"),
			createFn: func(_ context.Context, r *clients.Realm) (*clients.Realm, error) {
				return r, nil
			},
			wantSuccess: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &external{client: &mockRealmClient{createRealmFn: tt.createFn}}
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
			if !tt.wantSuccess {
				t.Error("expected successful create")
			}
		})
	}
}

func TestRealmDelete(t *testing.T) {
	tests := []struct {
		name       string
		mg         resource.Managed
		deleteFn   func(ctx context.Context, realm string) error
		wantErrStr string
	}{
		{
			name:       "wrong managed type",
			mg:         &wrongRealmMG{},
			wantErrStr: errNotRealm,
		},
		{
			name: "delete not found (ok)",
			mg:   newRealmCR("test"),
			deleteFn: func(_ context.Context, _ string) error {
				return errors.New("404 not found")
			},
		},
		{
			name: "delete error",
			mg:   newRealmCR("test"),
			deleteFn: func(_ context.Context, _ string) error {
				return errors.New("permission denied")
			},
			wantErrStr: "permission denied",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &external{client: &mockRealmClient{deleteRealmFn: tt.deleteFn}}
			_, err := e.Delete(context.Background(), tt.mg)
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
