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
	AuthenticationFlowKind             = reflect.TypeOf(AuthenticationFlow{}).Name()
	AuthenticationFlowGroupKind        = schema.GroupKind{Group: Group, Kind: AuthenticationFlowKind}
	AuthenticationFlowKindAPIVersion   = AuthenticationFlowKind + "." + SchemeGroupVersion.String()
	AuthenticationFlowGroupVersionKind = SchemeGroupVersion.WithKind(AuthenticationFlowKind)
)
