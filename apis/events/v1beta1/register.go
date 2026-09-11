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
	RealmEventsConfigKind             = reflect.TypeOf(RealmEventsConfig{}).Name()
	RealmEventsConfigGroupKind        = schema.GroupKind{Group: Group, Kind: RealmEventsConfigKind}
	RealmEventsConfigKindAPIVersion   = RealmEventsConfigKind + "." + SchemeGroupVersion.String()
	RealmEventsConfigGroupVersionKind = SchemeGroupVersion.WithKind(RealmEventsConfigKind)
)
