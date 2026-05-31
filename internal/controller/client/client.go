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

package client

import (
	"context"
	"strings"

	"github.com/pkg/errors"
	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/rossigee/provider-keycloak/apis/v1beta1"
	"github.com/rossigee/provider-keycloak/internal/clients"
)

const (
	errNotClient         = "managed resource is not a Client"
	errGetProviderConfig = "cannot get ProviderConfig"
	errClientNotFound    = "Keycloak client not found"
	errCreateClient      = "cannot create Keycloak client"
	errUpdateClient     = "cannot update Keycloak client"
	errDeleteClient     = "cannot delete Keycloak client"
	errGetClient        = "cannot get Keycloak client"
	errProviderNotReady = "Provider is not ready"
)

const controllerName = "client.keycloak.crossplane.io"

// Setup creates and adds a new Controller.
func Setup(mgr ctrl.Manager, o managed.Options) error {
	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1beta1.ClientGroupVersionKind),
		managed.WithExternalConnector(&connector{kube: mgr.GetClient()}),
		managed.WithLogger(o.Logger.WithValues("controller", v1beta1.ClientKind)),
		managed.WithRecorder(mgr.GetEventRecorder(controllerName)),
	)

	return ctrl.NewControllerManagedBy(mgr).
		Named(controllerName).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&v1beta1.Client{}).
		Complete(r)
}

type connector struct {
	kube client.Client
}

type external struct {
	kube   client.Client
	client clients.Client
}

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*v1beta1.Client)
	if !ok {
		return nil, errors.New(errNotClient)
	}

	pc := &v1beta1.ProviderConfig{}
	pcRef := cr.Spec.ProviderConfigRef

	if err := c.kube.Get(ctx, client.ObjectKey{Name: pcRef.Name}, pc); err != nil {
		return nil, errors.Wrap(err, errGetProviderConfig)
	}

	if !pc.Status.IsReady() {
		return nil, errors.New(errProviderNotReady)
	}

	kc, err := clients.NewClient(ctx, pc, c.kube)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create Keycloak client")
	}

	return &external{kube: c.kube, client: kc}, nil
}

func (e *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1beta1.Client)
	if !ok {
		return nil, errors.New(errNotClient)
	}

	realm := cr.Spec.ForProvider.Realm
	clientID := cr.Spec.ForProvider.ClientID

	kcClient, err := e.client.GetClient(ctx, realm, clientID)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errGetClient)
	}

	if kcClient == nil {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	cr.Status.SetConditions(xpv1.Available().WithMessage("Keycloak client is available"))

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: true,
	}, nil
}

func (e *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1beta1.Client)
	if !ok {
		return nil, errors.New(errNotClient)
	}

	realm := cr.Spec.ForProvider.Realm

	client := &clients.ClientRepresentation{
		ClientID:                   cr.Spec.ForProvider.ClientID,
		Name:                       cr.Spec.ForProvider.Name,
		Description:                cr.Spec.ForProvider.Description,
		Enabled:                    true,
		RootURL:                    cr.Spec.ForProvider.RootURL,
		BaseURL:                    cr.Spec.ForProvider.BaseURL,
		ValidRedirectURIs:          cr.Spec.ForProvider.ValidRedirectURIs,
		WebOrigins:                 cr.Spec.ForProvider.WebOrigins,
		StandardFlowEnabled:        true,
		DirectAccessGrantsEnabled:  true,
	}

	if cr.Spec.ForProvider.Enabled != nil {
		client.Enabled = *cr.Spec.ForProvider.Enabled
	}
	if cr.Spec.ForProvider.StandardFlowEnabled != nil {
		client.StandardFlowEnabled = *cr.Spec.ForProvider.StandardFlowEnabled
	}
	if cr.Spec.ForProvider.DirectAccessGrantsEnabled != nil {
		client.DirectAccessGrantsEnabled = *cr.Spec.ForProvider.DirectAccessGrantsEnabled
	}
	if cr.Spec.ForProvider.Protocol != "" {
		client.Protocol = cr.Spec.ForProvider.Protocol
	} else {
		client.Protocol = "openid"
	}
	if cr.Spec.ForProvider.Attributes != nil {
		client.Attributes = cr.Spec.ForProvider.Attributes
	}

	created, err := e.client.CreateClient(ctx, realm, client)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreateClient)
	}

	if created.ID != "" {
		cr.Status.AtProvider = map[string]interface{}{"clientId": created.ID}
	}

	cr.Status.SetConditions(xpv1.Creating().WithMessage("Created Keycloak client"))

	return managed.ExternalCreation{ConnectionDetails: managed.ConnectionDetails{}}, nil
}

func (e *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*v1beta1.Client)
	if !ok {
		return nil, errors.New(errNotClient)
	}

	realm := cr.Spec.ForProvider.Realm

	client := &clients.ClientRepresentation{
		ClientID:          cr.Spec.ForProvider.ClientID,
		Name:              cr.Spec.ForProvider.Name,
		Description:       cr.Spec.ForProvider.Description,
		RootURL:           cr.Spec.ForProvider.RootURL,
		BaseURL:           cr.Spec.ForProvider.BaseURL,
		ValidRedirectURIs: cr.Spec.ForProvider.ValidRedirectURIs,
		WebOrigins:        cr.Spec.ForProvider.WebOrigins,
	}

	if cr.Spec.ForProvider.Enabled != nil {
		client.Enabled = *cr.Spec.ForProvider.Enabled
	}
	if cr.Spec.ForProvider.Protocol != "" {
		client.Protocol = cr.Spec.ForProvider.Protocol
	}
	if cr.Spec.ForProvider.Attributes != nil {
		client.Attributes = cr.Spec.ForProvider.Attributes
	}

	existing, err := e.client.GetClient(ctx, realm, client.ClientID)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errGetClient)
	}
	if existing == nil {
		return managed.ExternalUpdate{}, errors.New(errClientNotFound)
	}

	client.ID = existing.ID
	err = e.client.UpdateClient(ctx, realm, client)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateClient)
	}

	cr.Status.SetConditions(xpv1.Updating().WithMessage("Updated Keycloak client"))

	return managed.ExternalUpdate{}, nil
}

func (e *external) Delete(ctx context.Context, mg resource.Managed) error {
	cr, ok := mg.(*v1beta1.Client)
	if !ok {
		return errors.New(errNotClient)
	}

	realm := cr.Spec.ForProvider.Realm

	existing, err := e.client.GetClient(ctx, realm, cr.Spec.ForProvider.ClientID)
	if err != nil {
		return errors.Wrap(err, errGetClient)
	}
	if existing == nil {
		return nil
	}

	err = e.client.DeleteClient(ctx, realm, existing.ID)
	if err != nil && !strings.Contains(err.Error(), "404") {
		return errors.Wrap(err, errDeleteClient)
	}

	cr.Status.SetConditions(xpv1.Deleting().WithMessage("Deleted Keycloak client"))
	return nil
}