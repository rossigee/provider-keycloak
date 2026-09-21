package identityprovider

import (
	"context"
	"testing"

	"github.com/rossigee/provider-keycloak/internal/clients"
	"github.com/rossigee/provider-keycloak/internal/controller/testhelpers"
)

type mockIdpClient struct {
	*testhelpers.BaseMockClient
	getIdentityProviderFn    func(ctx context.Context, realm, alias string) (*clients.IdentityProviderRepresentation, error)
	createIdentityProviderFn func(ctx context.Context, realm string, idp *clients.IdentityProviderRepresentation) (*clients.IdentityProviderRepresentation, error)
	updateIdentityProviderFn func(ctx context.Context, realm string, idp *clients.IdentityProviderRepresentation) error
	deleteIdentityProviderFn func(ctx context.Context, realm, alias string) error
}

func (m *mockIdpClient) GetIdentityProvider(ctx context.Context, realm, alias string) (*clients.IdentityProviderRepresentation, error) {
	if m.getIdentityProviderFn != nil {
		return m.getIdentityProviderFn(ctx, realm, alias)
	}
	return nil, nil
}

func (m *mockIdpClient) CreateIdentityProvider(ctx context.Context, realm string, idp *clients.IdentityProviderRepresentation) (*clients.IdentityProviderRepresentation, error) {
	if m.createIdentityProviderFn != nil {
		return m.createIdentityProviderFn(ctx, realm, idp)
	}
	return idp, nil
}

func (m *mockIdpClient) UpdateIdentityProvider(ctx context.Context, realm string, idp *clients.IdentityProviderRepresentation) error {
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

func TestIdentityProviderObserveExists(t *testing.T) {
	getCalled := false
	mockClient := &mockIdpClient{
		BaseMockClient: &testhelpers.BaseMockClient{},
		getIdentityProviderFn: func(ctx context.Context, realm, alias string) (*clients.IdentityProviderRepresentation, error) {
			getCalled = true
			return &clients.IdentityProviderRepresentation{
				Alias:       alias,
				DisplayName: "Test IDP",
			}, nil
		},
	}

	idp, err := mockClient.GetIdentityProvider(context.Background(), "test-realm", "oidc-provider")
	if err != nil {
		t.Fatalf("GetIdentityProvider failed: %v", err)
	}
	if !getCalled {
		t.Fatal("GetIdentityProvider was not called")
	}
	if idp == nil {
		t.Fatal("Expected identity provider but got nil")
	}
}

func TestIdentityProviderCreateSuccess(t *testing.T) {
	createCalled := false
	mockClient := &mockIdpClient{
		BaseMockClient: &testhelpers.BaseMockClient{},
		createIdentityProviderFn: func(ctx context.Context, realm string, idp *clients.IdentityProviderRepresentation) (*clients.IdentityProviderRepresentation, error) {
			createCalled = true
			return idp, nil
		},
	}

	idp := &clients.IdentityProviderRepresentation{
		Alias:       "oidc-provider",
		DisplayName: "Test IDP",
	}

	result, err := mockClient.CreateIdentityProvider(context.Background(), "test-realm", idp)
	if err != nil {
		t.Fatalf("CreateIdentityProvider failed: %v", err)
	}
	if !createCalled {
		t.Fatal("CreateIdentityProvider was not called")
	}
	if result == nil {
		t.Fatal("Expected identity provider but got nil")
	}
}

func TestIdentityProviderDeleteSuccess(t *testing.T) {
	deleteCalled := false
	mockClient := &mockIdpClient{
		BaseMockClient: &testhelpers.BaseMockClient{},
		deleteIdentityProviderFn: func(ctx context.Context, realm, alias string) error {
			deleteCalled = true
			return nil
		},
	}

	err := mockClient.DeleteIdentityProvider(context.Background(), "test-realm", "oidc-provider")
	if err != nil {
		t.Fatalf("DeleteIdentityProvider failed: %v", err)
	}
	if !deleteCalled {
		t.Fatal("DeleteIdentityProvider was not called")
	}
}
