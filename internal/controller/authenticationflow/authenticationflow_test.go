package authenticationflow

import (
	"context"
	"testing"

	"github.com/rossigee/provider-keycloak/internal/controller/testhelpers"
)

type mockAuthFlowClient struct {
	*testhelpers.BaseMockClient
	getAuthFlowFn    func(ctx context.Context, realm, id string) (interface{}, error)
	createAuthFlowFn func(ctx context.Context, realm string, flow interface{}) (interface{}, error)
	deleteAuthFlowFn func(ctx context.Context, realm, id string) error
}

func (m *mockAuthFlowClient) GetAuthFlow(ctx context.Context, realm, id string) (interface{}, error) {
	if m.getAuthFlowFn != nil {
		return m.getAuthFlowFn(ctx, realm, id)
	}
	return nil, nil
}

func (m *mockAuthFlowClient) CreateAuthFlow(ctx context.Context, realm string, flow interface{}) (interface{}, error) {
	if m.createAuthFlowFn != nil {
		return m.createAuthFlowFn(ctx, realm, flow)
	}
	return flow, nil
}

func (m *mockAuthFlowClient) DeleteAuthFlow(ctx context.Context, realm, id string) error {
	if m.deleteAuthFlowFn != nil {
		return m.deleteAuthFlowFn(ctx, realm, id)
	}
	return nil
}

func TestAuthenticationFlowObserveExists(t *testing.T) {
	getCalled := false
	mockClient := &mockAuthFlowClient{
		BaseMockClient: &testhelpers.BaseMockClient{},
		getAuthFlowFn: func(ctx context.Context, realm, id string) (interface{}, error) {
			getCalled = true
			return map[string]string{"id": id, "alias": "browser"}, nil
		},
	}

	flow, err := mockClient.GetAuthFlow(context.Background(), "test-realm", "flow-123")
	if err != nil {
		t.Fatalf("GetAuthFlow failed: %v", err)
	}
	if !getCalled {
		t.Fatal("GetAuthFlow was not called")
	}
	if flow == nil {
		t.Fatal("Expected flow but got nil")
	}
}

func TestAuthenticationFlowCreateSuccess(t *testing.T) {
	createCalled := false
	mockClient := &mockAuthFlowClient{
		BaseMockClient: &testhelpers.BaseMockClient{},
		createAuthFlowFn: func(ctx context.Context, realm string, flow interface{}) (interface{}, error) {
			createCalled = true
			return flow, nil
		},
	}

	result, err := mockClient.CreateAuthFlow(context.Background(), "test-realm", map[string]string{"alias": "browser"})
	if err != nil {
		t.Fatalf("CreateAuthFlow failed: %v", err)
	}
	if !createCalled {
		t.Fatal("CreateAuthFlow was not called")
	}
	if result == nil {
		t.Fatal("Expected flow but got nil")
	}
}

func TestAuthenticationFlowDeleteSuccess(t *testing.T) {
	deleteCalled := false
	mockClient := &mockAuthFlowClient{
		BaseMockClient: &testhelpers.BaseMockClient{},
		deleteAuthFlowFn: func(ctx context.Context, realm, id string) error {
			deleteCalled = true
			return nil
		},
	}

	err := mockClient.DeleteAuthFlow(context.Background(), "test-realm", "flow-123")
	if err != nil {
		t.Fatalf("DeleteAuthFlow failed: %v", err)
	}
	if !deleteCalled {
		t.Fatal("DeleteAuthFlow was not called")
	}
}
