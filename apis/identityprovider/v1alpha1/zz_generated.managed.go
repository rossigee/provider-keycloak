// SPDX-FileCopyrightText: 2025 The Crossplane Authors <https://crossplane.io>
//
// SPDX-License-Identifier: Apache-2.0

// Code generated. DO NOT EDIT.

package v1alpha1

import (
	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
)

// GetCondition returns the condition with the given type.
func (mg *IdentityProvider) GetCondition(ct xpv1.ConditionType) xpv1.Condition {
	return mg.Status.GetCondition(ct)
}

// SetConditions sets the given conditions on the resource.
func (mg *IdentityProvider) SetConditions(c ...xpv1.Condition) {
	mg.Status.SetConditions(c...)
}

// GetManagementPolicies returns the management policies.
func (mg *IdentityProvider) GetManagementPolicies() xpv1.ManagementPolicies {
	return mg.Spec.ManagementPolicies
}

// SetManagementPolicies sets the management policies.
func (mg *IdentityProvider) SetManagementPolicies(p xpv1.ManagementPolicies) {
	mg.Spec.ManagementPolicies = p
}

// GetProviderConfigReference returns the provider config reference.
func (mg *IdentityProvider) GetProviderConfigReference() *xpv1.ProviderConfigReference {
	return mg.Spec.ProviderConfigReference
}

// SetProviderConfigReference sets the provider config reference.
func (mg *IdentityProvider) SetProviderConfigReference(p *xpv1.ProviderConfigReference) {
	mg.Spec.ProviderConfigReference = p
}

// GetWriteConnectionSecretToReference returns the write connection secret reference.
func (mg *IdentityProvider) GetWriteConnectionSecretToReference() *xpv1.LocalSecretReference {
	return mg.Spec.WriteConnectionSecretToReference
}

// SetWriteConnectionSecretToReference sets the write connection secret reference.
func (mg *IdentityProvider) SetWriteConnectionSecretToReference(r *xpv1.LocalSecretReference) {
	mg.Spec.WriteConnectionSecretToReference = r
}
