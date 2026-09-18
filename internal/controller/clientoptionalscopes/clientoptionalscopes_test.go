/*
Copyright 2024 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
*/

package clientoptionalscopes

import (
	"context"
	"testing"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/rossigee/provider-keycloak/apis/openidclient/v1beta1"
	"github.com/rossigee/provider-keycloak/internal/clients"
	"github.com/rossigee/provider-keycloak/internal/controller/testhelpers"
)

const (
	optrealm     = "ROSSGolderLtd"
	optcid       = "vault"
	optcuuid     = "vault-uuid"
	optgroupsID  = "groups-uuid"
	optprofileID = "profile-uuid"
)

// optsStub embeds AssigningClient; only its lookup methods are overridden
// (everything else falls through to no-op stubs via the embedded BaseMockClient).
type optsStub struct {
	*testhelpers.AssigningClient

	clientByName    string
	scopeByName     map[string]*clients.ClientScopeRepresentation
	currentOptional []clients.ClientScopeRepresentation
	listErr         error
}

func newOptsStub() *optsStub {
	return &optsStub{
		AssigningClient: &testhelpers.AssigningClient{BaseMockClient: &testhelpers.BaseMockClient{}},
		clientByName:    optcid,
		scopeByName: map[string]*clients.ClientScopeRepresentation{
			"groups":  {ID: optgroupsID, Name: "groups"},
			"profile": {ID: optprofileID, Name: "profile"},
		},
	}
}

func (s *optsStub) GetClient(_ context.Context, _, clientID string) (*clients.ClientRepresentation, error) {
	if clientID != s.clientByName {
		return nil, nil
	}
	return &clients.ClientRepresentation{ID: optcuuid, ClientID: clientID}, nil
}

func (s *optsStub) GetClientScope(_ context.Context, _, name string) (*clients.ClientScopeRepresentation, error) {
	if sc, ok := s.scopeByName[name]; ok {
		return sc, nil
	}
	return nil, nil
}

func (s *optsStub) ListClientOptionalScopes(_ context.Context, _, _ string) ([]clients.ClientScopeRepresentation, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	out := make([]clients.ClientScopeRepresentation, len(s.currentOptional))
	copy(out, s.currentOptional)
	return out, nil
}

func newOptCR(desired []string) *v1beta1.ClientOptionalScopes {
	r := optrealm
	c := optcid
	return &v1beta1.ClientOptionalScopes{
		ObjectMeta: metav1.ObjectMeta{Name: "vault-optional-scopes"},
		Spec: v1beta1.ClientOptionalScopesSpec{
			ForProvider: v1beta1.ClientOptionalScopesParameters{
				RealmId:        &r,
				ClientId:       &c,
				OptionalScopes: desired,
			},
		},
	}
}

func TestObserveClientOptionalScopes(t *testing.T) {
	t.Run("up-to-date when current matches desired", func(t *testing.T) {
		s := newOptsStub()
		s.currentOptional = []clients.ClientScopeRepresentation{{ID: optgroupsID, Name: "groups"}}
		obs, err := ObserveClientOptionalScopes(context.Background(), s, newOptCR([]string{"groups"}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !obs.ResourceUpToDate {
			t.Errorf("expected up-to-date")
		}
		if !obs.ResourceExists {
			t.Errorf("expected resource to exist while scopes are attached")
		}
	})

	t.Run("drift when scopes differ", func(t *testing.T) {
		s := newOptsStub()
		s.currentOptional = []clients.ClientScopeRepresentation{{ID: optprofileID, Name: "profile"}}
		obs, err := ObserveClientOptionalScopes(context.Background(), s, newOptCR([]string{"groups"}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if obs.ResourceUpToDate {
			t.Errorf("expected drift to be reported")
		}
		if !obs.ResourceExists {
			t.Errorf("expected resource to exist while scopes are attached")
		}
	})

	t.Run("does not exist during deletion once scopes are removed", func(t *testing.T) {
		s := newOptsStub()
		cr := newOptCR([]string{"groups"})
		now := metav1.Now()
		cr.SetDeletionTimestamp(&now)
		obs, err := ObserveClientOptionalScopes(context.Background(), s, cr)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if obs.ResourceExists {
			t.Errorf("expected resource NOT to exist during deletion once scopes are detached")
		}
	})
}

func TestCreateClientOptionalScopes(t *testing.T) {
	t.Run("fails fast on unresolved scope", func(t *testing.T) {
		if _, err := CreateClientOptionalScopes(context.Background(), newOptsStub(), newOptCR([]string{"missing"})); err == nil {
			t.Fatal("expected error for unresolved scope")
		}
	})

	t.Run("happy path completes", func(t *testing.T) {
		if _, err := CreateClientOptionalScopes(context.Background(), newOptsStub(), newOptCR([]string{"groups"})); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestDeleteClientOptionalScopes(t *testing.T) {
	t.Run("no-op when client unknown", func(t *testing.T) {
		s := newOptsStub()
		s.clientByName = "different"
		if _, err := DeleteClientOptionalScopes(context.Background(), s, newOptCR([]string{"groups"})); err != nil {
			t.Fatalf("expected nil error when client unknown, got %v", err)
		}
	})

	t.Run("passes through list errors", func(t *testing.T) {
		s := newOptsStub()
		s.listErr = errors.New("boom")
		if _, err := DeleteClientOptionalScopes(context.Background(), s, newOptCR([]string{"groups"})); err == nil {
			t.Fatal("expected error")
		}
	})
}
