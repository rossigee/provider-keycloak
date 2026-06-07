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

package authenticationflow

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

	authenticationflowv1alpha1 "github.com/rossigee/provider-keycloak/apis/authenticationflow/v1alpha1"
	"github.com/rossigee/provider-keycloak/apis/v1beta1"
	"github.com/rossigee/provider-keycloak/internal/clients"
)

const (
	errNotAuthenticationFlow = "managed resource is not an AuthenticationFlow"
	errGetProviderConfig     = "cannot get ProviderConfig"
	errProviderNotReady      = "provider is not ready"
	controllerName           = "authenticationflow.keycloak.crossplane.io"
)

func Setup(mgr ctrl.Manager, o xpcontroller.Options) error {
	r := managed.NewReconciler(mgr,
		resource.ManagedKind(authenticationflowv1alpha1.SchemeGroupVersion.WithKind("AuthenticationFlow")),
		managed.WithExternalConnector(&connector{kube: mgr.GetClient()}),
		managed.WithLogger(o.Logger.WithValues("controller", "AuthenticationFlow")),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorder(controllerName))),
	)
	return ctrl.NewControllerManagedBy(mgr).
		Named(controllerName).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&authenticationflowv1alpha1.AuthenticationFlow{}).
		Complete(r)
}

type connector struct{ kube client.Client }
type external struct{ client clients.Client }

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*authenticationflowv1alpha1.AuthenticationFlow)
	if !ok {
		return nil, errors.New(errNotAuthenticationFlow)
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
	cr, ok := mg.(*authenticationflowv1alpha1.AuthenticationFlow)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotAuthenticationFlow)
	}
	flow, err := e.client.GetAuthenticationFlow(ctx, cr.Spec.ForProvider.RealmId, cr.Spec.ForProvider.Alias)
	if err != nil {
		return managed.ExternalObservation{}, err
	}
	cr.Status.SetConditions(xpv1.Available())
	upToDate := isAuthenticationFlowUpToDate(&cr.Spec.ForProvider, flow)
	return managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: upToDate}, nil
}

func (e *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*authenticationflowv1alpha1.AuthenticationFlow)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotAuthenticationFlow)
	}
	flow := &clients.AuthenticationFlowRepresentation{
		Alias:       cr.Spec.ForProvider.Alias,
		Description: deref(cr.Spec.ForProvider.Description),
		ProviderId:  cr.Spec.ForProvider.ProviderId,
		BuiltIn:     derefBool(cr.Spec.ForProvider.BuiltIn),
		TopLevel:    derefBool(cr.Spec.ForProvider.TopLevel),
	}
	_, err := e.client.CreateAuthenticationFlow(ctx, cr.Spec.ForProvider.RealmId, flow)
	if err != nil {
		return managed.ExternalCreation{}, err
	}
	cr.Status.SetConditions(xpv1.Creating())
	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*authenticationflowv1alpha1.AuthenticationFlow)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotAuthenticationFlow)
	}
	flow := &clients.AuthenticationFlowRepresentation{
		Alias:       cr.Spec.ForProvider.Alias,
		Description: deref(cr.Spec.ForProvider.Description),
		ProviderId:  cr.Spec.ForProvider.ProviderId,
		BuiltIn:     derefBool(cr.Spec.ForProvider.BuiltIn),
		TopLevel:    derefBool(cr.Spec.ForProvider.TopLevel),
	}
	return managed.ExternalUpdate{}, e.client.UpdateAuthenticationFlow(ctx, cr.Spec.ForProvider.RealmId, cr.Spec.ForProvider.Alias, flow)
}

func (e *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(*authenticationflowv1alpha1.AuthenticationFlow)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errNotAuthenticationFlow)
	}
	if err := e.client.DeleteAuthenticationFlow(ctx, cr.Spec.ForProvider.RealmId, cr.Spec.ForProvider.Alias); err != nil {
		return managed.ExternalDelete{}, err
	}
	cr.Status.SetConditions(xpv1.Deleting())
	return managed.ExternalDelete{}, nil
}

func isAuthenticationFlowUpToDate(desired *authenticationflowv1alpha1.AuthenticationFlowParameters, current *clients.AuthenticationFlowRepresentation) bool {
	if desired.Alias != current.Alias {
		return false
	}
	if deref(desired.Description) != current.Description {
		return false
	}
	if desired.ProviderId != current.ProviderId {
		return false
	}
	if derefBool(desired.BuiltIn) != current.BuiltIn {
		return false
	}
	if derefBool(desired.TopLevel) != current.TopLevel {
		return false
	}
	return true
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func derefBool(b *bool) bool {
	if b == nil {
		return false
	}
	return *b
}
