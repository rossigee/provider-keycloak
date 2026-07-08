package authz

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

	authzv1alpha1 "github.com/rossigee/provider-keycloak/apis/authz/v1alpha1"
	"github.com/rossigee/provider-keycloak/apis/v1beta1"
	"github.com/rossigee/provider-keycloak/internal/clients"
	"github.com/rossigee/provider-keycloak/internal/tracing"
)

const (
	errNotAuthzResource    = "managed resource is not an AuthzResource"
	errGetProviderConfig   = "cannot get ProviderConfig"
	errGetAuthzResource    = "cannot get Keycloak authz resource"
	errCreateAuthzResource = "cannot create Keycloak authz resource"
	errUpdateAuthzResource = "cannot update Keycloak authz resource"
	errDeleteAuthzResource = "cannot delete Keycloak authz resource"
	errProviderNotReady    = "provider is not ready"
	errClientNotFound      = "client not found"
)

const controllerName = "authzresources.authz.keycloak.crossplane.io"

func Setup(mgr ctrl.Manager, o xpcontroller.Options) error {
	r := managed.NewReconciler(mgr,
		resource.ManagedKind(authzv1alpha1.SchemeGroupVersion.WithKind("AuthzResource")),
		managed.WithExternalConnector(&connector{kube: mgr.GetClient()}),
		managed.WithLogger(o.Logger.WithValues("controller", "AuthzResource")),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorder(controllerName))),
	)

	return ctrl.NewControllerManagedBy(mgr).
		Named(controllerName).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&authzv1alpha1.AuthzResource{}).
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
	cr, ok := mg.(*authzv1alpha1.AuthzResource)
	if !ok {
		return nil, errors.New(errNotAuthzResource)
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
	_, span := tracing.StartSpan(ctx, "authz.observe",
		tracing.SpanAttrs("AuthzResource", mg.GetName(), "observe")...)
	defer span.End()

	cr, ok := mg.(*authzv1alpha1.AuthzResource)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotAuthzResource)
	}

	client, err := e.client.GetClient(ctx, cr.Spec.ForProvider.RealmId, cr.Spec.ForProvider.ClientId)
	if err != nil {
		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "not found") {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, errors.Wrap(err, errClientNotFound)
	}

	resources, err := e.client.ListAuthzResources(ctx, cr.Spec.ForProvider.RealmId, client.ID)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errGetAuthzResource)
	}

	var resource *clients.AuthzResourceRepresentation
	for i := range resources {
		if resources[i].Name == cr.Spec.ForProvider.Name {
			r := resources[i]
			resource = &r
			break
		}
	}

	if resource == nil {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	cr.Status.SetConditions(xpv1.Available())

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: authzResourceUpToDate(&cr.Spec.ForProvider, resource),
	}, nil
}

func (e *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	_, span := tracing.StartSpan(ctx, "authz.create",
		tracing.SpanAttrs("AuthzResource", mg.GetName(), "create")...)
	defer span.End()

	cr, ok := mg.(*authzv1alpha1.AuthzResource)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotAuthzResource)
	}

	client, err := e.client.GetClient(ctx, cr.Spec.ForProvider.RealmId, cr.Spec.ForProvider.ClientId)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errClientNotFound)
	}

	rep := authzResourceParamsToRepresentation(&cr.Spec.ForProvider)
	_, err = e.client.CreateAuthzResource(ctx, cr.Spec.ForProvider.RealmId, client.ID, rep)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreateAuthzResource)
	}

	cr.Status.SetConditions(xpv1.Creating())

	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	_, span := tracing.StartSpan(ctx, "authz.update",
		tracing.SpanAttrs("AuthzResource", mg.GetName(), "update")...)
	defer span.End()

	cr, ok := mg.(*authzv1alpha1.AuthzResource)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotAuthzResource)
	}

	client, err := e.client.GetClient(ctx, cr.Spec.ForProvider.RealmId, cr.Spec.ForProvider.ClientId)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errClientNotFound)
	}

	resources, err := e.client.ListAuthzResources(ctx, cr.Spec.ForProvider.RealmId, client.ID)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errGetAuthzResource)
	}

	var resourceID string
	for i := range resources {
		if resources[i].Name == cr.Spec.ForProvider.Name {
			resourceID = resources[i].ID
			break
		}
	}

	if resourceID == "" {
		return managed.ExternalUpdate{}, errors.New(errGetAuthzResource)
	}

	rep := authzResourceParamsToRepresentation(&cr.Spec.ForProvider)
	err = e.client.UpdateAuthzResource(ctx, cr.Spec.ForProvider.RealmId, client.ID, resourceID, rep)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateAuthzResource)
	}

	return managed.ExternalUpdate{}, nil
}

func (e *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	_, span := tracing.StartSpan(ctx, "authz.delete",
		tracing.SpanAttrs("AuthzResource", mg.GetName(), "delete")...)
	defer span.End()

	cr, ok := mg.(*authzv1alpha1.AuthzResource)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errNotAuthzResource)
	}

	client, err := e.client.GetClient(ctx, cr.Spec.ForProvider.RealmId, cr.Spec.ForProvider.ClientId)
	if err != nil {
		return managed.ExternalDelete{}, errors.Wrap(err, errClientNotFound)
	}

	resources, err := e.client.ListAuthzResources(ctx, cr.Spec.ForProvider.RealmId, client.ID)
	if err != nil {
		return managed.ExternalDelete{}, errors.Wrap(err, errGetAuthzResource)
	}

	var resourceID string
	for i := range resources {
		if resources[i].Name == cr.Spec.ForProvider.Name {
			resourceID = resources[i].ID
			break
		}
	}

	if resourceID == "" {
		return managed.ExternalDelete{}, nil
	}

	err = e.client.DeleteAuthzResource(ctx, cr.Spec.ForProvider.RealmId, client.ID, resourceID)
	if err != nil {
		return managed.ExternalDelete{}, errors.Wrap(err, errDeleteAuthzResource)
	}

	return managed.ExternalDelete{}, nil
}

func (e *external) Disconnect(_ context.Context) error {
	return nil
}

func authzResourceUpToDate(desired *authzv1alpha1.AuthzResourceParameters, actual *clients.AuthzResourceRepresentation) bool {
	if desired.Name != actual.Name {
		return false
	}
	if desired.DisplayName != nil {
		if actual.DisplayName == nil || *desired.DisplayName != *actual.DisplayName {
			return false
		}
	}
	return true
}

func authzResourceParamsToRepresentation(p *authzv1alpha1.AuthzResourceParameters) *clients.AuthzResourceRepresentation {
	return &clients.AuthzResourceRepresentation{
		Name:        p.Name,
		URIs:        p.URIs,
		Type:        p.Type,
		Scopes:      p.Scopes,
		DisplayName: p.DisplayName,
		IconURI:     p.IconURI,
	}
}
