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

// +groupName=scopes.keycloak.crossplane.io

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
)

// ClientScopeMappingParameters are the configurable fields of a ClientScopeMapping.
type ClientScopeMappingParameters struct {
	// RealmId is the ID of the realm.
	// +kubebuilder:validation:Required
	RealmId string `json:"realmId"`

	// ClientId is the client ID (internal UUID).
	// +kubebuilder:validation:Required
	ClientId string `json:"clientId"`

	// Scopes is the list of scopes to assign.
	// +optional
	Scopes []ScopeMapping `json:"scopes,omitempty"`
}

// ScopeMapping represents a scope to be assigned.
type ScopeMapping struct {
	// Name is the name of the scope.
	// +optional
	Name string `json:"name,omitempty"`

	// Id is the ID of the scope.
	// +optional
	Id string `json:"id,omitempty"`
}

// ClientScopeMappingSpec defines the desired state of ClientScopeMapping.
type ClientScopeMappingSpec struct {
	xpv1.ManagedResourceSpec `json:",inline"`
	ForProvider              ClientScopeMappingParameters `json:"forProvider"`
}

// ClientScopeMappingStatus defines the observed state of ClientScopeMapping.
type ClientScopeMappingStatus struct {
	xpv1.ManagedResourceStatus `json:",inline"`

	// AppliedScopes is the list of scopes currently assigned.
	// +optional
	AppliedScopes []ScopeMapping `json:"appliedScopes,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,keycloak}
// +kubebuilder:storageversion

// ClientScopeMapping manages client scope assignments for clients.
type ClientScopeMapping struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClientScopeMappingSpec   `json:"spec"`
	Status ClientScopeMappingStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ClientScopeMappingList contains a list of ClientScopeMapping.
type ClientScopeMappingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClientScopeMapping `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClientScopeMapping{}, &ClientScopeMappingList{})
}