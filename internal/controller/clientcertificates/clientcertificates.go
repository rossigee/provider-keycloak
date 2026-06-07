package clientcertificates

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

	clientcertificatesv1alpha1 "github.com/rossigee/provider-keycloak/apis/clientcertificates/v1alpha1"
	"github.com/rossigee/provider-keycloak/apis/v1beta1"
	"github.com/rossigee/provider-keycloak/internal/clients"
)

const (
	errNotClientCertificate = "managed resource is not a ClientCertificate"
	errGetProviderConfig    = "cannot get ProviderConfig"
	errGetClient            = "cannot get Keycloak client"
	errGenerateCertificate  = "cannot generate client certificate"
	errListCertificates     = "cannot list client certificates"
	errProviderNotReady     = "provider is not ready"
)

const controllerName = "clientcertificates.clientcertificates.keycloak.crossplane.io"

func Setup(mgr ctrl.Manager, o xpcontroller.Options) error {
	r := managed.NewReconciler(mgr,
		resource.ManagedKind(clientcertificatesv1alpha1.SchemeGroupVersion.WithKind("ClientCertificate")),
		managed.WithExternalConnector(&connector{kube: mgr.GetClient()}),
		managed.WithLogger(o.Logger.WithValues("controller", "ClientCertificate")),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorder(controllerName))),
	)

	return ctrl.NewControllerManagedBy(mgr).
		Named(controllerName).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&clientcertificatesv1alpha1.ClientCertificate{}).
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
	cr, ok := mg.(*clientcertificatesv1alpha1.ClientCertificate)
	if !ok {
		return nil, errors.New(errNotClientCertificate)
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
	cr, ok := mg.(*clientcertificatesv1alpha1.ClientCertificate)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotClientCertificate)
	}

	client, err := e.client.GetClient(ctx, cr.Spec.ForProvider.RealmId, cr.Spec.ForProvider.ClientId)
	if err != nil {
		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "not found") {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, errors.Wrap(err, errGetClient)
	}

	certs, err := e.client.ListClientCertificates(ctx, cr.Spec.ForProvider.RealmId, client.ID)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errListCertificates)
	}

	if len(certs) == 0 {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	latestCert := certs[len(certs)-1]

	cr.Status.Certificate = latestCert.Certificate
	cr.Status.PrivateKey = latestCert.PrivateKey
	cr.Status.SetConditions(xpv1.Available())

	return managed.ExternalObservation{
		ResourceExists: true,
	}, nil
}

func (e *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*clientcertificatesv1alpha1.ClientCertificate)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotClientCertificate)
	}

	client, err := e.client.GetClient(ctx, cr.Spec.ForProvider.RealmId, cr.Spec.ForProvider.ClientId)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errGetClient)
	}

	format := ""
	if cr.Spec.ForProvider.Format != nil {
		format = *cr.Spec.ForProvider.Format
	}

	cert, err := e.client.GenerateClientCertificate(ctx, cr.Spec.ForProvider.RealmId, client.ID, format)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errGenerateCertificate)
	}

	cr.Status.Certificate = cert.Certificate
	cr.Status.PrivateKey = cert.PrivateKey
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
