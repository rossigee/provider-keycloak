# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
