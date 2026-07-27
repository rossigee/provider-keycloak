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

package providerconfig

import (
	"context"
	"testing"

	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/rossigee/provider-keycloak/apis/v1beta1"
)

// TestReconcileStatusPersists guards against a regression where the
// ProviderConfig's Ready/Available condition was written with a plain
// Update() even though the CRD has the status subresource enabled - the API
// server silently discards .status changes made that way, so the condition
// never actually persisted. Every managed-resource controller in this
// provider (Realm, Group, User, ...) refuses to connect unless this
// condition is "True", so that bug made every one of them permanently fail
// with "provider is not ready".
func TestReconcileStatusPersists(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := v1beta1.SchemeBuilder.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to build scheme: %v", err)
	}

	pc := &v1beta1.ProviderConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "default"},
		Spec: v1beta1.ProviderConfigSpec{
			Credentials: v1beta1.ProviderCredentials{
				Source: xpv1.CredentialsSourceSecret,
				CommonCredentialSelectors: xpv1.CommonCredentialSelectors{
					SecretRef: &xpv1.SecretKeySelector{
						Key: "credentials",
						SecretReference: xpv1.SecretReference{
							Name:      "does-not-exist",
							Namespace: "crossplane-system",
						},
					},
				},
			},
		},
	}

	kube := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&v1beta1.ProviderConfig{}).
		WithObjects(pc).
		Build()

	r := &reconciler{kube: kube, logger: logr.Discard()}

	if _, err := r.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: client.ObjectKey{Name: "default"},
	}); err != nil {
		t.Fatalf("Reconcile returned unexpected error: %v", err)
	}

	got := &v1beta1.ProviderConfig{}
	if err := kube.Get(context.Background(), client.ObjectKey{Name: "default"}, got); err != nil {
		t.Fatalf("failed to fetch ProviderConfig after reconcile: %v", err)
	}

	if len(got.Status.Conditions) == 0 {
		t.Fatal("expected a condition to be persisted on ProviderConfig.status, found none - " +
			"status update is being silently discarded (likely a plain Update() against a " +
			"status-subresource-enabled CRD instead of Status().Update())")
	}
}
