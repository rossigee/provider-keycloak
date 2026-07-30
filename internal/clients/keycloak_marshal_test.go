package clients

import (
	"encoding/json"
	"testing"
)

func TestClientRepresentationMarshalRoundtrip(t *testing.T) {
	bTrue, bFalse := true, false
	s := "https://example.com/back"
	in := ClientRepresentation{
		ClientID: "test-app",
		HomeURL: "https://example.com/home",
		BackchannelLogoutURL: "https://example.com/back",
		BackchannelLogoutSessionRequired: &bTrue,
		BackchannelLogoutRevokeOfflineSessions: &bFalse,
		FrontchannelLogoutURL: &s,
		OAuth2DeviceAuthorizationGrantEnabled: &bTrue,
		StandardTokenExchangeEnabled: &bFalse,
		UseRefreshTokens: &bTrue,
		ClientSessionIdleTimeout: "1800",
		ClientSessionMaxLifespan: "36000",
		ClientOfflineSessionIdleTimeout: "2592000",
		ClientOfflineSessionMaxLifespan: "2592000",
	}
	b, err := json.Marshal(&in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	t.Logf("marshaled: %s", string(b))

	// Confirm routed fields are NOT at the top level
	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("unmarshal to map: %v", err)
	}
	for _, f := range []string{"homeUrl", "frontchannelLogoutUrl", "backchannelLogoutUrl", "backchannelLogoutSessionRequired", "backchannelLogoutRevokeOfflineSessions", "oauth2DeviceAuthorizationGrantEnabled", "standardTokenExchangeEnabled", "useRefreshTokens", "clientSessionIdleTimeout", "clientSessionMaxLifespan", "clientOfflineSessionIdleTimeout", "clientOfflineSessionMaxLifespan"} {
		if _, present := raw[f]; present {
			t.Errorf("field %q should be routed via attributes but appeared at top level: %v", f, raw[f])
		}
	}
	attrs, _ := raw["attributes"].(map[string]any)
	if attrs == nil {
		t.Fatalf("expected attributes map, got: %v", raw)
	}
	wantAttrs := map[string]string{
		"home.page.url": "https://example.com/home",
		"frontchannel.logout.url": "https://example.com/back",
		"backchannel.logout.url": "https://example.com/back",
		"backchannel.logout.session.required": "true",
		"backchannel.logout.revoke.offline.sessions": "false",
		"oauth2.device.authorization.grant.enabled": "true",
		"standard.token.exchange.enabled": "false",
		"use.refresh.tokens": "true",
		"client.session.idle.timeout": "1800",
		"client.session.max.lifespan": "36000",
		"client.offline.session.idle.timeout": "2592000",
		"client.offline.session.max.lifespan": "2592000",
	}
	for k, v := range wantAttrs {
		if got, ok := attrs[k]; !ok {
			t.Errorf("attribute %q missing", k)
		} else if got != v {
			t.Errorf("attribute %q = %v, want %q", k, got, v)
		}
	}

	// Round-trip back through Unmarshal and confirm values restored.
	var out ClientRepresentation
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal back: %v", err)
	}
	if out.HomeURL != in.HomeURL {
		t.Errorf("HomeURL = %q, want %q", out.HomeURL, in.HomeURL)
	}
	if out.BackchannelLogoutURL != in.BackchannelLogoutURL {
		t.Errorf("BackchannelLogoutURL = %q, want %q", out.BackchannelLogoutURL, in.BackchannelLogoutURL)
	}
	if *out.BackchannelLogoutSessionRequired != *in.BackchannelLogoutSessionRequired {
		t.Errorf("BackchannelLogoutSessionRequired = %v, want %v", out.BackchannelLogoutSessionRequired, in.BackchannelLogoutSessionRequired)
	}
	if *out.BackchannelLogoutRevokeOfflineSessions != *in.BackchannelLogoutRevokeOfflineSessions {
		t.Errorf("BackchannelLogoutRevokeOfflineSessions = %v, want %v", out.BackchannelLogoutRevokeOfflineSessions, in.BackchannelLogoutRevokeOfflineSessions)
	}
	if *out.FrontchannelLogoutURL != *in.FrontchannelLogoutURL {
		t.Errorf("FrontchannelLogoutURL = %v, want %v", out.FrontchannelLogoutURL, in.FrontchannelLogoutURL)
	}
	if *out.OAuth2DeviceAuthorizationGrantEnabled != *in.OAuth2DeviceAuthorizationGrantEnabled {
		t.Errorf("OAuth2DeviceAuthorizationGrantEnabled = %v, want %v", out.OAuth2DeviceAuthorizationGrantEnabled, in.OAuth2DeviceAuthorizationGrantEnabled)
	}
	if *out.StandardTokenExchangeEnabled != *in.StandardTokenExchangeEnabled {
		t.Errorf("StandardTokenExchangeEnabled = %v, want %v", out.StandardTokenExchangeEnabled, in.StandardTokenExchangeEnabled)
	}
	if *out.UseRefreshTokens != *in.UseRefreshTokens {
		t.Errorf("UseRefreshTokens = %v, want %v", out.UseRefreshTokens, in.UseRefreshTokens)
	}
	if out.ClientSessionIdleTimeout != in.ClientSessionIdleTimeout {
		t.Errorf("ClientSessionIdleTimeout = %q, want %q", out.ClientSessionIdleTimeout, in.ClientSessionIdleTimeout)
	}
	if out.ClientSessionMaxLifespan != in.ClientSessionMaxLifespan {
		t.Errorf("ClientSessionMaxLifespan = %q, want %q", out.ClientSessionMaxLifespan, in.ClientSessionMaxLifespan)
	}
	if out.ClientOfflineSessionIdleTimeout != in.ClientOfflineSessionIdleTimeout {
		t.Errorf("ClientOfflineSessionIdleTimeout = %q, want %q", out.ClientOfflineSessionIdleTimeout, in.ClientOfflineSessionIdleTimeout)
	}
	if out.ClientOfflineSessionMaxLifespan != in.ClientOfflineSessionMaxLifespan {
		t.Errorf("ClientOfflineSessionMaxLifespan = %q, want %q", out.ClientOfflineSessionMaxLifespan, in.ClientOfflineSessionMaxLifespan)
	}
}

func TestClientRepresentationUnmarshalFromKeycloak(t *testing.T) {
	// Simulated Keycloak 26.x GET response — fields Keycloak rejects on PUT
	// come back via attributes.<dot.notation>.
	resp := []byte(`{
		"clientId": "argo-workflows",
		"redirectUris": ["https://workflows.golder.lan/oauth2/callback"],
		"webOrigins": ["+"],
		"rootUrl": null,
		"attributes": {
			"backchannel.logout.url": "https://workflows.golder.lan/back",
			"backchannel.logout.session.required": "true",
			"use.refresh.tokens": "true",
			"client.session.idle.timeout": "1800",
			"client.offline.session.idle.timeout": "2592000",
			"realm_client": "false"
		}
	}`)
	var c ClientRepresentation
	if err := json.Unmarshal(resp, &c); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if c.BackchannelLogoutURL != "https://workflows.golder.lan/back" {
		t.Errorf("BackchannelLogoutURL = %q", c.BackchannelLogoutURL)
	}
	if c.BackchannelLogoutSessionRequired == nil || *c.BackchannelLogoutSessionRequired != true {
		t.Errorf("BackchannelLogoutSessionRequired = %v", c.BackchannelLogoutSessionRequired)
	}
	if c.UseRefreshTokens == nil || *c.UseRefreshTokens != true {
		t.Errorf("UseRefreshTokens = %v", c.UseRefreshTokens)
	}
	if c.ClientSessionIdleTimeout != "1800" {
		t.Errorf("ClientSessionIdleTimeout = %q", c.ClientSessionIdleTimeout)
	}
	if c.ClientOfflineSessionIdleTimeout != "2592000" {
		t.Errorf("ClientOfflineSessionIdleTimeout = %q", c.ClientOfflineSessionIdleTimeout)
	}
	if len(c.ValidRedirectURIs) != 1 || c.ValidRedirectURIs[0] != "https://workflows.golder.lan/oauth2/callback" {
		t.Errorf("ValidRedirectURIs = %v", c.ValidRedirectURIs)
	}
	if len(c.WebOrigins) != 1 || c.WebOrigins[0] != "+" {
		t.Errorf("WebOrigins = %v", c.WebOrigins)
	}
}
