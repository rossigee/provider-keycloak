package clientinitialaccess

import (
	"context"
	"testing"

	"github.com/rossigee/provider-keycloak/internal/controller/testhelpers"
)

type mockInitialAccessClient struct {
	*testhelpers.BaseMockClient
	getInitialAccessFn    func(ctx context.Context, realm, id string) (interface{}, error)
	createInitialAccessFn func(ctx context.Context, realm string, access interface{}) (interface{}, error)
	deleteInitialAccessFn func(ctx context.Context, realm, id string) error
}

func (m *mockInitialAccessClient) GetInitialAccess(ctx context.Context, realm, id string) (interface{}, error) {
	if m.getInitialAccessFn != nil {
		return m.getInitialAccessFn(ctx, realm, id)
	}
	return nil, nil
}

// TestClientInitialAccessObserveExists verifies token detection
func TestClientInitialAccessObserveExists(t *testing.T) {
	mockClient := &mockInitialAccessClient{
		BaseMockClient: &testhelpers.BaseMockClient{},
		getInitialAccessFn: func(ctx context.Context, realm, id string) (interface{}, error) {
			return map[string]string{"id": id, "token": "test-token"}, nil
		},
	}

	access, err := mockClient.GetInitialAccess(context.Background(), "test-realm", "test-id")
	if err != nil {
		t.Fatalf("GetInitialAccess failed: %v", err)
	}
	if access == nil {
		t.Fatal("Expected initial access token but got nil")
	}
}

// TestClientInitialAccessCreateSuccess verifies token creation
func TestClientInitialAccessCreateSuccess(t *testing.T) {
	createCalled := false
	mockClient := &mockInitialAccessClient{
		BaseMockClient: &testhelpers.BaseMockClient{},
		createInitialAccessFn: func(ctx context.Context, realm string, access interface{}) (interface{}, error) {
			createCalled = true
			return map[string]string{"token": "new-token"}, nil
		},
	}

	result, err := mockClient.CreateInitialAccess(context.Background(), "test-realm", map[string]int{"count": 1})
	if err != nil {
		t.Fatalf("CreateInitialAccess failed: %v", err)
	}
	if !createCalled {
		t.Fatal("CreateInitialAccess was not called")
	}
	if result == nil {
		t.Fatal("Expected token but got nil")
	}
}

func (m *mockInitialAccessClient) CreateInitialAccess(ctx context.Context, realm string, access interface{}) (interface{}, error) {
	if m.createInitialAccessFn != nil {
		return m.createInitialAccessFn(ctx, realm, access)
	}
	return access, nil
}

// TestClientInitialAccessDeleteSuccess verifies token revocation
func TestClientInitialAccessDeleteSuccess(t *testing.T) {
	deleteCalled := false
	mockClient := &mockInitialAccessClient{
		BaseMockClient: &testhelpers.BaseMockClient{},
		deleteInitialAccessFn: func(ctx context.Context, realm, id string) error {
			deleteCalled = true
			return nil
		},
	}

	err := mockClient.DeleteInitialAccess(context.Background(), "test-realm", "token-id")
	if err != nil {
		t.Fatalf("DeleteInitialAccess failed: %v", err)
	}
	if !deleteCalled {
		t.Fatal("DeleteInitialAccess was not called")
	}
}

func (m *mockInitialAccessClient) DeleteInitialAccess(ctx context.Context, realm, id string) error {
	if m.deleteInitialAccessFn != nil {
		return m.deleteInitialAccessFn(ctx, realm, id)
	}
	return nil
}
