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

package v1beta1

import (
	"k8s.io/apimachinery/pkg/runtime"
)

// ProviderConfigTypeMeta returns the ProviderConfig TypeMeta.
var ProviderConfigTypeMeta = runtime.TypeMeta{
	APIVersion: "keycloak.crossplane.io/v1beta1",
	Kind:       "ProviderConfig",
}

// ProviderConfigUsageTypeMeta returns the ProviderConfigUsage TypeMeta.
var ProviderConfigUsageTypeMeta = runtime.TypeMeta{
	APIVersion: "keycloak.crossplane.io/v1beta1",
	Kind:       "ProviderConfigUsage",
}

// ClientTypeMeta returns the Client TypeMeta.
var ClientTypeMeta = runtime.TypeMeta{
	APIVersion: "keycloak.crossplane.io/v1beta1",
	Kind:       "Client",
}

// UserTypeMeta returns the User TypeMeta.
var UserTypeMeta = runtime.TypeMeta{
	APIVersion: "keycloak.crossplane.io/v1beta1",
	Kind:       "User",
}

// GroupTypeMeta returns the Group TypeMeta.
var GroupTypeMeta = runtime.TypeMeta{
	APIVersion: "keycloak.crossplane.io/v1beta1",
	Kind:       "Group",
}

// RealmTypeMeta returns the Realm TypeMeta.
var RealmTypeMeta = runtime.TypeMeta{
	APIVersion: "keycloak.crossplane.io/v1beta1",
	Kind:       "Realm",
}