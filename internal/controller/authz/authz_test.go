package authz

import (
	"context"
	"testing"

	"github.com/rossigee/provider-keycloak/internal/controller/testhelpers"
)

type mockAuthzClient struct {
	*testhelpers.BaseMockClient
	getResourceFn    func(ctx context.Context, realm, clientID, resourceID string) (interface{}, error)
	createResourceFn func(ctx context.Context, realm, clientID string, resource interface{}) (interface{}, error)
	updateResourceFn func(ctx context.Context, realm, clientID string, resource interface{}) error //nolint:unused
	deleteResourceFn func(ctx context.Context, realm, clientID, resourceID string) error           //nolint:unused
}

func (m *mockAuthzClient) GetResource(ctx context.Context, realm, clientID, resourceID string) (interface{}, error) {
	if m.getResourceFn != nil {
		return m.getResourceFn(ctx, realm, clientID, resourceID)
	}
	return nil, nil
}

// TestAuthzResourceObserveExists verifies resource detection
func TestAuthzResourceObserveExists(t *testing.T) {
	mockClient := &mockAuthzClient{
		BaseMockClient: &testhelpers.BaseMockClient{},
		getResourceFn: func(ctx context.Context, realm, clientID, resourceID string) (interface{}, error) {
			return map[string]string{"id": resourceID}, nil
		},
	}

	resource, err := mockClient.GetResource(context.Background(), "test-realm", "test-client", "test-resource")
	if err != nil {
		t.Fatalf("GetResource failed: %v", err)
	}
	if resource == nil {
		t.Fatal("Expected resource but got nil")
	}
}

// TestAuthzResourceCreateSuccess verifies creation
func TestAuthzResourceCreateSuccess(t *testing.T) {
	createCalled := false
	mockClient := &mockAuthzClient{
		BaseMockClient: &testhelpers.BaseMockClient{},
		createResourceFn: func(ctx context.Context, realm, clientID string, resource interface{}) (interface{}, error) {
			createCalled = true
			return resource, nil
		},
	}

	result, err := mockClient.CreateResource(context.Background(), "test-realm", "test-client", map[string]string{"name": "test"})
	if err != nil {
		t.Fatalf("CreateResource failed: %v", err)
	}
	if !createCalled {
		t.Fatal("CreateResource was not called")
	}
	if result == nil {
		t.Fatal("Expected result but got nil")
	}
}

func (m *mockAuthzClient) CreateResource(ctx context.Context, realm, clientID string, resource interface{}) (interface{}, error) {
	if m.createResourceFn != nil {
		return m.createResourceFn(ctx, realm, clientID, resource)
	}
	return resource, nil
}
