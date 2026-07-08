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

package user

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

	userv1alpha1 "github.com/rossigee/provider-keycloak/apis/user/v1alpha1"
	"github.com/rossigee/provider-keycloak/apis/v1beta1"
	"github.com/rossigee/provider-keycloak/internal/clients"
	"github.com/rossigee/provider-keycloak/internal/tracing"
)

const (
	errNotUser           = "managed resource is not a User"
	errGetProviderConfig = "cannot get ProviderConfig"
	errProviderNotReady  = "provider is not ready"
	errGetUser           = "cannot get Keycloak user"
	errCreateUser        = "cannot create Keycloak user"
	errUpdateUser        = "cannot update Keycloak user"
	errDeleteUser        = "cannot delete Keycloak user"

	controllerName = "users.user.keycloak.crossplane.io"
)

// Setup registers the User controller.
func Setup(mgr ctrl.Manager, o xpcontroller.Options) error {
	r := managed.NewReconciler(mgr,
		resource.ManagedKind(userv1alpha1.SchemeGroupVersion.WithKind("User")),
		managed.WithExternalConnector(&connector{kube: mgr.GetClient()}),
		managed.WithLogger(o.Logger.WithValues("controller", "User")),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorder(controllerName))),
	)
	return ctrl.NewControllerManagedBy(mgr).
		Named(controllerName).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&userv1alpha1.User{}).
		Complete(r)
}

type connector struct{ kube client.Client }
type external struct{ client clients.Client }

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*userv1alpha1.User)
	if !ok {
		return nil, errors.New(errNotUser)
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
	_, span := tracing.StartSpan(ctx, "user.observe",
		tracing.SpanAttrs("User", mg.GetName(), "observe")...)
	defer span.End()

	cr, ok := mg.(*userv1alpha1.User)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotUser)
	}
	realmId, err := realmID(cr)
	if err != nil {
		return managed.ExternalObservation{}, err
	}
	u, err := e.client.GetUser(ctx, realmId, cr.Spec.ForProvider.Username)
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, errors.Wrap(err, errGetUser)
	}
	if u == nil {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}
	cr.Status.SetConditions(xpv1.Available())
	return managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: userUpToDate(&cr.Spec.ForProvider, u)}, nil
}

func (e *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	_, span := tracing.StartSpan(ctx, "user.create",
		tracing.SpanAttrs("User", mg.GetName(), "create")...)
	defer span.End()

	cr, ok := mg.(*userv1alpha1.User)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotUser)
	}
	realmId, err := realmID(cr)
	if err != nil {
		return managed.ExternalCreation{}, err
	}
	_, err = e.client.CreateUser(ctx, realmId, userParamsToRepresentation(&cr.Spec.ForProvider))
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreateUser)
	}
	cr.Status.SetConditions(xpv1.Creating())
	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	_, span := tracing.StartSpan(ctx, "user.update",
		tracing.SpanAttrs("User", mg.GetName(), "update")...)
	defer span.End()

	cr, ok := mg.(*userv1alpha1.User)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotUser)
	}
	realmId, err := realmID(cr)
	if err != nil {
		return managed.ExternalUpdate{}, err
	}
	existing, err := e.client.GetUser(ctx, realmId, cr.Spec.ForProvider.Username)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errGetUser)
	}
	if existing == nil {
		return managed.ExternalUpdate{}, errors.New("user not found for update")
	}
	rep := userParamsToRepresentation(&cr.Spec.ForProvider)
	rep.ID = existing.ID
	if err := e.client.UpdateUser(ctx, realmId, rep); err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateUser)
	}
	return managed.ExternalUpdate{}, nil
}

func (e *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	_, span := tracing.StartSpan(ctx, "user.delete",
		tracing.SpanAttrs("User", mg.GetName(), "delete")...)
	defer span.End()

	cr, ok := mg.(*userv1alpha1.User)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errNotUser)
	}
	realmId, err := realmID(cr)
	if err != nil {
		return managed.ExternalDelete{}, err
	}
	existing, err := e.client.GetUser(ctx, realmId, cr.Spec.ForProvider.Username)
	if err != nil {
		return managed.ExternalDelete{}, errors.Wrap(err, errGetUser)
	}
	if existing == nil {
		return managed.ExternalDelete{}, nil
	}
	err = e.client.DeleteUser(ctx, realmId, existing.ID)
	if err != nil && !strings.Contains(err.Error(), "404") {
		return managed.ExternalDelete{}, errors.Wrap(err, errDeleteUser)
	}
	cr.Status.SetConditions(xpv1.Deleting())
	return managed.ExternalDelete{}, nil
}

func realmID(cr *userv1alpha1.User) (string, error) {
	if cr.Spec.ForProvider.RealmId == nil || *cr.Spec.ForProvider.RealmId == "" {
		return "", errors.New("realmId is required")
	}
	return *cr.Spec.ForProvider.RealmId, nil
}

func userParamsToRepresentation(p *userv1alpha1.UserParameters) *clients.UserRepresentation {
	u := &clients.UserRepresentation{Username: p.Username}
	if p.Email != nil {
		u.Email = *p.Email
	}
	if p.EmailVerified != nil {
		u.EmailVerified = *p.EmailVerified
	}
	u.Enabled = true
	if p.Enabled != nil {
		u.Enabled = *p.Enabled
	}
	if p.FirstName != nil {
		u.FirstName = *p.FirstName
	}
	if p.LastName != nil {
		u.LastName = *p.LastName
	}
	return u
}

func userUpToDate(desired *userv1alpha1.UserParameters, actual *clients.UserRepresentation) bool {
	return userFlagsUpToDate(desired, actual) && userDetailsUpToDate(desired, actual)
}

func userFlagsUpToDate(desired *userv1alpha1.UserParameters, actual *clients.UserRepresentation) bool {
	if desired.Enabled != nil && *desired.Enabled != actual.Enabled {
		return false
	}
	if desired.EmailVerified != nil && *desired.EmailVerified != actual.EmailVerified {
		return false
	}
	return true
}

func userDetailsUpToDate(desired *userv1alpha1.UserParameters, actual *clients.UserRepresentation) bool {
	if desired.Email != nil && *desired.Email != actual.Email {
		return false
	}
	if desired.FirstName != nil && *desired.FirstName != actual.FirstName {
		return false
	}
	if desired.LastName != nil && *desired.LastName != actual.LastName {
		return false
	}
	return true
}
