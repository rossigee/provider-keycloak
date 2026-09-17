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

package clientscope

import (
	"context"
	"testing"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/rossigee/provider-keycloak/apis/scopes/v1beta1"
	"github.com/rossigee/provider-keycloak/internal/clients"
	"github.com/rossigee/provider-keycloak/internal/controller/testhelpers"
)

// scopeStub composes pointer-embedded BaseMockClient with a few
// pointer-receiver overrides. Every method on clients.Client that
// we don't override resolves to the no-op stub inside BaseMockClient.
type scopeStub struct {
	*testhelpers.BaseMockClient

	// control knobs
	getClientScope func(ctx context.Context, realm, name string) (*clients.ClientScopeRepresentation, error)
	updateHits     int
	updateErr      error
	createErr      error
	deleteErr      error
}

func (s *scopeStub) GetClientScope(ctx context.Context, realm, name string) (*clients.ClientScopeRepresentation, error) {
	if s.getClientScope != nil {
		return s.getClientScope(ctx, realm, name)
	}
	return s.BaseMockClient.GetClientScope(ctx, realm, name)
}

func (s *scopeStub) CreateClientScope(ctx context.Context, realm string, scope clients.ClientScopeRepresentation) error {
	if s.createErr != nil {
		return s.createErr
	}
	return s.BaseMockClient.CreateClientScope(ctx, realm, scope)
}

func (s *scopeStub) UpdateClientScope(ctx context.Context, realm string, scope clients.ClientScopeRepresentation) error {
	s.updateHits++
	if s.updateErr != nil {
		return s.updateErr
	}
	return s.BaseMockClient.UpdateClientScope(ctx, realm, scope)
}

func (s *scopeStub) DeleteClientScope(ctx context.Context, realm, name string) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	return s.BaseMockClient.DeleteClientScope(ctx, realm, name)
}

func newScopeCR(name string) *v1beta1.ClientScope {
	include := true
	proto := "openid-connect"
	return &v1beta1.ClientScope{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: v1beta1.ClientScopeSpec{
			ForProvider: v1beta1.ClientScopeParameters{
				RealmId:             "ROSSGolderLtd",
				Name:                name,
				Protocol:            &proto,
				IncludeInTokenScope: &include,
			},
		},
	}
}

// newScopeStub returns a stub with the BaseMockClient embedded so every
// non-overridden clients.Client method falls through to a no-op stub.
func newScopeStub() *scopeStub {
	return &scopeStub{BaseMockClient: &testhelpers.BaseMockClient{}}
}

func TestObserveClientScope(t *testing.T) {
	t.Run("present scope", func(t *testing.T) {
		s := newScopeStub()
		s.getClientScope = func(_ context.Context, _, name string) (*clients.ClientScopeRepresentation, error) {
			return &clients.ClientScopeRepresentation{ID: "scope-uuid", Name: name}, nil
		}
		obs, err := ObserveClientScope(context.Background(), s, newScopeCR("groups"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !obs.ResourceExists {
			t.Errorf("expected ResourceExists=true")
		}
	})

	t.Run("absent scope", func(t *testing.T) {
		s := newScopeStub()
		s.getClientScope = func(_ context.Context, _, _ string) (*clients.ClientScopeRepresentation, error) {
			return nil, nil
		}
		obs, err := ObserveClientScope(context.Background(), s, newScopeCR("missing"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if obs.ResourceExists {
			t.Errorf("expected ResourceExists=false")
		}
	})

	t.Run("GetClientScope error propagates", func(t *testing.T) {
		s := newScopeStub()
		s.getClientScope = func(_ context.Context, _, _ string) (*clients.ClientScopeRepresentation, error) {
			return nil, errors.New("boom")
		}
		if _, err := ObserveClientScope(context.Background(), s, newScopeCR("groups")); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestCreateClientScope(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		if _, err := CreateClientScope(context.Background(), newScopeStub(), newScopeCR("groups")); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("CreateClientScope error propagates", func(t *testing.T) {
		s := newScopeStub()
		s.createErr = errors.New("boom")
		if _, err := CreateClientScope(context.Background(), s, newScopeCR("groups")); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestUpdateClientScope(t *testing.T) {
	t.Run("happy path with existing scope", func(t *testing.T) {
		s := newScopeStub()
		s.getClientScope = func(_ context.Context, _, name string) (*clients.ClientScopeRepresentation, error) {
			return &clients.ClientScopeRepresentation{ID: "scope-uuid", Name: name}, nil
		}
		if _, err := UpdateClientScope(context.Background(), s, newScopeCR("groups")); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if s.updateHits != 1 {
			t.Errorf("expected 1 UpdateClientScope call, got %d", s.updateHits)
		}
	})

	t.Run("missing scope errors", func(t *testing.T) {
		s := newScopeStub()
		s.getClientScope = func(_ context.Context, _, _ string) (*clients.ClientScopeRepresentation, error) {
			return nil, nil
		}
		_, err := UpdateClientScope(context.Background(), s, newScopeCR("missing"))
		if err == nil {
			t.Fatal("expected error when scope is absent")
		}
	})

	t.Run("UpdateClientScope error propagates", func(t *testing.T) {
		s := newScopeStub()
		s.getClientScope = func(_ context.Context, _, name string) (*clients.ClientScopeRepresentation, error) {
			return &clients.ClientScopeRepresentation{ID: "scope-uuid", Name: name}, nil
		}
		s.updateErr = errors.New("boom")
		if _, err := UpdateClientScope(context.Background(), s, newScopeCR("groups")); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestDeleteClientScope(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		if _, err := DeleteClientScope(context.Background(), newScopeStub(), newScopeCR("groups")); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("DeleteClientScope error propagates", func(t *testing.T) {
		s := newScopeStub()
		s.deleteErr = errors.New("boom")
		if _, err := DeleteClientScope(context.Background(), s, newScopeCR("groups")); err == nil {
			t.Fatal("expected error")
		}
	})
}
