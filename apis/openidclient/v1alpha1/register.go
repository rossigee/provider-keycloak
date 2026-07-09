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

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	Group   = "openidclient.keycloak.crossplane.io"
	Version = "v1alpha1"
)

var (
	SchemeGroupVersion = schema.GroupVersion{Group: Group, Version: Version}
	SchemeBuilder      = runtime.NewSchemeBuilder(addKnownTypes)
	AddToScheme        = SchemeBuilder.AddToScheme
)

// Client type metadata.
var (
	ClientKind             = reflect.TypeOf(Client{}).Name()
	ClientGroupKind        = schema.GroupKind{Group: Group, Kind: ClientKind}.String()
	ClientKindAPIVersion   = ClientKind + "." + SchemeGroupVersion.String()
	ClientGroupVersionKind = SchemeGroupVersion.WithKind(ClientKind)
)

// ClientDefaultScopes type metadata.
var (
	ClientDefaultScopesKind             = reflect.TypeOf(ClientDefaultScopes{}).Name()
	ClientDefaultScopesGroupKind        = schema.GroupKind{Group: Group, Kind: ClientDefaultScopesKind}.String()
	ClientDefaultScopesKindAPIVersion   = ClientDefaultScopesKind + "." + SchemeGroupVersion.String()
	ClientDefaultScopesGroupVersionKind = SchemeGroupVersion.WithKind(ClientDefaultScopesKind)
)

// ClientOptionalScopes type metadata.
var (
	ClientOptionalScopesKind             = reflect.TypeOf(ClientOptionalScopes{}).Name()
	ClientOptionalScopesGroupKind        = schema.GroupKind{Group: Group, Kind: ClientOptionalScopesKind}.String()
	ClientOptionalScopesKindAPIVersion   = ClientOptionalScopesKind + "." + SchemeGroupVersion.String()
	ClientOptionalScopesGroupVersionKind = SchemeGroupVersion.WithKind(ClientOptionalScopesKind)
)

func addKnownTypes(s *runtime.Scheme) error {
	s.AddKnownTypes(SchemeGroupVersion,
		&Client{},
		&ClientList{},
		&ClientDefaultScopes{},
		&ClientDefaultScopesList{},
		&ClientOptionalScopes{},
		&ClientOptionalScopesList{},
	)
	return nil
}

