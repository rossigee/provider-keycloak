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
	RealmImportKind             = reflect.TypeOf(RealmImport{}).Name()
	RealmImportGroupKind        = schema.GroupKind{Group: Group, Kind: RealmImportKind}
	RealmImportKindAPIVersion   = RealmImportKind + "." + SchemeGroupVersion.String()
	RealmImportGroupVersionKind = SchemeGroupVersion.WithKind(RealmImportKind)
)
