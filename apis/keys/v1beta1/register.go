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
	RealmKeysKind             = reflect.TypeOf(RealmKeys{}).Name()
	RealmKeysGroupKind        = schema.GroupKind{Group: Group, Kind: RealmKeysKind}
	RealmKeysKindAPIVersion   = RealmKeysKind + "." + SchemeGroupVersion.String()
	RealmKeysGroupVersionKind = SchemeGroupVersion.WithKind(RealmKeysKind)
)
