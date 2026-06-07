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

// +groupName=clientcertificates.keycloak.crossplane.io

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
)

// ClientCertificateParameters are the configurable fields of a ClientCertificate.
type ClientCertificateParameters struct {
	// RealmId is the ID of the realm.
	// +kubebuilder:validation:Required
	RealmId string `json:"realmId"`

	// ClientId is the client ID (internal UUID).
	// +kubebuilder:validation:Required
	ClientId string `json:"clientId"`

	// Format is the certificate format (PEM, JKS).
	// +optional
	Format *string `json:"format,omitempty"`
}

// ClientCertificateSpec defines the desired state of ClientCertificate.
type ClientCertificateSpec struct {
	xpv1.ManagedResourceSpec `json:",inline"`
	ForProvider              ClientCertificateParameters `json:"forProvider"`
}

// ClientCertificateStatus defines the observed state of ClientCertificate.
type ClientCertificateStatus struct {
	xpv1.ManagedResourceStatus `json:",inline"`

	// Certificate is the generated certificate.
	// +optional
	Certificate string `json:"certificate,omitempty"`

	// PrivateKey is the private key.
	// +optional
	PrivateKey string `json:"privateKey,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,keycloak}
// +kubebuilder:storageversion

// ClientCertificate manages client certificates.
type ClientCertificate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClientCertificateSpec   `json:"spec"`
	Status ClientCertificateStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ClientCertificateList contains a list of ClientCertificate.
type ClientCertificateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClientCertificate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClientCertificate{}, &ClientCertificateList{})
}