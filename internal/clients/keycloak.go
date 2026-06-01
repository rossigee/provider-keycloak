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

package clients

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/rossigee/provider-keycloak/apis/v1beta1"
)

const (
	defaultTimeout       = 30 * time.Second
	adminPath            = "/admin/realms"
	oauthKeyClientID     = "client_id"
	oauthKeyClientSecret = "client_secret"
	maxErrBodyLen        = 256
)

// realmPath returns the safely encoded admin API path for a realm.
func realmPath(realm string) string {
	return adminPath + "/" + url.PathEscape(realm)
}

// Client interface for Keycloak API operations
type Client interface {
	// Realm operations
	GetRealm(ctx context.Context, realm string) (*Realm, error)
	CreateRealm(ctx context.Context, realm *Realm) (*Realm, error)
	UpdateRealm(ctx context.Context, realm *Realm) error
	DeleteRealm(ctx context.Context, realm string) error

	// Client operations
	GetClient(ctx context.Context, realm, clientID string) (*ClientRepresentation, error)
	CreateClient(ctx context.Context, realm string, client *ClientRepresentation) (*ClientRepresentation, error)
	UpdateClient(ctx context.Context, realm string, client *ClientRepresentation) error
	DeleteClient(ctx context.Context, realm, clientID string) error
	ListClients(ctx context.Context, realm string) ([]ClientRepresentation, error)

	// User operations
	GetUser(ctx context.Context, realm, username string) (*UserRepresentation, error)
	CreateUser(ctx context.Context, realm string, user *UserRepresentation) (*UserRepresentation, error)
	UpdateUser(ctx context.Context, realm string, user *UserRepresentation) error
	DeleteUser(ctx context.Context, realm, userID string) error
	ListUsers(ctx context.Context, realm string) ([]UserRepresentation, error)

	// Client secret operations
	GetClientSecret(ctx context.Context, realm, clientUUID string) (string, error)

	// Group operations
	GetGroup(ctx context.Context, realm, groupID string) (*GroupRepresentation, error)
	CreateGroup(ctx context.Context, realm string, group *GroupRepresentation) (*GroupRepresentation, error)
	UpdateGroup(ctx context.Context, realm string, group *GroupRepresentation) error
	DeleteGroup(ctx context.Context, realm, groupID string) error
	ListGroups(ctx context.Context, realm string) ([]GroupRepresentation, error)
	SearchGroups(ctx context.Context, realm, name string) ([]GroupRepresentation, error)

	// User group membership operations
	GetUserGroups(ctx context.Context, realm, userUUID string) ([]GroupRepresentation, error)
	AddUserToGroup(ctx context.Context, realm, userUUID, groupUUID string) error
	RemoveUserFromGroup(ctx context.Context, realm, userUUID, groupUUID string) error
	SearchUsers(ctx context.Context, realm, username string) ([]UserRepresentation, error)

	// Role operations
	GetRealmRole(ctx context.Context, realm, name string) (*RoleRepresentation, error)
	CreateRealmRole(ctx context.Context, realm string, role *RoleRepresentation) error
	UpdateRealmRole(ctx context.Context, realm, name string, role *RoleRepresentation) error
	DeleteRealmRole(ctx context.Context, realm, name string) error

	// Protocol mapper operations
	GetClientProtocolMapper(ctx context.Context, realm, clientUUID, mapperID string) (*ProtocolMapperRepresentation, error)
	CreateClientProtocolMapper(ctx context.Context, realm, clientUUID string, mapper *ProtocolMapperRepresentation) (string, error)
	UpdateClientProtocolMapper(ctx context.Context, realm, clientUUID string, mapper *ProtocolMapperRepresentation) error
	DeleteClientProtocolMapper(ctx context.Context, realm, clientUUID, mapperID string) error
	ListClientProtocolMappers(ctx context.Context, realm, clientUUID string) ([]ProtocolMapperRepresentation, error)
}

// keycloakClient implements Client
type keycloakClient struct {
	httpClient *http.Client
	baseURL    string
	token      string
	tokenExp   time.Time // token expiration time
	cfg        *Config   // for token refresh
}

// NewClient creates a new Keycloak API client using OAuth2 client credentials.
func NewClient(ctx context.Context, pc *v1beta1.ProviderConfig, kube client.Client) (*keycloakClient, error) {
	cfg, err := GetConfig(ctx, pc, kube)
	if err != nil {
		return nil, errors.Wrap(err, "cannot load provider config")
	}
	return NewClientFromConfig(ctx, cfg)
}

// NewClientFromConfig creates a new Keycloak API client from a resolved Config.
func NewClientFromConfig(ctx context.Context, cfg *Config) (*keycloakClient, error) {
	transport := http.DefaultTransport
	if cfg.RootCACertificate != "" {
		pool := x509.NewCertPool()
		pool.AppendCertsFromPEM([]byte(cfg.RootCACertificate))
		transport = &http.Transport{
			TLSClientConfig: &tls.Config{RootCAs: pool},
		}
	}

	httpClient := &http.Client{
		Timeout:   defaultTimeout,
		Transport: transport,
	}

	baseURL := strings.TrimSuffix(cfg.BaseURL, "/")

	token, exp, err := fetchOAuth2Token(ctx, httpClient, baseURL, cfg)
	if err != nil {
		return nil, errors.Wrap(err, "cannot obtain access token")
	}

	return &keycloakClient{
		httpClient: httpClient,
		baseURL:    baseURL,
		token:      token,
		tokenExp:   exp,
		cfg:        cfg,
	}, nil
}

// tokenResponse is the OAuth2 token endpoint response.
type tokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"` // token lifetime in seconds
	Error       string `json:"error"`
	ErrorDesc   string `json:"error_description"`
}

// fetchOAuth2Token obtains an access token via the client credentials grant.
// Returns the token and its expiration time.
func fetchOAuth2Token(ctx context.Context, hc *http.Client, baseURL string, cfg *Config) (string, time.Time, error) {
	tokenURL := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/token", baseURL, url.PathEscape(cfg.Realm))

	form := url.Values{
		"grant_type":         {"client_credentials"},
		oauthKeyClientID:     {cfg.ClientID},
		oauthKeyClientSecret: {cfg.ClientSecret},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", time.Time{}, errors.Wrap(err, "failed to create token request")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := hc.Do(req)
	if err != nil {
		return "", time.Time{}, errors.Wrap(err, "failed to execute token request")
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", time.Time{}, errors.Wrap(err, "failed to read token response")
	}

	var tr tokenResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return "", time.Time{}, errors.Wrap(err, "failed to parse token response")
	}

	if tr.Error != "" {
		return "", time.Time{}, errors.Errorf("token request failed: %s: %s", tr.Error, tr.ErrorDesc)
	}
	if tr.AccessToken == "" {
		return "", time.Time{}, errors.New("token response contained no access_token")
	}

	// Calculate expiry as now + ExpiresIn - 10 second buffer to refresh before expiration
	exp := time.Now().Add(time.Duration(tr.ExpiresIn)*time.Second - 10*time.Second)
	return tr.AccessToken, exp, nil
}

// refreshToken checks if the access token is expired and fetches a new one if necessary.
// If no config is available (e.g. in tests), it skips refresh.
func (k *keycloakClient) refreshToken(ctx context.Context) error {
	if time.Now().Before(k.tokenExp) {
		return nil // token still valid
	}
	if k.cfg == nil {
		return nil // no config available, skip refresh (e.g. in tests)
	}
	token, exp, err := fetchOAuth2Token(ctx, k.httpClient, k.baseURL, k.cfg)
	if err != nil {
		return err
	}
	k.token = token
	k.tokenExp = exp
	return nil
}

// =============================================================================
// HTTP Methods
// =============================================================================

func (c *keycloakClient) doRequest(ctx context.Context, method, path string, body interface{}) ([]byte, error) {
	if err := c.refreshToken(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to refresh access token")
	}

	var bodyReader io.Reader
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return nil, errors.Wrap(err, "failed to marshal request body")
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create request")
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute request")
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read response body")
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := string(respBody)
		if len(msg) > maxErrBodyLen {
			msg = msg[:maxErrBodyLen] + "..."
		}
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, msg)
	}

	return respBody, nil
}

// =============================================================================
// Realm Operations
// =============================================================================

type Realm struct {
	Realm                  string `json:"realm"`
	Enabled                bool   `json:"enabled"`
	DisplayName            string `json:"displayName,omitempty"`
	LoginWithEmailAllowed  bool   `json:"loginWithEmailAllowed"`
	DuplicateEmailsAllowed bool   `json:"duplicateEmailsAllowed"`
	ResetPasswordAllowed   bool   `json:"resetPasswordAllowed"`
	EditUsernameAllowed    bool   `json:"editUsernameAllowed"`
	BruteForceProtected    bool   `json:"bruteForceProtected"`
}

func (c *keycloakClient) GetRealm(ctx context.Context, realm string) (*Realm, error) {
	respBody, err := c.doRequest(ctx, http.MethodGet, realmPath(realm), nil)
	if err != nil {
		return nil, err
	}

	var r Realm
	if err := json.Unmarshal(respBody, &r); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal realm")
	}

	return &r, nil
}

func (c *keycloakClient) CreateRealm(ctx context.Context, realm *Realm) (*Realm, error) {
	respBody, err := c.doRequest(ctx, http.MethodPost, adminPath, realm)
	if err != nil {
		return nil, err
	}

	var r Realm
	if err := json.Unmarshal(respBody, &r); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal realm")
	}

	return &r, nil
}

func (c *keycloakClient) UpdateRealm(ctx context.Context, realm *Realm) error {
	_, err := c.doRequest(ctx, http.MethodPut, realmPath(realm.Realm), realm)
	return err
}

func (c *keycloakClient) DeleteRealm(ctx context.Context, realm string) error {
	_, err := c.doRequest(ctx, http.MethodDelete, realmPath(realm), nil)
	return err
}

// =============================================================================
// Client Operations
// =============================================================================

type ClientRepresentation struct {
	ID                        string            `json:"id,omitempty"`
	ClientID                  string            `json:"clientId"`
	Name                      string            `json:"name,omitempty"`
	Description               string            `json:"description,omitempty"`
	Enabled                   bool              `json:"enabled"`
	RootURL                   string            `json:"rootUrl,omitempty"`
	BaseURL                   string            `json:"baseUrl,omitempty"`
	ValidRedirectURIs         []string          `json:"validRedirectUris,omitempty"`
	WebOrigins                []string          `json:"webOrigins,omitempty"`
	StandardFlowEnabled       bool              `json:"standardFlowEnabled"`
	DirectAccessGrantsEnabled bool              `json:"directAccessGrantsEnabled"`
	ImplicitFlowEnabled       bool              `json:"implicitFlowEnabled"`
	ServiceAccountsEnabled    bool              `json:"serviceAccountsEnabled"`
	PublicClient              bool              `json:"publicClient"`
	Protocol                  string            `json:"protocol,omitempty"`
	Attributes                map[string]string `json:"attributes,omitempty"`
}

func (c *keycloakClient) GetClient(ctx context.Context, realm, clientID string) (*ClientRepresentation, error) {
	path := realmPath(realm) + "/clients?clientId=" + url.QueryEscape(clientID)
	respBody, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var clients []ClientRepresentation
	if err := json.Unmarshal(respBody, &clients); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal clients")
	}

	if len(clients) == 0 {
		return nil, nil
	}

	return &clients[0], nil
}

func (c *keycloakClient) CreateClient(ctx context.Context, realm string, client *ClientRepresentation) (*ClientRepresentation, error) {
	// Keycloak POST /clients returns HTTP 201 with an empty body.
	// The internal UUID is in the Location header's last path segment.
	id, err := c.doCreate(ctx, realmPath(realm)+"/clients", client)
	if err != nil {
		return nil, err
	}
	created := *client
	created.ID = id
	return &created, nil
}

// doCreate POSTs body to path and extracts the created resource UUID from the
// Location response header.  Keycloak returns Location: .../clients/{uuid}.
func (c *keycloakClient) doCreate(ctx context.Context, path string, body interface{}) (string, error) {
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return "", errors.Wrap(err, "failed to marshal request body")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", errors.Wrap(err, "failed to create request")
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", errors.Wrap(err, "failed to execute request")
	}
	defer func() { _ = resp.Body.Close() }()

	// Read and discard body to allow connection reuse.
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrBodyLen+1))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := string(respBody)
		if len(msg) > maxErrBodyLen {
			msg = msg[:maxErrBodyLen] + "..."
		}
		return "", fmt.Errorf("request failed with status %d: %s", resp.StatusCode, msg)
	}

	loc := resp.Header.Get("Location")
	if loc == "" {
		return "", nil
	}
	// Location is .../clients/{uuid} — UUID is the last path segment.
	parsed, err := url.Parse(loc)
	if err != nil {
		return "", errors.Wrap(err, "failed to parse Location header")
	}
	segments := strings.Split(strings.TrimRight(parsed.Path, "/"), "/")
	return segments[len(segments)-1], nil
}

func (c *keycloakClient) UpdateClient(ctx context.Context, realm string, client *ClientRepresentation) error {
	if client.ID == "" {
		return errors.New("client ID is required for update")
	}
	path := realmPath(realm) + "/clients/" + url.PathEscape(client.ID)
	_, err := c.doRequest(ctx, http.MethodPut, path, client)
	return err
}

func (c *keycloakClient) DeleteClient(ctx context.Context, realm, clientID string) error {
	path := realmPath(realm) + "/clients/" + url.PathEscape(clientID)
	_, err := c.doRequest(ctx, http.MethodDelete, path, nil)
	return err
}

func (c *keycloakClient) ListClients(ctx context.Context, realm string) ([]ClientRepresentation, error) {
	path := realmPath(realm) + "/clients"
	respBody, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var clients []ClientRepresentation
	if err := json.Unmarshal(respBody, &clients); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal clients")
	}

	return clients, nil
}

// =============================================================================
// User Operations
// =============================================================================

type UserRepresentation struct {
	ID            string              `json:"id,omitempty"`
	Username      string              `json:"username"`
	Email         string              `json:"email,omitempty"`
	FirstName     string              `json:"firstName,omitempty"`
	LastName      string              `json:"lastName,omitempty"`
	Enabled       bool                `json:"enabled"`
	EmailVerified bool                `json:"emailVerified"`
	Groups        []string            `json:"groups,omitempty"`
	RealmRoles    []string            `json:"realmRoles,omitempty"`
	ClientRoles   map[string][]string `json:"clientRoles,omitempty"`
	Attributes    map[string][]string `json:"attributes,omitempty"`
}

func (c *keycloakClient) GetUser(ctx context.Context, realm, username string) (*UserRepresentation, error) {
	path := realmPath(realm) + "/users?username=" + url.QueryEscape(username)
	respBody, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var users []UserRepresentation
	if err := json.Unmarshal(respBody, &users); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal users")
	}

	if len(users) == 0 {
		return nil, nil
	}

	return &users[0], nil
}

func (c *keycloakClient) CreateUser(ctx context.Context, realm string, user *UserRepresentation) (*UserRepresentation, error) {
	path := realmPath(realm) + "/users"
	respBody, err := c.doRequest(ctx, http.MethodPost, path, user)
	if err != nil {
		return nil, err
	}

	var created UserRepresentation
	if err := json.Unmarshal(respBody, &created); err != nil && len(respBody) > 0 {
		return nil, errors.Wrap(err, "failed to unmarshal created user")
	}

	return &created, nil
}

func (c *keycloakClient) UpdateUser(ctx context.Context, realm string, user *UserRepresentation) error {
	if user.ID == "" {
		return errors.New("user ID is required for update")
	}
	path := realmPath(realm) + "/users/" + url.PathEscape(user.ID)
	_, err := c.doRequest(ctx, http.MethodPut, path, user)
	return err
}

func (c *keycloakClient) DeleteUser(ctx context.Context, realm, userID string) error {
	path := realmPath(realm) + "/users/" + url.PathEscape(userID)
	_, err := c.doRequest(ctx, http.MethodDelete, path, nil)
	return err
}

func (c *keycloakClient) ListUsers(ctx context.Context, realm string) ([]UserRepresentation, error) {
	path := realmPath(realm) + "/users"
	respBody, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var users []UserRepresentation
	if err := json.Unmarshal(respBody, &users); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal users")
	}

	return users, nil
}

// =============================================================================
// Group Operations
// =============================================================================

type GroupRepresentation struct {
	ID          string              `json:"id,omitempty"`
	Name        string              `json:"name"`
	Path        string              `json:"path,omitempty"`
	RealmRoles  []string            `json:"realmRoles,omitempty"`
	ClientRoles map[string][]string `json:"clientRoles,omitempty"`
	Attributes  map[string]string   `json:"attributes,omitempty"`
}

func (c *keycloakClient) GetGroup(ctx context.Context, realm, groupID string) (*GroupRepresentation, error) {
	path := realmPath(realm) + "/groups/" + url.PathEscape(groupID)
	respBody, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var g GroupRepresentation
	if err := json.Unmarshal(respBody, &g); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal group")
	}

	return &g, nil
}

func (c *keycloakClient) CreateGroup(ctx context.Context, realm string, group *GroupRepresentation) (*GroupRepresentation, error) {
	path := realmPath(realm) + "/groups"
	respBody, err := c.doRequest(ctx, http.MethodPost, path, group)
	if err != nil {
		return nil, err
	}

	var created GroupRepresentation
	if err := json.Unmarshal(respBody, &created); err != nil && len(respBody) > 0 {
		return nil, errors.Wrap(err, "failed to unmarshal created group")
	}

	return &created, nil
}

func (c *keycloakClient) UpdateGroup(ctx context.Context, realm string, group *GroupRepresentation) error {
	if group.ID == "" {
		return errors.New("group ID is required for update")
	}
	path := realmPath(realm) + "/groups/" + url.PathEscape(group.ID)
	_, err := c.doRequest(ctx, http.MethodPut, path, group)
	return err
}

func (c *keycloakClient) DeleteGroup(ctx context.Context, realm, groupID string) error {
	path := realmPath(realm) + "/groups/" + url.PathEscape(groupID)
	_, err := c.doRequest(ctx, http.MethodDelete, path, nil)
	return err
}

func (c *keycloakClient) ListGroups(ctx context.Context, realm string) ([]GroupRepresentation, error) {
	path := realmPath(realm) + "/groups"
	respBody, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var groups []GroupRepresentation
	if err := json.Unmarshal(respBody, &groups); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal groups")
	}

	return groups, nil
}

// =============================================================================
// Client Secret Operations
// =============================================================================

type clientSecretResponse struct {
	Value string `json:"value"`
}

func (c *keycloakClient) GetClientSecret(ctx context.Context, realm, clientUUID string) (string, error) {
	path := realmPath(realm) + "/clients/" + url.PathEscape(clientUUID) + "/client-secret"
	respBody, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return "", err
	}
	var s clientSecretResponse
	if err := json.Unmarshal(respBody, &s); err != nil {
		return "", errors.Wrap(err, "failed to unmarshal client secret")
	}
	return s.Value, nil
}

// =============================================================================
// Extended Group Operations
// =============================================================================

func (c *keycloakClient) SearchGroups(ctx context.Context, realm, name string) ([]GroupRepresentation, error) {
	path := realmPath(realm) + "/groups?search=" + url.QueryEscape(name)
	respBody, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var groups []GroupRepresentation
	if err := json.Unmarshal(respBody, &groups); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal groups")
	}
	return groups, nil
}

func (c *keycloakClient) GetUserGroups(ctx context.Context, realm, userUUID string) ([]GroupRepresentation, error) {
	path := realmPath(realm) + "/users/" + url.PathEscape(userUUID) + "/groups"
	respBody, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var groups []GroupRepresentation
	if err := json.Unmarshal(respBody, &groups); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal user groups")
	}
	return groups, nil
}

func (c *keycloakClient) AddUserToGroup(ctx context.Context, realm, userUUID, groupUUID string) error {
	path := realmPath(realm) + "/users/" + url.PathEscape(userUUID) + "/groups/" + url.PathEscape(groupUUID)
	_, err := c.doRequest(ctx, http.MethodPut, path, nil)
	return err
}

func (c *keycloakClient) RemoveUserFromGroup(ctx context.Context, realm, userUUID, groupUUID string) error {
	path := realmPath(realm) + "/users/" + url.PathEscape(userUUID) + "/groups/" + url.PathEscape(groupUUID)
	_, err := c.doRequest(ctx, http.MethodDelete, path, nil)
	return err
}

func (c *keycloakClient) SearchUsers(ctx context.Context, realm, username string) ([]UserRepresentation, error) {
	path := realmPath(realm) + "/users?username=" + url.QueryEscape(username) + "&exact=true"
	respBody, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var users []UserRepresentation
	if err := json.Unmarshal(respBody, &users); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal users")
	}
	return users, nil
}

// =============================================================================
// Role Operations
// =============================================================================

// RoleRepresentation is a Keycloak realm or client role.
type RoleRepresentation struct {
	ID          string              `json:"id,omitempty"`
	Name        string              `json:"name"`
	Description string              `json:"description,omitempty"`
	Composite   bool                `json:"composite,omitempty"`
	ClientRole  bool                `json:"clientRole,omitempty"`
	Attributes  map[string][]string `json:"attributes,omitempty"`
}

func (c *keycloakClient) GetRealmRole(ctx context.Context, realm, name string) (*RoleRepresentation, error) {
	path := realmPath(realm) + "/roles/" + url.PathEscape(name)
	respBody, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var r RoleRepresentation
	if err := json.Unmarshal(respBody, &r); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal role")
	}
	return &r, nil
}

func (c *keycloakClient) CreateRealmRole(ctx context.Context, realm string, role *RoleRepresentation) error {
	path := realmPath(realm) + "/roles"
	_, err := c.doRequest(ctx, http.MethodPost, path, role)
	return err
}

func (c *keycloakClient) UpdateRealmRole(ctx context.Context, realm, name string, role *RoleRepresentation) error {
	path := realmPath(realm) + "/roles/" + url.PathEscape(name)
	_, err := c.doRequest(ctx, http.MethodPut, path, role)
	return err
}

func (c *keycloakClient) DeleteRealmRole(ctx context.Context, realm, name string) error {
	path := realmPath(realm) + "/roles/" + url.PathEscape(name)
	_, err := c.doRequest(ctx, http.MethodDelete, path, nil)
	return err
}

// =============================================================================
// Protocol Mapper Operations
// =============================================================================

// ProtocolMapperRepresentation is a Keycloak protocol mapper.
type ProtocolMapperRepresentation struct {
	ID             string            `json:"id,omitempty"`
	Name           string            `json:"name"`
	Protocol       string            `json:"protocol"`
	ProtocolMapper string            `json:"protocolMapper"`
	Config         map[string]string `json:"config,omitempty"`
}

func (c *keycloakClient) ListClientProtocolMappers(ctx context.Context, realm, clientUUID string) ([]ProtocolMapperRepresentation, error) {
	path := realmPath(realm) + "/clients/" + url.PathEscape(clientUUID) + "/protocol-mappers/models"
	respBody, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var mappers []ProtocolMapperRepresentation
	if err := json.Unmarshal(respBody, &mappers); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal protocol mappers")
	}
	return mappers, nil
}

func (c *keycloakClient) GetClientProtocolMapper(ctx context.Context, realm, clientUUID, mapperID string) (*ProtocolMapperRepresentation, error) {
	path := realmPath(realm) + "/clients/" + url.PathEscape(clientUUID) + "/protocol-mappers/models/" + url.PathEscape(mapperID)
	respBody, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var m ProtocolMapperRepresentation
	if err := json.Unmarshal(respBody, &m); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal protocol mapper")
	}
	return &m, nil
}

func (c *keycloakClient) CreateClientProtocolMapper(ctx context.Context, realm, clientUUID string, mapper *ProtocolMapperRepresentation) (string, error) {
	path := realmPath(realm) + "/clients/" + url.PathEscape(clientUUID) + "/protocol-mappers/models"
	return c.doCreate(ctx, path, mapper)
}

func (c *keycloakClient) UpdateClientProtocolMapper(ctx context.Context, realm, clientUUID string, mapper *ProtocolMapperRepresentation) error {
	if mapper.ID == "" {
		return errors.New("mapper ID is required for update")
	}
	path := realmPath(realm) + "/clients/" + url.PathEscape(clientUUID) + "/protocol-mappers/models/" + url.PathEscape(mapper.ID)
	_, err := c.doRequest(ctx, http.MethodPut, path, mapper)
	return err
}

func (c *keycloakClient) DeleteClientProtocolMapper(ctx context.Context, realm, clientUUID, mapperID string) error {
	path := realmPath(realm) + "/clients/" + url.PathEscape(clientUUID) + "/protocol-mappers/models/" + url.PathEscape(mapperID)
	_, err := c.doRequest(ctx, http.MethodDelete, path, nil)
	return err
}
