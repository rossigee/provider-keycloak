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

// +groupName=realm.keycloak.crossplane.io

package v1alpha1

import (
	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SmtpServerAuth holds SMTP authentication credentials.
type SmtpServerAuth struct {
	// Username for SMTP authentication.
	// +optional
	Username *string `json:"username,omitempty"`
	// PasswordSecretRef references the secret containing the SMTP password.
	// +optional
	PasswordSecretRef *SmtpPasswordSecretRef `json:"passwordSecretRef,omitempty"`
}

// SmtpPasswordSecretRef references a secret key holding the SMTP password.
type SmtpPasswordSecretRef struct {
	// Name of the secret.
	Name string `json:"name"`
	// Namespace of the secret.
	Namespace string `json:"namespace"`
	// Key within the secret.
	Key string `json:"key"`
}

// SmtpServer defines the SMTP server configuration for a realm.
type SmtpServer struct {
	// Host is the SMTP server hostname.
	// +optional
	Host *string `json:"host,omitempty"`
	// Port is the SMTP server port.
	// +optional
	Port *string `json:"port,omitempty"`
	// From is the sender email address.
	// +optional
	From *string `json:"from,omitempty"`
	// FromDisplayName is the display name for the sender.
	// +optional
	FromDisplayName *string `json:"fromDisplayName,omitempty"`
	// ReplyTo is the reply-to email address.
	// +optional
	ReplyTo *string `json:"replyTo,omitempty"`
	// ReplyToDisplayName is the display name for the reply-to address.
	// +optional
	ReplyToDisplayName *string `json:"replyToDisplayName,omitempty"`
	// EnvelopeFrom is the envelope from address.
	// +optional
	EnvelopeFrom *string `json:"envelopeFrom,omitempty"`
	// Ssl enables SSL.
	// +optional
	Ssl *bool `json:"ssl,omitempty"`
	// Starttls enables STARTTLS.
	// +optional
	Starttls *bool `json:"starttls,omitempty"`
	// Auth holds authentication credentials.
	// +optional
	Auth []SmtpServerAuth `json:"auth,omitempty"`
}

// RealmParameters are the configurable fields of a Realm.
type RealmParameters struct {
	// Realm is the realm name (ID).
	// +kubebuilder:validation:Required
	Realm string `json:"realm"`

	// Enabled indicates if the realm is enabled.
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// DisplayName is the display name of the realm.
	// +optional
	DisplayName *string `json:"displayName,omitempty"`

	// DisplayNameHtml is the HTML display name.
	// +optional
	DisplayNameHtml *string `json:"displayNameHtml,omitempty"`

	// SslRequired sets the SSL requirement (none, external, all).
	// +kubebuilder:validation:Enum=none;external;all
	// +optional
	SslRequired *string `json:"sslRequired,omitempty"`

	// RegistrationAllowed allows user self-registration.
	// +optional
	RegistrationAllowed *bool `json:"registrationAllowed,omitempty"`

	// RegistrationEmailAsUsername uses email as username during registration.
	// +optional
	RegistrationEmailAsUsername *bool `json:"registrationEmailAsUsername,omitempty"`

	// EditUsernameAllowed allows users to edit their username.
	// +optional
	EditUsernameAllowed *bool `json:"editUsernameAllowed,omitempty"`

	// ResetPasswordAllowed allows users to reset their password.
	// +optional
	ResetPasswordAllowed *bool `json:"resetPasswordAllowed,omitempty"`

	// RememberMe enables the remember-me feature.
	// +optional
	RememberMe *bool `json:"rememberMe,omitempty"`

	// VerifyEmail requires email verification after registration.
	// +optional
	VerifyEmail *bool `json:"verifyEmail,omitempty"`

	// LoginWithEmailAllowed allows login with email address.
	// +optional
	LoginWithEmailAllowed *bool `json:"loginWithEmailAllowed,omitempty"`

	// DuplicateEmailsAllowed allows multiple accounts with the same email.
	// +optional
	DuplicateEmailsAllowed *bool `json:"duplicateEmailsAllowed,omitempty"`

	// DefaultSignatureAlgorithm is the default algorithm for signing tokens.
	// +optional
	DefaultSignatureAlgorithm *string `json:"defaultSignatureAlgorithm,omitempty"`

	// RevokeRefreshToken revokes refresh tokens after use.
	// +optional
	RevokeRefreshToken *bool `json:"revokeRefreshToken,omitempty"`

	// RefreshTokenMaxReuse sets the maximum number of times a refresh token may be reused.
	// +optional
	RefreshTokenMaxReuse *int64 `json:"refreshTokenMaxReuse,omitempty"`

	// AccessTokenLifespan is the lifespan of access tokens (e.g. "30m0s").
	// +optional
	AccessTokenLifespan *string `json:"accessTokenLifespan,omitempty"`

	// AccessTokenLifespanForImplicitFlow is the lifespan for implicit flow tokens.
	// +optional
	AccessTokenLifespanForImplicitFlow *string `json:"accessTokenLifespanForImplicitFlow,omitempty"`

	// SsoSessionIdleTimeout is the SSO session idle timeout.
	// +optional
	SsoSessionIdleTimeout *string `json:"ssoSessionIdleTimeout,omitempty"`

	// SsoSessionMaxLifespan is the SSO session maximum lifespan.
	// +optional
	SsoSessionMaxLifespan *string `json:"ssoSessionMaxLifespan,omitempty"`

	// SsoSessionIdleTimeoutRememberMe is the idle timeout for remember-me sessions.
	// +optional
	SsoSessionIdleTimeoutRememberMe *string `json:"ssoSessionIdleTimeoutRememberMe,omitempty"`

	// SsoSessionMaxLifespanRememberMe is the maximum lifespan for remember-me sessions.
	// +optional
	SsoSessionMaxLifespanRememberMe *string `json:"ssoSessionMaxLifespanRememberMe,omitempty"`

	// OfflineSessionIdleTimeout is the offline session idle timeout.
	// +optional
	OfflineSessionIdleTimeout *string `json:"offlineSessionIdleTimeout,omitempty"`

	// OfflineSessionMaxLifespanEnabled enables the offline session maximum lifespan.
	// +optional
	OfflineSessionMaxLifespanEnabled *bool `json:"offlineSessionMaxLifespanEnabled,omitempty"`

	// OfflineSessionMaxLifespan is the offline session maximum lifespan.
	// +optional
	OfflineSessionMaxLifespan *string `json:"offlineSessionMaxLifespan,omitempty"`

	// ClientSessionIdleTimeout is the client session idle timeout.
	// +optional
	ClientSessionIdleTimeout *string `json:"clientSessionIdleTimeout,omitempty"`

	// ClientSessionMaxLifespan is the client session maximum lifespan.
	// +optional
	ClientSessionMaxLifespan *string `json:"clientSessionMaxLifespan,omitempty"`

	// AccessCodeLifespan is the lifespan of authorization codes.
	// +optional
	AccessCodeLifespan *string `json:"accessCodeLifespan,omitempty"`

	// AccessCodeLifespanUserAction is the lifespan of user action codes.
	// +optional
	AccessCodeLifespanUserAction *string `json:"accessCodeLifespanUserAction,omitempty"`

	// AccessCodeLifespanLogin is the lifespan of login action codes.
	// +optional
	AccessCodeLifespanLogin *string `json:"accessCodeLifespanLogin,omitempty"`

	// ActionTokenGeneratedByAdminLifespan is the lifespan of admin-generated action tokens.
	// +optional
	ActionTokenGeneratedByAdminLifespan *string `json:"actionTokenGeneratedByAdminLifespan,omitempty"`

	// ActionTokenGeneratedByUserLifespan is the lifespan of user-generated action tokens.
	// +optional
	ActionTokenGeneratedByUserLifespan *string `json:"actionTokenGeneratedByUserLifespan,omitempty"`

	// LoginTheme sets the login page theme.
	// +optional
	LoginTheme *string `json:"loginTheme,omitempty"`

	// AccountTheme sets the account console theme.
	// +optional
	AccountTheme *string `json:"accountTheme,omitempty"`

	// AdminTheme sets the admin console theme.
	// +optional
	AdminTheme *string `json:"adminTheme,omitempty"`

	// EmailTheme sets the email theme.
	// +optional
	EmailTheme *string `json:"emailTheme,omitempty"`

	// SmtpServer configures the SMTP server for outgoing emails.
	// +optional
	SmtpServer []SmtpServer `json:"smtpServer,omitempty"`

	// Attributes is a map of realm attributes.
	// +optional
	Attributes map[string]string `json:"attributes,omitempty"`

	// PasswordPolicy is the password policy string.
	// +optional
	PasswordPolicy *string `json:"passwordPolicy,omitempty"`

	// DefaultDefaultClientScopes is the list of default client scopes.
	// +optional
	DefaultDefaultClientScopes []string `json:"defaultDefaultClientScopes,omitempty"`

	// DefaultOptionalClientScopes is the list of optional client scopes.
	// +optional
	DefaultOptionalClientScopes []string `json:"defaultOptionalClientScopes,omitempty"`

	// BrowserFlow overrides the browser authentication flow.
	// +optional
	BrowserFlow *string `json:"browserFlow,omitempty"`

	// RegistrationFlow overrides the registration flow.
	// +optional
	RegistrationFlow *string `json:"registrationFlow,omitempty"`

	// DirectGrantFlow overrides the direct grant flow.
	// +optional
	DirectGrantFlow *string `json:"directGrantFlow,omitempty"`

	// ResetCredentialsFlow overrides the reset credentials flow.
	// +optional
	ResetCredentialsFlow *string `json:"resetCredentialsFlow,omitempty"`

	// ClientAuthenticationFlow overrides the client authentication flow.
	// +optional
	ClientAuthenticationFlow *string `json:"clientAuthenticationFlow,omitempty"`

	// UserManagedAccess enables user-managed access.
	// +optional
	UserManagedAccess *bool `json:"userManagedAccess,omitempty"`

	// AdminPermissionsEnabled enables fine-grained admin permissions.
	// +optional
	AdminPermissionsEnabled *bool `json:"adminPermissionsEnabled,omitempty"`
}

// RealmSpec defines the desired state of a Realm.
type RealmSpec struct {
	xpv1.ManagedResourceSpec `json:",inline"`
	ForProvider              RealmParameters `json:"forProvider"`
}

// RealmStatus defines the observed state of a Realm.
type RealmStatus struct {
	xpv1.ManagedResourceStatus `json:",inline"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={crossplane,managed,keycloak}
// +kubebuilder:storageversion
// +kubebuilder:group=realm.keycloak.crossplane.io
// +kubebuilder:printcolumn:name="REALM",type="string",JSONPath=".spec.forProvider.realm"
// +kubebuilder:printcolumn:name="ENABLED",type="boolean",JSONPath=".spec.forProvider.enabled"
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// A Realm is a Keycloak realm.
type Realm struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RealmSpec   `json:"spec"`
	Status RealmStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RealmList contains a list of Realm.
type RealmList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Realm `json:"items"`
}
