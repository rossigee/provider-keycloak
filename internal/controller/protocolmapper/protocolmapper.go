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

package protocolmapper

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

	clientv1alpha1 "github.com/rossigee/provider-keycloak/apis/client/v1alpha1"
	"github.com/rossigee/provider-keycloak/apis/v1beta1"
	"github.com/rossigee/provider-keycloak/internal/clients"
)

const (
	errNotMapper         = "managed resource is not a ProtocolMapper"
	errGetProviderConfig = "cannot get ProviderConfig"
	errProviderNotReady  = "provider is not ready"
	errGetMapper         = "cannot get Keycloak protocol mapper"
	errCreateMapper      = "cannot create Keycloak protocol mapper"
	errUpdateMapper      = "cannot update Keycloak protocol mapper"
	errDeleteMapper      = "cannot delete Keycloak protocol mapper"
	errResolveClient     = "cannot resolve client UUID"

	controllerName = "protocolmappers.client.keycloak.crossplane.io"
)

// Setup registers the ProtocolMapper controller.
func Setup(mgr ctrl.Manager, o xpcontroller.Options) error {
	r := managed.NewReconciler(mgr,
		resource.ManagedKind(clientv1alpha1.SchemeGroupVersion.WithKind("ProtocolMapper")),
		managed.WithExternalConnector(&connector{kube: mgr.GetClient()}),
		managed.WithLogger(o.Logger.WithValues("controller", "ProtocolMapper")),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorder(controllerName))),
	)
	return ctrl.NewControllerManagedBy(mgr).
		Named(controllerName).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&clientv1alpha1.ProtocolMapper{}).
		Complete(r)
}

type connector struct{ kube client.Client }

type external struct {
	kc clients.Client
}

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*clientv1alpha1.ProtocolMapper)
	if !ok {
		return nil, errors.New(errNotMapper)
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
	return &external{kc: kc}, nil
}

func (e *external) Disconnect(_ context.Context) error { return nil }

// resolveClientUUID looks up the Keycloak internal client UUID from the clientId.
func (e *external) resolveClientUUID(ctx context.Context, realm, clientID string) (string, error) {
	c, err := e.kc.GetClient(ctx, realm, clientID)
	if err != nil {
		return "", errors.Wrap(err, errResolveClient)
	}
	if c == nil {
		return "", errors.Errorf("client %q not found in realm %q", clientID, realm)
	}
	return c.ID, nil
}

func (e *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*clientv1alpha1.ProtocolMapper)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotMapper)
	}
	realmId, clientID, err := realmAndClient(cr)
	if err != nil {
		return managed.ExternalObservation{}, err
	}
	clientUUID, err := e.resolveClientUUID(ctx, realmId, clientID)
	if err != nil {
		return managed.ExternalObservation{}, err
	}
	mappers, err := e.kc.ListClientProtocolMappers(ctx, realmId, clientUUID)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errGetMapper)
	}
	for i := range mappers {
		if mappers[i].Name == cr.Spec.ForProvider.Name {
			cr.Status.SetConditions(xpv1.Available())
			return managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: mapperUpToDate(&cr.Spec.ForProvider, &mappers[i])}, nil
		}
	}
	return managed.ExternalObservation{ResourceExists: false}, nil
}

func (e *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*clientv1alpha1.ProtocolMapper)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotMapper)
	}
	realmId, clientID, err := realmAndClient(cr)
	if err != nil {
		return managed.ExternalCreation{}, err
	}
	clientUUID, err := e.resolveClientUUID(ctx, realmId, clientID)
	if err != nil {
		return managed.ExternalCreation{}, err
	}
	rep := mapperParamsToRepresentation(&cr.Spec.ForProvider)
	_, err = e.kc.CreateClientProtocolMapper(ctx, realmId, clientUUID, rep)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreateMapper)
	}
	cr.Status.SetConditions(xpv1.Creating())
	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*clientv1alpha1.ProtocolMapper)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotMapper)
	}
	realmId, clientID, err := realmAndClient(cr)
	if err != nil {
		return managed.ExternalUpdate{}, err
	}
	clientUUID, err := e.resolveClientUUID(ctx, realmId, clientID)
	if err != nil {
		return managed.ExternalUpdate{}, err
	}
	mappers, err := e.kc.ListClientProtocolMappers(ctx, realmId, clientUUID)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errGetMapper)
	}
	for i := range mappers {
		if mappers[i].Name == cr.Spec.ForProvider.Name {
			rep := mapperParamsToRepresentation(&cr.Spec.ForProvider)
			rep.ID = mappers[i].ID
			if err := e.kc.UpdateClientProtocolMapper(ctx, realmId, clientUUID, rep); err != nil {
				return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateMapper)
			}
			return managed.ExternalUpdate{}, nil
		}
	}
	return managed.ExternalUpdate{}, errors.New("mapper not found for update")
}

func (e *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(*clientv1alpha1.ProtocolMapper)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errNotMapper)
	}
	realmId, clientID, err := realmAndClient(cr)
	if err != nil {
		return managed.ExternalDelete{}, err
	}
	clientUUID, err := e.resolveClientUUID(ctx, realmId, clientID)
	if err != nil {
		return managed.ExternalDelete{}, err
	}
	mappers, err := e.kc.ListClientProtocolMappers(ctx, realmId, clientUUID)
	if err != nil {
		return managed.ExternalDelete{}, errors.Wrap(err, errGetMapper)
	}
	for i := range mappers {
		if mappers[i].Name == cr.Spec.ForProvider.Name {
			err := e.kc.DeleteClientProtocolMapper(ctx, realmId, clientUUID, mappers[i].ID)
			if err != nil && !strings.Contains(err.Error(), "404") {
				return managed.ExternalDelete{}, errors.Wrap(err, errDeleteMapper)
			}
			cr.Status.SetConditions(xpv1.Deleting())
			return managed.ExternalDelete{}, nil
		}
	}
	return managed.ExternalDelete{}, nil
}

func realmAndClient(cr *clientv1alpha1.ProtocolMapper) (realmId, clientId string, err error) {
	if cr.Spec.ForProvider.RealmId == nil || *cr.Spec.ForProvider.RealmId == "" {
		return "", "", errors.New("realmId is required")
	}
	if cr.Spec.ForProvider.ClientId == nil || *cr.Spec.ForProvider.ClientId == "" {
		return "", "", errors.New("clientId is required")
	}
	return *cr.Spec.ForProvider.RealmId, *cr.Spec.ForProvider.ClientId, nil
}

func mapperParamsToRepresentation(p *clientv1alpha1.ProtocolMapperParameters) *clients.ProtocolMapperRepresentation {
	m := &clients.ProtocolMapperRepresentation{
		Name:           p.Name,
		Protocol:       p.Protocol,
		ProtocolMapper: p.ProtocolMapper,
	}
	if p.Config != nil {
		m.Config = p.Config
	}
	return m
}

func mapperUpToDate(desired *clientv1alpha1.ProtocolMapperParameters, actual *clients.ProtocolMapperRepresentation) bool {
	if desired.ProtocolMapper != actual.ProtocolMapper {
		return false
	}
	for k, v := range desired.Config {
		if actual.Config[k] != v {
			return false
		}
	}
	return true
}
