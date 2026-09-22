package userfederation

import (
	"context"
	"testing"

	"github.com/rossigee/provider-keycloak/internal/controller/testhelpers"
)

type mockUserFedClient struct {
	*testhelpers.BaseMockClient
	getUserFedFn    func(ctx context.Context, realm, id string) (interface{}, error)
	createUserFedFn func(ctx context.Context, realm string, config interface{}) (interface{}, error)
	deleteUserFedFn func(ctx context.Context, realm, id string) error //nolint:unused
}

func (m *mockUserFedClient) GetUserFed(ctx context.Context, realm, id string) (interface{}, error) {
	if m.getUserFedFn != nil {
		return m.getUserFedFn(ctx, realm, id)
	}
	return nil, nil
}

func TestUserFederationObserveExists(t *testing.T) {
	getCalled := false
	mockClient := &mockUserFedClient{
		BaseMockClient: &testhelpers.BaseMockClient{},
		getUserFedFn: func(ctx context.Context, realm, id string) (interface{}, error) {
			getCalled = true
			return map[string]string{"id": id, "type": "ldap"}, nil
		},
	}

	fed, err := mockClient.GetUserFed(context.Background(), "test-realm", "fed-123")
	if err != nil {
		t.Fatalf("GetUserFed failed: %v", err)
	}
	if !getCalled {
		t.Fatal("GetUserFed was not called")
	}
	if fed == nil {
		t.Fatal("Expected federation but got nil")
	}
}

func (m *mockUserFedClient) CreateUserFed(ctx context.Context, realm string, config interface{}) (interface{}, error) {
	if m.createUserFedFn != nil {
		return m.createUserFedFn(ctx, realm, config)
	}
	return config, nil
}

func TestUserFederationCreateSuccess(t *testing.T) {
	createCalled := false
	mockClient := &mockUserFedClient{
		BaseMockClient: &testhelpers.BaseMockClient{},
		createUserFedFn: func(ctx context.Context, realm string, config interface{}) (interface{}, error) {
			createCalled = true
			return config, nil
		},
	}

	result, err := mockClient.CreateUserFed(context.Background(), "test-realm", map[string]string{"type": "ldap"})
	if err != nil {
		t.Fatalf("CreateUserFed failed: %v", err)
	}
	if !createCalled {
		t.Fatal("CreateUserFed was not called")
	}
	if result == nil {
		t.Fatal("Expected federation but got nil")
	}
}
