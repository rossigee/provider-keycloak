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

package group

import (
	"context"
	"errors"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	groupv1beta1 "github.com/rossigee/provider-keycloak/apis/group/v1beta1"
	"github.com/rossigee/provider-keycloak/internal/clients"
	"github.com/rossigee/provider-keycloak/internal/controller/testhelpers"
)

func groupCR(realm, name string) *groupv1beta1.Group {
	cr := &groupv1beta1.Group{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: groupv1beta1.GroupSpec{
			ForProvider: groupv1beta1.GroupParameters{Name: name},
		},
	}
	if realm != "" {
		cr.Spec.ForProvider.RealmId = &realm
	}
	return cr
}

// The spec names a group; Keycloak addresses it by UUID. Update has to look the
// name up and send the resolved ID back, because PUT /groups with no ID (or the
// wrong one) updates nothing and still returns success - the attributes would
// silently stop being reconciled.
func TestGroupUpdateSendsTheResolvedID(t *testing.T) {
	var got *clients.GroupRepresentation
	m := &mockGroupClient{
		BaseMockClient: &testhelpers.BaseMockClient{},
		searchGroupsFn: func(_ context.Context, _, name string) ([]clients.GroupRepresentation, error) {
			return []clients.GroupRepresentation{
				{ID: "other-id", Name: "unrelated"},
				{ID: "resolved-id", Name: name},
			}, nil
		},
		updateGroupFn: func(_ context.Context, _ string, g *clients.GroupRepresentation) error {
			got = g
			return nil
		},
	}

	e := &external{client: m}
	if _, err := e.Update(context.Background(), groupCR("realm", "admins")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("UpdateGroup was not called")
	}
	if got.ID != "resolved-id" {
		t.Errorf("UpdateGroup sent ID %q, want %q - the resolved UUID, not the name or a sibling's id", got.ID, "resolved-id")
	}
	if got.Name != "admins" {
		t.Errorf("UpdateGroup sent name %q, want %q", got.Name, "admins")
	}
}

// If the search finds nothing, Update must say so rather than reporting success.
func TestGroupUpdateFailsWhenGroupAbsent(t *testing.T) {
	m := &mockGroupClient{
		BaseMockClient: &testhelpers.BaseMockClient{},
		searchGroupsFn: func(_ context.Context, _, _ string) ([]clients.GroupRepresentation, error) {
			return nil, nil
		},
		updateGroupFn: func(_ context.Context, _ string, _ *clients.GroupRepresentation) error {
			t.Error("UpdateGroup was called for a group that does not exist")
			return nil
		},
	}
	e := &external{client: m}
	if _, err := e.Update(context.Background(), groupCR("realm", "missing")); err == nil {
		t.Fatal("expected an error when the group is absent")
	}
}

func TestGroupUpdateSurfacesErrors(t *testing.T) {
	boom := errors.New("boom")

	t.Run("search fails", func(t *testing.T) {
		m := &mockGroupClient{
			BaseMockClient: &testhelpers.BaseMockClient{},
			searchGroupsFn: func(_ context.Context, _, _ string) ([]clients.GroupRepresentation, error) {
				return nil, boom
			},
		}
		e := &external{client: m}
		if _, err := e.Update(context.Background(), groupCR("realm", "admins")); err == nil {
			t.Fatal("expected the search error")
		}
	})

	t.Run("update fails", func(t *testing.T) {
		m := &mockGroupClient{
			BaseMockClient: &testhelpers.BaseMockClient{},
			searchGroupsFn: func(_ context.Context, _, name string) ([]clients.GroupRepresentation, error) {
				return []clients.GroupRepresentation{{ID: "resolved-id", Name: name}}, nil
			},
			updateGroupFn: func(_ context.Context, _ string, _ *clients.GroupRepresentation) error { return boom },
		}
		e := &external{client: m}
		if _, err := e.Update(context.Background(), groupCR("realm", "admins")); err == nil {
			t.Fatal("expected the update error")
		}
	})

	t.Run("realmId missing", func(t *testing.T) {
		m := &mockGroupClient{BaseMockClient: &testhelpers.BaseMockClient{}}
		e := &external{client: m}
		if _, err := e.Update(context.Background(), groupCR("", "admins")); err == nil {
			t.Fatal("expected an error when realmId is unset")
		}
	})
}
