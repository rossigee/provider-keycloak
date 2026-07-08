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

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
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
	r := &reconciler{kube: mgr.GetClient(), logger: mgr.GetLogger()}
	return ctrl.NewControllerManagedBy(mgr).
		Named(controllerName).
		For(&v1beta1.ProviderConfig{}).
		Complete(r)
}

type reconciler struct {
	kube   client.Client
	logger logr.Logger
}

func (r *reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := r.logger.WithValues("providerconfig", req.Name)
	log.Info("reconciling ProviderConfig")

	pc := &v1beta1.ProviderConfig{}
	if err := r.kube.Get(ctx, req.NamespacedName, pc); err != nil {
		log.Error(err, "failed to get ProviderConfig")
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	log.Info("attempting to connect to Keycloak")
	kc, err := clients.NewClient(ctx, pc, r.kube)
	if err != nil {
		log.Error(err, "failed to connect to Keycloak")
		pc.Status.SetConditions(xpv1.Unavailable().WithMessage(err.Error()))
	} else {
		log.Info("successfully connected to Keycloak")
		pc.Status.SetConditions(xpv1.Available())
		_ = kc // avoid unused warning
	}

	// The CRD doesn't have status subresource enabled
	// Try updating with the entire object including status
	// Fetch fresh copy to avoid conflicts
	fresh := &v1beta1.ProviderConfig{}
	if err := r.kube.Get(ctx, client.ObjectKey{Name: pc.GetName()}, fresh); err != nil {
		log.Error(err, "failed to get latest ProviderConfig for status update")
		return reconcile.Result{RequeueAfter: 30 * time.Second}, client.IgnoreNotFound(err)
	}

	log.Info("Updating ProviderConfig", "name", fresh.GetName(), "status", pc.Status)
	fresh.Status = pc.Status

	// Try using client.Update which updates the entire object
	if err := r.kube.Update(ctx, fresh); err != nil {
		log.Error(err, "failed to update ProviderConfig", "error", err)
		if errors.IsConflict(err) {
			log.Info("conflict updating, will retry", "error", err)
			return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
		}
		return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
	}
	log.Info("ProviderConfig updated successfully")

	log.Info("ProviderConfig status updated successfully")

	return reconcile.Result{RequeueAfter: 5 * time.Minute}, nil
}
