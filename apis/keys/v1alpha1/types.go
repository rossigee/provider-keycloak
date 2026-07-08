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

// +groupName=keys.keycloak.crossplane.io

package v1alpha1

import (
	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RealmKeysParameters are the configurable fields of RealmKeys.
type RealmKeysParameters struct {
	// RealmId is the ID of the realm.
	// +kubebuilder:validation:Required
	RealmId string `json:"realmId"`
}

// RealmKeysSpec defines the desired state of RealmKeys.
type RealmKeysSpec struct {
	xpv1.ManagedResourceSpec `json:",inline"`
	ForProvider              RealmKeysParameters `json:"forProvider"`
}

// RealmKeysStatus defines the observed state of RealmKeys.
type RealmKeysStatus struct {
	xpv1.ManagedResourceStatus `json:",inline"`

	// Keys is the list of keys for the realm.
	// +optional
	Keys []KeyInfo `json:"keys,omitempty"`
}

// KeyInfo represents information about a key.
type KeyInfo struct {
	// Kid is the key ID.
	// +optional
	Kid string `json:"kid,omitempty"`

	// Type is the key type.
	// +optional
	Type string `json:"type,omitempty"`

	// Algorithm is the key algorithm.
	// +optional
	Algorithm string `json:"algorithm,omitempty"`

	// Status is the key status.
	// +optional
	Status string `json:"status,omitempty"`

	// Certificate is the key certificate.
	// +optional
	Certificate string `json:"certificate,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,keycloak}
// +kubebuilder:storageversion

// RealmKeys provides read-only access to realm keys.
type RealmKeys struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RealmKeysSpec   `json:"spec"`
	Status RealmKeysStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RealmKeysList contains a list of RealmKeys.
type RealmKeysList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RealmKeys `json:"items"`
}
