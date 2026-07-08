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

// +groupName=component.keycloak.crossplane.io

package v1alpha1

import (
	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ComponentParameters are the configurable fields of a Component.
type ComponentParameters struct {
	// RealmId is the ID of the realm.
	// +kubebuilder:validation:Required
	RealmId string `json:"realmId"`

	// Name is the name of the component.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// ProviderType is the type of the component provider (e.g., "ldap", "org.keycloak.keys.KeyProvider").
	// +kubebuilder:validation:Required
	ProviderType string `json:"providerType"`

	// ProviderId is the ID of the component provider.
	// +optional
	ProviderId *string `json:"providerId,omitempty"`

	// Config is the component configuration as key-value pairs.
	// +optional
	Config map[string][]string `json:"config,omitempty"`

	// SubType is the component sub type.
	// +optional
	SubType *string `json:"subType,omitempty"`
}

// ComponentSpec defines the desired state of Component.
type ComponentSpec struct {
	xpv1.ManagedResourceSpec `json:",inline"`
	ForProvider              ComponentParameters `json:"forProvider"`
}

// ComponentStatus defines the observed state of Component.
type ComponentStatus struct {
	xpv1.ManagedResourceStatus `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,keycloak}
// +kubebuilder:storageversion

// Component manages generic Keycloak components.
type Component struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ComponentSpec   `json:"spec"`
	Status ComponentStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ComponentList contains a list of Component.
type ComponentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Component `json:"items"`
}
