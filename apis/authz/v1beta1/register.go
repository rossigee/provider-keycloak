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
	AuthzResourceKind             = reflect.TypeOf(AuthzResource{}).Name()
	AuthzResourceGroupKind        = schema.GroupKind{Group: Group, Kind: AuthzResourceKind}
	AuthzResourceKindAPIVersion   = AuthzResourceKind + "." + SchemeGroupVersion.String()
	AuthzResourceGroupVersionKind = SchemeGroupVersion.WithKind(AuthzResourceKind)
)
