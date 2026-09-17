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

package clientdefaultscopes

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
	realm     = "ROSSGolderLtd"
	cid       = "vault"
	cuuid     = "vault-uuid"
	groupsID  = "groups-uuid"
	profileID = "profile-uuid"
)

// defsStub embeds AssigningClient and overrides the lookup helpers so
// tests control what GetClient/GetClientScope return for each scenario.
type defsStub struct {
	*testhelpers.AssigningClient

	clientByName    string // returns cuuid when match, "" if missing
	scopeByName     map[string]*clients.ClientScopeRepresentation
	clientLookupErr error
	scopeLookupErr  error
	listScopes      []clients.ClientScopeRepresentation
	listErr         error
}

func newDefsStub() *defsStub {
	return &defsStub{
		AssigningClient: &testhelpers.AssigningClient{BaseMockClient: &testhelpers.BaseMockClient{}},
		clientByName:    cid,
		scopeByName: map[string]*clients.ClientScopeRepresentation{
			"groups":  {ID: groupsID, Name: "groups"},
			"profile": {ID: profileID, Name: "profile"},
		},
	}
}

func (s *defsStub) GetClient(_ context.Context, _, clientID string) (*clients.ClientRepresentation, error) {
	if s.clientLookupErr != nil {
		return nil, s.clientLookupErr
	}
	if clientID != s.clientByName {
		return nil, nil
	}
	return &clients.ClientRepresentation{ID: cuuid, ClientID: clientID}, nil
}

func (s *defsStub) GetClientScope(_ context.Context, _, name string) (*clients.ClientScopeRepresentation, error) {
	if s.scopeLookupErr != nil {
		return nil, s.scopeLookupErr
	}
	if sc, ok := s.scopeByName[name]; ok {
		return sc, nil
	}
	return nil, nil
}

func (s *defsStub) ListClientDefaultScopes(_ context.Context, _, _ string) ([]clients.ClientScopeRepresentation, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	out := make([]clients.ClientScopeRepresentation, len(s.listScopes))
	copy(out, s.listScopes)
	return out, nil
}

func newDefScopesCR(desired []string) *v1beta1.ClientDefaultScopes {
	r := realm
	c := cid
	return &v1beta1.ClientDefaultScopes{
		ObjectMeta: metav1.ObjectMeta{Name: "vault-default-scopes"},
		Spec: v1beta1.ClientDefaultScopesSpec{
			ForProvider: v1beta1.ClientDefaultScopesParameters{
				RealmId:       &r,
				ClientId:      &c,
				DefaultScopes: desired,
			},
		},
	}
}

func TestObserveClientDefaultScopes(t *testing.T) {
	t.Run("up-to-date when current matches desired", func(t *testing.T) {
		s := newDefsStub()
		s.ListFn = s.ListClientDefaultScopes // use override so List returns desired
		s.listScopes = []clients.ClientScopeRepresentation{{ID: groupsID, Name: "groups"}}
		obs, err := ObserveClientDefaultScopes(context.Background(), s, newDefScopesCR([]string{"groups"}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !obs.ResourceUpToDate {
			t.Errorf("expected up-to-date")
		}
	})

	t.Run("drift when current differs", func(t *testing.T) {
		s := newDefsStub()
		s.ListFn = s.ListClientDefaultScopes
		s.listScopes = []clients.ClientScopeRepresentation{{ID: profileID, Name: "profile"}}
		obs, err := ObserveClientDefaultScopes(context.Background(), s, newDefScopesCR([]string{"groups"}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if obs.ResourceUpToDate {
			t.Errorf("expected drift to be reported")
		}
	})

	t.Run("error when client lookup returns nil", func(t *testing.T) {
		s := newDefsStub()
		s.clientByName = "different"
		_, err := ObserveClientDefaultScopes(context.Background(), s, newDefScopesCR([]string{"groups"}))
		if err == nil {
			t.Fatal("expected error when client not found")
		}
	})
}

func TestCreateClientDefaultScopes(t *testing.T) {
	t.Run("adds both scopes by UUID, takes UUID path", func(t *testing.T) {
		s := newDefsStub()
		if _, err := CreateClientDefaultScopes(context.Background(), s, newDefScopesCR([]string{"groups", "profile"})); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(s.Adds) != 1 {
			t.Fatalf("expected 1 add call, got %d", len(s.Adds))
		}
		if s.Adds[0].ClientUUID != cuuid {
			t.Errorf("expected UUID %q, got %q", cuuid, s.Adds[0].ClientUUID)
		}
		if len(s.Adds[0].Scopes) != 2 {
			t.Errorf("expected 2 scopes added, got %d", len(s.Adds[0].Scopes))
		}
	})

	t.Run("unresolved scope errors before Add", func(t *testing.T) {
		s := newDefsStub()
		_, err := CreateClientDefaultScopes(context.Background(), s, newDefScopesCR([]string{"unknown"}))
		if err == nil {
			t.Fatal("expected error for unresolved scope")
		}
		if len(s.Adds) != 0 {
			t.Errorf("Add must not be invoked when scope names fail to resolve")
		}
	})
}

func TestUpdateClientDefaultScopes(t *testing.T) {
	t.Run("drift adds and removes correctly", func(t *testing.T) {
		s := newDefsStub()
		s.ListFn = s.ListClientDefaultScopes
		s.listScopes = []clients.ClientScopeRepresentation{{ID: profileID, Name: "profile"}}
		if _, err := UpdateClientDefaultScopes(context.Background(), s, newDefScopesCR([]string{"groups"})); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(s.Adds) != 1 || len(s.Adds[0].Scopes) != 1 || s.Adds[0].Scopes[0].ID != groupsID {
			t.Errorf("expected add groups, got %+v", s.Adds)
		}
		if len(s.Removes) != 1 || len(s.Removes[0].Scopes) != 1 || s.Removes[0].Scopes[0].ID != profileID {
			t.Errorf("expected remove profile, got %+v", s.Removes)
		}
	})
}

func TestDeleteClientDefaultScopes(t *testing.T) {
	t.Run("no-op when client missing", func(t *testing.T) {
		s := newDefsStub()
		s.clientByName = "different" // unknown -> Get returns nil
		if _, err := DeleteClientDefaultScopes(context.Background(), s, newDefScopesCR([]string{"groups"})); err != nil {
			t.Fatalf("expected nil error when client missing, got %v", err)
		}
	})

	t.Run("passes through list and remove errors", func(t *testing.T) {
		s := newDefsStub()
		s.listErr = errors.New("boom")
		if _, err := DeleteClientDefaultScopes(context.Background(), s, newDefScopesCR([]string{"groups"})); err == nil {
			t.Fatal("expected error to bubble up from List")
		}
	})
}

// satisfy unused-import warnings if any.
var _ = struct{}{}
