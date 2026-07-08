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

// +groupName=rolemappings.keycloak.crossplane.io

package v1alpha1

import (
	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ClientRoleMappingParameters are the configurable fields of a ClientRoleMapping.
type ClientRoleMappingParameters struct {
	// RealmId is the ID of the realm.
	// +kubebuilder:validation:Required
	RealmId string `json:"realmId"`

	// UserId is the ID of the user.
	// +kubebuilder:validation:Required
	UserId string `json:"userId"`

	// ClientId is the client ID (internal UUID).
	// +kubebuilder:validation:Required
	ClientId string `json:"clientId"`

	// Roles is the list of roles to assign.
	// +optional
	Roles []RoleMapping `json:"roles,omitempty"`
}

// RoleMapping represents a role to be assigned.
type RoleMapping struct {
	// Name is the name of the role.
	// +optional
	Name string `json:"name,omitempty"`

	// Id is the ID of the role.
	// +optional
	Id string `json:"id,omitempty"`
}

// ClientRoleMappingSpec defines the desired state of ClientRoleMapping.
type ClientRoleMappingSpec struct {
	xpv1.ManagedResourceSpec `json:",inline"`
	ForProvider              ClientRoleMappingParameters `json:"forProvider"`
}

// ClientRoleMappingStatus defines the observed state of ClientRoleMapping.
type ClientRoleMappingStatus struct {
	xpv1.ManagedResourceStatus `json:",inline"`

	// AppliedRoles is the list of roles currently assigned.
	// +optional
	AppliedRoles []RoleMapping `json:"appliedRoles,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,keycloak}
// +kubebuilder:storageversion

// ClientRoleMapping manages client role assignments for users.
type ClientRoleMapping struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClientRoleMappingSpec   `json:"spec"`
	Status ClientRoleMappingStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ClientRoleMappingList contains a list of ClientRoleMapping.
type ClientRoleMappingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClientRoleMapping `json:"items"`
}
