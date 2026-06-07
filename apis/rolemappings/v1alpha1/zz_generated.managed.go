// SPDX-FileCopyrightText: 2025 The Crossplane Authors

// Code generated. DO NOT EDIT.

package v1alpha1

import (
	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
)

func (mg *ClientRoleMapping) GetCondition(ct xpv1.ConditionType) xpv1.Condition {
	return mg.Status.GetCondition(ct)
}

func (mg *ClientRoleMapping) SetConditions(c ...xpv1.Condition) {
	mg.Status.SetConditions(c...)
}

func (mg *ClientRoleMapping) GetManagementPolicies() xpv1.ManagementPolicies {
	return mg.Spec.ManagementPolicies
}

func (mg *ClientRoleMapping) SetManagementPolicies(p xpv1.ManagementPolicies) {
	mg.Spec.ManagementPolicies = p
}

func (mg *ClientRoleMapping) GetProviderConfigReference() *xpv1.ProviderConfigReference {
	return mg.Spec.ProviderConfigReference
}

func (mg *ClientRoleMapping) SetProviderConfigReference(p *xpv1.ProviderConfigReference) {
	mg.Spec.ProviderConfigReference = p
}

func (mg *ClientRoleMapping) GetWriteConnectionSecretToReference() *xpv1.LocalSecretReference {
	return mg.Spec.WriteConnectionSecretToReference
}

func (mg *ClientRoleMapping) SetWriteConnectionSecretToReference(r *xpv1.LocalSecretReference) {
	mg.Spec.WriteConnectionSecretToReference = r
}
