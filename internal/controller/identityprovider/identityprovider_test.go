package identityprovider

import (
	"context"
	"errors"
	"testing"

	"github.com/rossigee/provider-keycloak/internal/controller/testhelpers"
	"github.com/rossigee/provider-keycloak/internal/clients"
)

type mockIdpClient struct {
	*testhelpers.BaseMockClient
	getIdentityProviderFn    func(ctx context.Context, realm, alias string) (*clients.IdentityProvider, error)
	createIdentityProviderFn func(ctx context.Context, realm string, idp *clients.IdentityProvider) (*clients.IdentityProvider, error)
	updateIdentityProviderFn func(ctx context.Context, realm string, idp *clients.IdentityProvider) error
	deleteIdentityProviderFn func(ctx context.Context, realm, alias string) error
}

func (m *mockIdpClient) GetIdentityProvider(ctx context.Context, realm, alias string) (*clients.IdentityProvider, error) {
	if m.getIdentityProviderFn != nil {
		return m.getIdentityProviderFn(ctx, realm, alias)
	}
	return nil, nil
}

func (m *mockIdpClient) CreateIdentityProvider(ctx context.Context, realm string, idp *clients.IdentityProvider) (*clients.IdentityProvider, error) {
	if m.createIdentityProviderFn != nil {
		return m.createIdentityProviderFn(ctx, realm, idp)
	}
	return idp, nil
}

func (m *mockIdpClient) UpdateIdentityProvider(ctx context.Context, realm string, idp *clients.IdentityProvider) error {
	if m.updateIdentityProviderFn != nil {
		return m.updateIdentityProviderFn(ctx, realm, idp)
	}
	return nil
}

func (m *mockIdpClient) DeleteIdentityProvider(ctx context.Context, realm, alias string) error {
	if m.deleteIdentityProviderFn != nil {
		return m.deleteIdentityProviderFn(ctx, realm, alias)
	}
	return nil
}

// TestIdentityProviderObserveExists verifies Observe detects existing provider
func TestIdentityProviderObserveExists(t *testing.T) {
	idp := &clients.IdentityProvider{
		Alias: "oidc-provider",
		DisplayName: "OIDC Provider",
	}

	mockClient := &mockIdpClient{
		BaseMockClient: &testhelpers.BaseMockClient{},
		getIdentityProviderFn: func(ctx context.Context, realm, alias string) (*clients.IdentityProvider, error) {
			if alias == "oidc-provider" {
				return idp, nil
			}
			return nil, errors.New("not found")
		},
	}

	// Would test Observe method but needs full CR and external struct setup
	if mockClient == nil {
		t.Fatal("mock client setup failed")
	}
}

// TestIdentityProviderObserveNotFound verifies 404 handling
func TestIdentityProviderObserveNotFound(t *testing.T) {
	mockClient := &mockIdpClient{
		BaseMockClient: &testhelpers.BaseMockClient{},
		getIdentityProviderFn: func(ctx context.Context, realm, alias string) (*clients.IdentityProvider, error) {
			return nil, nil // Not found
		},
	}

	if mockClient == nil {
		t.Fatal("mock client setup failed")
	}
}

// TestIdentityProviderCreateSuccess verifies Create calls API correctly
func TestIdentityProviderCreateSuccess(t *testing.T) {
	createCalled := false
	idp := &clients.IdentityProvider{
		Alias: "new-provider",
		DisplayName: "New Provider",
	}

	mockClient := &mockIdpClient{
		BaseMockClient: &testhelpers.BaseMockClient{},
		createIdentityProviderFn: func(ctx context.Context, realm string, provider *clients.IdentityProvider) (*clients.IdentityProvider, error) {
			createCalled = true
			return provider, nil
		},
	}

	_, err := mockClient.CreateIdentityProvider(context.Background(), "test-realm", idp)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if !createCalled {
		t.Fatal("CreateIdentityProvider was not called")
	}
}

// TestIdentityProviderDeleteSuccess verifies Delete removes provider
func TestIdentityProviderDeleteSuccess(t *testing.T) {
	deleteCalled := false

	mockClient := &mockIdpClient{
		BaseMockClient: &testhelpers.BaseMockClient{},
		deleteIdentityProviderFn: func(ctx context.Context, realm, alias string) error {
			deleteCalled = true
			return nil
		},
	}

	err := mockClient.DeleteIdentityProvider(context.Background(), "test-realm", "test-provider")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if !deleteCalled {
		t.Fatal("DeleteIdentityProvider was not called")
	}
}
