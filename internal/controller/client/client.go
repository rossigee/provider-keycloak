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

	xpcontroller "github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	openidclientv1alpha1 "github.com/rossigee/provider-keycloak/apis/openidclient/v1alpha1"
	"github.com/rossigee/provider-keycloak/apis/v1beta1"
	"github.com/rossigee/provider-keycloak/internal/clients"
)

const (
	errNotClient         = "managed resource is not a Client"
	errGetProviderConfig = "cannot get ProviderConfig"
	errClientNotFound    = "Keycloak client not found"
	errCreateClient      = "cannot create Keycloak client"
	errUpdateClient      = "cannot update Keycloak client"
	errDeleteClient      = "cannot delete Keycloak client"
	errGetClient         = "cannot get Keycloak client"
	errProviderNotReady  = "provider is not ready"
)

const controllerName = "clients.openidclient.keycloak.crossplane.io"

// Setup creates and adds a new Controller.
func Setup(mgr ctrl.Manager, o xpcontroller.Options) error {
	r := managed.NewReconciler(mgr,
		resource.ManagedKind(openidclientv1alpha1.SchemeGroupVersion.WithKind("Client")),
		managed.WithExternalConnector(&connector{kube: mgr.GetClient()}),
		managed.WithLogger(o.Logger.WithValues("controller", "Client")),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorder(controllerName))),
	)

	return ctrl.NewControllerManagedBy(mgr).
		Named(controllerName).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&openidclientv1alpha1.Client{}).
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
	cr, ok := mg.(*openidclientv1alpha1.Client)
	if !ok {
		return nil, errors.New(errNotClient)
	}

	pcRef := cr.Spec.ProviderConfigReference
	if pcRef == nil {
		return nil, errors.New(errGetProviderConfig + ": providerConfigRef is required")
	}

	pc := &v1beta1.ProviderConfig{}
	if err := c.kube.Get(ctx, client.ObjectKey{Name: pcRef.Name}, pc); err != nil {
		return nil, errors.Wrap(err, errGetProviderConfig)
	}

	// Try to connect to Keycloak - this will determine if provider is ready
	// We don't check ProviderConfig status because CRD may not have status subresource
	kc, err := clients.NewClient(ctx, pc, c.kube)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create Keycloak client")
	}

	return &external{kube: c.kube, client: kc}, nil
}

func (e *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*openidclientv1alpha1.Client)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotClient)
	}

	realmId := ""
	if cr.Spec.ForProvider.RealmId != nil {
		realmId = *cr.Spec.ForProvider.RealmId
	}
	if realmId == "" {
		return managed.ExternalObservation{}, errors.New("realmId is required")
	}

	kcClient, err := e.client.GetClient(ctx, realmId, cr.Spec.ForProvider.ClientId)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errGetClient)
	}

	if kcClient == nil {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	cr.Status.SetConditions(xpv1.Available().WithMessage("Keycloak client is available"))

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: clientUpToDate(&cr.Spec.ForProvider, kcClient),
	}, nil
}

// clientUpToDate returns true when the desired spec matches the live Keycloak state.
func clientUpToDate(desired *openidclientv1alpha1.ClientParameters, actual *clients.ClientRepresentation) bool {
	return clientFlagsUpToDate(desired, actual) && clientURLsUpToDate(desired, actual)
}

func boolChanged(desired *bool, actual bool) bool {
	return desired != nil && *desired != actual
}

func clientFlagsUpToDate(desired *openidclientv1alpha1.ClientParameters, actual *clients.ClientRepresentation) bool {
	flags := []struct {
		desired *bool
		actual  bool
	}{
		{desired.Enabled, actual.Enabled},
		{desired.StandardFlowEnabled, actual.StandardFlowEnabled},
		{desired.ImplicitFlowEnabled, actual.ImplicitFlowEnabled},
		{desired.DirectAccessGrantsEnabled, actual.DirectAccessGrantsEnabled},
		{desired.ServiceAccountsEnabled, actual.ServiceAccountsEnabled},
		{desired.PublicClient, actual.PublicClient},
		{desired.BearerOnly, actual.BearerOnly},
		{desired.ConsentRequired, actual.ConsentRequired},
		{desired.FullScopeAllowed, actual.FullScopeAllowed},
		{desired.AlwaysDisplayInConsole, actual.AlwaysDisplayInConsole},
	}
	for _, f := range flags {
		if boolChanged(f.desired, f.actual) {
			return false
		}
	}
	// Check fields that are now pointers
	if pointerBoolChanged(desired.FrontchannelLogoutEnabled, actual.FrontchannelLogoutEnabled) {
		return false
	}
	if pointerBoolChanged(desired.BackchannelLogoutSessionRequired, actual.BackchannelLogoutSessionRequired) {
		return false
	}
	if pointerBoolChanged(desired.BackchannelLogoutRevokeOfflineSessions, actual.BackchannelLogoutRevokeOfflineSessions) {
		return false
	}
	if pointerBoolChanged(desired.AuthorizationServicesEnabled, actual.AuthorizationServicesEnabled) {
		return false
	}
	if pointerBoolChanged(desired.OAuth2DeviceAuthorizationGrantEnabled, actual.OAuth2DeviceAuthorizationGrantEnabled) {
		return false
	}
	if pointerBoolChanged(desired.StandardTokenExchangeEnabled, actual.StandardTokenExchangeEnabled) {
		return false
	}
	if pointerBoolChanged(desired.UseRefreshTokens, actual.UseRefreshTokens) {
		return false
	}
	return true
}

func stringChanged(desired *string, actual string) bool {
	return desired != nil && *desired != actual
}

func pointerStringChanged(desired, actual *string) bool {
	// If desired is nil, we don't want to change anything - treat as up to date
	if desired == nil {
		return false
	}
	// If actual is nil, treat as empty string
	actualVal := ""
	if actual != nil {
		actualVal = *actual
	}
	return *desired != actualVal
}

func clientURLsUpToDate(desired *openidclientv1alpha1.ClientParameters, actual *clients.ClientRepresentation) bool {
	fields := []struct {
		desired *string
		actual  string
	}{
		{desired.RootUrl, actual.RootURL},
		{desired.HomeUrl, actual.HomeURL},
		{desired.BaseUrl, actual.BaseURL},
		{desired.AdminUrl, actual.AdminURL},
		{desired.Protocol, actual.Protocol},
		{desired.BackchannelLogoutUrl, actual.BackchannelLogoutURL},
		{desired.PkceCodeChallengeMethod, actual.PkceCodeChallengeMethod},
		{desired.AccessTokenLifespan, actual.AccessTokenLifespan},
		{desired.ClientSessionIdleTimeout, actual.ClientSessionIdleTimeout},
		{desired.ClientSessionMaxLifespan, actual.ClientSessionMaxLifespan},
		{desired.ClientOfflineSessionIdleTimeout, actual.ClientOfflineSessionIdleTimeout},
		{desired.ClientOfflineSessionMaxLifespan, actual.ClientOfflineSessionMaxLifespan},
	}
	for _, f := range fields {
		if stringChanged(f.desired, f.actual) {
			return false
		}
	}
	// Check FrontchannelLogoutUrl separately since it's a pointer now
	if pointerStringChanged(desired.FrontchannelLogoutUrl, actual.FrontchannelLogoutURL) {
		return false
	}
	if desired.ValidRedirectUris != nil && !stringSlicesEqual(desired.ValidRedirectUris, actual.ValidRedirectURIs) {
		return false
	}
	if desired.WebOrigins != nil && !stringSlicesEqual(desired.WebOrigins, actual.WebOrigins) {
		return false
	}
	return true
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	seen := make(map[string]int, len(a))
	for _, v := range a {
		seen[v]++
	}
	for _, v := range b {
		seen[v]--
		if seen[v] < 0 {
			return false
		}
	}
	return true
}

func (e *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*openidclientv1alpha1.Client)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotClient)
	}

	realmId := ""
	if cr.Spec.ForProvider.RealmId != nil {
		realmId = *cr.Spec.ForProvider.RealmId
	}
	if realmId == "" {
		return managed.ExternalCreation{}, errors.New("realmId is required")
	}

	rep := clientParamsToRepresentation(&cr.Spec.ForProvider)

	created, err := e.client.CreateClient(ctx, realmId, rep)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreateClient)
	}

	if created != nil && created.ID != "" && cr.Spec.ForProvider.ClientSecretSecretRef != nil {
		if err := e.writeClientSecret(ctx, realmId, created.ID, cr.Spec.ForProvider.ClientSecretSecretRef); err != nil {
			return managed.ExternalCreation{}, errors.Wrap(err, "cannot write client secret")
		}
	}

	cr.Status.SetConditions(xpv1.Creating().WithMessage("creating Keycloak client"))

	return managed.ExternalCreation{ConnectionDetails: managed.ConnectionDetails{}}, nil
}

func (e *external) writeClientSecret(ctx context.Context, realm, clientUUID string, ref *openidclientv1alpha1.ClientSecretSecretRef) error {
	secretValue, err := e.client.GetClientSecret(ctx, realm, clientUUID)
	if err != nil {
		return errors.Wrap(err, "cannot fetch client secret from Keycloak")
	}

	secret := &corev1.Secret{}
	nn := types.NamespacedName{Name: ref.Name, Namespace: ref.Namespace}
	if err := e.kube.Get(ctx, nn, secret); err != nil {
		if client.IgnoreNotFound(err) != nil {
			return errors.Wrap(err, "cannot get target secret")
		}
		// Secret does not exist — create it.
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: ref.Name, Namespace: ref.Namespace},
			Data:       map[string][]byte{ref.Key: []byte(secretValue)},
		}
		return errors.Wrap(e.kube.Create(ctx, secret), "cannot create target secret")
	}
	// Secret exists — update the key.
	if secret.Data == nil {
		secret.Data = make(map[string][]byte)
	}
	secret.Data[ref.Key] = []byte(secretValue)
	return errors.Wrap(e.kube.Update(ctx, secret), "cannot update target secret")
}

func (e *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*openidclientv1alpha1.Client)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotClient)
	}

	realmId := ""
	if cr.Spec.ForProvider.RealmId != nil {
		realmId = *cr.Spec.ForProvider.RealmId
	}
	if realmId == "" {
		return managed.ExternalUpdate{}, errors.New("realmId is required")
	}

	existing, err := e.client.GetClient(ctx, realmId, cr.Spec.ForProvider.ClientId)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errGetClient)
	}
	if existing == nil {
		return managed.ExternalUpdate{}, errors.New(errClientNotFound)
	}

	rep := clientParamsToRepresentation(&cr.Spec.ForProvider)
	rep.ID = existing.ID

	if err := e.client.UpdateClient(ctx, realmId, rep); err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateClient)
	}

	return managed.ExternalUpdate{}, nil
}

func (e *external) Disconnect(_ context.Context) error { return nil }

func (e *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(*openidclientv1alpha1.Client)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errNotClient)
	}

	realmId := ""
	if cr.Spec.ForProvider.RealmId != nil {
		realmId = *cr.Spec.ForProvider.RealmId
	}
	if realmId == "" {
		return managed.ExternalDelete{}, errors.New("realmId is required")
	}

	existing, err := e.client.GetClient(ctx, realmId, cr.Spec.ForProvider.ClientId)
	if err != nil {
		return managed.ExternalDelete{}, errors.Wrap(err, errGetClient)
	}
	if existing == nil {
		return managed.ExternalDelete{}, nil
	}

	err = e.client.DeleteClient(ctx, realmId, existing.ID)
	if err != nil && !strings.Contains(err.Error(), "404") {
		return managed.ExternalDelete{}, errors.Wrap(err, errDeleteClient)
	}

	cr.Status.SetConditions(xpv1.Deleting().WithMessage("deleting Keycloak client"))
	return managed.ExternalDelete{}, nil
}

// clientParamsToRepresentation maps CR parameters to a Keycloak API representation.
func clientParamsToRepresentation(p *openidclientv1alpha1.ClientParameters) *clients.ClientRepresentation {
	rep := &clients.ClientRepresentation{
		ClientID:                               p.ClientId,
		Enabled:                                boolVal(p.Enabled, true),
		StandardFlowEnabled:                    boolVal(p.StandardFlowEnabled, false),
		ImplicitFlowEnabled:                    boolVal(p.ImplicitFlowEnabled, false),
		DirectAccessGrantsEnabled:              boolVal(p.DirectAccessGrantsEnabled, false),
		ServiceAccountsEnabled:                 boolVal(p.ServiceAccountsEnabled, false),
		PublicClient:                           boolVal(p.PublicClient, false),
		BearerOnly:                             boolVal(p.BearerOnly, false),
		ConsentRequired:                        boolVal(p.ConsentRequired, false),
		FullScopeAllowed:                       boolVal(p.FullScopeAllowed, false),
		AlwaysDisplayInConsole:                 boolVal(p.AlwaysDisplayInConsole, false),
		FrontchannelLogoutEnabled:              p.FrontchannelLogoutEnabled,
		BackchannelLogoutSessionRequired:       p.BackchannelLogoutSessionRequired,
		BackchannelLogoutRevokeOfflineSessions: p.BackchannelLogoutRevokeOfflineSessions,
		AuthorizationServicesEnabled:           p.AuthorizationServicesEnabled,
		OAuth2DeviceAuthorizationGrantEnabled:  p.OAuth2DeviceAuthorizationGrantEnabled,
		StandardTokenExchangeEnabled:           p.StandardTokenExchangeEnabled,
		UseRefreshTokens:                       p.UseRefreshTokens,
	}
	setClientStrings(rep, p)
	setClientSlices(rep, p)
	return rep
}

func setClientStrings(rep *clients.ClientRepresentation, p *openidclientv1alpha1.ClientParameters) {
	stringFields := []struct {
		source *string
		target *string
	}{
		{p.Name, &rep.Name},
		{p.Description, &rep.Description},
		{p.RootUrl, &rep.RootURL},
		{p.HomeUrl, &rep.HomeURL},
		{p.BaseUrl, &rep.BaseURL},
		{p.AdminUrl, &rep.AdminURL},
		{p.Protocol, &rep.Protocol},
		{p.PkceCodeChallengeMethod, &rep.PkceCodeChallengeMethod},
		{p.AccessTokenLifespan, &rep.AccessTokenLifespan},
		{p.ClientSessionIdleTimeout, &rep.ClientSessionIdleTimeout},
		{p.ClientSessionMaxLifespan, &rep.ClientSessionMaxLifespan},
		{p.ClientOfflineSessionIdleTimeout, &rep.ClientOfflineSessionIdleTimeout},
		{p.ClientOfflineSessionMaxLifespan, &rep.ClientOfflineSessionMaxLifespan},
		{p.BackchannelLogoutUrl, &rep.BackchannelLogoutURL},
	}
	for _, f := range stringFields {
		if f.source != nil {
			*f.target = *f.source
		}
	}
	// Handle FrontchannelLogoutUrl specially - it's a pointer now
	if p.FrontchannelLogoutUrl != nil {
		rep.FrontchannelLogoutURL = p.FrontchannelLogoutUrl
	} else {
		rep.FrontchannelLogoutURL = nil
	}
}

func setClientSlices(rep *clients.ClientRepresentation, p *openidclientv1alpha1.ClientParameters) {
	if p.ValidRedirectUris != nil {
		rep.ValidRedirectURIs = p.ValidRedirectUris
	}
	if p.WebOrigins != nil {
		rep.WebOrigins = p.WebOrigins
	}
}

func boolVal(p *bool, def bool) bool {
	if p == nil {
		return def
	}
	return *p
}

func pointerBoolChanged(desired, actual *bool) bool {
	// If desired is nil, we don't want to change anything - treat as up to date
	if desired == nil {
		return false
	}
	// If actual is nil, treat as false
	actualVal := false
	if actual != nil {
		actualVal = *actual
	}
	return *desired != actualVal
}
