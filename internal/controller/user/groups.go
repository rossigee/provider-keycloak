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
	"sort"
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
	errNotGroups              = "managed resource is not a Groups"
	errResolveUser            = "cannot resolve referenced User"
	errResolveGroup           = "cannot resolve referenced Group"
	errMissingUserRef         = "userId or userIdRef must be set"
	errMissingGroupRef        = "at least one of groupIds or groupIdsRefs must be set"
	errGetUserGroups          = "cannot get Keycloak user's group memberships"
	errAddUserToGroup         = "cannot add user to Keycloak group"
	errRemoveUserGroup        = "cannot remove user from Keycloak group"
	errGetResolvedUser        = "cannot get resolved user from Keycloak"
	errGetResolvedGroup       = "cannot get resolved group from Keycloak"
	errResolvedUserMissing    = "referenced user not yet present in Keycloak"
	errResolvedGroupMissing   = "referenced group not yet present in Keycloak"
	errListGroups             = "cannot list Groups resources to look for a conflicting membership owner"
	errMembershipConflict     = "Groups resource(s) %s also manage the complete group membership for this user: Exhaustive defaults to true, so each resource deletes the memberships the others declare. Merge them into a single resource, or set exhaustive: false on all but one"
	msgMembershipNotConverged = "Keycloak group membership does not yet match the desired state"

	// reasonMembershipConflict is reported when another Groups resource also
	// claims exhaustive ownership of the same user's group membership.
	reasonMembershipConflict = event.Reason("MembershipConflict")

	groupsControllerName = "groups.user.keycloak.m.crossplane.io"
)

// SetupGroups registers the Groups (user↔group membership) controller.
func SetupGroups(mgr ctrl.Manager, o xpcontroller.Options) error {
	recorder := event.NewAPIRecorder(mgr.GetEventRecorder(groupsControllerName))
	opts := []managed.ReconcilerOption{
		managed.WithExternalConnector(&groupsConnector{kube: mgr.GetClient(), recorder: recorder}),
		managed.WithLogger(o.Logger.WithValues("controller", "Groups")),
		managed.WithRecorder(recorder),
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

type groupsConnector struct {
	kube     client.Client
	recorder event.Recorder
}

type groupsExternal struct {
	client   clients.Client
	kube     client.Client
	recorder event.Recorder
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
	return &groupsExternal{client: kc, kube: c.kube, recorder: c.recorder}, nil
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

// conflictingMembershipOwners returns the names of the other Groups resources
// in the same namespace that also claim exhaustive ownership of the same
// Keycloak user's group membership.
//
// Exhaustive defaults to true, which makes a Groups resource authoritative for
// the complete set of a user's memberships: sync removes every group not
// listed. Two exhaustive resources for one user therefore delete each other's
// memberships on every reconcile. That failure is invisible from either
// resource - both keep reporting Synced and Ready - while the user's
// membership oscillates, so detect it here instead of letting them fight.
func (e *groupsExternal) conflictingMembershipOwners(ctx context.Context, cr *userv1beta1.Groups, realmID, userUUID string) ([]string, error) {
	l := &userv1beta1.GroupsList{}
	if err := e.kube.List(ctx, l, client.InNamespace(cr.GetNamespace())); err != nil {
		return nil, errors.Wrap(err, errListGroups)
	}

	var conflicts []string
	for i := range l.Items {
		other := &l.Items[i]

		switch {
		case other.GetName() == cr.GetName(),
			other.GetDeletionTimestamp() != nil,
			!boolValue(other.Spec.ForProvider.Exhaustive):
			continue
		}

		otherRealm, err := groupsRealmID(other)
		if err != nil || otherRealm != realmID {
			continue
		}

		// A sibling whose user cannot be resolved yet is not evidence of a
		// conflict, so don't report it as one.
		otherUUID, err := e.resolveUserID(ctx, realmID, &other.Spec.ForProvider, other.GetNamespace())
		if err != nil {
			continue
		}

		if otherUUID == userUUID {
			conflicts = append(conflicts, other.GetName())
		}
	}

	// Sorted so the reported message is stable and the resulting Kubernetes
	// event aggregates instead of producing a new one on every reconcile.
	sort.Strings(conflicts)

	return conflicts, nil
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

	conflicts, err := e.conflictingMembershipOwners(ctx, cr, realmID, userUUID)
	if err != nil {
		return managed.ExternalObservation{}, err
	}
	if len(conflicts) > 0 {
		// Returning an error stops the reconciler before Update, so this
		// resource performs no membership changes while the configuration is
		// ambiguous.
		err := errors.Errorf(errMembershipConflict, strings.Join(conflicts, ", "))
		cr.Status.SetConditions(xpv1.Unavailable().WithMessage(err.Error()))
		if e.recorder != nil {
			e.recorder.Event(cr, event.Warning(reasonMembershipConflict, err))
		}
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

	// Only claim Available once the observed membership matches what this
	// resource declares. Reporting Available while sync is about to rewrite the
	// membership hides the exact state an operator needs to see.
	if upToDate {
		cr.Status.SetConditions(xpv1.Available())
	} else {
		cr.Status.SetConditions(xpv1.Unavailable().WithMessage(msgMembershipNotConverged))
	}

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
