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
	ClientCertificateKind             = reflect.TypeOf(ClientCertificate{}).Name()
	ClientCertificateGroupKind        = schema.GroupKind{Group: Group, Kind: ClientCertificateKind}
	ClientCertificateKindAPIVersion   = ClientCertificateKind + "." + SchemeGroupVersion.String()
	ClientCertificateGroupVersionKind = SchemeGroupVersion.WithKind(ClientCertificateKind)
)
