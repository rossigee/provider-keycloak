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

package clientscope

import (
	"context"

	xpcontroller "github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/pkg/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	scopesv1alpha1 "github.com/rossigee/provider-keycloak/apis/scopes/v1alpha1"
	"github.com/rossigee/provider-keycloak/apis/v1beta1"
	"github.com/rossigee/provider-keycloak/internal/clients"
)

const (
	errNotClientScope     = "managed resource is not a ClientScope"
	errGetProviderConfig  = "cannot get ProviderConfig"
	errProviderNotReady   = "provider is not ready"
	controllerName        = "clientscopes.scopes.keycloak.crossplane.io"
)

func Setup(mgr ctrl.Manager, o xpcontroller.Options) error {
	r := managed.NewReconciler(mgr,
		resource.ManagedKind(scopesv1alpha1.SchemeGroupVersion.WithKind("ClientScope")),
		managed.WithExternalConnector(&connector{kube: mgr.GetClient()}),
		managed.WithLogger(o.Logger.WithValues("controller", "ClientScope")),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorder(controllerName))),
	)
	return ctrl.NewControllerManagedBy(mgr).
		Named(controllerName).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&scopesv1alpha1.ClientScope{}).
		Complete(r)
}

type connector struct{ kube client.Client }
type external struct{ client clients.Client }

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*scopesv1alpha1.ClientScope)
	if !ok {
		return nil, errors.New(errNotClientScope)
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
	cr, ok := mg.(*scopesv1alpha1.ClientScope)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotClientScope)
	}

	scope, err := e.client.GetClientScope(ctx, cr.Spec.ForProvider.RealmId, cr.Spec.ForProvider.Name)
	if err != nil {
		return managed.ExternalObservation{}, err
	}

	if scope == nil {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	cr.Status.SetConditions(xpv1.Available())
	return managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true}, nil
}

func (e *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*scopesv1alpha1.ClientScope)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotClientScope)
	}

	scope := clients.ClientScopeRepresentation{
		Name:                cr.Spec.ForProvider.Name,
		Description:         "",
		Protocol:            "openid-connect",
		IncludeInTokenScope: true,
	}

	if cr.Spec.ForProvider.Description != nil {
		scope.Description = *cr.Spec.ForProvider.Description
	}
	if cr.Spec.ForProvider.Protocol != nil {
		scope.Protocol = *cr.Spec.ForProvider.Protocol
	}
	if cr.Spec.ForProvider.IncludeInTokenScope != nil {
		scope.IncludeInTokenScope = *cr.Spec.ForProvider.IncludeInTokenScope
	}

	if err := e.client.CreateClientScope(ctx, cr.Spec.ForProvider.RealmId, scope); err != nil {
		return managed.ExternalCreation{}, err
	}

	cr.Status.SetConditions(xpv1.Creating())
	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*scopesv1alpha1.ClientScope)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotClientScope)
	}

	scope, err := e.client.GetClientScope(ctx, cr.Spec.ForProvider.RealmId, cr.Spec.ForProvider.Name)
	if err != nil {
		return managed.ExternalUpdate{}, err
	}

	if scope == nil {
		return managed.ExternalUpdate{}, errors.New("client scope not found")
	}

	scope.Description = ""
	if cr.Spec.ForProvider.Description != nil {
		scope.Description = *cr.Spec.ForProvider.Description
	}

	if err := e.client.UpdateClientScope(ctx, cr.Spec.ForProvider.RealmId, *scope); err != nil {
		return managed.ExternalUpdate{}, err
	}

	return managed.ExternalUpdate{}, nil
}

func (e *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(*scopesv1alpha1.ClientScope)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errNotClientScope)
	}

	if err := e.client.DeleteClientScope(ctx, cr.Spec.ForProvider.RealmId, cr.Spec.ForProvider.Name); err != nil {
		return managed.ExternalDelete{}, err
	}

	cr.Status.SetConditions(xpv1.Deleting())
	return managed.ExternalDelete{}, nil
}
