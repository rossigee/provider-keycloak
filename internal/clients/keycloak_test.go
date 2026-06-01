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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const (
	testRealm      = "master"
	testToken      = "tok"
	testClientUUID = "uuid-1"
	testClientName = "my-app"
)

func TestFetchOAuth2Token(t *testing.T) {
	tests := []struct {
		name       string
		handler    http.HandlerFunc
		wantToken  string
		wantErrStr string
	}{
		{
			name: "successful token",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
					return
				}
				if err := r.ParseForm(); err != nil {
					http.Error(w, "bad form", http.StatusBadRequest)
					return
				}
				if r.Form.Get("grant_type") != "client_credentials" {
					http.Error(w, "wrong grant_type", http.StatusBadRequest)
					return
				}
				if err := json.NewEncoder(w).Encode(tokenResponse{AccessToken: "tok-abc123"}); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}
			},
			wantToken: "tok-abc123",
		},
		{
			name: "server returns oauth2 error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if err := json.NewEncoder(w).Encode(tokenResponse{
					Error:     "invalid_client",
					ErrorDesc: "bad credentials",
				}); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}
			},
			wantErrStr: "invalid_client",
		},
		{
			name: "empty access_token",
			handler: func(w http.ResponseWriter, r *http.Request) {
				if err := json.NewEncoder(w).Encode(tokenResponse{}); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}
			},
			wantErrStr: "no access_token",
		},
		{
			name: "server returns 500",
			handler: func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "internal server error", http.StatusInternalServerError)
			},
			wantErrStr: "parse token response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(tt.handler)
			defer srv.Close()

			cfg := &Config{
				BaseURL:      srv.URL,
				Realm:        testRealm,
				ClientID:     "crossplane",
				ClientSecret: "secret",
			}

			token, err := fetchOAuth2Token(context.Background(), srv.Client(), srv.URL, cfg)
			if tt.wantErrStr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErrStr)
				}
				if !strings.Contains(err.Error(), tt.wantErrStr) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.wantErrStr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if token != tt.wantToken {
				t.Errorf("token = %q, want %q", token, tt.wantToken)
			}
		})
	}
}

func TestNewClientFromConfig(t *testing.T) {
	// Server that provides a valid token and responds to admin API calls.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "token") {
			if err := json.NewEncoder(w).Encode(tokenResponse{AccessToken: "test-token"}); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := &Config{
		BaseURL:      srv.URL,
		Realm:        testRealm,
		ClientID:     "crossplane",
		ClientSecret: "secret",
	}

	kc, err := NewClientFromConfig(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if kc == nil {
		t.Fatal("expected non-nil client")
	}
	if kc.token != "test-token" {
		t.Errorf("token = %q, want %q", kc.token, "test-token")
	}
}

func TestNewClientFromConfigTokenFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewEncoder(w).Encode(tokenResponse{Error: "unauthorized", ErrorDesc: "bad creds"}); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	cfg := &Config{
		BaseURL:      srv.URL,
		Realm:        testRealm,
		ClientID:     "x",
		ClientSecret: "y",
	}

	_, err := NewClientFromConfig(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "unauthorized") {
		t.Errorf("error %q does not contain %q", err.Error(), "unauthorized")
	}
}

func TestGetClient(t *testing.T) {
	tests := []struct {
		name       string
		response   interface{}
		statusCode int
		wantNil    bool
		wantErrStr string
	}{
		{
			name:       "client found",
			response:   []ClientRepresentation{{ID: testClientUUID, ClientID: testClientName}},
			statusCode: http.StatusOK,
			wantNil:    false,
		},
		{
			name:       "client not found (empty list)",
			response:   []ClientRepresentation{},
			statusCode: http.StatusOK,
			wantNil:    true,
		},
		{
			name:       "server error",
			statusCode: http.StatusInternalServerError,
			wantErrStr: "500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				if tt.response != nil {
					if err := json.NewEncoder(w).Encode(tt.response); err != nil {
						http.Error(w, err.Error(), http.StatusInternalServerError)
					}
				}
			}))
			defer srv.Close()

			kc := &keycloakClient{httpClient: srv.Client(), baseURL: srv.URL, token: testToken}
			result, err := kc.GetClient(context.Background(), "myrealm", testClientName)

			if tt.wantErrStr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErrStr)
				}
				if !strings.Contains(err.Error(), tt.wantErrStr) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.wantErrStr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantNil && result != nil {
				t.Error("expected nil result")
			}
			if !tt.wantNil && result == nil {
				t.Error("expected non-nil result")
			}
			if !tt.wantNil && result != nil && result.ClientID != "my-app" {
				t.Errorf("ClientID = %q, want %q", result.ClientID, "my-app")
			}
		})
	}
}

func TestCreateClient(t *testing.T) {
	t.Run("empty body returns no error and zero ID", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusCreated)
		}))
		defer srv.Close()

		kc := &keycloakClient{httpClient: srv.Client(), baseURL: srv.URL, token: testToken}
		created, err := kc.CreateClient(context.Background(), "myrealm", &ClientRepresentation{ClientID: "new-app"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if created == nil {
			t.Fatal("expected non-nil result")
		}
		if created.ID != "" {
			t.Errorf("expected empty ID from 201 with empty body, got %q", created.ID)
		}
	})

	t.Run("server error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "conflict", http.StatusConflict)
		}))
		defer srv.Close()

		kc := &keycloakClient{httpClient: srv.Client(), baseURL: srv.URL, token: testToken}
		_, err := kc.CreateClient(context.Background(), "myrealm", &ClientRepresentation{ClientID: "dup"})
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "409") {
			t.Errorf("error %q does not contain status code", err.Error())
		}
	})
}

func TestCreateClientLocation(t *testing.T) {
	t.Run("Location header UUID is captured", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Location", "http://"+r.Host+"/admin/realms/myrealm/clients/abc-123-uuid")
			w.WriteHeader(http.StatusCreated)
		}))
		defer srv.Close()

		kc := &keycloakClient{httpClient: srv.Client(), baseURL: srv.URL, token: testToken}
		created, err := kc.CreateClient(context.Background(), "myrealm", &ClientRepresentation{ClientID: "new-app"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if created.ID != "abc-123-uuid" {
			t.Errorf("ID = %q, want %q", created.ID, "abc-123-uuid")
		}
	})

	t.Run("no Location header returns empty ID", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusCreated)
		}))
		defer srv.Close()

		kc := &keycloakClient{httpClient: srv.Client(), baseURL: srv.URL, token: testToken}
		created, err := kc.CreateClient(context.Background(), "myrealm", &ClientRepresentation{ClientID: "new-app"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if created.ID != "" {
			t.Errorf("expected empty ID, got %q", created.ID)
		}
	})
}

func TestErrorBodyTruncation(t *testing.T) {
	longBody := strings.Repeat("x", maxErrBodyLen*2)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, longBody, http.StatusInternalServerError)
	}))
	defer srv.Close()

	kc := &keycloakClient{httpClient: srv.Client(), baseURL: srv.URL, token: testToken}
	_, err := kc.GetClient(context.Background(), "myrealm", testClientName)
	if err == nil {
		t.Fatal("expected error")
	}
	// Error message must not contain the full body.
	if len(err.Error()) > maxErrBodyLen+100 {
		t.Errorf("error message too long (%d bytes), body was not truncated", len(err.Error()))
	}
	if !strings.Contains(err.Error(), "...") {
		t.Error("expected truncation marker '...' in error message")
	}
}

func TestURLEncoding(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.RawPath
		if gotPath == "" {
			gotPath = r.URL.Path
		}
		if err := json.NewEncoder(w).Encode([]ClientRepresentation{}); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	kc := &keycloakClient{httpClient: srv.Client(), baseURL: srv.URL, token: testToken}

	// Realm name containing a slash would break the path without encoding.
	_, _ = kc.GetClient(context.Background(), "my/realm", "client&id=evil")

	if strings.Contains(gotPath, "my/realm") && !strings.Contains(gotPath, "my%2Frealm") {
		t.Errorf("realm slash was not path-encoded: %s", gotPath)
	}
}

func TestDeleteClient(t *testing.T) {
	t.Run("successful deletion", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}))
		defer srv.Close()

		kc := &keycloakClient{httpClient: srv.Client(), baseURL: srv.URL, token: testToken}
		if err := kc.DeleteClient(context.Background(), "myrealm", "uuid-1"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("404 not found", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "not found", http.StatusNotFound)
		}))
		defer srv.Close()

		kc := &keycloakClient{httpClient: srv.Client(), baseURL: srv.URL, token: testToken}
		err := kc.DeleteClient(context.Background(), "myrealm", "uuid-1")
		if err == nil {
			t.Fatal("expected error from 404")
		}
		if !strings.Contains(err.Error(), "404") {
			t.Errorf("error %q does not contain 404", err.Error())
		}
	})
}
