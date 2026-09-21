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

package user

import (
	"context"
	"strings"

	xpcontroller "github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/pkg/errors"
	"github.com/rossigee/provider-keycloak/internal/features"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	groupv1beta1 "github.com/rossigee/provider-keycloak/apis/group/v1beta1"
	userv1beta1 "github.com/rossigee/provider-keycloak/apis/user/v1beta1"
	"github.com/rossigee/provider-keycloak/apis/v1beta1"
	"github.com/rossigee/provider-keycloak/internal/clients"
	"github.com/rossigee/provider-keycloak/internal/tracing"
)

const (
	errNotGroups           = "managed resource is not a Groups"
	errResolveUser         = "cannot resolve referenced User"
	errResolveGroup        = "cannot resolve referenced Group"
	errMissingUserRef      = "userId or userIdRef must be set"
	errMissingGroupRef     = "at least one of groupIds or groupIdsRefs must be set"
	errGetUserGroups       = "cannot get Keycloak user's group memberships"
	errAddUserToGroup      = "cannot add user to Keycloak group"
	errRemoveUserGroup     = "cannot remove user from Keycloak group"
	errGetResolvedUser     = "cannot get resolved user from Keycloak"
	errGetResolvedGroup    = "cannot get resolved group from Keycloak"
	errResolvedUserMissing = "referenced user not yet present in Keycloak"
	errResolvedGroupMissing = "referenced group not yet present in Keycloak"

	groupsControllerName = "groups.user.keycloak.m.crossplane.io"
)

// SetupGroups registers the Groups (user↔group membership) controller.
func SetupGroups(mgr ctrl.Manager, o xpcontroller.Options) error {
	opts := []managed.ReconcilerOption{
		managed.WithExternalConnector(&groupsConnector{kube: mgr.GetClient()}),
		managed.WithLogger(o.Logger.WithValues("controller", "Groups")),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorder(groupsControllerName))),
	}
	if o.Features.Enabled(features.EnableAlphaManagementPolicies) {
		opts = append(opts, managed.WithManagementPolicies())
	}

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(userv1beta1.SchemeGroupVersion.WithKind("Groups")),
		opts...)
	return ctrl.NewControllerManagedBy(mgr).
		Named(groupsControllerName).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&userv1beta1.Groups{}).
		Complete(r)
}

type groupsConnector struct{ kube client.Client }
type groupsExternal struct {
	client clients.Client
	kube   client.Client
}

func (c *groupsConnector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*userv1beta1.Groups)
	if !ok {
		return nil, errors.New(errNotGroups)
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
	return &groupsExternal{client: kc, kube: c.kube}, nil
}

func (e *groupsExternal) Disconnect(_ context.Context) error { return nil }

// resolveUserID returns the Keycloak UUID for the user this Groups resource
// targets, resolving UserIdRef by reading the referenced User CR's spec
// and querying Keycloak directly.
func (e *groupsExternal) resolveUserID(ctx context.Context, realmID string, p *userv1beta1.GroupsParameters, namespace string) (string, error) {
	if p.UserId != nil && *p.UserId != "" {
		return *p.UserId, nil
	}
	if p.UserIdRef == nil || p.UserIdRef.Name == "" {
		return "", errors.New(errMissingUserRef)
	}

	ref := &userv1beta1.User{}
	if err := e.kube.Get(ctx, client.ObjectKey{Name: p.UserIdRef.Name, Namespace: namespace}, ref); err != nil {
		return "", errors.Wrap(err, errResolveUser)
	}

	u, err := e.client.GetUser(ctx, realmID, ref.Spec.ForProvider.Username)
	if err != nil {
		return "", errors.Wrap(err, errGetResolvedUser)
	}
	if u == nil {
		return "", errors.New(errResolvedUserMissing)
	}
	return u.ID, nil
}

// resolveGroupIDs returns the set of Keycloak group UUIDs this Groups
// resource targets, resolving GroupIdsRefs the same way.
func (e *groupsExternal) resolveGroupIDs(ctx context.Context, realmID string, p *userv1beta1.GroupsParameters, namespace string) ([]string, error) {
	ids := append([]string{}, p.GroupIds...)

	for _, ref := range p.GroupIdsRefs {
		if ref.Name == "" {
			continue
		}
		g := &groupv1beta1.Group{}
		if err := e.kube.Get(ctx, client.ObjectKey{Name: ref.Name, Namespace: namespace}, g); err != nil {
			return nil, errors.Wrap(err, errResolveGroup)
		}

		matches, err := e.client.SearchGroups(ctx, realmID, g.Spec.ForProvider.Name)
		if err != nil {
			return nil, errors.Wrap(err, errGetResolvedGroup)
		}

		found := false
		for i := range matches {
			if matches[i].Name == g.Spec.ForProvider.Name {
				ids = append(ids, matches[i].ID)
				found = true
				break
			}
		}
		if !found {
			return nil, errors.New(errResolvedGroupMissing)
		}
	}

	if len(ids) == 0 {
		return nil, errors.New(errMissingGroupRef)
	}

	return ids, nil
}

func (e *groupsExternal) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	_, span := tracing.StartSpan(ctx, "groups.observe",
		tracing.SpanAttrs("Groups", mg.GetName(), "observe")...)
	defer span.End()

	cr, ok := mg.(*userv1beta1.Groups)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotGroups)
	}

	realmID, err := groupsRealmID(cr)
	if err != nil {
		return managed.ExternalObservation{}, err
	}

	userUUID, err := e.resolveUserID(ctx, realmID, &cr.Spec.ForProvider, cr.GetNamespace())
	if err != nil {
		return managed.ExternalObservation{}, err
	}

	desiredGroupIDs, err := e.resolveGroupIDs(ctx, realmID, &cr.Spec.ForProvider, cr.GetNamespace())
	if err != nil {
		return managed.ExternalObservation{}, err
	}

	actual, err := e.client.GetUserGroups(ctx, realmID, userUUID)
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errGetUserGroups)
	}

	actualSet := newStringSet(groupIDSet(actual))
	desiredSet := newStringSet(desiredGroupIDs)

	upToDate := desiredSet.isSubsetOf(actualSet)
	if boolValue(cr.Spec.ForProvider.Exhaustive) {
		upToDate = desiredSet.equals(actualSet)
	}

	cr.Status.SetConditions(xpv1.Available())
	return managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: upToDate}, nil
}

func (e *groupsExternal) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	_, span := tracing.StartSpan(ctx, "groups.create",
		tracing.SpanAttrs("Groups", mg.GetName(), "create")...)
	defer span.End()

	cr, ok := mg.(*userv1beta1.Groups)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotGroups)
	}

	cr.Status.SetConditions(xpv1.Creating())
	return managed.ExternalCreation{}, e.sync(ctx, cr)
}

func (e *groupsExternal) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	_, span := tracing.StartSpan(ctx, "groups.update",
		tracing.SpanAttrs("Groups", mg.GetName(), "update")...)
	defer span.End()

	cr, ok := mg.(*userv1beta1.Groups)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotGroups)
	}

	return managed.ExternalUpdate{}, e.sync(ctx, cr)
}

// sync adds the user to any desired group they're not yet in, and — only
// when Exhaustive is true — removes membership in any group not in the
// desired set. Shared by Create and Update since group "membership" has no
// separate creation step from Keycloak's perspective, only PUT/DELETE on
// individual (user,group) pairs.
func (e *groupsExternal) sync(ctx context.Context, cr *userv1beta1.Groups) error {
	realmID, err := groupsRealmID(cr)
	if err != nil {
		return err
	}

	userUUID, err := e.resolveUserID(ctx, realmID, &cr.Spec.ForProvider, cr.GetNamespace())
	if err != nil {
		return err
	}

	desiredGroupIDs, err := e.resolveGroupIDs(ctx, realmID, &cr.Spec.ForProvider, cr.GetNamespace())
	if err != nil {
		return err
	}

	actual, err := e.client.GetUserGroups(ctx, realmID, userUUID)
	if err != nil {
		return errors.Wrap(err, errGetUserGroups)
	}

	actualSet := newStringSet(groupIDSet(actual))
	desiredSet := newStringSet(desiredGroupIDs)

	// Add user to any desired group they're not yet in
	for id := range desiredSet {
		if !actualSet[id] {
			if err := e.client.AddUserToGroup(ctx, realmID, userUUID, id); err != nil {
				return errors.Wrap(err, errAddUserToGroup)
			}
		}
	}

	// If Exhaustive, remove membership in any group not in the desired set
	if boolValue(cr.Spec.ForProvider.Exhaustive) {
		for id := range actualSet {
			if !desiredSet[id] {
				if err := e.client.RemoveUserFromGroup(ctx, realmID, userUUID, id); err != nil {
					return errors.Wrap(err, errRemoveUserGroup)
				}
			}
		}
	}

	return nil
}

func (e *groupsExternal) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	_, span := tracing.StartSpan(ctx, "groups.delete",
		tracing.SpanAttrs("Groups", mg.GetName(), "delete")...)
	defer span.End()

	cr, ok := mg.(*userv1beta1.Groups)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errNotGroups)
	}

	realmID, err := groupsRealmID(cr)
	if err != nil {
		return managed.ExternalDelete{}, err
	}

	userUUID, err := e.resolveUserID(ctx, realmID, &cr.Spec.ForProvider, cr.GetNamespace())
	if err != nil {
		// User already gone from Keycloak (or ref no longer resolvable) —
		// nothing to clean up membership-side.
		return managed.ExternalDelete{}, nil
	}

	desiredGroupIDs, err := e.resolveGroupIDs(ctx, realmID, &cr.Spec.ForProvider, cr.GetNamespace())
	if err != nil {
		return managed.ExternalDelete{}, nil
	}

	cr.Status.SetConditions(xpv1.Deleting())

	// This resource "owns" exactly the memberships it lists, regardless of
	// Exhaustive — Exhaustive only affects whether *other*, unlisted
	// memberships get touched during sync().
	for _, id := range desiredGroupIDs {
		if err := e.client.RemoveUserFromGroup(ctx, realmID, userUUID, id); err != nil && !strings.Contains(err.Error(), "404") {
			return managed.ExternalDelete{}, errors.Wrap(err, errRemoveUserGroup)
		}
	}

	return managed.ExternalDelete{}, nil
}

func groupsRealmID(cr *userv1beta1.Groups) (string, error) {
	if cr.Spec.ForProvider.RealmId == nil || *cr.Spec.ForProvider.RealmId == "" {
		return "", errors.New("realmId is required")
	}
	return *cr.Spec.ForProvider.RealmId, nil
}

func boolValue(b *bool) bool {
	return b == nil || *b
}

type stringSet map[string]bool

func newStringSet(ss []string) stringSet {
	s := make(stringSet)
	for _, v := range ss {
		s[v] = true
	}
	return s
}

func groupIDSet(gs []clients.GroupRepresentation) []string {
	var ids []string
	for i := range gs {
		ids = append(ids, gs[i].ID)
	}
	return ids
}

func (s stringSet) isSubsetOf(o stringSet) bool {
	for k := range s {
		if !o[k] {
			return false
		}
	}
	return true
}

func (s stringSet) equals(o stringSet) bool {
	if len(s) != len(o) {
		return false
	}
	for k := range s {
		if !o[k] {
			return false
		}
	}
	return true
}
