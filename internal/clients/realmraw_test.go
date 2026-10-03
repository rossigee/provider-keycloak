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
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// UpdateRealmRaw inlines the whole request flow rather than going through
// doRequest, which is why it was missed when rate-limit tracking was added to
// doRequest in v0.19.2. These tests pin the behaviour that fix established.

func realmRawServer(t *testing.T, handler http.HandlerFunc) (*keycloakClient, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return newTestKeycloakClient(srv.Client(), srv.URL, "tok"), srv
}

func TestUpdateRealmRawPutsToRealmPath(t *testing.T) {
	var gotMethod, gotPath, gotAuth, gotBody string
	kc, _ := realmRawServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		b := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(b)
		gotBody = string(b)
		w.WriteHeader(http.StatusOK)
	})

	if err := kc.UpdateRealmRaw(context.Background(), "master", []byte(`{"realm":"master"}`)); err != nil {
		t.Fatalf("UpdateRealmRaw failed: %v", err)
	}
	if gotMethod != http.MethodPut {
		t.Errorf("method = %s, want PUT", gotMethod)
	}
	if want := "/admin/realms/master"; gotPath != want {
		t.Errorf("path = %s, want %s", gotPath, want)
	}
	if gotAuth != "Bearer tok" {
		t.Errorf("Authorization = %q, want the bearer token", gotAuth)
	}
	if gotBody != `{"realm":"master"}` {
		t.Errorf("body = %q, want the realm JSON verbatim", gotBody)
	}
}

// TestUpdateRealmRawRecordsRateLimit is the regression test for the v0.19.2 fix:
// a 429 here used to be returned as a plain error, so the backoff was never
// recorded and every caller hammered a rate-limited endpoint.
func TestUpdateRealmRawRecordsRateLimit(t *testing.T) {
	kc, _ := realmRawServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "3")
		w.WriteHeader(http.StatusTooManyRequests)
	})

	err := kc.UpdateRealmRaw(context.Background(), "master", []byte(`{}`))
	if err == nil {
		t.Fatal("a 429 must be surfaced as an error")
	}

	var rle *RateLimitError
	if !errors.As(err, &rle) {
		t.Fatalf("error = %T (%v), want *RateLimitError so the reconciler can respect the deadline", err, err)
	}
	if kc.rateLimitUntil.IsZero() {
		t.Error("a 429 must record a rate-limit deadline")
	}
	// recordRateLimitHit adds jitter on top of Retry-After, so the window is
	// the header plus up to rateLimitBackoffJitter.
	wait := time.Until(kc.rateLimitUntil)
	if wait < 2500*time.Millisecond || wait > rateLimitBackoffJitter+4*time.Second {
		t.Errorf("deadline is %v away, want the 3s Retry-After plus up to %v of jitter", wait, rateLimitBackoffJitter)
	}
}

// TestUpdateRealmRawRespectsBackoff checks the recorded deadline actually
// short-circuits the next attempt rather than being advisory only.
func TestUpdateRealmRawRespectsBackoff(t *testing.T) {
	calls := 0
	kc, _ := realmRawServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
	})

	_ = kc.UpdateRealmRaw(context.Background(), "master", []byte(`{}`))
	first := calls

	err := kc.UpdateRealmRaw(context.Background(), "master", []byte(`{}`))
	if err == nil {
		t.Fatal("a call during the backoff window must be refused")
	}
	if calls != first {
		t.Errorf("server was called %d times during backoff, want no further calls", calls-first)
	}
	if !strings.Contains(err.Error(), "rate limit") && !errors.As(err, new(*RateLimitError)) {
		t.Logf("backoff error: %v", err)
	}
}

// TestUpdateRealmRawClearsBackoffOnSuccess is the other half: a 2xx must reset
// the window, or a provider that hit one 429 would refuse requests forever.
func TestUpdateRealmRawClearsBackoffOnSuccess(t *testing.T) {
	kc, _ := realmRawServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Start inside an active window so the success path is what clears it.
	kc.rateLimitUntil = time.Now().Add(30 * time.Second)
	kc.rateLimitConsecutive = 3

	if err := kc.UpdateRealmRaw(context.Background(), "master", []byte(`{}`)); err == nil {
		t.Fatal("precondition: a call inside the backoff window should be refused")
	}

	// With the window expired the request goes out and succeeds.
	kc.rateLimitUntil = time.Now().Add(-time.Second)
	if err := kc.UpdateRealmRaw(context.Background(), "master", []byte(`{}`)); err != nil {
		t.Fatalf("UpdateRealmRaw failed: %v", err)
	}
	if !kc.rateLimitUntil.IsZero() {
		t.Errorf("rateLimitUntil = %v, want cleared after a 2xx", kc.rateLimitUntil)
	}
	if kc.rateLimitConsecutive != 0 {
		t.Errorf("rateLimitConsecutive = %d, want reset after a 2xx", kc.rateLimitConsecutive)
	}
}

func TestUpdateRealmRawSurfacesHTTPErrors(t *testing.T) {
	cases := map[string]struct {
		status int
		body   string
	}{
		"conflict":     {http.StatusConflict, "realm exists"},
		"not found":    {http.StatusNotFound, "no such realm"},
		"server error": {http.StatusInternalServerError, "boom"},
		"unauthorized": {http.StatusUnauthorized, "bad token"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			kc, _ := realmRawServer(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			})

			err := kc.UpdateRealmRaw(context.Background(), "master", []byte(`{}`))
			if err == nil {
				t.Fatalf("HTTP %d must be surfaced as an error", tc.status)
			}
			if !strings.Contains(err.Error(), tc.body) {
				t.Errorf("error %q does not include the response body %q", err, tc.body)
			}
			if !strings.Contains(err.Error(), http.StatusText(tc.status)) &&
				!strings.Contains(err.Error(), itoa(tc.status)) {
				t.Errorf("error %q does not mention the status %d", err, tc.status)
			}
		})
	}
}

// TestUpdateRealmRawReportsTransportFailure keeps an unreachable Keycloak
// distinct from a rejected request.
func TestUpdateRealmRawReportsTransportFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close() // nothing is listening now

	kc := newTestKeycloakClient(http.DefaultClient, url, "tok")
	if err := kc.UpdateRealmRaw(context.Background(), "master", []byte(`{}`)); err == nil {
		t.Fatal("an unreachable Keycloak must be surfaced as an error")
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
