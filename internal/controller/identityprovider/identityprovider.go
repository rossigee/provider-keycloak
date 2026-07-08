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

package identityprovider

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

	identityproviderv1alpha1 "github.com/rossigee/provider-keycloak/apis/identityprovider/v1alpha1"
	"github.com/rossigee/provider-keycloak/apis/v1beta1"
	"github.com/rossigee/provider-keycloak/internal/clients"
	"github.com/rossigee/provider-keycloak/internal/tracing"
)

const (
	errNotIdentityProvider = "managed resource is not an IdentityProvider"
	errGetProviderConfig   = "cannot get ProviderConfig"
	errProviderNotReady    = "provider is not ready"
	controllerName         = "identityprovider.keycloak.crossplane.io"
)

func Setup(mgr ctrl.Manager, o xpcontroller.Options) error {
	r := managed.NewReconciler(mgr,
		resource.ManagedKind(identityproviderv1alpha1.SchemeGroupVersion.WithKind("IdentityProvider")),
		managed.WithExternalConnector(&connector{kube: mgr.GetClient()}),
		managed.WithLogger(o.Logger.WithValues("controller", "IdentityProvider")),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorder(controllerName))),
	)
	return ctrl.NewControllerManagedBy(mgr).
		Named(controllerName).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&identityproviderv1alpha1.IdentityProvider{}).
		Complete(r)
}

type connector struct{ kube client.Client }
type external struct{ client clients.Client }

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*identityproviderv1alpha1.IdentityProvider)
	if !ok {
		return nil, errors.New(errNotIdentityProvider)
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
	_, span := tracing.StartSpan(ctx, "identityprovider.observe",
		tracing.SpanAttrs("IdentityProvider", mg.GetName(), "observe")...)
	defer span.End()

	cr, ok := mg.(*identityproviderv1alpha1.IdentityProvider)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotIdentityProvider)
	}
	idp, err := e.client.GetIdentityProvider(ctx, cr.Spec.ForProvider.RealmId, cr.Spec.ForProvider.Alias)
	if err != nil {
		return managed.ExternalObservation{}, err
	}
	cr.Status.SetConditions(xpv1.Available())
	upToDate := isIdentityProviderUpToDate(&cr.Spec.ForProvider, idp)
	return managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: upToDate}, nil
}

func (e *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	_, span := tracing.StartSpan(ctx, "identityprovider.create",
		tracing.SpanAttrs("IdentityProvider", mg.GetName(), "create")...)
	defer span.End()

	cr, ok := mg.(*identityproviderv1alpha1.IdentityProvider)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotIdentityProvider)
	}
	idp := &clients.IdentityProviderRepresentation{
		Alias:                     cr.Spec.ForProvider.Alias,
		DisplayName:               deref(cr.Spec.ForProvider.DisplayName),
		ProviderId:                cr.Spec.ForProvider.ProviderId,
		Enabled:                   derefBool(cr.Spec.ForProvider.Enabled),
		TrustEmail:                derefBool(cr.Spec.ForProvider.TrustEmail),
		FirstBrokerLoginFlowAlias: deref(cr.Spec.ForProvider.FirstBrokerLoginFlowAlias),
		PostBrokerLoginFlowAlias:  deref(cr.Spec.ForProvider.PostBrokerLoginFlowAlias),
		Config:                    cr.Spec.ForProvider.Config,
	}
	_, err := e.client.CreateIdentityProvider(ctx, cr.Spec.ForProvider.RealmId, idp)
	if err != nil {
		return managed.ExternalCreation{}, err
	}
	cr.Status.SetConditions(xpv1.Creating())
	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	_, span := tracing.StartSpan(ctx, "identityprovider.update",
		tracing.SpanAttrs("IdentityProvider", mg.GetName(), "update")...)
	defer span.End()

	cr, ok := mg.(*identityproviderv1alpha1.IdentityProvider)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotIdentityProvider)
	}
	idp := &clients.IdentityProviderRepresentation{
		Alias:                     cr.Spec.ForProvider.Alias,
		DisplayName:               deref(cr.Spec.ForProvider.DisplayName),
		ProviderId:                cr.Spec.ForProvider.ProviderId,
		Enabled:                   derefBool(cr.Spec.ForProvider.Enabled),
		TrustEmail:                derefBool(cr.Spec.ForProvider.TrustEmail),
		FirstBrokerLoginFlowAlias: deref(cr.Spec.ForProvider.FirstBrokerLoginFlowAlias),
		PostBrokerLoginFlowAlias:  deref(cr.Spec.ForProvider.PostBrokerLoginFlowAlias),
		Config:                    cr.Spec.ForProvider.Config,
	}
	return managed.ExternalUpdate{}, e.client.UpdateIdentityProvider(ctx, cr.Spec.ForProvider.RealmId, cr.Spec.ForProvider.Alias, idp)
}

func (e *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	_, span := tracing.StartSpan(ctx, "identityprovider.delete",
		tracing.SpanAttrs("IdentityProvider", mg.GetName(), "delete")...)
	defer span.End()

	cr, ok := mg.(*identityproviderv1alpha1.IdentityProvider)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errNotIdentityProvider)
	}
	if err := e.client.DeleteIdentityProvider(ctx, cr.Spec.ForProvider.RealmId, cr.Spec.ForProvider.Alias); err != nil {
		return managed.ExternalDelete{}, err
	}
	cr.Status.SetConditions(xpv1.Deleting())
	return managed.ExternalDelete{}, nil
}

func isIdentityProviderUpToDate(desired *identityproviderv1alpha1.IdentityProviderParameters, current *clients.IdentityProviderRepresentation) bool {
	if desired.Alias != current.Alias {
		return false
	}
	if deref(desired.DisplayName) != current.DisplayName {
		return false
	}
	if desired.ProviderId != current.ProviderId {
		return false
	}
	if derefBool(desired.Enabled) != current.Enabled {
		return false
	}
	if derefBool(desired.TrustEmail) != current.TrustEmail {
		return false
	}
	if deref(desired.FirstBrokerLoginFlowAlias) != current.FirstBrokerLoginFlowAlias {
		return false
	}
	if deref(desired.PostBrokerLoginFlowAlias) != current.PostBrokerLoginFlowAlias {
		return false
	}
	if !configMapEqual(desired.Config, current.Config) {
		return false
	}
	return true
}

func configMapEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
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
