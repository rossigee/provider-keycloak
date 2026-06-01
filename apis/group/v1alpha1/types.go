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

// GroupParameters are the configurable fields of a Group.
type GroupParameters struct {
	// RealmId is the ID of the realm this group belongs to.
	// +optional
	RealmId *string `json:"realmId,omitempty"`
	// RealmIdRef is a reference to a Realm.
	// +optional
	RealmIdRef *xpv1.Reference `json:"realmIdRef,omitempty"`
	// RealmIdSelector selects a reference to a Realm.
	// +optional
	RealmIdSelector *xpv1.Selector `json:"realmIdSelector,omitempty"`

	// Name is the group name.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// ParentId is the Keycloak UUID of the parent group (for sub-groups).
	// +optional
	ParentId *string `json:"parentId,omitempty"`
	// ParentIdRef is a reference to a parent Group.
	// +optional
	ParentIdRef *xpv1.Reference `json:"parentIdRef,omitempty"`
	// ParentIdSelector selects a reference to a parent Group.
	// +optional
	ParentIdSelector *xpv1.Selector `json:"parentIdSelector,omitempty"`

	// Description is a human-readable description of the group.
	// +optional
	Description *string `json:"description,omitempty"`

	// Attributes is a map of group attributes.
	// +optional
	Attributes map[string]string `json:"attributes,omitempty"`
}

// GroupSpec defines the desired state of a Group.
type GroupSpec struct {
	xpv1.ManagedResourceSpec `json:",inline"`
	ForProvider              GroupParameters `json:"forProvider"`
}

// GroupStatus defines the observed state of a Group.
type GroupStatus struct {
	xpv1.ManagedResourceStatus `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,keycloak}
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:name="REALM",type="string",JSONPath=".spec.forProvider.realmId"
// +kubebuilder:printcolumn:name="NAME",type="string",JSONPath=".spec.forProvider.name"
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// A Group is a Keycloak group.
type Group struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GroupSpec   `json:"spec"`
	Status GroupStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// GroupList contains a list of Group.
type GroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Group `json:"items"`
}
