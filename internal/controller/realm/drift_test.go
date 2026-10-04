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
	"reflect"
	"testing"

	realmv1beta1 "github.com/rossigee/provider-keycloak/apis/realm/v1beta1"
	"github.com/rossigee/provider-keycloak/internal/clients"
)

// realmComparableFields lists every RealmParameters field that also exists on
// clients.Realm, skipping FrontendURL which is a pointer on both sides and is
// handled separately below.
//
// This exists so a field added to the representation without a matching check
// in realmUpToDate fails the suite. Such a field would be sent to Keycloak and
// then never compared, so drift would go unnoticed forever while the resource
// reported itself up to date.
func realmComparableFields(t *testing.T) []string {
	t.Helper()

	pt := reflect.TypeOf(realmv1beta1.RealmParameters{})
	rt := reflect.TypeOf(clients.Realm{})

	var out []string
	for i := 0; i < pt.NumField(); i++ {
		f := pt.Field(i)
		if f.Type.Kind() != reflect.Pointer {
			continue
		}
		rf, ok := rt.FieldByName(f.Name)
		if !ok {
			continue
		}
		if rf.Type.Kind() != reflect.Pointer && rf.Type == f.Type.Elem() {
			out = append(out, f.Name)
		}
	}
	return out
}

// nonZero sets a pointer field to a value that differs from its zero value, so
// a comparison against an unset actual field must report drift.
func nonZero(t *testing.T, p *realmv1beta1.RealmParameters, name string) {
	t.Helper()

	f := reflect.ValueOf(p).Elem().FieldByName(name)
	elem := reflect.New(f.Type().Elem())
	switch elem.Elem().Interface().(type) {
	case bool:
		elem.Elem().SetBool(true)
	case string:
		elem.Elem().SetString("drifted")
	case int64:
		elem.Elem().SetInt(1)
	default:
		t.Fatalf("field %s has unhandled type %s", name, f.Type())
	}
	f.Set(elem)
}

func TestRealmUpToDateDetectsDriftOnEveryField(t *testing.T) {
	fields := realmComparableFields(t)
	if len(fields) < 25 {
		t.Fatalf("only found %d comparable realm fields, expected the full set", len(fields))
	}

	for _, name := range fields {
		t.Run(name, func(t *testing.T) {
			p := &realmv1beta1.RealmParameters{}
			nonZero(t, p, name)

			if realmUpToDate(p, &clients.Realm{}) {
				t.Errorf("%s differs from the actual realm but realmUpToDate reported "+
					"it up to date; the field would never be reconciled", name)
			}
		})
	}
}

func TestRealmUpToDateAcceptsAMatchOnEveryField(t *testing.T) {
	for _, name := range realmComparableFields(t) {
		t.Run(name, func(t *testing.T) {
			p := &realmv1beta1.RealmParameters{}
			nonZero(t, p, name)

			actual := &clients.Realm{}
			av := reflect.ValueOf(actual).Elem().FieldByName(name)
			av.Set(reflect.ValueOf(p).Elem().FieldByName(name).Elem())

			if !realmUpToDate(p, actual) {
				t.Errorf("%s matches the actual realm but was reported as drift", name)
			}
		})
	}
}

// Unset fields mean "not managed", so they must not be treated as drift. Without
// this, every optional realm setting an operator left alone in Keycloak would
// show as permanent drift and the resource would update forever.
func TestRealmUpToDateTreatsUnsetFieldsAsUnmanaged(t *testing.T) {
	if !realmUpToDate(&realmv1beta1.RealmParameters{}, &clients.Realm{}) {
		t.Error("an empty spec reported drift against an empty realm; unset fields " +
			"mean unmanaged, not mismatched")
	}

	// Even when Keycloak has settings the spec never mentions.
	if !realmUpToDate(&realmv1beta1.RealmParameters{}, &clients.Realm{
		Enabled: true, DisplayName: "set by an operator", RegistrationAllowed: true,
	}) {
		t.Error("settings present in Keycloak but absent from the spec were reported " +
			"as drift")
	}
}

// FrontendURL is a *string on both sides, so it needs its own handling.
func TestRealmUpToDateHandlesFrontendURL(t *testing.T) {
	p := &realmv1beta1.RealmParameters{}
	setStr := reflect.ValueOf(p).Elem().FieldByName("FrontendURL")
	el := reflect.New(setStr.Type().Elem())
	el.Elem().SetString("https://drifted.example.com")
	setStr.Set(el)

	if realmUpToDate(p, &clients.Realm{}) {
		t.Error("a differing FrontendURL was reported as up to date")
	}

	same := "https://drifted.example.com"
	if !realmUpToDate(p, &clients.Realm{FrontendURL: &same}) {
		t.Error("a matching FrontendURL was reported as drift")
	}
}

// RealmParameters -> clients.Realm: unset fields stay at the representation's
// zero value, and Enabled defaults to true because Keycloak treats a realm with
// no explicit enabled flag as disabled, which would take the realm offline.
func TestRealmParamsToRepresentation(t *testing.T) {
	t.Run("defaults enabled to true", func(t *testing.T) {
		got := realmParamsToRepresentation(context.Background(), nil, &realmv1beta1.RealmParameters{Realm: "r"})
		if !got.Enabled {
			t.Error("Enabled = false with no explicit setting; an unset enabled flag must " +
				"default to true or the realm comes up disabled")
		}
	})

	t.Run("carries declared fields across", func(t *testing.T) {
		p := &realmv1beta1.RealmParameters{Realm: "master"}
		p.Enabled = bp(false)
		p.DisplayName = sp("Example")
		p.RefreshTokenMaxReuse = func() *int64 { v := int64(0); return &v }()

		got := realmParamsToRepresentation(context.Background(), nil, p)
		if got.Realm != "master" {
			t.Errorf("Realm = %q, want master", got.Realm)
		}
		if got.Enabled {
			t.Error("Enabled = true, want the explicitly disabled false")
		}
		if got.DisplayName != "Example" {
			t.Errorf("DisplayName = %q, want Example", got.DisplayName)
		}
	})

	t.Run("leaves unset fields at zero", func(t *testing.T) {
		got := realmParamsToRepresentation(context.Background(), nil, &realmv1beta1.RealmParameters{Realm: "r"})
		if got.DisplayName != "" || got.RememberMe || got.VerifyEmail {
			t.Errorf("unset fields were populated: %+v", got)
		}
	})
}
