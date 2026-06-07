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

// +groupName=clientinitialaccess.keycloak.crossplane.io

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
)

// ClientInitialAccessParameters are the configurable fields of a ClientInitialAccess.
type ClientInitialAccessParameters struct {
	// RealmId is the ID of the realm.
	// +kubebuilder:validation:Required
	RealmId string `json:"realmId"`

	// Count is the maximum number of times this token can be used.
	// +kubebuilder:validation:Required
	Count int32 `json:"count"`

	// Expiration is the expiration in seconds from now when this token expires.
	// +kubebuilder:validation:Required
	Expiration int32 `json:"expiration"`
}

// ClientInitialAccessSpec defines the desired state of ClientInitialAccess.
type ClientInitialAccessSpec struct {
	xpv1.ManagedResourceSpec `json:",inline"`
	ForProvider              ClientInitialAccessParameters `json:"forProvider"`
}

// ClientInitialAccessStatus defines the observed state of ClientInitialAccess.
type ClientInitialAccessStatus struct {
	xpv1.ManagedResourceStatus `json:",inline"`

	// Token is the generated initial access token (set after creation).
	// +optional
	Token string `json:"token,omitempty"`

	// RemainingCount is the number of times the token can still be used.
	// +optional
	RemainingCount int32 `json:"remainingCount,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,keycloak}
// +kubebuilder:storageversion

// ClientInitialAccess manages client initial access tokens for dynamic client registration.
type ClientInitialAccess struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClientInitialAccessSpec   `json:"spec"`
	Status ClientInitialAccessStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ClientInitialAccessList contains a list of ClientInitialAccess.
type ClientInitialAccessList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClientInitialAccess `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClientInitialAccess{}, &ClientInitialAccessList{})
}