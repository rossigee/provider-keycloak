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

package clientdefaultscopes

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

	"github.com/rossigee/provider-keycloak/internal/features"

	openidclientv1beta1 "github.com/rossigee/provider-keycloak/apis/openidclient/v1beta1"
	"github.com/rossigee/provider-keycloak/apis/v1beta1"
	"github.com/rossigee/provider-keycloak/internal/clients"
	"github.com/rossigee/provider-keycloak/internal/tracing"
)

const (
	errNotClientDefaultScopes = "managed resource is not a ClientDefaultScopes"
	errGetProviderConfig      = "cannot get ProviderConfig"
	errProviderNotReady       = "provider is not ready"
	errResolveClient          = "cannot resolve client UUID"
	errResolveScope           = "cannot resolve client scope"
	controllerName            = "clientdefaultscopes.client.keycloak.m.crossplane.io"
)

// resolveClientUUID looks up the Keycloak internal client UUID from the clientId.
func resolveClientUUID(ctx context.Context, kc clients.Client, realm, clientID string) (string, error) {
	c, err := kc.GetClient(ctx, realm, clientID)
	if err != nil {
		return "", errors.Wrap(err, errResolveClient)
	}
	if c == nil {
		return "", errors.Errorf("client %q not found in realm %q", clientID, realm)
	}
	return c.ID, nil
}

// resolveScopeIDs maps scope names to their Keycloak internal UUIDs.
func resolveScopeIDs(ctx context.Context, kc clients.Client, realm string, names []string) ([]clients.ClientScopeRepresentation, error) {
	result := make([]clients.ClientScopeRepresentation, 0, len(names))
	for _, n := range names {
		s, err := kc.GetClientScope(ctx, realm, n)
		if err != nil {
			return nil, errors.Wrap(err, errResolveScope)
		}
		if s == nil {
			return nil, errors.Errorf("client scope %q not found in realm %q", n, realm)
		}
		result = append(result, *s)
	}
	return result, nil
}

func Setup(mgr ctrl.Manager, o xpcontroller.Options) error {
	opts := []managed.ReconcilerOption{
		managed.WithExternalConnector(&connector{kube: mgr.GetClient()}),
		managed.WithLogger(o.Logger.WithValues("controller", "ClientDefaultScopes")),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorder(controllerName))),
	}
	if o.Features.Enabled(features.EnableAlphaManagementPolicies) {
		opts = append(opts, managed.WithManagementPolicies())
	}

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(openidclientv1beta1.SchemeGroupVersion.WithKind("ClientDefaultScopes")),
		opts...)
	return ctrl.NewControllerManagedBy(mgr).
		Named(controllerName).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&openidclientv1beta1.ClientDefaultScopes{}).
		Complete(r)
}

type connector struct{ kube client.Client }
type external struct{ client clients.Client }

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*openidclientv1beta1.ClientDefaultScopes)
	if !ok {
		return nil, errors.New(errNotClientDefaultScopes)
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
	shared := clients.GetConnector(c.kube)
	kc, err := shared.Connect(ctx, pcRef.Name)
	if err != nil {
		return nil, errors.Wrap(err, "cannot connect to Keycloak")
	}
	return &external{client: kc}, nil
}

func (e *external) Disconnect(_ context.Context) error { return nil }

func (e *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*openidclientv1beta1.ClientDefaultScopes)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotClientDefaultScopes)
	}
	return ObserveClientDefaultScopes(ctx, e.client, cr)
}

func (e *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*openidclientv1beta1.ClientDefaultScopes)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotClientDefaultScopes)
	}
	return CreateClientDefaultScopes(ctx, e.client, cr)
}

func (e *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*openidclientv1beta1.ClientDefaultScopes)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotClientDefaultScopes)
	}
	return UpdateClientDefaultScopes(ctx, e.client, cr)
}

func (e *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(*openidclientv1beta1.ClientDefaultScopes)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errNotClientDefaultScopes)
	}
	return DeleteClientDefaultScopes(ctx, e.client, cr)
}

// ObserveClientDefaultScopes reports whether the requested default scopes
// match the scopes already assigned to the client.
func ObserveClientDefaultScopes(ctx context.Context, kc clients.Client, cr *openidclientv1beta1.ClientDefaultScopes) (managed.ExternalObservation, error) {
	_, span := tracing.StartSpan(ctx, "clientdefaultscopes.observe",
		tracing.SpanAttrs("ClientDefaultScopes", cr.GetName(), "observe")...)
	defer span.End()

	clientUUID, err := resolveClientUUID(ctx, kc, deref(cr.Spec.ForProvider.RealmId), deref(cr.Spec.ForProvider.ClientId))
	if err != nil {
		return managed.ExternalObservation{}, err
	}
	current, err := kc.ListClientDefaultScopes(ctx, deref(cr.Spec.ForProvider.RealmId), clientUUID)
	if err != nil {
		return managed.ExternalObservation{}, err
	}
	cr.Status.SetConditions(xpv1.Available())
	upToDate := stringsScopeMatch(cr.Spec.ForProvider.DefaultScopes, current)
	// Outside deletion the external "resource" is the scope assignment on the
	// client; we report it as existing so the reconciler drives changes through
	// Update (Create is not used for this resource type). During deletion we
	// must report it as gone once no scopes remain: the managed reconciler only
	// finalizes a delete when Observe reports ResourceExists=false, otherwise it
	// re-enters the delete branch forever.
	exists := cr.GetDeletionTimestamp() == nil || len(current) > 0
	return managed.ExternalObservation{ResourceExists: exists, ResourceUpToDate: upToDate}, nil
}

// CreateClientDefaultScopes adds every requested default scope UUID to the
// client. Errors out if any scope name does not exist in the realm.
func CreateClientDefaultScopes(ctx context.Context, kc clients.Client, cr *openidclientv1beta1.ClientDefaultScopes) (managed.ExternalCreation, error) {
	_, span := tracing.StartSpan(ctx, "clientdefaultscopes.create",
		tracing.SpanAttrs("ClientDefaultScopes", cr.GetName(), "create")...)
	defer span.End()

	clientUUID, err := resolveClientUUID(ctx, kc, deref(cr.Spec.ForProvider.RealmId), deref(cr.Spec.ForProvider.ClientId))
	if err != nil {
		return managed.ExternalCreation{}, err
	}
	scopes, err := resolveScopeIDs(ctx, kc, deref(cr.Spec.ForProvider.RealmId), cr.Spec.ForProvider.DefaultScopes)
	if err != nil {
		return managed.ExternalCreation{}, err
	}
	if err := kc.AddClientDefaultScopes(ctx, deref(cr.Spec.ForProvider.RealmId), clientUUID, scopes); err != nil {
		return managed.ExternalCreation{}, err
	}
	cr.Status.SetConditions(xpv1.Creating())
	return managed.ExternalCreation{}, nil
}

// UpdateClientDefaultScopes reconciles the client's default scopes with the
// desired list by adding missing and removing obsolete ones.
func UpdateClientDefaultScopes(ctx context.Context, kc clients.Client, cr *openidclientv1beta1.ClientDefaultScopes) (managed.ExternalUpdate, error) {
	_, span := tracing.StartSpan(ctx, "clientdefaultscopes.update",
		tracing.SpanAttrs("ClientDefaultScopes", cr.GetName(), "update")...)
	defer span.End()

	clientUUID, err := resolveClientUUID(ctx, kc, deref(cr.Spec.ForProvider.RealmId), deref(cr.Spec.ForProvider.ClientId))
	if err != nil {
		return managed.ExternalUpdate{}, err
	}
	current, err := kc.ListClientDefaultScopes(ctx, deref(cr.Spec.ForProvider.RealmId), clientUUID)
	if err != nil {
		return managed.ExternalUpdate{}, err
	}
	desired, err := resolveScopeIDs(ctx, kc, deref(cr.Spec.ForProvider.RealmId), cr.Spec.ForProvider.DefaultScopes)
	if err != nil {
		return managed.ExternalUpdate{}, err
	}
	toAdd := scopeDiff(desired, current)
	toRemove := scopeDiff(current, desired)
	if len(toAdd) > 0 {
		if err := kc.AddClientDefaultScopes(ctx, deref(cr.Spec.ForProvider.RealmId), clientUUID, toAdd); err != nil {
			return managed.ExternalUpdate{}, err
		}
	}
	if len(toRemove) > 0 {
		if err := kc.RemoveClientDefaultScopes(ctx, deref(cr.Spec.ForProvider.RealmId), clientUUID, toRemove); err != nil {
			return managed.ExternalUpdate{}, err
		}
	}
	return managed.ExternalUpdate{}, nil
}

// DeleteClientDefaultScopes removes every default scope the client currently
// has. No-op when the client does not exist.
func DeleteClientDefaultScopes(ctx context.Context, kc clients.Client, cr *openidclientv1beta1.ClientDefaultScopes) (managed.ExternalDelete, error) {
	_, span := tracing.StartSpan(ctx, "clientdefaultscopes.delete",
		tracing.SpanAttrs("ClientDefaultScopes", cr.GetName(), "delete")...)
	defer span.End()

	clientUUID, err := resolveClientUUID(ctx, kc, deref(cr.Spec.ForProvider.RealmId), deref(cr.Spec.ForProvider.ClientId))
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return managed.ExternalDelete{}, nil
		}
		return managed.ExternalDelete{}, err
	}
	current, err := kc.ListClientDefaultScopes(ctx, deref(cr.Spec.ForProvider.RealmId), clientUUID)
	if err != nil && !strings.Contains(err.Error(), "404") {
		return managed.ExternalDelete{}, err
	}
	if len(current) > 0 {
		if err := kc.RemoveClientDefaultScopes(ctx, deref(cr.Spec.ForProvider.RealmId), clientUUID, current); err != nil {
			return managed.ExternalDelete{}, err
		}
	}
	cr.Status.SetConditions(xpv1.Deleting())
	return managed.ExternalDelete{}, nil
}

func stringsScopeMatch(desired []string, current []clients.ClientScopeRepresentation) bool {
	if len(desired) != len(current) {
		return false
	}
	for _, d := range desired {
		found := false
		for _, c := range current {
			if d == c.ID || d == c.Name {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func scopeDiff(desired, current []clients.ClientScopeRepresentation) []clients.ClientScopeRepresentation {
	var diff []clients.ClientScopeRepresentation
	for _, d := range desired {
		found := false
		for _, c := range current {
			if (d.ID != "" && d.ID == c.ID) || (d.Name != "" && d.Name == c.Name) {
				found = true
				break
			}
		}
		if !found {
			diff = append(diff, d)
		}
	}
	return diff
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
