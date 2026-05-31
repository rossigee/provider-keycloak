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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/rossigee/provider-keycloak/apis/v1beta1"
)

const (
	// Default timeout for API requests
	defaultTimeout = 30 * time.Second

	// API paths
	adminPath = "/admin/realms"
)

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

	// Group operations
	GetGroup(ctx context.Context, realm, groupID string) (*GroupRepresentation, error)
	CreateGroup(ctx context.Context, realm string, group *GroupRepresentation) (*GroupRepresentation, error)
	UpdateGroup(ctx context.Context, realm string, group *GroupRepresentation) error
	DeleteGroup(ctx context.Context, realm, groupID string) error
	ListGroups(ctx context.Context, realm string) ([]GroupRepresentation, error)
}

// keycloakClient implements Client
type keycloakClient struct {
	httpClient *http.Client
	baseURL   string
	token     string
}

// NewClient creates a new Keycloak API client
func NewClient(ctx context.Context, cfg *v1beta1.ProviderConfig, kube client.Client) (*keycloakClient, error) {
	if cfg.Spec.BaseURL == "" {
		return nil, errors.New("baseURL is required")
	}

	// Get authentication token from secret
	token, err := getTokenFromSecret(ctx, cfg, kube)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get authentication token")
	}

	// Setup HTTP client
	httpClient := &http.Client{
		Timeout: defaultTimeout,
	}

	// Handle insecure connections
	if cfg.Spec.Insecure != nil && *cfg.Spec.Insecure {
		transport := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		httpClient.Transport = transport
	}

	baseURL := strings.TrimSuffix(cfg.Spec.BaseURL, "/")

	return &keycloakClient{
		httpClient: httpClient,
		baseURL:    baseURL,
		token:      token,
	}, nil
}

// =============================================================================
// HTTP Methods
// =============================================================================

func (c *keycloakClient) doRequest(ctx context.Context, method, path string, body interface{}) ([]byte, error) {
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
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read response body")
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// =============================================================================
// Realm Operations
// =============================================================================

type Realm struct {
	Realm                 string   `json:"realm"`
	Enabled               bool     `json:"enabled"`
	DisplayName           string   `json:"displayName,omitempty"`
	LoginWithEmailAllowed bool     `json:"loginWithEmailAllowed"`
	DuplicateEmailsAllowed bool   `json:"duplicateEmailsAllowed"`
	ResetPasswordAllowed  bool     `json:"resetPasswordAllowed"`
	EditUsernameAllowed   bool     `json:"editUsernameAllowed"`
	BruteForceProtected   bool     `json:"bruteForceProtected"`
}

func (c *keycloakClient) GetRealm(ctx context.Context, realm string) (*Realm, error) {
	path := fmt.Sprintf("%s/%s", adminPath, realm)
	respBody, err := c.doRequest(ctx, http.MethodGet, path, nil)
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
	path := fmt.Sprintf("%s/%s", adminPath, realm.Realm)
	_, err := c.doRequest(ctx, http.MethodPut, path, realm)
	return err
}

func (c *keycloakClient) DeleteRealm(ctx context.Context, realm string) error {
	path := fmt.Sprintf("%s/%s", adminPath, realm)
	_, err := c.doRequest(ctx, http.MethodDelete, path, nil)
	return err
}

// =============================================================================
// Client Operations
// =============================================================================

type ClientRepresentation struct {
	ID                  string            `json:"id,omitempty"`
	ClientID            string            `json:"clientId"`
	Name                string            `json:"name,omitempty"`
	Description         string            `json:"description,omitempty"`
	Enabled             bool              `json:"enabled"`
	RootURL             string            `json:"rootUrl,omitempty"`
	BaseURL             string            `json:"baseUrl,omitempty"`
	ValidRedirectURIs   []string          `json:"validRedirectUris,omitempty"`
	WebOrigins          []string          `json:"webOrigins,omitempty"`
	StandardFlowEnabled bool              `json:"standardFlowEnabled"`
	DirectAccessGrantsEnabled bool         `json:"directAccessGrantsEnabled"`
	ImplicitFlowEnabled bool              `json:"implicitFlowEnabled"`
	ServiceAccountsEnabled bool           `json:"serviceAccountsEnabled"`
	PublicClient        bool              `json:"publicClient"`
	Protocol            string            `json:"protocol,omitempty"`
	Attributes          map[string]string `json:"attributes,omitempty"`
}

func (c *keycloakClient) GetClient(ctx context.Context, realm, clientID string) (*ClientRepresentation, error) {
	path := fmt.Sprintf("%s/%s/clients?clientId=%s", adminPath, realm, clientID)
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
	path := fmt.Sprintf("%s/%s/clients", adminPath, realm)
	respBody, err := c.doRequest(ctx, http.MethodPost, path, client)
	if err != nil {
		return nil, err
	}

	var created ClientRepresentation
	// POST to clients returns the ID in Location header, not the object
	if err := json.Unmarshal(respBody, &created); err != nil && len(respBody) > 0 {
		return nil, errors.Wrap(err, "failed to unmarshal created client")
	}

	return &created, nil
}

func (c *keycloakClient) UpdateClient(ctx context.Context, realm string, client *ClientRepresentation) error {
	if client.ID == "" {
		return errors.New("client ID is required for update")
	}
	path := fmt.Sprintf("%s/%s/clients/%s", adminPath, realm, client.ID)
	_, err := c.doRequest(ctx, http.MethodPut, path, client)
	return err
}

func (c *keycloakClient) DeleteClient(ctx context.Context, realm, clientID string) error {
	path := fmt.Sprintf("%s/%s/clients/%s", adminPath, realm, clientID)
	_, err := c.doRequest(ctx, http.MethodDelete, path, nil)
	return err
}

func (c *keycloakClient) ListClients(ctx context.Context, realm string) ([]ClientRepresentation, error) {
	path := fmt.Sprintf("%s/%s/clients", adminPath, realm)
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
	ID            string            `json:"id,omitempty"`
	Username      string            `json:"username"`
	Email         string            `json:"email,omitempty"`
	FirstName     string            `json:"firstName,omitempty"`
	LastName      string            `json:"lastName,omitempty"`
	Enabled       bool              `json:"enabled"`
	EmailVerified bool              `json:"emailVerified"`
	Groups        []string          `json:"groups,omitempty"`
	RealmRoles    []string          `json:"realmRoles,omitempty"`
	ClientRoles   map[string][]string `json:"clientRoles,omitempty"`
	Attributes    map[string][]string `json:"attributes,omitempty"`
}

func (c *keycloakClient) GetUser(ctx context.Context, realm, username string) (*UserRepresentation, error) {
	path := fmt.Sprintf("%s/%s/users?username=%s", adminPath, realm, username)
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
	path := fmt.Sprintf("%s/%s/users", adminPath, realm)
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
	path := fmt.Sprintf("%s/%s/users/%s", adminPath, realm, user.ID)
	_, err := c.doRequest(ctx, http.MethodPut, path, user)
	return err
}

func (c *keycloakClient) DeleteUser(ctx context.Context, realm, userID string) error {
	path := fmt.Sprintf("%s/%s/users/%s", adminPath, realm, userID)
	_, err := c.doRequest(ctx, http.MethodDelete, path, nil)
	return err
}

func (c *keycloakClient) ListUsers(ctx context.Context, realm string) ([]UserRepresentation, error) {
	path := fmt.Sprintf("%s/%s/users", adminPath, realm)
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
	ID   string            `json:"id,omitempty"`
	Name string            `json:"name"`
	Path string            `json:"path,omitempty"`
	RealmRoles    []string          `json:"realmRoles,omitempty"`
	ClientRoles   map[string][]string `json:"clientRoles,omitempty"`
	Attributes    map[string]string `json:"attributes,omitempty"`
}

func (c *keycloakClient) GetGroup(ctx context.Context, realm, groupID string) (*GroupRepresentation, error) {
	path := fmt.Sprintf("%s/%s/groups/%s", adminPath, realm, groupID)
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
	path := fmt.Sprintf("%s/%s/groups", adminPath, realm)
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
	path := fmt.Sprintf("%s/%s/groups/%s", adminPath, realm, group.ID)
	_, err := c.doRequest(ctx, http.MethodPut, path, group)
	return err
}

func (c *keycloakClient) DeleteGroup(ctx context.Context, realm, groupID string) error {
	path := fmt.Sprintf("%s/%s/groups/%s", adminPath, realm, groupID)
	_, err := c.doRequest(ctx, http.MethodDelete, path, nil)
	return err
}

func (c *keycloakClient) ListGroups(ctx context.Context, realm string) ([]GroupRepresentation, error) {
	path := fmt.Sprintf("%s/%s/groups", adminPath, realm)
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
// Helper Functions
// =============================================================================

func getTokenFromSecret(ctx context.Context, cfg *v1beta1.ProviderConfig, kube client.Client) (string, error) {
	if cfg.Spec.Credentials.SecretRef == nil {
		return "", errors.New("credentials.secretRef is required")
	}

	secret := &corev1.Secret{}
	err := kube.Get(ctx, types.NamespacedName{
		Name:      cfg.Spec.Credentials.SecretRef.Name,
		Namespace: cfg.Spec.Credentials.SecretRef.Namespace,
	}, secret)
	if err != nil {
		return "", errors.Wrap(err, "failed to get credentials secret")
	}

	key := cfg.Spec.Credentials.SecretRef.Key
	if key == "" {
		key = "token"
	}

	tokenBytes, ok := secret.Data[key]
	if !ok {
		return "", errors.Errorf("key %q not found in credentials secret", key)
	}

	return string(tokenBytes), nil
}