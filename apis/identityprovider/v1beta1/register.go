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
	IdentityProviderKind             = reflect.TypeOf(IdentityProvider{}).Name()
	IdentityProviderGroupKind        = schema.GroupKind{Group: Group, Kind: IdentityProviderKind}
	IdentityProviderKindAPIVersion   = IdentityProviderKind + "." + SchemeGroupVersion.String()
	IdentityProviderGroupVersionKind = SchemeGroupVersion.WithKind(IdentityProviderKind)
)
