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

package providerconfig

import (
	"context"
	"time"

	"github.com/pkg/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"

	"github.com/rossigee/provider-keycloak/apis/v1beta1"
	"github.com/rossigee/provider-keycloak/internal/clients"
)

const controllerName = "providerconfig.keycloak.crossplane.io"

// Setup registers the ProviderConfig controller.
func Setup(mgr ctrl.Manager) error {
	r := &reconciler{kube: mgr.GetClient()}
	return ctrl.NewControllerManagedBy(mgr).
		Named(controllerName).
		For(&v1beta1.ProviderConfig{}).
		Complete(r)
}

type reconciler struct {
	kube client.Client
}

func (r *reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	pc := &v1beta1.ProviderConfig{}
	if err := r.kube.Get(ctx, req.NamespacedName, pc); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	_, err := clients.NewClient(ctx, pc, r.kube)
	if err != nil {
		pc.Status.SetConditions(xpv1.Unavailable().WithMessage(err.Error()))
	} else {
		pc.Status.SetConditions(xpv1.Available())
	}

	return reconcile.Result{RequeueAfter: 5 * time.Minute},
		errors.Wrap(r.kube.Status().Update(ctx, pc), "cannot update ProviderConfig status")
}
