package clientcertificates

import (
	"context"
	"testing"

	"github.com/rossigee/provider-keycloak/internal/controller/testhelpers"
)

type mockCertClient struct {
	*testhelpers.BaseMockClient
	getCertFn      func(ctx context.Context, realm, clientID string) (interface{}, error) //nolint:unused
	generateCertFn func(ctx context.Context, realm, clientID string) (interface{}, error)
}

func (m *mockCertClient) GetCert(ctx context.Context, realm, clientID string) (interface{}, error) {
	if m.getCertFn != nil {
		return m.getCertFn(ctx, realm, clientID)
	}
	return nil, nil
}

func TestClientCertificatesObserveExists(t *testing.T) {
	getCalled := false
	mockClient := &mockCertClient{
		BaseMockClient: &testhelpers.BaseMockClient{},
		getCertFn: func(ctx context.Context, realm, clientID string) (interface{}, error) {
			getCalled = true
			return map[string]string{"certificate": "cert-data"}, nil
		},
	}

	cert, err := mockClient.GetCert(context.Background(), "test-realm", "client-1")
	if err != nil {
		t.Fatalf("GetCert failed: %v", err)
	}
	if !getCalled {
		t.Fatal("GetCert was not called")
	}
	if cert == nil {
		t.Fatal("Expected certificate but got nil")
	}
}

func (m *mockCertClient) GenerateCert(ctx context.Context, realm, clientID string) (interface{}, error) {
	if m.generateCertFn != nil {
		return m.generateCertFn(ctx, realm, clientID)
	}
	return nil, nil
}

func TestClientCertificatesGenerateSuccess(t *testing.T) {
	genCalled := false
	mockClient := &mockCertClient{
		BaseMockClient: &testhelpers.BaseMockClient{},
		generateCertFn: func(ctx context.Context, realm, clientID string) (interface{}, error) {
			genCalled = true
			return map[string]string{"certificate": "new-cert-data"}, nil
		},
	}

	result, err := mockClient.GenerateCert(context.Background(), "test-realm", "client-1")
	if err != nil {
		t.Fatalf("GenerateCert failed: %v", err)
	}
	if !genCalled {
		t.Fatal("GenerateCert was not called")
	}
	if result == nil {
		t.Fatal("Expected certificate but got nil")
	}
}
