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
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	csv1alpha1 "github.com/rossigee/provider-keycloak/apis/scopes/v1alpha1"
	"github.com/rossigee/provider-keycloak/apis/v1beta1"
	"github.com/rossigee/provider-keycloak/internal/clients"
	"github.com/rossigee/provider-keycloak/internal/tracing"
)

const (
	errNotClientScopeMapping = "managed resource is not a ClientScopeMapping"
	errGetProviderConfig     = "cannot get ProviderConfig"
	errProviderNotReady      = "provider is not ready"
	controllerName           = "clientscopemappings.scopes.keycloak.crossplane.io"
)

func Setup(mgr ctrl.Manager, o xpcontroller.Options) error {
	r := managed.NewReconciler(mgr,
		resource.ManagedKind(csv1alpha1.SchemeGroupVersion.WithKind("ClientScopeMapping")),
		managed.WithExternalConnector(&connector{kube: mgr.GetClient()}),
		managed.WithLogger(o.Logger.WithValues("controller", "ClientScopeMapping")),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorder(controllerName))),
	)
	return ctrl.NewControllerManagedBy(mgr).
		Named(controllerName).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&csv1alpha1.ClientScopeMapping{}).
		Complete(r)
}

type connector struct{ kube client.Client }
type external struct{ client clients.Client }

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*csv1alpha1.ClientScopeMapping)
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
	kc, err := clients.NewClient(ctx, pc, c.kube)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create Keycloak client")
	}
	return &external{client: kc}, nil
}

func (e *external) Disconnect(_ context.Context) error { return nil }

func (e *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	_, span := tracing.StartSpan(ctx, "clientscopemapping.observe",
		tracing.SpanAttrs("ClientScopeMapping", mg.GetName(), "observe")...)
	defer span.End()

	cr, ok := mg.(*csv1alpha1.ClientScopeMapping)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotClientScopeMapping)
	}
	current, err := e.client.ListClientScopeMappings(ctx, cr.Spec.ForProvider.RealmId, cr.Spec.ForProvider.ClientId)
	if err != nil {
		return managed.ExternalObservation{}, err
	}
	cr.Status.SetConditions(xpv1.Available())
	upToDate := scopesMatch(cr.Spec.ForProvider.Scopes, current)
	cr.Status.AppliedScopes = toScopeMappings(current)
	return managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: upToDate}, nil
}

func (e *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	_, span := tracing.StartSpan(ctx, "clientscopemapping.create",
		tracing.SpanAttrs("ClientScopeMapping", mg.GetName(), "create")...)
	defer span.End()

	cr, ok := mg.(*csv1alpha1.ClientScopeMapping)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotClientScopeMapping)
	}
	scopes := toRoleRepresentations(cr.Spec.ForProvider.Scopes)
	if err := e.client.AddClientScopeMappings(ctx, cr.Spec.ForProvider.RealmId, cr.Spec.ForProvider.ClientId, scopes); err != nil {
		return managed.ExternalCreation{}, err
	}
	cr.Status.SetConditions(xpv1.Creating())
	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	_, span := tracing.StartSpan(ctx, "clientscopemapping.update",
		tracing.SpanAttrs("ClientScopeMapping", mg.GetName(), "update")...)
	defer span.End()

	cr, ok := mg.(*csv1alpha1.ClientScopeMapping)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotClientScopeMapping)
	}
	current, err := e.client.ListClientScopeMappings(ctx, cr.Spec.ForProvider.RealmId, cr.Spec.ForProvider.ClientId)
	if err != nil {
		return managed.ExternalUpdate{}, err
	}
	desired := toRoleRepresentations(cr.Spec.ForProvider.Scopes)
	toAdd := scopeDiff(desired, current)
	toRemove := scopeDiff(current, desired)
	if len(toAdd) > 0 {
		if err := e.client.AddClientScopeMappings(ctx, cr.Spec.ForProvider.RealmId, cr.Spec.ForProvider.ClientId, toAdd); err != nil {
			return managed.ExternalUpdate{}, err
		}
	}
	if len(toRemove) > 0 {
		if err := e.client.RemoveClientScopeMappings(ctx, cr.Spec.ForProvider.RealmId, cr.Spec.ForProvider.ClientId, toRemove); err != nil {
			return managed.ExternalUpdate{}, err
		}
	}
	return managed.ExternalUpdate{}, nil
}

func (e *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	_, span := tracing.StartSpan(ctx, "clientscopemapping.delete",
		tracing.SpanAttrs("ClientScopeMapping", mg.GetName(), "delete")...)
	defer span.End()

	cr, ok := mg.(*csv1alpha1.ClientScopeMapping)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errNotClientScopeMapping)
	}
	current, err := e.client.ListClientScopeMappings(ctx, cr.Spec.ForProvider.RealmId, cr.Spec.ForProvider.ClientId)
	if err != nil && !strings.Contains(err.Error(), "404") {
		return managed.ExternalDelete{}, err
	}
	if len(current) > 0 {
		if err := e.client.RemoveClientScopeMappings(ctx, cr.Spec.ForProvider.RealmId, cr.Spec.ForProvider.ClientId, current); err != nil {
			return managed.ExternalDelete{}, err
		}
	}
	cr.Status.SetConditions(xpv1.Deleting())
	return managed.ExternalDelete{}, nil
}

func scopesMatch(desired []csv1alpha1.ScopeMapping, current []clients.RoleRepresentation) bool {
	if len(desired) != len(current) {
		return false
	}
	for _, d := range desired {
		found := false
		for _, c := range current {
			if (d.Id != "" && d.Id == c.ID) || (d.Name != "" && d.Name == c.Name) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func scopeDiff(desired, current []clients.RoleRepresentation) []clients.RoleRepresentation {
	var diff []clients.RoleRepresentation
	for _, d := range desired {
		found := false
		for _, c := range current {
			if (d.ID != "" && d.ID == c.ID) || (d.Name != "" && d.Name == c.Name) {
				found = true
				break
			}
		}
		if !found {
			diff = append(diff, d)
		}
	}
	return diff
}

func toRoleRepresentations(scopes []csv1alpha1.ScopeMapping) []clients.RoleRepresentation {
	result := make([]clients.RoleRepresentation, len(scopes))
	for i, s := range scopes {
		result[i] = clients.RoleRepresentation{ID: s.Id, Name: s.Name}
	}
	return result
}

func toScopeMappings(roles []clients.RoleRepresentation) []csv1alpha1.ScopeMapping {
	result := make([]csv1alpha1.ScopeMapping, len(roles))
	for i, r := range roles {
		result[i] = csv1alpha1.ScopeMapping{Id: r.ID, Name: r.Name}
	}
	return result
}
