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
	"reflect"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

// Package type metadata.
const (
	Group   = "authorizationpolicy.keycloak.crossplane.io"
	Version = "v1alpha1"
)

var (
	SchemeGroupVersion = schema.GroupVersion{Group: Group, Version: Version}
	SchemeBuilder      = &scheme.Builder{GroupVersion: SchemeGroupVersion}
	AddToScheme        = SchemeBuilder.AddToScheme
)

// AuthorizationPolicy type metadata.
var (
	AuthorizationPolicyKind             = reflect.TypeOf(AuthorizationPolicy{}).Name()
	AuthorizationPolicyGroupKind        = schema.GroupKind{Group: Group, Kind: AuthorizationPolicyKind}.String()
	AuthorizationPolicyKindAPIVersion   = AuthorizationPolicyKind + "." + SchemeGroupVersion.String()
	AuthorizationPolicyGroupVersionKind = SchemeGroupVersion.WithKind(AuthorizationPolicyKind)
)
