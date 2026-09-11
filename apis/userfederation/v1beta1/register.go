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
	UserFederationProviderKind             = reflect.TypeOf(UserFederationProvider{}).Name()
	UserFederationProviderGroupKind        = schema.GroupKind{Group: Group, Kind: UserFederationProviderKind}
	UserFederationProviderKindAPIVersion   = UserFederationProviderKind + "." + SchemeGroupVersion.String()
	UserFederationProviderGroupVersionKind = SchemeGroupVersion.WithKind(UserFederationProviderKind)
)
