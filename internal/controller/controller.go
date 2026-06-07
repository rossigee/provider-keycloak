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

package controller

import (
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"

	"github.com/rossigee/provider-keycloak/internal/controller/authz"
	"github.com/rossigee/provider-keycloak/internal/controller/client"
	"github.com/rossigee/provider-keycloak/internal/controller/clientcertificates"
	"github.com/rossigee/provider-keycloak/internal/controller/clientinitialaccess"
	"github.com/rossigee/provider-keycloak/internal/controller/clientrolemapping"
	"github.com/rossigee/provider-keycloak/internal/controller/clientscopemapping"
	"github.com/rossigee/provider-keycloak/internal/controller/component"
	"github.com/rossigee/provider-keycloak/internal/controller/events"
	"github.com/rossigee/provider-keycloak/internal/controller/group"
	"github.com/rossigee/provider-keycloak/internal/controller/protocolmapper"
	"github.com/rossigee/provider-keycloak/internal/controller/providerconfig"
	"github.com/rossigee/provider-keycloak/internal/controller/realm"
	"github.com/rossigee/provider-keycloak/internal/controller/realmimpexp"
	"github.com/rossigee/provider-keycloak/internal/controller/realmkeys"
	"github.com/rossigee/provider-keycloak/internal/controller/role"
	"github.com/rossigee/provider-keycloak/internal/controller/user"
	"github.com/rossigee/provider-keycloak/internal/controller/userfederation"
)

// Setup sets up Keycloak provider controllers.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	if err := providerconfig.Setup(mgr); err != nil {
		return err
	}
	for _, setup := range []func(ctrl.Manager, controller.Options) error{
		client.Setup,
		realm.Setup,
		user.Setup,
		group.Setup,
		role.Setup,
		protocolmapper.Setup,
		authz.Setup,
		clientcertificates.Setup,
		events.Setup,
		realmimpexp.Setup,
		userfederation.Setup,
		clientrolemapping.Setup,
		clientscopemapping.Setup,
		clientinitialaccess.Setup,
		component.Setup,
		realmkeys.Setup,
	} {
		if err := setup(mgr, o); err != nil {
			return err
		}
	}
	return nil
}
