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

// The read wrappers are uniform: build a path, call doRequest, unmarshal. They
// are individually trivial, but a wrong path is silent - the call 404s, the
// resource reads as absent, and the reconciler creates a duplicate or reports
// the object gone. Nothing else in the suite checks the URLs.
//
// The expected paths below come from the Keycloak Admin REST API, not from this
// code: asserting a path against the implementation that builds it would only
// prove the two agree with each other.
type wrapperCase struct {
	body     string
	wantPath string
	// wantQuery is checked only when non-empty, since several endpoints use
	// query parameters rather than path segments.
	wantQuery map[string]string
	call      func(context.Context, *keycloakClient) (int, error)
}

func TestReadWrapperPaths(t *testing.T) {
	const (
		realm     = "master"
		clientID  = "cid-1"
		userID    = "uid-1"
		groupID   = "gid-1"
		mapperID  = "mid-1"
		alias     = "my-idp"
		flowAlias = "my-flow"
		compID    = "comp-1"
	)

	listBody := func(field, value string) string {
		return `[{"` + field + `":"` + value + `"}]`
	}
	objBody := func(field, value string) string {
		return `{"` + field + `":"` + value + `"}`
	}

	cases := map[string]wrapperCase{
		"ListClients": {
			body: listBody("clientId", "c1"), wantPath: "/admin/realms/master/clients",
			call: func(ctx context.Context, c *keycloakClient) (int, error) {
				r, err := c.ListClients(ctx, realm)
				return len(r), err
			},
		},
		"GetClient": {
			// Keycloak addresses a client by internal UUID on /clients/{id}, but
			// this wrapper looks it up by its configured clientId, which the admin
			// console also does - via the list endpoint with a filter.
			body: listBody("id", clientID), wantPath: "/admin/realms/master/clients",
			wantQuery: map[string]string{"clientId": clientID},
			call: func(ctx context.Context, c *keycloakClient) (int, error) {
				r, err := c.GetClient(ctx, realm, clientID)
				if err != nil || r == nil {
					return 0, err
				}
				return 1, nil
			},
		},
		"ListUsers": {
			body: listBody("username", "alice"), wantPath: "/admin/realms/master/users",
			call: func(ctx context.Context, c *keycloakClient) (int, error) {
				r, err := c.ListUsers(ctx, realm)
				return len(r), err
			},
		},
		"GetUser": {
			// Also a list-with-filter: this wrapper is given a username, not the
			// internal UUID that /users/{id} requires.
			body:      listBody("username", "alice"),
			wantPath:  "/admin/realms/master/users",
			wantQuery: map[string]string{"username": "alice"},
			call: func(ctx context.Context, c *keycloakClient) (int, error) {
				r, err := c.GetUser(ctx, realm, "alice")
				if err != nil || r == nil {
					return 0, err
				}
				return 1, nil
			},
		},
		"SearchUsers": {
			body: listBody("username", "ali"), wantPath: "/admin/realms/master/users",
			wantQuery: map[string]string{"username": "ali"},
			call: func(ctx context.Context, c *keycloakClient) (int, error) {
				r, err := c.SearchUsers(ctx, realm, "ali")
				return len(r), err
			},
		},
		"ListGroups": {
			body: listBody("name", "g1"), wantPath: "/admin/realms/master/groups",
			call: func(ctx context.Context, c *keycloakClient) (int, error) {
				r, err := c.ListGroups(ctx, realm)
				return len(r), err
			},
		},
		"GetGroup": {
			body: objBody("id", groupID), wantPath: "/admin/realms/master/groups/" + groupID,
			call: func(ctx context.Context, c *keycloakClient) (int, error) {
				r, err := c.GetGroup(ctx, realm, groupID)
				if err != nil || r == nil {
					return 0, err
				}
				return 1, nil
			},
		},
		"SearchGroups": {
			body: listBody("name", "gr"), wantPath: "/admin/realms/master/groups",
			wantQuery: map[string]string{"search": "gr"},
			call: func(ctx context.Context, c *keycloakClient) (int, error) {
				r, err := c.SearchGroups(ctx, realm, "gr")
				return len(r), err
			},
		},
		"GetUserGroups": {
			body: listBody("name", "g1"), wantPath: "/admin/realms/master/users/" + userID + "/groups",
			call: func(ctx context.Context, c *keycloakClient) (int, error) {
				r, err := c.GetUserGroups(ctx, realm, userID)
				return len(r), err
			},
		},
		"GetRealmKeys": {
			body:     `{"active":{},"keys":[{"kid":"k1"}]}`,
			wantPath: "/admin/realms/master/keys",
			call: func(ctx context.Context, c *keycloakClient) (int, error) {
				r, err := c.GetRealmKeys(ctx, realm)
				if err != nil || r == nil {
					return 0, err
				}
				return len(r.Keys), nil
			},
		},
		"GetRealmEventsConfig": {
			body: `{"eventsEnabled":true}`, wantPath: "/admin/realms/master/events/config",
			call: func(ctx context.Context, c *keycloakClient) (int, error) {
				r, err := c.GetRealmEventsConfig(ctx, realm)
				if err != nil || r == nil {
					return 0, err
				}
				return 1, nil
			},
		},
		"GetRealmRole": {
			body: objBody("name", "admin"), wantPath: "/admin/realms/master/roles/admin",
			call: func(ctx context.Context, c *keycloakClient) (int, error) {
				r, err := c.GetRealmRole(ctx, realm, "admin")
				if err != nil || r == nil {
					return 0, err
				}
				return 1, nil
			},
		},
		"GetClientRole": {
			body: objBody("name", "viewer"), wantPath: "/admin/realms/master/clients/" + clientID + "/roles/viewer",
			call: func(ctx context.Context, c *keycloakClient) (int, error) {
				r, err := c.GetClientRole(ctx, realm, clientID, "viewer")
				if err != nil || r == nil {
					return 0, err
				}
				return 1, nil
			},
		},
		"ListUserClientRoleMappings": {
			body:     listBody("name", "admin"),
			wantPath: "/admin/realms/master/users/" + userID + "/role-mappings/clients/" + clientID,
			call: func(ctx context.Context, c *keycloakClient) (int, error) {
				r, err := c.ListUserClientRoleMappings(ctx, realm, userID, clientID)
				return len(r), err
			},
		},
		"ListClientScopes": {
			body: listBody("name", "s1"), wantPath: "/admin/realms/master/client-scopes",
			call: func(ctx context.Context, c *keycloakClient) (int, error) {
				r, err := c.ListClientScopes(ctx, realm)
				return len(r), err
			},
		},
		"GetClientScope": {
			// /client-scopes/{id} needs the internal UUID, so this wrapper lists
			// and filters by name.
			body: listBody("name", "s1"), wantPath: "/admin/realms/master/client-scopes",
			call: func(ctx context.Context, c *keycloakClient) (int, error) {
				r, err := c.GetClientScope(ctx, realm, "s1")
				if err != nil || r == nil {
					return 0, err
				}
				return 1, nil
			},
		},
		"ListClientDefaultScopes": {
			body:     listBody("name", "s1"),
			wantPath: "/admin/realms/master/clients/" + clientID + "/default-client-scopes",
			call: func(ctx context.Context, c *keycloakClient) (int, error) {
				r, err := c.ListClientDefaultScopes(ctx, realm, clientID)
				return len(r), err
			},
		},
		"ListClientOptionalScopes": {
			body:     listBody("name", "s1"),
			wantPath: "/admin/realms/master/clients/" + clientID + "/optional-client-scopes",
			call: func(ctx context.Context, c *keycloakClient) (int, error) {
				r, err := c.ListClientOptionalScopes(ctx, realm, clientID)
				return len(r), err
			},
		},
		"ListClientScopeMappings": {
			body: listBody("name", "s1"),
			// The realm-level scope mappings, as opposed to the
			// /scope-mappings/clients/{id} variant for a single client.
			wantPath: "/admin/realms/master/clients/" + clientID + "/scope-mappings/realm",
			call: func(ctx context.Context, c *keycloakClient) (int, error) {
				r, err := c.ListClientScopeMappings(ctx, realm, clientID)
				return len(r), err
			},
		},
		"ListClientProtocolMappers": {
			body:     listBody("id", mapperID),
			wantPath: "/admin/realms/master/clients/" + clientID + "/protocol-mappers/models",
			call: func(ctx context.Context, c *keycloakClient) (int, error) {
				r, err := c.ListClientProtocolMappers(ctx, realm, clientID)
				return len(r), err
			},
		},
		"GetClientProtocolMapper": {
			body:     objBody("id", mapperID),
			wantPath: "/admin/realms/master/clients/" + clientID + "/protocol-mappers/models/" + mapperID,
			call: func(ctx context.Context, c *keycloakClient) (int, error) {
				r, err := c.GetClientProtocolMapper(ctx, realm, clientID, mapperID)
				if err != nil || r == nil {
					return 0, err
				}
				return 1, nil
			},
		},
		"ListIdentityProviders": {
			body: listBody("alias", alias), wantPath: "/admin/realms/master/identity-provider/instances",
			call: func(ctx context.Context, c *keycloakClient) (int, error) {
				r, err := c.ListIdentityProviders(ctx, realm)
				return len(r), err
			},
		},
		"GetIdentityProvider": {
			body:     objBody("alias", alias),
			wantPath: "/admin/realms/master/identity-provider/instances/" + alias,
			call: func(ctx context.Context, c *keycloakClient) (int, error) {
				r, err := c.GetIdentityProvider(ctx, realm, alias)
				if err != nil || r == nil {
					return 0, err
				}
				return 1, nil
			},
		},
		"ListUserFederationProviders": {
			body: listBody("name", "ldap"), wantPath: "/admin/realms/master/user-federation/instances",
			call: func(ctx context.Context, c *keycloakClient) (int, error) {
				r, err := c.ListUserFederationProviders(ctx, realm)
				return len(r), err
			},
		},
		"GetUserFederationProvider": {
			body: objBody("id", compID), wantPath: "/admin/realms/master/user-federation/instances/" + compID,
			call: func(ctx context.Context, c *keycloakClient) (int, error) {
				r, err := c.GetUserFederationProvider(ctx, realm, compID)
				if err != nil || r == nil {
					return 0, err
				}
				return 1, nil
			},
		},
		"GetComponent": {
			body: objBody("id", compID), wantPath: "/admin/realms/master/components/" + compID,
			call: func(ctx context.Context, c *keycloakClient) (int, error) {
				r, err := c.GetComponent(ctx, realm, compID)
				if err != nil || r == nil {
					return 0, err
				}
				return 1, nil
			},
		},
		"ListComponentsByType": {
			body:     listBody("id", compID),
			wantPath: "/admin/realms/master/components",
			wantQuery: map[string]string{
				"type": "org.keycloak.keys.KeyProvider",
				"name": "rsa-generated",
			},
			// name is appended only when non-empty.
			call: func(ctx context.Context, c *keycloakClient) (int, error) {
				r, err := c.ListComponentsByType(ctx, realm, "org.keycloak.keys.KeyProvider", "rsa-generated")
				return len(r), err
			},
		},
		"ListAuthzResources": {
			body:     listBody("name", "orders"),
			wantPath: "/admin/realms/master/clients/" + clientID + "/authz/resource",
			call: func(ctx context.Context, c *keycloakClient) (int, error) {
				r, err := c.ListAuthzResources(ctx, realm, clientID)
				return len(r), err
			},
		},
		"GetAuthzResource": {
			body:     objBody("name", "orders"),
			wantPath: "/admin/realms/master/clients/" + clientID + "/authz/resource/" + compID,
			call: func(ctx context.Context, c *keycloakClient) (int, error) {
				r, err := c.GetAuthzResource(ctx, realm, clientID, compID)
				if err != nil || r == nil {
					return 0, err
				}
				return 1, nil
			},
		},
		"ListAuthorizationPolicies": {
			body:     listBody("name", "only-admin"),
			wantPath: "/admin/realms/master/clients/" + clientID + "/authz/resource-server/policy",
			call: func(ctx context.Context, c *keycloakClient) (int, error) {
				r, err := c.ListAuthorizationPolicies(ctx, realm, clientID)
				return len(r), err
			},
		},
		"GetAuthorizationPolicy": {
			body:     objBody("name", "only-admin"),
			wantPath: "/admin/realms/master/clients/" + clientID + "/authz/resource-server/policy/" + compID,
			call: func(ctx context.Context, c *keycloakClient) (int, error) {
				r, err := c.GetAuthorizationPolicy(ctx, realm, clientID, compID)
				if err != nil || r == nil {
					return 0, err
				}
				return 1, nil
			},
		},
		"ListAuthenticationFlows": {
			body: listBody("alias", flowAlias), wantPath: "/admin/realms/master/authentication/flows",
			call: func(ctx context.Context, c *keycloakClient) (int, error) {
				r, err := c.ListAuthenticationFlows(ctx, realm)
				return len(r), err
			},
		},
		"GetAuthenticationFlow": {
			body:     objBody("alias", flowAlias),
			wantPath: "/admin/realms/master/authentication/flows/" + flowAlias,
			call: func(ctx context.Context, c *keycloakClient) (int, error) {
				r, err := c.GetAuthenticationFlow(ctx, realm, flowAlias)
				if err != nil || r == nil {
					return 0, err
				}
				return 1, nil
			},
		},
		"ListClientCertificates": {
			body:     listBody("id", "cert-1"),
			wantPath: "/admin/realms/master/clients/" + clientID + "/certificates",
			call: func(ctx context.Context, c *keycloakClient) (int, error) {
				r, err := c.ListClientCertificates(ctx, realm, clientID)
				return len(r), err
			},
		},
		"GetClientCertificate": {
			body:     objBody("id", "cert-1"),
			wantPath: "/admin/realms/master/clients/" + clientID + "/certificates/cert-1",
			call: func(ctx context.Context, c *keycloakClient) (int, error) {
				r, err := c.GetClientCertificate(ctx, realm, clientID, "cert-1")
				if err != nil || r == nil {
					return 0, err
				}
				return 1, nil
			},
		},
		"ListClientInitialAccess": {
			body: listBody("id", "cia-1"), wantPath: "/admin/realms/master/clients-initial-access",
			call: func(ctx context.Context, c *keycloakClient) (int, error) {
				r, err := c.ListClientInitialAccess(ctx, realm)
				return len(r), err
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var gotMethod, gotPath string
			gotQuery := map[string]string{}
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotMethod = r.Method
				gotPath = r.URL.Path
				for k, v := range r.URL.Query() {
					gotQuery[k] = v[0]
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			kc := newTestKeycloakClient(srv.Client(), srv.URL, "tok")
			got, err := tc.call(context.Background(), kc)
			if err != nil {
				t.Fatalf("%s failed: %v", name, err)
			}
			if got < 1 {
				t.Fatalf("%s returned nothing for a non-empty response body %s", name, tc.body)
			}
			if gotMethod != http.MethodGet {
				t.Errorf("method = %s, want GET", gotMethod)
			}
			if gotPath != tc.wantPath {
				t.Errorf("path = %s, want %s", gotPath, tc.wantPath)
			}
			for k, want := range tc.wantQuery {
				if gotQuery[k] != want {
					t.Errorf("query %s = %q, want %q (full query: %v)", k, gotQuery[k], want, gotQuery)
				}
			}
		})
	}
}
