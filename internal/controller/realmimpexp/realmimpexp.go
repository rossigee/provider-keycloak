package realmimpexp

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

	realmimpexpv1alpha1 "github.com/rossigee/provider-keycloak/apis/realmimpexp/v1alpha1"
	"github.com/rossigee/provider-keycloak/apis/v1beta1"
	"github.com/rossigee/provider-keycloak/internal/clients"
)

const (
	errNotRealmImport          = "managed resource is not a RealmImport"
	errGetProviderConfig       = "cannot get ProviderConfig"
	errImportRealm             = "cannot import realm"
	errProviderNotReady        = "provider is not ready"
	errRealmAlreadyExists      = "realm already exists"
)

const controllerName = "realmimports.realmimpexp.keycloak.crossplane.io"

func Setup(mgr ctrl.Manager, o xpcontroller.Options) error {
	r := managed.NewReconciler(mgr,
		resource.ManagedKind(realmimpexpv1alpha1.SchemeGroupVersion.WithKind("RealmImport")),
		managed.WithExternalConnector(&connector{kube: mgr.GetClient()}),
		managed.WithLogger(o.Logger.WithValues("controller", "RealmImport")),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorder(controllerName))),
	)

	return ctrl.NewControllerManagedBy(mgr).
		Named(controllerName).
		WithOptions(o.ForControllerRuntime()).
		For(&realmimpexpv1alpha1.RealmImport{}).
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
	cr, ok := mg.(*realmimpexpv1alpha1.RealmImport)
	if !ok {
		return nil, errors.New(errNotRealmImport)
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
	cr, ok := mg.(*realmimpexpv1alpha1.RealmImport)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotRealmImport)
	}

	cr.Status.SetConditions(xpv1.Available())

	return managed.ExternalObservation{
		ResourceExists: true,
	}, nil
}

func (e *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*realmimpexpv1alpha1.RealmImport)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotRealmImport)
	}

	ifNotExists := false
	if cr.Spec.ForProvider.IfNotExists != nil {
		ifNotExists = *cr.Spec.ForProvider.IfNotExists
	}

	err := e.client.ImportRealm(ctx, cr.Spec.ForProvider.RealmJSON, ifNotExists)
	if err != nil {
		cr.Status.SetConditions(xpv1.Unavailable().WithMessage(err.Error()))
		return managed.ExternalCreation{}, errors.Wrap(err, errImportRealm)
	}

	cr.Status.SetConditions(xpv1.Available())

	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	return managed.ExternalUpdate{}, nil
}

func (e *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	return managed.ExternalDelete{}, nil
}

func (e *external) Disconnect(_ context.Context) error {
	return nil
}