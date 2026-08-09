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

package realm

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	xpcontroller "github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	realmv1alpha1 "github.com/rossigee/provider-keycloak/apis/realm/v1alpha1"
	"github.com/rossigee/provider-keycloak/apis/v1beta1"
	"github.com/rossigee/provider-keycloak/internal/clients"
	)

const (
	errNotRealm          = "managed resource is not a Realm"
	errGetProviderConfig = "cannot get ProviderConfig"
	errProviderNotReady  = "provider is not ready"
	errGetRealm          = "cannot get Keycloak realm"
	errCreateRealm       = "cannot create Keycloak realm"
	errUpdateRealm       = "cannot update Keycloak realm"
	errDeleteRealm       = "cannot delete Keycloak realm"

	controllerName = "realms.realm.keycloak.crossplane.io"
)

// Setup registers the Realm controller.
func Setup(mgr ctrl.Manager, o xpcontroller.Options) error {
	r := managed.NewReconciler(mgr,
		resource.ManagedKind(realmv1alpha1.SchemeGroupVersion.WithKind("Realm")),
		managed.WithExternalConnector(&connector{kube: mgr.GetClient()}),
		managed.WithLogger(o.Logger.WithValues("controller", "Realm")),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorder(controllerName))),
	)
	return ctrl.NewControllerManagedBy(mgr).
		Named(controllerName).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&realmv1alpha1.Realm{}).
		Complete(r)
}

type connector struct{ kube client.Client }
type external struct {
	kube   client.Client
	client clients.Client
}

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*realmv1alpha1.Realm)
	if !ok {
		return nil, errors.New(errNotRealm)
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
	return &external{kube: c.kube, client: kc}, nil
}


func (e *external) Disconnect(_ context.Context) error { return nil }

func (e *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*realmv1alpha1.Realm)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotRealm)
	}
	realm := cr.Spec.ForProvider.Realm
	r, err := e.client.GetRealm(ctx, realm)
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, errors.Wrap(err, errGetRealm)
	}
	if r == nil {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}
	cr.Status.SetConditions(xpv1.Available())
	upToDate := realmUpToDate(&cr.Spec.ForProvider, r)
	return managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: upToDate}, nil
}

func (e *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*realmv1alpha1.Realm)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotRealm)
	}
	_, err := e.client.CreateRealm(ctx, realmParamsToRepresentation(ctx, e.kube, &cr.Spec.ForProvider))
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreateRealm)
	}
	cr.Status.SetConditions(xpv1.Creating())
	return managed.ExternalCreation{}, nil
}

func (e *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*realmv1alpha1.Realm)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotRealm)
	}
	// Get raw realm JSON from Keycloak to preserve ALL fields (not just struct fields)
	realmName := cr.Spec.ForProvider.Realm
	currentJSON, err := e.client.GetRawRealm(ctx, realmName)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, "cannot fetch current realm for update")
	}

	// Unmarshal into map to preserve unknown fields
	currentMap := make(map[string]interface{})
	if err := json.Unmarshal(currentJSON, &currentMap); err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, "failed to unmarshal realm response")
	}

	// Apply desired changes from spec
	if cr.Spec.ForProvider.DisplayName != nil {
		currentMap["displayName"] = *cr.Spec.ForProvider.DisplayName
	}
	if cr.Spec.ForProvider.DisplayNameHtml != nil {
		currentMap["displayNameHtml"] = *cr.Spec.ForProvider.DisplayNameHtml
	}
	if cr.Spec.ForProvider.SslRequired != nil {
		currentMap["sslRequired"] = *cr.Spec.ForProvider.SslRequired
	}
	if cr.Spec.ForProvider.RegistrationAllowed != nil {
		currentMap["registrationAllowed"] = *cr.Spec.ForProvider.RegistrationAllowed
	}
	if cr.Spec.ForProvider.RegistrationEmailAsUsername != nil {
		currentMap["registrationEmailAsUsername"] = *cr.Spec.ForProvider.RegistrationEmailAsUsername
	}
	if cr.Spec.ForProvider.EditUsernameAllowed != nil {
		currentMap["editUsernameAllowed"] = *cr.Spec.ForProvider.EditUsernameAllowed
	}
	if cr.Spec.ForProvider.ResetPasswordAllowed != nil {
		currentMap["resetPasswordAllowed"] = *cr.Spec.ForProvider.ResetPasswordAllowed
	}
	if cr.Spec.ForProvider.RememberMe != nil {
		currentMap["rememberMe"] = *cr.Spec.ForProvider.RememberMe
	}
	if cr.Spec.ForProvider.VerifyEmail != nil {
		currentMap["verifyEmail"] = *cr.Spec.ForProvider.VerifyEmail
	}
	if cr.Spec.ForProvider.LoginWithEmailAllowed != nil {
		currentMap["loginWithEmailAllowed"] = *cr.Spec.ForProvider.LoginWithEmailAllowed
	}
	if cr.Spec.ForProvider.DuplicateEmailsAllowed != nil {
		currentMap["duplicateEmailsAllowed"] = *cr.Spec.ForProvider.DuplicateEmailsAllowed
	}
	if cr.Spec.ForProvider.DefaultSignatureAlgorithm != nil {
		currentMap["defaultSignatureAlgorithm"] = *cr.Spec.ForProvider.DefaultSignatureAlgorithm
	}
	if cr.Spec.ForProvider.RevokeRefreshToken != nil {
		currentMap["revokeRefreshToken"] = *cr.Spec.ForProvider.RevokeRefreshToken
	}
	if cr.Spec.ForProvider.RefreshTokenMaxReuse != nil {
		currentMap["refreshTokenMaxReuse"] = *cr.Spec.ForProvider.RefreshTokenMaxReuse
	}
	if cr.Spec.ForProvider.AccessTokenLifespan != nil {
		currentMap["accessTokenLifespan"] = clients.DurationToSeconds(*cr.Spec.ForProvider.AccessTokenLifespan)
	}
	if cr.Spec.ForProvider.AccessTokenLifespanForImplicitFlow != nil {
		currentMap["accessTokenLifespanForImplicitFlow"] = clients.DurationToSeconds(*cr.Spec.ForProvider.AccessTokenLifespanForImplicitFlow)
	}
	if cr.Spec.ForProvider.SsoSessionIdleTimeout != nil {
		currentMap["ssoSessionIdleTimeout"] = clients.DurationToSeconds(*cr.Spec.ForProvider.SsoSessionIdleTimeout)
	}
	if cr.Spec.ForProvider.SsoSessionMaxLifespan != nil {
		currentMap["ssoSessionMaxLifespan"] = clients.DurationToSeconds(*cr.Spec.ForProvider.SsoSessionMaxLifespan)
	}
	if cr.Spec.ForProvider.SsoSessionIdleTimeoutRememberMe != nil {
		currentMap["ssoSessionIdleTimeoutRememberMe"] = clients.DurationToSeconds(*cr.Spec.ForProvider.SsoSessionIdleTimeoutRememberMe)
	}
	if cr.Spec.ForProvider.SsoSessionMaxLifespanRememberMe != nil {
		currentMap["ssoSessionMaxLifespanRememberMe"] = clients.DurationToSeconds(*cr.Spec.ForProvider.SsoSessionMaxLifespanRememberMe)
	}
	if cr.Spec.ForProvider.OfflineSessionIdleTimeout != nil {
		currentMap["offlineSessionIdleTimeout"] = clients.DurationToSeconds(*cr.Spec.ForProvider.OfflineSessionIdleTimeout)
	}
	if cr.Spec.ForProvider.OfflineSessionMaxLifespanEnabled != nil {
		currentMap["offlineSessionMaxLifespanEnabled"] = *cr.Spec.ForProvider.OfflineSessionMaxLifespanEnabled
	}
	if cr.Spec.ForProvider.OfflineSessionMaxLifespan != nil {
		currentMap["offlineSessionMaxLifespan"] = clients.DurationToSeconds(*cr.Spec.ForProvider.OfflineSessionMaxLifespan)
	}
	if cr.Spec.ForProvider.ClientSessionIdleTimeout != nil {
		currentMap["clientSessionIdleTimeout"] = clients.DurationToSeconds(*cr.Spec.ForProvider.ClientSessionIdleTimeout)
	}
	if cr.Spec.ForProvider.ClientSessionMaxLifespan != nil {
		currentMap["clientSessionMaxLifespan"] = clients.DurationToSeconds(*cr.Spec.ForProvider.ClientSessionMaxLifespan)
	}
	if cr.Spec.ForProvider.AccessCodeLifespan != nil {
		currentMap["accessCodeLifespan"] = clients.DurationToSeconds(*cr.Spec.ForProvider.AccessCodeLifespan)
	}
	if cr.Spec.ForProvider.AccessCodeLifespanUserAction != nil {
		currentMap["accessCodeLifespanUserAction"] = clients.DurationToSeconds(*cr.Spec.ForProvider.AccessCodeLifespanUserAction)
	}
	if cr.Spec.ForProvider.AccessCodeLifespanLogin != nil {
		currentMap["accessCodeLifespanLogin"] = clients.DurationToSeconds(*cr.Spec.ForProvider.AccessCodeLifespanLogin)
	}
	if cr.Spec.ForProvider.ActionTokenGeneratedByAdminLifespan != nil {
		currentMap["actionTokenGeneratedByAdminLifespan"] = clients.DurationToSeconds(*cr.Spec.ForProvider.ActionTokenGeneratedByAdminLifespan)
	}
	if cr.Spec.ForProvider.ActionTokenGeneratedByUserLifespan != nil {
		currentMap["actionTokenGeneratedByUserLifespan"] = clients.DurationToSeconds(*cr.Spec.ForProvider.ActionTokenGeneratedByUserLifespan)
	}
	if cr.Spec.ForProvider.PasswordPolicy != nil {
		currentMap["passwordPolicy"] = *cr.Spec.ForProvider.PasswordPolicy
	}
	if cr.Spec.ForProvider.LoginTheme != nil {
		currentMap["loginTheme"] = *cr.Spec.ForProvider.LoginTheme
	}
	if cr.Spec.ForProvider.AccountTheme != nil {
		currentMap["accountTheme"] = *cr.Spec.ForProvider.AccountTheme
	}
	if cr.Spec.ForProvider.AdminTheme != nil {
		currentMap["adminTheme"] = *cr.Spec.ForProvider.AdminTheme
	}
	if cr.Spec.ForProvider.EmailTheme != nil {
		currentMap["emailTheme"] = *cr.Spec.ForProvider.EmailTheme
	}
	if len(cr.Spec.ForProvider.DefaultDefaultClientScopes) > 0 {
		currentMap["defaultDefaultClientScopes"] = cr.Spec.ForProvider.DefaultDefaultClientScopes
	}
	if len(cr.Spec.ForProvider.DefaultOptionalClientScopes) > 0 {
		currentMap["defaultOptionalClientScopes"] = cr.Spec.ForProvider.DefaultOptionalClientScopes
	}
	if cr.Spec.ForProvider.BrowserFlow != nil {
		currentMap["browserFlow"] = *cr.Spec.ForProvider.BrowserFlow
	}
	if cr.Spec.ForProvider.RegistrationFlow != nil {
		currentMap["registrationFlow"] = *cr.Spec.ForProvider.RegistrationFlow
	}
	if cr.Spec.ForProvider.DirectGrantFlow != nil {
		currentMap["directGrantFlow"] = *cr.Spec.ForProvider.DirectGrantFlow
	}
	if cr.Spec.ForProvider.ResetCredentialsFlow != nil {
		currentMap["resetCredentialsFlow"] = *cr.Spec.ForProvider.ResetCredentialsFlow
	}
	if cr.Spec.ForProvider.ClientAuthenticationFlow != nil {
		currentMap["clientAuthenticationFlow"] = *cr.Spec.ForProvider.ClientAuthenticationFlow
	}
	if cr.Spec.ForProvider.UserManagedAccess != nil {
		currentMap["userManagedAccess"] = *cr.Spec.ForProvider.UserManagedAccess
	}
	if cr.Spec.ForProvider.AdminPermissionsEnabled != nil {
		currentMap["adminPermissionsEnabled"] = *cr.Spec.ForProvider.AdminPermissionsEnabled
	}
	// SMTP server config. Keycloak's RealmRepresentation.smtpServer is
	// typed as Map[String] (flat string->string), NOT a nested object -
	// Keycloak 26.x rejects nested-object forms with "Cannot parse the
	// JSON". The controller resolves PasswordSecretRef via the kube
	// client to a plaintext password before building the map.
	if len(cr.Spec.ForProvider.SmtpServer) > 0 {
		smtp, err := buildSmtpServerMap(ctx, e.kube, &cr.Spec.ForProvider.SmtpServer[0])
		if err != nil {
			return managed.ExternalUpdate{}, errors.Wrap(err, "cannot build smtp server config")
		}
		currentMap["smtpServer"] = smtp
	}
	// Remove top-level frontendUrl from currentMap - Keycloak only accepts it in attributes
	delete(currentMap, "frontendUrl")
	// Handle both top-level FrontendURL field and attributes map
	// Get or create attributes map in currentMap
	attrs := map[string]string{}
	if a, ok := currentMap["attributes"].(map[string]string); ok {
		attrs = a
	} else if a, ok := currentMap["attributes"].(map[string]interface{}); ok {
		// Handle interface{} map from JSON unmarshalling
		for k, v := range a {
			if sv, ok := v.(string); ok {
				attrs[k] = sv
			}
		}
	}
	// Handle top-level FrontendURL field from spec
	if cr.Spec.ForProvider.FrontendURL != nil {
		attrs["frontendUrl"] = *cr.Spec.ForProvider.FrontendURL
	}
	// Handle attributes map from spec
	for k, v := range cr.Spec.ForProvider.Attributes {
		attrs[k] = v
	}
	if len(attrs) > 0 {
		currentMap["attributes"] = attrs
	}

	// Send updated map back to Keycloak
	updatedJSON, err := json.Marshal(currentMap)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, "failed to marshal updated realm")
	}

	if err := e.client.UpdateRealmRaw(ctx, realmName, updatedJSON); err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateRealm)
	}
	return managed.ExternalUpdate{}, nil
}

func (e *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(*realmv1alpha1.Realm)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errNotRealm)
	}
	err := e.client.DeleteRealm(ctx, cr.Spec.ForProvider.Realm)
	if err != nil && !strings.Contains(err.Error(), "404") {
		return managed.ExternalDelete{}, errors.Wrap(err, errDeleteRealm)
	}
	cr.Status.SetConditions(xpv1.Deleting())
	return managed.ExternalDelete{}, nil
}

func realmParamsToRepresentation(ctx context.Context, kube client.Client, p *realmv1alpha1.RealmParameters) *clients.Realm {
	r := &clients.Realm{Realm: p.Realm}
	if p.Enabled != nil {
		r.Enabled = *p.Enabled
	} else {
		r.Enabled = true
	}
	if p.DisplayName != nil {
		r.DisplayName = *p.DisplayName
	}
	if p.DisplayNameHtml != nil {
		r.DisplayNameHtml = *p.DisplayNameHtml
	}
	if p.SslRequired != nil {
		r.SslRequired = *p.SslRequired
	}
	if p.RegistrationAllowed != nil {
		r.RegistrationAllowed = *p.RegistrationAllowed
	}
	if p.RegistrationEmailAsUsername != nil {
		r.RegistrationEmailAsUsername = *p.RegistrationEmailAsUsername
	}
	if p.EditUsernameAllowed != nil {
		r.EditUsernameAllowed = *p.EditUsernameAllowed
	}
	if p.ResetPasswordAllowed != nil {
		r.ResetPasswordAllowed = *p.ResetPasswordAllowed
	}
	if p.RememberMe != nil {
		r.RememberMe = *p.RememberMe
	}
	if p.VerifyEmail != nil {
		r.VerifyEmail = *p.VerifyEmail
	}
	if p.LoginWithEmailAllowed != nil {
		r.LoginWithEmailAllowed = *p.LoginWithEmailAllowed
	}
	if p.DuplicateEmailsAllowed != nil {
		r.DuplicateEmailsAllowed = *p.DuplicateEmailsAllowed
	}
	if p.DefaultSignatureAlgorithm != nil {
		r.DefaultSignatureAlgorithm = *p.DefaultSignatureAlgorithm
	}
	if p.RevokeRefreshToken != nil {
		r.RevokeRefreshToken = *p.RevokeRefreshToken
	}
	if p.RefreshTokenMaxReuse != nil {
		r.RefreshTokenMaxReuse = *p.RefreshTokenMaxReuse
	}
	if p.AccessTokenLifespan != nil {
		r.AccessTokenLifespan = clients.DurationToSeconds(*p.AccessTokenLifespan)
	}
	if p.AccessTokenLifespanForImplicitFlow != nil {
		r.AccessTokenLifespanForImplicitFlow = clients.DurationToSeconds(*p.AccessTokenLifespanForImplicitFlow)
	}
	if p.SsoSessionIdleTimeout != nil {
		r.SsoSessionIdleTimeout = clients.DurationToSeconds(*p.SsoSessionIdleTimeout)
	}
	if p.SsoSessionMaxLifespan != nil {
		r.SsoSessionMaxLifespan = clients.DurationToSeconds(*p.SsoSessionMaxLifespan)
	}
	if p.SsoSessionIdleTimeoutRememberMe != nil {
		r.SsoSessionIdleTimeoutRememberMe = clients.DurationToSeconds(*p.SsoSessionIdleTimeoutRememberMe)
	}
	if p.SsoSessionMaxLifespanRememberMe != nil {
		r.SsoSessionMaxLifespanRememberMe = clients.DurationToSeconds(*p.SsoSessionMaxLifespanRememberMe)
	}
	if p.OfflineSessionIdleTimeout != nil {
		r.OfflineSessionIdleTimeout = clients.DurationToSeconds(*p.OfflineSessionIdleTimeout)
	}
	if p.OfflineSessionMaxLifespanEnabled != nil {
		r.OfflineSessionMaxLifespanEnabled = *p.OfflineSessionMaxLifespanEnabled
	}
	if p.OfflineSessionMaxLifespan != nil {
		r.OfflineSessionMaxLifespan = clients.DurationToSeconds(*p.OfflineSessionMaxLifespan)
	}
	if p.ClientSessionIdleTimeout != nil {
		r.ClientSessionIdleTimeout = clients.DurationToSeconds(*p.ClientSessionIdleTimeout)
	}
	if p.ClientSessionMaxLifespan != nil {
		r.ClientSessionMaxLifespan = clients.DurationToSeconds(*p.ClientSessionMaxLifespan)
	}
	if p.AccessCodeLifespan != nil {
		r.AccessCodeLifespan = clients.DurationToSeconds(*p.AccessCodeLifespan)
	}
	if p.AccessCodeLifespanUserAction != nil {
		r.AccessCodeLifespanUserAction = clients.DurationToSeconds(*p.AccessCodeLifespanUserAction)
	}
	if p.AccessCodeLifespanLogin != nil {
		r.AccessCodeLifespanLogin = clients.DurationToSeconds(*p.AccessCodeLifespanLogin)
	}
	if p.ActionTokenGeneratedByAdminLifespan != nil {
		r.ActionTokenGeneratedByAdminLifespan = clients.DurationToSeconds(*p.ActionTokenGeneratedByAdminLifespan)
	}
	if p.ActionTokenGeneratedByUserLifespan != nil {
		r.ActionTokenGeneratedByUserLifespan = clients.DurationToSeconds(*p.ActionTokenGeneratedByUserLifespan)
	}
	if p.PasswordPolicy != nil {
		r.PasswordPolicy = *p.PasswordPolicy
	}
	if p.LoginTheme != nil {
		r.LoginTheme = *p.LoginTheme
	}
	if p.AccountTheme != nil {
		r.AccountTheme = *p.AccountTheme
	}
	if p.AdminTheme != nil {
		r.AdminTheme = *p.AdminTheme
	}
	if p.EmailTheme != nil {
		r.EmailTheme = *p.EmailTheme
	}
	if len(p.DefaultDefaultClientScopes) > 0 {
		r.DefaultDefaultClientScopes = p.DefaultDefaultClientScopes
	}
	if len(p.DefaultOptionalClientScopes) > 0 {
		r.DefaultOptionalClientScopes = p.DefaultOptionalClientScopes
	}
	if p.BrowserFlow != nil {
		r.BrowserFlow = *p.BrowserFlow
	}
	if p.RegistrationFlow != nil {
		r.RegistrationFlow = *p.RegistrationFlow
	}
	if p.DirectGrantFlow != nil {
		r.DirectGrantFlow = *p.DirectGrantFlow
	}
	if p.ResetCredentialsFlow != nil {
		r.ResetCredentialsFlow = *p.ResetCredentialsFlow
	}
	if p.ClientAuthenticationFlow != nil {
		r.ClientAuthenticationFlow = *p.ClientAuthenticationFlow
	}
	if p.UserManagedAccess != nil {
		r.UserManagedAccess = *p.UserManagedAccess
	}
	if p.AdminPermissionsEnabled != nil {
		r.AdminPermissionsEnabled = *p.AdminPermissionsEnabled
	}
	if p.FrontendURL != nil {
		if r.Attributes == nil {
			r.Attributes = make(map[string]string)
		}
		r.Attributes["frontendUrl"] = *p.FrontendURL
	}
	if len(p.Attributes) > 0 {
		if r.Attributes == nil {
			r.Attributes = p.Attributes
		} else {
			for k, v := range p.Attributes {
				r.Attributes[k] = v
			}
		}
	}
	if len(p.SmtpServer) > 0 {
		smtp, err := buildSmtpServerMap(ctx, kube, &p.SmtpServer[0])
		if err != nil {
			// Don't fail Create just because SMTP can't be wired - the
			// subsequent Observe/Update cycle will retry with a properly
			// populated kube client. We log via stderr to surface the
			// misconfiguration during reconciliation.
			fmt.Printf("WARN: realm %s: smtpServer build skipped: %v\n", p.Realm, err)
		} else {
			r.SmtpServer = smtp
		}
	}
	return r
}

func realmUpToDate(desired *realmv1alpha1.RealmParameters, actual *clients.Realm) bool {
	if desired.Enabled != nil && *desired.Enabled != actual.Enabled {
		return false
	}
	if desired.DisplayName != nil && *desired.DisplayName != actual.DisplayName {
		return false
	}
	if desired.DisplayNameHtml != nil && *desired.DisplayNameHtml != actual.DisplayNameHtml {
		return false
	}
	if desired.SslRequired != nil && *desired.SslRequired != actual.SslRequired {
		return false
	}
	if desired.RegistrationAllowed != nil && *desired.RegistrationAllowed != actual.RegistrationAllowed {
		return false
	}
	if desired.RegistrationEmailAsUsername != nil && *desired.RegistrationEmailAsUsername != actual.RegistrationEmailAsUsername {
		return false
	}
	if desired.EditUsernameAllowed != nil && *desired.EditUsernameAllowed != actual.EditUsernameAllowed {
		return false
	}
	if desired.ResetPasswordAllowed != nil && *desired.ResetPasswordAllowed != actual.ResetPasswordAllowed {
		return false
	}
	if desired.RememberMe != nil && *desired.RememberMe != actual.RememberMe {
		return false
	}
	if desired.VerifyEmail != nil && *desired.VerifyEmail != actual.VerifyEmail {
		return false
	}
	if desired.LoginWithEmailAllowed != nil && *desired.LoginWithEmailAllowed != actual.LoginWithEmailAllowed {
		return false
	}
	if desired.DuplicateEmailsAllowed != nil && *desired.DuplicateEmailsAllowed != actual.DuplicateEmailsAllowed {
		return false
	}
	if desired.DefaultSignatureAlgorithm != nil && *desired.DefaultSignatureAlgorithm != actual.DefaultSignatureAlgorithm {
		return false
	}
	if desired.RevokeRefreshToken != nil && *desired.RevokeRefreshToken != actual.RevokeRefreshToken {
		return false
	}
	if desired.RefreshTokenMaxReuse != nil && *desired.RefreshTokenMaxReuse != actual.RefreshTokenMaxReuse {
		return false
	}
	if desired.AccessTokenLifespan != nil && clients.DurationToSeconds(*desired.AccessTokenLifespan) != int(secondsToNumber(actual.AccessTokenLifespan)) {
		return false
	}
	if desired.AccessTokenLifespanForImplicitFlow != nil && clients.DurationToSeconds(*desired.AccessTokenLifespanForImplicitFlow) != int(secondsToNumber(actual.AccessTokenLifespanForImplicitFlow)) {
		return false
	}
	if desired.SsoSessionIdleTimeout != nil && clients.DurationToSeconds(*desired.SsoSessionIdleTimeout) != int(secondsToNumber(actual.SsoSessionIdleTimeout)) {
		return false
	}
	if desired.SsoSessionMaxLifespan != nil && clients.DurationToSeconds(*desired.SsoSessionMaxLifespan) != int(secondsToNumber(actual.SsoSessionMaxLifespan)) {
		return false
	}
	if desired.SsoSessionIdleTimeoutRememberMe != nil && clients.DurationToSeconds(*desired.SsoSessionIdleTimeoutRememberMe) != int(secondsToNumber(actual.SsoSessionIdleTimeoutRememberMe)) {
		return false
	}
	if desired.SsoSessionMaxLifespanRememberMe != nil && clients.DurationToSeconds(*desired.SsoSessionMaxLifespanRememberMe) != int(secondsToNumber(actual.SsoSessionMaxLifespanRememberMe)) {
		return false
	}
	if desired.OfflineSessionIdleTimeout != nil && clients.DurationToSeconds(*desired.OfflineSessionIdleTimeout) != int(secondsToNumber(actual.OfflineSessionIdleTimeout)) {
		return false
	}
	if desired.OfflineSessionMaxLifespanEnabled != nil && *desired.OfflineSessionMaxLifespanEnabled != actual.OfflineSessionMaxLifespanEnabled {
		return false
	}
	if desired.OfflineSessionMaxLifespan != nil && clients.DurationToSeconds(*desired.OfflineSessionMaxLifespan) != int(secondsToNumber(actual.OfflineSessionMaxLifespan)) {
		return false
	}
	if desired.ClientSessionIdleTimeout != nil && clients.DurationToSeconds(*desired.ClientSessionIdleTimeout) != int(secondsToNumber(actual.ClientSessionIdleTimeout)) {
		return false
	}
	if desired.ClientSessionMaxLifespan != nil && clients.DurationToSeconds(*desired.ClientSessionMaxLifespan) != int(secondsToNumber(actual.ClientSessionMaxLifespan)) {
		return false
	}
	if desired.AccessCodeLifespan != nil && clients.DurationToSeconds(*desired.AccessCodeLifespan) != int(secondsToNumber(actual.AccessCodeLifespan)) {
		return false
	}
	if desired.AccessCodeLifespanUserAction != nil && clients.DurationToSeconds(*desired.AccessCodeLifespanUserAction) != int(secondsToNumber(actual.AccessCodeLifespanUserAction)) {
		return false
	}
	if desired.AccessCodeLifespanLogin != nil && clients.DurationToSeconds(*desired.AccessCodeLifespanLogin) != int(secondsToNumber(actual.AccessCodeLifespanLogin)) {
		return false
	}
	if desired.ActionTokenGeneratedByAdminLifespan != nil && clients.DurationToSeconds(*desired.ActionTokenGeneratedByAdminLifespan) != int(secondsToNumber(actual.ActionTokenGeneratedByAdminLifespan)) {
		return false
	}
	if desired.ActionTokenGeneratedByUserLifespan != nil && clients.DurationToSeconds(*desired.ActionTokenGeneratedByUserLifespan) != int(secondsToNumber(actual.ActionTokenGeneratedByUserLifespan)) {
		return false
	}
	if desired.PasswordPolicy != nil && *desired.PasswordPolicy != actual.PasswordPolicy {
		return false
	}
	if desired.LoginTheme != nil && *desired.LoginTheme != actual.LoginTheme {
		return false
	}
	if desired.AccountTheme != nil && *desired.AccountTheme != actual.AccountTheme {
		return false
	}
	if desired.AdminTheme != nil && *desired.AdminTheme != actual.AdminTheme {
		return false
	}
	if desired.EmailTheme != nil && *desired.EmailTheme != actual.EmailTheme {
		return false
	}
	if desired.BrowserFlow != nil && *desired.BrowserFlow != actual.BrowserFlow {
		return false
	}
	if desired.RegistrationFlow != nil && *desired.RegistrationFlow != actual.RegistrationFlow {
		return false
	}
	if desired.DirectGrantFlow != nil && *desired.DirectGrantFlow != actual.DirectGrantFlow {
		return false
	}
	if desired.ResetCredentialsFlow != nil && *desired.ResetCredentialsFlow != actual.ResetCredentialsFlow {
		return false
	}
	if desired.ClientAuthenticationFlow != nil && *desired.ClientAuthenticationFlow != actual.ClientAuthenticationFlow {
		return false
	}
	if desired.UserManagedAccess != nil && *desired.UserManagedAccess != actual.UserManagedAccess {
		return false
	}
	if desired.AdminPermissionsEnabled != nil && *desired.AdminPermissionsEnabled != actual.AdminPermissionsEnabled {
		return false
	}
	if desired.FrontendURL != nil {
		actualFrontendURL := ""
		if actual.FrontendURL != nil {
			actualFrontendURL = *actual.FrontendURL
		}
		if *desired.FrontendURL != actualFrontendURL {
			return false
		}
	}
	// Check attributes
	if desired.Attributes != nil {
		if actual.Attributes == nil {
			return false
		}
		for k, v := range desired.Attributes {
			if actual.Attributes[k] != v {
				return false
			}
		}
	}
	// SMTP server drift detection. We compare the spec-defined fields
	// against the live Keycloak smtpServer map (which Keycloak
	// serialises as a flat map[string]string). The actual password is
	// masked by Keycloak ("**********") in GET responses, so we only
	// compare non-secret fields here; rotation correctness is enforced
	// by the controller pushing the new password on every Update.
	if len(desired.SmtpServer) > 0 {
		if actual.SmtpServer == nil {
			return false
		}
		smtp := buildSmtpServerMapFields(&desired.SmtpServer[0])
		for k, want := range smtp {
			if actual.SmtpServer[k] != want {
				return false
			}
		}
	}
	return true
}

// buildSmtpServerMap resolves the PasswordSecretRef via kube and returns
// the flat map[string]string Keycloak expects for RealmRepresentation.smtpServer.
// Keycloak 26.x rejects any nested-object form ("Cannot parse the JSON"),
// so auth.username and auth.password are flattened to "user" / "password".
func buildSmtpServerMap(ctx context.Context, kube client.Client, p *realmv1alpha1.SmtpServer) (map[string]string, error) {
	m := buildSmtpServerMapFields(p)

	if len(p.Auth) > 0 && p.Auth[0].PasswordSecretRef != nil {
		if kube == nil {
			return nil, errors.New("kube client unavailable - cannot resolve SMTP password SecretRef")
		}
		ref := p.Auth[0].PasswordSecretRef
		var sec corev1.Secret
		if err := kube.Get(ctx, client.ObjectKey{Name: ref.Name, Namespace: ref.Namespace}, &sec); err != nil {
			return nil, errors.Wrapf(err, "cannot read SMTP password secret %s/%s", ref.Namespace, ref.Name)
		}
		pw, ok := sec.Data[ref.Key]
		if !ok {
			return nil, errors.Errorf("SMTP password secret %s/%s has no key %q", ref.Namespace, ref.Name, ref.Key)
		}
		m["auth"] = "true"
		if p.Auth[0].Username != nil {
			m["user"] = *p.Auth[0].Username
		}
		m["password"] = string(pw)
	}
	return m, nil
}

// buildSmtpServerMapFields returns the non-secret fields of an SmtpServer
// in the flat map[string]string shape Keycloak expects.
func buildSmtpServerMapFields(p *realmv1alpha1.SmtpServer) map[string]string {
	m := map[string]string{}
	if p.Host != nil {
		m["host"] = *p.Host
	}
	if p.Port != nil {
		m["port"] = *p.Port
	}
	if p.From != nil {
		m["from"] = *p.From
	}
	if p.FromDisplayName != nil {
		m["fromDisplayName"] = *p.FromDisplayName
	}
	if p.ReplyTo != nil {
		m["replyTo"] = *p.ReplyTo
	}
	if p.ReplyToDisplayName != nil {
		m["replyToDisplayName"] = *p.ReplyToDisplayName
	}
	if p.EnvelopeFrom != nil {
		m["envelopeFrom"] = *p.EnvelopeFrom
	}
	if p.Ssl != nil {
		m["ssl"] = strconv.FormatBool(*p.Ssl)
	}
	if p.Starttls != nil {
		m["starttls"] = strconv.FormatBool(*p.Starttls)
	}
	return m
}

// secondsToNumber extracts a numeric value from an interface{} (JSON number).
// Returns 0 for nil or unrecognized types.
func secondsToNumber(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case json.Number:
		f, _ := val.Float64()
		return f
	default:
		return 0
	}
}

