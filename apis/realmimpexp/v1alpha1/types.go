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

// +groupName=realmimpexp.keycloak.crossplane.io

package v1alpha1

import (
	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RealmImportParameters are the configurable fields of a RealmImport.
type RealmImportParameters struct {
	// RealmId is the ID of the realm to import.
	// +kubebuilder:validation:Required
	RealmId string `json:"realmId"`

	// IfNotExists indicates to not fail if realm already exists.
	// +optional
	IfNotExists *bool `json:"ifNotExists,omitempty"`

	// Realm JSON representation (full or partial).
	// +kubebuilder:validation:Required
	RealmJSON string `json:"realmJson"`
}

// RealmImportSpec defines the desired state of RealmImport.
type RealmImportSpec struct {
	xpv1.ManagedResourceSpec `json:",inline"`
	ForProvider              RealmImportParameters `json:"forProvider"`
}

// RealmImportStatus defines the observed state of RealmImport.
type RealmImportStatus struct {
	xpv1.ManagedResourceStatus `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,keycloak}
// +kubebuilder:storageversion

// RealmImport manages realm import operations.
type RealmImport struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RealmImportSpec   `json:"spec"`
	Status RealmImportStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RealmImportList contains a list of RealmImport.
type RealmImportList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RealmImport `json:"items"`
}
