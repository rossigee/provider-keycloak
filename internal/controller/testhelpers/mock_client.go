/*
Copyright 2024 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
*/

// Package testhelpers exports BaseMockClient, a no-op implementation of
// clients.Client. Unit tests embed it via pointer to satisfy the full
// client interface; per-test wrappers override the methods they exercise.
package testhelpers

import (
	"context"

	"github.com/rossigee/provider-keycloak/internal/clients"
)

// AddScopeCall records one ClientDefaultScopes/OptionalScopes assignment
// invocation. Captured by AssigningClient so unit tests can assert that the
// controller reached the right Add endpoint with the right scopes.
type AddScopeCall struct {
	ClientUUID string
	Scopes     []clients.ClientScopeRepresentation
}

// AssigningClient embeds BaseMockClient and overrides the three scope
// assignment methods to record their inputs. Use it from a controller
// unit test whenever you want to assert which scopes were added/removed.
//
// Example:
//
//	kc := &AssigningClient{  // embedded BaseMockClient via zero value pointer
//	    BaseMockClient: &testhelpers.BaseMockClient{},
//	}
type AssigningClient struct {
	*BaseMockClient

	Adds    []AddScopeCall
	Removes []AddScopeCall
	ListFn  func(ctx context.Context, realm, clientUUID string) ([]clients.ClientScopeRepresentation, error)
}

func (a *AssigningClient) AddClientDefaultScopes(ctx context.Context, realm, clientUUID string, scopes []clients.ClientScopeRepresentation) error {
	a.Adds = append(a.Adds, AddScopeCall{ClientUUID: clientUUID, Scopes: scopes})
	return a.BaseMockClient.AddClientDefaultScopes(ctx, realm, clientUUID, scopes)
}

func (a *AssigningClient) RemoveClientDefaultScopes(ctx context.Context, realm, clientUUID string, scopes []clients.ClientScopeRepresentation) error {
	a.Removes = append(a.Removes, AddScopeCall{ClientUUID: clientUUID, Scopes: scopes})
	return a.BaseMockClient.RemoveClientDefaultScopes(ctx, realm, clientUUID, scopes)
}

func (a *AssigningClient) ListClientDefaultScopes(ctx context.Context, realm, clientUUID string) ([]clients.ClientScopeRepresentation, error) {
	if a.ListFn != nil {
		return a.ListFn(ctx, realm, clientUUID)
	}
	return a.BaseMockClient.ListClientDefaultScopes(ctx, realm, clientUUID)
}

func (a *AssigningClient) AddClientOptionalScopes(ctx context.Context, realm, clientUUID string, scopes []clients.ClientScopeRepresentation) error {
	return a.BaseMockClient.AddClientOptionalScopes(ctx, realm, clientUUID, scopes)
}

func (a *AssigningClient) RemoveClientOptionalScopes(ctx context.Context, realm, clientUUID string, scopes []clients.ClientScopeRepresentation) error {
	return a.BaseMockClient.RemoveClientOptionalScopes(ctx, realm, clientUUID, scopes)
}

func (a *AssigningClient) ListClientOptionalScopes(ctx context.Context, realm, clientUUID string) ([]clients.ClientScopeRepresentation, error) {
	return a.BaseMockClient.ListClientOptionalScopes(ctx, realm, clientUUID)
}

// BaseMockClient is a no-op clients.Client implementation. Every method
// returns nil / zero values; per-test wrappers override the methods they
// exercise.
type BaseMockClient struct{}

// Realm
func (m *BaseMockClient) GetRealm(context.Context, string) (*clients.Realm, error) {
	return nil, nil
}
func (m *BaseMockClient) CreateRealm(context.Context, *clients.Realm) (*clients.Realm, error) {
	return nil, nil
}
func (m *BaseMockClient) UpdateRealm(context.Context, *clients.Realm) error { return nil }
func (m *BaseMockClient) DeleteRealm(context.Context, string) error         { return nil }
func (m *BaseMockClient) ImportRealm(context.Context, string, bool) error   { return nil }

// Client
func (m *BaseMockClient) GetClient(context.Context, string, string) (*clients.ClientRepresentation, error) {
	return nil, nil
}
func (m *BaseMockClient) CreateClient(context.Context, string, *clients.ClientRepresentation) (*clients.ClientRepresentation, error) {
	return nil, nil
}
func (m *BaseMockClient) UpdateClient(context.Context, string, *clients.ClientRepresentation) error {
	return nil
}
func (m *BaseMockClient) DeleteClient(context.Context, string, string) error { return nil }
func (m *BaseMockClient) ListClients(context.Context, string) ([]clients.ClientRepresentation, error) {
	return nil, nil
}
func (m *BaseMockClient) GetClientSecret(context.Context, string, string) (string, error) {
	return "", nil
}
func (m *BaseMockClient) ResetClientSecret(context.Context, string, string, string) error {
	return nil
}

// Protocol mappers
func (m *BaseMockClient) GetClientProtocolMapper(context.Context, string, string, string) (*clients.ProtocolMapperRepresentation, error) {
	return nil, nil
}
func (m *BaseMockClient) CreateClientProtocolMapper(context.Context, string, string, *clients.ProtocolMapperRepresentation) (string, error) {
	return "", nil
}
func (m *BaseMockClient) UpdateClientProtocolMapper(context.Context, string, string, *clients.ProtocolMapperRepresentation) error {
	return nil
}
func (m *BaseMockClient) DeleteClientProtocolMapper(context.Context, string, string, string) error {
	return nil
}
func (m *BaseMockClient) ListClientProtocolMappers(context.Context, string, string) ([]clients.ProtocolMapperRepresentation, error) {
	return nil, nil
}

// Users
func (m *BaseMockClient) GetUser(context.Context, string, string) (*clients.UserRepresentation, error) {
	return nil, nil
}
func (m *BaseMockClient) CreateUser(context.Context, string, *clients.UserRepresentation) (*clients.UserRepresentation, error) {
	return nil, nil
}
func (m *BaseMockClient) UpdateUser(context.Context, string, *clients.UserRepresentation) error {
	return nil
}
func (m *BaseMockClient) DeleteUser(context.Context, string, string) error { return nil }
func (m *BaseMockClient) ListUsers(context.Context, string) ([]clients.UserRepresentation, error) {
	return nil, nil
}
func (m *BaseMockClient) SearchUsers(context.Context, string, string) ([]clients.UserRepresentation, error) {
	return nil, nil
}

// Groups
func (m *BaseMockClient) GetGroup(context.Context, string, string) (*clients.GroupRepresentation, error) {
	return nil, nil
}
func (m *BaseMockClient) CreateGroup(context.Context, string, *clients.GroupRepresentation) (*clients.GroupRepresentation, error) {
	return nil, nil
}
func (m *BaseMockClient) UpdateGroup(context.Context, string, *clients.GroupRepresentation) error {
	return nil
}
func (m *BaseMockClient) DeleteGroup(context.Context, string, string) error { return nil }
func (m *BaseMockClient) ListGroups(context.Context, string) ([]clients.GroupRepresentation, error) {
	return nil, nil
}
func (m *BaseMockClient) SearchGroups(context.Context, string, string) ([]clients.GroupRepresentation, error) {
	return nil, nil
}
func (m *BaseMockClient) GetUserGroups(context.Context, string, string) ([]clients.GroupRepresentation, error) {
	return nil, nil
}
func (m *BaseMockClient) AddUserToGroup(context.Context, string, string, string) error { return nil }
func (m *BaseMockClient) RemoveUserFromGroup(context.Context, string, string, string) error {
	return nil
}

// Roles
func (m *BaseMockClient) GetRealmRole(context.Context, string, string) (*clients.RoleRepresentation, error) {
	return nil, nil
}
func (m *BaseMockClient) CreateRealmRole(context.Context, string, *clients.RoleRepresentation) error {
	return nil
}
func (m *BaseMockClient) UpdateRealmRole(context.Context, string, string, *clients.RoleRepresentation) error {
	return nil
}
func (m *BaseMockClient) DeleteRealmRole(context.Context, string, string) error { return nil }
func (m *BaseMockClient) GetClientRole(context.Context, string, string, string) (*clients.RoleRepresentation, error) {
	return nil, nil
}
func (m *BaseMockClient) CreateClientRole(context.Context, string, string, *clients.RoleRepresentation) error {
	return nil
}
func (m *BaseMockClient) UpdateClientRole(context.Context, string, string, string, *clients.RoleRepresentation) error {
	return nil
}
func (m *BaseMockClient) DeleteClientRole(context.Context, string, string, string) error {
	return nil
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

// Client scopes
func (m *BaseMockClient) ListClientDefaultScopes(context.Context, string, string) ([]clients.ClientScopeRepresentation, error) {
	return nil, nil
}
func (m *BaseMockClient) AddClientDefaultScopes(context.Context, string, string, []clients.ClientScopeRepresentation) error {
	return nil
}
func (m *BaseMockClient) RemoveClientDefaultScopes(context.Context, string, string, []clients.ClientScopeRepresentation) error {
	return nil
}
func (m *BaseMockClient) ListClientOptionalScopes(context.Context, string, string) ([]clients.ClientScopeRepresentation, error) {
	return nil, nil
}
func (m *BaseMockClient) AddClientOptionalScopes(context.Context, string, string, []clients.ClientScopeRepresentation) error {
	return nil
}
func (m *BaseMockClient) RemoveClientOptionalScopes(context.Context, string, string, []clients.ClientScopeRepresentation) error {
	return nil
}
func (m *BaseMockClient) GetClientScope(_ context.Context, _ string, name string) (*clients.ClientScopeRepresentation, error) {
	return &clients.ClientScopeRepresentation{ID: "scope-uuid", Name: name}, nil
}
func (m *BaseMockClient) CreateClientScope(_ context.Context, _ string, _ clients.ClientScopeRepresentation) error {
	return nil
}
func (m *BaseMockClient) UpdateClientScope(_ context.Context, _ string, _ clients.ClientScopeRepresentation) error {
	return nil
}
func (m *BaseMockClient) DeleteClientScope(_ context.Context, _ string, _ string) error {
	return nil
}

// Identity providers
func (m *BaseMockClient) GetIdentityProvider(context.Context, string, string) (*clients.IdentityProviderRepresentation, error) {
	return nil, nil
}
func (m *BaseMockClient) CreateIdentityProvider(context.Context, string, *clients.IdentityProviderRepresentation) (string, error) {
	return "", nil
}
func (m *BaseMockClient) UpdateIdentityProvider(context.Context, string, string, *clients.IdentityProviderRepresentation) error {
	return nil
}
func (m *BaseMockClient) DeleteIdentityProvider(context.Context, string, string) error {
	return nil
}
func (m *BaseMockClient) ListIdentityProviders(context.Context, string) ([]clients.IdentityProviderRepresentation, error) {
	return nil, nil
}

// Authentication flows
func (m *BaseMockClient) GetAuthenticationFlow(context.Context, string, string) (*clients.AuthenticationFlowRepresentation, error) {
	return nil, nil
}
func (m *BaseMockClient) CreateAuthenticationFlow(context.Context, string, *clients.AuthenticationFlowRepresentation) (string, error) {
	return "", nil
}
func (m *BaseMockClient) UpdateAuthenticationFlow(context.Context, string, string, *clients.AuthenticationFlowRepresentation) error {
	return nil
}
func (m *BaseMockClient) DeleteAuthenticationFlow(context.Context, string, string) error {
	return nil
}
func (m *BaseMockClient) ListAuthenticationFlows(context.Context, string) ([]clients.AuthenticationFlowRepresentation, error) {
	return nil, nil
}

// Authorization policies + resources
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
func (m *BaseMockClient) GetAuthzResource(context.Context, string, string, string) (*clients.AuthzResourceRepresentation, error) {
	return nil, nil
}
func (m *BaseMockClient) CreateAuthzResource(context.Context, string, string, *clients.AuthzResourceRepresentation) (string, error) {
	return "", nil
}
func (m *BaseMockClient) UpdateAuthzResource(context.Context, string, string, string, *clients.AuthzResourceRepresentation) error {
	return nil
}
func (m *BaseMockClient) DeleteAuthzResource(context.Context, string, string, string) error {
	return nil
}
func (m *BaseMockClient) ListAuthzResources(context.Context, string, string) ([]clients.AuthzResourceRepresentation, error) {
	return nil, nil
}

// User federation
func (m *BaseMockClient) GetUserFederationProvider(context.Context, string, string) (*clients.UserFederationProviderRepresentation, error) {
	return nil, nil
}
func (m *BaseMockClient) CreateUserFederationProvider(context.Context, string, *clients.UserFederationProviderRepresentation) (string, error) {
	return "", nil
}
func (m *BaseMockClient) UpdateUserFederationProvider(context.Context, string, string, *clients.UserFederationProviderRepresentation) error {
	return nil
}
func (m *BaseMockClient) DeleteUserFederationProvider(context.Context, string, string) error {
	return nil
}
func (m *BaseMockClient) ListUserFederationProviders(context.Context, string) ([]clients.UserFederationProviderRepresentation, error) {
	return nil, nil
}

// Client cert / initial access
func (m *BaseMockClient) GetClientCertificate(context.Context, string, string, string) (*clients.ClientCertificateRepresentation, error) {
	return nil, nil
}
func (m *BaseMockClient) GenerateClientCertificate(context.Context, string, string, string) (*clients.ClientCertificateRepresentation, error) {
	return nil, nil
}
func (m *BaseMockClient) ListClientCertificates(context.Context, string, string) ([]clients.ClientCertificateRepresentation, error) {
	return nil, nil
}
func (m *BaseMockClient) DeleteClientCertificate(context.Context, string, string, string) error {
	return nil
}
func (m *BaseMockClient) CreateClientInitialAccess(context.Context, string, int32, int32) (*clients.ClientInitialAccessRepresentation, error) {
	return nil, nil
}
func (m *BaseMockClient) ListClientInitialAccess(context.Context, string) ([]clients.ClientInitialAccessRepresentation, error) {
	return nil, nil
}
func (m *BaseMockClient) DeleteClientInitialAccess(context.Context, string, string) error {
	return nil
}

// Components
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

// Realm top-level
func (m *BaseMockClient) GetRealmKeys(context.Context, string) (*clients.RealmKeysRepresentation, error) {
	return nil, nil
}
func (m *BaseMockClient) GetRealmEventsConfig(context.Context, string) (*clients.RealmEventsConfigRepresentation, error) {
	return nil, nil
}
func (m *BaseMockClient) UpdateRealmEventsConfig(context.Context, string, *clients.RealmEventsConfigRepresentation) error {
	return nil
}
func (m *BaseMockClient) GetRawRealm(context.Context, string) ([]byte, error)  { return nil, nil }
func (m *BaseMockClient) UpdateRealmRaw(context.Context, string, []byte) error { return nil }
