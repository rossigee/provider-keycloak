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

// +groupName=authenticationflow.keycloak.crossplane.io

package v1alpha1

import (
	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AuthenticationFlowParameters are the configurable fields of an AuthenticationFlow.
type AuthenticationFlowParameters struct {
	// RealmId is the ID of the realm.
	// +kubebuilder:validation:Required
	RealmId string `json:"realmId"`

	// Alias is the authentication flow alias (unique identifier).
	// +kubebuilder:validation:Required
	Alias string `json:"alias"`

	// Description is a description of the authentication flow.
	// +optional
	Description *string `json:"description,omitempty"`

	// ProviderId is the flow provider type (e.g., "basic-flow", "client-flow").
	// +kubebuilder:validation:Required
	ProviderId string `json:"providerId"`

	// BuiltIn indicates if this is a built-in flow.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=false
	BuiltIn *bool `json:"builtIn,omitempty"`

	// TopLevel indicates if this flow is a top-level flow.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=true
	TopLevel *bool `json:"topLevel,omitempty"`
}

// AuthenticationFlowSpec defines the desired state of an AuthenticationFlow.
type AuthenticationFlowSpec struct {
	xpv1.ManagedResourceSpec `json:",inline"`
	ForProvider              AuthenticationFlowParameters `json:"forProvider"`
}

// AuthenticationFlowStatus defines the observed state of an AuthenticationFlow.
type AuthenticationFlowStatus struct {
	xpv1.ManagedResourceStatus `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,keycloak}
// +kubebuilder:storageversion

// An AuthenticationFlow manages Keycloak authentication flows (login, registration, etc).
type AuthenticationFlow struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AuthenticationFlowSpec   `json:"spec"`
	Status AuthenticationFlowStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AuthenticationFlowList contains a list of AuthenticationFlow.
type AuthenticationFlowList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AuthenticationFlow `json:"items"`
}
