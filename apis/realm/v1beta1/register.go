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
	RealmKind             = reflect.TypeOf(Realm{}).Name()
	RealmGroupKind        = schema.GroupKind{Group: Group, Kind: RealmKind}
	RealmKindAPIVersion   = RealmKind + "." + SchemeGroupVersion.String()
	RealmGroupVersionKind = SchemeGroupVersion.WithKind(RealmKind)
)
