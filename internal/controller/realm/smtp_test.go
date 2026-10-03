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
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	realmv1beta1 "github.com/rossigee/provider-keycloak/apis/realm/v1beta1"
)

func sp(s string) *string { return &s }
func bp(b bool) *bool     { return &b }

// smtpSecret builds a kube client holding the referenced SMTP password secret.
func smtpSecret(t *testing.T, data map[string][]byte) client.Client {
	t.Helper()
	sch := runtime.NewScheme()
	if err := corev1.AddToScheme(sch); err != nil {
		t.Fatalf("scheme: %v", err)
	}
	return fake.NewClientBuilder().WithScheme(sch).WithObjects(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "smtp-creds", Namespace: "keycloak"},
		Data:       data,
	}).Build()
}

// emptyKube returns a client with no secrets at all.
func emptyKube(t *testing.T) client.Client {
	t.Helper()
	sch := runtime.NewScheme()
	if err := corev1.AddToScheme(sch); err != nil {
		t.Fatalf("scheme: %v", err)
	}
	return fake.NewClientBuilder().WithScheme(sch).Build()
}

// The SMTP password arrives via a Secret and is flattened into the
// map[string]string shape Keycloak expects. Getting the resolution wrong either
// fails the reconcile or silently ships an unauthenticated mail config.
func TestBuildSmtpServerMapResolvesPassword(t *testing.T) {
	pwRef := &realmv1beta1.SmtpPasswordSecretRef{Name: "smtp-creds", Namespace: "keycloak", Key: "password"}

	t.Run("reads the password and enables auth", func(t *testing.T) {
		p := &realmv1beta1.SmtpServer{
			Host: sp("smtp.example.com"),
			Auth: []realmv1beta1.SmtpServerAuth{{
				Username:          sp("mailer"),
				PasswordSecretRef: pwRef,
			}},
		}
		kube := smtpSecret(t, map[string][]byte{"password": []byte("hunter2")})

		m, err := buildSmtpServerMap(context.Background(), kube, p)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if m["password"] != "hunter2" {
			t.Errorf("password = %q, want %q", m["password"], "hunter2")
		}
		if m["user"] != "mailer" {
			t.Errorf("user = %q, want %q", m["user"], "mailer")
		}
		if m["auth"] != "true" {
			t.Errorf("auth = %q, want %q", m["auth"], "true")
		}
		if m["host"] != "smtp.example.com" {
			t.Errorf("host = %q, want %q", m["host"], "smtp.example.com")
		}
	})

	t.Run("missing secret key is an error", func(t *testing.T) {
		p := &realmv1beta1.SmtpServer{
			Auth: []realmv1beta1.SmtpServerAuth{{PasswordSecretRef: pwRef}},
		}
		kube := smtpSecret(t, map[string][]byte{"other": []byte("x")})

		if _, err := buildSmtpServerMap(context.Background(), kube, p); err == nil {
			t.Fatal("expected an error when the referenced key is absent")
		}
	})

	t.Run("missing secret is an error", func(t *testing.T) {
		p := &realmv1beta1.SmtpServer{
			Auth: []realmv1beta1.SmtpServerAuth{{PasswordSecretRef: pwRef}},
		}
		if _, err := buildSmtpServerMap(context.Background(), emptyKube(t), p); err == nil {
			t.Fatal("expected an error when the secret does not exist")
		}
	})

	// Without a kube client the password cannot be resolved. Returning a map
	// with auth unset here would quietly ship an SMTP config that silently fails
	// to authenticate, so it has to be an error.
	t.Run("nil kube client is an error", func(t *testing.T) {
		p := &realmv1beta1.SmtpServer{
			Auth: []realmv1beta1.SmtpServerAuth{{PasswordSecretRef: pwRef}},
		}
		if _, err := buildSmtpServerMap(context.Background(), nil, p); err == nil {
			t.Fatal("expected an error when the kube client is unavailable")
		}
	})

	// auth is only turned on when credentials were actually resolved. Sending
	// auth=true with no credentials makes Keycloak attempt authentication it
	// cannot complete.
	t.Run("auth stays off without a password ref", func(t *testing.T) {
		p := &realmv1beta1.SmtpServer{Host: sp("smtp.example.com")}
		m, err := buildSmtpServerMap(context.Background(), nil, p)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, ok := m["auth"]; ok {
			t.Errorf("auth = %q, want absent when no credentials are configured", m["auth"])
		}
		if _, ok := m["password"]; ok {
			t.Error("password key present with no credentials configured")
		}
	})
}

func TestBuildSmtpServerMapFields(t *testing.T) {
	t.Run("maps every field Keycloak expects", func(t *testing.T) {
		m := buildSmtpServerMapFields(&realmv1beta1.SmtpServer{
			Host:               sp("smtp.example.com"),
			Port:               sp("587"),
			From:               sp("from@example.com"),
			FromDisplayName:    sp("Example"),
			ReplyTo:            sp("reply@example.com"),
			ReplyToDisplayName: sp("Reply"),
			EnvelopeFrom:       sp("envelope@example.com"),
			Ssl:                bp(true),
			Starttls:           bp(false),
		})

		want := map[string]string{
			"host":               "smtp.example.com",
			"port":               "587",
			"from":               "from@example.com",
			"fromDisplayName":    "Example",
			"replyTo":            "reply@example.com",
			"replyToDisplayName": "Reply",
			"envelopeFrom":       "envelope@example.com",
			"ssl":                "true",
			"starttls":           "false",
		}
		for k, v := range want {
			if m[k] != v {
				t.Errorf("%s = %q, want %q", k, m[k], v)
			}
		}
	})

	// Keycloak reads these as strings, so the booleans have to be formatted
	// rather than dropped. A dropped ssl=false and a dropped ssl=true mean very
	// different things for a mail server.
	t.Run("formats booleans as strings", func(t *testing.T) {
		m := buildSmtpServerMapFields(&realmv1beta1.SmtpServer{Ssl: bp(false), Starttls: bp(true)})
		if m["ssl"] != "false" {
			t.Errorf("ssl = %q, want %q - an explicit false must survive, not be dropped as a zero value", m["ssl"], "false")
		}
		if m["starttls"] != "true" {
			t.Errorf("starttls = %q, want %q", m["starttls"], "true")
		}
	})

	t.Run("omits unset fields entirely", func(t *testing.T) {
		m := buildSmtpServerMapFields(&realmv1beta1.SmtpServer{Host: sp("smtp.example.com")})
		if len(m) != 1 {
			t.Errorf("map = %v, want only the host", m)
		}
	})
}

// secondsToNumber reads values out of the realm map, where JSON numbers arrive as
// float64 unless the decoder was told otherwise. Returning 0 for an unexpected
// type is silent, so it is worth pinning what happens in each case.
func TestSecondsToNumber(t *testing.T) {
	cases := []struct {
		name string
		in   interface{}
		want float64
	}{
		{"float64 from JSON", float64(300), 300},
		{"int", int(300), 300},
		{"int64", int64(300), 300},
		{"negative stays negative", float64(-1), -1},
		{"nil falls back to zero", nil, 0},
		{"unrecognised type falls back to zero", "300", 0},
		{"bool falls back to zero", true, 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := secondsToNumber(tc.in); got != tc.want {
				t.Errorf("secondsToNumber(%#v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}
