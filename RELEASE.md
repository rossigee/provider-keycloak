# v0.1.0 Release Preparation

**Status:** Ready for testing validation  
**Test Image:** `ghcr.io/rossigee/provider-keycloak:test` (commit f400b34)  
**Release Date:** 2026-06-06

## What's Included in v0.1.0

### 🔧 Critical Fix
- **Bug #1: CRD Group Registration**
  - Root cause: CRD manifests had empty API groups preventing Crossplane RBAC initialization
  - Impact: Provider failed to start when CRDs were deleted/recreated
  - Solution: Added `+groupName` declarations to all API types, regenerated CRDs
  - Status: Fixed and included in test image

### ✨ New Features
- **20+ new Client configuration fields:**
  - URLs: `homeUrl`, `adminUrl`, `frontchannelLogoutUrl`, `backchannelLogoutUrl`
  - Logout: `backchannelLogoutSessionRequired`, `backchannelLogoutRevokeOfflineSessions`
  - Timeouts: `clientSessionIdleTimeout`, `clientSessionMaxLifespan`, offline variants
  - Flags: `publicClient`, `bearerOnly`, `consentRequired`, `fullScopeAllowed`, `alwaysDisplayInConsole`
  - Advanced: `authorizationServicesEnabled`, `oauth2DeviceAuthorizationGrantEnabled`, `standardTokenExchangeEnabled`, `useRefreshTokens`
  - Protocol: `protocol`, `pkceCodeChallengeMethod`, `accessTokenLifespan`

### 📚 Documentation
- CHANGELOG.md: Comprehensive change documentation
- examples/client-advanced.yaml: 4 example client configurations
- docs/resource-analysis.md: Analysis of all resources and expansion opportunities
- README.md: Updated API group documentation

### 🎯 Code Quality
- All linting passes (0 issues)
- Refactored comparison functions for maintainability
- Comprehensive field sync logic
- Proper error handling

## Release Checklist

### ✅ Completed
- [x] Code changes implemented
- [x] All linting passes (golangci-lint, go vet, go fmt)
- [x] Commit eafcd59: Bug fix + feature additions
- [x] Commit f400b34: Documentation + examples
- [x] Test image built: `ghcr.io/rossigee/provider-keycloak:test`
- [x] Project memory updated
- [x] CHANGELOG created
- [x] Examples created
- [x] Resource analysis documented

### ⏳ Pending
- [ ] User validation: Test image passes through separate test suite
- [ ] **If tests pass:**
  - [ ] Tag release: `git tag v0.1.0`
  - [ ] Push tags: `git push origin v0.1.0`
  - [ ] Push commits: `git push origin master`
  - [ ] Create GitHub release
  - [ ] Build production images
  - [ ] Publish to container registries

### ❌ Not Required (Out of Scope)
- Integration tests (user's test suite will validate)
- Full test coverage (adequate for critical paths)
- Other resource field expansion (future versions)

## Pre-Release Testing Instructions

The user should validate the test image (`ghcr.io/rossigee/provider-keycloak:test`) with their test suite by:

1. **Deploy the test image**
   ```bash
   docker pull ghcr.io/rossigee/provider-keycloak:test
   # Or use in a Kubernetes deployment
   ```

2. **Verify the RBAC fix** (Bug #1)
   - Deploy provider with test image
   - Delete a CRD (e.g., clients.openidclient.keycloak.crossplane.io)
   - Recreate the same CRD
   - Provider should continue operating (no RBAC initialization failure)

3. **Test new Client fields**
   - Apply example manifests from `examples/client-advanced.yaml`
   - Verify all configuration fields sync properly
   - Check that field changes are detected and applied

4. **Validate all resource types**
   - Test existing Realm, User, Role, Group, ProtocolMapper resources
   - Ensure no regressions from CRD changes

## Post-Release Tasks

After user validates and approves:

```bash
# Tag the release
git tag -a v0.1.0 -m "Release v0.1.0: CRD group registration fix + client feature expansion"

# Push to remote
git push origin master
git push origin v0.1.0

# Build production images (if needed for multi-platform)
make -j4 build
docker build -t ghcr.io/rossigee/provider-keycloak:v0.1.0 -f cluster/images/provider-keycloak/Dockerfile .
docker push ghcr.io/rossigee/provider-keycloak:v0.1.0
```

## Known Issues & Limitations

- None blocking release
- Resource analysis identified expansion opportunities for User/Realm resources (deferred to v0.2.0)

## Version Notes

This is v0.1.0 - the first production-ready release with:
- Complete Keycloak resource type coverage
- CRD group registration fix
- Comprehensive client configuration support
- Full controller implementations for all resource types
- Proper OAuth2 token refresh mechanism
- Kubernetes secret integration for client credentials

## Support & Next Steps

After v0.1.0 is validated:

**v0.2.0 Planning (Based on user feedback)**
- User resource field expansion (if requested)
- Realm configuration expansion (if requested)
- Additional test coverage
- Performance optimization

**Contact:** Ross Golder (ross@golder.org)
