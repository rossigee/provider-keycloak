package clientscopemapping

import (
	"context"
	"testing"

	"github.com/rossigee/provider-keycloak/internal/controller/testhelpers"
)

type mockClientScopeMappingClient struct {
	*testhelpers.BaseMockClient
	addClientScopeToClientFn    func(ctx context.Context, realm, clientID, scopeID string) error
	removeClientScopeFromClientFn func(ctx context.Context, realm, clientID, scopeID string) error
	getClientScopesMappingFn    func(ctx context.Context, realm, clientID string) (interface{}, error)
}

func (m *mockClientScopeMappingClient) AddClientScopeToClient(ctx context.Context, realm, clientID, scopeID string) error {
	if m.addClientScopeToClientFn != nil {
		return m.addClientScopeToClientFn(ctx, realm, clientID, scopeID)
	}
	return nil
}

func (m *mockClientScopeMappingClient) RemoveClientScopeFromClient(ctx context.Context, realm, clientID, scopeID string) error {
	if m.removeClientScopeFromClientFn != nil {
		return m.removeClientScopeFromClientFn(ctx, realm, clientID, scopeID)
	}
	return nil
}

func (m *mockClientScopeMappingClient) GetClientScopesMapping(ctx context.Context, realm, clientID string) (interface{}, error) {
	if m.getClientScopesMappingFn != nil {
		return m.getClientScopesMappingFn(ctx, realm, clientID)
	}
	return nil, nil
}

func TestClientScopeMappingAddScope(t *testing.T) {
	addCalled := false
	mockClient := &mockClientScopeMappingClient{
		BaseMockClient: &testhelpers.BaseMockClient{},
		addClientScopeToClientFn: func(ctx context.Context, realm, clientID, scopeID string) error {
			addCalled = true
			return nil
		},
	}

	err := mockClient.AddClientScopeToClient(context.Background(), "test-realm", "client-1", "scope-1")
	if err != nil {
		t.Fatalf("AddClientScopeToClient failed: %v", err)
	}
	if !addCalled {
		t.Fatal("AddClientScopeToClient was not called")
	}
}

func TestClientScopeMappingRemoveScope(t *testing.T) {
	removeCalled := false
	mockClient := &mockClientScopeMappingClient{
		BaseMockClient: &testhelpers.BaseMockClient{},
		removeClientScopeFromClientFn: func(ctx context.Context, realm, clientID, scopeID string) error {
			removeCalled = true
			return nil
		},
	}

	err := mockClient.RemoveClientScopeFromClient(context.Background(), "test-realm", "client-1", "scope-1")
	if err != nil {
		t.Fatalf("RemoveClientScopeFromClient failed: %v", err)
	}
	if !removeCalled {
		t.Fatal("RemoveClientScopeFromClient was not called")
	}
}
