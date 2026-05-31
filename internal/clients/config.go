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

package clients

import (
	"context"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/rossigee/provider-keycloak/apis/v1beta1"
)

// Config contains the credentials and connection details for the Keycloak API.
type Config struct {
	BaseURL  string
	Token    string
	Insecure bool
}

// GetConfig extracts the Keycloak connection config from a ProviderConfig.
func GetConfig(ctx context.Context, pc *v1beta1.ProviderConfig, kube client.Client) (*Config, error) {
	if pc.Spec.Credentials.SecretRef == nil {
		return nil, errors.New("credentials.secretRef is required")
	}

	secret := &corev1.Secret{}
	nn := types.NamespacedName{
		Name:      pc.Spec.Credentials.SecretRef.Name,
		Namespace: pc.Spec.Credentials.SecretRef.Namespace,
	}
	if err := kube.Get(ctx, nn, secret); err != nil {
		return nil, errors.Wrap(err, "cannot get credentials secret")
	}

	key := pc.Spec.Credentials.SecretRef.Key
	if key == "" {
		key = "token"
	}

	tokenBytes, ok := secret.Data[key]
	if !ok {
		return nil, errors.Errorf("key %q not found in credentials secret", key)
	}

	insecure := false
	if pc.Spec.Insecure != nil {
		insecure = *pc.Spec.Insecure
	}

	return &Config{
		BaseURL:  pc.Spec.BaseURL,
		Token:    string(tokenBytes),
		Insecure: insecure,
	}, nil
}
