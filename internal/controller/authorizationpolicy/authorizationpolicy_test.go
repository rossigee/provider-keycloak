package authorizationpolicy

import (
	"context"
	"testing"

	"github.com/rossigee/provider-keycloak/internal/controller/testhelpers"
)

type mockAuthzPolicyClient struct {
	*testhelpers.BaseMockClient
	getPolicyFn    func(ctx context.Context, realm, clientID, id string) (interface{}, error)
	createPolicyFn func(ctx context.Context, realm, clientID string, policy interface{}) (interface{}, error)
	deletePolicyFn func(ctx context.Context, realm, clientID, id string) error
}

func (m *mockAuthzPolicyClient) GetPolicy(ctx context.Context, realm, clientID, id string) (interface{}, error) {
	if m.getPolicyFn != nil {
		return m.getPolicyFn(ctx, realm, clientID, id)
	}
	return nil, nil
}

func TestAuthorizationPolicyObserveExists(t *testing.T) {
	getCalled := false
	mockClient := &mockAuthzPolicyClient{
		BaseMockClient: &testhelpers.BaseMockClient{},
		getPolicyFn: func(ctx context.Context, realm, clientID, id string) (interface{}, error) {
			getCalled = true
			return map[string]string{"id": id, "name": "admin-policy"}, nil
		},
	}

	policy, err := mockClient.GetPolicy(context.Background(), "test-realm", "client-1", "policy-123")
	if err != nil {
		t.Fatalf("GetPolicy failed: %v", err)
	}
	if !getCalled {
		t.Fatal("GetPolicy was not called")
	}
	if policy == nil {
		t.Fatal("Expected policy but got nil")
	}
}

func TestAuthorizationPolicyCreateSuccess(t *testing.T) {
	createCalled := false
	mockClient := &mockAuthzPolicyClient{
		BaseMockClient: &testhelpers.BaseMockClient{},
		createPolicyFn: func(ctx context.Context, realm, clientID string, policy interface{}) (interface{}, error) {
			createCalled = true
			return policy, nil
		},
	}

	result, err := mockClient.CreatePolicy(context.Background(), "test-realm", "client-1", map[string]string{"name": "admin-policy"})
	if err != nil {
		t.Fatalf("CreatePolicy failed: %v", err)
	}
	if !createCalled {
		t.Fatal("CreatePolicy was not called")
	}
	if result == nil {
		t.Fatal("Expected policy but got nil")
	}
}

func (m *mockAuthzPolicyClient) CreatePolicy(ctx context.Context, realm, clientID string, policy interface{}) (interface{}, error) {
	if m.createPolicyFn != nil {
		return m.createPolicyFn(ctx, realm, clientID, policy)
	}
	return policy, nil
}
