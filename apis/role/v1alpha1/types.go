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

// +groupName=role.keycloak.crossplane.io

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
)

// RoleParameters are the configurable fields of a Role.
type RoleParameters struct {
	// RealmId is the ID of the realm this role belongs to.
	// +optional
	RealmId *string `json:"realmId,omitempty"`
	// RealmIdRef is a reference to a Realm.
	// +optional
	RealmIdRef *xpv1.Reference `json:"realmIdRef,omitempty"`
	// RealmIdSelector selects a reference to a Realm.
	// +optional
	RealmIdSelector *xpv1.Selector `json:"realmIdSelector,omitempty"`

	// ClientId scopes the role to a specific client (for client roles).
	// When set, this creates a client role; when unset, a realm role.
	// +optional
	ClientId *string `json:"clientId,omitempty"`
	// ClientIdRef is a reference to a Client.
	// +optional
	ClientIdRef *xpv1.Reference `json:"clientIdRef,omitempty"`
	// ClientIdSelector selects a reference to a Client.
	// +optional
	ClientIdSelector *xpv1.Selector `json:"clientIdSelector,omitempty"`

	// Name is the role name.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Description is a human-readable description of the role.
	// +optional
	Description *string `json:"description,omitempty"`

	// Attributes is a map of role attributes.
	// +optional
	Attributes map[string]string `json:"attributes,omitempty"`

	// CompositeRoles is the list of role names that compose this role.
	// +optional
	CompositeRoles []string `json:"compositeRoles,omitempty"`
	// CompositeRolesRefs is a list of references to Roles to compose.
	// +optional
	CompositeRolesRefs []xpv1.Reference `json:"compositeRolesRefs,omitempty"`
}

// RoleSpec defines the desired state of a Role.
type RoleSpec struct {
	xpv1.ManagedResourceSpec `json:",inline"`
	ForProvider              RoleParameters `json:"forProvider"`
}

// RoleStatus defines the observed state of a Role.
type RoleStatus struct {
	xpv1.ManagedResourceStatus `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,keycloak}
// +kubebuilder:storageversion
// +kubebuilder:group=role.keycloak.crossplane.io
// +kubebuilder:printcolumn:name="REALM",type="string",JSONPath=".spec.forProvider.realmId"
// +kubebuilder:printcolumn:name="NAME",type="string",JSONPath=".spec.forProvider.name"
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// A Role is a Keycloak realm or client role.
type Role struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RoleSpec   `json:"spec"`
	Status RoleStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RoleList contains a list of Role.
type RoleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Role `json:"items"`
}
