# Testing Guide for Provider-Keycloak Controllers

## Current Coverage: 44% (11/25 controllers)

Controllers with NO tests (HIGH PRIORITY):
- authz
- identityprovider
- clientinitialaccess
- clientrolemapping
- clientscopemapping
- authenticationflow
- authorizationpolicy
- clientcertificates
- component
- events
- realmimpexp
- realmkeys
- userfederation

## Test Template

Each controller test should follow this pattern:

```go
package controllerpackage

import (
	"context"
	"testing"

	"github.com/rossigee/provider-keycloak/internal/controller/testhelpers"
	"github.com/rossigee/provider-keycloak/internal/clients"
)

type mockClient struct {
	*testhelpers.BaseMockClient
	// Add operation-specific function pointers
	// E.g., for IdentityProvider:
	getIdentityProviderFn    func(ctx context.Context, realm, alias string) (interface{}, error)
	createIdentityProviderFn func(ctx context.Context, realm string, idp interface{}) (interface{}, error)
	updateIdentityProviderFn func(ctx context.Context, realm string, idp interface{}) error
	deleteIdentityProviderFn func(ctx context.Context, realm, alias string) error
}

// Implement all required clients.Client interface methods
func (m *mockClient) GetIdentityProvider(ctx context.Context, realm, alias string) (interface{}, error) {
	if m.getIdentityProviderFn != nil {
		return m.getIdentityProviderFn(ctx, realm, alias)
	}
	return nil, nil
}

// Add test cases:
// 1. TestObserveResourceExists - verify Observe detects existing resources
// 2. TestCreateResourceSuccess - verify Create calls API and returns correct status
// 3. TestUpdateResourceSuccess - verify Update applies changes
// 4. TestDeleteResourceSuccess - verify Delete removes resource
// 5. TestErrorHandling - verify errors are properly wrapped and reported
// 6. TestResourceNotFound - verify 404s are handled correctly
```

## Key Testing Patterns

### 1. Mock Client Setup
- Inherit from `testhelpers.BaseMockClient`
- Use function pointers for operation implementations
- Allow tests to control return values and errors

### 2. Test Cases per Controller
Every controller test suite should include:
- **Observe**: Resource exists in Keycloak
- **Observe**: Resource doesn't exist (404 handling)
- **Create**: Successful creation
- **Create**: Error handling (API errors, rate limiting)
- **Update**: Successful update
- **Delete**: Successful deletion
- **Delete**: Resource already deleted (idempotency)

### 3. Integration with K8s Client
Use `testclient.NewClientBuilder()` for fake K8s client:
```go
kubeClient := testclient.NewClientBuilder().Build()
```

## Critical Bug Caught by Testing

**Groups Controller (v0.19.4)**: Shipped without tests, caused production bug where:
- Controller reported "Synced: True" without verifying user was added to group
- OIDC tokens missing admin group claim
- Would have been caught by test verifying `AddUserToGroup` was actually called

## Next Steps

1. ✅ Add tests for: authz, identityprovider, clientinitialacession (HIGH)
2. Add tests for: clientrolemapping, clientscopemapping (MEDIUM)
3. Enforce: All new controllers must have tests before merge
4. Add: Pre-commit hook to verify controller_test.go exists for new controllers
