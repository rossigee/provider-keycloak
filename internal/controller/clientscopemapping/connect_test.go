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

package clientscopemapping

import (
	"context"
	"testing"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"

	realmv1beta1 "github.com/rossigee/provider-keycloak/apis/realm/v1beta1"
	csv1beta1 "github.com/rossigee/provider-keycloak/apis/scopes/v1beta1"
	"github.com/rossigee/provider-keycloak/internal/controller/testhelpers"
)

// Connect is the gate every reconcile passes through: it resolves the
// ProviderConfig, checks readiness, and only then reaches Keycloak. Each early
// return is a distinct operator-visible failure - a mistyped reference, a
// ProviderConfig that was never applied, one that is not Ready yet - and none of
// them were covered.
//
// The success path runs as far as the Keycloak token endpoint, which
// ConnectFixture serves, so it proves a usable external client was produced
// rather than merely that the readiness check was passed.
func TestConnectRejectsUnusableInput(t *testing.T) {
	ctx := context.Background()
	f := testhelpers.NewConnectFixture(t)
	c := &connector{kube: f.Client}

	t.Run("wrong managed type", func(t *testing.T) {
		if _, err := c.Connect(ctx, &realmv1beta1.Realm{}); err == nil {
			t.Error("Connect accepted an unrelated managed type")
		}
	})

	t.Run("no providerConfigRef", func(t *testing.T) {
		if _, err := c.Connect(ctx, &csv1beta1.ClientScopeMapping{}); err == nil {
			t.Error("Connect accepted a resource with no providerConfigRef")
		}
	})

	t.Run("providerConfig does not exist", func(t *testing.T) {
		_, err := c.Connect(ctx, connectorCR("nonexistent"))
		if err == nil {
			t.Fatal("Connect accepted a reference to a ProviderConfig that does not exist")
		}
		// Assert the failure is specifically the lookup failing. Two different
		// mistakes both return *an* error here - ignoring the lookup error
		// entirely, so the readiness check trips on an empty ProviderConfig -
		// so the error has to be inspected to tell a correct rejection from an
		// incidental one.
		if !kerrors.IsNotFound(err) {
			t.Errorf("error = %v, want a NotFound for the missing ProviderConfig", err)
		}
	})

	t.Run("providerConfig not ready", func(t *testing.T) {
		testhelpers.NotReadyProviderConfig(ctx, t, f.Client, testhelpers.ProviderConfigName)
		t.Cleanup(func() {
			testhelpers.MarkProviderConfigReady(ctx, t, f.Client, testhelpers.ProviderConfigName)
		})

		if _, err := c.Connect(ctx, connectorCR(testhelpers.ProviderConfigName)); err == nil {
			t.Error("Connect accepted a ProviderConfig that is not Ready")
		}
	})
}

func TestConnectSucceedsWhenProviderConfigIsReady(t *testing.T) {
	ctx := context.Background()
	f := testhelpers.NewConnectFixture(t)
	c := &connector{kube: f.Client}

	got, err := c.Connect(ctx, connectorCR(testhelpers.ProviderConfigName))
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	if got == nil {
		t.Fatal("Connect returned a nil external client")
	}
	if f.TokenRequests == 0 {
		t.Error("Connect never called the Keycloak token endpoint, so it did not " +
			"actually reach Keycloak")
	}
}

// connectorCR builds a ClientScopeMapping referencing the named ProviderConfig.
func connectorCR(pcName string) *csv1beta1.ClientScopeMapping {
	return &csv1beta1.ClientScopeMapping{
		ObjectMeta: metav1.ObjectMeta{Name: "clientscopemapping"},
		Spec:       csv1beta1.ClientScopeMappingSpec{ProviderConfigReference: &xpv1.ProviderConfigReference{Name: pcName}},
	}
}

func TestDisconnectSucceeds(t *testing.T) {
	e := &external{}
	if err := e.Disconnect(context.Background()); err != nil {
		t.Errorf("Disconnect returned %v, want nil", err)
	}
}
