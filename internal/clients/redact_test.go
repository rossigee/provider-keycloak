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
	"net/http"
	"strings"
	"testing"
)

func TestRedactSensitiveJSONRemovesCredentialValues(t *testing.T) {
	cases := map[string]struct {
		in       string
		mustNot  []string
		mustKeep []string
	}{
		"keycloak reset password payload": {
			in:       `{"realmId":"r","username":"u","password":"hunter2","temporary":false}`,
			mustNot:  []string{"hunter2"},
			mustKeep: []string{"REDACTED", `"username":"u"`, `"realmId":"r"`},
		},
		"client secret": {
			in:       `{"clientId":"c","clientSecret":"s3cr3t","publicClient":false}`,
			mustNot:  []string{"s3cr3t"},
			mustKeep: []string{"REDACTED", `"clientId":"c"`},
		},
		"snake_case token": {
			in:      `{"access_token":"eyJhbGciOi","refresh_token":"r3fr3sh"}`,
			mustNot: []string{"eyJhbGciOi", "r3fr3sh"},
		},
		"private key": {
			in:      `{"privateKey":"-----BEGIN PRIVATE KEY-----abc"}`,
			mustNot: []string{"BEGIN PRIVATE KEY"},
		},
		"whitespace around colon": {
			in:      `{"password" : "hunter2"}`,
			mustNot: []string{"hunter2"},
		},
		"uppercase field name": {
			in:      `{"PASSWORD":"hunter2"}`,
			mustNot: []string{"hunter2"},
		},
		"token": {
			in:      `{"accessToken":"eyJhbGciOi","refreshToken":"r3fr3sh"}`,
			mustNot: []string{"eyJhbGciOi", "r3fr3sh"},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := string(redactSensitiveJSON([]byte(tc.in)))
			for _, secret := range tc.mustNot {
				if strings.Contains(got, secret) {
					t.Errorf("redacted output still contains %q:\n%s", secret, got)
				}
			}
			for _, keep := range tc.mustKeep {
				if !strings.Contains(got, keep) {
					t.Errorf("redacted output lost %q, which is needed for debugging:\n%s", keep, got)
				}
			}
		})
	}
}

func TestRedactSensitiveJSONLeavesNonSecretsAlone(t *testing.T) {
	in := `{"realmId":"master","enabled":true,"description":"a \"quoted\" value","redirectUris":["https://x"]}`
	got := string(redactSensitiveJSON([]byte(in)))
	if got != in {
		t.Errorf("non-sensitive payload was modified:\n got: %s\nwant: %s", got, in)
	}
}

// TestRedactSensitiveJSONKeepsGenericValueFields records a deliberate scope
// decision. A nested Keycloak credentials array carries its secret in a field
// called "value", which is far too common a name to redact - doing so would
// strip most of the payload and leave the dump useless. It is safe to skip
// because this provider's request types have no Credentials field, so it never
// puts one on the wire.
func TestRedactSensitiveJSONKeepsGenericValueFields(t *testing.T) {
	in := `{"type":"password","value":"kept-because-out-of-scope"}`
	got := string(redactSensitiveJSON([]byte(in)))
	if got != in {
		t.Errorf("generic value fields must not be redacted:\n got: %s\nwant: %s", got, in)
	}
}

func TestRedactSensitiveJSONHandlesNonJSON(t *testing.T) {
	in := []byte("not json at all")
	if got := string(redactSensitiveJSON(in)); got != string(in) {
		t.Errorf("non-JSON body should pass through unchanged, got %q", got)
	}
}

func TestRedactHeadersRemovesCredentials(t *testing.T) {
	h := http.Header{}
	h.Set("Authorization", "Bearer super-secret-token")
	h.Set("Content-Type", "application/json")

	got := redactHeaders(h)

	if strings.Contains(strings.Join(got.Values("Authorization"), ","), "super-secret-token") {
		t.Errorf("bearer token survived redaction: %v", got)
	}
	if got.Get("Authorization") != "REDACTED" {
		t.Errorf("Authorization = %q, want REDACTED", got.Get("Authorization"))
	}
	if got.Get("Content-Type") != "application/json" {
		t.Errorf("harmless header was altered: %v", got)
	}
}

// TestRedactHeadersDoesNotMutateInput is the property the wire dump depends on:
// it clones req.Header and redacts the clone, so the real request keeps its
// Authorization. Mutating in place would break every authenticated call.
func TestRedactHeadersDoesNotMutateInput(t *testing.T) {
	h := http.Header{}
	h.Set("Authorization", "Bearer super-secret-token")

	_ = redactHeaders(h)

	if h.Get("Authorization") != "Bearer super-secret-token" {
		t.Errorf("input was mutated, Authorization = %q", h.Get("Authorization"))
	}
}

func TestRedactHeadersCoversCookieAndProxyAuthorization(t *testing.T) {
	h := http.Header{}
	h.Set("Cookie", "session=abc123")
	h.Set("Proxy-Authorization", "Basic dXNlcjpwYXNz")
	h.Set("Set-Cookie", "session=abc123")

	got := redactHeaders(h)
	for _, name := range []string{"Cookie", "Proxy-Authorization", "Set-Cookie"} {
		if v := got.Get(name); v != "REDACTED" {
			t.Errorf("%s = %q, want REDACTED", name, v)
		}
	}
}

func TestRedactHeadersNilAndAbsent(t *testing.T) {
	if got := redactHeaders(nil); got != nil {
		t.Errorf("redactHeaders(nil) = %v, want nil", got)
	}
	if got := redactHeaders(http.Header{}); len(got) != 0 {
		t.Errorf("empty headers should stay empty, got %v", got)
	}
}
