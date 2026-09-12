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
	authenticationflowv1beta1 "github.com/rossigee/provider-keycloak/apis/authenticationflow/v1beta1"
	authorizationpolicyv1beta1 "github.com/rossigee/provider-keycloak/apis/authorizationpolicy/v1beta1"
	authzv1beta1 "github.com/rossigee/provider-keycloak/apis/authz/v1beta1"
	clientv1beta1 "github.com/rossigee/provider-keycloak/apis/client/v1beta1"
	clientcertificatesv1beta1 "github.com/rossigee/provider-keycloak/apis/clientcertificates/v1beta1"
	clientinitialaccessv1beta1 "github.com/rossigee/provider-keycloak/apis/clientinitialaccess/v1beta1"
	componentv1beta1 "github.com/rossigee/provider-keycloak/apis/component/v1beta1"
	eventv1beta1 "github.com/rossigee/provider-keycloak/apis/events/v1beta1"
	groupv1beta1 "github.com/rossigee/provider-keycloak/apis/group/v1beta1"
	identityproviderv1beta1 "github.com/rossigee/provider-keycloak/apis/identityprovider/v1beta1"
	keys "github.com/rossigee/provider-keycloak/apis/keys/v1beta1"
	openidclientv1beta1 "github.com/rossigee/provider-keycloak/apis/openidclient/v1beta1"
	realmv1beta1 "github.com/rossigee/provider-keycloak/apis/realm/v1beta1"
	realmimpexpv1beta1 "github.com/rossigee/provider-keycloak/apis/realmimpexp/v1beta1"
	rolev1beta1 "github.com/rossigee/provider-keycloak/apis/role/v1beta1"
	rolemappingsv1beta1 "github.com/rossigee/provider-keycloak/apis/rolemappings/v1beta1"
	scopesv1beta1 "github.com/rossigee/provider-keycloak/apis/scopes/v1beta1"
	userv1beta1 "github.com/rossigee/provider-keycloak/apis/user/v1beta1"
	userfederationv1beta1 "github.com/rossigee/provider-keycloak/apis/userfederation/v1beta1"
	v1beta1 "github.com/rossigee/provider-keycloak/apis/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
)

func init() {
	AddToSchemes = append(AddToSchemes,
		v1beta1.SchemeBuilder.AddToScheme,
		authenticationflowv1beta1.SchemeBuilder.AddToScheme,
		authorizationpolicyv1beta1.SchemeBuilder.AddToScheme,
		clientcertificatesv1beta1.SchemeBuilder.AddToScheme,
		clientinitialaccessv1beta1.SchemeBuilder.AddToScheme,
		clientv1beta1.SchemeBuilder.AddToScheme,
		componentv1beta1.SchemeBuilder.AddToScheme,
		eventv1beta1.SchemeBuilder.AddToScheme,
		groupv1beta1.SchemeBuilder.AddToScheme,
		identityproviderv1beta1.SchemeBuilder.AddToScheme,
		keys.SchemeBuilder.AddToScheme,
		authzv1beta1.SchemeBuilder.AddToScheme,
		openidclientv1beta1.SchemeBuilder.AddToScheme,
		realmv1beta1.SchemeBuilder.AddToScheme,
		realmimpexpv1beta1.SchemeBuilder.AddToScheme,
		rolev1beta1.SchemeBuilder.AddToScheme,
		rolemappingsv1beta1.SchemeBuilder.AddToScheme,
		scopesv1beta1.SchemeBuilder.AddToScheme,
		userv1beta1.SchemeBuilder.AddToScheme,
		userfederationv1beta1.SchemeBuilder.AddToScheme,
	)
}

// AddToSchemes may be used to add all resources defined in the project to a Scheme.
var AddToSchemes runtime.SchemeBuilder

// AddToScheme adds all Resources to the Scheme.
func AddToScheme(s *runtime.Scheme) error {
	return AddToSchemes.AddToScheme(s)
}
