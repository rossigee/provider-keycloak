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
	"strings"

	xpcontroller "github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/pkg/errors"
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
type external struct{ client clients.Client }

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
	return &external{client: kc}, nil
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
	_, err := e.client.CreateRealm(ctx, realmParamsToRepresentation(&cr.Spec.ForProvider))
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
	if cr.Spec.ForProvider.LoginWithEmailAllowed != nil {
		currentMap["loginWithEmailAllowed"] = *cr.Spec.ForProvider.LoginWithEmailAllowed
	}
	if cr.Spec.ForProvider.DuplicateEmailsAllowed != nil {
		currentMap["duplicateEmailsAllowed"] = *cr.Spec.ForProvider.DuplicateEmailsAllowed
	}
	if cr.Spec.ForProvider.ResetPasswordAllowed != nil {
		currentMap["resetPasswordAllowed"] = *cr.Spec.ForProvider.ResetPasswordAllowed
	}
	if cr.Spec.ForProvider.EditUsernameAllowed != nil {
		currentMap["editUsernameAllowed"] = *cr.Spec.ForProvider.EditUsernameAllowed
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
	// Remove top-level frontendUrl from currentMap - Keycloak only accepts it in attributes
	delete(currentMap, "frontendUrl")
	// Also handle attributes map
	if cr.Spec.ForProvider.Attributes != nil && len(cr.Spec.ForProvider.Attributes) > 0 {
		for k, v := range cr.Spec.ForProvider.Attributes {
			currentMap[k] = v
		}
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

func realmParamsToRepresentation(p *realmv1alpha1.RealmParameters) *clients.Realm {
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
	if p.LoginWithEmailAllowed != nil {
		r.LoginWithEmailAllowed = *p.LoginWithEmailAllowed
	}
	if p.DuplicateEmailsAllowed != nil {
		r.DuplicateEmailsAllowed = *p.DuplicateEmailsAllowed
	}
	if p.EditUsernameAllowed != nil {
		r.EditUsernameAllowed = *p.EditUsernameAllowed
	}
	if p.ResetPasswordAllowed != nil {
		r.ResetPasswordAllowed = *p.ResetPasswordAllowed
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
	if p.FrontendURL != nil {
		if r.Attributes == nil {
			r.Attributes = make(map[string]string)
		}
		r.Attributes["frontendUrl"] = *p.FrontendURL
	}
	if p.Attributes != nil && len(p.Attributes) > 0 {
		if r.Attributes == nil {
			r.Attributes = p.Attributes
		} else {
			for k, v := range p.Attributes {
				r.Attributes[k] = v
			}
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
	if desired.LoginWithEmailAllowed != nil && *desired.LoginWithEmailAllowed != actual.LoginWithEmailAllowed {
		return false
	}
	if desired.DuplicateEmailsAllowed != nil && *desired.DuplicateEmailsAllowed != actual.DuplicateEmailsAllowed {
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
	return true
}

