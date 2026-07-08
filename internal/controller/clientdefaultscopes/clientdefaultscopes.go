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

package clientdefaultscopes

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

	openidclientv1alpha1 "github.com/rossigee/provider-keycloak/apis/openidclient/v1alpha1"
	"github.com/rossigee/provider-keycloak/apis/v1beta1"
	"github.com/rossigee/provider-keycloak/internal/clients"
	"github.com/rossigee/provider-keycloak/internal/tracing"
)

const (
	errNotClientDefaultScopes = "managed resource is not a ClientDefaultScopes"
	errGetProviderConfig      = "cannot get ProviderConfig"
	errProviderNotReady       = "provider is not ready"
	controllerName            = "clientdefaultscopes.client.keycloak.crossplane.io"
)

func Setup(mgr ctrl.Manager, o xpcontroller.Options) error {
	r := managed.NewReconciler(mgr,
		resource.ManagedKind(openidclientv1alpha1.SchemeGroupVersion.WithKind("ClientDefaultScopes")),
		managed.WithExternalConnector(&connector{kube: mgr.GetClient()}),
		managed.WithLogger(o.Logger.WithValues("controller", "ClientDefaultScopes")),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorder(controllerName))),
	)
	return ctrl.NewControllerManagedBy(mgr).
		Named(controllerName).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&openidclientv1alpha1.ClientDefaultScopes{}).
		Complete(r)
}

type connector struct{ kube client.Client }
type external struct{ client clients.Client }

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*openidclientv1alpha1.ClientDefaultScopes)
	if !ok {
		return nil, errors.New(errNotClientDefaultScopes)
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
	_, span := tracing.StartSpan(ctx, "clientdefaultscopes.observe",
		tracing.SpanAttrs("ClientDefaultScopes", mg.GetName(), "observe")...)
	defer span.End()

	cr, ok := mg.(*openidclientv1alpha1.ClientDefaultScopes)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotClientDefaultScopes)
	}
	current, err := e.client.ListClientDefaultScopes(ctx, deref(cr.Spec.ForProvider.RealmId), deref(cr.Spec.ForProvider.ClientId))
	if err != nil {
		return managed.ExternalObservation{}, err
	}
	cr.Status.SetConditions(xpv1.Available())
	upToDate := stringsScopeMatch(cr.Spec.ForProvider.DefaultScopes, current)
	return managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: upToDate}, nil
}

func (e *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	_, span := tracing.StartSpan(ctx, "clientdefaultscopes.create",
		tracing.SpanAttrs("ClientDefaultScopes", mg.GetName(), "create")...)
	defer span.End()

	cr, ok := mg.(*openidclientv1alpha1.ClientDefaultScopes)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotClientDefaultScopes)
	}
	scopes := stringSliceToScopes(cr.Spec.ForProvider.DefaultScopes)
	if err := e.client.AddClientDefaultScopes(ctx, deref(cr.Spec.ForProvider.RealmId), deref(cr.Spec.ForProvider.ClientId), scopes); err != nil {
		return managed.ExternalCreation{}, err
	}
	cr.Status.SetConditions(xpv1.Creating())
	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	_, span := tracing.StartSpan(ctx, "clientdefaultscopes.update",
		tracing.SpanAttrs("ClientDefaultScopes", mg.GetName(), "update")...)
	defer span.End()

	cr, ok := mg.(*openidclientv1alpha1.ClientDefaultScopes)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotClientDefaultScopes)
	}
	current, err := e.client.ListClientDefaultScopes(ctx, deref(cr.Spec.ForProvider.RealmId), deref(cr.Spec.ForProvider.ClientId))
	if err != nil {
		return managed.ExternalUpdate{}, err
	}
	desired := stringSliceToScopes(cr.Spec.ForProvider.DefaultScopes)
	toAdd := scopeDiff(desired, current)
	toRemove := scopeDiff(current, desired)
	if len(toAdd) > 0 {
		if err := e.client.AddClientDefaultScopes(ctx, deref(cr.Spec.ForProvider.RealmId), deref(cr.Spec.ForProvider.ClientId), toAdd); err != nil {
			return managed.ExternalUpdate{}, err
		}
	}
	if len(toRemove) > 0 {
		if err := e.client.RemoveClientDefaultScopes(ctx, deref(cr.Spec.ForProvider.RealmId), deref(cr.Spec.ForProvider.ClientId), toRemove); err != nil {
			return managed.ExternalUpdate{}, err
		}
	}
	return managed.ExternalUpdate{}, nil
}

func (e *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	_, span := tracing.StartSpan(ctx, "clientdefaultscopes.delete",
		tracing.SpanAttrs("ClientDefaultScopes", mg.GetName(), "delete")...)
	defer span.End()

	cr, ok := mg.(*openidclientv1alpha1.ClientDefaultScopes)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errNotClientDefaultScopes)
	}
	current, err := e.client.ListClientDefaultScopes(ctx, deref(cr.Spec.ForProvider.RealmId), deref(cr.Spec.ForProvider.ClientId))
	if err != nil && !strings.Contains(err.Error(), "404") {
		return managed.ExternalDelete{}, err
	}
	if len(current) > 0 {
		if err := e.client.RemoveClientDefaultScopes(ctx, deref(cr.Spec.ForProvider.RealmId), deref(cr.Spec.ForProvider.ClientId), current); err != nil {
			return managed.ExternalDelete{}, err
		}
	}
	cr.Status.SetConditions(xpv1.Deleting())
	return managed.ExternalDelete{}, nil
}

func stringsScopeMatch(desired []string, current []clients.ClientScopeRepresentation) bool {
	if len(desired) != len(current) {
		return false
	}
	for _, d := range desired {
		found := false
		for _, c := range current {
			if d == c.ID || d == c.Name {
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

func scopeDiff(desired, current []clients.ClientScopeRepresentation) []clients.ClientScopeRepresentation {
	var diff []clients.ClientScopeRepresentation
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

func stringSliceToScopes(scopes []string) []clients.ClientScopeRepresentation {
	result := make([]clients.ClientScopeRepresentation, len(scopes))
	for i, s := range scopes {
		result[i] = clients.ClientScopeRepresentation{ID: s}
	}
	return result
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
