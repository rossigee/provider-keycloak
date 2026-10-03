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

package clients

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The write wrappers are the same shape as the read ones: build a path, call
// doRequest, return. A wrong method or path fails silently in the same way - the
// controller sees a 404 and reports the resource as unchanged or absent.
//
// Expected methods and paths come from the Keycloak Admin REST API. This
// complements TestReadWrapperPaths, which covers the reads.
type writeCase struct {
	wantMethod string
	wantPath   string
	body       string
	call       func(context.Context, *keycloakClient) error
}

func TestWriteWrapperMethodsAndPaths(t *testing.T) {
	const (
		realm    = "master"
		clientID = "cid-1"
		userID   = "uid-1"
		groupID  = "gid-1"
	)

	noop := func(context.Context, *keycloakClient) error { return nil }

	cases := map[string]writeCase{
		// Realm roles: POST/PUT/DELETE on the roles collection.
		"CreateRealmRole": {
			wantMethod: http.MethodPost, wantPath: "/admin/realms/master/roles",
			body: `{}`, call: func(ctx context.Context, c *keycloakClient) error {
				return c.CreateRealmRole(ctx, realm, &RoleRepresentation{Name: "r"})
			},
		},
		"UpdateRealmRole": {
			wantMethod: http.MethodPut, wantPath: "/admin/realms/master/roles/admin",
			body: `{}`, call: func(ctx context.Context, c *keycloakClient) error {
				return c.UpdateRealmRole(ctx, realm, "admin", &RoleRepresentation{Name: "admin"})
			},
		},
		"DeleteRealmRole": {
			wantMethod: http.MethodDelete, wantPath: "/admin/realms/master/roles/admin",
			call: func(ctx context.Context, c *keycloakClient) error {
				return c.DeleteRealmRole(ctx, realm, "admin")
			},
		},

		// Client roles live under the client, not the realm.
		"CreateClientRole": {
			wantMethod: http.MethodPost, wantPath: "/admin/realms/master/clients/cid-1/roles",
			body: `{}`, call: func(ctx context.Context, c *keycloakClient) error {
				return c.CreateClientRole(ctx, realm, clientID, &RoleRepresentation{Name: "viewer"})
			},
		},
		"UpdateClientRole": {
			wantMethod: http.MethodPut, wantPath: "/admin/realms/master/clients/cid-1/roles/viewer",
			body: `{}`, call: func(ctx context.Context, c *keycloakClient) error {
				return c.UpdateClientRole(ctx, realm, clientID, "viewer", &RoleRepresentation{Name: "viewer"})
			},
		},
		"DeleteClientRole": {
			wantMethod: http.MethodDelete, wantPath: "/admin/realms/master/clients/cid-1/roles/viewer",
			call: func(ctx context.Context, c *keycloakClient) error {
				return c.DeleteClientRole(ctx, realm, clientID, "viewer")
			},
		},

		// Group membership is a PUT/DELETE pair on the nested collection, which
		// is what makes membership idempotent rather than additive.
		"AddUserToGroup": {
			wantMethod: http.MethodPut, wantPath: "/admin/realms/master/users/uid-1/groups/gid-1",
			call: func(ctx context.Context, c *keycloakClient) error {
				return c.AddUserToGroup(ctx, realm, userID, groupID)
			},
		},
		"RemoveUserFromGroup": {
			wantMethod: http.MethodDelete, wantPath: "/admin/realms/master/users/uid-1/groups/gid-1",
			call: func(ctx context.Context, c *keycloakClient) error {
				return c.RemoveUserFromGroup(ctx, realm, userID, groupID)
			},
		},

		// Role and scope mappings on a client.
		"AddClientScopeMappings": {
			wantMethod: http.MethodPost, wantPath: "/admin/realms/master/clients/cid-1/scope-mappings/realm",
			body: `[{}]`, call: func(ctx context.Context, c *keycloakClient) error {
				return c.AddClientScopeMappings(ctx, realm, clientID, []RoleRepresentation{{Name: "s"}})
			},
		},
		"RemoveClientScopeMappings": {
			wantMethod: http.MethodDelete, wantPath: "/admin/realms/master/clients/cid-1/scope-mappings/realm",
			body: `[{}]`, call: func(ctx context.Context, c *keycloakClient) error {
				return c.RemoveClientScopeMappings(ctx, realm, clientID, []RoleRepresentation{{Name: "s"}})
			},
		},
		"AddClientDefaultScopes": {
			wantMethod: http.MethodPut, wantPath: "/admin/realms/master/clients/cid-1/default-client-scopes/s1",
			body: `[{"id":"s1"}]`, call: func(ctx context.Context, c *keycloakClient) error {
				return c.AddClientDefaultScopes(ctx, realm, clientID, []ClientScopeRepresentation{{ID: "s1"}})
			},
		},
		"RemoveClientDefaultScopes": {
			wantMethod: http.MethodDelete, wantPath: "/admin/realms/master/clients/cid-1/default-client-scopes/s1",
			body: `[{"id":"s1"}]`, call: func(ctx context.Context, c *keycloakClient) error {
				return c.RemoveClientDefaultScopes(ctx, realm, clientID, []ClientScopeRepresentation{{ID: "s1"}})
			},
		},
		"AddClientOptionalScopes": {
			wantMethod: http.MethodPut, wantPath: "/admin/realms/master/clients/cid-1/optional-client-scopes/s1",
			body: `[{"id":"s1"}]`, call: func(ctx context.Context, c *keycloakClient) error {
				return c.AddClientOptionalScopes(ctx, realm, clientID, []ClientScopeRepresentation{{ID: "s1"}})
			},
		},
		"RemoveClientOptionalScopes": {
			wantMethod: http.MethodDelete, wantPath: "/admin/realms/master/clients/cid-1/optional-client-scopes/s1",
			body: `[{"id":"s1"}]`, call: func(ctx context.Context, c *keycloakClient) error {
				return c.RemoveClientOptionalScopes(ctx, realm, clientID, []ClientScopeRepresentation{{ID: "s1"}})
			},
		},

		// Client role mappings for a user.
		"AddUserClientRoleMappings": {
			wantMethod: http.MethodPost, wantPath: "/admin/realms/master/users/uid-1/role-mappings/clients/cid-1",
			body: `[{"id":"r1"}]`, call: func(ctx context.Context, c *keycloakClient) error {
				return c.AddUserClientRoleMappings(ctx, realm, userID, clientID, []RoleRepresentation{{ID: "r1"}})
			},
		},
		"RemoveUserClientRoleMappings": {
			wantMethod: http.MethodDelete, wantPath: "/admin/realms/master/users/uid-1/role-mappings/clients/cid-1",
			body: `[{"id":"r1"}]`, call: func(ctx context.Context, c *keycloakClient) error {
				return c.RemoveUserClientRoleMappings(ctx, realm, userID, clientID, []RoleRepresentation{{ID: "r1"}})
			},
		},

		// Identity providers.
		"CreateIdentityProvider": {
			wantMethod: http.MethodPost, wantPath: "/admin/realms/master/identity-provider/instances",
			body: `{"alias":"google"}`, call: func(ctx context.Context, c *keycloakClient) error {
				_, err := c.CreateIdentityProvider(ctx, realm, &IdentityProviderRepresentation{Alias: "google"})
				return err
			},
		},
		"UpdateIdentityProvider": {
			wantMethod: http.MethodPut, wantPath: "/admin/realms/master/identity-provider/instances/google",
			body: `{}`, call: func(ctx context.Context, c *keycloakClient) error {
				return c.UpdateIdentityProvider(ctx, realm, "google", &IdentityProviderRepresentation{Alias: "google"})
			},
		},
		"DeleteIdentityProvider": {
			wantMethod: http.MethodDelete, wantPath: "/admin/realms/master/identity-provider/instances/google",
			call: func(ctx context.Context, c *keycloakClient) error {
				return c.DeleteIdentityProvider(ctx, realm, "google")
			},
		},

		// Authentication flows.
		"CreateAuthenticationFlow": {
			wantMethod: http.MethodPost, wantPath: "/admin/realms/master/authentication/flows",
			body: `{"alias":"f"}`, call: func(ctx context.Context, c *keycloakClient) error {
				_, err := c.CreateAuthenticationFlow(ctx, realm, &AuthenticationFlowRepresentation{Alias: "f"})
				return err
			},
		},
		"UpdateAuthenticationFlow": {
			wantMethod: http.MethodPut, wantPath: "/admin/realms/master/authentication/flows/f",
			body: `{}`, call: func(ctx context.Context, c *keycloakClient) error {
				return c.UpdateAuthenticationFlow(ctx, realm, "f", &AuthenticationFlowRepresentation{Alias: "f"})
			},
		},
		"DeleteAuthenticationFlow": {
			wantMethod: http.MethodDelete, wantPath: "/admin/realms/master/authentication/flows/f",
			call: func(ctx context.Context, c *keycloakClient) error {
				return c.DeleteAuthenticationFlow(ctx, realm, "f")
			},
		},

		// Authorization policies and resources.
		"CreateAuthorizationPolicy": {
			wantMethod: http.MethodPost, wantPath: "/admin/realms/master/clients/cid-1/authz/resource-server/policy",
			body: `{"name":"p"}`, call: func(ctx context.Context, c *keycloakClient) error {
				_, err := c.CreateAuthorizationPolicy(ctx, realm, clientID, &AuthorizationPolicyRepresentation{Name: "p"})
				return err
			},
		},
		"UpdateAuthorizationPolicy": {
			wantMethod: http.MethodPut, wantPath: "/admin/realms/master/clients/cid-1/authz/resource-server/policy/p",
			body: `{}`, call: func(ctx context.Context, c *keycloakClient) error {
				return c.UpdateAuthorizationPolicy(ctx, realm, clientID, "p", &AuthorizationPolicyRepresentation{Name: "p"})
			},
		},
		"DeleteAuthorizationPolicy": {
			wantMethod: http.MethodDelete, wantPath: "/admin/realms/master/clients/cid-1/authz/resource-server/policy/p",
			call: func(ctx context.Context, c *keycloakClient) error {
				return c.DeleteAuthorizationPolicy(ctx, realm, clientID, "p")
			},
		},
		"CreateAuthzResource": {
			wantMethod: http.MethodPost, wantPath: "/admin/realms/master/clients/cid-1/authz/resource",
			body: `{"name":"orders"}`, call: func(ctx context.Context, c *keycloakClient) error {
				_, err := c.CreateAuthzResource(ctx, realm, clientID, &AuthzResourceRepresentation{Name: "orders"})
				return err
			},
		},
		"UpdateAuthzResource": {
			wantMethod: http.MethodPut, wantPath: "/admin/realms/master/clients/cid-1/authz/resource/orders",
			body: `{}`, call: func(ctx context.Context, c *keycloakClient) error {
				return c.UpdateAuthzResource(ctx, realm, clientID, "orders", &AuthzResourceRepresentation{Name: "orders"})
			},
		},
		"DeleteAuthzResource": {
			wantMethod: http.MethodDelete, wantPath: "/admin/realms/master/clients/cid-1/authz/resource/orders",
			call: func(ctx context.Context, c *keycloakClient) error {
				return c.DeleteAuthzResource(ctx, realm, clientID, "orders")
			},
		},

		// Components.
		"CreateComponent": {
			wantMethod: http.MethodPost, wantPath: "/admin/realms/master/components",
			body: `{"id":"comp-1"}`, call: func(ctx context.Context, c *keycloakClient) error {
				_, err := c.CreateComponent(ctx, realm, &ComponentRepresentation{Name: "c"})
				return err
			},
		},
		"UpdateComponent": {
			wantMethod: http.MethodPut, wantPath: "/admin/realms/master/components/comp-1",
			body: `{}`, call: func(ctx context.Context, c *keycloakClient) error {
				return c.UpdateComponent(ctx, realm, "comp-1", &ComponentRepresentation{Name: "c"})
			},
		},
		"DeleteComponent": {
			wantMethod: http.MethodDelete, wantPath: "/admin/realms/master/components/comp-1",
			call: func(ctx context.Context, c *keycloakClient) error {
				return c.DeleteComponent(ctx, realm, "comp-1")
			},
		},

		// User federation providers.
		"CreateUserFederationProvider": {
			wantMethod: http.MethodPost, wantPath: "/admin/realms/master/user-federation/instances",
			body: `{"name":"ldap"}`, call: func(ctx context.Context, c *keycloakClient) error {
				_, err := c.CreateUserFederationProvider(ctx, realm, &UserFederationProviderRepresentation{Name: "ldap"})
				return err
			},
		},
		"UpdateUserFederationProvider": {
			wantMethod: http.MethodPut, wantPath: "/admin/realms/master/user-federation/instances/ldap",
			body: `{}`, call: func(ctx context.Context, c *keycloakClient) error {
				return c.UpdateUserFederationProvider(ctx, realm, "ldap", &UserFederationProviderRepresentation{Name: "ldap"})
			},
		},
		"DeleteUserFederationProvider": {
			wantMethod: http.MethodDelete, wantPath: "/admin/realms/master/user-federation/instances/ldap",
			call: func(ctx context.Context, c *keycloakClient) error {
				return c.DeleteUserFederationProvider(ctx, realm, "ldap")
			},
		},

		// Client lifecycle.
		"UpdateClient": {
			wantMethod: http.MethodPut, wantPath: "/admin/realms/master/clients/cid-1",
			body: `{}`, call: func(ctx context.Context, c *keycloakClient) error {
				return c.UpdateClient(ctx, realm, &ClientRepresentation{ID: clientID})
			},
		},
		"UpdateRealmEventsConfig": {
			wantMethod: http.MethodPut, wantPath: "/admin/realms/master/events/config",
			body: `{}`, call: func(ctx context.Context, c *keycloakClient) error {
				return c.UpdateRealmEventsConfig(ctx, realm, &RealmEventsConfigRepresentation{})
			},
		},
		"CreateClientScope": {
			wantMethod: http.MethodPost, wantPath: "/admin/realms/master/client-scopes",
			body: `{"name":"s"}`, call: func(ctx context.Context, c *keycloakClient) error {
				return c.CreateClientScope(ctx, realm, ClientScopeRepresentation{Name: "s"})
			},
		},
		"UpdateClientScope": {
			wantMethod: http.MethodPut, wantPath: "/admin/realms/master/client-scopes/s",
			body: `{}`, call: func(ctx context.Context, c *keycloakClient) error {
				return c.UpdateClientScope(ctx, realm, ClientScopeRepresentation{Name: "s", ID: "s"})
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var gotMethod, gotPath string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotMethod = r.Method
				gotPath = r.URL.Path
				w.Header().Set("Content-Type", "application/json")
				if tc.body != "" {
					_, _ = w.Write([]byte(tc.body))
				} else {
					w.WriteHeader(http.StatusNoContent)
				}
			}))
			defer srv.Close()

			kc := newTestKeycloakClient(srv.Client(), srv.URL, "tok")
			if err := tc.call(context.Background(), kc); err != nil {
				t.Fatalf("%s failed: %v", name, err)
			}
			if gotMethod != tc.wantMethod {
				t.Errorf("method = %s, want %s", gotMethod, tc.wantMethod)
			}
			if gotPath != tc.wantPath {
				t.Errorf("path = %s, want %s", gotPath, tc.wantPath)
			}
		})
	}

	// Guard against the table silently matching nothing, which would make the
	// whole test vacuous.
	if len(cases) < 30 {
		t.Errorf("table has only %d cases; expected the full write-wrapper set", len(cases))
	}
	_ = noop
}

// TestResetClientSecretMethodIsUnverified records a discrepancy found while
// writing the table above, rather than quietly asserting either behaviour.
//
// ResetClientSecret sends PUT to /admin/realms/{realm}/clients/{uuid}/client-secret.
// The Keycloak Admin REST API documents only GET (read the secret) and POST
// (regenerate it) on that path - there is no PUT handler, so this would be
// rejected as 405 against a real Keycloak.
//
// It has no production callers, so nothing is broken today; this is dead code
// with a latent defect. The test pins the current behaviour so that changing it
// is a deliberate act, and the open question is whether to fix the method,
// delete the method, or leave it.
func TestResetClientSecretMethodIsUnverified(t *testing.T) {
	var gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	kc := newTestKeycloakClient(srv.Client(), srv.URL, "tok")
	if err := kc.ResetClientSecret(context.Background(), "master", "cid-1", "s3cr3t"); err != nil {
		t.Fatalf("ResetClientSecret failed: %v", err)
	}

	if gotMethod != http.MethodPut {
		t.Errorf("method = %s; Keycloak documents POST for regenerating a secret, "+
			"so either this has been corrected deliberately or the expectation in this test is stale", gotMethod)
	}
}
