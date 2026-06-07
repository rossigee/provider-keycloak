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

package protocolmapper

import (
	"github.com/rossigee/provider-keycloak/internal/controller/testhelpers"
	"context"
	"errors"
	"testing"

	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	clientv1alpha1 "github.com/rossigee/provider-keycloak/apis/client/v1alpha1"
	realmv1alpha1 "github.com/rossigee/provider-keycloak/apis/realm/v1alpha1"
	"github.com/rossigee/provider-keycloak/internal/clients"
)

type mockMapperClient struct {
	*testhelpers.BaseMockClient
	getClientFn    func(ctx context.Context, realm, clientID string) (*clients.ClientRepresentation, error)
	listMappersFn  func(ctx context.Context, realm, clientUUID string) ([]clients.ProtocolMapperRepresentation, error)
	createMapperFn func(ctx context.Context, realm, clientUUID string, p *clients.ProtocolMapperRepresentation) (string, error)
	updateMapperFn func(ctx context.Context, realm, clientUUID string, p *clients.ProtocolMapperRepresentation) error
	deleteMapperFn func(ctx context.Context, realm, clientUUID, mapperID string) error
}

func (m *mockMapperClient) GetClient(ctx context.Context, realm, clientID string) (*clients.ClientRepresentation, error) {
	return m.getClientFn(ctx, realm, clientID)
}
func (m *mockMapperClient) ListClientProtocolMappers(ctx context.Context, realm, clientUUID string) ([]clients.ProtocolMapperRepresentation, error) {
	return m.listMappersFn(ctx, realm, clientUUID)
}
func (m *mockMapperClient) CreateClientProtocolMapper(ctx context.Context, realm, clientUUID string, p *clients.ProtocolMapperRepresentation) (string, error) {
	return m.createMapperFn(ctx, realm, clientUUID, p)
}
func (m *mockMapperClient) UpdateClientProtocolMapper(ctx context.Context, realm, clientUUID string, p *clients.ProtocolMapperRepresentation) error {
	return m.updateMapperFn(ctx, realm, clientUUID, p)
}
func (m *mockMapperClient) DeleteClientProtocolMapper(ctx context.Context, realm, clientUUID, mapperID string) error {
	return m.deleteMapperFn(ctx, realm, clientUUID, mapperID)
}

func (m *mockMapperClient) GetRealm(ctx context.Context, realm string) (*clients.Realm, error) {
	return nil, nil
}
func (m *mockMapperClient) CreateRealm(ctx context.Context, r *clients.Realm) (*clients.Realm, error) {
	return nil, nil
}
func (m *mockMapperClient) UpdateRealm(ctx context.Context, r *clients.Realm) error { return nil }
func (m *mockMapperClient) DeleteRealm(ctx context.Context, realm string) error     { return nil }
func (m *mockMapperClient) CreateClient(ctx context.Context, realm string, c *clients.ClientRepresentation) (*clients.ClientRepresentation, error) {
	return nil, nil
}
func (m *mockMapperClient) UpdateClient(ctx context.Context, realm string, c *clients.ClientRepresentation) error {
	return nil
}
func (m *mockMapperClient) DeleteClient(ctx context.Context, realm, clientID string) error {
	return nil
}
func (m *mockMapperClient) ListClients(ctx context.Context, realm string) ([]clients.ClientRepresentation, error) {
	return nil, nil
}
func (m *mockMapperClient) GetUser(ctx context.Context, realm, username string) (*clients.UserRepresentation, error) {
	return nil, nil
}
func (m *mockMapperClient) CreateUser(ctx context.Context, realm string, u *clients.UserRepresentation) (*clients.UserRepresentation, error) {
	return nil, nil
}
func (m *mockMapperClient) UpdateUser(ctx context.Context, realm string, u *clients.UserRepresentation) error {
	return nil
}
func (m *mockMapperClient) DeleteUser(ctx context.Context, realm, userID string) error { return nil }
func (m *mockMapperClient) ListUsers(ctx context.Context, realm string) ([]clients.UserRepresentation, error) {
	return nil, nil
}
func (m *mockMapperClient) GetGroup(ctx context.Context, realm, groupID string) (*clients.GroupRepresentation, error) {
	return nil, nil
}
func (m *mockMapperClient) CreateGroup(ctx context.Context, realm string, g *clients.GroupRepresentation) (*clients.GroupRepresentation, error) {
	return nil, nil
}
func (m *mockMapperClient) UpdateGroup(ctx context.Context, realm string, g *clients.GroupRepresentation) error {
	return nil
}
func (m *mockMapperClient) DeleteGroup(ctx context.Context, realm, groupID string) error { return nil }
func (m *mockMapperClient) ListGroups(ctx context.Context, realm string) ([]clients.GroupRepresentation, error) {
	return nil, nil
}
func (m *mockMapperClient) SearchGroups(ctx context.Context, realm, name string) ([]clients.GroupRepresentation, error) {
	return nil, nil
}
func (m *mockMapperClient) GetClientSecret(ctx context.Context, realm, clientID string) (string, error) {
	return "", nil
}
func (m *mockMapperClient) GetUserGroups(ctx context.Context, realm, userID string) ([]clients.GroupRepresentation, error) {
	return nil, nil
}
func (m *mockMapperClient) AddUserToGroup(ctx context.Context, realm, userID, groupID string) error {
	return nil
}
func (m *mockMapperClient) RemoveUserFromGroup(ctx context.Context, realm, userID, groupID string) error {
	return nil
}
func (m *mockMapperClient) SearchUsers(ctx context.Context, realm, username string) ([]clients.UserRepresentation, error) {
	return nil, nil
}
func (m *mockMapperClient) GetRealmRole(ctx context.Context, realm, roleName string) (*clients.RoleRepresentation, error) {
	return nil, nil
}
func (m *mockMapperClient) CreateRealmRole(ctx context.Context, realm string, r *clients.RoleRepresentation) error {
	return nil
}
func (m *mockMapperClient) UpdateRealmRole(ctx context.Context, realm, roleName string, r *clients.RoleRepresentation) error {
	return nil
}
func (m *mockMapperClient) DeleteRealmRole(ctx context.Context, realm, roleName string) error {
	return nil
}
func (m *mockMapperClient) GetClientProtocolMapper(ctx context.Context, realm, clientID, mapperID string) (*clients.ProtocolMapperRepresentation, error) {
	return nil, nil
}
func (m *mockMapperClient) GetUserFederationProvider(_ context.Context, _, _ string) (*clients.UserFederationProviderRepresentation, error) { return nil, nil }
func (m *mockMapperClient) CreateUserFederationProvider(_ context.Context, _ string, _ *clients.UserFederationProviderRepresentation) (string, error) { return "", nil }
func (m *mockMapperClient) UpdateUserFederationProvider(_ context.Context, _, _ string, _ *clients.UserFederationProviderRepresentation) error { return nil }
func (m *mockMapperClient) DeleteUserFederationProvider(_ context.Context, _, _ string) error { return nil }
func (m *mockMapperClient) ListUserFederationProviders(_ context.Context, _ string) ([]clients.UserFederationProviderRepresentation, error) { return nil, nil }
func (m *mockMapperClient) GetRealmEventsConfig(_ context.Context, _ string) (*clients.RealmEventsConfigRepresentation, error) { return nil, nil }
func (m *mockMapperClient) UpdateRealmEventsConfig(_ context.Context, _ string, _ *clients.RealmEventsConfigRepresentation) error { return nil }
func (m *mockMapperClient) ImportRealm(_ context.Context, _ string, _ bool) error { return nil }
func (m *mockMapperClient) GetAuthzResource(_ context.Context, _, _, _ string) (*clients.AuthzResourceRepresentation, error) { return nil, nil }
func (m *mockMapperClient) CreateAuthzResource(_ context.Context, _, _ string, _ *clients.AuthzResourceRepresentation) (string, error) { return "", nil }
func (m *mockMapperClient) UpdateAuthzResource(_ context.Context, _, _, _ string, _ *clients.AuthzResourceRepresentation) error { return nil }
func (m *mockMapperClient) DeleteAuthzResource(_ context.Context, _, _, _ string) error { return nil }
func (m *mockMapperClient) ListAuthzResources(_ context.Context, _, _ string) ([]clients.AuthzResourceRepresentation, error) { return nil, nil }
func (m *mockMapperClient) GetClientCertificate(_ context.Context, _, _, _ string) (*clients.ClientCertificateRepresentation, error) { return nil, nil }
func (m *mockMapperClient) GenerateClientCertificate(_ context.Context, _, _ string, _ string) (*clients.ClientCertificateRepresentation, error) { return nil, nil }
func (m *mockMapperClient) ListClientCertificates(_ context.Context, _, _ string) ([]clients.ClientCertificateRepresentation, error) { return nil, nil }

func newMapperCR(realmId, clientId, name string) *clientv1alpha1.ProtocolMapper {
	cr := &clientv1alpha1.ProtocolMapper{
		ObjectMeta: metav1.ObjectMeta{Name: "test-mapper", Namespace: "default"},
		Spec: clientv1alpha1.ProtocolMapperSpec{
			ForProvider: clientv1alpha1.ProtocolMapperParameters{
				Name:     name,
				Protocol: "openid-connect",
			},
		},
	}
	if realmId != "" {
		cr.Spec.ForProvider.RealmId = &realmId
	}
	if clientId != "" {
		cr.Spec.ForProvider.ClientId = &clientId
	}
	return cr
}

type wrongMapperMG = realmv1alpha1.Realm

func TestMapperObserve(t *testing.T) {
	tests := []struct {
		name        string
		mg          resource.Managed
		getClientFn func(ctx context.Context, realm, clientID string) (*clients.ClientRepresentation, error)
		listFn      func(ctx context.Context, realm, clientUUID string) ([]clients.ProtocolMapperRepresentation, error)
		wantExists  bool
		wantErrStr  string
	}{
		{
			name:       "wrong managed type",
			mg:         &wrongMapperMG{},
			wantErrStr: errNotMapper,
		},
		{
			name:       "empty realmId",
			mg:         newMapperCR("", "client-id", "mapper-name"),
			wantErrStr: "realmId is required",
		},
		{
			name:       "empty clientId",
			mg:         newMapperCR("realm", "", "mapper-name"),
			wantErrStr: "clientId is required",
		},
		{
			name: "client not found",
			mg:   newMapperCR("realm", "client-id", "mapper-name"),
			getClientFn: func(_ context.Context, _, _ string) (*clients.ClientRepresentation, error) {
				return nil, errors.New("not found")
			},
			wantErrStr: errResolveClient,
		},
		{
			name: "mapper not found",
			mg:   newMapperCR("realm", "client-id", "mapper-name"),
			getClientFn: func(_ context.Context, _, _ string) (*clients.ClientRepresentation, error) {
				return &clients.ClientRepresentation{ID: "uuid"}, nil
			},
			listFn: func(_ context.Context, _, _ string) ([]clients.ProtocolMapperRepresentation, error) {
				return nil, nil
			},
			wantExists: false,
		},
		{
			name: "mapper found",
			mg:   newMapperCR("realm", "client-id", "mapper-name"),
			getClientFn: func(_ context.Context, _, _ string) (*clients.ClientRepresentation, error) {
				return &clients.ClientRepresentation{ID: "uuid"}, nil
			},
			listFn: func(_ context.Context, _, _ string) ([]clients.ProtocolMapperRepresentation, error) {
				return []clients.ProtocolMapperRepresentation{{Name: "mapper-name"}}, nil
			},
			wantExists: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mc := &mockMapperClient{
				getClientFn:   tt.getClientFn,
				listMappersFn: tt.listFn,
			}
			e := &external{kc: mc}
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
