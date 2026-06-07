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
	"encoding/json"
	"testing"

	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/rossigee/provider-keycloak/apis/v1beta1"
)

const (
	testKCURL    = "https://kc.example.com"
	testClientID = "crossplane"
	testClientPW = "test-client-password-value"
	jsonKeyURL   = "url"
)

func TestParseCredentials(t *testing.T) {
	tests := []struct {
		name    string
		input   map[string]interface{}
		wantURL string
		wantErr string
	}{
		{
			name: "full valid credentials",
			input: map[string]interface{}{
				jsonKeyURL: testKCURL, "base_path": "/auth",
				"realm": "myrealm", oauthKeyClientID: testClientID, oauthKeyClientSecret: testClientPW,
			},
			wantURL: "https://kc.example.com/auth",
		},
		{
			name: "defaults realm and base_path",
			input: map[string]interface{}{
				jsonKeyURL: testKCURL, oauthKeyClientID: testClientID, oauthKeyClientSecret: testClientPW,
			},
			wantURL: "https://kc.example.com/auth",
		},
		{
			name:    "missing url",
			input:   map[string]interface{}{oauthKeyClientID: "x", oauthKeyClientSecret: "y"},
			wantErr: "url",
		},
		{
			name:    "missing client_id",
			input:   map[string]interface{}{"url": "https://kc.example.com", oauthKeyClientSecret: "y"},
			wantErr: "client_id",
		},
		{
			name:    "missing client_secret",
			input:   map[string]interface{}{"url": "https://kc.example.com", "client_id": "x"},
			wantErr: "client_secret",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw, err := json.Marshal(tt.input)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			cfg, err := parseCredentials(raw)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !containsString(err.Error(), tt.wantErr) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.BaseURL != tt.wantURL {
				t.Errorf("BaseURL = %q, want %q", cfg.BaseURL, tt.wantURL)
			}
		})
	}
}

func TestParseCredentialsDefaults(t *testing.T) {
	raw, err := json.Marshal(map[string]interface{}{
		jsonKeyURL: testKCURL, "client_id": "x", oauthKeyClientSecret: "y",
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	cfg, err := parseCredentials(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Realm != defaultRealm {
		t.Errorf("default realm = %q, want %q", cfg.Realm, defaultRealm)
	}
	if cfg.BaseURL != testKCURL+"/auth" {
		t.Errorf("BaseURL = %q, want %q", cfg.BaseURL, testKCURL+"/auth")
	}
}

func TestParseCredentialsInvalidJSON(t *testing.T) {
	_, err := parseCredentials([]byte("not-json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestGetConfig(t *testing.T) {
	s := runtime.NewScheme()
	_ = corev1.AddToScheme(s)
	_ = v1beta1.SchemeBuilder.AddToScheme(s)

	validCreds, err := json.Marshal(ProviderCredentials{
		URL: testKCURL, BasePath: "/auth",
		Realm: defaultRealm, ClientID: "x", ClientSecret: "y",
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "my-secret", Namespace: "default"},
		Data:       map[string][]byte{"credentials": validCreds},
	}

	tests := []struct {
		name    string
		pc      *v1beta1.ProviderConfig
		objects []client.Object
		wantErr bool
	}{
		{
			name: "nil secretRef",
			pc: &v1beta1.ProviderConfig{
				Spec: v1beta1.ProviderConfigSpec{Credentials: v1beta1.ProviderCredentials{Source: "Secret"}},
			},
			wantErr: true,
		},
		{
			name: "secret not found",
			pc: &v1beta1.ProviderConfig{
				Spec: v1beta1.ProviderConfigSpec{Credentials: v1beta1.ProviderCredentials{
					CommonCredentialSelectors: xpv1.CommonCredentialSelectors{
						SecretRef: &xpv1.SecretKeySelector{
							SecretReference: xpv1.SecretReference{Name: "missing", Namespace: "default"},
							Key:             "credentials",
						},
					},
				}},
			},
			wantErr: true,
		},
		{
			name: "key not found in secret",
			pc: &v1beta1.ProviderConfig{
				Spec: v1beta1.ProviderConfigSpec{Credentials: v1beta1.ProviderCredentials{
					CommonCredentialSelectors: xpv1.CommonCredentialSelectors{
						SecretRef: &xpv1.SecretKeySelector{
							SecretReference: xpv1.SecretReference{Name: "my-secret", Namespace: "default"},
							Key:             "no-such-key",
						},
					},
				}},
			},
			objects: []client.Object{secret},
			wantErr: true,
		},
		{
			name: "valid secret",
			pc: &v1beta1.ProviderConfig{
				Spec: v1beta1.ProviderConfigSpec{Credentials: v1beta1.ProviderCredentials{
					CommonCredentialSelectors: xpv1.CommonCredentialSelectors{
						SecretRef: &xpv1.SecretKeySelector{
							SecretReference: xpv1.SecretReference{Name: "my-secret", Namespace: "default"},
							Key:             "credentials",
						},
					},
				}},
			},
			objects: []client.Object{secret},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kube := fake.NewClientBuilder().WithScheme(s).WithObjects(tt.objects...).Build()
			_, err := GetConfig(context.Background(), tt.pc, kube)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func containsString(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsAt(s, sub))
}

func containsAt(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
