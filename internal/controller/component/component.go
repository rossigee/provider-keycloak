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

package component

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

	compv1alpha1 "github.com/rossigee/provider-keycloak/apis/component/v1alpha1"
	"github.com/rossigee/provider-keycloak/apis/v1beta1"
	"github.com/rossigee/provider-keycloak/internal/clients"
)

const (
	errNotComponent      = "managed resource is not a Component"
	errGetProviderConfig = "cannot get ProviderConfig"
	errProviderNotReady  = "provider is not ready"
	controllerName       = "components.component.keycloak.crossplane.io"
)

func Setup(mgr ctrl.Manager, o xpcontroller.Options) error {
	r := managed.NewReconciler(mgr,
		resource.ManagedKind(compv1alpha1.SchemeGroupVersion.WithKind("Component")),
		managed.WithExternalConnector(&connector{kube: mgr.GetClient()}),
		managed.WithLogger(o.Logger.WithValues("controller", "Component")),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorder(controllerName))),
	)
	return ctrl.NewControllerManagedBy(mgr).
		Named(controllerName).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&compv1alpha1.Component{}).
		Complete(r)
}

type connector struct{ kube client.Client }
type external struct{ client clients.Client }

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*compv1alpha1.Component)
	if !ok {
		return nil, errors.New(errNotComponent)
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
	cr, ok := mg.(*compv1alpha1.Component)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotComponent)
	}
	compID := getComponentID(cr)
	if compID == "" {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}
	_, err := e.client.GetComponent(ctx, cr.Spec.ForProvider.RealmId, compID)
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, err
	}
	cr.Status.SetConditions(xpv1.Available())
	return managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: true}, nil
}

func (e *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*compv1alpha1.Component)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotComponent)
	}
	rep := &clients.ComponentRepresentation{
		Name:         cr.Spec.ForProvider.Name,
		ProviderType: cr.Spec.ForProvider.ProviderType,
		Config:       cr.Spec.ForProvider.Config,
	}
	if cr.Spec.ForProvider.ProviderId != nil {
		rep.ProviderID = *cr.Spec.ForProvider.ProviderId
	}
	if cr.Spec.ForProvider.SubType != nil {
		rep.SubType = *cr.Spec.ForProvider.SubType
	}
	id, err := e.client.CreateComponent(ctx, cr.Spec.ForProvider.RealmId, rep)
	if err != nil {
		return managed.ExternalCreation{}, err
	}
	setComponentID(cr, id)
	cr.Status.SetConditions(xpv1.Creating())
	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*compv1alpha1.Component)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotComponent)
	}
	compID := getComponentID(cr)
	rep := &clients.ComponentRepresentation{
		ID:           compID,
		Name:         cr.Spec.ForProvider.Name,
		ProviderType: cr.Spec.ForProvider.ProviderType,
		Config:       cr.Spec.ForProvider.Config,
	}
	if cr.Spec.ForProvider.ProviderId != nil {
		rep.ProviderID = *cr.Spec.ForProvider.ProviderId
	}
	if cr.Spec.ForProvider.SubType != nil {
		rep.SubType = *cr.Spec.ForProvider.SubType
	}
	if err := e.client.UpdateComponent(ctx, cr.Spec.ForProvider.RealmId, compID, rep); err != nil {
		return managed.ExternalUpdate{}, err
	}
	return managed.ExternalUpdate{}, nil
}

func (e *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(*compv1alpha1.Component)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errNotComponent)
	}
	compID := getComponentID(cr)
	if compID != "" {
		err := e.client.DeleteComponent(ctx, cr.Spec.ForProvider.RealmId, compID)
		if err != nil && !strings.Contains(err.Error(), "404") {
			return managed.ExternalDelete{}, err
		}
	}
	cr.Status.SetConditions(xpv1.Deleting())
	return managed.ExternalDelete{}, nil
}

func getComponentID(cr *compv1alpha1.Component) string {
	if cr.Annotations != nil {
		if id, ok := cr.Annotations["keycloak.crossplane.io/component-id"]; ok {
			return id
		}
	}
	return ""
}

func setComponentID(cr *compv1alpha1.Component, id string) {
	if cr.Annotations == nil {
		cr.Annotations = make(map[string]string)
	}
	cr.Annotations["keycloak.crossplane.io/component-id"] = id
}
