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

// +groupName=userfederation.keycloak.crossplane.io

package v1alpha1

import (
	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// UserFederationProviderParameters are the configurable fields of a UserFederationProvider.
type UserFederationProviderParameters struct {
	// RealmId is the ID of the realm.
	// +kubebuilder:validation:Required
	RealmId string `json:"realmId"`

	// Name is the name of the user federation provider.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// ProviderName is the provider type (e.g., "ldap", "kerberos").
	// +kubebuilder:validation:Required
	ProviderName string `json:"providerName"`

	// Priority is the priority of the provider.
	// +optional
	Priority *int32 `json:"priority,omitempty"`

	// Config is the provider configuration as key-value pairs.
	// +optional
	Config map[string]string `json:"config,omitempty"`

	// Enabled indicates if the provider is enabled.
	// +optional
	Enabled *bool `json:"enabled,omitempty"`
}

// UserFederationProviderSpec defines the desired state of UserFederationProvider.
type UserFederationProviderSpec struct {
	xpv1.ManagedResourceSpec `json:",inline"`
	ForProvider              UserFederationProviderParameters `json:"forProvider"`
}

// UserFederationProviderStatus defines the observed state of UserFederationProvider.
type UserFederationProviderStatus struct {
	xpv1.ManagedResourceStatus `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,keycloak}
// +kubebuilder:storageversion

// UserFederationProvider manages user federation providers (non-LDAP).
type UserFederationProvider struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   UserFederationProviderSpec   `json:"spec"`
	Status UserFederationProviderStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// UserFederationProviderList contains a list of UserFederationProvider.
type UserFederationProviderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []UserFederationProvider `json:"items"`
}
