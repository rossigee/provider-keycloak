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

package clientrolemapping

import (
	"context"
	"strings"

	xpcontroller "github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/pkg/errors"
	"github.com/rossigee/provider-keycloak/internal/features"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	crv1beta1 "github.com/rossigee/provider-keycloak/apis/rolemappings/v1beta1"
	"github.com/rossigee/provider-keycloak/apis/v1beta1"
	"github.com/rossigee/provider-keycloak/internal/clients"
	"github.com/rossigee/provider-keycloak/internal/controller/deletecomplete"
	"github.com/rossigee/provider-keycloak/internal/controller/mappingreconcile"
	"github.com/rossigee/provider-keycloak/internal/tracing"
)

const (
	errNotClientRoleMapping = "managed resource is not a ClientRoleMapping"
	errGetProviderConfig    = "cannot get ProviderConfig"
	errProviderNotReady     = "provider is not ready"
	controllerName          = "clientrolemappings.rolemappings.keycloak.m.crossplane.io"
)

func Setup(mgr ctrl.Manager, o xpcontroller.Options) error {
	opts := []managed.ReconcilerOption{
		managed.WithExternalConnector(&connector{kube: mgr.GetClient()}),
		managed.WithLogger(o.Logger.WithValues("controller", "ClientRoleMapping")),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorder(controllerName))),
	}
	if o.Features.Enabled(features.EnableAlphaManagementPolicies) {
		opts = append(opts, managed.WithManagementPolicies())
	}

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(crv1beta1.SchemeGroupVersion.WithKind("ClientRoleMapping")),
		opts...)
	return ctrl.NewControllerManagedBy(mgr).
		Named(controllerName).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&crv1beta1.ClientRoleMapping{}).
		Complete(r)
}

type connector struct{ kube client.Client }
type external struct {
	client clients.Client
	kube   client.Client
}

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*crv1beta1.ClientRoleMapping)
	if !ok {
		return nil, errors.New(errNotClientRoleMapping)
	}
	pcRef := cr.Spec.ProviderConfigReference
	if pcRef == nil {
		return nil, errors.New(errGetProviderConfig + ": providerConfigRef is required")
	}
	pc := &v1beta1.ProviderConfig{}
	if err := c.kube.Get(ctx, client.ObjectKey{Name: pcRef.Name}, pc); err != nil {
		return nil, errors.Wrap(err, errGetProviderConfig)
	}
	if pc.Status.GetCondition(xpv1.TypeReady).Status != "True" {
		return nil, errors.New(errProviderNotReady)
	}
	shared := clients.GetConnector(c.kube)
	kc, err := shared.Connect(ctx, pcRef.Name)
	if err != nil {
		return nil, errors.Wrap(err, "cannot connect to Keycloak")
	}
	return &external{client: kc, kube: c.kube}, nil
}

func (e *external) Disconnect(_ context.Context) error { return nil }

func (e *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	_, span := tracing.StartSpan(ctx, "clientrolemapping.observe",
		tracing.SpanAttrs("ClientRoleMapping", mg.GetName(), "observe")...)
	defer span.End()

	cr, ok := mg.(*crv1beta1.ClientRoleMapping)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotClientRoleMapping)
	}

	// Delete has already released the mappings this resource owns. Report them
	// gone so the reconciler reaches RemoveFinalizer instead of re-running
	// Delete, which would otherwise re-apply the removals on every pass.
	if deletecomplete.Done(cr) {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}
	current, err := e.client.ListUserClientRoleMappings(ctx, cr.Spec.ForProvider.RealmId, cr.Spec.ForProvider.UserId, cr.Spec.ForProvider.ClientId)
	if err != nil {
		return managed.ExternalObservation{}, err
	}
	cr.Status.SetConditions(xpv1.Available())

	// This set is additive: the resource owns the roles it declares and no
	// others, so several resources may share one user and client. Entries in
	// current that this resource does not own - a sibling's, or added in
	// Keycloak directly - neither make it out of date nor get removed.
	declared := declaredEntries(cr.Spec.ForProvider.Roles)
	owned := mappingreconcile.Prune(ownedEntries(cr.Status.AppliedRoles), currentEntries(current))
	cr.Status.AppliedRoles = fromEntries(owned)

	// Out of date when a declared role is missing, and equally when a role this
	// resource owns is no longer declared. Checking only the first would mean a
	// role dropped from the spec is never taken off the user: the resource would
	// report itself settled while still holding the role, because Update is
	// never reached.
	_, stale := mappingreconcile.Plan(declared, owned, currentEntries(current))

	return managed.ExternalObservation{
		ResourceExists: true,
		ResourceUpToDate: mappingreconcile.UpToDate(declared, currentEntries(current)) &&
			len(stale) == 0,
	}, nil
}

func (e *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	_, span := tracing.StartSpan(ctx, "clientrolemapping.create",
		tracing.SpanAttrs("ClientRoleMapping", mg.GetName(), "create")...)
	defer span.End()

	cr, ok := mg.(*crv1beta1.ClientRoleMapping)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotClientRoleMapping)
	}
	roles := toRoleRepresentations(declaredEntries(cr.Spec.ForProvider.Roles))
	if err := e.client.AddUserClientRoleMappings(ctx, cr.Spec.ForProvider.RealmId, cr.Spec.ForProvider.UserId, cr.Spec.ForProvider.ClientId, roles); err != nil {
		return managed.ExternalCreation{}, err
	}
	cr.Status.SetConditions(xpv1.Creating())

	// Record ownership as soon as the write lands, so a later pass can tell
	// these roles from a sibling's and a Delete can remove exactly them.
	cr.Status.AppliedRoles = cr.Spec.ForProvider.Roles
	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	_, span := tracing.StartSpan(ctx, "clientrolemapping.update",
		tracing.SpanAttrs("ClientRoleMapping", mg.GetName(), "update")...)
	defer span.End()

	cr, ok := mg.(*crv1beta1.ClientRoleMapping)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotClientRoleMapping)
	}
	current, err := e.client.ListUserClientRoleMappings(ctx, cr.Spec.ForProvider.RealmId, cr.Spec.ForProvider.UserId, cr.Spec.ForProvider.ClientId)
	if err != nil {
		return managed.ExternalUpdate{}, err
	}
	declared := declaredEntries(cr.Spec.ForProvider.Roles)
	owned := ownedEntries(cr.Status.AppliedRoles)

	// Additions are declared roles the user is missing. Removals are roles this
	// resource applied that the spec no longer declares - and only those. A role
	// belonging to a sibling resource is absent from owned, so it is never taken.
	add, remove := mappingreconcile.Plan(declared, owned, currentEntries(current))
	if len(add) > 0 {
		if err := e.client.AddUserClientRoleMappings(ctx, cr.Spec.ForProvider.RealmId, cr.Spec.ForProvider.UserId, cr.Spec.ForProvider.ClientId, toRoleRepresentations(add)); err != nil {
			return managed.ExternalUpdate{}, err
		}
	}
	if len(remove) > 0 {
		if err := e.client.RemoveUserClientRoleMappings(ctx, cr.Spec.ForProvider.RealmId, cr.Spec.ForProvider.UserId, cr.Spec.ForProvider.ClientId, toRoleRepresentations(remove)); err != nil {
			return managed.ExternalUpdate{}, err
		}
	}

	// Ownership is the declared set from here, pruned against what the user
	// actually has, so a failed write is not recorded as applied.
	cr.Status.AppliedRoles = fromEntries(mappingreconcile.Adopt(declared, append(currentEntries(current), add...)))
	return managed.ExternalUpdate{}, nil
}

func (e *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	_, span := tracing.StartSpan(ctx, "clientrolemapping.delete",
		tracing.SpanAttrs("ClientRoleMapping", mg.GetName(), "delete")...)
	defer span.End()

	cr, ok := mg.(*crv1beta1.ClientRoleMapping)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errNotClientRoleMapping)
	}

	// Already released. Removing again would strip mappings a sibling resource
	// may have since declared, so Delete must be safe to call on its own.
	if deletecomplete.Done(cr) {
		return managed.ExternalDelete{}, nil
	}
	current, err := e.client.ListUserClientRoleMappings(ctx, cr.Spec.ForProvider.RealmId, cr.Spec.ForProvider.UserId, cr.Spec.ForProvider.ClientId)
	if err != nil && !strings.Contains(err.Error(), "404") {
		return managed.ExternalDelete{}, err
	}

	// Remove only the roles this resource owns and that are still present.
	// Removing every role on the user would strip a sibling resource's entries
	// as well, which is the failure additive ownership exists to prevent.
	release := mappingreconcile.Release(ownedEntries(cr.Status.AppliedRoles), currentEntries(current))
	if len(release) > 0 {
		if err := e.client.RemoveUserClientRoleMappings(ctx, cr.Spec.ForProvider.RealmId, cr.Spec.ForProvider.UserId, cr.Spec.ForProvider.ClientId, toRoleRepresentations(release)); err != nil {
			return managed.ExternalDelete{}, err
		}
	}
	cr.Status.SetConditions(xpv1.Deleting())
	return managed.ExternalDelete{}, deletecomplete.Mark(ctx, e.kube, cr)
}

// declaredEntries projects the spec's roles into the shared reconciliation type.
func declaredEntries(roles []crv1beta1.RoleMapping) []mappingreconcile.Entry {
	return mappingreconcile.Convert(roles, func(r crv1beta1.RoleMapping) mappingreconcile.Entry {
		return mappingreconcile.Entry{ID: r.Id, Name: r.Name}
	})
}

// ownedEntries projects the roles this resource last applied.
func ownedEntries(applied []crv1beta1.RoleMapping) []mappingreconcile.Entry {
	return mappingreconcile.Convert(applied, func(r crv1beta1.RoleMapping) mappingreconcile.Entry {
		return mappingreconcile.Entry{ID: r.Id, Name: r.Name}
	})
}

// currentEntries projects the roles Keycloak currently has on the user.
func currentEntries(current []clients.RoleRepresentation) []mappingreconcile.Entry {
	return mappingreconcile.Convert(current, func(r clients.RoleRepresentation) mappingreconcile.Entry {
		return mappingreconcile.Entry{ID: r.ID, Name: r.Name}
	})
}

func fromEntries(entries []mappingreconcile.Entry) []crv1beta1.RoleMapping {
	out := make([]crv1beta1.RoleMapping, 0, len(entries))
	for _, e := range entries {
		out = append(out, crv1beta1.RoleMapping{Id: e.ID, Name: e.Name})
	}
	return out
}

func toRoleRepresentations(entries []mappingreconcile.Entry) []clients.RoleRepresentation {
	result := make([]clients.RoleRepresentation, len(entries))
	for i, r := range entries {
		result[i] = clients.RoleRepresentation{ID: r.ID, Name: r.Name}
	}
	return result
}
