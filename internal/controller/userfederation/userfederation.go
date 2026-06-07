package userfederation

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

	userfederationv1alpha1 "github.com/rossigee/provider-keycloak/apis/userfederation/v1alpha1"
	"github.com/rossigee/provider-keycloak/apis/v1beta1"
	"github.com/rossigee/provider-keycloak/internal/clients"
)

const (
	errNotUserFederationProvider   = "managed resource is not a UserFederationProvider"
	errGetProviderConfig           = "cannot get ProviderConfig"
	errGetUserFederationProvider   = "cannot get Keycloak user federation provider"
	errCreateUserFederationProvider = "cannot create Keycloak user federation provider"
	errUpdateUserFederationProvider = "cannot update Keycloak user federation provider"
	errDeleteUserFederationProvider = "cannot delete Keycloak user federation provider"
	errProviderNotReady            = "provider is not ready"
)

const controllerName = "userfederationproviders.userfederation.keycloak.crossplane.io"

func Setup(mgr ctrl.Manager, o xpcontroller.Options) error {
	r := managed.NewReconciler(mgr,
		resource.ManagedKind(userfederationv1alpha1.SchemeGroupVersion.WithKind("UserFederationProvider")),
		managed.WithExternalConnector(&connector{kube: mgr.GetClient()}),
		managed.WithLogger(o.Logger.WithValues("controller", "UserFederationProvider")),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorder(controllerName))),
	)

	return ctrl.NewControllerManagedBy(mgr).
		Named(controllerName).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&userfederationv1alpha1.UserFederationProvider{}).
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
	cr, ok := mg.(*userfederationv1alpha1.UserFederationProvider)
	if !ok {
		return nil, errors.New(errNotUserFederationProvider)
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

	return &external{kube: c.kube, client: kc}, nil
}

func (e *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*userfederationv1alpha1.UserFederationProvider)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotUserFederationProvider)
	}

	providers, err := e.client.ListUserFederationProviders(ctx, cr.Spec.ForProvider.RealmId)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errGetUserFederationProvider)
	}

	var provider *clients.UserFederationProviderRepresentation
	for i := range providers {
		if providers[i].Name == cr.Spec.ForProvider.Name {
			p := providers[i]
			provider = &p
			break
		}
	}

	if provider == nil {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	cr.Status.SetConditions(xpv1.Available())

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: userFederationUpToDate(&cr.Spec.ForProvider, provider),
	}, nil
}

func (e *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*userfederationv1alpha1.UserFederationProvider)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotUserFederationProvider)
	}

	rep := userFederationParamsToRepresentation(&cr.Spec.ForProvider)
	_, err := e.client.CreateUserFederationProvider(ctx, cr.Spec.ForProvider.RealmId, rep)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreateUserFederationProvider)
	}

	cr.Status.SetConditions(xpv1.Creating())

	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*userfederationv1alpha1.UserFederationProvider)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotUserFederationProvider)
	}

	providers, err := e.client.ListUserFederationProviders(ctx, cr.Spec.ForProvider.RealmId)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errGetUserFederationProvider)
	}

	var providerID string
	for i := range providers {
		if providers[i].Name == cr.Spec.ForProvider.Name {
			providerID = providers[i].ID
			break
		}
	}

	if providerID == "" {
		return managed.ExternalUpdate{}, errors.New(errGetUserFederationProvider)
	}

	rep := userFederationParamsToRepresentation(&cr.Spec.ForProvider)
	err = e.client.UpdateUserFederationProvider(ctx, cr.Spec.ForProvider.RealmId, providerID, rep)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateUserFederationProvider)
	}

	return managed.ExternalUpdate{}, nil
}

func (e *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(*userfederationv1alpha1.UserFederationProvider)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errNotUserFederationProvider)
	}

	providers, err := e.client.ListUserFederationProviders(ctx, cr.Spec.ForProvider.RealmId)
	if err != nil {
		return managed.ExternalDelete{}, errors.Wrap(err, errGetUserFederationProvider)
	}

	var providerID string
	for i := range providers {
		if providers[i].Name == cr.Spec.ForProvider.Name {
			providerID = providers[i].ID
			break
		}
	}

	if providerID == "" {
		return managed.ExternalDelete{}, nil
	}

	err = e.client.DeleteUserFederationProvider(ctx, cr.Spec.ForProvider.RealmId, providerID)
	if err != nil {
		return managed.ExternalDelete{}, errors.Wrap(err, errDeleteUserFederationProvider)
	}

	return managed.ExternalDelete{}, nil
}

func (e *external) Disconnect(_ context.Context) error {
	return nil
}

func userFederationUpToDate(desired *userfederationv1alpha1.UserFederationProviderParameters, actual *clients.UserFederationProviderRepresentation) bool {
	if desired.Name != actual.Name {
		return false
	}
	if desired.ProviderName != actual.ProviderName {
		return false
	}
	if desired.Priority != nil && actual.Priority != *desired.Priority {
		return false
	}
	if desired.Enabled != nil {
		if actual.Enabled == nil || *desired.Enabled != *actual.Enabled {
			return false
		}
	}
	return true
}

func userFederationParamsToRepresentation(p *userfederationv1alpha1.UserFederationProviderParameters) *clients.UserFederationProviderRepresentation {
	rep := &clients.UserFederationProviderRepresentation{
		Name:         p.Name,
		ProviderName: p.ProviderName,
		Config:       p.Config,
	}
	if p.Priority != nil {
		rep.Priority = *p.Priority
	}
	if p.Enabled != nil {
		rep.Enabled = p.Enabled
	}
	return rep
}