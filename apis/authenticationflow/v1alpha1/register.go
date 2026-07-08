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
	Group   = "authenticationflow.keycloak.crossplane.io"
	Version = "v1alpha1"
)

var (
	SchemeGroupVersion = schema.GroupVersion{Group: Group, Version: Version}
	SchemeBuilder      = runtime.NewSchemeBuilder(addKnownTypes)
	AddToScheme        = SchemeBuilder.AddToScheme
)

// AuthenticationFlow type metadata.
var (
	AuthenticationFlowKind             = reflect.TypeOf(AuthenticationFlow{}).Name()
	AuthenticationFlowGroupKind        = schema.GroupKind{Group: Group, Kind: AuthenticationFlowKind}.String()
	AuthenticationFlowKindAPIVersion   = AuthenticationFlowKind + "." + SchemeGroupVersion.String()
	AuthenticationFlowGroupVersionKind = SchemeGroupVersion.WithKind(AuthenticationFlowKind)
)

func addKnownTypes(s *runtime.Scheme) error {
	s.AddKnownTypes(SchemeGroupVersion,
		&AuthenticationFlow{},
		&AuthenticationFlowList{},
	)
	return nil
}
