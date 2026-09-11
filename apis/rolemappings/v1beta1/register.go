/*
Copyright 2024 The Crossplane Authors.
Licensed under the Apache License, Version 2.0.
*/

package v1beta1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"reflect"
)

var (
	ClientRoleMappingKind             = reflect.TypeOf(ClientRoleMapping{}).Name()
	ClientRoleMappingGroupKind        = schema.GroupKind{Group: Group, Kind: ClientRoleMappingKind}
	ClientRoleMappingKindAPIVersion   = ClientRoleMappingKind + "." + SchemeGroupVersion.String()
	ClientRoleMappingGroupVersionKind = SchemeGroupVersion.WithKind(ClientRoleMappingKind)
)
