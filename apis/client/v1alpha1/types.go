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

// ProtocolMapperParameters are the configurable fields of a ProtocolMapper.
type ProtocolMapperParameters struct {
	// RealmId is the ID of the realm.
	// +optional
	RealmId *string `json:"realmId,omitempty"`
	// RealmIdRef is a reference to a Realm.
	// +optional
	RealmIdRef *xpv1.Reference `json:"realmIdRef,omitempty"`
	// RealmIdSelector selects a reference to a Realm.
	// +optional
	RealmIdSelector *xpv1.Selector `json:"realmIdSelector,omitempty"`

	// ClientId is the Keycloak internal UUID of the client this mapper belongs to.
	// Mutually exclusive with ClientScopeId.
	// +optional
	ClientId *string `json:"clientId,omitempty"`
	// ClientIdRef is a reference to a Client.
	// +optional
	ClientIdRef *xpv1.Reference `json:"clientIdRef,omitempty"`
	// ClientIdSelector selects a reference to a Client.
	// +optional
	ClientIdSelector *xpv1.Selector `json:"clientIdSelector,omitempty"`

	// ClientScopeId is the Keycloak internal UUID of the client scope this mapper belongs to.
	// Mutually exclusive with ClientId.
	// +optional
	ClientScopeId *string `json:"clientScopeId,omitempty"`
	// ClientScopeIdRef is a reference to a ClientScope.
	// +optional
	ClientScopeIdRef *xpv1.Reference `json:"clientScopeIdRef,omitempty"`
	// ClientScopeIdSelector selects a reference to a ClientScope.
	// +optional
	ClientScopeIdSelector *xpv1.Selector `json:"clientScopeIdSelector,omitempty"`

	// Name is the mapper name.
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Protocol is the protocol of the mapper (openid-connect or saml).
	// +kubebuilder:validation:Enum=openid-connect;saml
	// +kubebuilder:validation:Required
	Protocol string `json:"protocol"`

	// ProtocolMapper is the provider ID of the mapper type.
	// +kubebuilder:validation:Required
	ProtocolMapper string `json:"protocolMapper"`

	// Config is the mapper-specific configuration.
	// +optional
	Config map[string]string `json:"config,omitempty"`
}

// ProtocolMapperSpec defines the desired state of a ProtocolMapper.
type ProtocolMapperSpec struct {
	xpv1.ManagedResourceSpec `json:",inline"`
	ForProvider              ProtocolMapperParameters `json:"forProvider"`
}

// ProtocolMapperStatus defines the observed state of a ProtocolMapper.
type ProtocolMapperStatus struct {
	xpv1.ManagedResourceStatus `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,keycloak}
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:name="REALM",type="string",JSONPath=".spec.forProvider.realmId"
// +kubebuilder:printcolumn:name="NAME",type="string",JSONPath=".spec.forProvider.name"
// +kubebuilder:printcolumn:name="MAPPER",type="string",JSONPath=".spec.forProvider.protocolMapper"
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// A ProtocolMapper is a Keycloak protocol mapper attached to a client or client scope.
type ProtocolMapper struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ProtocolMapperSpec   `json:"spec"`
	Status ProtocolMapperStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ProtocolMapperList contains a list of ProtocolMapper.
type ProtocolMapperList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ProtocolMapper `json:"items"`
}
