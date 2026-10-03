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

package clientcertificates

import (
	"context"
	"errors"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	clientcertificatesv1beta1 "github.com/rossigee/provider-keycloak/apis/clientcertificates/v1beta1"
	rolev1beta1 "github.com/rossigee/provider-keycloak/apis/role/v1beta1"
	"github.com/rossigee/provider-keycloak/internal/clients"
	"github.com/rossigee/provider-keycloak/internal/controller/testhelpers"
)

var errBoom = errors.New("boom")

type certStub struct {
	*testhelpers.BaseMockClient
	client    *clients.ClientRepresentation
	clientErr error
	certs     []clients.ClientCertificateRepresentation
	listErr   error
	generated *clients.ClientCertificateRepresentation
	genErr    error
	genFor    [2]string
	listCalls int
}

func (s *certStub) GetClient(context.Context, string, string) (*clients.ClientRepresentation, error) {
	if s.clientErr != nil {
		return nil, s.clientErr
	}
	if s.client == nil {
		return &clients.ClientRepresentation{ID: "cid-uuid"}, nil
	}
	return s.client, nil
}

func (s *certStub) ListClientCertificates(context.Context, string, string) ([]clients.ClientCertificateRepresentation, error) {
	s.listCalls++
	if s.listErr != nil {
		return nil, s.listErr
	}
	return s.certs, nil
}

func (s *certStub) GenerateClientCertificate(_ context.Context, _, clientID, format string) (*clients.ClientCertificateRepresentation, error) {
	if s.genErr != nil {
		return nil, s.genErr
	}
	s.genFor = [2]string{clientID, format}
	s.generated = &clients.ClientCertificateRepresentation{ID: "cert-1", Certificate: "CERT", PrivateKey: "KEY"}
	return s.generated, nil
}

func strPtr(s string) *string { return &s }

func newCertCR() *clientcertificatesv1beta1.ClientCertificate {
	return &clientcertificatesv1beta1.ClientCertificate{
		ObjectMeta: metav1.ObjectMeta{Name: "c", Namespace: "ns"},
		Spec: clientcertificatesv1beta1.ClientCertificateSpec{
			ForProvider: clientcertificatesv1beta1.ClientCertificateParameters{
				RealmId: "master", ClientId: "cid", Format: strPtr("PEM"),
			},
		},
	}
}

func newExternal(s *certStub) *external { return &external{client: s} }

// TestObserveReportsAbsentWhenParentClientGone covers the orphaned-parent case:
// a client decommissioned in Keycloak must be reported absent so the reconciler
// can release the finalizer, not handed an error it retries forever.
func TestObserveReportsAbsentWhenParentClientGone(t *testing.T) {
	for _, msg := range []string{"HTTP 404: not found", "client not found"} {
		t.Run(msg, func(t *testing.T) {
			e := newExternal(&certStub{clientErr: errors.New(msg)})

			obs, err := e.Observe(context.Background(), newCertCR())
			if err != nil {
				t.Fatalf("a missing parent client must be absence, not an error: %v", err)
			}
			if obs.ResourceExists {
				t.Fatal("a certificate whose client is gone must be reported absent")
			}
		})
	}
}

// TestObserveReportsAbsentWithNoCertificates covers a client that exists but has
// had its certificates removed out of band.
func TestObserveReportsAbsentWithNoCertificates(t *testing.T) {
	e := newExternal(&certStub{certs: nil})

	obs, err := e.Observe(context.Background(), newCertCR())
	if err != nil {
		t.Fatalf("Observe failed: %v", err)
	}
	if obs.ResourceExists {
		t.Fatal("a client with no certificates must be reported absent")
	}
}

// TestObserveRecordsLatestCertificate pins that the newest entry wins, since
// Keycloak returns them oldest first and the private key must match the cert.
func TestObserveRecordsLatestCertificate(t *testing.T) {
	e := newExternal(&certStub{certs: []clients.ClientCertificateRepresentation{
		{ID: "old", Certificate: "OLD", PrivateKey: "OLDKEY"},
		{ID: "new", Certificate: "NEW", PrivateKey: "NEWKEY"},
	}})
	cr := newCertCR()

	obs, err := e.Observe(context.Background(), cr)
	if err != nil {
		t.Fatalf("Observe failed: %v", err)
	}
	if !obs.ResourceExists {
		t.Fatal("an existing certificate must be reported present")
	}
	if cr.Status.Certificate != "NEW" || cr.Status.PrivateKey != "NEWKEY" {
		t.Errorf("status = %q/%q, want the newest entry NEW/NEWKEY",
			cr.Status.Certificate, cr.Status.PrivateKey)
	}
}

func TestObservePropagatesClientError(t *testing.T) {
	e := newExternal(&certStub{clientErr: errBoom})
	if _, err := e.Observe(context.Background(), newCertCR()); err == nil {
		t.Fatal("a transport failure must surface as an error, not as an absent certificate")
	}
}

func TestObservePropagatesListError(t *testing.T) {
	e := newExternal(&certStub{listErr: errBoom})
	if _, err := e.Observe(context.Background(), newCertCR()); err == nil {
		t.Fatal("a failed list must surface as an error")
	}
}

func TestObserveRejectsWrongType(t *testing.T) {
	e := newExternal(&certStub{})
	if _, err := e.Observe(context.Background(), &rolev1beta1.Role{}); err == nil {
		t.Fatal("Observe must reject a managed resource of the wrong type")
	}
}

func TestCreateGeneratesCertificate(t *testing.T) {
	s := &certStub{}
	e := newExternal(s)
	cr := newCertCR()

	if _, err := e.Create(context.Background(), cr); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if s.genFor != [2]string{"cid-uuid", "PEM"} {
		t.Errorf("generated against client/format %v, want the resolved client UUID and PEM", s.genFor)
	}
}

func TestCreatePropagatesClientError(t *testing.T) {
	e := newExternal(&certStub{clientErr: errBoom})
	if _, err := e.Create(context.Background(), newCertCR()); err == nil {
		t.Fatal("Create must surface a failed client lookup")
	}
}

func TestCreatePropagatesGenerateError(t *testing.T) {
	e := newExternal(&certStub{genErr: errBoom})
	if _, err := e.Create(context.Background(), newCertCR()); err == nil {
		t.Fatal("Create must surface a failed certificate generation")
	}
}

func TestCreateRejectsWrongType(t *testing.T) {
	e := newExternal(&certStub{})
	if _, err := e.Create(context.Background(), &rolev1beta1.Role{}); err == nil {
		t.Fatal("Create must reject a managed resource of the wrong type")
	}
}

// TestDeleteIsANoOp records the property the finalizer fix depends on: a
// Keycloak-issued certificate cannot be revoked, so Delete releases the
// finalizer without calling the API.
func TestDeleteIsANoOp(t *testing.T) {
	s := &certStub{clientErr: errBoom, listErr: errBoom}
	e := newExternal(s)

	if _, err := e.Delete(context.Background(), newCertCR()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if s.listCalls != 0 {
		t.Errorf("Delete called Keycloak %d times; there is nothing to revoke", s.listCalls)
	}
}

// TestDeleteIgnoresResourceType records that this controller's Delete performs no
// type assertion. It is a blanket no-op - a Keycloak-issued certificate cannot
// be revoked - so it never inspects the resource and accepts anything. That is
// deliberate, unlike its Observe and Create which both reject a wrong type.
func TestDeleteIgnoresResourceType(t *testing.T) {
	s := &certStub{clientErr: errBoom}
	e := newExternal(s)

	if _, err := e.Delete(context.Background(), &rolev1beta1.Role{}); err != nil {
		t.Errorf("Delete is a no-op and must not fail on any input, got: %v", err)
	}
	if s.listCalls != 0 {
		t.Errorf("Delete called Keycloak %d times", s.listCalls)
	}
}
