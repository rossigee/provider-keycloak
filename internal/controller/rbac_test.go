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

package controller

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
)

const (
	systemRoleName    = "crossplane:provider:provider-keycloak:system"
	systemBindingName = "crossplane:provider:provider-keycloak:system"
	editRoleName      = "crossplane:provider:provider-keycloak:aggregate-to-edit"
	viewRoleName      = "crossplane:provider:provider-keycloak:aggregate-to-view"
)

func newFakeKube(t *testing.T) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := rbacv1.AddToScheme(scheme); err != nil {
		t.Fatalf("cannot build scheme: %v", err)
	}
	return fake.NewClientBuilder().WithScheme(scheme).Build()
}

func mustRunSetupRBAC(t *testing.T) client.Client {
	t.Helper()
	kube := newFakeKube(t)
	if err := setupRBAC(kube, logging.NewNopLogger()); err != nil {
		t.Fatalf("setupRBAC failed: %v", err)
	}
	return kube
}

func getRole(t *testing.T, kube client.Client, name string) *rbacv1.ClusterRole {
	t.Helper()
	role := &rbacv1.ClusterRole{}
	if err := kube.Get(t.Context(), client.ObjectKey{Name: name}, role); err != nil {
		t.Fatalf("cannot read ClusterRole %s: %v", name, err)
	}
	return role
}

// grantedGroups returns every API group the role has any rule for.
func grantedGroups(rules []rbacv1.PolicyRule) map[string]bool {
	out := map[string]bool{}
	for _, r := range rules {
		for _, g := range r.APIGroups {
			out[g] = true
		}
	}
	return out
}

// crdGroups reads the API groups from the generated CRD manifests. The filename
// is "<group>_<kind>.yaml" by controller-gen convention, so the group is the
// part before the first underscore.
func crdGroups(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join("..", "..", "package", "crds"))
	if err != nil {
		t.Fatalf("cannot read package/crds: %v", err)
	}
	seen := map[string]bool{}
	var groups []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		group, _, ok := strings.Cut(e.Name(), "_")
		if !ok || seen[group] {
			continue
		}
		seen[group] = true
		groups = append(groups, group)
	}
	sort.Strings(groups)
	return groups
}

// TestSetupRBACGrantsEveryCRDGroup is the regression test for the bug class
// that shipped in this provider: the ClusterRole requested permissions on an
// API group that no CRD is actually registered under, so the provider had no
// grant matching its own CRDs and silently failed to read them.
//
// Deriving the expected groups from the generated manifests rather than from a
// hand-written list means a new API group cannot be added without RBAC, and a
// renamed group cannot drift out of sync unnoticed.
func TestSetupRBACGrantsEveryCRDGroup(t *testing.T) {
	kube := mustRunSetupRBAC(t)
	granted := grantedGroups(getRole(t, kube, systemRoleName).Rules)

	var missing []string
	for _, g := range crdGroups(t) {
		if !granted[g] {
			missing = append(missing, g)
		}
	}
	if len(missing) > 0 {
		t.Errorf("the system ClusterRole has no rule for these CRD groups:\n  %s\n"+
			"the provider would be denied access to those resources",
			strings.Join(missing, "\n  "))
	}
}

// TestSetupRBACProviderConfigUsesTheRegisteredGroup pins the specific defect.
// keycloak.m.crossplane.io is what apis/v1beta1 declares and what
// package/crds/keycloak.m.crossplane.io_providerconfigs.yaml is generated
// under; keycloak.crossplane.io matched nothing.
func TestSetupRBACProviderConfigUsesTheRegisteredGroup(t *testing.T) {
	kube := mustRunSetupRBAC(t)
	rules := getRole(t, kube, systemRoleName).Rules

	var wrongGroup bool
	for _, r := range rules {
		for _, g := range r.APIGroups {
			if g == "keycloak.crossplane.io" {
				wrongGroup = true
			}
		}
	}
	if wrongGroup {
		t.Error("RBAC references keycloak.crossplane.io, which is not a registered API group; " +
			"the ProviderConfig CRDs are generated under keycloak.m.crossplane.io")
	}

	// And the resources must actually be covered, not just the group named.
	var haveResources bool
	for _, r := range rules {
		if !contains(r.APIGroups, "keycloak.m.crossplane.io") {
			continue
		}
		for _, res := range r.Resources {
			if res == "providerconfigs" || res == "providerconfigusages" {
				haveResources = true
			}
		}
	}
	if !haveResources {
		t.Error("no rule grants providerconfigs or providerconfigusages under keycloak.m.crossplane.io")
	}
}

// TestSetupRBACGrantsFinalizersForEveryGroup mirrors the resource rules into
// the */finalizers rule. A group present in the CRDs but missing there means
// the provider could not remove its own finalizers, so a deleted resource would
// never terminate.
func TestSetupRBACGrantsFinalizersForEveryGroup(t *testing.T) {
	kube := mustRunSetupRBAC(t)
	rules := getRole(t, kube, systemRoleName).Rules

	var finalizerGroups map[string]bool
	for _, r := range rules {
		if len(r.Resources) == 1 && r.Resources[0] == "*/finalizers" {
			finalizerGroups = map[string]bool{}
			for _, g := range r.APIGroups {
				finalizerGroups[g] = true
			}
		}
	}
	if finalizerGroups == nil {
		t.Fatal("no */finalizers rule found on the system ClusterRole")
	}

	var missing []string
	for _, g := range crdGroups(t) {
		if !finalizerGroups[g] {
			missing = append(missing, g)
		}
	}
	if len(missing) > 0 {
		t.Errorf("these CRD groups cannot have their finalizers updated:\n  %s",
			strings.Join(missing, "\n  "))
	}
}

// TestSetupRBACGrantsStructuredEvents covers the grant that was missing while
// package/crossplane.yaml already declared it. Without it the provider cannot
// record events on Kubernetes 1.19+.
func TestSetupRBACGrantsStructuredEvents(t *testing.T) {
	kube := mustRunSetupRBAC(t)
	rules := getRole(t, kube, systemRoleName).Rules

	var ok bool
	for _, r := range rules {
		if !contains(r.APIGroups, "events.k8s.io") {
			continue
		}
		for _, res := range r.Resources {
			if res == "events" {
				ok = true
			}
		}
	}
	if !ok {
		t.Error("no rule grants create/patch/update on events.k8s.io events")
	}
}

func TestSetupRBACCreatesEveryRoleAndBinding(t *testing.T) {
	kube := mustRunSetupRBAC(t)

	for _, name := range []string{systemRoleName, editRoleName, viewRoleName} {
		if err := kube.Get(t.Context(), client.ObjectKey{Name: name}, &rbacv1.ClusterRole{}); err != nil {
			t.Errorf("ClusterRole %s was not created: %v", name, err)
		}
	}
	binding := &rbacv1.ClusterRoleBinding{}
	if err := kube.Get(t.Context(), client.ObjectKey{Name: systemBindingName}, binding); err != nil {
		t.Fatalf("ClusterRoleBinding was not created: %v", err)
	}
	if binding.RoleRef.Name != systemRoleName {
		t.Errorf("binding RoleRef = %q, want %q", binding.RoleRef.Name, systemRoleName)
	}
	if len(binding.Subjects) != 1 || binding.Subjects[0].Kind != "ServiceAccount" {
		t.Errorf("binding subjects = %+v, want a single ServiceAccount", binding.Subjects)
	}
}

func TestSetupRBACLabelsAggregateRoles(t *testing.T) {
	kube := mustRunSetupRBAC(t)

	edit := getRole(t, kube, editRoleName)
	for _, label := range []string{
		"rbac.crossplane.io/aggregate-to-edit",
		"rbac.crossplane.io/aggregate-to-admin",
		"rbac.crossplane.io/aggregate-to-crossplane",
	} {
		if edit.Labels[label] != "true" {
			t.Errorf("edit role missing label %s", label)
		}
	}

	view := getRole(t, kube, viewRoleName)
	if view.Labels["rbac.crossplane.io/aggregate-to-view"] != "true" {
		t.Error("view role missing aggregate-to-view label")
	}
}

// TestSetupRBACIsIdempotent covers a restart against a cluster that already has
// the roles: AlreadyExists must be tolerated rather than returned as an error.
func TestSetupRBACIsIdempotent(t *testing.T) {
	kube := mustRunSetupRBAC(t)
	if err := setupRBAC(kube, logging.NewNopLogger()); err != nil {
		t.Fatalf("setupRBAC must be safe to run again, got: %v", err)
	}
}

// TestWithVerbsKeepsGroupsAndResources checks the helper that narrows the
// aggregate roles, since a slip there would silently drop every group.
func TestWithVerbsKeepsGroupsAndResources(t *testing.T) {
	in := []rbacv1.PolicyRule{
		{APIGroups: []string{"a"}, Resources: []string{"x"}, Verbs: []string{"get"}},
		{APIGroups: []string{"b"}, Resources: []string{"y"}, Verbs: []string{"get"}},
	}
	out := withVerbs(in, []string{"*"})

	if len(out) != len(in) {
		t.Fatalf("len = %d, want %d", len(out), len(in))
	}
	for i := range in {
		if len(out[i].APIGroups) != len(in[i].APIGroups) || out[i].APIGroups[0] != in[i].APIGroups[0] {
			t.Errorf("rule %d groups changed: %v", i, out[i].APIGroups)
		}
		if len(out[i].Verbs) != 1 || out[i].Verbs[0] != "*" {
			t.Errorf("rule %d verbs = %v, want [*]", i, out[i].Verbs)
		}
	}
	if in[0].Verbs[0] != "get" {
		t.Error("withVerbs mutated its input")
	}
}

func contains(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
}
