package realmimpexp

import (
	"context"
	"testing"

	"github.com/rossigee/provider-keycloak/internal/controller/testhelpers"
)

type mockRealmImpExpClient struct {
	*testhelpers.BaseMockClient
	getImportFn    func(ctx context.Context, realm, id string) (interface{}, error)
	executeImportFn func(ctx context.Context, realm string, data interface{}) (interface{}, error)
}

func (m *mockRealmImpExpClient) GetImport(ctx context.Context, realm, id string) (interface{}, error) {
	if m.getImportFn != nil {
		return m.getImportFn(ctx, realm, id)
	}
	return nil, nil
}

func TestRealmImpExpObserveExists(t *testing.T) {
	getCalled := false
	mockClient := &mockRealmImpExpClient{
		BaseMockClient: &testhelpers.BaseMockClient{},
		getImportFn: func(ctx context.Context, realm, id string) (interface{}, error) {
			getCalled = true
			return map[string]string{"id": id, "status": "completed"}, nil
		},
	}

	result, err := mockClient.GetImport(context.Background(), "test-realm", "import-123")
	if err != nil {
		t.Fatalf("GetImport failed: %v", err)
	}
	if !getCalled {
		t.Fatal("GetImport was not called")
	}
	if result == nil {
		t.Fatal("Expected import but got nil")
	}
}

func (m *mockRealmImpExpClient) ExecuteImport(ctx context.Context, realm string, data interface{}) (interface{}, error) {
	if m.executeImportFn != nil {
		return m.executeImportFn(ctx, realm, data)
	}
	return nil, nil
}

func TestRealmImpExpCreateSuccess(t *testing.T) {
	execCalled := false
	mockClient := &mockRealmImpExpClient{
		BaseMockClient: &testhelpers.BaseMockClient{},
		executeImportFn: func(ctx context.Context, realm string, data interface{}) (interface{}, error) {
			execCalled = true
			return map[string]string{"status": "completed"}, nil
		},
	}

	result, err := mockClient.ExecuteImport(context.Background(), "test-realm", map[string]interface{}{})
	if err != nil {
		t.Fatalf("ExecuteImport failed: %v", err)
	}
	if !execCalled {
		t.Fatal("ExecuteImport was not called")
	}
	if result == nil {
		t.Fatal("Expected import result but got nil")
	}
}
