# provider-keycloak

A native [Crossplane](https://crossplane.io/) provider for [Keycloak](https://www.keycloak.org/) that manages Keycloak resources using the Keycloak Admin REST API directly — no Terraform, no Upjet.

The provider implements the same API surface as [crossplane-contrib/provider-keycloak](https://github.com/crossplane-contrib/provider-keycloak) so that existing manifests written for that provider work without modification.

## Managed Resource Types

| Kind | API Group | Description |
|------|-----------|-------------|
| `ProviderConfig` | `keycloak.crossplane.io/v1beta1` | Connection credentials for a Keycloak instance |
| `Client` | `openidclient.keycloak.crossplane.io/v1alpha1` | OpenID Connect client |
| `ClientDefaultScopes` | `openidclient.keycloak.crossplane.io/v1alpha1` | Default scopes assigned to a client |
| `ClientOptionalScopes` | `openidclient.keycloak.crossplane.io/v1alpha1` | Optional scopes assigned to a client |
| `Realm` | `realm.keycloak.crossplane.io/v1alpha1` | Keycloak realm |
| `User` | `user.keycloak.crossplane.io/v1alpha1` | Keycloak user |
| `Groups` | `user.keycloak.crossplane.io/v1alpha1` | Group memberships for a user |
| `Group` | `group.keycloak.crossplane.io/v1alpha1` | Keycloak group |
| `Role` | `role.keycloak.crossplane.io/v1alpha1` | Realm or client role |
| `ProtocolMapper` | `client.keycloak.crossplane.io/v1alpha1` | Protocol mapper on a client or client scope |

## ProviderConfig

The `ProviderConfig` references a Kubernetes Secret containing a JSON credentials blob:

```yaml
apiVersion: keycloak.crossplane.io/v1beta1
kind: ProviderConfig
metadata:
  name: keycloak-rossgolderltd
spec:
  credentials:
    source: Secret
    secretRef:
      name: keycloak-admin-credentials
      namespace: crossplane-system
      key: credentials
```

The secret value at `key` must be a JSON object:

```json
{
  "url": "https://keycloak.example.com",
  "base_path": "/auth",
  "realm": "master",
  "client_id": "crossplane",
  "client_secret": "your-client-secret",
  "root_ca_certificate": ""
}
```

The provider authenticates via OAuth2 client credentials grant (`grant_type=client_credentials`). `base_path` defaults to `/auth`; `realm` defaults to `master`.

## Example Resources

### OpenID Connect Client

```yaml
apiVersion: openidclient.keycloak.crossplane.io/v1alpha1
kind: Client
metadata:
  name: my-app
  namespace: crossplane-keycloak
spec:
  forProvider:
    realmId: my-realm
    clientId: my-app
    accessType: CONFIDENTIAL
    name: "My Application"
    standardFlowEnabled: true
    directAccessGrantsEnabled: false
    serviceAccountsEnabled: false
    validRedirectUris:
      - "https://my-app.example.com/*"
    webOrigins:
      - "+"
    clientSecretSecretRef:
      name: my-app-client-secret
      namespace: crossplane-keycloak
      key: client-secret
  providerConfigRef:
    name: keycloak-rossgolderltd
  deletionPolicy: Delete
```

### Realm

```yaml
apiVersion: realm.keycloak.crossplane.io/v1alpha1
kind: Realm
metadata:
  name: my-realm
  namespace: crossplane-keycloak
spec:
  forProvider:
    realm: my-realm
    displayName: "My Realm"
    enabled: true
    loginWithEmailAllowed: true
    sslRequired: external
  providerConfigRef:
    name: keycloak-rossgolderltd
```

### User

```yaml
apiVersion: user.keycloak.crossplane.io/v1alpha1
kind: User
metadata:
  name: my-realm-alice
  namespace: crossplane-keycloak
spec:
  forProvider:
    realmId: my-realm
    username: alice
    email: alice@example.com
    emailVerified: true
    enabled: true
    firstName: Alice
    lastName: Example
  providerConfigRef:
    name: keycloak-rossgolderltd
```

## Authentication

The provider authenticates to Keycloak using OAuth2 client credentials. The `client_id` in the credentials secret must have the `realm-management` client role (or equivalent admin permissions) in the target realm.

## Cross-references

All `realmId`, `clientId`, `userId`, `parentId`, etc. fields support both direct values and Crossplane cross-references:

```yaml
# Direct value
realmId: my-realm

# Reference to a Realm CR (uses the Keycloak internal ID from status)
realmIdRef:
  name: my-realm-cr
```

## Development

```shell
# Build
make build

# Lint
make lint

# Run tests
make test

# Generate CRDs
cd apis && go generate ./...
```

The `zz_generated.deepcopy.go` and `zz_generated.managed.go` files are maintained by hand — do not regenerate them with `controller-gen object:...` or `angryjet`.
