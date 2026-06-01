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

package realm

import (
	"context"
	"strings"

	xpcontroller "github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	realmv1alpha1 "github.com/rossigee/provider-keycloak/apis/realm/v1alpha1"
	"github.com/rossigee/provider-keycloak/apis/v1beta1"
	"github.com/rossigee/provider-keycloak/internal/clients"
)

const (
	errNotRealm          = "managed resource is not a Realm"
	errGetProviderConfig = "cannot get ProviderConfig"
	errProviderNotReady  = "provider is not ready"
	errGetRealm          = "cannot get Keycloak realm"
	errCreateRealm       = "cannot create Keycloak realm"
	errUpdateRealm       = "cannot update Keycloak realm"
	errDeleteRealm       = "cannot delete Keycloak realm"

	controllerName = "realms.realm.keycloak.crossplane.io"
)

// Setup registers the Realm controller.
func Setup(mgr ctrl.Manager, o xpcontroller.Options) error {
	r := managed.NewReconciler(mgr,
		resource.ManagedKind(realmv1alpha1.SchemeGroupVersion.WithKind("Realm")),
		managed.WithExternalConnector(&connector{kube: mgr.GetClient()}),
		managed.WithLogger(o.Logger.WithValues("controller", "Realm")),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorder(controllerName))),
	)
	return ctrl.NewControllerManagedBy(mgr).
		Named(controllerName).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&realmv1alpha1.Realm{}).
		Complete(r)
}

type connector struct{ kube client.Client }
type external struct{ client clients.Client }

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*realmv1alpha1.Realm)
	if !ok {
		return nil, errors.New(errNotRealm)
	}
	pcRef := cr.Spec.ProviderConfigReference
	if pcRef == nil {
		return nil, errors.New(errGetProviderConfig + ": providerConfigRef is required")
	}
	pc := &v1beta1.ProviderConfig{}
	if err := c.kube.Get(ctx, client.ObjectKey{Name: pcRef.Name}, pc); err != nil {
		return nil, errors.Wrap(err, errGetProviderConfig)
	}
	if pc.Status.GetCondition(xpv1.TypeReady).Status != corev1.ConditionTrue {
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
	cr, ok := mg.(*realmv1alpha1.Realm)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotRealm)
	}
	realm := cr.Spec.ForProvider.Realm
	r, err := e.client.GetRealm(ctx, realm)
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, errors.Wrap(err, errGetRealm)
	}
	if r == nil {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}
	cr.Status.SetConditions(xpv1.Available())
	return managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: realmUpToDate(&cr.Spec.ForProvider, r)}, nil
}

func (e *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*realmv1alpha1.Realm)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotRealm)
	}
	_, err := e.client.CreateRealm(ctx, realmParamsToRepresentation(&cr.Spec.ForProvider))
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreateRealm)
	}
	cr.Status.SetConditions(xpv1.Creating())
	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*realmv1alpha1.Realm)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotRealm)
	}
	if err := e.client.UpdateRealm(ctx, realmParamsToRepresentation(&cr.Spec.ForProvider)); err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateRealm)
	}
	return managed.ExternalUpdate{}, nil
}

func (e *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(*realmv1alpha1.Realm)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errNotRealm)
	}
	err := e.client.DeleteRealm(ctx, cr.Spec.ForProvider.Realm)
	if err != nil && !strings.Contains(err.Error(), "404") {
		return managed.ExternalDelete{}, errors.Wrap(err, errDeleteRealm)
	}
	cr.Status.SetConditions(xpv1.Deleting())
	return managed.ExternalDelete{}, nil
}

func realmParamsToRepresentation(p *realmv1alpha1.RealmParameters) *clients.Realm {
	r := &clients.Realm{Realm: p.Realm}
	if p.Enabled != nil {
		r.Enabled = *p.Enabled
	} else {
		r.Enabled = true
	}
	if p.DisplayName != nil {
		r.DisplayName = *p.DisplayName
	}
	if p.LoginWithEmailAllowed != nil {
		r.LoginWithEmailAllowed = *p.LoginWithEmailAllowed
	}
	if p.DuplicateEmailsAllowed != nil {
		r.DuplicateEmailsAllowed = *p.DuplicateEmailsAllowed
	}
	if p.EditUsernameAllowed != nil {
		r.EditUsernameAllowed = *p.EditUsernameAllowed
	}
	if p.ResetPasswordAllowed != nil {
		r.ResetPasswordAllowed = *p.ResetPasswordAllowed
	}
	return r
}

func realmUpToDate(desired *realmv1alpha1.RealmParameters, actual *clients.Realm) bool {
	if desired.Enabled != nil && *desired.Enabled != actual.Enabled {
		return false
	}
	if desired.DisplayName != nil && *desired.DisplayName != actual.DisplayName {
		return false
	}
	if desired.LoginWithEmailAllowed != nil && *desired.LoginWithEmailAllowed != actual.LoginWithEmailAllowed {
		return false
	}
	if desired.DuplicateEmailsAllowed != nil && *desired.DuplicateEmailsAllowed != actual.DuplicateEmailsAllowed {
		return false
	}
	return true
}
