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

// +groupName=user.keycloak.crossplane.io

package v1alpha1

import (
	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// FederatedIdentity represents an identity from an external provider.
type FederatedIdentity struct {
	// IdentityProvider is the alias of the identity provider.
	IdentityProvider string `json:"identityProvider"`
	// UserId is the user ID at the identity provider.
	UserId string `json:"userId"`
	// UserName is the username at the identity provider.
	UserName string `json:"userName"`
}

// UserParameters are the configurable fields of a User.
type UserParameters struct {
	// RealmId is the ID of the realm this user belongs to.
	// +optional
	RealmId *string `json:"realmId,omitempty"`
	// RealmIdRef is a reference to a Realm.
	// +optional
	RealmIdRef *xpv1.Reference `json:"realmIdRef,omitempty"`
	// RealmIdSelector selects a reference to a Realm.
	// +optional
	RealmIdSelector *xpv1.Selector `json:"realmIdSelector,omitempty"`

	// Username is the user's login name.
	// +kubebuilder:validation:Required
	Username string `json:"username"`

	// Email is the user's email address.
	// +optional
	Email *string `json:"email,omitempty"`

	// EmailVerified indicates if the user's email has been verified.
	// +optional
	EmailVerified *bool `json:"emailVerified,omitempty"`

	// Enabled indicates if the user account is enabled.
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// FirstName is the user's first name.
	// +optional
	FirstName *string `json:"firstName,omitempty"`

	// LastName is the user's last name.
	// +optional
	LastName *string `json:"lastName,omitempty"`

	// Attributes is a map of user attributes.
	// +optional
	Attributes map[string]string `json:"attributes,omitempty"`

	// RequiredActions is the list of required actions the user must perform on next login.
	// +optional
	RequiredActions []string `json:"requiredActions,omitempty"`

	// FederatedIdentity is the list of federated identities for this user.
	// +optional
	FederatedIdentity []FederatedIdentity `json:"federatedIdentity,omitempty"`
}

// UserSpec defines the desired state of a User.
type UserSpec struct {
	xpv1.ManagedResourceSpec `json:",inline"`
	ForProvider              UserParameters `json:"forProvider"`
}

// UserStatus defines the observed state of a User.
type UserStatus struct {
	xpv1.ManagedResourceStatus `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,keycloak}
// +kubebuilder:storageversion
// +kubebuilder:group=user.keycloak.crossplane.io
// +kubebuilder:printcolumn:name="REALM",type="string",JSONPath=".spec.forProvider.realmId"
// +kubebuilder:printcolumn:name="USERNAME",type="string",JSONPath=".spec.forProvider.username"
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// A User is a Keycloak user.
type User struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   UserSpec   `json:"spec"`
	Status UserStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// UserList contains a list of User.
type UserList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []User `json:"items"`
}

// GroupsParameters are the configurable fields of a Groups membership resource.
type GroupsParameters struct {
	// RealmId is the ID of the realm.
	// +optional
	RealmId *string `json:"realmId,omitempty"`
	// RealmIdRef is a reference to a Realm.
	// +optional
	RealmIdRef *xpv1.Reference `json:"realmIdRef,omitempty"`
	// RealmIdSelector selects a reference to a Realm.
	// +optional
	RealmIdSelector *xpv1.Selector `json:"realmIdSelector,omitempty"`

	// UserId is the Keycloak internal user UUID.
	// +optional
	UserId *string `json:"userId,omitempty"`
	// UserIdRef is a reference to a User.
	// +optional
	UserIdRef *xpv1.Reference `json:"userIdRef,omitempty"`
	// UserIdSelector selects a reference to a User.
	// +optional
	UserIdSelector *xpv1.Selector `json:"userIdSelector,omitempty"`

	// GroupIds is the list of Keycloak internal group UUIDs.
	// +optional
	GroupIds []string `json:"groupIds,omitempty"`
	// GroupIdsRefs is a list of references to Groups.
	// +optional
	GroupIdsRefs []xpv1.Reference `json:"groupIdsRefs,omitempty"`
	// GroupIdsSelector selects references to Groups.
	// +optional
	GroupIdsSelector *xpv1.Selector `json:"groupIdsSelector,omitempty"`

	// Exhaustive indicates whether this resource manages the complete set of group
	// memberships for the user (removing unlisted groups) or is additive only.
	// +kubebuilder:default=true
	// +optional
	Exhaustive *bool `json:"exhaustive,omitempty"`
}

// GroupsSpec defines the desired state of Groups membership.
type GroupsSpec struct {
	xpv1.ManagedResourceSpec `json:",inline"`
	ForProvider              GroupsParameters `json:"forProvider"`
}

// GroupsStatus defines the observed state of Groups membership.
type GroupsStatus struct {
	xpv1.ManagedResourceStatus `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,keycloak}
// +kubebuilder:storageversion
// +kubebuilder:group=user.keycloak.crossplane.io
// +kubebuilder:printcolumn:name="REALM",type="string",JSONPath=".spec.forProvider.realmId"
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// Groups manages the group memberships of a Keycloak user.
type Groups struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GroupsSpec   `json:"spec"`
	Status GroupsStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// GroupsList contains a list of Groups.
type GroupsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Groups `json:"items"`
}
