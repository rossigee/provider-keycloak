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

// +groupName=events.keycloak.crossplane.io

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
)

// RealmEventsConfigParameters are the configurable fields of RealmEventsConfig.
type RealmEventsConfigParameters struct {
	// RealmId is the ID of the realm.
	// +kubebuilder:validation:Required
	RealmId string `json:"realmId"`

	// EventsEnabled enables events.
	// +optional
	EventsEnabled *bool `json:"eventsEnabled,omitempty"`

	// EventsExpiration is the expiration time for events in seconds.
	// +optional
	EventsExpiration *int64 `json:"eventsExpiration,omitempty"`

	// EventsListeners is the list of event listeners.
	// +optional
	EventsListeners []string `json:"eventsListeners,omitempty"`

	// EnabledEvents is the list of enabled event types.
	// +optional
	EnabledEvents []string `json:"enabledEvents,omitempty"`

	// AdminEventsEnabled enables admin events.
	// +optional
	AdminEventsEnabled *bool `json:"adminEventsEnabled,omitempty"`

	// AdminEventsDetailsEnabled includes details in admin events.
	// +optional
	AdminEventsDetailsEnabled *bool `json:"adminEventsDetailsEnabled,omitempty"`
}

// RealmEventsConfigSpec defines the desired state of RealmEventsConfig.
type RealmEventsConfigSpec struct {
	xpv1.ManagedResourceSpec `json:",inline"`
	ForProvider              RealmEventsConfigParameters `json:"forProvider"`
}

// RealmEventsConfigStatus defines the observed state of RealmEventsConfig.
type RealmEventsConfigStatus struct {
	xpv1.ManagedResourceStatus `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,keycloak}
// +kubebuilder:storageversion

// RealmEventsConfig manages realm events configuration.
type RealmEventsConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RealmEventsConfigSpec   `json:"spec"`
	Status RealmEventsConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RealmEventsConfigList contains a list of RealmEventsConfig.
type RealmEventsConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RealmEventsConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RealmEventsConfig{}, &RealmEventsConfigList{})
}