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

// +groupName=identityprovider.keycloak.crossplane.io

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
)

// IdentityProviderParameters are the configurable fields of an IdentityProvider.
type IdentityProviderParameters struct {
	// RealmId is the ID of the realm.
	// +kubebuilder:validation:Required
	RealmId string `json:"realmId"`

	// Alias is the identity provider alias (unique identifier).
	// +kubebuilder:validation:Required
	Alias string `json:"alias"`

	// DisplayName is the display name for the identity provider.
	// +optional
	DisplayName *string `json:"displayName,omitempty"`

	// ProviderId is the identity provider type (e.g., "oidc", "saml").
	// +kubebuilder:validation:Required
	ProviderId string `json:"providerId"`

	// Enabled indicates if the identity provider is enabled.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=true
	Enabled *bool `json:"enabled,omitempty"`

	// TrustEmail indicates if emails from this provider are trusted.
	// +optional
	TrustEmail *bool `json:"trustEmail,omitempty"`

	// FirstBrokerLoginFlowAlias is the flow to use on first broker login.
	// +optional
	FirstBrokerLoginFlowAlias *string `json:"firstBrokerLoginFlowAlias,omitempty"`

	// PostBrokerLoginFlowAlias is the flow to use after broker login.
	// +optional
	PostBrokerLoginFlowAlias *string `json:"postBrokerLoginFlowAlias,omitempty"`

	// Config is the provider-specific configuration as key-value pairs.
	// +optional
	Config map[string]string `json:"config,omitempty"`
}

// IdentityProviderSpec defines the desired state of an IdentityProvider.
type IdentityProviderSpec struct {
	xpv1.ManagedResourceSpec `json:",inline"`
	ForProvider              IdentityProviderParameters `json:"forProvider"`
}

// IdentityProviderStatus defines the observed state of an IdentityProvider.
type IdentityProviderStatus struct {
	xpv1.ManagedResourceStatus `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,keycloak}
// +kubebuilder:storageversion

// An IdentityProvider manages Keycloak identity providers for federation (SAML, OIDC, etc).
type IdentityProvider struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   IdentityProviderSpec   `json:"spec"`
	Status IdentityProviderStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// IdentityProviderList contains a list of IdentityProvider.
type IdentityProviderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []IdentityProvider `json:"items"`
}

func init() {
	SchemeBuilder.Register(&IdentityProvider{}, &IdentityProviderList{})
}
