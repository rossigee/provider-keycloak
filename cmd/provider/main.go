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
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
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
	"github.com/rossigee/provider-keycloak/internal/version"
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

// options is the provider's parsed runtime configuration. It exists so the flag
// surface can be exercised directly, and so the values can be handed to the
// manager and controller options where a test can see them land.
type options struct {
	debug                    bool
	leaderElection           bool
	syncInterval             time.Duration
	pollInterval             time.Duration
	maxReconcileRate         int
	cacheInitTimeout         time.Duration
	pollStateMetricInterval  time.Duration
	metricsBindAddress       string
	enableManagementPolicies bool
}

// parseOptions builds the flag surface and parses args against it.
func parseOptions(name, description string, args []string) (*options, error) {
	var o options

	app := kingpin.New(name, description).DefaultEnvars()
	app.Flag("debug", "Run with debug logging.").Short('d').BoolVar(&o.debug)
	app.Flag("leader-election", "Use leader election for the controller manager.").Short('l').Default("false").OverrideDefaultFromEnvar("LEADER_ELECTION").BoolVar(&o.leaderElection)
	app.Flag("sync", "How often all resources will be double-checked for drift from the desired state.").Short('s').Default("1h").DurationVar(&o.syncInterval)
	app.Flag("poll", "How often individual resources will be checked for drift from the desired state").Default("1m").DurationVar(&o.pollInterval)
	app.Flag("max-reconcile-rate", "The global maximum rate per second at which resources may checked for drift from the desired state.").Default("10").IntVar(&o.maxReconcileRate)
	app.Flag("cache-init-timeout", "Timeout for cache initialization on startup; increase this if the provider fails to start on slow Kubernetes API servers.").Default("5m").DurationVar(&o.cacheInitTimeout)
	app.Flag("poll-state-metric", "State metric recording interval").Default("5s").DurationVar(&o.pollStateMetricInterval)
	app.Flag("metrics-bind-address", "The address the metrics endpoint binds to.").Default(":8080").StringVar(&o.metricsBindAddress)
	app.Flag("enable-management-policies", "Enable support for Management Policies. Use --no-enable-management-policies to disable.").Default("true").BoolVar(&o.enableManagementPolicies)

	_, err := app.Parse(args)
	return &o, err
}

// managerOptions assembles the controller-runtime manager configuration.
func (o *options) managerOptions() ctrl.Options {
	return ctrl.Options{
		Cache: cache.Options{
			SyncPeriod: &o.syncInterval,
		},
		// controller-runtime uses both ConfigMaps and Leases for leader
		// election by default. Leases expire after 15 seconds, with a
		// 10 second renewal deadline. We've observed leader loss due to
		// renewal deadlines being exceeded when under high load - i.e.
		// hundreds of reconciles per second and ~200rps to the API
		// server. Switching to Leases only and longer leases appears to
		// alleviate this.
		LeaderElection:             o.leaderElection,
		LeaderElectionID:           "crossplane-leader-election-cp-provider-template",
		LeaderElectionResourceLock: resourcelock.LeasesResourceLock,
		LeaseDuration:              func() *time.Duration { d := 60 * time.Second; return &d }(),
		RenewDeadline:              func() *time.Duration { d := 50 * time.Second; return &d }(),
		Controller: config.Controller{
			// This was previously hardcoded to 10 minutes, which made the
			// --cache-init-timeout flag a no-op: it was parsed and logged but
			// never reached the manager, so raising it to get past a slow API
			// server did nothing.
			CacheSyncTimeout: o.cacheInitTimeout,
		},
		Metrics: metricserver.Options{
			BindAddress: o.metricsBindAddress,
		},
	}
}

// controllerOptions assembles the crossplane-runtime controller configuration.
func (o *options) controllerOptions(log logging.Logger, mrStateMetrics *statemetrics.MRStateMetrics) xpcontroller.Options {
	featureFlags := &feature.Flags{}
	if o.enableManagementPolicies {
		featureFlags.Enable(features.EnableAlphaManagementPolicies)
		log.Info("Alpha feature enabled", "flag", features.EnableAlphaManagementPolicies)
	}

	return xpcontroller.Options{
		Logger:                  log,
		MaxConcurrentReconciles: o.maxReconcileRate,
		PollInterval:            o.pollInterval,
		GlobalRateLimiter:       ratelimiter.NewGlobal(o.maxReconcileRate),
		Features:                featureFlags,
		MetricOptions: &xpcontroller.MetricOptions{
			PollStateMetricInterval: o.pollStateMetricInterval,
			MRStateMetrics:          mrStateMetrics,
		},
	}
}

// stateManagedLists returns every resource whose poll state is recorded as a
// metric. Kept as data so the set can be asserted rather than counted by eye.
func stateManagedLists() []resource.ManagedList {
	return []resource.ManagedList{
		&authenticationflowv1beta1.AuthenticationFlowList{},
		&authorizationpolicyv1beta1.AuthorizationPolicyList{},
		&authzv1beta1.AuthzResourceList{},
		&clientv1beta1.ProtocolMapperList{},
		&clientcertificatesv1beta1.ClientCertificateList{},
		&clientinitialaccessv1beta1.ClientInitialAccessList{},
		&componentv1beta1.ComponentList{},
		&eventsv1beta1.RealmEventsConfigList{},
		&groupv1beta1.GroupList{},
		&identityproviderv1beta1.IdentityProviderList{},
		&keysv1beta1.RealmKeysList{},
		&openidclientv1beta1.ClientList{},
		&openidclientv1beta1.ClientDefaultScopesList{},
		&openidclientv1beta1.ClientOptionalScopesList{},
		&realmv1beta1.RealmList{},
		&realmimpexpv1beta1.RealmImportList{},
		&rolev1beta1.RoleList{},
		&rolemappingsv1beta1.ClientRoleMappingList{},
		&scopesv1beta1.ClientScopeMappingList{},
		&scopesv1beta1.ClientScopeList{},
		&userv1beta1.UserList{},
		&userv1beta1.GroupsList{},
		&userfederationv1beta1.UserFederationProviderList{},
	}
}

func main() {
	o, err := parseOptions(filepath.Base(os.Args[0]), "Keycloak support for Crossplane.", os.Args[1:])
	kingpin.FatalIfError(err, "Cannot parse flags")

	zl := zap.New(zap.UseDevMode(o.debug))
	log := logging.NewLogrLogger(zl.WithName("provider-keycloak"))

	shutdownTracing := tracing.Init("provider-keycloak")
	defer shutdownTracing(context.Background())
	ctrl.SetLogger(zl)

	log.Info("Provider starting up",
		"provider", "provider-keycloak",
		"version", version.Version,
		"go-version", runtime.Version(),
		"platform", runtime.GOOS+"/"+runtime.GOARCH,
		"sync-interval", o.syncInterval.String(),
		"poll-interval", o.pollInterval.String(),
		"max-reconcile-rate", o.maxReconcileRate,
		"cache-init-timeout", o.cacheInitTimeout.String(),
		"leader-election", o.leaderElection,
		"debug-mode", o.debug)

	cfg, err := ctrl.GetConfig()
	kingpin.FatalIfError(err, "Cannot get API server rest config")

	mgrOpts := o.managerOptions()
	mgr, err := ctrl.NewManager(ratelimiter.LimitRESTConfig(cfg, o.maxReconcileRate), mgrOpts)
	kingpin.FatalIfError(err, "Cannot create controller manager")
	kingpin.FatalIfError(apis.AddToScheme(mgr.GetScheme()), "Cannot add Http APIs to scheme")

	mrStateMetrics := statemetrics.NewMRStateMetrics()
	metrics.Registry.MustRegister(mrStateMetrics)

	ctrlOpts := o.controllerOptions(log, mrStateMetrics)

	kingpin.FatalIfError(controller.Setup(mgr, ctrlOpts), "Cannot setup Keycloak controllers")

	for _, list := range stateManagedLists() {
		kingpin.FatalIfError(mgr.Add(statemetrics.NewMRStateRecorder(mgr.GetClient(), ctrlOpts.Logger, ctrlOpts.MetricOptions.MRStateMetrics, list, ctrlOpts.MetricOptions.PollStateMetricInterval)), "Cannot register state metrics")
	}

	kingpin.FatalIfError(mgr.AddHealthzCheck("healthz", healthz.Ping), "Cannot add health check")
	kingpin.FatalIfError(mgr.AddReadyzCheck("readyz", healthz.Ping), "Cannot add ready check")

	kingpin.FatalIfError(mgr.Start(ctrl.SetupSignalHandler()), "Cannot start controller manager")
}
