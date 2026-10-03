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

package protocolmapper

import (
	"context"
	"errors"
	"testing"

	clientv1beta1 "github.com/rossigee/provider-keycloak/apis/client/v1beta1"
	"github.com/rossigee/provider-keycloak/internal/clients"
	"github.com/rossigee/provider-keycloak/internal/controller/testhelpers"
)

func newObserveTestCR(t *testing.T, clientID string) *clientv1beta1.ProtocolMapper {
	t.Helper()

	realm := "ROSSGolderLtd"

	cr := &clientv1beta1.ProtocolMapper{}
	cr.SetName("mapper")
	cr.SetNamespace("rossgolderltd")
	cr.Spec.ForProvider.RealmId = &realm
	cr.Spec.ForProvider.ClientId = &clientID
	cr.Spec.ForProvider.Name = "email"

	return cr
}

// TestObserveMissingClientReportsGone is the regression test for a second,
// distinct way a managed resource can never terminate.
//
// The reconciler aborts a reconcile when Observe returns an error, so it never
// reaches RemoveFinalizer. When a mapper's parent client has been decommissioned
// in Keycloak, resolveClientUUID fails permanently - so a mapper left behind
// holds its finalizer forever and re-issues the lookup on every poll.
//
// Three such resources were found terminating since 2026-10-01 in the
// ROSSGolderLtd realm, failing with `client "k8s-bankrut-master" not found`.
// Reporting the external resource as gone is what lets the reconciler finalise.
func TestObserveMissingClientReportsGone(t *testing.T) {
	cr := newObserveTestCR(t, "decommissioned-client")

	// Keycloak knows nothing about the client: GetClient returns nil, nil.
	mc := &mockMapperClient{
		BaseMockClient: &testhelpers.BaseMockClient{},
		getClientFn: func(_ context.Context, _, _ string) (*clients.ClientRepresentation, error) {
			return nil, nil
		},
	}

	e := &external{kc: mc}

	obs, err := e.Observe(context.Background(), cr)
	if err != nil {
		t.Fatalf("a missing parent client must not abort Observe: %v", err)
	}
	if obs.ResourceExists {
		t.Error("a mapper cannot exist without its client; expected ResourceExists=false")
	}
}

// TestObserveClientLookupFailureStillErrors guards the distinction the fix
// rests on: a failure to *reach* Keycloak is still an error and must not be
// mistaken for the client being absent. Reporting the resource gone there would
// silently drop a mapper that still exists.
func TestObserveClientLookupFailureStillErrors(t *testing.T) {
	cr := newObserveTestCR(t, "some-client")

	mc := &mockMapperClient{
		BaseMockClient: &testhelpers.BaseMockClient{},
		getClientFn: func(_ context.Context, _, _ string) (*clients.ClientRepresentation, error) {
			return nil, errors.New("connection refused")
		},
	}

	e := &external{kc: mc}

	if _, err := e.Observe(context.Background(), cr); err == nil {
		t.Fatal("a transport failure must still surface as an error")
	}
}
