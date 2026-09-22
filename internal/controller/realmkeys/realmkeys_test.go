package realmkeys

import (
	"context"
	"testing"

	"github.com/rossigee/provider-keycloak/internal/controller/testhelpers"
)

type mockRealmKeysClient struct {
	*testhelpers.BaseMockClient
	getKeysFn    func(ctx context.Context, realm string) (interface{}, error)
	generateKeyFn func(ctx context.Context, realm, algorithm string) (interface{}, error)
}

func (m *mockRealmKeysClient) GetKeys(ctx context.Context, realm string) (interface{}, error) {
	if m.getKeysFn != nil {
		return m.getKeysFn(ctx, realm)
	}
	return nil, nil
}

func TestRealmKeysObserveExists(t *testing.T) {
	getCalled := false
	mockClient := &mockRealmKeysClient{
		BaseMockClient: &testhelpers.BaseMockClient{},
		getKeysFn: func(ctx context.Context, realm string) (interface{}, error) {
			getCalled = true
			return map[string]interface{}{"keys": []map[string]string{{"kid": "key-1"}}}, nil
		},
	}

	keys, err := mockClient.GetKeys(context.Background(), "test-realm")
	if err != nil {
		t.Fatalf("GetKeys failed: %v", err)
	}
	if !getCalled {
		t.Fatal("GetKeys was not called")
	}
	if keys == nil {
		t.Fatal("Expected keys but got nil")
	}
}

func (m *mockRealmKeysClient) GenerateKey(ctx context.Context, realm, algorithm string) (interface{}, error) {
	if m.generateKeyFn != nil {
		return m.generateKeyFn(ctx, realm, algorithm)
	}
	return nil, nil
}

func TestRealmKeysGenerateSuccess(t *testing.T) {
	genCalled := false
	mockClient := &mockRealmKeysClient{
		BaseMockClient: &testhelpers.BaseMockClient{},
		generateKeyFn: func(ctx context.Context, realm, algorithm string) (interface{}, error) {
			genCalled = true
			return map[string]string{"kid": "new-key-1", "algorithm": algorithm}, nil
		},
	}

	result, err := mockClient.GenerateKey(context.Background(), "test-realm", "RS256")
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}
	if !genCalled {
		t.Fatal("GenerateKey was not called")
	}
	if result == nil {
		t.Fatal("Expected key but got nil")
	}
}
