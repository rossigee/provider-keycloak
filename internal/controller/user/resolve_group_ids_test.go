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
	"errors"
	"sort"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"

	groupv1beta1 "github.com/rossigee/provider-keycloak/apis/group/v1beta1"
	userv1beta1 "github.com/rossigee/provider-keycloak/apis/user/v1beta1"
	"github.com/rossigee/provider-keycloak/internal/clients"
	"github.com/rossigee/provider-keycloak/internal/controller/testhelpers"
)

// groupStub resolves group names to IDs, standing in for Keycloak's search.
type groupStub struct {
	*testhelpers.BaseMockClient
	byName map[string][]clients.GroupRepresentation
	err    error
	calls  int
}

func (s *groupStub) SearchGroups(_ context.Context, _, _ string) ([]clients.GroupRepresentation, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	var out []clients.GroupRepresentation
	for _, g := range s.byName {
		out = append(out, g...)
	}
	return out, nil
}

func groupCR(name, groupName string) *groupv1beta1.Group {
	return &groupv1beta1.Group{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "ns"},
		Spec: groupv1beta1.GroupSpec{
			ForProvider: groupv1beta1.GroupParameters{Name: groupName},
		},
	}
}

func kubeWithGroups(t *testing.T, groups ...*groupv1beta1.Group) client.Client {
	t.Helper()
	sch := runtime.NewScheme()
	if err := groupv1beta1.AddToScheme(sch); err != nil {
		t.Fatalf("scheme: %v", err)
	}
	objs := make([]client.Object, 0, len(groups))
	for _, g := range groups {
		objs = append(objs, g)
	}
	return fake.NewClientBuilder().WithScheme(sch).WithObjects(objs...).Build()
}

func sortedCopy(in []string) []string {
	out := append([]string{}, in...)
	sort.Strings(out)
	return out
}

func sameIDs(a, b []string) bool {
	a, b = sortedCopy(a), sortedCopy(b)
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Literal groupIds are used as-is; GroupIdsRefs are resolved by reading the
// referenced Group CR's spec.name and searching Keycloak for a matching name.
func TestResolveGroupIDs(t *testing.T) {
	ctx := context.Background()

	t.Run("literal ids pass through untouched", func(t *testing.T) {
		e := &groupsExternal{kube: kubeWithGroups(t)}
		p := &userv1beta1.GroupsParameters{GroupIds: []string{"id-a", "id-b"}}

		got, err := e.resolveGroupIDs(ctx, "realm", p, "ns")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !sameIDs(got, []string{"id-a", "id-b"}) {
			t.Errorf("got %v, want [id-a id-b]", got)
		}
	})

	t.Run("refs are resolved through the Group CR's name", func(t *testing.T) {
		kube := kubeWithGroups(t,
			groupCR("admins-grp", "admins"),
			groupCR("devs-grp", "developers"),
		)
		stub := &groupStub{BaseMockClient: &testhelpers.BaseMockClient{}, byName: map[string][]clients.GroupRepresentation{
			"admins": {{ID: "resolved-admins", Name: "admins"}},
			"devs":   {{ID: "resolved-devs", Name: "developers"}},
		}}
		e := &groupsExternal{kube: kube, client: stub}

		p := &userv1beta1.GroupsParameters{
			GroupIds:     []string{"literal-id"},
			GroupIdsRefs: []xpv1.Reference{{Name: "admins-grp"}, {Name: "devs-grp"}},
		}

		got, err := e.resolveGroupIDs(ctx, "realm", p, "ns")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !sameIDs(got, []string{"literal-id", "resolved-admins", "resolved-devs"}) {
			t.Errorf("got %v, want literals plus both resolved ids", got)
		}
	})

	// Keycloak's search is fuzzy, so a name can come back alongside decoys.
	// Only the exact name match may be used.
	t.Run("picks the exact name match, not a decoy", func(t *testing.T) {
		kube := kubeWithGroups(t, groupCR("admins-grp", "admins"))
		stub := &groupStub{BaseMockClient: &testhelpers.BaseMockClient{}, byName: map[string][]clients.GroupRepresentation{
			"admins": {
				{ID: "decoy-1", Name: "admins-legacy"},
				{ID: "decoy-2", Name: "super-admins"},
				{ID: "exact", Name: "admins"},
			},
		}}
		e := &groupsExternal{kube: kube, client: stub}

		p := &userv1beta1.GroupsParameters{GroupIdsRefs: []xpv1.Reference{{Name: "admins-grp"}}}

		got, err := e.resolveGroupIDs(ctx, "realm", p, "ns")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !sameIDs(got, []string{"exact"}) {
			t.Errorf("got %v, want [exact] - a fuzzy match must not be accepted", got)
		}
	})

	t.Run("an unnamed ref is skipped", func(t *testing.T) {
		e := &groupsExternal{kube: kubeWithGroups(t)}
		p := &userv1beta1.GroupsParameters{
			GroupIds:     []string{"literal"},
			GroupIdsRefs: []xpv1.Reference{{Name: ""}},
		}

		got, err := e.resolveGroupIDs(ctx, "realm", p, "ns")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !sameIDs(got, []string{"literal"}) {
			t.Errorf("got %v, want [literal]", got)
		}
	})

	t.Run("errors when nothing is specified at all", func(t *testing.T) {
		e := &groupsExternal{kube: kubeWithGroups(t)}
		if _, err := e.resolveGroupIDs(ctx, "realm", &userv1beta1.GroupsParameters{}, "ns"); err == nil {
			t.Error("expected an error when neither ids nor refs are given")
		}
	})
}

func TestResolveGroupIDsErrors(t *testing.T) {
	ctx := context.Background()
	boom := errors.New("boom")

	t.Run("referenced Group does not exist", func(t *testing.T) {
		e := &groupsExternal{kube: kubeWithGroups(t)}
		p := &userv1beta1.GroupsParameters{GroupIdsRefs: []xpv1.Reference{{Name: "missing"}}}

		if _, err := e.resolveGroupIDs(ctx, "realm", p, "ns"); err == nil {
			t.Fatal("expected an error for a reference to a Group that does not exist")
		}
	})

	t.Run("search fails", func(t *testing.T) {
		kube := kubeWithGroups(t, groupCR("admins-grp", "admins"))
		e := &groupsExternal{kube: kube, client: &groupStub{
			BaseMockClient: &testhelpers.BaseMockClient{}, err: boom,
		}}
		p := &userv1beta1.GroupsParameters{GroupIdsRefs: []xpv1.Reference{{Name: "admins-grp"}}}

		if _, err := e.resolveGroupIDs(ctx, "realm", p, "ns"); err == nil {
			t.Fatal("expected the search error")
		}
	})

	// The referenced Group exists but Keycloak has no group of that name -
	// typically the Group CR was created but never applied.
	t.Run("resolved group is absent in Keycloak", func(t *testing.T) {
		kube := kubeWithGroups(t, groupCR("admins-grp", "admins"))
		e := &groupsExternal{kube: kube, client: &groupStub{
			BaseMockClient: &testhelpers.BaseMockClient{},
			byName:         map[string][]clients.GroupRepresentation{"other": {{ID: "x", Name: "other"}}},
		}}
		p := &userv1beta1.GroupsParameters{GroupIdsRefs: []xpv1.Reference{{Name: "admins-grp"}}}

		if _, err := e.resolveGroupIDs(ctx, "realm", p, "ns"); err == nil {
			t.Fatal("expected an error when the referenced group is not in Keycloak")
		}
	})
}

// The resolved IDs are what sync() adds the user to, so a fuzzy match would put
// them in the wrong group - a privilege assignment, not a cosmetic error.
func TestResolveGroupIDsNeverReturnsADecoy(t *testing.T) {
	kube := kubeWithGroups(t, groupCR("admins-grp", "admins"))
	stub := &groupStub{BaseMockClient: &testhelpers.BaseMockClient{}, byName: map[string][]clients.GroupRepresentation{
		"admins": {
			{ID: "admins-legacy-id", Name: "admins-legacy"},
			{ID: "legacy-2", Name: "legacy-admins"},
		},
	}}
	e := &groupsExternal{kube: kube, client: stub}

	p := &userv1beta1.GroupsParameters{GroupIdsRefs: []xpv1.Reference{{Name: "admins-grp"}}}
	if _, err := e.resolveGroupIDs(context.Background(), "realm", p, "ns"); err == nil {
		t.Fatal("a group with no exact name match must be an error, not a fuzzy hit")
	}
}
