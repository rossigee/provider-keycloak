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
	AuthorizationPolicyKind             = reflect.TypeOf(AuthorizationPolicy{}).Name()
	AuthorizationPolicyGroupKind        = schema.GroupKind{Group: Group, Kind: AuthorizationPolicyKind}
	AuthorizationPolicyKindAPIVersion   = AuthorizationPolicyKind + "." + SchemeGroupVersion.String()
	AuthorizationPolicyGroupVersionKind = SchemeGroupVersion.WithKind(AuthorizationPolicyKind)
)
