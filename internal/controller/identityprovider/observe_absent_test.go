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

package identityprovider

import (
	"context"
	"errors"
	"fmt"
	"testing"

	identityproviderv1beta1 "github.com/rossigee/provider-keycloak/apis/identityprovider/v1beta1"
	"github.com/rossigee/provider-keycloak/internal/clients"
	"github.com/rossigee/provider-keycloak/internal/controller/testhelpers"
)

// absentIdpStub embeds BaseMockClient, which implements the whole
// clients.Client interface, and overrides only the lookup under test.
type absentIdpStub struct {
	*testhelpers.BaseMockClient
	getFn func(ctx context.Context, realm, alias string) (*clients.IdentityProviderRepresentation, error)
}

func (s *absentIdpStub) GetIdentityProvider(ctx context.Context, realm, alias string) (*clients.IdentityProviderRepresentation, error) {
	return s.getFn(ctx, realm, alias)
}

// notFoundError mimics what the Keycloak client returns for HTTP 404: an error
// whose message keeps the "404" substring but which unwraps to ErrNotFound.
type notFoundError struct{}

func (notFoundError) Error() string {
	return `request failed with status 404: {"error":"HTTP 404 Not Found"}`
}

func (notFoundError) Unwrap() error { return clients.ErrNotFound }

func newIdpCR() *identityproviderv1beta1.IdentityProvider {
	realm := "ROSSGolderLtd"

	cr := &identityproviderv1beta1.IdentityProvider{}
	cr.SetName("rossgolderltd-timewarp")
	cr.SetNamespace("rossgolderltd")
	cr.Spec.ForProvider.RealmId = realm
	cr.Spec.ForProvider.Alias = "timewarp"

	return cr
}

// TestObserveAbsentIdentityProviderIsNotAnError is the regression test for the
// create-direction twin of the orphaned-parent wedge.
//
// GetIdentityProvider propagated Keycloak's 404 as a plain error. The managed
// reconciler aborts a reconcile when Observe errors, so it never reached Create -
// and the first Observe of any new IdentityProvider always 404s. Both live
// IdentityProviders were 18 days old and had never been created.
func TestObserveAbsentIdentityProviderIsNotAnError(t *testing.T) {
	mc := &absentIdpStub{
		BaseMockClient: &testhelpers.BaseMockClient{},
		getFn: func(_ context.Context, _, _ string) (*clients.IdentityProviderRepresentation, error) {
			return nil, fmt.Errorf("wrapping: %w", notFoundError{})
		},
	}

	e := &external{client: mc}

	obs, err := e.Observe(context.Background(), newIdpCR())
	if err != nil {
		t.Fatalf("an absent identity provider must not abort Observe: %v", err)
	}
	if obs.ResourceExists {
		t.Error("expected ResourceExists=false so the reconciler proceeds to Create")
	}
}

// TestObserveIdpTransportFailureStillErrors pins the distinction. Reporting the
// resource absent on a transport failure would send the reconciler to Create on
// every poll, duplicating or fighting with the object that already exists.
func TestObserveIdpTransportFailureStillErrors(t *testing.T) {
	mc := &absentIdpStub{
		BaseMockClient: &testhelpers.BaseMockClient{},
		getFn: func(_ context.Context, _, _ string) (*clients.IdentityProviderRepresentation, error) {
			return nil, errors.New("connection refused")
		},
	}

	e := &external{client: mc}

	if _, err := e.Observe(context.Background(), newIdpCR()); err == nil {
		t.Fatal("a transport failure must still surface as an error")
	}
}
