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

// +groupName=authorizationpolicy.keycloak.crossplane.io

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
)

// AuthorizationPolicyParameters are the configurable fields of an AuthorizationPolicy.
type AuthorizationPolicyParameters struct {
	// RealmId is the ID of the realm.
	// +kubebuilder:validation:Required
	RealmId string `json:"realmId"`

	// ClientId is the ID of the client.
	// +kubebuilder:validation:Required
	ClientId string `json:"clientId"`

	// Name is the policy name.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Type is the policy type (e.g., "role", "user", "resource", "scope", "aggregate").
	// +kubebuilder:validation:Required
	Type string `json:"type"`

	// Description is a description of the policy.
	// +optional
	Description *string `json:"description,omitempty"`

	// Logic is the policy logic (POSITIVE or NEGATIVE).
	// +optional
	Logic *string `json:"logic,omitempty"`

	// Config is the policy-specific configuration as key-value pairs.
	// +optional
	Config map[string]string `json:"config,omitempty"`
}

// AuthorizationPolicySpec defines the desired state of an AuthorizationPolicy.
type AuthorizationPolicySpec struct {
	xpv1.ManagedResourceSpec `json:",inline"`
	ForProvider              AuthorizationPolicyParameters `json:"forProvider"`
}

// AuthorizationPolicyStatus defines the observed state of an AuthorizationPolicy.
type AuthorizationPolicyStatus struct {
	xpv1.ManagedResourceStatus `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,keycloak}
// +kubebuilder:storageversion

// An AuthorizationPolicy manages Keycloak authorization policies for fine-grained access control.
type AuthorizationPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AuthorizationPolicySpec   `json:"spec"`
	Status AuthorizationPolicyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AuthorizationPolicyList contains a list of AuthorizationPolicy.
type AuthorizationPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AuthorizationPolicy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AuthorizationPolicy{}, &AuthorizationPolicyList{})
}
