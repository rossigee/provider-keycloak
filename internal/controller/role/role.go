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

package role

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

	rolev1alpha1 "github.com/rossigee/provider-keycloak/apis/role/v1alpha1"
	"github.com/rossigee/provider-keycloak/apis/v1beta1"
	"github.com/rossigee/provider-keycloak/internal/clients"
)

const (
	errNotRole           = "managed resource is not a Role"
	errGetProviderConfig = "cannot get ProviderConfig"
	errProviderNotReady  = "provider is not ready"
	errGetRole           = "cannot get Keycloak role"
	errCreateRole        = "cannot create Keycloak role"
	errUpdateRole        = "cannot update Keycloak role"
	errDeleteRole        = "cannot delete Keycloak role"

	controllerName = "roles.role.keycloak.crossplane.io"
)

// Setup registers the Role controller.
func Setup(mgr ctrl.Manager, o xpcontroller.Options) error {
	r := managed.NewReconciler(mgr,
		resource.ManagedKind(rolev1alpha1.SchemeGroupVersion.WithKind("Role")),
		managed.WithExternalConnector(&connector{kube: mgr.GetClient()}),
		managed.WithLogger(o.Logger.WithValues("controller", "Role")),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorder(controllerName))),
	)
	return ctrl.NewControllerManagedBy(mgr).
		Named(controllerName).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&rolev1alpha1.Role{}).
		Complete(r)
}

type connector struct{ kube client.Client }
type external struct{ client clients.Client }

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*rolev1alpha1.Role)
	if !ok {
		return nil, errors.New(errNotRole)
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
	cr, ok := mg.(*rolev1alpha1.Role)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotRole)
	}
	realmId, err := realmID(cr)
	if err != nil {
		return managed.ExternalObservation{}, err
	}
	r, err := e.client.GetRealmRole(ctx, realmId, cr.Spec.ForProvider.Name)
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, errors.Wrap(err, errGetRole)
	}
	if r == nil {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}
	cr.Status.SetConditions(xpv1.Available())
	upToDate := cr.Spec.ForProvider.Description == nil || *cr.Spec.ForProvider.Description == r.Description
	return managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: upToDate}, nil
}

func (e *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*rolev1alpha1.Role)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotRole)
	}
	realmId, err := realmID(cr)
	if err != nil {
		return managed.ExternalCreation{}, err
	}
	rep := roleParamsToRepresentation(&cr.Spec.ForProvider)
	if err := e.client.CreateRealmRole(ctx, realmId, rep); err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreateRole)
	}
	cr.Status.SetConditions(xpv1.Creating())
	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*rolev1alpha1.Role)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotRole)
	}
	realmId, err := realmID(cr)
	if err != nil {
		return managed.ExternalUpdate{}, err
	}
	rep := roleParamsToRepresentation(&cr.Spec.ForProvider)
	if err := e.client.UpdateRealmRole(ctx, realmId, cr.Spec.ForProvider.Name, rep); err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateRole)
	}
	return managed.ExternalUpdate{}, nil
}

func (e *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(*rolev1alpha1.Role)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errNotRole)
	}
	realmId, err := realmID(cr)
	if err != nil {
		return managed.ExternalDelete{}, err
	}
	err = e.client.DeleteRealmRole(ctx, realmId, cr.Spec.ForProvider.Name)
	if err != nil && !strings.Contains(err.Error(), "404") {
		return managed.ExternalDelete{}, errors.Wrap(err, errDeleteRole)
	}
	cr.Status.SetConditions(xpv1.Deleting())
	return managed.ExternalDelete{}, nil
}

func realmID(cr *rolev1alpha1.Role) (string, error) {
	if cr.Spec.ForProvider.RealmId == nil || *cr.Spec.ForProvider.RealmId == "" {
		return "", errors.New("realmId is required")
	}
	return *cr.Spec.ForProvider.RealmId, nil
}

func roleParamsToRepresentation(p *rolev1alpha1.RoleParameters) *clients.RoleRepresentation {
	r := &clients.RoleRepresentation{Name: p.Name}
	if p.Description != nil {
		r.Description = *p.Description
	}
	return r
}
