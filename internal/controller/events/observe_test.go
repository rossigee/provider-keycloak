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

package events

import (
	"context"
	"errors"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
	eventv1beta1 "github.com/rossigee/provider-keycloak/apis/events/v1beta1"
	rolev1beta1 "github.com/rossigee/provider-keycloak/apis/role/v1beta1"
	"github.com/rossigee/provider-keycloak/internal/clients"
	"github.com/rossigee/provider-keycloak/internal/controller/testhelpers"
)

var errBoom = errors.New("boom")

type eventsStub struct {
	*testhelpers.BaseMockClient
	config       *clients.RealmEventsConfigRepresentation
	getErr       error
	updateErr    error
	getCalls     int
	updateCalls  int
	gotUpdated   *clients.RealmEventsConfigRepresentation
	gotUpdateFor string
}

func (s *eventsStub) GetRealmEventsConfig(context.Context, string) (*clients.RealmEventsConfigRepresentation, error) {
	s.getCalls++
	if s.getErr != nil {
		return nil, s.getErr
	}
	return s.config, nil
}

func (s *eventsStub) UpdateRealmEventsConfig(_ context.Context, realm string, rep *clients.RealmEventsConfigRepresentation) error {
	s.updateCalls++
	s.gotUpdated = rep
	s.gotUpdateFor = realm
	return s.updateErr
}

func boolPtr(b bool) *bool { return &b }

func newEventsCR() *eventv1beta1.RealmEventsConfig {
	return &eventv1beta1.RealmEventsConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "ev", Namespace: "ns"},
		Spec: eventv1beta1.RealmEventsConfigSpec{
			ForProvider: eventv1beta1.RealmEventsConfigParameters{
				RealmId:            "master",
				EventsEnabled:      boolPtr(true),
				AdminEventsEnabled: boolPtr(false),
			},
		},
	}
}

func newExternal(s *eventsStub) *external { return &external{client: s} }

// TestObserveReportsAbsentWhenDeleting is the behavioural counterpart to the
// structural guard in deletecomplete. An events config has nothing to release,
// so once deletion is requested Observe must say so, or the reconciler re-runs
// Delete forever and the finalizer is never removed.
func TestObserveReportsAbsentWhenDeleting(t *testing.T) {
	s := &eventsStub{config: &clients.RealmEventsConfigRepresentation{EventsEnabled: boolPtr(true)}}
	e := newExternal(s)

	cr := newEventsCR()
	now := metav1.NewTime(time.Now())
	cr.SetDeletionTimestamp(&now)

	obs, err := e.Observe(context.Background(), cr)
	if err != nil {
		t.Fatalf("Observe failed: %v", err)
	}
	if obs.ResourceExists {
		t.Fatal("a resource being deleted with a no-op Delete must be reported absent, " +
			"or the finalizer is never removed")
	}
	if s.getCalls != 0 {
		t.Errorf("Keycloak was queried %d times for a resource being deleted", s.getCalls)
	}
}

func TestObserveReportsUpToDate(t *testing.T) {
	s := &eventsStub{config: &clients.RealmEventsConfigRepresentation{
		EventsEnabled: boolPtr(true), AdminEventsEnabled: boolPtr(false),
	}}
	e := newExternal(s)
	cr := newEventsCR()

	obs, err := e.Observe(context.Background(), cr)
	if err != nil {
		t.Fatalf("Observe failed: %v", err)
	}
	if !obs.ResourceExists {
		t.Fatal("a live events config must be reported present")
	}
	if !obs.ResourceUpToDate {
		t.Error("a config matching the spec must be reported up to date")
	}
	if c := cr.Status.GetCondition(xpv1.TypeReady); c.Status != "True" {
		t.Errorf("Ready condition = %+v, want True", c)
	}
}

func TestObserveDetectsDrift(t *testing.T) {
	s := &eventsStub{config: &clients.RealmEventsConfigRepresentation{
		EventsEnabled: boolPtr(false), AdminEventsEnabled: boolPtr(false),
	}}
	e := newExternal(s)

	obs, err := e.Observe(context.Background(), newEventsCR())
	if err != nil {
		t.Fatalf("Observe failed: %v", err)
	}
	if obs.ResourceUpToDate {
		t.Error("a config disabled in Keycloak but enabled in the spec must be reported out of date")
	}
}

// TestObservePropagatesGetError keeps a transport failure distinct from an
// absent config: reporting absent on error would send the reconciler to Create
// on every poll.
func TestObservePropagatesGetError(t *testing.T) {
	e := newExternal(&eventsStub{getErr: errBoom})

	obs, err := e.Observe(context.Background(), newEventsCR())
	if err == nil {
		t.Fatal("a failed lookup must surface as an error, not as an absent resource")
	}
	if obs.ResourceExists {
		t.Error("a failed lookup must not report the resource absent")
	}
}

func TestObserveRejectsWrongType(t *testing.T) {
	e := newExternal(&eventsStub{})
	if _, err := e.Observe(context.Background(), &rolev1beta1.Role{}); err == nil {
		t.Fatal("Observe must reject a managed resource of the wrong type")
	}
}

func TestCreateWritesConfig(t *testing.T) {
	s := &eventsStub{}
	e := newExternal(s)

	if _, err := e.Create(context.Background(), newEventsCR()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if s.updateCalls != 1 {
		t.Fatalf("UpdateRealmEventsConfig called %d times, want 1", s.updateCalls)
	}
	if s.gotUpdateFor != "master" {
		t.Errorf("updated realm = %q, want master", s.gotUpdateFor)
	}
	if s.gotUpdated == nil || s.gotUpdated.EventsEnabled == nil || !*s.gotUpdated.EventsEnabled {
		t.Errorf("written config = %+v, want EventsEnabled true", s.gotUpdated)
	}
}

func TestCreatePropagatesError(t *testing.T) {
	e := newExternal(&eventsStub{updateErr: errBoom})
	if _, err := e.Create(context.Background(), newEventsCR()); err == nil {
		t.Fatal("Create must surface a write failure")
	}
}

func TestUpdateWritesConfig(t *testing.T) {
	s := &eventsStub{}
	e := newExternal(s)

	if _, err := e.Update(context.Background(), newEventsCR()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if s.updateCalls != 1 || s.gotUpdated == nil {
		t.Errorf("Update did not write the config (calls=%d)", s.updateCalls)
	}
}

func TestUpdatePropagatesError(t *testing.T) {
	e := newExternal(&eventsStub{updateErr: errBoom})
	if _, err := e.Update(context.Background(), newEventsCR()); err == nil {
		t.Fatal("Update must surface a write failure")
	}
}

// TestDeleteIsANoOp records the property the finalizer fix relies on.
func TestDeleteIsANoOp(t *testing.T) {
	s := &eventsStub{updateErr: errBoom}
	e := newExternal(s)

	if _, err := e.Delete(context.Background(), newEventsCR()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if s.updateCalls != 0 {
		t.Errorf("Delete wrote to Keycloak %d times; there is nothing to release", s.updateCalls)
	}
}

func TestRejectsWrongTypeOnWritePaths(t *testing.T) {
	e := newExternal(&eventsStub{})
	if _, err := e.Create(context.Background(), &rolev1beta1.Role{}); err == nil {
		t.Error("Create must reject a managed resource of the wrong type")
	}
	if _, err := e.Update(context.Background(), &rolev1beta1.Role{}); err == nil {
		t.Error("Update must reject a managed resource of the wrong type")
	}
}

func TestDisconnectSucceeds(t *testing.T) {
	e := newExternal(&eventsStub{})
	if err := e.Disconnect(context.Background()); err != nil {
		t.Errorf("Disconnect = %v, want nil", err)
	}
}
