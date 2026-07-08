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

// +groupName=authz.keycloak.crossplane.io

package v1alpha1

import (
	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AuthzResourceParameters are the configurable fields of an AuthzResource.
type AuthzResourceParameters struct {
	// RealmId is the ID of the realm.
	// +kubebuilder:validation:Required
	RealmId string `json:"realmId"`

	// ClientId is the client ID.
	// +kubebuilder:validation:Required
	ClientId string `json:"clientId"`

	// Name is the resource name.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// URIs are the resource URIs.
	// +optional
	URIs []string `json:"uris,omitempty"`

	// Type is the resource type.
	// +optional
	Type *string `json:"type,omitempty"`

	// Scopes are the resource scopes.
	// +optional
	Scopes []string `json:"scopes,omitempty"`

	// DisplayName is the display name.
	// +optional
	DisplayName *string `json:"displayName,omitempty"`

	// IconURI is the icon URI.
	// +optional
	IconURI *string `json:"iconUri,omitempty"`
}

// AuthzResourceSpec defines the desired state of AuthzResource.
type AuthzResourceSpec struct {
	xpv1.ManagedResourceSpec `json:",inline"`
	ForProvider              AuthzResourceParameters `json:"forProvider"`
}

// AuthzResourceStatus defines the observed state of AuthzResource.
type AuthzResourceStatus struct {
	xpv1.ManagedResourceStatus `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,keycloak}
// +kubebuilder:storageversion

// AuthzResource manages authorization resources (UMA).
type AuthzResource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AuthzResourceSpec   `json:"spec"`
	Status AuthzResourceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AuthzResourceList contains a list of AuthzResource.
type AuthzResourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AuthzResource `json:"items"`
}
