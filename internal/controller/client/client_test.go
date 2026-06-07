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

package client

import (
	"context"
	"errors"
	"testing"

	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	openidclientv1alpha1 "github.com/rossigee/provider-keycloak/apis/openidclient/v1alpha1"
	"github.com/rossigee/provider-keycloak/internal/clients"
	"github.com/rossigee/provider-keycloak/internal/controller/testhelpers"
)

const (
	testUUID  = "uuid-1"
	testAppID = "my-app"
	testRealm = "myrealm"
)

// mockClient is a test double for clients.Client.
type mockClient struct {
	*testhelpers.BaseMockClient
	getClientFn    func(ctx context.Context, realm, clientID string) (*clients.ClientRepresentation, error)
	createClientFn func(ctx context.Context, realm string, c *clients.ClientRepresentation) (*clients.ClientRepresentation, error)
	updateClientFn func(ctx context.Context, realm string, c *clients.ClientRepresentation) error
	deleteClientFn func(ctx context.Context, realm, clientID string) error
}

func (m *mockClient) GetClient(ctx context.Context, realm, clientID string) (*clients.ClientRepresentation, error) {
	return m.getClientFn(ctx, realm, clientID)
}
func (m *mockClient) CreateClient(ctx context.Context, realm string, c *clients.ClientRepresentation) (*clients.ClientRepresentation, error) {
	return m.createClientFn(ctx, realm, c)
}
func (m *mockClient) UpdateClient(ctx context.Context, realm string, c *clients.ClientRepresentation) error {
	return m.updateClientFn(ctx, realm, c)
}
func (m *mockClient) DeleteClient(ctx context.Context, realm, clientID string) error {
	return m.deleteClientFn(ctx, realm, clientID)
}

// Unused interface methods — satisfy clients.Client.
func (m *mockClient) GetRealm(ctx context.Context, realm string) (*clients.Realm, error) {
	return nil, nil
}
func (m *mockClient) CreateRealm(ctx context.Context, r *clients.Realm) (*clients.Realm, error) {
	return nil, nil
}
func (m *mockClient) UpdateRealm(ctx context.Context, r *clients.Realm) error { return nil }
func (m *mockClient) DeleteRealm(ctx context.Context, realm string) error     { return nil }
func (m *mockClient) ListClients(ctx context.Context, realm string) ([]clients.ClientRepresentation, error) {
	return nil, nil
}
func (m *mockClient) GetUser(ctx context.Context, realm, username string) (*clients.UserRepresentation, error) {
	return nil, nil
}
func (m *mockClient) CreateUser(ctx context.Context, realm string, u *clients.UserRepresentation) (*clients.UserRepresentation, error) {
	return nil, nil
}
func (m *mockClient) UpdateUser(ctx context.Context, realm string, u *clients.UserRepresentation) error {
	return nil
}
func (m *mockClient) DeleteUser(ctx context.Context, realm, userID string) error { return nil }
func (m *mockClient) ListUsers(ctx context.Context, realm string) ([]clients.UserRepresentation, error) {
	return nil, nil
}
func (m *mockClient) GetGroup(ctx context.Context, realm, groupID string) (*clients.GroupRepresentation, error) {
	return nil, nil
}
func (m *mockClient) CreateGroup(ctx context.Context, realm string, g *clients.GroupRepresentation) (*clients.GroupRepresentation, error) {
	return nil, nil
}
func (m *mockClient) UpdateGroup(ctx context.Context, realm string, g *clients.GroupRepresentation) error {
	return nil
}
func (m *mockClient) DeleteGroup(ctx context.Context, realm, groupID string) error { return nil }
func (m *mockClient) ListGroups(ctx context.Context, realm string) ([]clients.GroupRepresentation, error) {
	return nil, nil
}
func (m *mockClient) SearchGroups(_ context.Context, _, _ string) ([]clients.GroupRepresentation, error) {
	return nil, nil
}
func (m *mockClient) GetClientSecret(_ context.Context, _, _ string) (string, error) { return "", nil }
func (m *mockClient) GetUserGroups(_ context.Context, _, _ string) ([]clients.GroupRepresentation, error) {
	return nil, nil
}
func (m *mockClient) AddUserToGroup(_ context.Context, _, _, _ string) error      { return nil }
func (m *mockClient) RemoveUserFromGroup(_ context.Context, _, _, _ string) error { return nil }
func (m *mockClient) SearchUsers(_ context.Context, _, _ string) ([]clients.UserRepresentation, error) {
	return nil, nil
}
func (m *mockClient) GetRealmRole(_ context.Context, _, _ string) (*clients.RoleRepresentation, error) {
	return nil, nil
}
func (m *mockClient) CreateRealmRole(_ context.Context, _ string, _ *clients.RoleRepresentation) error {
	return nil
}
func (m *mockClient) UpdateRealmRole(_ context.Context, _, _ string, _ *clients.RoleRepresentation) error {
	return nil
}
func (m *mockClient) DeleteRealmRole(_ context.Context, _, _ string) error { return nil }
func (m *mockClient) GetClientProtocolMapper(_ context.Context, _, _, _ string) (*clients.ProtocolMapperRepresentation, error) {
	return nil, nil
}
func (m *mockClient) CreateClientProtocolMapper(_ context.Context, _, _ string, _ *clients.ProtocolMapperRepresentation) (string, error) {
	return "", nil
}
func (m *mockClient) UpdateClientProtocolMapper(_ context.Context, _, _ string, _ *clients.ProtocolMapperRepresentation) error {
	return nil
}
func (m *mockClient) DeleteClientProtocolMapper(_ context.Context, _, _, _ string) error { return nil }
func (m *mockClient) ListClientProtocolMappers(_ context.Context, _, _ string) ([]clients.ProtocolMapperRepresentation, error) {
	return nil, nil
}

// newClient interface methods
func (m *mockClient) GetUserFederationProvider(_ context.Context, _, _ string) (*clients.UserFederationProviderRepresentation, error) {
	return nil, nil
}
func (m *mockClient) CreateUserFederationProvider(_ context.Context, _ string, _ *clients.UserFederationProviderRepresentation) (string, error) {
	return "", nil
}
func (m *mockClient) UpdateUserFederationProvider(_ context.Context, _, _ string, _ *clients.UserFederationProviderRepresentation) error {
	return nil
}
func (m *mockClient) DeleteUserFederationProvider(_ context.Context, _, _ string) error {
	return nil
}
func (m *mockClient) ListUserFederationProviders(_ context.Context, _ string) ([]clients.UserFederationProviderRepresentation, error) {
	return nil, nil
}
func (m *mockClient) GetRealmEventsConfig(_ context.Context, _ string) (*clients.RealmEventsConfigRepresentation, error) {
	return nil, nil
}
func (m *mockClient) UpdateRealmEventsConfig(_ context.Context, _ string, _ *clients.RealmEventsConfigRepresentation) error {
	return nil
}
func (m *mockClient) ImportRealm(_ context.Context, _ string, _ bool) error {
	return nil
}
func (m *mockClient) GetAuthzResource(_ context.Context, _, _, _ string) (*clients.AuthzResourceRepresentation, error) {
	return nil, nil
}
func (m *mockClient) CreateAuthzResource(_ context.Context, _, _ string, _ *clients.AuthzResourceRepresentation) (string, error) {
	return "", nil
}
func (m *mockClient) UpdateAuthzResource(_ context.Context, _, _, _ string, _ *clients.AuthzResourceRepresentation) error {
	return nil
}
func (m *mockClient) DeleteAuthzResource(_ context.Context, _, _, _ string) error {
	return nil
}
func (m *mockClient) ListAuthzResources(_ context.Context, _, _ string) ([]clients.AuthzResourceRepresentation, error) {
	return nil, nil
}
func (m *mockClient) GetClientCertificate(_ context.Context, _, _, _ string) (*clients.ClientCertificateRepresentation, error) {
	return nil, nil
}
func (m *mockClient) GenerateClientCertificate(_ context.Context, _, _ string, _ string) (*clients.ClientCertificateRepresentation, error) {
	return nil, nil
}
func (m *mockClient) ListClientCertificates(_ context.Context, _, _ string) ([]clients.ClientCertificateRepresentation, error) {
	return nil, nil
}

// newCR returns a minimal Client CR for testing.
func newCR(realmId, clientId string) *openidclientv1alpha1.Client {
	cr := &openidclientv1alpha1.Client{
		ObjectMeta: metav1.ObjectMeta{Name: "test-client", Namespace: "default"},
		Spec: openidclientv1alpha1.ClientSpec{
			ForProvider: openidclientv1alpha1.ClientParameters{
				ClientId: clientId,
			},
		},
	}
	if realmId != "" {
		cr.Spec.ForProvider.RealmId = &realmId
	}
	return cr
}

// wrongMG is a non-Client managed resource used to test type assertion failures.
// We use ClientDefaultScopes because it IS a resource.Managed but is not *Client.
type wrongMG = openidclientv1alpha1.ClientDefaultScopes

// =============================================================================
// Observe
// =============================================================================

func TestObserve(t *testing.T) {
	tests := []struct {
		name        string
		mg          resource.Managed
		getClientFn func(ctx context.Context, realm, id string) (*clients.ClientRepresentation, error)
		wantExists  bool
		wantErrStr  string
	}{
		{
			name:       "wrong managed type",
			mg:         &wrongMG{},
			wantErrStr: errNotClient,
		},
		{
			name:       "empty realmId",
			mg:         newCR("", testAppID),
			wantErrStr: "realmId is required",
		},
		{
			name: "GetClient returns error",
			mg:   newCR("myrealm", testAppID),
			getClientFn: func(_ context.Context, _, _ string) (*clients.ClientRepresentation, error) {
				return nil, errors.New("connection refused")
			},
			wantErrStr: "connection refused",
		},
		{
			name: "client not found",
			mg:   newCR("myrealm", testAppID),
			getClientFn: func(_ context.Context, _, _ string) (*clients.ClientRepresentation, error) {
				return nil, nil
			},
			wantExists: false,
		},
		{
			name: "client found",
			mg:   newCR("myrealm", testAppID),
			getClientFn: func(_ context.Context, _, _ string) (*clients.ClientRepresentation, error) {
				return &clients.ClientRepresentation{ID: testUUID, ClientID: testAppID}, nil
			},
			wantExists: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &external{client: &mockClient{getClientFn: tt.getClientFn}}
			obs, err := e.Observe(context.Background(), tt.mg)
			if tt.wantErrStr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErrStr)
				}
				if !containsStr(err.Error(), tt.wantErrStr) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.wantErrStr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if obs.ResourceExists != tt.wantExists {
				t.Errorf("ResourceExists = %v, want %v", obs.ResourceExists, tt.wantExists)
			}
			if tt.wantExists {
				cr := tt.mg.(*openidclientv1alpha1.Client)
				cond := cr.Status.GetCondition(xpv1.TypeReady)
				if cond.Status != corev1.ConditionTrue {
					t.Errorf("expected Ready condition True, got %v", cond.Status)
				}
			}
		})
	}
}

// =============================================================================
// Create
// =============================================================================

func TestCreate(t *testing.T) {
	tests := []struct {
		name           string
		mg             resource.Managed
		createClientFn func(ctx context.Context, realm string, c *clients.ClientRepresentation) (*clients.ClientRepresentation, error)
		wantErrStr     string
	}{
		{
			name:       "wrong managed type",
			mg:         &wrongMG{},
			wantErrStr: errNotClient,
		},
		{
			name:       "empty realmId",
			mg:         newCR("", testAppID),
			wantErrStr: "realmId is required",
		},
		{
			name: "CreateClient error",
			mg:   newCR("myrealm", testAppID),
			createClientFn: func(_ context.Context, _ string, _ *clients.ClientRepresentation) (*clients.ClientRepresentation, error) {
				return nil, errors.New("conflict")
			},
			wantErrStr: "conflict",
		},
		{
			name: "successful create",
			mg:   newCR("myrealm", testAppID),
			createClientFn: func(_ context.Context, _ string, c *clients.ClientRepresentation) (*clients.ClientRepresentation, error) {
				return &clients.ClientRepresentation{ID: "new-uuid", ClientID: c.ClientID}, nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &external{client: &mockClient{createClientFn: tt.createClientFn}}
			creation, err := e.Create(context.Background(), tt.mg)
			if tt.wantErrStr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErrStr)
				}
				if !containsStr(err.Error(), tt.wantErrStr) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.wantErrStr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			_ = creation // ExternalCreation returned on success
		})
	}
}

// =============================================================================
// Update
// =============================================================================

func TestUpdate(t *testing.T) {
	tests := []struct {
		name           string
		mg             resource.Managed
		getClientFn    func(ctx context.Context, realm, id string) (*clients.ClientRepresentation, error)
		updateClientFn func(ctx context.Context, realm string, c *clients.ClientRepresentation) error
		wantErrStr     string
	}{
		{
			name:       "wrong managed type",
			mg:         &wrongMG{},
			wantErrStr: errNotClient,
		},
		{
			name:       "empty realmId",
			mg:         newCR("", testAppID),
			wantErrStr: "realmId is required",
		},
		{
			name: "GetClient error",
			mg:   newCR("myrealm", testAppID),
			getClientFn: func(_ context.Context, _, _ string) (*clients.ClientRepresentation, error) {
				return nil, errors.New("timeout")
			},
			wantErrStr: "timeout",
		},
		{
			name: "client not found",
			mg:   newCR("myrealm", testAppID),
			getClientFn: func(_ context.Context, _, _ string) (*clients.ClientRepresentation, error) {
				return nil, nil
			},
			wantErrStr: errClientNotFound,
		},
		{
			name: "UpdateClient error",
			mg:   newCR("myrealm", testAppID),
			getClientFn: func(_ context.Context, _, _ string) (*clients.ClientRepresentation, error) {
				return &clients.ClientRepresentation{ID: testUUID}, nil
			},
			updateClientFn: func(_ context.Context, _ string, _ *clients.ClientRepresentation) error {
				return errors.New("update failed")
			},
			wantErrStr: "update failed",
		},
		{
			name: "successful update",
			mg:   newCR("myrealm", testAppID),
			getClientFn: func(_ context.Context, _, _ string) (*clients.ClientRepresentation, error) {
				return &clients.ClientRepresentation{ID: testUUID}, nil
			},
			updateClientFn: func(_ context.Context, _ string, c *clients.ClientRepresentation) error {
				if c.ID != testUUID {
					return errors.New("ID not set on update")
				}
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &external{client: &mockClient{
				getClientFn:    tt.getClientFn,
				updateClientFn: tt.updateClientFn,
			}}
			_, err := e.Update(context.Background(), tt.mg)
			if tt.wantErrStr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErrStr)
				}
				if !containsStr(err.Error(), tt.wantErrStr) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.wantErrStr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// =============================================================================
// Delete
// =============================================================================

func TestDelete(t *testing.T) {
	tests := []struct {
		name           string
		mg             resource.Managed
		getClientFn    func(ctx context.Context, realm, id string) (*clients.ClientRepresentation, error)
		deleteClientFn func(ctx context.Context, realm, id string) error
		wantErrStr     string
	}{
		{
			name:       "wrong managed type",
			mg:         &wrongMG{},
			wantErrStr: errNotClient,
		},
		{
			name:       "empty realmId",
			mg:         newCR("", testAppID),
			wantErrStr: "realmId is required",
		},
		{
			name: "GetClient error",
			mg:   newCR("myrealm", testAppID),
			getClientFn: func(_ context.Context, _, _ string) (*clients.ClientRepresentation, error) {
				return nil, errors.New("lookup failed")
			},
			wantErrStr: "lookup failed",
		},
		{
			name: "client already gone",
			mg:   newCR("myrealm", testAppID),
			getClientFn: func(_ context.Context, _, _ string) (*clients.ClientRepresentation, error) {
				return nil, nil
			},
		},
		{
			name: "DeleteClient 404 is not an error",
			mg:   newCR("myrealm", testAppID),
			getClientFn: func(_ context.Context, _, _ string) (*clients.ClientRepresentation, error) {
				return &clients.ClientRepresentation{ID: testUUID}, nil
			},
			deleteClientFn: func(_ context.Context, _, _ string) error {
				return errors.New("request failed with status 404: not found")
			},
		},
		{
			name: "DeleteClient non-404 error",
			mg:   newCR("myrealm", testAppID),
			getClientFn: func(_ context.Context, _, _ string) (*clients.ClientRepresentation, error) {
				return &clients.ClientRepresentation{ID: testUUID}, nil
			},
			deleteClientFn: func(_ context.Context, _, _ string) error {
				return errors.New("request failed with status 500: server error")
			},
			wantErrStr: "500",
		},
		{
			name: "successful deletion",
			mg:   newCR("myrealm", testAppID),
			getClientFn: func(_ context.Context, _, _ string) (*clients.ClientRepresentation, error) {
				return &clients.ClientRepresentation{ID: testUUID}, nil
			},
			deleteClientFn: func(_ context.Context, _, _ string) error { return nil },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &external{client: &mockClient{
				getClientFn:    tt.getClientFn,
				deleteClientFn: tt.deleteClientFn,
			}}
			_, err := e.Delete(context.Background(), tt.mg)
			if tt.wantErrStr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErrStr)
				}
				if !containsStr(err.Error(), tt.wantErrStr) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.wantErrStr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// =============================================================================
// clientParamsToRepresentation
// =============================================================================

func TestClientParamsToRepresentation(t *testing.T) {
	enabled := true
	stdFlow := true
	svcAccts := false
	rootURL := "https://app.example.com"
	name := "My App"

	p := &openidclientv1alpha1.ClientParameters{
		ClientId:               "my-app",
		Enabled:                &enabled,
		StandardFlowEnabled:    &stdFlow,
		ServiceAccountsEnabled: &svcAccts,
		RootUrl:                &rootURL,
		Name:                   &name,
		ValidRedirectUris:      []string{"https://app.example.com/*"},
		WebOrigins:             []string{"+"},
	}

	rep := clientParamsToRepresentation(p)

	if rep.ClientID != "my-app" {
		t.Errorf("ClientID = %q, want %q", rep.ClientID, testAppID)
	}
	if !rep.Enabled {
		t.Error("Enabled should be true")
	}
	if !rep.StandardFlowEnabled {
		t.Error("StandardFlowEnabled should be true")
	}
	if rep.ServiceAccountsEnabled {
		t.Error("ServiceAccountsEnabled should be false")
	}
	if rep.RootURL != rootURL {
		t.Errorf("RootURL = %q, want %q", rep.RootURL, rootURL)
	}
	if rep.Name != name {
		t.Errorf("Name = %q, want %q", rep.Name, name)
	}
	if len(rep.ValidRedirectURIs) != 1 || rep.ValidRedirectURIs[0] != "https://app.example.com/*" {
		t.Errorf("ValidRedirectURIs = %v", rep.ValidRedirectURIs)
	}
	if len(rep.WebOrigins) != 1 || rep.WebOrigins[0] != "+" {
		t.Errorf("WebOrigins = %v", rep.WebOrigins)
	}
}

func TestClientParamsToRepresentationDefaults(t *testing.T) {
	p := &openidclientv1alpha1.ClientParameters{ClientId: "bare"}
	rep := clientParamsToRepresentation(p)
	if !rep.Enabled {
		t.Error("Enabled default should be true")
	}
}

// =============================================================================
// clientUpToDate / drift detection
// =============================================================================

func TestClientUpToDate(t *testing.T) {
	trueVal := true
	falseVal := false
	rootURL := "https://app.example.com"
	otherURL := "https://other.example.com"

	tests := []struct {
		name    string
		desired openidclientv1alpha1.ClientParameters
		actual  clients.ClientRepresentation
		want    bool
	}{
		{
			name:    "no desired overrides — always up to date",
			desired: openidclientv1alpha1.ClientParameters{ClientId: testAppID},
			actual:  clients.ClientRepresentation{ClientID: testAppID, Enabled: true},
			want:    true,
		},
		{
			name:    "enabled matches",
			desired: openidclientv1alpha1.ClientParameters{Enabled: &trueVal},
			actual:  clients.ClientRepresentation{Enabled: true},
			want:    true,
		},
		{
			name:    "enabled drifted",
			desired: openidclientv1alpha1.ClientParameters{Enabled: &trueVal},
			actual:  clients.ClientRepresentation{Enabled: false},
			want:    false,
		},
		{
			name:    "standardFlowEnabled drifted",
			desired: openidclientv1alpha1.ClientParameters{StandardFlowEnabled: &trueVal},
			actual:  clients.ClientRepresentation{StandardFlowEnabled: false},
			want:    false,
		},
		{
			name:    "directAccessGrantsEnabled drifted",
			desired: openidclientv1alpha1.ClientParameters{DirectAccessGrantsEnabled: &falseVal},
			actual:  clients.ClientRepresentation{DirectAccessGrantsEnabled: true},
			want:    false,
		},
		{
			name:    "serviceAccountsEnabled drifted",
			desired: openidclientv1alpha1.ClientParameters{ServiceAccountsEnabled: &trueVal},
			actual:  clients.ClientRepresentation{ServiceAccountsEnabled: false},
			want:    false,
		},
		{
			name:    "rootUrl drifted",
			desired: openidclientv1alpha1.ClientParameters{RootUrl: &rootURL},
			actual:  clients.ClientRepresentation{RootURL: otherURL},
			want:    false,
		},
		{
			name:    "rootUrl matches",
			desired: openidclientv1alpha1.ClientParameters{RootUrl: &rootURL},
			actual:  clients.ClientRepresentation{RootURL: rootURL},
			want:    true,
		},
		{
			name:    "validRedirectUris drifted (different values)",
			desired: openidclientv1alpha1.ClientParameters{ValidRedirectUris: []string{"https://a.com/*"}},
			actual:  clients.ClientRepresentation{ValidRedirectURIs: []string{"https://b.com/*"}},
			want:    false,
		},
		{
			name:    "validRedirectUris matches (order-independent)",
			desired: openidclientv1alpha1.ClientParameters{ValidRedirectUris: []string{"https://b.com/*", "https://a.com/*"}},
			actual:  clients.ClientRepresentation{ValidRedirectURIs: []string{"https://a.com/*", "https://b.com/*"}},
			want:    true,
		},
		{
			name:    "webOrigins drifted",
			desired: openidclientv1alpha1.ClientParameters{WebOrigins: []string{"+"}},
			actual:  clients.ClientRepresentation{WebOrigins: []string{"https://app.example.com"}},
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := clientUpToDate(&tt.desired, &tt.actual)
			if got != tt.want {
				t.Errorf("clientUpToDate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestObserveDriftTriggersUpdate(t *testing.T) {
	enabled := true
	// Keycloak has Enabled=false, spec wants true → drift → ResourceUpToDate:false
	cr := newCR(testRealm, testAppID)
	cr.Spec.ForProvider.Enabled = &enabled

	e := &external{client: &mockClient{
		getClientFn: func(_ context.Context, _, _ string) (*clients.ClientRepresentation, error) {
			return &clients.ClientRepresentation{ID: testUUID, ClientID: testAppID, Enabled: false}, nil
		},
	}}

	obs, err := e.Observe(context.Background(), cr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !obs.ResourceExists {
		t.Error("ResourceExists should be true")
	}
	if obs.ResourceUpToDate {
		t.Error("ResourceUpToDate should be false when drift exists")
	}
}

// =============================================================================
// Helpers
// =============================================================================

func containsStr(s, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// Ensure wrongMG satisfies resource.Managed so the test compiles.
var _ resource.Managed = &wrongMG{}
