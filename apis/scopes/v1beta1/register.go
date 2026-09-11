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
	ClientScopeKind             = reflect.TypeOf(ClientScope{}).Name()
	ClientScopeGroupKind        = schema.GroupKind{Group: Group, Kind: ClientScopeKind}
	ClientScopeKindAPIVersion   = ClientScopeKind + "." + SchemeGroupVersion.String()
	ClientScopeGroupVersionKind = SchemeGroupVersion.WithKind(ClientScopeKind)
)
