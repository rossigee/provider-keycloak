package events

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

	eventv1alpha1 "github.com/rossigee/provider-keycloak/apis/events/v1alpha1"
	"github.com/rossigee/provider-keycloak/apis/v1beta1"
	"github.com/rossigee/provider-keycloak/internal/clients"
	"github.com/rossigee/provider-keycloak/internal/tracing"
)

const (
	errNotRealmEventsConfig    = "managed resource is not a RealmEventsConfig"
	errGetProviderConfig       = "cannot get ProviderConfig"
	errGetRealmEventsConfig    = "cannot get Keycloak realm events config"
	errUpdateRealmEventsConfig = "cannot update Keycloak realm events config"
	errProviderNotReady        = "provider is not ready"
)

const controllerName = "realmeventsconfigs.events.keycloak.crossplane.io"

func Setup(mgr ctrl.Manager, o xpcontroller.Options) error {
	r := managed.NewReconciler(mgr,
		resource.ManagedKind(eventv1alpha1.SchemeGroupVersion.WithKind("RealmEventsConfig")),
		managed.WithExternalConnector(&connector{kube: mgr.GetClient()}),
		managed.WithLogger(o.Logger.WithValues("controller", "RealmEventsConfig")),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorder(controllerName))),
	)

	return ctrl.NewControllerManagedBy(mgr).
		Named(controllerName).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&eventv1alpha1.RealmEventsConfig{}).
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
	cr, ok := mg.(*eventv1alpha1.RealmEventsConfig)
	if !ok {
		return nil, errors.New(errNotRealmEventsConfig)
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
	_, span := tracing.StartSpan(ctx, "events.observe",
		tracing.SpanAttrs("RealmEventsConfig", mg.GetName(), "observe")...)
	defer span.End()

	cr, ok := mg.(*eventv1alpha1.RealmEventsConfig)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotRealmEventsConfig)
	}

	config, err := e.client.GetRealmEventsConfig(ctx, cr.Spec.ForProvider.RealmId)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errGetRealmEventsConfig)
	}

	cr.Status.SetConditions(xpv1.Available())

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: eventsConfigUpToDate(&cr.Spec.ForProvider, config),
	}, nil
}

func (e *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	_, span := tracing.StartSpan(ctx, "events.create",
		tracing.SpanAttrs("RealmEventsConfig", mg.GetName(), "create")...)
	defer span.End()

	cr, ok := mg.(*eventv1alpha1.RealmEventsConfig)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotRealmEventsConfig)
	}

	rep := eventsConfigParamsToRepresentation(&cr.Spec.ForProvider)
	err := e.client.UpdateRealmEventsConfig(ctx, cr.Spec.ForProvider.RealmId, rep)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errUpdateRealmEventsConfig)
	}

	cr.Status.SetConditions(xpv1.Creating())

	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	_, span := tracing.StartSpan(ctx, "events.update",
		tracing.SpanAttrs("RealmEventsConfig", mg.GetName(), "update")...)
	defer span.End()

	cr, ok := mg.(*eventv1alpha1.RealmEventsConfig)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotRealmEventsConfig)
	}

	rep := eventsConfigParamsToRepresentation(&cr.Spec.ForProvider)
	err := e.client.UpdateRealmEventsConfig(ctx, cr.Spec.ForProvider.RealmId, rep)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateRealmEventsConfig)
	}

	return managed.ExternalUpdate{}, nil
}

func (e *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	_, span := tracing.StartSpan(ctx, "events.delete",
		tracing.SpanAttrs("RealmEventsConfig", mg.GetName(), "delete")...)
	defer span.End()

	return managed.ExternalDelete{}, nil
}

func (e *external) Disconnect(_ context.Context) error {
	return nil
}

func eventsConfigUpToDate(desired *eventv1alpha1.RealmEventsConfigParameters, actual *clients.RealmEventsConfigRepresentation) bool {
	if desired.EventsEnabled != nil {
		if actual.EventsEnabled == nil || *desired.EventsEnabled != *actual.EventsEnabled {
			return false
		}
	}
	if desired.AdminEventsEnabled != nil {
		if actual.AdminEventsEnabled == nil || *desired.AdminEventsEnabled != *actual.AdminEventsEnabled {
			return false
		}
	}
	if desired.AdminEventsDetailsEnabled != nil {
		if actual.AdminEventsDetailsEnabled == nil || *desired.AdminEventsDetailsEnabled != *actual.AdminEventsDetailsEnabled {
			return false
		}
	}
	return true
}

func eventsConfigParamsToRepresentation(p *eventv1alpha1.RealmEventsConfigParameters) *clients.RealmEventsConfigRepresentation {
	return &clients.RealmEventsConfigRepresentation{
		EventsEnabled:             p.EventsEnabled,
		EventsExpiration:          p.EventsExpiration,
		EventsListeners:           p.EventsListeners,
		EnabledEvents:             p.EnabledEvents,
		AdminEventsEnabled:        p.AdminEventsEnabled,
		AdminEventsDetailsEnabled: p.AdminEventsDetailsEnabled,
	}
}
