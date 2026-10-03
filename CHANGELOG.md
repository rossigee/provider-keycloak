# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.19.9] - 2026-10-03

### Added
- **`internal/controller/deletecomplete`**: shared helper recording that a controller has finished releasing its external resource, so the reconciler can reach `RemoveFinalizer`. `Groups` moves onto it, unchanged in behaviour.
- **Structural regression test across every controller's `Observe`**, failing if any can only ever report `ResourceExists: true` — the shape that prevents a managed resource from ever terminating. It follows same-package delegation, so a controller whose `Observe` delegates to a shared helper is judged on the helper's body.

### Fixed
- **Groups controller: deleting a resource no longer wedges it forever on its finalizer**
  - The reconciler removes a managed resource's finalizer only once `Observe` reports the external resource gone, and while `Observe` keeps reporting it present it re-runs `Delete` on every pass. `Observe` hardcoded `ResourceExists: true`, so that verification never succeeded: every `Groups` delete re-entered the delete branch indefinitely. The object never terminated, the condition pair stayed at `Ready=False/Deleting` + `Synced=True/ReconcileSuccess`, nothing was logged (that path logs at debug), and each pass re-applied the membership removals — so a user's group membership oscillated against any surviving resource still declaring it. Observed during a realm migration, where two terminating resources stripped `admin` and `backups` every few seconds while their replacement re-added them.
  - `Delete` now records completion in a `keycloak.m.crossplane.io/delete-completed` annotation, patched explicitly because the reconciler only writes back status. `Observe` reports `ResourceExists: false` once it is present, so the reconciler reaches `RemoveFinalizer`. `Delete` also short-circuits on that annotation, so it cannot release memberships another resource has since declared.
  - The two paths with nothing left to release — user unresolvable, group refs unresolvable — previously returned without recording anything and would have hung identically; both now record completion.
  - A failed release is not recorded as complete, so the finalizer is not dropped while the membership is still in Keycloak. The existing tolerance of a `404` from Keycloak is unchanged.

- **Delete never completed for nine further controllers, not just `Groups`**
  - Auditing every `Observe` for a path that reports the external resource as absent found nine more with none. All wedged on their finalizer when deleted, and where `Delete` has side effects those repeated on every reconcile pass.
  - `authenticationflow`, `authorizationpolicy`, `identityprovider`, `clientrolemapping` and `clientscopemapping` now record completion via `deletecomplete`, report `ResourceExists: false` once recorded, and short-circuit `Delete` so a repeat call cannot release state a sibling resource has since declared.
  - `realmkeys`, `events` and `realmimpexp` have a no-op `Delete` — nothing in Keycloak to release — so their `Observe` reports `ResourceExists: false` as soon as deletion is requested. Their `Delete` bodies did nothing at all and returned `nil`, yet the finalizer still could not be removed.
  - `clientdefaultscopes`, `clientoptionalscopes` and `clientscope` were re-checked after the structural test flagged them: each delegates to a shared `ObserveX` helper that does report absence, so they are not affected.

- **`IdentityProvider` could never be created; the scope controllers could not be deleted once their client was gone**
  - The create-direction twin of the `ProtocolMapper` fix above. `GetIdentityProvider` propagated Keycloak's 404 as a plain error, and the first `Observe` of any new resource always 404s — so the reconciler aborted before `Create` and an `IdentityProvider` could not be created at all. Both live resources were 18 days old and had never been created, while looking correct in gitops.
  - `ClientDefaultScopes` and `ClientOptionalScopes` now report `ResourceExists: false` when the client is absent, so a resource left behind by a decommissioned client releases its finalizer. Found live: `rossgolderltd-k8s-bankrut-master-default-scopes`, terminating since 2026-10-01.
  - `internal/clients` gains `ErrNotFound`, wrapped by a `notFoundError` that keeps the previous message shape — several callers still match on the `"404"` substring — so `errors.Is` can be used without breaking them.
  - A failure to *reach* Keycloak still returns an error in every case, so a live resource is never mistaken for a missing one. Each controller has a test pinning that, because getting it wrong would send the reconciler to `Create` on every poll.
  - Updates one pre-existing test that asserted the old behaviour — that a missing client is an error — to assert the new contract.

- **`ProtocolMapper`: a mapper left behind by a decommissioned client could never be deleted**
  - A second, distinct reason a managed resource never terminates. The reconciler aborts a reconcile when `Observe` returns an error, so it never reaches `RemoveFinalizer`. `resolveClientUUID` failed permanently once a parent client was removed from Keycloak, so any mapper left behind held its finalizer forever and re-issued the lookup on every poll — adding load against an API that was already rate limiting.
  - `Observe` now distinguishes *the client is absent* from *Keycloak could not be reached*, and reports `ResourceExists: false` for the former. A mapper cannot outlive its client, so there is nothing left to release and the reconciler can finalise. Transport and auth failures still surface as errors, so a live mapper is never mistaken for a missing one.
  - Found in `ROSSGolderLtd`: three mappers terminating since 2026-10-01 with `client "k8s-bankrut-master" not found`, a client no longer declared in gitops.

### Known issues
- **`ClientRoleMapping` and `ClientScopeMapping` own the complete set, undocumented.** `rolesMatch`/`scopesMatch` require `len(desired) == len(current)`, so each resource is exhaustive over its parent rather than additive, and `Delete` removes the entire set. Two resources for the same parent would fight as `Groups` did, and deleting one removes its siblings' entries. Left unchanged here — this is a semantics decision (make them additive, or document and guard the exhaustive behaviour), not a mechanical fix.

## [0.19.7] - 2026-10-03

### Fixed
- **Critical:** ProviderConfig RBAC was granted on the wrong API group
  - `setupRBAC` requested ProviderConfig and ProviderConfigUsage permissions on `keycloak.crossplane.io`, but the CRDs are generated under `keycloak.m.crossplane.io` (see `apis/v1beta1/groupversion_info.go` and `package/crds/keycloak.m.crossplane.io_providerconfigs.yaml`). The provider's own ClusterRole therefore never matched its own ProviderConfig CRDs.
  - The same mismatch in the `*/finalizers` rule is corrected.
- **Critical:** Missing `events.k8s.io` grant in the runtime ClusterRole
  - `setupRBAC` omitted `create`/`patch`/`update` on `events.k8s.io`, even though `package/crossplane.yaml` already declared it. The two are now consistent.
- **Groups controller: two resources no longer fight over one user's group membership**
  - `Exhaustive` defaults to `true`, which makes a `Groups` resource authoritative for the *complete* set of a user's memberships — `sync` removes every group not listed. Two `Groups` resources for the same user therefore deleted each other's memberships on every reconcile, while both kept reporting `Synced=True` and `Ready=True`. The user's membership oscillated and the conflict was invisible from either resource.
  - `Observe` now lists sibling `Groups` resources in the namespace and, when another *exhaustive* resource resolves to the same Keycloak user in the same realm, refuses to sync: it marks the resource `Ready=False`, emits a `Warning` event with reason `MembershipConflict` naming the conflicting resources, and returns an error so the reconciler stops before `Update` performs any membership changes.
  - Additive siblings (`exhaustive: false`), siblings in another realm, siblings resolving to a different user, and siblings being deleted are not treated as conflicts.
- **Groups controller: `Ready` now reflects convergence**
  - `Observe` set `Available()` unconditionally, reporting `Ready=True` even when the observed membership differed from what the resource declares and `Update` was about to rewrite it. It now sets `Available()` only when the membership matches, and `Ready=False` with an explanatory message otherwise.

### Added
- `VERSION` file and an `internal/version` package, so the resolved release version is logged once at provider startup instead of being implicit.
- `spec.crossplane.version: ">=v2.5.0"` in `package/crossplane.yaml`, and `CROSSPLANE_VERSION ?= 2.5.0` in the Makefile.

### Changed
- **Release publishing** is now tag-driven and restricted to exact SemVer tags pointing at the current `origin/master`. It builds and publishes `linux_amd64` and `linux_arm64` xpkg files, aliases `latest`, verifies equal digests and both architectures, and authenticates to ghcr.io with a GitHub token instead of OIDC claims.
- `scripts/release.sh` hardened: validates the SemVer format, requires a clean worktree, requires `HEAD` to equal `origin/master`, requires `VERSION` to match the requested version, and refuses to proceed if the tag already exists locally or remotely.
- README install instructions use `kubectl crossplane install provider` against the pinned version, replacing a placeholder Helm repository.

## [0.19.6] - 2026-09-22

### Fixed
- **Critical:** Rate limit backoff deadline was being ignored by Crossplane's managed.Reconciler because deadline info was embedded in error message text, not in a structured error type
  - Created custom `RateLimitError` type with `Deadline()` and `RequeueAfter()` methods
  - Added jitter to backoff deadline to prevent thundering herd after backoff window expires
  - Crossplane can now detect RateLimitError and apply the actual deadline instead of generic backoff
  - Fixes Group membership sync failures where deadlines were ignored and reconciles retried immediately upon expiration

### Added
- **Comprehensive unit test coverage: 100% (23/23 controllers)**
  - Previously: 44% coverage (11/25 controllers), Groups controller shipped untested → shipped bug in v0.19.4
  - Now: All 23 controllers have regression tests using mock pattern
  - 68+ test cases covering observe, create, update, delete operations
  - Tests verify that operations actually persist (e.g., `AddUserToGroup` really adds the user)
  - Prevents regression of silent failures like Groups controller OIDC sync bug
  - New tests: authenticationflow, authorizationpolicy, clientcertificates, component, events, identityprovider, realmimpexp, realmkeys, userfederation, user/groups

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
