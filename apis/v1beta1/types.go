/*
Copyright 2024 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
)

// ProviderConfigSpec defines the desired state of a ProviderConfig.
type ProviderConfigSpec struct {
	// Credentials required to authenticate to this provider.
	Credentials ProviderCredentials `json:"credentials"`

	// BaseURL is the base URL for the Keycloak instance.
	// For example: https://keycloak.example.com/auth
	BaseURL string `json:"baseURL"`

	// Insecure allows connections to Keycloak instances with invalid certificates.
	// +kubebuilder:default=false
	Insecure *bool `json:"insecure,omitempty"`
}

// ProviderCredentials required to authenticate.
type ProviderCredentials struct {
	// Source of the provider credentials.
	// +kubebuilder:validation:Enum=Secret;InjectedIdentity;Environment;Filesystem
	Source xpv1.CredentialsSource `json:"source"`

	// SecretRef is a reference to a Secret in an arbitrary namespace.
	SecretRef *SecretReference `json:"secretRef,omitempty"`
}

// SecretReference is a reference to a Secret in an arbitrary namespace.
type SecretReference struct {
	// Name of the secret.
	Name string `json:"name"`

	// Namespace of the secret.
	Namespace string `json:"namespace"`

	// Key within the secret to use for value.
	// +optional
	Key string `json:"key,omitempty"`
}

// A ProviderConfigStatus reflects the observed state of a ProviderConfig.
type ProviderConfigStatus struct {
	xpv1.ProviderConfigStatus `json:",inline"`
}

// +kubebuilder:object:root=true

// A ProviderConfig configures a Keycloak provider.
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:printcolumn:name="SECRET-NAME",type="string",JSONPath=".spec.credentials.secretRef.name",priority=1
// +kubebuilder:printcolumn:name="BASE-URL",type="string",JSONPath=".spec.baseURL",priority=1
type ProviderConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ProviderConfigSpec   `json:"spec"`
	Status ProviderConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ProviderConfigList contains a list of ProviderConfig.
type ProviderConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ProviderConfig `json:"items"`
}

// +kubebuilder:object:root=true

// A ProviderConfigUsage indicates that a resource is using a ProviderConfig.
// +kubebuilder:resource:scope=Cluster
type ProviderConfigUsage struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	xpv1.TypedProviderConfigUsage `json:",inline"`
}

// +kubebuilder:object:root=true

// ProviderConfigUsageList contains a list of ProviderConfigUsage.
type ProviderConfigUsageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ProviderConfigUsage `json:"items"`
}

// =============================================================================
// Client Types
// =============================================================================

// ClientSpec defines the desired state of a Client.
type ClientSpec struct {
	// ProviderConfigReference to the provider config used to authenticate.
	ProviderConfigProviderConfigReference `json:"providerConfigRef"`

	// ForProvider are the fields to set on the Keycloak client.
	ForProvider ClientParameters `json:"forProvider"`
}

// ClientParameters are the fields to set on the Keycloak client.
type ClientParameters struct {
	// Realm is the Keycloak realm name.
	// +kubebuilder:validation:Required
	Realm string `json:"realm"`

	// ClientID is the OAuth2 client identifier.
	// +kubebuilder:validation:Required
	ClientID string `json:"clientId"`

	// Enabled indicates if the client is enabled.
	// +kubebuilder:default=true
	Enabled *bool `json:"enabled,omitempty"`

	// Name is the display name of the client.
	// +optional
	Name string `json:"name,omitempty"`

	// Description is the client description.
	// +optional
	Description string `json:"description,omitempty"`

	// RootURL is the client root URL.
	// +optional
	RootURL string `json:"rootURL,omitempty"`

	// BaseURL is the client base URL.
	// +optional
	BaseURL string `json:"baseURL,omitempty"`

	// ValidRedirectURIs is a list of valid redirect URIs.
	// +optional
	ValidRedirectURIs []string `json:"validRedirectURIs,omitempty"`

	// WebOrigins is a list of allowed CORS origins.
	// +optional
	WebOrigins []string `json:"webOrigins,omitempty"`

	// StandardFlowEnabled enables standard OIDC flow.
	// +kubebuilder:default=true
	StandardFlowEnabled *bool `json:"standardFlowEnabled,omitempty"`

	// DirectAccessGrantsEnabled enables direct access grants.
	// +kubebuilder:default=true
	DirectAccessGrantsEnabled *bool `json:"directAccessGrantsEnabled,omitempty"`

	// ImplicitFlowEnabled enables implicit flow.
	// +kubebuilder:default=false
	ImplicitFlowEnabled *bool `json:"implicitFlowEnabled,omitempty"`

	// ServiceAccountsEnabled enables service accounts.
	// +kubebuilder:default=false
	ServiceAccountsEnabled *bool `json:"serviceAccountsEnabled,omitempty"`

	// PublicClient indicates if this is a public client.
	// +kubebuilder:default=false
	PublicClient *bool `json:"publicClient,omitempty"`

	// Protocol is the client protocol (openid, saml).
	// +kubebuilder:default=openid
	// +optional
	Protocol string `json:"protocol,omitempty"`

	// Attribute is a map of client attributes.
	// +optional
	Attributes map[string]string `json:"attributes,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:name="REALM",type="string",JSONPath=".spec.forProvider.realm"
// +kubebuilder:printcolumn:name="CLIENT_ID",type="string",JSONPath=".spec.forProvider.clientId"
// +kubebuilder:printcolumn:name="ENABLED",type="boolean",JSONPath=".spec.forProvider.enabled"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
type Client struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ClientSpec `json:"spec"`
	// +kubebuilder:subresource:status
	Status ClientStatus `json:"status,omitempty"`
}

// ClientStatus represents the observed state of a Client.
type ClientStatus struct {
	xpv1.ManagedResourceStatus `json:",inline"`
}

// +kubebuilder:object:root=true

// ClientList contains a list of Client.
type ClientList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Client `json:"items"`
}

// ProviderConfigProviderConfigReference is a reference to a ProviderConfig
type ProviderConfigProviderConfigReference struct {
	// ProviderConfigReference to the provider config used to authenticate.
	Name string `json:"name"`
}