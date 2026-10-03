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

package clientscopemapping

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

	csv1beta1 "github.com/rossigee/provider-keycloak/apis/scopes/v1beta1"
	"github.com/rossigee/provider-keycloak/apis/v1beta1"
	"github.com/rossigee/provider-keycloak/internal/clients"
	"github.com/rossigee/provider-keycloak/internal/controller/deletecomplete"
	"github.com/rossigee/provider-keycloak/internal/controller/mappingreconcile"
	"github.com/rossigee/provider-keycloak/internal/tracing"
)

const (
	errNotClientScopeMapping = "managed resource is not a ClientScopeMapping"
	errGetProviderConfig     = "cannot get ProviderConfig"
	errProviderNotReady      = "provider is not ready"
	controllerName           = "clientscopemappings.scopes.keycloak.m.crossplane.io"
)

func Setup(mgr ctrl.Manager, o xpcontroller.Options) error {
	opts := []managed.ReconcilerOption{
		managed.WithExternalConnector(&connector{kube: mgr.GetClient()}),
		managed.WithLogger(o.Logger.WithValues("controller", "ClientScopeMapping")),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorder(controllerName))),
	}
	if o.Features.Enabled(features.EnableAlphaManagementPolicies) {
		opts = append(opts, managed.WithManagementPolicies())
	}

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(csv1beta1.SchemeGroupVersion.WithKind("ClientScopeMapping")),
		opts...)
	return ctrl.NewControllerManagedBy(mgr).
		Named(controllerName).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&csv1beta1.ClientScopeMapping{}).
		Complete(r)
}

type connector struct{ kube client.Client }
type external struct {
	client clients.Client
	kube   client.Client
}

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*csv1beta1.ClientScopeMapping)
	if !ok {
		return nil, errors.New(errNotClientScopeMapping)
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
	_, span := tracing.StartSpan(ctx, "clientscopemapping.observe",
		tracing.SpanAttrs("ClientScopeMapping", mg.GetName(), "observe")...)
	defer span.End()

	cr, ok := mg.(*csv1beta1.ClientScopeMapping)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotClientScopeMapping)
	}

	// Delete has already released the mappings this resource owns. Report them
	// gone so the reconciler reaches RemoveFinalizer instead of re-running
	// Delete, which would otherwise re-apply the removals on every pass.
	if deletecomplete.Done(cr) {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}
	current, err := e.client.ListClientScopeMappings(ctx, cr.Spec.ForProvider.RealmId, cr.Spec.ForProvider.ClientId)
	if err != nil {
		return managed.ExternalObservation{}, err
	}
	cr.Status.SetConditions(xpv1.Available())

	// This set is additive: the resource owns the scopes it declares and no
	// others, so several resources may share one client. Entries in current that
	// this resource does not own - a sibling's, or added in Keycloak directly -
	// neither make it out of date nor get removed.
	declared := declaredEntries(cr.Spec.ForProvider.Scopes)
	owned := mappingreconcile.Prune(ownedEntries(cr.Status.AppliedScopes), currentEntries(current))
	cr.Status.AppliedScopes = fromEntries(owned)

	// Out of date when a declared scope is missing, and equally when a scope this
	// resource owns is no longer declared. Checking only the first would mean a
	// scope dropped from the spec is never taken off the client: the resource
	// would report itself settled while still holding the scope, because Update
	// is never reached.
	_, stale := mappingreconcile.Plan(declared, owned, currentEntries(current))

	return managed.ExternalObservation{
		ResourceExists: true,
		ResourceUpToDate: mappingreconcile.UpToDate(declared, currentEntries(current)) &&
			len(stale) == 0,
	}, nil
}

func (e *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	_, span := tracing.StartSpan(ctx, "clientscopemapping.create",
		tracing.SpanAttrs("ClientScopeMapping", mg.GetName(), "create")...)
	defer span.End()

	cr, ok := mg.(*csv1beta1.ClientScopeMapping)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotClientScopeMapping)
	}
	scopes := toRoleRepresentations(declaredEntries(cr.Spec.ForProvider.Scopes))
	if err := e.client.AddClientScopeMappings(ctx, cr.Spec.ForProvider.RealmId, cr.Spec.ForProvider.ClientId, scopes); err != nil {
		return managed.ExternalCreation{}, err
	}
	cr.Status.SetConditions(xpv1.Creating())

	// Record ownership as soon as the write lands, so a later pass can tell
	// these scopes from a sibling's and a Delete can remove exactly them.
	cr.Status.AppliedScopes = cr.Spec.ForProvider.Scopes
	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	_, span := tracing.StartSpan(ctx, "clientscopemapping.update",
		tracing.SpanAttrs("ClientScopeMapping", mg.GetName(), "update")...)
	defer span.End()

	cr, ok := mg.(*csv1beta1.ClientScopeMapping)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotClientScopeMapping)
	}
	current, err := e.client.ListClientScopeMappings(ctx, cr.Spec.ForProvider.RealmId, cr.Spec.ForProvider.ClientId)
	if err != nil {
		return managed.ExternalUpdate{}, err
	}
	declared := declaredEntries(cr.Spec.ForProvider.Scopes)
	owned := ownedEntries(cr.Status.AppliedScopes)

	// Additions are declared scopes the client is missing. Removals are scopes
	// this resource applied that the spec no longer declares - and only those. A
	// scope belonging to a sibling resource is absent from owned, so it is never
	// taken.
	add, remove := mappingreconcile.Plan(declared, owned, currentEntries(current))
	if len(add) > 0 {
		if err := e.client.AddClientScopeMappings(ctx, cr.Spec.ForProvider.RealmId, cr.Spec.ForProvider.ClientId, toRoleRepresentations(add)); err != nil {
			return managed.ExternalUpdate{}, err
		}
	}
	if len(remove) > 0 {
		if err := e.client.RemoveClientScopeMappings(ctx, cr.Spec.ForProvider.RealmId, cr.Spec.ForProvider.ClientId, toRoleRepresentations(remove)); err != nil {
			return managed.ExternalUpdate{}, err
		}
	}

	// Ownership is the declared set from here, pruned against what the client
	// actually has, so a failed write is not recorded as applied.
	cr.Status.AppliedScopes = fromEntries(mappingreconcile.Adopt(declared, append(currentEntries(current), add...)))
	return managed.ExternalUpdate{}, nil
}

func (e *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	_, span := tracing.StartSpan(ctx, "clientscopemapping.delete",
		tracing.SpanAttrs("ClientScopeMapping", mg.GetName(), "delete")...)
	defer span.End()

	cr, ok := mg.(*csv1beta1.ClientScopeMapping)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errNotClientScopeMapping)
	}

	// Already released. Removing again would strip mappings a sibling resource
	// may have since declared, so Delete must be safe to call on its own.
	if deletecomplete.Done(cr) {
		return managed.ExternalDelete{}, nil
	}
	current, err := e.client.ListClientScopeMappings(ctx, cr.Spec.ForProvider.RealmId, cr.Spec.ForProvider.ClientId)
	if err != nil && !strings.Contains(err.Error(), "404") {
		return managed.ExternalDelete{}, err
	}
	// Remove only the scopes this resource owns and that are still present.
	// Removing every scope on the client would strip a sibling resource's
	// entries as well, which is the failure additive ownership exists to prevent.
	release := mappingreconcile.Release(ownedEntries(cr.Status.AppliedScopes), currentEntries(current))
	if len(release) > 0 {
		if err := e.client.RemoveClientScopeMappings(ctx, cr.Spec.ForProvider.RealmId, cr.Spec.ForProvider.ClientId, toRoleRepresentations(release)); err != nil {
			return managed.ExternalDelete{}, err
		}
	}
	cr.Status.SetConditions(xpv1.Deleting())
	return managed.ExternalDelete{}, deletecomplete.Mark(ctx, e.kube, cr)
}

// declaredEntries projects the spec's scopes into the shared reconciliation type.
func declaredEntries(scopes []csv1beta1.ScopeMapping) []mappingreconcile.Entry {
	return mappingreconcile.Convert(scopes, func(s csv1beta1.ScopeMapping) mappingreconcile.Entry {
		return mappingreconcile.Entry{ID: s.Id, Name: s.Name}
	})
}

// ownedEntries projects the scopes this resource last applied.
func ownedEntries(applied []csv1beta1.ScopeMapping) []mappingreconcile.Entry {
	return mappingreconcile.Convert(applied, func(s csv1beta1.ScopeMapping) mappingreconcile.Entry {
		return mappingreconcile.Entry{ID: s.Id, Name: s.Name}
	})
}

// currentEntries projects the scopes Keycloak currently has on the client.
func currentEntries(current []clients.RoleRepresentation) []mappingreconcile.Entry {
	return mappingreconcile.Convert(current, func(r clients.RoleRepresentation) mappingreconcile.Entry {
		return mappingreconcile.Entry{ID: r.ID, Name: r.Name}
	})
}

func fromEntries(entries []mappingreconcile.Entry) []csv1beta1.ScopeMapping {
	out := make([]csv1beta1.ScopeMapping, 0, len(entries))
	for _, e := range entries {
		out = append(out, csv1beta1.ScopeMapping{Id: e.ID, Name: e.Name})
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
