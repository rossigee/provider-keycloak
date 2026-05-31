//go:build !generate
// +build !generate

/*
Copyright 2024 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1beta1

import (
	"k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto copies all properties from source to target.
func (in *ProviderConfig) DeepCopyInto(target *ProviderConfig) {
	*target = *in
	target.TypeMeta = in.TypeMeta
	target.ObjectMeta = in.ObjectMeta
	target.Spec = in.Spec
	target.Status = in.Status
}

// DeepCopy creates a copy of the object.
func (in *ProviderConfig) DeepCopy() *ProviderConfig {
	if in == nil {
		return nil
	}
	out := new(ProviderConfig)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject returns a copy of the object.
func (in *ProviderConfig) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto copies all properties from source to target.
func (in *ProviderConfigList) DeepCopyInto(target *ProviderConfigList) {
	*target = *in
	target.TypeMeta = in.TypeMeta
	target.ListMeta = in.ListMeta
	if in.Items != nil {
		target.Items = make([]ProviderConfig, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&target.Items[i])
		}
	}
}

// DeepCopy creates a copy of the object.
func (in *ProviderConfigList) DeepCopy() *ProviderConfigList {
	if in == nil {
		return nil
	}
	out := new(ProviderConfigList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject returns a copy of the object.
func (in *ProviderConfigList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto copies all properties from source to target.
func (in *Client) DeepCopyInto(target *Client) {
	*target = *in
	target.TypeMeta = in.TypeMeta
	target.ObjectMeta = in.ObjectMeta
	target.Spec = in.Spec
	target.Status = in.Status
}

// DeepCopy creates a copy of the object.
func (in *Client) DeepCopy() *Client {
	if in == nil {
		return nil
	}
	out := new(Client)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject returns a copy of the object.
func (in *Client) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto copies all properties from source to target.
func (in *ClientList) DeepCopyInto(target *ClientList) {
	*target = *in
	target.TypeMeta = in.TypeMeta
	target.ListMeta = in.ListMeta
	if in.Items != nil {
		target.Items = make([]Client, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&target.Items[i])
		}
	}
}

// DeepCopy creates a copy of the object.
func (in *ClientList) DeepCopy() *ClientList {
	if in == nil {
		return nil
	}
	out := new(ClientList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject returns a copy of the object.
func (in *ClientList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}