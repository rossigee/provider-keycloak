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
	"errors"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	realmv1beta1 "github.com/rossigee/provider-keycloak/apis/realm/v1beta1"
	rolev1beta1 "github.com/rossigee/provider-keycloak/apis/role/v1beta1"
	"github.com/rossigee/provider-keycloak/internal/controller/testhelpers"
)

var errBoom = errors.New("boom")

type realmStub struct {
	*testhelpers.BaseMockClient
	raw        []byte
	rawErr     error
	updateErr  error
	updatedRaw []byte
	updateCall int
}

func (s *realmStub) GetRawRealm(context.Context, string) ([]byte, error) {
	if s.rawErr != nil {
		return nil, s.rawErr
	}
	return s.raw, nil
}

func (s *realmStub) UpdateRealmRaw(_ context.Context, _ string, raw []byte) error {
	s.updateCall++
	s.updatedRaw = raw
	return s.updateErr
}

func strPtr(s string) *string { return &s }
func boolPtr(b bool) *bool    { return &b }

func newCR() *realmv1beta1.Realm {
	return &realmv1beta1.Realm{
		ObjectMeta: metav1.ObjectMeta{Name: "r", Namespace: "ns"},
		Spec: realmv1beta1.RealmSpec{
			ForProvider: realmv1beta1.RealmParameters{Realm: "master"},
		},
	}
}

func i64Ptr(i int64) *int64 { return &i }

func newExternal(s *realmStub) *external {
	scheme := runtime.NewScheme()
	_ = realmv1beta1.AddToScheme(scheme)
	return &external{
		client: s,
		kube:   fake.NewClientBuilder().WithScheme(scheme).WithObjects(newCR()).Build(),
	}
}

func written(t *testing.T, s *realmStub) map[string]interface{} {
	t.Helper()
	if s.updatedRaw == nil {
		t.Fatal("Update did not write anything back to Keycloak")
	}
	var m map[string]interface{}
	if err := json.Unmarshal(s.updatedRaw, &m); err != nil {
		t.Fatalf("written realm is not valid JSON: %v", err)
	}
	return m
}

// TestUpdatePreservesFieldsTheCRDDoesNotModel is the point of Update's
// read-modify-write. Keycloak realms carry far more configuration than the CRD
// models, so Update fetches the live realm as a map, overlays only the fields
// the spec actually sets, and writes the whole thing back. Decoding into the
// struct instead would silently drop everything this provider does not
// understand on every reconcile.
func TestUpdatePreservesFieldsTheCRDDoesNotModel(t *testing.T) {
	s := &realmStub{raw: []byte(`{
		"realm": "master",
		"displayName": "old",
		"someKeycloakKnobWeDoNotModel": {"nested": true},
		"anotherUnknownField": "keep me",
		"browserSecurityHeaders": {"xFrameOptions": "DENY"}
	}`)}
	e := newExternal(s)

	cr := newCR()
	cr.Spec.ForProvider.DisplayName = strPtr("new")

	if _, err := e.Update(context.Background(), cr); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	got := written(t, s)
	for _, key := range []string{"someKeycloakKnobWeDoNotModel", "anotherUnknownField", "browserSecurityHeaders"} {
		if _, ok := got[key]; !ok {
			t.Errorf("Update dropped %q, which the CRD does not model:\n%s", key, s.updatedRaw)
		}
	}
	if got["displayName"] != "new" {
		t.Errorf("displayName = %v, want the spec's value", got["displayName"])
	}
}

// TestUpdateLeavesUnsetSpecFieldsAlone covers the other half: a field the spec
// does not set must keep whatever Keycloak has, rather than being zeroed.
func TestUpdateLeavesUnsetSpecFieldsAlone(t *testing.T) {
	s := &realmStub{raw: []byte(`{
		"realm": "master",
		"displayName": "keep me",
		"registrationAllowed": true,
		"rememberMe": true
	}`)}
	e := newExternal(s)

	if _, err := e.Update(context.Background(), newCR()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	got := written(t, s)
	for key, want := range map[string]interface{}{
		"displayName":         "keep me",
		"registrationAllowed": true,
		"rememberMe":          true,
	} {
		if got[key] != want {
			t.Errorf("%s = %v, want the existing %v; an unset spec field must not overwrite it", key, got[key], want)
		}
	}
}

func TestUpdateAppliesEachFieldKind(t *testing.T) {
	s := &realmStub{raw: []byte(`{"realm":"master"}`)}
	e := newExternal(s)

	cr := newCR()
	cr.Spec.ForProvider.DisplayName = strPtr("My Realm")
	cr.Spec.ForProvider.SslRequired = strPtr("external")
	cr.Spec.ForProvider.RegistrationAllowed = boolPtr(true)
	cr.Spec.ForProvider.RefreshTokenMaxReuse = i64Ptr(7)
	// A duration in the spec must be converted to the seconds Keycloak wants.
	cr.Spec.ForProvider.AccessTokenLifespan = strPtr("5m0s")
	cr.Spec.ForProvider.SsoSessionIdleTimeout = strPtr("30m0s")

	if _, err := e.Update(context.Background(), cr); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	got := written(t, s)
	for key, want := range map[string]interface{}{
		"displayName":           "My Realm",
		"sslRequired":           "external",
		"registrationAllowed":   true,
		"refreshTokenMaxReuse":  float64(7),
		"accessTokenLifespan":   float64(300),
		"ssoSessionIdleTimeout": float64(1800),
	} {
		if got[key] != want {
			t.Errorf("%s = %v (%T), want %v (%T)", key, got[key], got[key], want, want)
		}
	}
}

func TestUpdatePropagatesFetchError(t *testing.T) {
	e := newExternal(&realmStub{rawErr: errBoom})
	cr := newCR()

	if _, err := e.Update(context.Background(), cr); err == nil {
		t.Fatal("Update must surface a failed fetch of the current realm")
	}
}

func TestUpdateRejectsUnparseableRealm(t *testing.T) {
	s := &realmStub{raw: []byte("this is not json")}
	e := newExternal(s)

	if _, err := e.Update(context.Background(), newCR()); err == nil {
		t.Fatal("Update must surface an unparseable realm rather than writing garbage back")
	}
	if s.updateCall != 0 {
		t.Error("Update wrote to Keycloak despite failing to parse the current realm")
	}
}

func TestUpdatePropagatesWriteError(t *testing.T) {
	s := &realmStub{raw: []byte(`{"realm":"master"}`), updateErr: errBoom}
	e := newExternal(s)

	if _, err := e.Update(context.Background(), newCR()); err == nil {
		t.Fatal("Update must surface a failed write")
	}
}

func TestUpdateRejectsWrongType(t *testing.T) {
	e := newExternal(&realmStub{})
	if _, err := e.Update(context.Background(), &rolev1beta1.Role{}); err == nil {
		t.Fatal("Update must reject a managed resource of the wrong type")
	}
}
