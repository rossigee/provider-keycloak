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

package protocolmapper

import (
	"context"
	"errors"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	clientv1beta1 "github.com/rossigee/provider-keycloak/apis/client/v1beta1"
	"github.com/rossigee/provider-keycloak/internal/clients"
)

func mapperCR(realm, clientID, name string) *clientv1beta1.ProtocolMapper {
	cr := &clientv1beta1.ProtocolMapper{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: clientv1beta1.ProtocolMapperSpec{
			ForProvider: clientv1beta1.ProtocolMapperParameters{
				Name:     name,
				Protocol: "openid-connect",
			},
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

// The spec names a mapper; Keycloak addresses it by UUID. Update looks the name
// up and copies the resolved ID onto the representation before writing, because
// a mapper representation without the right ID does not update the existing one.
func TestProtocolMapperUpdateSendsTheResolvedID(t *testing.T) {
	var got *clients.ProtocolMapperRepresentation
	mc := &mockMapperClient{
		getClientFn: func(_ context.Context, _, _ string) (*clients.ClientRepresentation, error) {
			return &clients.ClientRepresentation{ID: "client-uuid"}, nil
		},
		listMappersFn: func(_ context.Context, _, _ string) ([]clients.ProtocolMapperRepresentation, error) {
			return []clients.ProtocolMapperRepresentation{
				{ID: "other-id", Name: "unrelated"},
				{ID: "resolved-id", Name: "groups"},
			}, nil
		},
		updateMapperFn: func(_ context.Context, _, _ string, p *clients.ProtocolMapperRepresentation) error {
			got = p
			return nil
		},
	}

	e := &external{kc: mc}
	if _, err := e.Update(context.Background(), mapperCR("realm", "my-client", "groups")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("UpdateClientProtocolMapper was not called")
	}
	if got.ID != "resolved-id" {
		t.Errorf("sent mapper ID %q, want %q - the resolved UUID, not the name or a sibling's id", got.ID, "resolved-id")
	}
}

func TestProtocolMapperUpdateFailsWhenMapperAbsent(t *testing.T) {
	mc := &mockMapperClient{
		getClientFn: func(_ context.Context, _, _ string) (*clients.ClientRepresentation, error) {
			return &clients.ClientRepresentation{ID: "client-uuid"}, nil
		},
		listMappersFn: func(_ context.Context, _, _ string) ([]clients.ProtocolMapperRepresentation, error) {
			return nil, nil
		},
		updateMapperFn: func(_ context.Context, _, _ string, _ *clients.ProtocolMapperRepresentation) error {
			t.Error("UpdateClientProtocolMapper was called for a mapper that does not exist")
			return nil
		},
	}
	e := &external{kc: mc}
	if _, err := e.Update(context.Background(), mapperCR("realm", "my-client", "missing")); err == nil {
		t.Fatal("expected an error when the mapper is absent")
	}
}

func TestProtocolMapperUpdateSurfacesErrors(t *testing.T) {
	boom := errors.New("boom")

	base := func() *mockMapperClient {
		return &mockMapperClient{
			getClientFn: func(_ context.Context, _, _ string) (*clients.ClientRepresentation, error) {
				return &clients.ClientRepresentation{ID: "client-uuid"}, nil
			},
			listMappersFn: func(_ context.Context, _, _ string) ([]clients.ProtocolMapperRepresentation, error) {
				return []clients.ProtocolMapperRepresentation{{ID: "resolved-id", Name: "groups"}}, nil
			},
			updateMapperFn: func(_ context.Context, _, _ string, _ *clients.ProtocolMapperRepresentation) error { return nil },
		}
	}

	t.Run("client lookup fails", func(t *testing.T) {
		mc := base()
		mc.getClientFn = func(_ context.Context, _, _ string) (*clients.ClientRepresentation, error) { return nil, boom }
		e := &external{kc: mc}
		if _, err := e.Update(context.Background(), mapperCR("realm", "my-client", "groups")); err == nil {
			t.Fatal("expected the client lookup error")
		}
	})

	t.Run("client absent", func(t *testing.T) {
		mc := base()
		mc.getClientFn = func(_ context.Context, _, _ string) (*clients.ClientRepresentation, error) { return nil, nil }
		e := &external{kc: mc}
		if _, err := e.Update(context.Background(), mapperCR("realm", "my-client", "groups")); err == nil {
			t.Fatal("expected an error when the client is absent")
		}
	})

	t.Run("list fails", func(t *testing.T) {
		mc := base()
		mc.listMappersFn = func(_ context.Context, _, _ string) ([]clients.ProtocolMapperRepresentation, error) {
			return nil, boom
		}
		e := &external{kc: mc}
		if _, err := e.Update(context.Background(), mapperCR("realm", "my-client", "groups")); err == nil {
			t.Fatal("expected the list error")
		}
	})

	t.Run("update fails", func(t *testing.T) {
		mc := base()
		mc.updateMapperFn = func(_ context.Context, _, _ string, _ *clients.ProtocolMapperRepresentation) error { return boom }
		e := &external{kc: mc}
		if _, err := e.Update(context.Background(), mapperCR("realm", "my-client", "groups")); err == nil {
			t.Fatal("expected the update error")
		}
	})

	t.Run("realmId missing", func(t *testing.T) {
		e := &external{kc: base()}
		if _, err := e.Update(context.Background(), mapperCR("", "my-client", "groups")); err == nil {
			t.Fatal("expected an error when realmId is unset")
		}
	})

	t.Run("clientId missing", func(t *testing.T) {
		e := &external{kc: base()}
		if _, err := e.Update(context.Background(), mapperCR("realm", "", "groups")); err == nil {
			t.Fatal("expected an error when clientId is unset")
		}
	})
}
