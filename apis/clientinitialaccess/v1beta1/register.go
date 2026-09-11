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
	ClientInitialAccessKind             = reflect.TypeOf(ClientInitialAccess{}).Name()
	ClientInitialAccessGroupKind        = schema.GroupKind{Group: Group, Kind: ClientInitialAccessKind}
	ClientInitialAccessKindAPIVersion   = ClientInitialAccessKind + "." + SchemeGroupVersion.String()
	ClientInitialAccessGroupVersionKind = SchemeGroupVersion.WithKind(ClientInitialAccessKind)
)
