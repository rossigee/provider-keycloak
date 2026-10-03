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

package testhelpers

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"

	"github.com/rossigee/provider-keycloak/apis/v1beta1"
	"github.com/rossigee/provider-keycloak/internal/clients"
)

// ConnectFixture makes clients.GetConnector(...).Connect(...) succeed against a
// fake Keycloak, so a controller's Connect can be driven all the way through to
// a live external client rather than stopping at the readiness check.
//
// Every controller's Connect ends by calling the process-wide Connector, which
// reads its ProviderConfig from the API server and performs a real OAuth token
// exchange. Serving the token endpoint is what makes that reachable from a unit
// test; without it the success path cannot be covered at all.
type ConnectFixture struct {
	// Client is a kube client holding the ProviderConfig and credentials secret.
	Client client.Client

	// ProviderConfig is the Ready ProviderConfig the fixture created.
	ProviderConfig *v1beta1.ProviderConfig

	// TokenRequests counts calls made to the fake token endpoint.
	TokenRequests int
}

// ProviderConfigName is the name of the ProviderConfig NewConnectFixture creates.
const ProviderConfigName = "test-provider"

// NewConnectFixture starts a fake Keycloak token endpoint and returns a kube
// client holding a Ready ProviderConfig pointing at it.
//
// It resets the process-wide Connector before and after the test, because that
// connector is a singleton and records a backoff after any failure - without a
// reset, one test's failure makes the next test's Connect fail for the wrong
// reason.
func NewConnectFixture(t *testing.T) *ConnectFixture {
	t.Helper()

	f := &ConnectFixture{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.TokenRequests++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"fake-token","expires_in":3600}`))
	}))
	t.Cleanup(srv.Close)

	creds := fmt.Sprintf(`{"url":%q,"realm":"master","client_id":"provider","client_secret":"secret"}`, srv.URL)

	pc := &v1beta1.ProviderConfig{
		ObjectMeta: metav1.ObjectMeta{Name: ProviderConfigName},
		Spec: v1beta1.ProviderConfigSpec{
			Credentials: v1beta1.ProviderCredentials{
				SecretRef: &xpv1.SecretKeySelector{
					SecretReference: xpv1.SecretReference{
						Name:      "keycloak-creds",
						Namespace: "crossplane-system",
					},
					Key: "credentials",
				},
			},
		},
		Status: v1beta1.ProviderConfigStatus{
			ProviderConfigStatus: xpv1.ProviderConfigStatus{
				Conditions: []xpv1.Condition{{
					Type:   xpv1.TypeReady,
					Status: "True",
					Reason: "Available",
				}},
			},
		},
	}

	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "keycloak-creds", Namespace: "crossplane-system"},
		Data:       map[string][]byte{"credentials": []byte(creds)},
	}

	sch := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{corev1.AddToScheme, v1beta1.AddToScheme} {
		if err := add(sch); err != nil {
			t.Fatalf("building scheme: %v", err)
		}
	}

	f.Client = fake.NewClientBuilder().
		WithScheme(sch).
		WithObjects(pc, sec).
		WithStatusSubresource(pc).
		Build()
	f.ProviderConfig = pc

	clients.ResetConnector()
	t.Cleanup(clients.ResetConnector)

	return f
}

// NotReadyProviderConfig returns the ProviderConfig name with its Ready condition
// cleared, for the branch where the provider config is not usable yet.
func NotReadyProviderConfig(ctx context.Context, t *testing.T, c client.Client, name string) {
	t.Helper()

	pc := &v1beta1.ProviderConfig{}
	if err := c.Get(ctx, client.ObjectKey{Name: name}, pc); err != nil {
		t.Fatalf("getting ProviderConfig: %v", err)
	}
	pc.Status.SetConditions(xpv1.Unavailable())
	if err := c.Status().Update(ctx, pc); err != nil {
		t.Fatalf("clearing Ready condition: %v", err)
	}
}

// MarkProviderConfigReady restores the Ready condition after a test cleared it.
func MarkProviderConfigReady(ctx context.Context, t *testing.T, c client.Client, name string) {
	t.Helper()

	pc := &v1beta1.ProviderConfig{}
	if err := c.Get(ctx, client.ObjectKey{Name: name}, pc); err != nil {
		t.Fatalf("getting ProviderConfig: %v", err)
	}
	pc.Status.SetConditions(xpv1.Available())
	if err := c.Status().Update(ctx, pc); err != nil {
		t.Fatalf("restoring Ready condition: %v", err)
	}
}
