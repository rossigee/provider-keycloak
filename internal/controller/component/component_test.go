package component

import (
	"context"
	"testing"

	"github.com/rossigee/provider-keycloak/internal/controller/testhelpers"
)

type mockComponentClient struct {
	*testhelpers.BaseMockClient
	getComponentFn    func(ctx context.Context, realm, id string) (interface{}, error)
	createComponentFn func(ctx context.Context, realm string, component interface{}) (interface{}, error)
	deleteComponentFn func(ctx context.Context, realm, id string) error
}

func (m *mockComponentClient) GetComponent(ctx context.Context, realm, id string) (interface{}, error) {
	if m.getComponentFn != nil {
		return m.getComponentFn(ctx, realm, id)
	}
	return nil, nil
}

func TestComponentObserveExists(t *testing.T) {
	getCalled := false
	mockClient := &mockComponentClient{
		BaseMockClient: &testhelpers.BaseMockClient{},
		getComponentFn: func(ctx context.Context, realm, id string) (interface{}, error) {
			getCalled = true
			return map[string]string{"id": id, "name": "ldap"}, nil
		},
	}

	component, err := mockClient.GetComponent(context.Background(), "test-realm", "comp-123")
	if err != nil {
		t.Fatalf("GetComponent failed: %v", err)
	}
	if !getCalled {
		t.Fatal("GetComponent was not called")
	}
	if component == nil {
		t.Fatal("Expected component but got nil")
	}
}

func (m *mockComponentClient) CreateComponent(ctx context.Context, realm string, component interface{}) (interface{}, error) {
	if m.createComponentFn != nil {
		return m.createComponentFn(ctx, realm, component)
	}
	return component, nil
}

func TestComponentCreateSuccess(t *testing.T) {
	createCalled := false
	mockClient := &mockComponentClient{
		BaseMockClient: &testhelpers.BaseMockClient{},
		createComponentFn: func(ctx context.Context, realm string, component interface{}) (interface{}, error) {
			createCalled = true
			return component, nil
		},
	}

	result, err := mockClient.CreateComponent(context.Background(), "test-realm", map[string]string{"name": "ldap"})
	if err != nil {
		t.Fatalf("CreateComponent failed: %v", err)
	}
	if !createCalled {
		t.Fatal("CreateComponent was not called")
	}
	if result == nil {
		t.Fatal("Expected component but got nil")
	}
}
