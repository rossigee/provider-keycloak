/*
Copyright 2020 The Crossplane Authors.

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

package main

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"time"

	xpcontroller "github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/feature"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/v2/pkg/statemetrics"
	"github.com/rossigee/provider-keycloak/apis"
	authenticationflowv1beta1 "github.com/rossigee/provider-keycloak/apis/authenticationflow/v1beta1"
	authorizationpolicyv1beta1 "github.com/rossigee/provider-keycloak/apis/authorizationpolicy/v1beta1"
	authzv1beta1 "github.com/rossigee/provider-keycloak/apis/authz/v1beta1"
	clientv1beta1 "github.com/rossigee/provider-keycloak/apis/client/v1beta1"
	clientcertificatesv1beta1 "github.com/rossigee/provider-keycloak/apis/clientcertificates/v1beta1"
	clientinitialaccessv1beta1 "github.com/rossigee/provider-keycloak/apis/clientinitialaccess/v1beta1"
	componentv1beta1 "github.com/rossigee/provider-keycloak/apis/component/v1beta1"
	eventsv1beta1 "github.com/rossigee/provider-keycloak/apis/events/v1beta1"
	groupv1beta1 "github.com/rossigee/provider-keycloak/apis/group/v1beta1"
	identityproviderv1beta1 "github.com/rossigee/provider-keycloak/apis/identityprovider/v1beta1"
	keysv1beta1 "github.com/rossigee/provider-keycloak/apis/keys/v1beta1"
	openidclientv1beta1 "github.com/rossigee/provider-keycloak/apis/openidclient/v1beta1"
	realmv1beta1 "github.com/rossigee/provider-keycloak/apis/realm/v1beta1"
	realmimpexpv1beta1 "github.com/rossigee/provider-keycloak/apis/realmimpexp/v1beta1"
	rolev1beta1 "github.com/rossigee/provider-keycloak/apis/role/v1beta1"
	rolemappingsv1beta1 "github.com/rossigee/provider-keycloak/apis/rolemappings/v1beta1"
	scopesv1beta1 "github.com/rossigee/provider-keycloak/apis/scopes/v1beta1"
	userv1beta1 "github.com/rossigee/provider-keycloak/apis/user/v1beta1"
	userfederationv1beta1 "github.com/rossigee/provider-keycloak/apis/userfederation/v1beta1"
	controller "github.com/rossigee/provider-keycloak/internal/controller"
	"github.com/rossigee/provider-keycloak/internal/features"
	"github.com/rossigee/provider-keycloak/internal/tracing"
	"gopkg.in/alecthomas/kingpin.v2"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
	metricserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

func main() {
	var (
		app                      = kingpin.New(filepath.Base(os.Args[0]), "Keycloak support for Crossplane.").DefaultEnvars()
		debug                    = app.Flag("debug", "Run with debug logging.").Short('d').Bool()
		leaderElection           = app.Flag("leader-election", "Use leader election for the controller manager.").Short('l').Default("false").OverrideDefaultFromEnvar("LEADER_ELECTION").Bool()
		syncInterval             = app.Flag("sync", "How often all resources will be double-checked for drift from the desired state.").Short('s').Default("1h").Duration()
		pollInterval             = app.Flag("poll", "How often individual resources will be checked for drift from the desired state").Default("1m").Duration()
		maxReconcileRate         = app.Flag("max-reconcile-rate", "The global maximum rate per second at which resources may checked for drift from the desired state.").Default("10").Int()
		cacheInitTimeout         = app.Flag("cache-init-timeout", "Timeout for cache initialization on startup; increase this if the provider fails to start on slow Kubernetes API servers.").Default("5m").Duration()
		pollStateMetricInterval  = app.Flag("poll-state-metric", "State metric recording interval").Default("5s").Duration()
		metricsBindAddress       = app.Flag("metrics-bind-address", "The address the metrics endpoint binds to.").Default(":8080").String()
		enableManagementPolicies = app.Flag("enable-management-policies", "Enable support for Management Policies.").Default("true").Bool()

		// namespace = app.Flag("namespace", "Namespace used to set as default scope in default secret store config.").Default("crossplane-system").Envar("POD_NAMESPACE").String()
	)
	kingpin.MustParse(app.Parse(os.Args[1:]))

	zl := zap.New(zap.UseDevMode(*debug))
	log := logging.NewLogrLogger(zl.WithName("provider-keycloak"))

	shutdownTracing := tracing.Init("provider-keycloak")
	defer shutdownTracing(context.Background())
	ctrl.SetLogger(zl)

	log.Info("Provider starting up",
		"provider", "provider-keycloak",
		"go-version", runtime.Version(),
		"platform", runtime.GOOS+"/"+runtime.GOARCH,
		"sync-interval", syncInterval.String(),
		"poll-interval", pollInterval.String(),
		"max-reconcile-rate", *maxReconcileRate,
		"cache-init-timeout", cacheInitTimeout.String(),
		"leader-election", *leaderElection,
		"debug-mode", *debug)

	cfg, err := ctrl.GetConfig()
	kingpin.FatalIfError(err, "Cannot get API server rest config")

	mgr, err := ctrl.NewManager(ratelimiter.LimitRESTConfig(cfg, *maxReconcileRate), ctrl.Options{
		Cache: cache.Options{
			SyncPeriod: syncInterval,
		},
		// controller-runtime uses both ConfigMaps and Leases for leader
		// election by default. Leases expire after 15 seconds, with a
		// 10 second renewal deadline. We've observed leader loss due to
		// renewal deadlines being exceeded when under high load - i.e.
		// hundreds of reconciles per second and ~200rps to the API
		// server. Switching to Leases only and longer leases appears to
		// alleviate this.
		LeaderElection:             *leaderElection,
		LeaderElectionID:           "crossplane-leader-election-cp-provider-template",
		LeaderElectionResourceLock: resourcelock.LeasesResourceLock,
		LeaseDuration:              func() *time.Duration { d := 60 * time.Second; return &d }(),
		RenewDeadline:              func() *time.Duration { d := 50 * time.Second; return &d }(),
		Controller: config.Controller{
			CacheSyncTimeout: 10 * time.Minute,
		},
		Metrics: metricserver.Options{
			BindAddress: *metricsBindAddress,
		},
	})
	kingpin.FatalIfError(err, "Cannot create controller manager")
	kingpin.FatalIfError(apis.AddToScheme(mgr.GetScheme()), "Cannot add Http APIs to scheme")

	mrStateMetrics := statemetrics.NewMRStateMetrics()
	metrics.Registry.MustRegister(mrStateMetrics)

	featureFlags := &feature.Flags{}
	if *enableManagementPolicies {
		featureFlags.Enable(features.EnableAlphaManagementPolicies)
		log.Info("Alpha feature enabled", "flag", features.EnableAlphaManagementPolicies)
	}

	mo := xpcontroller.MetricOptions{
		PollStateMetricInterval: *pollStateMetricInterval,
		MRStateMetrics:          mrStateMetrics,
	}

	o := xpcontroller.Options{
		Logger:                  log,
		MaxConcurrentReconciles: *maxReconcileRate,
		PollInterval:            *pollInterval,
		GlobalRateLimiter:       ratelimiter.NewGlobal(*maxReconcileRate),
		Features:                featureFlags,
		MetricOptions:           &mo,
	}

	kingpin.FatalIfError(controller.Setup(mgr, o), "Cannot setup Keycloak controllers")

	kingpin.FatalIfError(mgr.Add(statemetrics.NewMRStateRecorder(mgr.GetClient(), o.Logger, o.MetricOptions.MRStateMetrics, &authenticationflowv1beta1.AuthenticationFlowList{}, o.MetricOptions.PollStateMetricInterval)), "Cannot register state metrics for AuthenticationFlow")
	kingpin.FatalIfError(mgr.Add(statemetrics.NewMRStateRecorder(mgr.GetClient(), o.Logger, o.MetricOptions.MRStateMetrics, &authorizationpolicyv1beta1.AuthorizationPolicyList{}, o.MetricOptions.PollStateMetricInterval)), "Cannot register state metrics for AuthorizationPolicy")
	kingpin.FatalIfError(mgr.Add(statemetrics.NewMRStateRecorder(mgr.GetClient(), o.Logger, o.MetricOptions.MRStateMetrics, &authzv1beta1.AuthzResourceList{}, o.MetricOptions.PollStateMetricInterval)), "Cannot register state metrics for AuthzResource")
	kingpin.FatalIfError(mgr.Add(statemetrics.NewMRStateRecorder(mgr.GetClient(), o.Logger, o.MetricOptions.MRStateMetrics, &clientv1beta1.ProtocolMapperList{}, o.MetricOptions.PollStateMetricInterval)), "Cannot register state metrics for ProtocolMapper")
	kingpin.FatalIfError(mgr.Add(statemetrics.NewMRStateRecorder(mgr.GetClient(), o.Logger, o.MetricOptions.MRStateMetrics, &clientcertificatesv1beta1.ClientCertificateList{}, o.MetricOptions.PollStateMetricInterval)), "Cannot register state metrics for ClientCertificate")
	kingpin.FatalIfError(mgr.Add(statemetrics.NewMRStateRecorder(mgr.GetClient(), o.Logger, o.MetricOptions.MRStateMetrics, &clientinitialaccessv1beta1.ClientInitialAccessList{}, o.MetricOptions.PollStateMetricInterval)), "Cannot register state metrics for ClientInitialAccess")
	kingpin.FatalIfError(mgr.Add(statemetrics.NewMRStateRecorder(mgr.GetClient(), o.Logger, o.MetricOptions.MRStateMetrics, &componentv1beta1.ComponentList{}, o.MetricOptions.PollStateMetricInterval)), "Cannot register state metrics for Component")
	kingpin.FatalIfError(mgr.Add(statemetrics.NewMRStateRecorder(mgr.GetClient(), o.Logger, o.MetricOptions.MRStateMetrics, &eventsv1beta1.RealmEventsConfigList{}, o.MetricOptions.PollStateMetricInterval)), "Cannot register state metrics for RealmEventsConfig")
	kingpin.FatalIfError(mgr.Add(statemetrics.NewMRStateRecorder(mgr.GetClient(), o.Logger, o.MetricOptions.MRStateMetrics, &groupv1beta1.GroupList{}, o.MetricOptions.PollStateMetricInterval)), "Cannot register state metrics for Group")
	kingpin.FatalIfError(mgr.Add(statemetrics.NewMRStateRecorder(mgr.GetClient(), o.Logger, o.MetricOptions.MRStateMetrics, &identityproviderv1beta1.IdentityProviderList{}, o.MetricOptions.PollStateMetricInterval)), "Cannot register state metrics for IdentityProvider")
	kingpin.FatalIfError(mgr.Add(statemetrics.NewMRStateRecorder(mgr.GetClient(), o.Logger, o.MetricOptions.MRStateMetrics, &keysv1beta1.RealmKeysList{}, o.MetricOptions.PollStateMetricInterval)), "Cannot register state metrics for RealmKeys")
	kingpin.FatalIfError(mgr.Add(statemetrics.NewMRStateRecorder(mgr.GetClient(), o.Logger, o.MetricOptions.MRStateMetrics, &openidclientv1beta1.ClientList{}, o.MetricOptions.PollStateMetricInterval)), "Cannot register state metrics for Client")
	kingpin.FatalIfError(mgr.Add(statemetrics.NewMRStateRecorder(mgr.GetClient(), o.Logger, o.MetricOptions.MRStateMetrics, &openidclientv1beta1.ClientDefaultScopesList{}, o.MetricOptions.PollStateMetricInterval)), "Cannot register state metrics for ClientDefaultScopes")
	kingpin.FatalIfError(mgr.Add(statemetrics.NewMRStateRecorder(mgr.GetClient(), o.Logger, o.MetricOptions.MRStateMetrics, &openidclientv1beta1.ClientOptionalScopesList{}, o.MetricOptions.PollStateMetricInterval)), "Cannot register state metrics for ClientOptionalScopes")
	kingpin.FatalIfError(mgr.Add(statemetrics.NewMRStateRecorder(mgr.GetClient(), o.Logger, o.MetricOptions.MRStateMetrics, &realmv1beta1.RealmList{}, o.MetricOptions.PollStateMetricInterval)), "Cannot register state metrics for Realm")
	kingpin.FatalIfError(mgr.Add(statemetrics.NewMRStateRecorder(mgr.GetClient(), o.Logger, o.MetricOptions.MRStateMetrics, &realmimpexpv1beta1.RealmImportList{}, o.MetricOptions.PollStateMetricInterval)), "Cannot register state metrics for RealmImport")
	kingpin.FatalIfError(mgr.Add(statemetrics.NewMRStateRecorder(mgr.GetClient(), o.Logger, o.MetricOptions.MRStateMetrics, &rolev1beta1.RoleList{}, o.MetricOptions.PollStateMetricInterval)), "Cannot register state metrics for Role")
	kingpin.FatalIfError(mgr.Add(statemetrics.NewMRStateRecorder(mgr.GetClient(), o.Logger, o.MetricOptions.MRStateMetrics, &rolemappingsv1beta1.ClientRoleMappingList{}, o.MetricOptions.PollStateMetricInterval)), "Cannot register state metrics for ClientRoleMapping")
	kingpin.FatalIfError(mgr.Add(statemetrics.NewMRStateRecorder(mgr.GetClient(), o.Logger, o.MetricOptions.MRStateMetrics, &scopesv1beta1.ClientScopeMappingList{}, o.MetricOptions.PollStateMetricInterval)), "Cannot register state metrics for ClientScopeMapping")
	kingpin.FatalIfError(mgr.Add(statemetrics.NewMRStateRecorder(mgr.GetClient(), o.Logger, o.MetricOptions.MRStateMetrics, &scopesv1beta1.ClientScopeList{}, o.MetricOptions.PollStateMetricInterval)), "Cannot register state metrics for ClientScope")
	kingpin.FatalIfError(mgr.Add(statemetrics.NewMRStateRecorder(mgr.GetClient(), o.Logger, o.MetricOptions.MRStateMetrics, &userv1beta1.UserList{}, o.MetricOptions.PollStateMetricInterval)), "Cannot register state metrics for User")
	kingpin.FatalIfError(mgr.Add(statemetrics.NewMRStateRecorder(mgr.GetClient(), o.Logger, o.MetricOptions.MRStateMetrics, &userv1beta1.GroupsList{}, o.MetricOptions.PollStateMetricInterval)), "Cannot register state metrics for Groups")
	kingpin.FatalIfError(mgr.Add(statemetrics.NewMRStateRecorder(mgr.GetClient(), o.Logger, o.MetricOptions.MRStateMetrics, &userfederationv1beta1.UserFederationProviderList{}, o.MetricOptions.PollStateMetricInterval)), "Cannot register state metrics for UserFederationProvider")

	kingpin.FatalIfError(mgr.AddHealthzCheck("healthz", healthz.Ping), "Cannot add health check")
	kingpin.FatalIfError(mgr.AddReadyzCheck("readyz", healthz.Ping), "Cannot add ready check")

	kingpin.FatalIfError(mgr.Start(ctrl.SetupSignalHandler()), "Cannot start controller manager")
}
