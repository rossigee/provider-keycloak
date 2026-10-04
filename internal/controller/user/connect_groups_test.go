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

package user

import (
	"context"
	"testing"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"

	realmv1beta1 "github.com/rossigee/provider-keycloak/apis/realm/v1beta1"
	userv1beta1 "github.com/rossigee/provider-keycloak/apis/user/v1beta1"
	"github.com/rossigee/provider-keycloak/internal/controller/testhelpers"
)

// The user package runs two connectors: one for User and one for Groups. A
// generator that writes one file per package silently overwrites the first with
// the second, which is how this one was missed - the Groups connector sat at
// 0% while the PR claimed every connector was covered.
func TestGroupsConnectRejectsUnusableInput(t *testing.T) {
	ctx := context.Background()
	f := testhelpers.NewConnectFixture(t)
	c := &groupsConnector{kube: f.Client, recorder: event.NewNopRecorder()}

	t.Run("wrong managed type", func(t *testing.T) {
		if _, err := c.Connect(ctx, &realmv1beta1.Realm{}); err == nil {
			t.Error("Connect accepted an unrelated managed type")
		}
	})

	t.Run("no providerConfigRef", func(t *testing.T) {
		if _, err := c.Connect(ctx, &userv1beta1.Groups{}); err == nil {
			t.Error("Connect accepted a resource with no providerConfigRef")
		}
	})

	t.Run("providerConfig does not exist", func(t *testing.T) {
		_, err := c.Connect(ctx, groupsConnectorCR("nonexistent"))
		if err == nil {
			t.Fatal("Connect accepted a reference to a ProviderConfig that does not exist")
		}
		if !kerrors.IsNotFound(err) {
			t.Errorf("error = %v, want a NotFound for the missing ProviderConfig", err)
		}
	})

	t.Run("providerConfig not ready", func(t *testing.T) {
		testhelpers.NotReadyProviderConfig(ctx, t, f.Client, testhelpers.ProviderConfigName)
		t.Cleanup(func() {
			testhelpers.MarkProviderConfigReady(ctx, t, f.Client, testhelpers.ProviderConfigName)
		})

		if _, err := c.Connect(ctx, groupsConnectorCR(testhelpers.ProviderConfigName)); err == nil {
			t.Error("Connect accepted a ProviderConfig that is not Ready")
		}
	})
}

func TestGroupsConnectSucceedsWhenProviderConfigIsReady(t *testing.T) {
	ctx := context.Background()
	f := testhelpers.NewConnectFixture(t)
	c := &groupsConnector{kube: f.Client, recorder: event.NewNopRecorder()}

	got, err := c.Connect(ctx, groupsConnectorCR(testhelpers.ProviderConfigName))
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

func TestGroupsDisconnectSucceeds(t *testing.T) {
	e := &groupsExternal{}
	if err := e.Disconnect(context.Background()); err != nil {
		t.Errorf("Disconnect returned %v, want nil", err)
	}
}

func groupsConnectorCR(pcName string) *userv1beta1.Groups {
	return &userv1beta1.Groups{
		ObjectMeta: metav1.ObjectMeta{Name: "groups"},
		Spec:       userv1beta1.GroupsSpec{ProviderConfigReference: &xpv1.ProviderConfigReference{Name: pcName}},
	}
}
