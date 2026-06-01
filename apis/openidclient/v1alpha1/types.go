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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
)

// ClientSecretSecretRef references a Kubernetes secret key holding the client secret.
type ClientSecretSecretRef struct {
	// Name of the secret.
	Name string `json:"name"`
	// Namespace of the secret.
	Namespace string `json:"namespace"`
	// Key within the secret.
	Key string `json:"key"`
}

// ClientParameters are the configurable fields of a Client.
type ClientParameters struct {
	// RealmId is the ID of the realm this client belongs to.
	// +optional
	RealmId *string `json:"realmId,omitempty"`
	// RealmIdRef is a reference to a Realm to populate realmId.
	// +optional
	RealmIdRef *xpv1.Reference `json:"realmIdRef,omitempty"`
	// RealmIdSelector selects a reference to a Realm.
	// +optional
	RealmIdSelector *xpv1.Selector `json:"realmIdSelector,omitempty"`

	// ClientId is the OAuth2 client identifier.
	// +kubebuilder:validation:Required
	ClientId string `json:"clientId"`

	// AccessType specifies the client access type (CONFIDENTIAL, PUBLIC, BEARER_ONLY).
	// +kubebuilder:validation:Enum=CONFIDENTIAL;PUBLIC;BEARER_ONLY
	// +optional
	AccessType *string `json:"accessType,omitempty"`

	// Name is the display name of the client.
	// +optional
	Name *string `json:"name,omitempty"`

	// Description is the client description.
	// +optional
	Description *string `json:"description,omitempty"`

	// Enabled indicates if the client is enabled.
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// RootUrl is the client root URL appended to relative URLs.
	// +optional
	RootUrl *string `json:"rootUrl,omitempty"`

	// BaseUrl is the default URL for redirecting when no redirect URI is specified.
	// +optional
	BaseUrl *string `json:"baseUrl,omitempty"`

	// AdminUrl is the URL to the admin interface of the client.
	// +optional
	AdminUrl *string `json:"adminUrl,omitempty"`

	// ValidRedirectUris is the list of valid redirect URIs.
	// +optional
	ValidRedirectUris []string `json:"validRedirectUris,omitempty"`

	// ValidPostLogoutRedirectUris is the list of valid post-logout redirect URIs.
	// +optional
	ValidPostLogoutRedirectUris []string `json:"validPostLogoutRedirectUris,omitempty"`

	// WebOrigins is the list of allowed CORS origins.
	// +optional
	WebOrigins []string `json:"webOrigins,omitempty"`

	// StandardFlowEnabled enables the standard OpenID Connect redirect based flow.
	// +optional
	StandardFlowEnabled *bool `json:"standardFlowEnabled,omitempty"`

	// ImplicitFlowEnabled enables the implicit flow.
	// +optional
	ImplicitFlowEnabled *bool `json:"implicitFlowEnabled,omitempty"`

	// DirectAccessGrantsEnabled enables the resource owner password credentials grant.
	// +optional
	DirectAccessGrantsEnabled *bool `json:"directAccessGrantsEnabled,omitempty"`

	// ServiceAccountsEnabled enables service accounts (client credentials grant).
	// +optional
	ServiceAccountsEnabled *bool `json:"serviceAccountsEnabled,omitempty"`

	// FullScopeAllowed allows all roles to be mapped as scope.
	// +optional
	FullScopeAllowed *bool `json:"fullScopeAllowed,omitempty"`

	// ConsentRequired requires user consent before the client receives tokens.
	// +optional
	ConsentRequired *bool `json:"consentRequired,omitempty"`

	// AlwaysDisplayInConsole always displays this client in the account console.
	// +optional
	AlwaysDisplayInConsole *bool `json:"alwaysDisplayInConsole,omitempty"`

	// FrontchannelLogoutEnabled enables front-channel logout.
	// +optional
	FrontchannelLogoutEnabled *bool `json:"frontchannelLogoutEnabled,omitempty"`

	// FrontchannelLogoutUrl is the URL for front-channel logout.
	// +optional
	FrontchannelLogoutUrl *string `json:"frontchannelLogoutUrl,omitempty"`

	// BackchannelLogoutUrl is the URL for back-channel logout.
	// +optional
	BackchannelLogoutUrl *string `json:"backchannelLogoutUrl,omitempty"`

	// BackchannelLogoutSessionRequired specifies whether a session is required on back-channel logout.
	// +optional
	BackchannelLogoutSessionRequired *bool `json:"backchannelLogoutSessionRequired,omitempty"`

	// BackchannelLogoutRevokeOfflineSessions revokes offline sessions on back-channel logout.
	// +optional
	BackchannelLogoutRevokeOfflineSessions *bool `json:"backchannelLogoutRevokeOfflineSessions,omitempty"`

	// LoginTheme overrides the login theme for this client.
	// +optional
	LoginTheme *string `json:"loginTheme,omitempty"`

	// PkceCodeChallengeMethod specifies the PKCE code challenge method.
	// +optional
	PkceCodeChallengeMethod *string `json:"pkceCodeChallengeMethod,omitempty"`

	// AccessTokenLifespan overrides the realm access token lifespan for this client.
	// +optional
	AccessTokenLifespan *string `json:"accessTokenLifespan,omitempty"`

	// ClientSecretSecretRef references the Kubernetes secret that will receive the generated client secret.
	// +optional
	ClientSecretSecretRef *ClientSecretSecretRef `json:"clientSecretSecretRef,omitempty"`

	// ExtraConfig is a map of additional Keycloak client configuration.
	// +optional
	ExtraConfig map[string]string `json:"extraConfig,omitempty"`
}

// ClientSpec defines the desired state of a Client.
type ClientSpec struct {
	xpv1.ManagedResourceSpec `json:",inline"`
	ForProvider              ClientParameters `json:"forProvider"`
}

// ClientStatus defines the observed state of a Client.
type ClientStatus struct {
	xpv1.ManagedResourceStatus `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,keycloak}
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:name="REALM",type="string",JSONPath=".spec.forProvider.realmId"
// +kubebuilder:printcolumn:name="CLIENT-ID",type="string",JSONPath=".spec.forProvider.clientId"
// +kubebuilder:printcolumn:name="ACCESS-TYPE",type="string",JSONPath=".spec.forProvider.accessType"
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// A Client is an OpenID Connect client managed by Keycloak.
type Client struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClientSpec   `json:"spec"`
	Status ClientStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ClientList contains a list of Client.
type ClientList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Client `json:"items"`
}

// ClientDefaultScopesParameters are the configurable fields of a ClientDefaultScopes.
type ClientDefaultScopesParameters struct {
	// RealmId is the ID of the realm.
	// +optional
	RealmId *string `json:"realmId,omitempty"`
	// RealmIdRef is a reference to a Realm.
	// +optional
	RealmIdRef *xpv1.Reference `json:"realmIdRef,omitempty"`
	// RealmIdSelector selects a reference to a Realm.
	// +optional
	RealmIdSelector *xpv1.Selector `json:"realmIdSelector,omitempty"`

	// ClientId is the internal Keycloak UUID of the client.
	// +optional
	ClientId *string `json:"clientId,omitempty"`
	// ClientIdRef is a reference to a Client to populate clientId.
	// +optional
	ClientIdRef *xpv1.Reference `json:"clientIdRef,omitempty"`
	// ClientIdSelector selects a reference to a Client.
	// +optional
	ClientIdSelector *xpv1.Selector `json:"clientIdSelector,omitempty"`

	// DefaultScopes is the list of default scopes to assign.
	// +kubebuilder:validation:Required
	DefaultScopes []string `json:"defaultScopes"`
}

// ClientDefaultScopesSpec defines the desired state of ClientDefaultScopes.
type ClientDefaultScopesSpec struct {
	xpv1.ManagedResourceSpec `json:",inline"`
	ForProvider              ClientDefaultScopesParameters `json:"forProvider"`
}

// ClientDefaultScopesStatus defines the observed state of ClientDefaultScopes.
type ClientDefaultScopesStatus struct {
	xpv1.ManagedResourceStatus `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,keycloak}
// +kubebuilder:storageversion

// ClientDefaultScopes manages the default scopes assigned to a client.
type ClientDefaultScopes struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClientDefaultScopesSpec   `json:"spec"`
	Status ClientDefaultScopesStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ClientDefaultScopesList contains a list of ClientDefaultScopes.
type ClientDefaultScopesList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClientDefaultScopes `json:"items"`
}

// ClientOptionalScopesParameters are the configurable fields of a ClientOptionalScopes.
type ClientOptionalScopesParameters struct {
	// RealmId is the ID of the realm.
	// +optional
	RealmId *string `json:"realmId,omitempty"`
	// RealmIdRef is a reference to a Realm.
	// +optional
	RealmIdRef *xpv1.Reference `json:"realmIdRef,omitempty"`
	// RealmIdSelector selects a reference to a Realm.
	// +optional
	RealmIdSelector *xpv1.Selector `json:"realmIdSelector,omitempty"`

	// ClientId is the internal Keycloak UUID of the client.
	// +optional
	ClientId *string `json:"clientId,omitempty"`
	// ClientIdRef is a reference to a Client to populate clientId.
	// +optional
	ClientIdRef *xpv1.Reference `json:"clientIdRef,omitempty"`
	// ClientIdSelector selects a reference to a Client.
	// +optional
	ClientIdSelector *xpv1.Selector `json:"clientIdSelector,omitempty"`

	// OptionalScopes is the list of optional scopes to assign.
	// +kubebuilder:validation:Required
	OptionalScopes []string `json:"optionalScopes"`
}

// ClientOptionalScopesSpec defines the desired state of ClientOptionalScopes.
type ClientOptionalScopesSpec struct {
	xpv1.ManagedResourceSpec `json:",inline"`
	ForProvider              ClientOptionalScopesParameters `json:"forProvider"`
}

// ClientOptionalScopesStatus defines the observed state of ClientOptionalScopes.
type ClientOptionalScopesStatus struct {
	xpv1.ManagedResourceStatus `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,keycloak}
// +kubebuilder:storageversion

// ClientOptionalScopes manages the optional scopes assigned to a client.
type ClientOptionalScopes struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClientOptionalScopesSpec   `json:"spec"`
	Status ClientOptionalScopesStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ClientOptionalScopesList contains a list of ClientOptionalScopes.
type ClientOptionalScopesList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClientOptionalScopes `json:"items"`
}
