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
	"time"
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

			token, _, err := fetchOAuth2Token(context.Background(), srv.Client(), srv.URL, cfg)
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

const (
	testScopeName = "groups"
	testScopeUUID = "scope-uuid-1"
)

func TestListClientScopes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/client-scopes") || strings.Contains(r.URL.Path, "/clients/") {
			http.Error(w, "unexpected path "+r.URL.Path, http.StatusBadRequest)
			return
		}
		if err := json.NewEncoder(w).Encode([]ClientScopeRepresentation{
			{ID: "id-a", Name: "profile"},
			{ID: testScopeUUID, Name: testScopeName},
			{ID: "id-b", Name: "email"},
		}); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	kc := &keycloakClient{httpClient: srv.Client(), baseURL: srv.URL, token: testToken}
	scopes, err := kc.ListClientScopes(context.Background(), "myrealm")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(scopes) != 3 {
		t.Errorf("got %d scopes, want 3", len(scopes))
	}
	if scopes[1].Name != testScopeName || scopes[1].ID != testScopeUUID {
		t.Errorf("scope mismatch: %+v", scopes[1])
	}
}

func TestGetClientScopeByName(t *testing.T) {
	t.Run("found by name", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("clientId") != "" || strings.Contains(r.URL.Path, "/clients/") {
				http.Error(w, "must be list endpoint", http.StatusInternalServerError)
				return
			}
			if err := json.NewEncoder(w).Encode([]ClientScopeRepresentation{
				{ID: testScopeUUID, Name: testScopeName},
			}); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		}))
		defer srv.Close()

		kc := &keycloakClient{httpClient: srv.Client(), baseURL: srv.URL, token: testToken}
		got, err := kc.GetClientScope(context.Background(), "myrealm", testScopeName)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got == nil {
			t.Fatal("expected non-nil scope")
		}
		if got.ID != testScopeUUID {
			t.Errorf("ID = %q, want %q", got.ID, testScopeUUID)
		}
	})

	t.Run("not found returns nil without error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := json.NewEncoder(w).Encode([]ClientScopeRepresentation{
				{ID: "id-a", Name: "profile"},
			}); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		}))
		defer srv.Close()

		kc := &keycloakClient{httpClient: srv.Client(), baseURL: srv.URL, token: testToken}
		got, err := kc.GetClientScope(context.Background(), "myrealm", "missing")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != nil {
			t.Errorf("expected nil, got %+v", got)
		}
	})

	t.Run("404 responds nil without error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "not found", http.StatusNotFound)
		}))
		defer srv.Close()

		kc := &keycloakClient{httpClient: srv.Client(), baseURL: srv.URL, token: testToken}
		got, err := kc.GetClientScope(context.Background(), "myrealm", "anything")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != nil {
			t.Errorf("expected nil, got %+v", got)
		}
	})
}

func TestUpdateClientScopeUsesUUID(t *testing.T) {
	t.Run("uses supplied ID and resolves name when missing", func(t *testing.T) {
		var lastPath string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			lastPath = r.URL.Path
			// PUT must come second; the resolve GET returns the scope with UUID.
			if r.Method == http.MethodGet {
				if err := json.NewEncoder(w).Encode([]ClientScopeRepresentation{
					{ID: testScopeUUID, Name: testScopeName},
				}); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}
				return
			}
			w.WriteHeader(http.StatusNoContent)
		}))
		defer srv.Close()

		kc := &keycloakClient{httpClient: srv.Client(), baseURL: srv.URL, token: testToken}
		scope := ClientScopeRepresentation{Name: testScopeName} // no ID
		if err := kc.UpdateClientScope(context.Background(), "myrealm", scope); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(lastPath, testScopeUUID) {
			t.Errorf("expected path to contain %q, got %q", testScopeUUID, lastPath)
		}
	})

	t.Run("name lookup 404 returns errors", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.Contains(r.URL.Path, "/client-scopes") {
				http.Error(w, "bad path", http.StatusBadRequest)
				return
			}
			// Empty list - name not present.
			if err := json.NewEncoder(w).Encode([]ClientScopeRepresentation{}); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		}))
		defer srv.Close()

		kc := &keycloakClient{httpClient: srv.Client(), baseURL: srv.URL, token: testToken}
		// Update with a name but no ID should resolve the name first.
		err := kc.UpdateClientScope(context.Background(), "myrealm", ClientScopeRepresentation{Name: testScopeName})
		if err == nil {
			t.Fatal("expected 'not found' error")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("error %q does not contain 'not found'", err.Error())
		}
	})
}

func TestDeleteClientScopeUsesUUID(t *testing.T) {
	t.Run("resolves name to UUID before delete", func(t *testing.T) {
		var lastPath string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			lastPath = r.URL.Path
			// The resolve GET must return the scope first.
			if r.Method == http.MethodGet {
				if err := json.NewEncoder(w).Encode([]ClientScopeRepresentation{
					{ID: testScopeUUID, Name: testScopeName},
				}); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}
				return
			}
			w.WriteHeader(http.StatusNoContent)
		}))
		defer srv.Close()

		kc := &keycloakClient{httpClient: srv.Client(), baseURL: srv.URL, token: testToken}
		if err := kc.DeleteClientScope(context.Background(), "myrealm", testScopeName); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(lastPath, testScopeUUID) {
			t.Errorf("expected path to contain %q, got %q", testScopeUUID, lastPath)
		}
	})

	t.Run("absent scope is a no-op", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := json.NewEncoder(w).Encode([]ClientScopeRepresentation{}); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		}))
		defer srv.Close()

		kc := &keycloakClient{httpClient: srv.Client(), baseURL: srv.URL, token: testToken}
		if err := kc.DeleteClientScope(context.Background(), "myrealm", "missing"); err != nil {
			t.Fatalf("expected nil error for missing scope, got %v", err)
		}
	})
}

func TestListClientDefaultScopesUsesUUIDEndpoint(t *testing.T) {
	var lastPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastPath = r.URL.Path
		if !strings.Contains(lastPath, "/clients/"+testClientUUID+"/default-client-scopes") {
			http.Error(w, "expected UUID-based path, got "+lastPath, http.StatusBadRequest)
			return
		}
		if err := json.NewEncoder(w).Encode([]ClientScopeRepresentation{}); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	kc := &keycloakClient{httpClient: srv.Client(), baseURL: srv.URL, token: testToken}
	if _, err := kc.ListClientDefaultScopes(context.Background(), "myrealm", testClientUUID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(lastPath, testClientUUID) {
		t.Errorf("path did not contain UUID: %q", lastPath)
	}
}

// Rate Limit Tests

func TestParseRetryAfter(t *testing.T) {
	tests := []struct {
		name     string
		header   string
		expected int64 // seconds
	}{
		{"delta-seconds: 60", "60", 60},
		{"delta-seconds: 1", "1", 1},
		{"delta-seconds: 0 is invalid", "0", 0},
		{"invalid number returns 0", "abc", 0},
		{"empty string returns 0", "", 0},
		{"negative number returns 0", "-5", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseRetryAfter(tt.header)
			expected := time.Duration(tt.expected) * time.Second
			if got != expected {
				t.Errorf("parseRetryAfter(%q) = %v, want %v", tt.header, got, expected)
			}
		})
	}
}

func TestRateLimitBackoffTracking(t *testing.T) {
	kc := &keycloakClient{token: testToken, baseURL: "https://test.example.com"}

	// Initially no backoff
	wait, err := kc.checkRateLimitBackoff()
	if err != nil {
		t.Fatalf("unexpected error checking initial backoff: %v", err)
	}
	if wait != 0 {
		t.Errorf("expected no initial backoff, got %v", wait)
	}

	// Record a 429 with Retry-After header
	kc.recordRateLimitHit(10 * time.Second)

	// Should now report backoff
	wait, err = kc.checkRateLimitBackoff()
	if err == nil {
		t.Fatal("expected ErrRateLimited when in backoff")
	}
	if !strings.Contains(err.Error(), "rate limited") {
		t.Errorf("error should mention rate limiting, got: %v", err)
	}
	if wait < 9*time.Second || wait > 11*time.Second {
		t.Errorf("expected ~10s backoff, got %v", wait)
	}

	// Clear backoff
	kc.clearRateLimitBackoff()
	wait, err = kc.checkRateLimitBackoff()
	if err != nil {
		t.Fatalf("unexpected error after clearing backoff: %v", err)
	}
	if wait != 0 {
		t.Errorf("expected no backoff after clear, got %v", wait)
	}
}

func TestRateLimitExponentialBackoff(t *testing.T) {
	kc := &keycloakClient{token: testToken, baseURL: "https://test.example.com"}

	// Consecutive hits without Retry-After should use exponential backoff
	// Hit 1: 1s * 2^(1-1) = 1s
	kc.recordRateLimitHit(0)
	wait, err := kc.checkRateLimitBackoff()
	if err == nil {
		t.Fatal("expected backoff after 1st hit")
	}
	if wait < 900*time.Millisecond || wait > 1100*time.Millisecond {
		t.Errorf("expected ~1s after 1st hit, got %v", wait)
	}

	kc.clearRateLimitBackoff()

	// Hit 2: 1s * 2^(2-1) = 2s
	kc.recordRateLimitHit(0)
	kc.recordRateLimitHit(0)
	wait, err = kc.checkRateLimitBackoff()
	if err == nil {
		t.Fatal("expected backoff after 2nd hit")
	}
	if wait < 1900*time.Millisecond || wait > 2100*time.Millisecond {
		t.Errorf("expected ~2s after 2nd hit, got %v", wait)
	}

	kc.clearRateLimitBackoff()

	// Hit 3: 1s * 2^(3-1) = 4s
	for i := 0; i < 3; i++ {
		kc.recordRateLimitHit(0)
	}
	wait, err = kc.checkRateLimitBackoff()
	if err == nil {
		t.Fatal("expected backoff after 3rd hit")
	}
	if wait < 3900*time.Millisecond || wait > 4100*time.Millisecond {
		t.Errorf("expected ~4s after 3rd hit, got %v", wait)
	}
}

func TestRateLimitRetryAfterCap(t *testing.T) {
	kc := &keycloakClient{token: testToken, baseURL: "https://test.example.com"}

	// Retry-After larger than max should be capped at 30s
	kc.recordRateLimitHit(120 * time.Second)

	wait, err := kc.checkRateLimitBackoff()
	if err == nil {
		t.Fatal("expected ErrRateLimited")
	}
	if wait > 31*time.Second {
		t.Errorf("expected backoff capped at 30s, got %v", wait)
	}
}

func TestDoRequestHandles429(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		// Handle token endpoint
		if strings.Contains(r.URL.Path, "/protocol/openid-connect/token") {
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(tokenResponse{
				AccessToken: "test-token",
				ExpiresIn:   3600,
			}); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}

		// Rate limit the first call
		if callCount == 2 {
			w.Header().Set("Retry-After", "1")
			http.Error(w, "rate limited", http.StatusTooManyRequests)
			return
		}
		// Subsequent calls succeed
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]string{"realm": "test"}); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	cfg := &Config{
		BaseURL:      srv.URL,
		Realm:        "test",
		ClientID:     "test-client",
		ClientSecret: "secret",
	}

	kc, err := NewClientFromConfig(context.Background(), cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// First request should get 429 and trigger backoff
	_, err = kc.doRequest(context.Background(), http.MethodGet, "/admin/realms/test", nil)
	if err == nil {
		t.Fatal("expected error on 429 response")
	}
	if !strings.Contains(err.Error(), "rate limited") {
		t.Errorf("expected rate limited error, got: %v", err)
	}

	// Second request should fail due to backoff still being active
	_, err = kc.doRequest(context.Background(), http.MethodGet, "/admin/realms/test", nil)
	if err == nil {
		t.Fatal("expected error due to active backoff")
	}
	if !strings.Contains(err.Error(), "rate limit backoff") {
		t.Errorf("expected backoff error, got: %v", err)
	}

	// Wait for backoff to clear and try again
	time.Sleep(1100 * time.Millisecond)
	_, err = kc.doRequest(context.Background(), http.MethodGet, "/admin/realms/test", nil)
	if err != nil {
		t.Fatalf("expected success after backoff cleared, got: %v", err)
	}
}

func TestDoCreateHandles429(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		// Handle token endpoint
		if strings.Contains(r.URL.Path, "/protocol/openid-connect/token") {
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(tokenResponse{
				AccessToken: "test-token",
				ExpiresIn:   3600,
			}); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}

		// Rate limit the first client creation call
		if callCount == 2 {
			w.Header().Set("Retry-After", "1")
			http.Error(w, "rate limited", http.StatusTooManyRequests)
			return
		}
		// Subsequent calls succeed
		w.Header().Set("Location", "/admin/realms/test/clients/new-uuid")
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	cfg := &Config{
		BaseURL:      srv.URL,
		Realm:        "test",
		ClientID:     "test-client",
		ClientSecret: "secret",
	}

	kc, err := NewClientFromConfig(context.Background(), cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// First request should get 429 and trigger backoff
	_, err = kc.doCreate(context.Background(), "/admin/realms/test/clients", map[string]string{"clientId": "test"})
	if err == nil {
		t.Fatal("expected error on 429 response")
	}
	if !strings.Contains(err.Error(), "rate limited") {
		t.Errorf("expected rate limited error, got: %v", err)
	}

	// Second request should fail due to active backoff
	_, err = kc.doCreate(context.Background(), "/admin/realms/test/clients", map[string]string{"clientId": "test"})
	if err == nil {
		t.Fatal("expected error due to active backoff")
	}

	// Wait for backoff and retry
	time.Sleep(1100 * time.Millisecond)
	id, err := kc.doCreate(context.Background(), "/admin/realms/test/clients", map[string]string{"clientId": "test"})
	if err != nil {
		t.Fatalf("expected success after backoff cleared, got: %v", err)
	}
	if id != "new-uuid" {
		t.Errorf("expected extracted UUID, got %q", id)
	}
}
