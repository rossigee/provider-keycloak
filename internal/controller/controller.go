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
	"context"
	"os"

	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/rossigee/provider-keycloak/internal/controller/authenticationflow"
	"github.com/rossigee/provider-keycloak/internal/controller/authorizationpolicy"
	"github.com/rossigee/provider-keycloak/internal/controller/authz"
	"github.com/rossigee/provider-keycloak/internal/controller/client"
	"github.com/rossigee/provider-keycloak/internal/controller/clientcertificates"
	"github.com/rossigee/provider-keycloak/internal/controller/clientdefaultscopes"
	"github.com/rossigee/provider-keycloak/internal/controller/clientinitialaccess"
	"github.com/rossigee/provider-keycloak/internal/controller/clientoptionalscopes"
	"github.com/rossigee/provider-keycloak/internal/controller/clientrolemapping"
	"github.com/rossigee/provider-keycloak/internal/controller/clientscope"
	"github.com/rossigee/provider-keycloak/internal/controller/clientscopemapping"
	"github.com/rossigee/provider-keycloak/internal/controller/component"
	"github.com/rossigee/provider-keycloak/internal/controller/events"
	"github.com/rossigee/provider-keycloak/internal/controller/group"
	"github.com/rossigee/provider-keycloak/internal/controller/identityprovider"
	"github.com/rossigee/provider-keycloak/internal/controller/protocolmapper"
	"github.com/rossigee/provider-keycloak/internal/controller/providerconfig"
	"github.com/rossigee/provider-keycloak/internal/controller/realm"
	"github.com/rossigee/provider-keycloak/internal/controller/realmimpexp"
	"github.com/rossigee/provider-keycloak/internal/controller/realmkeys"
	"github.com/rossigee/provider-keycloak/internal/controller/role"
	"github.com/rossigee/provider-keycloak/internal/controller/user"
	"github.com/rossigee/provider-keycloak/internal/controller/userfederation"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// Setup sets up Keycloak provider controllers.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	if err := setupRBAC(mgr.GetClient(), o.Logger); err != nil {
		o.Logger.Info("RBAC setup warning (may be transient)", "error", err)
	}
	if err := providerconfig.Setup(mgr); err != nil {
		return err
	}
	for _, setup := range []func(ctrl.Manager, controller.Options) error{
		authenticationflow.Setup,
		authorizationpolicy.Setup,
		client.Setup,
		realm.Setup,
		user.Setup,
		group.Setup,
		role.Setup,
		protocolmapper.Setup,
		authz.Setup,
		clientcertificates.Setup,
		events.Setup,
		realmimpexp.Setup,
		userfederation.Setup,
		clientrolemapping.Setup,
		clientscopemapping.Setup,
		clientscope.Setup,
		clientdefaultscopes.Setup,
		clientoptionalscopes.Setup,
		clientinitialaccess.Setup,
		component.Setup,
		realmkeys.Setup,
		identityprovider.Setup,
	} {
		if err := setup(mgr, o); err != nil {
			return err
		}
	}
	return nil
}

func setupRBAC(c k8sclient.Client, l logging.Logger) error {
	ctx := context.Background()

	rules := []rbacv1.PolicyRule{
		{APIGroups: []string{""}, Resources: []string{"secrets"}, Verbs: []string{"get", "list", "watch"}},
		{APIGroups: []string{"authenticationflow.keycloak.crossplane.io"}, Resources: []string{"authenticationflows", "authenticationflows/status"}, Verbs: []string{"get", "list", "watch", "update", "patch", "create"}},
		{APIGroups: []string{"authorizationpolicy.keycloak.crossplane.io"}, Resources: []string{"authorizationpolicies", "authorizationpolicies/status"}, Verbs: []string{"get", "list", "watch", "update", "patch", "create"}},
		{APIGroups: []string{"authz.keycloak.crossplane.io"}, Resources: []string{"authzresources", "authzresources/status"}, Verbs: []string{"get", "list", "watch", "update", "patch", "create"}},
		{APIGroups: []string{"clientcertificates.keycloak.crossplane.io"}, Resources: []string{"clientcertificates", "clientcertificates/status"}, Verbs: []string{"get", "list", "watch", "update", "patch", "create"}},
		{APIGroups: []string{"openidclient.keycloak.crossplane.io"}, Resources: []string{"clientdefaultscopes", "clientdefaultscopes/status", "clientoptionalscopes", "clientoptionalscopes/status", "clients", "clients/status"}, Verbs: []string{"get", "list", "watch", "update", "patch", "create"}},
		{APIGroups: []string{"clientinitialaccess.keycloak.crossplane.io"}, Resources: []string{"clientinitialaccesses", "clientinitialaccesses/status"}, Verbs: []string{"get", "list", "watch", "update", "patch", "create"}},
		{APIGroups: []string{"rolemappings.keycloak.crossplane.io"}, Resources: []string{"clientrolemappings", "clientrolemappings/status"}, Verbs: []string{"get", "list", "watch", "update", "patch", "create"}},
		{APIGroups: []string{"scopes.keycloak.crossplane.io"}, Resources: []string{"clientscopemappings", "clientscopemappings/status", "clientscopes", "clientscopes/status"}, Verbs: []string{"get", "list", "watch", "update", "patch", "create"}},
		{APIGroups: []string{"component.keycloak.crossplane.io"}, Resources: []string{"components", "components/status"}, Verbs: []string{"get", "list", "watch", "update", "patch", "create"}},
		{APIGroups: []string{"group.keycloak.crossplane.io"}, Resources: []string{"groups", "groups/status"}, Verbs: []string{"get", "list", "watch", "update", "patch", "create"}},
		{APIGroups: []string{"user.keycloak.crossplane.io"}, Resources: []string{"groups", "groups/status", "users", "users/status"}, Verbs: []string{"get", "list", "watch", "update", "patch", "create"}},
		{APIGroups: []string{"identityprovider.keycloak.crossplane.io"}, Resources: []string{"identityproviders", "identityproviders/status"}, Verbs: []string{"get", "list", "watch", "update", "patch", "create"}},
		{APIGroups: []string{"client.keycloak.crossplane.io"}, Resources: []string{"protocolmappers", "protocolmappers/status"}, Verbs: []string{"get", "list", "watch", "update", "patch", "create"}},
		{APIGroups: []string{"keycloak.crossplane.io"}, Resources: []string{"providerconfigs", "providerconfigs/status", "providerconfigusages", "providerconfigusages/status"}, Verbs: []string{"get", "list", "watch", "update", "patch", "create"}},
		{APIGroups: []string{"events.keycloak.crossplane.io"}, Resources: []string{"realmeventsconfigs", "realmeventsconfigs/status"}, Verbs: []string{"get", "list", "watch", "update", "patch", "create"}},
		{APIGroups: []string{"realmimpexp.keycloak.crossplane.io"}, Resources: []string{"realmimports", "realmimports/status"}, Verbs: []string{"get", "list", "watch", "update", "patch", "create"}},
		{APIGroups: []string{"keys.keycloak.crossplane.io"}, Resources: []string{"realmkeys", "realmkeys/status"}, Verbs: []string{"get", "list", "watch", "update", "patch", "create"}},
		{APIGroups: []string{"realm.keycloak.crossplane.io"}, Resources: []string{"realms", "realms/status"}, Verbs: []string{"get", "list", "watch", "update", "patch", "create"}},
		{APIGroups: []string{"role.keycloak.crossplane.io"}, Resources: []string{"roles", "roles/status"}, Verbs: []string{"get", "list", "watch", "update", "patch", "create"}},
		{APIGroups: []string{"userfederation.keycloak.crossplane.io"}, Resources: []string{"userfederationproviders", "userfederationproviders/status"}, Verbs: []string{"get", "list", "watch", "update", "patch", "create"}},
		{APIGroups: []string{"users.user.keycloak.crossplane.io"}, Resources: []string{"groups", "groups/status"}, Verbs: []string{"get", "list", "watch", "update", "patch", "create"}},
		{
			APIGroups: []string{"authenticationflow.keycloak.crossplane.io", "authorizationpolicy.keycloak.crossplane.io", "authz.keycloak.crossplane.io", "clientcertificates.keycloak.crossplane.io", "openidclient.keycloak.crossplane.io", "clientinitialaccess.keycloak.crossplane.io", "rolemappings.keycloak.crossplane.io", "scopes.keycloak.crossplane.io", "component.keycloak.crossplane.io", "group.keycloak.crossplane.io", "user.keycloak.crossplane.io", "identityprovider.keycloak.crossplane.io", "client.keycloak.crossplane.io", "keycloak.crossplane.io", "events.keycloak.crossplane.io", "realmimpexp.keycloak.crossplane.io", "keys.keycloak.crossplane.io", "realm.keycloak.crossplane.io", "role.keycloak.crossplane.io", "userfederation.keycloak.crossplane.io", "users.user.keycloak.crossplane.io"},
			Resources: []string{"*/finalizers"},
			Verbs:     []string{"update"},
		},
		{APIGroups: []string{"", "coordination.k8s.io"}, Resources: []string{"secrets", "configmaps", "events", "leases"}, Verbs: []string{"*"}},
	}

	system := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "crossplane:provider:provider-keycloak:system",
			Labels: map[string]string{
				"rbac.crossplane.io/system": "provider-keycloak",
			},
		},
		Rules: rules,
	}
	if err := c.Create(ctx, system); err != nil && !errors.IsAlreadyExists(err) {
		return err
	}
	if err := c.Update(ctx, system); err != nil {
		l.Info("system role update", "err", err)
	}

	binding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "crossplane:provider:provider-keycloak:system",
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "crossplane:provider:provider-keycloak:system",
		},
		Subjects: []rbacv1.Subject{{
			Kind:      "ServiceAccount",
			Name:      os.Getenv("REVISION_NAME"),
			Namespace: "crossplane-system",
		}},
	}
	if err := c.Create(ctx, binding); err != nil && !errors.IsAlreadyExists(err) {
		return err
	}
	if err := c.Update(ctx, binding); err != nil {
		l.Info("system binding update", "err", err)
	}

	edit := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "crossplane:provider:provider-keycloak:aggregate-to-edit",
			Labels: map[string]string{
				"rbac.crossplane.io/aggregate-to-edit":       "true",
				"rbac.crossplane.io/aggregate-to-admin":      "true",
				"rbac.crossplane.io/aggregate-to-crossplane": "true",
				"rbac.crossplane.io/system":                  "provider-keycloak",
			},
		},
		Rules: withVerbs(rules, []string{"*"}),
	}
	if err := c.Create(ctx, edit); err != nil && !errors.IsAlreadyExists(err) {
		l.Info("aggregate-to-edit create warning (non-fatal)", "err", err)
	}
	_ = c.Update(ctx, edit)

	view := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "crossplane:provider:provider-keycloak:aggregate-to-view",
			Labels: map[string]string{
				"rbac.crossplane.io/aggregate-to-view": "true",
				"rbac.crossplane.io/system":            "provider-keycloak",
			},
		},
		Rules: withVerbs(rules, []string{"get", "list", "watch"}),
	}
	if err := c.Create(ctx, view); err != nil && !errors.IsAlreadyExists(err) {
		l.Info("aggregate-to-view create warning (non-fatal)", "err", err)
	}
	_ = c.Update(ctx, view)

	l.Info("provider self-managed RBAC roles ensured")
	return nil
}

func withVerbs(r []rbacv1.PolicyRule, verbs []string) []rbacv1.PolicyRule {
	out := make([]rbacv1.PolicyRule, len(r))
	for i := range r {
		out[i] = r[i]
		out[i].Verbs = verbs
	}
	return out
}
