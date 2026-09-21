# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.19.6] - 2026-09-22

### Fixed
- **Critical:** Rate limit backoff deadline was being ignored by Crossplane's managed.Reconciler because deadline info was embedded in error message text, not in a structured error type
  - Created custom `RateLimitError` type with `Deadline()` and `RequeueAfter()` methods
  - Added jitter to backoff deadline to prevent thundering herd after backoff window expires
  - Crossplane can now detect RateLimitError and apply the actual deadline instead of generic backoff
  - Fixes Group membership sync failures where deadlines were ignored and reconciles retried immediately upon expiration

## [0.19.5] - 2026-09-21

### Fixed
- **Critical:** Rate limit backoff (HTTP 429) handling had race condition where concurrent reconcilers all passed the backoff check simultaneously, then all hit 429 together, resetting backoff indefinitely
  - Added semaphore-based concurrency limiter (capacity ~3) to prevent thundering herd on 429 errors
  - Now only ~3 requests can be in-flight at once; if any hit 429, backoff triggers before others also breach limit
  - Fixes repeated 429 errors that prevent Group membership and other operations from syncing

## [0.19.4] - 2026-09-21

### Added
- Implement missing `Groups` (user↔group membership) Crossplane controller
  - Manages Keycloak user-to-group membership via GitOps
  - Resolves `UserIdRef` and `GroupIdsRefs` by querying Keycloak directly
  - Supports `Exhaustive` flag: when true, removes unlisted group memberships; when false, additive-only
  - Enables OIDC group-based RBAC (e.g., cluster-admin via Keycloak group membership)

## [0.19.2] - 2026-09-20

### Fixed
- **Critical:** HTTP 429 rate limit backoff mechanism was incorrectly clearing on error responses, preventing backoff from working
  - Rate limit backoff now persists until successful (2xx) response
  - Applies Retry-After header from server when present (RFC 7231 compliant)
  - Falls back to exponential backoff (1s → 2s → 4s... capped at 30s) for consecutive hits
- Scope deletion now correctly reports `ResourceExists=false` when scope not found

### Added
- Comprehensive rate limit test coverage (7 new test functions, coverage 38.1% → 41.5%)
  - Tests for Retry-After parsing, backoff state management, exponential backoff progression
  - Integration tests for doRequest() and doCreate() with rate limiting
- Rate limit tracking for `doCreate()` POST operations (was missing)
- Rate limit tracking for `UpdateRealmRaw()` raw realm updates (was missing)
- Complete technical documentation: [docs/RATE_LIMITING.md](docs/RATE_LIMITING.md)

### Changed
- Rate limit handling now consistent across all HTTP methods (doRequest, doCreate, UpdateRealmRaw)
- Release workflow now on canonical single-job pattern

## [0.1.0] - 2026-06-06

### Fixed
- **Critical:** Resolve CRD group registration issue when CRDs are deleted/recreated ([#1](https://github.com/rossigee/provider-keycloak/issues/1))
  - CRD manifests now have properly defined API groups, allowing Crossplane RBAC system to properly initialize provider revision clusterroles
  - Regenerated all CRD manifests with correct group specifications using kubebuilder annotations

### Added
- Comprehensive Client resource configuration support with 20+ new fields:
  - **Additional URLs:** `homeUrl`, `adminUrl`, `frontchannelLogoutUrl`, `backchannelLogoutUrl`
  - **Logout Configuration:** `backchannelLogoutSessionRequired`, `backchannelLogoutRevokeOfflineSessions`
  - **Session Timeouts:** `clientSessionIdleTimeout`, `clientSessionMaxLifespan`, `clientOfflineSessionIdleTimeout`, `clientOfflineSessionMaxLifespan`
  - **Client Flags:** `publicClient`, `bearerOnly`, `consentRequired`, `fullScopeAllowed`, `alwaysDisplayInConsole`, `authorizationServicesEnabled`, `oauth2DeviceAuthorizationGrantEnabled`, `standardTokenExchangeEnabled`, `useRefreshTokens`
  - **Protocol Configuration:** `protocol`, `pkceCodeChallengeMethod`, `accessTokenLifespan`
- Enhanced client controller with comprehensive field sync and change detection logic
- Improved code quality through refactored state comparison functions (reduced cyclomatic complexity)

### Changed
- Restructured CRD naming from underscore-prefixed to properly-grouped filenames (e.g., `_clients.yaml` → `openidclient.keycloak.crossplane.io_clients.yaml`)
- Updated API type definitions with package-level group declarations for proper CRD generation

### Documentation
- Fixed README API group documentation for Group resource (was `group.keycloak.crossplane.io`, now correctly `user.keycloak.crossplane.io`)
- Updated managed resource types table with accurate API groups

## Initial Release

Initial native Crossplane provider for Keycloak with support for:
- OpenID Connect Clients
- Realms
- Users and Groups
- Roles
- Protocol Mappers
- ProviderConfig for Keycloak instance credentials
