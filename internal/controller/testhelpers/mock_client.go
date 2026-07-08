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

package testhelpers

import (
	"context"

	"github.com/rossigee/provider-keycloak/internal/clients"
)

// BaseMockClient provides stub implementations for all Client interface methods.
// Embed this in test-specific mock clients to avoid repetition.
type BaseMockClient struct{}

func (m *BaseMockClient) AddClientDefaultScopes(context.Context, string, string, []clients.ClientScopeRepresentation) error {
	return nil
}
func (m *BaseMockClient) RemoveClientDefaultScopes(context.Context, string, string, []clients.ClientScopeRepresentation) error {
	return nil
}
func (m *BaseMockClient) ListClientDefaultScopes(context.Context, string, string) ([]clients.ClientScopeRepresentation, error) {
	return nil, nil
}
func (m *BaseMockClient) AddClientOptionalScopes(context.Context, string, string, []clients.ClientScopeRepresentation) error {
	return nil
}
func (m *BaseMockClient) RemoveClientOptionalScopes(context.Context, string, string, []clients.ClientScopeRepresentation) error {
	return nil
}
func (m *BaseMockClient) ListClientOptionalScopes(context.Context, string, string) ([]clients.ClientScopeRepresentation, error) {
	return nil, nil
}
func (m *BaseMockClient) GetIdentityProvider(context.Context, string, string) (*clients.IdentityProviderRepresentation, error) {
	return nil, nil
}
func (m *BaseMockClient) CreateIdentityProvider(context.Context, string, *clients.IdentityProviderRepresentation) (string, error) {
	return "", nil
}
func (m *BaseMockClient) UpdateIdentityProvider(context.Context, string, string, *clients.IdentityProviderRepresentation) error {
	return nil
}
func (m *BaseMockClient) DeleteIdentityProvider(context.Context, string, string) error { return nil }
func (m *BaseMockClient) ListIdentityProviders(context.Context, string) ([]clients.IdentityProviderRepresentation, error) {
	return nil, nil
}
func (m *BaseMockClient) GetAuthenticationFlow(context.Context, string, string) (*clients.AuthenticationFlowRepresentation, error) {
	return nil, nil
}
func (m *BaseMockClient) CreateAuthenticationFlow(context.Context, string, *clients.AuthenticationFlowRepresentation) (string, error) {
	return "", nil
}
func (m *BaseMockClient) UpdateAuthenticationFlow(context.Context, string, string, *clients.AuthenticationFlowRepresentation) error {
	return nil
}
func (m *BaseMockClient) DeleteAuthenticationFlow(context.Context, string, string) error { return nil }
func (m *BaseMockClient) ListAuthenticationFlows(context.Context, string) ([]clients.AuthenticationFlowRepresentation, error) {
	return nil, nil
}
func (m *BaseMockClient) GetAuthorizationPolicy(context.Context, string, string, string) (*clients.AuthorizationPolicyRepresentation, error) {
	return nil, nil
}
func (m *BaseMockClient) CreateAuthorizationPolicy(context.Context, string, string, *clients.AuthorizationPolicyRepresentation) (string, error) {
	return "", nil
}
func (m *BaseMockClient) UpdateAuthorizationPolicy(context.Context, string, string, string, *clients.AuthorizationPolicyRepresentation) error {
	return nil
}
func (m *BaseMockClient) DeleteAuthorizationPolicy(context.Context, string, string, string) error {
	return nil
}
func (m *BaseMockClient) ListAuthorizationPolicies(context.Context, string, string) ([]clients.AuthorizationPolicyRepresentation, error) {
	return nil, nil
}
func (m *BaseMockClient) ListUserClientRoleMappings(context.Context, string, string, string) ([]clients.RoleRepresentation, error) {
	return nil, nil
}
func (m *BaseMockClient) AddUserClientRoleMappings(context.Context, string, string, string, []clients.RoleRepresentation) error {
	return nil
}
func (m *BaseMockClient) RemoveUserClientRoleMappings(context.Context, string, string, string, []clients.RoleRepresentation) error {
	return nil
}
func (m *BaseMockClient) ListClientScopeMappings(context.Context, string, string) ([]clients.RoleRepresentation, error) {
	return nil, nil
}
func (m *BaseMockClient) AddClientScopeMappings(context.Context, string, string, []clients.RoleRepresentation) error {
	return nil
}
func (m *BaseMockClient) RemoveClientScopeMappings(context.Context, string, string, []clients.RoleRepresentation) error {
	return nil
}
func (m *BaseMockClient) GetClientRole(context.Context, string, string, string) (*clients.RoleRepresentation, error) {
	return nil, nil
}
func (m *BaseMockClient) CreateClientRole(context.Context, string, string, *clients.RoleRepresentation) error {
	return nil
}
func (m *BaseMockClient) UpdateClientRole(context.Context, string, string, string, *clients.RoleRepresentation) error {
	return nil
}
func (m *BaseMockClient) DeleteClientRole(context.Context, string, string, string) error { return nil }
func (m *BaseMockClient) CreateClientInitialAccess(context.Context, string, int32, int32) (*clients.ClientInitialAccessRepresentation, error) {
	return nil, nil
}
func (m *BaseMockClient) ListClientInitialAccess(context.Context, string) ([]clients.ClientInitialAccessRepresentation, error) {
	return nil, nil
}
func (m *BaseMockClient) DeleteClientInitialAccess(context.Context, string, string) error { return nil }
func (m *BaseMockClient) GetComponent(context.Context, string, string) (*clients.ComponentRepresentation, error) {
	return nil, nil
}
func (m *BaseMockClient) CreateComponent(context.Context, string, *clients.ComponentRepresentation) (string, error) {
	return "", nil
}
func (m *BaseMockClient) UpdateComponent(context.Context, string, string, *clients.ComponentRepresentation) error {
	return nil
}
func (m *BaseMockClient) DeleteComponent(context.Context, string, string) error { return nil }
func (m *BaseMockClient) ListComponentsByType(context.Context, string, string, string) ([]clients.ComponentRepresentation, error) {
	return nil, nil
}
func (m *BaseMockClient) GetRealmKeys(context.Context, string) (*clients.RealmKeysRepresentation, error) {
	return nil, nil
}
func (m *BaseMockClient) GetClientScope(_ context.Context, _, _ string) (*clients.ClientScopeRepresentation, error) {
	return nil, nil
}
func (m *BaseMockClient) CreateClientScope(_ context.Context, _ string, _ clients.ClientScopeRepresentation) error {
	return nil
}
func (m *BaseMockClient) UpdateClientScope(_ context.Context, _ string, _ clients.ClientScopeRepresentation) error {
	return nil
}
func (m *BaseMockClient) DeleteClientScope(_ context.Context, _, _ string) error { return nil }
