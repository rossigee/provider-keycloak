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

package role

import (
	"context"
	"errors"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	realmv1beta1 "github.com/rossigee/provider-keycloak/apis/realm/v1beta1"
	rolev1beta1 "github.com/rossigee/provider-keycloak/apis/role/v1beta1"
	"github.com/rossigee/provider-keycloak/internal/clients"
	"github.com/rossigee/provider-keycloak/internal/controller/testhelpers"
)

// clientRoleSpy adds the client-role hooks that mockRoleClient lacks, so the
// test can tell which endpoint Update chose.
type clientRoleSpy struct {
	*mockRoleClient
	updateClientRole func(realm, clientUUID, name string) error
}

func (s *clientRoleSpy) UpdateClientRole(_ context.Context, realm, clientUUID, name string, _ *clients.RoleRepresentation) error {
	return s.updateClientRole(realm, clientUUID, name)
}

func roleCR(realm, clientID, name string) *rolev1beta1.Role {
	cr := &rolev1beta1.Role{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: rolev1beta1.RoleSpec{
			ForProvider: rolev1beta1.RoleParameters{Name: name},
		},
	}
	if realm != "" {
		cr.Spec.ForProvider.RealmId = &realm
	}
	if clientID != "" {
		cr.Spec.ForProvider.ClientId = &clientID
	}
	return cr
}

// A Role with no clientId is a realm role; one with a clientId is a client role.
// They live at different Keycloak endpoints, so picking the wrong one is a 404
// that surfaces as a role that never updates.
func TestRoleUpdatePicksTheRightEndpoint(t *testing.T) {
	t.Run("realm role when clientId is unset", func(t *testing.T) {
		var realmHit bool
		m := &mockRoleClient{
			BaseMockClient: &testhelpers.BaseMockClient{},
			updateRealmRoleFn: func(_ context.Context, _, _ string, _ *clients.RoleRepresentation) error {
				realmHit = true
				return nil
			},
		}
		e := &external{client: m}
		if _, err := e.Update(context.Background(), roleCR("realm", "", "admin")); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !realmHit {
			t.Error("UpdateRealmRole was not called for a role with no clientId")
		}
	})

	t.Run("client role when clientId is set", func(t *testing.T) {
		var gotRealm, gotClient, gotName string
		m := &clientRoleSpy{
			mockRoleClient: &mockRoleClient{BaseMockClient: &testhelpers.BaseMockClient{}},
			updateClientRole: func(realm, clientUUID, name string) error {
				gotRealm, gotClient, gotName = realm, clientUUID, name
				return nil
			},
		}
		e := &external{client: m}
		if _, err := e.Update(context.Background(), roleCR("realm", "my-client", "viewer")); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotRealm != "realm" || gotClient != "my-client" || gotName != "viewer" {
			t.Errorf("UpdateClientRole(%q, %q, %q), want (realm, my-client, viewer)", gotRealm, gotClient, gotName)
		}
	})

	t.Run("surfaces the update error", func(t *testing.T) {
		boom := errors.New("boom")
		m := &mockRoleClient{
			BaseMockClient:    &testhelpers.BaseMockClient{},
			updateRealmRoleFn: func(_ context.Context, _, _ string, _ *clients.RoleRepresentation) error { return boom },
		}
		e := &external{client: m}
		if _, err := e.Update(context.Background(), roleCR("realm", "", "admin")); err == nil {
			t.Fatal("expected the update error")
		}
	})

	t.Run("requires a realmId", func(t *testing.T) {
		m := &mockRoleClient{BaseMockClient: &testhelpers.BaseMockClient{}}
		e := &external{client: m}
		if _, err := e.Update(context.Background(), roleCR("", "", "admin")); err == nil {
			t.Fatal("expected an error when realmId is unset")
		}
	})
}

func TestRoleUpdateRejectsWrongManagedType(t *testing.T) {
	e := &external{client: &mockRoleClient{BaseMockClient: &testhelpers.BaseMockClient{}}}
	if _, err := e.Update(context.Background(), &realmv1beta1.Realm{}); err == nil {
		t.Error("Update accepted an unrelated managed type")
	}
	if _, err := e.Delete(context.Background(), &realmv1beta1.Realm{}); err == nil {
		t.Error("Delete accepted an unrelated managed type")
	}
}
