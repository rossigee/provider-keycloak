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

package authorizationpolicy

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

	authorizationpolicyv1alpha1 "github.com/rossigee/provider-keycloak/apis/authorizationpolicy/v1alpha1"
	"github.com/rossigee/provider-keycloak/apis/v1beta1"
	"github.com/rossigee/provider-keycloak/internal/clients"
)

const (
	errNotAuthorizationPolicy = "managed resource is not an AuthorizationPolicy"
	errGetProviderConfig      = "cannot get ProviderConfig"
	errProviderNotReady       = "provider is not ready"
	controllerName            = "authorizationpolicy.keycloak.crossplane.io"
)

func Setup(mgr ctrl.Manager, o xpcontroller.Options) error {
	r := managed.NewReconciler(mgr,
		resource.ManagedKind(authorizationpolicyv1alpha1.SchemeGroupVersion.WithKind("AuthorizationPolicy")),
		managed.WithExternalConnector(&connector{kube: mgr.GetClient()}),
		managed.WithLogger(o.Logger.WithValues("controller", "AuthorizationPolicy")),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorder(controllerName))),
	)
	return ctrl.NewControllerManagedBy(mgr).
		Named(controllerName).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&authorizationpolicyv1alpha1.AuthorizationPolicy{}).
		Complete(r)
}

type connector struct{ kube client.Client }
type external struct{ client clients.Client }

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*authorizationpolicyv1alpha1.AuthorizationPolicy)
	if !ok {
		return nil, errors.New(errNotAuthorizationPolicy)
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
	cr, ok := mg.(*authorizationpolicyv1alpha1.AuthorizationPolicy)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotAuthorizationPolicy)
	}
	policy, err := e.client.GetAuthorizationPolicy(ctx, cr.Spec.ForProvider.RealmId, cr.Spec.ForProvider.ClientId, cr.GetAnnotations()["crossplane.io/external-name"])
	if err != nil {
		return managed.ExternalObservation{}, err
	}
	cr.Status.SetConditions(xpv1.Available())
	upToDate := isAuthorizationPolicyUpToDate(&cr.Spec.ForProvider, policy)
	return managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: upToDate}, nil
}

func (e *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*authorizationpolicyv1alpha1.AuthorizationPolicy)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotAuthorizationPolicy)
	}
	policy := &clients.AuthorizationPolicyRepresentation{
		Name:        cr.Spec.ForProvider.Name,
		Type:        cr.Spec.ForProvider.Type,
		Description: deref(cr.Spec.ForProvider.Description),
		Logic:       deref(cr.Spec.ForProvider.Logic),
		Config:      cr.Spec.ForProvider.Config,
	}
	id, err := e.client.CreateAuthorizationPolicy(ctx, cr.Spec.ForProvider.RealmId, cr.Spec.ForProvider.ClientId, policy)
	if err != nil {
		return managed.ExternalCreation{}, err
	}
	cr.Annotations["crossplane.io/external-name"] = id
	cr.Status.SetConditions(xpv1.Creating())
	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*authorizationpolicyv1alpha1.AuthorizationPolicy)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotAuthorizationPolicy)
	}
	policy := &clients.AuthorizationPolicyRepresentation{
		Name:        cr.Spec.ForProvider.Name,
		Type:        cr.Spec.ForProvider.Type,
		Description: deref(cr.Spec.ForProvider.Description),
		Logic:       deref(cr.Spec.ForProvider.Logic),
		Config:      cr.Spec.ForProvider.Config,
	}
	return managed.ExternalUpdate{}, e.client.UpdateAuthorizationPolicy(ctx, cr.Spec.ForProvider.RealmId, cr.Spec.ForProvider.ClientId, cr.GetAnnotations()["crossplane.io/external-name"], policy)
}

func (e *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(*authorizationpolicyv1alpha1.AuthorizationPolicy)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errNotAuthorizationPolicy)
	}
	if err := e.client.DeleteAuthorizationPolicy(ctx, cr.Spec.ForProvider.RealmId, cr.Spec.ForProvider.ClientId, cr.GetAnnotations()["crossplane.io/external-name"]); err != nil {
		return managed.ExternalDelete{}, err
	}
	cr.Status.SetConditions(xpv1.Deleting())
	return managed.ExternalDelete{}, nil
}

func isAuthorizationPolicyUpToDate(desired *authorizationpolicyv1alpha1.AuthorizationPolicyParameters, current *clients.AuthorizationPolicyRepresentation) bool {
	if desired.Name != current.Name {
		return false
	}
	if desired.Type != current.Type {
		return false
	}
	if deref(desired.Description) != current.Description {
		return false
	}
	if deref(desired.Logic) != current.Logic {
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
