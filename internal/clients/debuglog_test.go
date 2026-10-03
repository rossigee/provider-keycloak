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
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// captureStdout runs fn with os.Stdout redirected and returns what it wrote.
// doRequest logs through fmt.Printf, so this is the only way to assert on what
// the debug path actually emits rather than on the helpers in isolation.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("cannot create pipe: %v", err)
	}
	orig := os.Stdout
	os.Stdout = w

	done := make(chan string, 1)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()

	fn()

	os.Stdout = orig
	_ = w.Close()
	out := <-done
	_ = r.Close()
	return out
}

// withDebugHTTP turns the debug path on for the duration of a test.
func withDebugHTTP(t *testing.T) {
	t.Helper()
	orig := debugHTTP
	debugHTTP = true
	t.Cleanup(func() { debugHTTP = orig })
}

const secretToken = "super-secret-bearer-token"

// TestDoRequestDebugDoesNotLeakRequestCredentials covers the request half of
// code-scanning alerts 4 and 5.
//
// The redaction helpers existed but were only applied to one of the three
// logging sites. The wire dump re-marshalled the raw body and cloned the
// headers verbatim, so enabling KEYCLOAK_PROVIDER_DEBUG_HTTP wrote the bearer
// token and the payload's password to the pod log. Asserting on the helpers
// alone would not have caught that; this drives doRequest and reads its output.
// The wire dump is guarded by an exact `path == adminPath`, so only realm
// creation (POST /admin/realms) reaches it. The test has to post there
// exactly; posting to adminPath+"/users" silently skips the dump and the test
// would pass while proving nothing. That is the same trap the original bug sat
// in, which is why there is an explicit assertion below that a dump occurred.
func TestDoRequestDebugDoesNotLeakRequestCredentials(t *testing.T) {
	withDebugHTTP(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"r1","realm":"master"}`))
	}))
	defer srv.Close()

	kc := newTestKeycloakClient(srv.Client(), srv.URL, secretToken)

	body := map[string]string{"realm": "master", "password": "hunter2"}
	out := captureStdout(t, func() {
		if _, err := kc.doRequest(t.Context(), http.MethodPost, adminPath, body); err != nil {
			t.Errorf("doRequest failed: %v", err)
		}
	})

	if out == "" {
		t.Fatal("expected the debug path to produce output; nothing was captured, so this test proves nothing")
	}
	if !strings.Contains(out, "wire dump") {
		t.Fatalf("the wire dump did not run, so this test is not exercising the leak it claims to:\n%s", out)
	}
	if strings.Contains(out, secretToken) {
		t.Errorf("the bearer token reached the log:\n%s", out)
	}
	if strings.Contains(out, "hunter2") {
		t.Errorf("the payload password reached the log:\n%s", out)
	}
	if !strings.Contains(out, "master") {
		t.Errorf("redaction destroyed the diagnostic value of the dump:\n%s", out)
	}
}

// TestDoRequestDebugDoesNotLeakResponseCredentials covers the response half:
// the response dump printed the whole body and the whole header set.
func TestDoRequestDebugDoesNotLeakResponseCredentials(t *testing.T) {
	withDebugHTTP(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Set-Cookie", "KEYCLOAK_SESSION=session-secret-value")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"c1","clientId":"app","clientSecret":"s3cr3t-value"}`))
	}))
	defer srv.Close()

	kc := newTestKeycloakClient(srv.Client(), srv.URL, secretToken)

	out := captureStdout(t, func() {
		if _, err := kc.doRequest(t.Context(), http.MethodGet, adminPath+"/clients/c1", nil); err != nil {
			t.Errorf("doRequest failed: %v", err)
		}
	})

	if out == "" {
		t.Fatal("expected the debug path to produce output; nothing was captured, so this test proves nothing")
	}
	if strings.Contains(out, "s3cr3t-value") {
		t.Errorf("the client secret in the response body reached the log:\n%s", out)
	}
	if strings.Contains(out, "session-secret-value") {
		t.Errorf("the session cookie in the response headers reached the log:\n%s", out)
	}
	// The body is printed with %q, so its quotes arrive escaped. Assert on the
	// field name and value separately rather than on a literal JSON fragment.
	if !strings.Contains(out, `clientId`) || !strings.Contains(out, `app`) {
		t.Errorf("redaction destroyed the diagnostic value of the dump:\n%s", out)
	}
}

// TestDoRequestDebugDoesNotLeakOnOtherMethods guards the paths that do not take
// the wire-dump branch but still log a response.
func TestDoRequestDebugDoesNotLeakOnOtherMethods(t *testing.T) {
	withDebugHTTP(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"privateKey":"-----BEGIN PRIVATE KEY-----leaked"}`))
	}))
	defer srv.Close()

	kc := newTestKeycloakClient(srv.Client(), srv.URL, secretToken)

	out := captureStdout(t, func() {
		if _, err := kc.doRequest(t.Context(), http.MethodGet, adminPath+"/keys", nil); err != nil {
			t.Errorf("doRequest failed: %v", err)
		}
	})

	if strings.Contains(out, "BEGIN PRIVATE KEY") {
		t.Errorf("a private key reached the log:\n%s", out)
	}
	if strings.Contains(out, secretToken) {
		t.Errorf("the bearer token reached the log:\n%s", out)
	}
}

// TestDoRequestLeavesRealRequestIntact proves the redaction did not achieve
// safety by mutating the live request. redactHeaders must clone, because the
// wire dump is built from req.Header - if redaction mutated it in place the
// real request would be sent with Authorization: REDACTED and every
// authenticated call would fail.
func TestDoRequestLeavesRealRequestIntact(t *testing.T) {
	withDebugHTTP(t)

	var gotAuth string
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	kc := newTestKeycloakClient(srv.Client(), srv.URL, secretToken)

	// A POST to adminPath is the branch that performs the wire dump, so this
	// exercises the redaction that could have corrupted the real request.
	_ = captureStdout(t, func() {
		if _, err := kc.doRequest(t.Context(), http.MethodPost, adminPath, map[string]string{"realm": "master", "password": "hunter2"}); err != nil {
			t.Errorf("doRequest failed: %v", err)
		}
	})

	if gotAuth != "Bearer "+secretToken {
		t.Errorf("server received Authorization %q, want the real token", gotAuth)
	}
	if !strings.Contains(gotBody, "hunter2") {
		t.Errorf("server received body %q, want the real password on the wire", gotBody)
	}
}
