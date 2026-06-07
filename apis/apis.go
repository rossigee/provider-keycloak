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

// Package apis contains Kubernetes API for the Keycloak provider.
package apis

import (
	"k8s.io/apimachinery/pkg/runtime"

	authenticationflowv1alpha1 "github.com/rossigee/provider-keycloak/apis/authenticationflow/v1alpha1"
	authorizationpolicyv1alpha1 "github.com/rossigee/provider-keycloak/apis/authorizationpolicy/v1alpha1"
	authzv1alpha1 "github.com/rossigee/provider-keycloak/apis/authz/v1alpha1"
	clientv1alpha1 "github.com/rossigee/provider-keycloak/apis/client/v1alpha1"
	clientcertificatesv1alpha1 "github.com/rossigee/provider-keycloak/apis/clientcertificates/v1alpha1"
	clientinitialaccessv1alpha1 "github.com/rossigee/provider-keycloak/apis/clientinitialaccess/v1alpha1"
	componentv1alpha1 "github.com/rossigee/provider-keycloak/apis/component/v1alpha1"
	eventv1alpha1 "github.com/rossigee/provider-keycloak/apis/events/v1alpha1"
	groupv1alpha1 "github.com/rossigee/provider-keycloak/apis/group/v1alpha1"
	identityproviderv1alpha1 "github.com/rossigee/provider-keycloak/apis/identityprovider/v1alpha1"
	keys "github.com/rossigee/provider-keycloak/apis/keys/v1alpha1"
	openidclientv1alpha1 "github.com/rossigee/provider-keycloak/apis/openidclient/v1alpha1"
	realmv1alpha1 "github.com/rossigee/provider-keycloak/apis/realm/v1alpha1"
	realmimpexpv1alpha1 "github.com/rossigee/provider-keycloak/apis/realmimpexp/v1alpha1"
	rolev1alpha1 "github.com/rossigee/provider-keycloak/apis/role/v1alpha1"
	rolemappingsv1alpha1 "github.com/rossigee/provider-keycloak/apis/rolemappings/v1alpha1"
	scopesv1alpha1 "github.com/rossigee/provider-keycloak/apis/scopes/v1alpha1"
	userv1alpha1 "github.com/rossigee/provider-keycloak/apis/user/v1alpha1"
	userfederationv1alpha1 "github.com/rossigee/provider-keycloak/apis/userfederation/v1alpha1"
	"github.com/rossigee/provider-keycloak/apis/v1beta1"
)

func init() {
	AddToSchemes = append(AddToSchemes,
		v1beta1.SchemeBuilder.AddToScheme,
		authenticationflowv1alpha1.SchemeBuilder.AddToScheme,
		authorizationpolicyv1alpha1.SchemeBuilder.AddToScheme,
		clientcertificatesv1alpha1.SchemeBuilder.AddToScheme,
		clientinitialaccessv1alpha1.SchemeBuilder.AddToScheme,
		clientv1alpha1.SchemeBuilder.AddToScheme,
		componentv1alpha1.SchemeBuilder.AddToScheme,
		eventv1alpha1.SchemeBuilder.AddToScheme,
		groupv1alpha1.SchemeBuilder.AddToScheme,
		identityproviderv1alpha1.SchemeBuilder.AddToScheme,
		keys.SchemeBuilder.AddToScheme,
		authzv1alpha1.SchemeBuilder.AddToScheme,
		openidclientv1alpha1.SchemeBuilder.AddToScheme,
		realmv1alpha1.SchemeBuilder.AddToScheme,
		realmimpexpv1alpha1.SchemeBuilder.AddToScheme,
		rolev1alpha1.SchemeBuilder.AddToScheme,
		rolemappingsv1alpha1.SchemeBuilder.AddToScheme,
		scopesv1alpha1.SchemeBuilder.AddToScheme,
		userv1alpha1.SchemeBuilder.AddToScheme,
		userfederationv1alpha1.SchemeBuilder.AddToScheme,
	)
}

// AddToSchemes may be used to add all resources defined in the project to a Scheme.
var AddToSchemes runtime.SchemeBuilder

// AddToScheme adds all Resources to the Scheme.
func AddToScheme(s *runtime.Scheme) error {
	return AddToSchemes.AddToScheme(s)
}
