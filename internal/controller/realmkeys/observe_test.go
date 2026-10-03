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
	"errors"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
	keysv1beta1 "github.com/rossigee/provider-keycloak/apis/keys/v1beta1"
	rolev1beta1 "github.com/rossigee/provider-keycloak/apis/role/v1beta1"
	"github.com/rossigee/provider-keycloak/internal/clients"
	"github.com/rossigee/provider-keycloak/internal/controller/testhelpers"
)

type keysStub struct {
	*testhelpers.BaseMockClient
	keys    *clients.RealmKeysRepresentation
	getErr  error
	getCall int
}

func (s *keysStub) GetRealmKeys(context.Context, string) (*clients.RealmKeysRepresentation, error) {
	s.getCall++
	if s.getErr != nil {
		return nil, s.getErr
	}
	return s.keys, nil
}

func newKeysCR() *keysv1beta1.RealmKeys {
	return &keysv1beta1.RealmKeys{
		ObjectMeta: metav1.ObjectMeta{Name: "keys", Namespace: "ns"},
		Spec: keysv1beta1.RealmKeysSpec{
			ForProvider: keysv1beta1.RealmKeysParameters{RealmId: "master"},
		},
	}
}

func newExternal(s *keysStub) *external {
	return &external{client: s}
}

// TestObserveReportsAbsentWhenDeleting is the behavioural counterpart to the
// structural guard in deletecomplete. This controller's Delete is a no-op, so
// once deletion is requested there is nothing left in Keycloak to release and
// Observe must say so - otherwise the reconciler re-runs Delete forever and the
// finalizer is never removed.
func TestObserveReportsAbsentWhenDeleting(t *testing.T) {
	s := &keysStub{keys: &clients.RealmKeysRepresentation{}}
	e := newExternal(s)

	cr := newKeysCR()
	now := metav1.NewTime(time.Now())
	cr.SetDeletionTimestamp(&now)

	obs, err := e.Observe(context.Background(), cr)
	if err != nil {
		t.Fatalf("Observe failed: %v", err)
	}
	if obs.ResourceExists {
		t.Fatal("a resource being deleted with a no-op Delete must be reported absent, " +
			"or the reconciler re-runs Delete on every pass and never removes the finalizer")
	}
	if s.getCall != 0 {
		t.Errorf("Keycloak was queried %d times for a resource being deleted; it should not need to be", s.getCall)
	}
}

func TestObserveReportsPresentAndRecordsKeys(t *testing.T) {
	s := &keysStub{keys: &clients.RealmKeysRepresentation{
		Keys: []clients.KeyInfoRepresentation{
			{Kid: "kid-1", Algorithm: "RS256", Status: "active"},
		},
	}}
	e := newExternal(s)
	cr := newKeysCR()

	obs, err := e.Observe(context.Background(), cr)
	if err != nil {
		t.Fatalf("Observe failed: %v", err)
	}
	if !obs.ResourceExists {
		t.Fatal("a live realm's keys must be reported present")
	}
	if len(cr.Status.Keys) != 1 || cr.Status.Keys[0].Kid != "kid-1" {
		t.Errorf("status keys = %+v, want the key returned by Keycloak", cr.Status.Keys)
	}
	if c := cr.Status.GetCondition(xpv1.TypeReady); c.Status != "True" {
		t.Errorf("Ready condition = %+v, want True", c)
	}
}

// TestObservePropagatesGetError keeps a transport failure distinct from an
// absent realm: reporting absent on error would send the reconciler to Create
// on every poll.
func TestObservePropagatesGetError(t *testing.T) {
	s := &keysStub{getErr: errBoom}
	e := newExternal(s)

	obs, err := e.Observe(context.Background(), newKeysCR())
	if err == nil {
		t.Fatal("a failed lookup must surface as an error, not as an absent resource")
	}
	if obs.ResourceExists {
		t.Error("a failed lookup must not report the resource absent")
	}
}

func TestObserveRejectsWrongType(t *testing.T) {
	e := newExternal(&keysStub{})
	if _, err := e.Observe(context.Background(), &rolev1beta1.Role{}); err == nil {
		t.Fatal("Observe must reject a managed resource of the wrong type")
	}
}

// TestDeleteIsANoOp records the property the finalizer fix depends on: there is
// nothing in Keycloak to release, so Delete must not call anything.
func TestDeleteIsANoOp(t *testing.T) {
	s := &keysStub{getErr: errBoom}
	e := newExternal(s)
	cr := newKeysCR()

	if _, err := e.Delete(context.Background(), cr); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if s.getCall != 0 {
		t.Errorf("Delete called the API %d times; it has nothing to release", s.getCall)
	}
	if c := cr.Status.GetCondition(xpv1.TypeReady); c.Status != "False" || c.Reason != "Deleting" {
		t.Errorf("Deleting condition = %+v, want False/Deleting", c)
	}
}

var errBoom = errors.New("boom")
