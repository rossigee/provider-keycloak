package clientrolemapping

import (
	"context"
	"testing"

	"github.com/rossigee/provider-keycloak/internal/controller/testhelpers"
)

type mockClientRoleMappingClient struct {
	*testhelpers.BaseMockClient
	addClientRoleToUserFn    func(ctx context.Context, realm, userID, clientID, roleID string) error
	removeClientRoleFromUserFn func(ctx context.Context, realm, userID, clientID, roleID string) error
	getUserClientRolesFn     func(ctx context.Context, realm, userID, clientID string) (interface{}, error)
}

func (m *mockClientRoleMappingClient) AddClientRoleToUser(ctx context.Context, realm, userID, clientID, roleID string) error {
	if m.addClientRoleToUserFn != nil {
		return m.addClientRoleToUserFn(ctx, realm, userID, clientID, roleID)
	}
	return nil
}

func (m *mockClientRoleMappingClient) RemoveClientRoleFromUser(ctx context.Context, realm, userID, clientID, roleID string) error {
	if m.removeClientRoleFromUserFn != nil {
		return m.removeClientRoleFromUserFn(ctx, realm, userID, clientID, roleID)
	}
	return nil
}

func (m *mockClientRoleMappingClient) GetUserClientRoles(ctx context.Context, realm, userID, clientID string) (interface{}, error) {
	if m.getUserClientRolesFn != nil {
		return m.getUserClientRolesFn(ctx, realm, userID, clientID)
	}
	return nil, nil
}

func TestClientRoleMappingAddRole(t *testing.T) {
	addCalled := false
	mockClient := &mockClientRoleMappingClient{
		BaseMockClient: &testhelpers.BaseMockClient{},
		addClientRoleToUserFn: func(ctx context.Context, realm, userID, clientID, roleID string) error {
			addCalled = true
			return nil
		},
	}

	err := mockClient.AddClientRoleToUser(context.Background(), "test-realm", "user-1", "client-1", "role-1")
	if err != nil {
		t.Fatalf("AddClientRoleToUser failed: %v", err)
	}
	if !addCalled {
		t.Fatal("AddClientRoleToUser was not called")
	}
}

func TestClientRoleMappingRemoveRole(t *testing.T) {
	removeCalled := false
	mockClient := &mockClientRoleMappingClient{
		BaseMockClient: &testhelpers.BaseMockClient{},
		removeClientRoleFromUserFn: func(ctx context.Context, realm, userID, clientID, roleID string) error {
			removeCalled = true
			return nil
		},
	}

	err := mockClient.RemoveClientRoleFromUser(context.Background(), "test-realm", "user-1", "client-1", "role-1")
	if err != nil {
		t.Fatalf("RemoveClientRoleFromUser failed: %v", err)
	}
	if !removeCalled {
		t.Fatal("RemoveClientRoleFromUser was not called")
	}
}
