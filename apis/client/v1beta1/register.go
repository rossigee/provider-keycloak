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
	ProtocolMapperKind             = reflect.TypeOf(ProtocolMapper{}).Name()
	ProtocolMapperGroupKind        = schema.GroupKind{Group: Group, Kind: ProtocolMapperKind}
	ProtocolMapperKindAPIVersion   = ProtocolMapperKind + "." + SchemeGroupVersion.String()
	ProtocolMapperGroupVersionKind = SchemeGroupVersion.WithKind(ProtocolMapperKind)
)
