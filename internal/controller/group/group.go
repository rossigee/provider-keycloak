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

package group

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

	groupv1alpha1 "github.com/rossigee/provider-keycloak/apis/group/v1alpha1"
	"github.com/rossigee/provider-keycloak/apis/v1beta1"
	"github.com/rossigee/provider-keycloak/internal/clients"
	"github.com/rossigee/provider-keycloak/internal/tracing"
)

const (
	errNotGroup          = "managed resource is not a Group"
	errGetProviderConfig = "cannot get ProviderConfig"
	errProviderNotReady  = "provider is not ready"
	errGetGroup          = "cannot get Keycloak group"
	errCreateGroup       = "cannot create Keycloak group"
	errUpdateGroup       = "cannot update Keycloak group"
	errDeleteGroup       = "cannot delete Keycloak group"

	controllerName = "groups.group.keycloak.crossplane.io"
)

// Setup registers the Group controller.
func Setup(mgr ctrl.Manager, o xpcontroller.Options) error {
	r := managed.NewReconciler(mgr,
		resource.ManagedKind(groupv1alpha1.SchemeGroupVersion.WithKind("Group")),
		managed.WithExternalConnector(&connector{kube: mgr.GetClient()}),
		managed.WithLogger(o.Logger.WithValues("controller", "Group")),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorder(controllerName))),
	)
	return ctrl.NewControllerManagedBy(mgr).
		Named(controllerName).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&groupv1alpha1.Group{}).
		Complete(r)
}

type connector struct{ kube client.Client }
type external struct{ client clients.Client }

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*groupv1alpha1.Group)
	if !ok {
		return nil, errors.New(errNotGroup)
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
	_, span := tracing.StartSpan(ctx, "group.observe",
		tracing.SpanAttrs("Group", mg.GetName(), "observe")...)
	defer span.End()

	cr, ok := mg.(*groupv1alpha1.Group)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotGroup)
	}
	realmId, err := realmID(cr)
	if err != nil {
		return managed.ExternalObservation{}, err
	}
	groups, err := e.client.SearchGroups(ctx, realmId, cr.Spec.ForProvider.Name)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errGetGroup)
	}
	for i := range groups {
		if groups[i].Name == cr.Spec.ForProvider.Name {
			cr.Status.SetConditions(xpv1.Available())
			return managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true}, nil
		}
	}
	return managed.ExternalObservation{ResourceExists: false}, nil
}

func (e *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	_, span := tracing.StartSpan(ctx, "group.create",
		tracing.SpanAttrs("Group", mg.GetName(), "create")...)
	defer span.End()

	cr, ok := mg.(*groupv1alpha1.Group)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotGroup)
	}
	realmId, err := realmID(cr)
	if err != nil {
		return managed.ExternalCreation{}, err
	}
	rep := &clients.GroupRepresentation{Name: cr.Spec.ForProvider.Name}
	if cr.Spec.ForProvider.Attributes != nil {
		rep.Attributes = cr.Spec.ForProvider.Attributes
	}
	_, err = e.client.CreateGroup(ctx, realmId, rep)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreateGroup)
	}
	cr.Status.SetConditions(xpv1.Creating())
	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	_, span := tracing.StartSpan(ctx, "group.update",
		tracing.SpanAttrs("Group", mg.GetName(), "update")...)
	defer span.End()

	cr, ok := mg.(*groupv1alpha1.Group)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotGroup)
	}
	realmId, err := realmID(cr)
	if err != nil {
		return managed.ExternalUpdate{}, err
	}
	groups, err := e.client.SearchGroups(ctx, realmId, cr.Spec.ForProvider.Name)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errGetGroup)
	}
	for i := range groups {
		if groups[i].Name == cr.Spec.ForProvider.Name {
			rep := &clients.GroupRepresentation{
				ID:         groups[i].ID,
				Name:       cr.Spec.ForProvider.Name,
				Attributes: cr.Spec.ForProvider.Attributes,
			}
			if err := e.client.UpdateGroup(ctx, realmId, rep); err != nil {
				return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateGroup)
			}
			return managed.ExternalUpdate{}, nil
		}
	}
	return managed.ExternalUpdate{}, errors.New("group not found for update")
}

func (e *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	_, span := tracing.StartSpan(ctx, "group.delete",
		tracing.SpanAttrs("Group", mg.GetName(), "delete")...)
	defer span.End()

	cr, ok := mg.(*groupv1alpha1.Group)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errNotGroup)
	}
	realmId, err := realmID(cr)
	if err != nil {
		return managed.ExternalDelete{}, err
	}
	groups, err := e.client.SearchGroups(ctx, realmId, cr.Spec.ForProvider.Name)
	if err != nil {
		return managed.ExternalDelete{}, errors.Wrap(err, errGetGroup)
	}
	for i := range groups {
		if groups[i].Name == cr.Spec.ForProvider.Name {
			err := e.client.DeleteGroup(ctx, realmId, groups[i].ID)
			if err != nil && !strings.Contains(err.Error(), "404") {
				return managed.ExternalDelete{}, errors.Wrap(err, errDeleteGroup)
			}
			cr.Status.SetConditions(xpv1.Deleting())
			return managed.ExternalDelete{}, nil
		}
	}
	return managed.ExternalDelete{}, nil
}

func realmID(cr *groupv1alpha1.Group) (string, error) {
	if cr.Spec.ForProvider.RealmId == nil || *cr.Spec.ForProvider.RealmId == "" {
		return "", errors.New("realmId is required")
	}
	return *cr.Spec.ForProvider.RealmId, nil
}
