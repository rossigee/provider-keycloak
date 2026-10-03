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

package realmkeys

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
	keysv1beta1 "github.com/rossigee/provider-keycloak/apis/keys/v1beta1"
	realmv1beta1 "github.com/rossigee/provider-keycloak/apis/realm/v1beta1"
	"github.com/rossigee/provider-keycloak/internal/clients"
)

func keysCR(realm string) *keysv1beta1.RealmKeys {
	return &keysv1beta1.RealmKeys{
		ObjectMeta: metav1.ObjectMeta{Name: "realm-keys"},
		Spec: keysv1beta1.RealmKeysSpec{
			ForProvider: keysv1beta1.RealmKeysParameters{RealmId: realm},
		},
	}
}

// RealmKeys is a read-and-record resource: Keycloak owns the realm's keys and
// there is nothing this controller can create or change. Create and Update are
// therefore deliberate no-ops, and saying so in a test is the only thing that
// stops a future reader "fixing" them into real mutations.
func TestCreateIsANoOpThatStillSetsCreating(t *testing.T) {
	e := &external{}
	cr := keysCR("master")

	if _, err := e.Create(context.Background(), cr); err != nil {
		t.Fatalf("Create returned %v, want nil - there is nothing to create in Keycloak", err)
	}
	// xpv1.Creating() marks the resource not-yet-ready. Asserting the Reason as
	// well as the Status matters: Creating and Deleting both report Ready=False,
	// so the Status alone cannot tell "just created" from "on its way out".
	cond := cr.Status.GetCondition(xpv1.TypeReady)
	if cond.Status != corev1.ConditionFalse {
		t.Errorf("status = %q, want %q while creating", cond.Status, corev1.ConditionFalse)
	}
	if cond.Reason != xpv1.ReasonCreating {
		t.Errorf("reason = %q, want %q - Creating and Deleting share Ready=False, "+
			"so the reason is what distinguishes them", cond.Reason, xpv1.ReasonCreating)
	}
}

func TestCreateRejectsWrongType(t *testing.T) {
	e := &external{}
	if _, err := e.Create(context.Background(), &realmv1beta1.Realm{}); err == nil {
		t.Error("Create accepted an unrelated managed type")
	}
}

func TestUpdateIsANoOp(t *testing.T) {
	e := &external{}
	cr := keysCR("master")

	// Update never runs in practice, because Observe always reports
	// ResourceUpToDate. If that ever changes, Update still has to succeed
	// without touching Keycloak, or the resource would loop.
	if _, err := e.Update(context.Background(), cr); err != nil {
		t.Fatalf("Update returned %v, want nil - realm keys cannot be updated", err)
	}
}

// Observe always reports up-to-date, which is what keeps the no-op Update from
// ever being reached. Asserted explicitly because flipping that flag is what
// would turn this into an infinite reconcile.
func TestObserveAlwaysReportsUpToDate(t *testing.T) {
	e := &external{client: &keysStub{keys: &clients.RealmKeysRepresentation{}}}

	obs, err := e.Observe(context.Background(), keysCR("master"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !obs.ResourceExists {
		t.Error("ResourceExists = false, want true for a live realm")
	}
	if !obs.ResourceUpToDate {
		t.Error("ResourceUpToDate = false; Observe must always report up-to-date so the " +
			"no-op Update is never driven, since Keycloak owns the realm keys")
	}
}
