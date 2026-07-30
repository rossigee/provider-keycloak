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
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/rossigee/provider-keycloak/apis/v1beta1"
)

func TestRefreshTokenBackoff(t *testing.T) {
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(tokenResponse{AccessToken: "tok", ExpiresIn: 60})
	}))
	defer srv2.Close()

	cfg := &Config{BaseURL: srv2.URL, Realm: testRealm, ClientID: "x", ClientSecret: "y"}
	kc, err := NewClientFromConfig(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Force the token to be expired and inject a failure so the next refresh
	// sits inside the backoff window.
	kc.mu.Lock()
	kc.tokenExp = time.Now().Add(-1 * time.Second)
	kc.recordFailureLocked(errors.New("simulated"), time.Now())
	kc.mu.Unlock()

	if err := kc.refreshToken(context.Background()); !errors.Is(err, ErrAuthUnavailable) {
		t.Fatalf("expected ErrAuthUnavailable inside backoff, got %v", err)
	}

	// Clear backoff and confirm refresh succeeds.
	kc.mu.Lock()
	kc.backoffUntil = time.Now().Add(-1 * time.Second)
	kc.mu.Unlock()

	if err := kc.refreshToken(context.Background()); err != nil {
		t.Fatalf("expected refresh to succeed after backoff cleared, got %v", err)
	}
	if kc.token != "tok" {
		t.Fatalf("token not updated: %q", kc.token)
	}
}

func TestConnectorBackoff(t *testing.T) {
	ResetConnector()
	defer ResetConnector()

	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("oops"))
	}))
	defer srv.Close()

	pc, kube := newFailingProviderConfig(t, srv.URL)

	c := NewConnector(kube)
	c.connectBackoffInitial = 100 * time.Millisecond
	c.connectBackoffMax = 200 * time.Millisecond

	if _, err := c.Connect(context.Background(), pc.Name); !errors.Is(err, ErrAuthUnavailable) {
		t.Fatalf("expected ErrAuthUnavailable, got %v", err)
	}
	firstHits := atomic.LoadInt32(&hits)
	if firstHits == 0 {
		t.Fatalf("expected upstream to be hit at least once")
	}

	// Repeated calls within the backoff window must NOT hit upstream again.
	for i := 0; i < 5; i++ {
		if _, err := c.Connect(context.Background(), pc.Name); !errors.Is(err, ErrAuthUnavailable) {
			t.Fatalf("call %d: expected ErrAuthUnavailable, got %v", i, err)
		}
	}
	if got := atomic.LoadInt32(&hits); got != firstHits {
		t.Fatalf("expected upstream hit count to stay at %d, got %d", firstHits, got)
	}

	// After backoff elapses, next Connect should retry upstream.
	time.Sleep(250 * time.Millisecond)
	_, _ = c.Connect(context.Background(), pc.Name)
	if got := atomic.LoadInt32(&hits); got <= firstHits {
		t.Fatalf("expected upstream hit count to grow past %d after backoff, got %d", firstHits, got)
	}
}

func newFailingProviderConfig(t *testing.T, baseURL string) (*v1beta1.ProviderConfig, client.Client) {
	t.Helper()
	s := runtime.NewScheme()
	if err := corev1.AddToScheme(s); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}
	if err := v1beta1.SchemeBuilder.AddToScheme(s); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}

	creds, err := json.Marshal(ProviderCredentials{
		URL:         baseURL,
		BasePath:    "/auth",
		Realm:       testRealm,
		ClientID:    "x",
		ClientSecret: "y",
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	pc := &v1beta1.ProviderConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "default"},
		Spec: v1beta1.ProviderConfigSpec{
			Credentials: v1beta1.ProviderCredentials{
				CommonCredentialSelectors: xpv1.CommonCredentialSelectors{
					SecretRef: &xpv1.SecretKeySelector{
						SecretReference: xpv1.SecretReference{Name: "kc-creds", Namespace: "default"},
						Key:             "credentials",
					},
				},
			},
		},
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "kc-creds", Namespace: "default"},
		Data:       map[string][]byte{"credentials": creds},
	}
	kube := fake.NewClientBuilder().WithScheme(s).WithObjects(pc, secret).Build()
	return pc, kube
}

