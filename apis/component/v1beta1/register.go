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
	ComponentKind             = reflect.TypeOf(Component{}).Name()
	ComponentGroupKind        = schema.GroupKind{Group: Group, Kind: ComponentKind}
	ComponentKindAPIVersion   = ComponentKind + "." + SchemeGroupVersion.String()
	ComponentGroupVersionKind = SchemeGroupVersion.WithKind(ComponentKind)
)
