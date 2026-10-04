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

package main

import (
	"reflect"
	"testing"
	"time"

	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/statemetrics"

	"github.com/rossigee/provider-keycloak/internal/features"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

// These tests exist because the subprocess tests could only prove a flag was
// accepted and reported. They now cover where each value actually lands, which
// is the part that was previously unverified - see the note in main_test.go.

func parse(t *testing.T, args ...string) *options {
	t.Helper()
	// Only valid parses are exercised here. kingpin handles --help and invalid
	// input by terminating the process, which would take the test binary with
	// it; those paths stay covered by the subprocess tests in main_test.go.
	//
	// Note kingpin does not accept "--flag=false" for booleans - it rejects the
	// trailing "false" as a stray positional. Negation is "--no-flag".
	o, err := parseOptions("provider", "test", args)
	if err != nil {
		t.Fatalf("parseOptions(%v) failed: %v", args, err)
	}
	return o
}

func TestParseOptionsDefaults(t *testing.T) {
	o := parse(t)

	if o.debug {
		t.Error("debug = true, want false")
	}
	if o.leaderElection {
		t.Error("leaderElection = true, want false")
	}
	if o.syncInterval != time.Hour {
		t.Errorf("syncInterval = %v, want 1h", o.syncInterval)
	}
	if o.pollInterval != time.Minute {
		t.Errorf("pollInterval = %v, want 1m", o.pollInterval)
	}
	if o.maxReconcileRate != 10 {
		t.Errorf("maxReconcileRate = %d, want 10", o.maxReconcileRate)
	}
	if o.cacheInitTimeout != 5*time.Minute {
		t.Errorf("cacheInitTimeout = %v, want 5m", o.cacheInitTimeout)
	}
	if o.pollStateMetricInterval != 5*time.Second {
		t.Errorf("pollStateMetricInterval = %v, want 5s", o.pollStateMetricInterval)
	}
	if o.metricsBindAddress != ":8080" {
		t.Errorf("metricsBindAddress = %q, want \":8080\"", o.metricsBindAddress)
	}
	if !o.enableManagementPolicies {
		t.Error("enableManagementPolicies = false, want true")
	}
}

func TestParseOptionsAppliesOverrides(t *testing.T) {
	o := parse(t,
		"--debug",
		"--leader-election",
		"--sync=15m",
		"--poll=30s",
		"--max-reconcile-rate=42",
		"--cache-init-timeout=90s",
		"--poll-state-metric=1m",
		"--metrics-bind-address=127.0.0.1:9090",
		"--no-enable-management-policies",
	)

	if !o.debug || !o.leaderElection {
		t.Errorf("debug=%v leaderElection=%v, want both true", o.debug, o.leaderElection)
	}
	if o.syncInterval != 15*time.Minute {
		t.Errorf("syncInterval = %v, want 15m", o.syncInterval)
	}
	if o.pollInterval != 30*time.Second {
		t.Errorf("pollInterval = %v, want 30s", o.pollInterval)
	}
	if o.maxReconcileRate != 42 {
		t.Errorf("maxReconcileRate = %d, want 42", o.maxReconcileRate)
	}
	if o.cacheInitTimeout != 90*time.Second {
		t.Errorf("cacheInitTimeout = %v, want 90s", o.cacheInitTimeout)
	}
	if o.pollStateMetricInterval != time.Minute {
		t.Errorf("pollStateMetricInterval = %v, want 1m", o.pollStateMetricInterval)
	}
	if o.metricsBindAddress != "127.0.0.1:9090" {
		t.Errorf("metricsBindAddress = %q, want 127.0.0.1:9090", o.metricsBindAddress)
	}
	if o.enableManagementPolicies {
		t.Error("enableManagementPolicies = true, want false when explicitly disabled")
	}
}

func TestParseOptionsShortForms(t *testing.T) {
	o := parse(t, "-d", "-l", "-s", "2h")
	if !o.debug {
		t.Error("-d did not enable debug")
	}
	if !o.leaderElection {
		t.Error("-l did not enable leader election")
	}
	if o.syncInterval != 2*time.Hour {
		t.Errorf("syncInterval = %v, want 2h from -s", o.syncInterval)
	}
}

func TestParseOptionsReadsLeaderElectionEnvar(t *testing.T) {
	t.Setenv("LEADER_ELECTION", "true")
	if o := parse(t); !o.leaderElection {
		t.Error("LEADER_ELECTION=true did not enable leader election")
	}
}

func TestParseOptionsFlagBeatsEnvar(t *testing.T) {
	t.Setenv("LEADER_ELECTION", "true")
	if o := parse(t, "--no-leader-election"); o.leaderElection {
		t.Error("--no-leader-election should override LEADER_ELECTION=true")
	}
}

// The regression that motivated the refactor. --cache-init-timeout was parsed
// and logged but never reached the manager, because CacheSyncTimeout was
// hardcoded to 10 minutes. An operator raising it on a slow API server - exactly
// what the flag's own help text tells them to do - got nothing.
func TestCacheInitTimeoutReachesTheManager(t *testing.T) {
	for _, d := range []time.Duration{time.Minute, 90 * time.Second, 20 * time.Minute} {
		o := parse(t, "--cache-init-timeout="+d.String())
		got := o.managerOptions().Controller.CacheSyncTimeout
		if got != d {
			t.Errorf("--cache-init-timeout=%s produced CacheSyncTimeout %v; the flag must "+
				"reach the manager or raising it is a no-op", d, got)
		}
	}

	// And the default must not silently stay at the old hardcoded value.
	o := parse(t)
	if got := o.managerOptions().Controller.CacheSyncTimeout; got != 5*time.Minute {
		t.Errorf("default CacheSyncTimeout = %v, want 5m (the documented default)", got)
	}
}

func TestManagerOptionsLeaderElection(t *testing.T) {
	// Leader loss was observed when the renewal deadline was exceeded under
	// load, so these three values are load-bearing.
	mo := parse(t, "--leader-election").managerOptions()

	if !mo.LeaderElection {
		t.Error("LeaderElection = false, want true")
	}
	if mo.LeaderElectionID != "crossplane-leader-election-cp-provider-template" {
		t.Errorf("LeaderElectionID = %q", mo.LeaderElectionID)
	}
	if mo.LeaderElectionResourceLock != resourcelock.LeasesResourceLock {
		t.Errorf("LeaderElectionResourceLock = %v, want Leases only", mo.LeaderElectionResourceLock)
	}
	if mo.LeaseDuration == nil || *mo.LeaseDuration != 60*time.Second {
		t.Errorf("LeaseDuration = %v, want 60s", mo.LeaseDuration)
	}
	if mo.RenewDeadline == nil || *mo.RenewDeadline != 50*time.Second {
		t.Errorf("RenewDeadline = %v, want 50s", mo.RenewDeadline)
	}
	if mo.LeaseDuration != nil && mo.RenewDeadline != nil && *mo.RenewDeadline >= *mo.LeaseDuration {
		t.Error("RenewDeadline must be shorter than LeaseDuration or the lease expires before it renews")
	}
}

func TestManagerOptionsCarriesSyncAndMetrics(t *testing.T) {
	mo := parse(t, "--sync=7m", "--metrics-bind-address=127.0.0.1:1234").managerOptions()

	if mo.Cache.SyncPeriod == nil || *mo.Cache.SyncPeriod != 7*time.Minute {
		t.Errorf("Cache.SyncPeriod = %v, want 7m", mo.Cache.SyncPeriod)
	}
	if mo.Metrics.BindAddress != "127.0.0.1:1234" {
		t.Errorf("Metrics.BindAddress = %q, want 127.0.0.1:1234", mo.Metrics.BindAddress)
	}
}

func TestControllerOptions(t *testing.T) {
	log := logging.NewNopLogger()
	mrState := statemetrics.NewMRStateMetrics()

	co := parse(t, "--max-reconcile-rate=7", "--poll=45s", "--poll-state-metric=2m").controllerOptions(log, mrState)

	if co.MaxConcurrentReconciles != 7 {
		t.Errorf("MaxConcurrentReconciles = %d, want 7", co.MaxConcurrentReconciles)
	}
	if co.PollInterval != 45*time.Second {
		t.Errorf("PollInterval = %v, want 45s", co.PollInterval)
	}
	if co.GlobalRateLimiter == nil {
		t.Error("GlobalRateLimiter is nil")
	}
	if co.MetricOptions == nil {
		t.Fatal("MetricOptions is nil")
	}
	if co.MetricOptions.PollStateMetricInterval != 2*time.Minute {
		t.Errorf("PollStateMetricInterval = %v, want 2m", co.MetricOptions.PollStateMetricInterval)
	}
	if co.MetricOptions.MRStateMetrics != mrState {
		t.Error("MRStateMetrics was not passed through")
	}
}

func TestControllerOptionsManagementPoliciesFlag(t *testing.T) {
	log := logging.NewNopLogger()
	mrState := statemetrics.NewMRStateMetrics()

	on := parse(t).controllerOptions(log, mrState)
	if !on.Features.Enabled(features.EnableAlphaManagementPolicies) {
		t.Error("management policies should be enabled by default")
	}

	off := parse(t, "--no-enable-management-policies").controllerOptions(log, mrState)
	if off.Features.Enabled(features.EnableAlphaManagementPolicies) {
		t.Error("management policies should be disabled when the flag is false")
	}
}

// Every managed resource the provider runs a controller for needs a poll-state
// metric recorder. A list entry dropped here is invisible: the controller still
// works, the resource simply stops reporting state. Asserted against an explicit
// expected set so a deletion fails rather than passing quietly.
func TestStateManagedListsCoversEveryResource(t *testing.T) {
	want := map[string]bool{
		"AuthenticationFlowList":     true,
		"AuthorizationPolicyList":    true,
		"AuthzResourceList":          true,
		"ProtocolMapperList":         true,
		"ClientCertificateList":      true,
		"ClientInitialAccessList":    true,
		"ComponentList":              true,
		"RealmEventsConfigList":      true,
		"GroupList":                  true,
		"IdentityProviderList":       true,
		"RealmKeysList":              true,
		"ClientList":                 true,
		"ClientDefaultScopesList":    true,
		"ClientOptionalScopesList":   true,
		"RealmList":                  true,
		"RealmImportList":            true,
		"RoleList":                   true,
		"ClientRoleMappingList":      true,
		"ClientScopeMappingList":     true,
		"ClientScopeList":            true,
		"UserList":                   true,
		"GroupsList":                 true,
		"UserFederationProviderList": true,
	}

	got := map[string]int{}
	for _, l := range stateManagedLists() {
		name := reflect.TypeOf(l).Elem().Name()
		got[name]++
	}

	for name := range want {
		switch n := got[name]; {
		case n == 0:
			t.Errorf("%s has no poll-state metric recorder", name)
		case n > 1:
			t.Errorf("%s has %d recorders, want 1", name, n)
		}
	}
	for name, n := range got {
		if n > 0 && !want[name] {
			t.Errorf("unexpected recorder for %s", name)
		}
	}
	if len(got) != len(want) {
		t.Errorf("recorder set has %d entries, want %d", len(got), len(want))
	}
}

// Guards against the list returning something that will not compile as a
// ManagedList, and against a nil entry slipping in.
func TestStateManagedListsAreUsableManagedLists(t *testing.T) {
	lists := stateManagedLists()
	if len(lists) == 0 {
		t.Fatal("stateManagedLists returned nothing")
	}
	for i, l := range lists {
		if l == nil {
			t.Errorf("entry %d is nil", i)
			continue
		}
		if reflect.TypeOf(l).Kind() != reflect.Pointer {
			t.Errorf("entry %d is %T, want a pointer to a list type", i, l)
		}
	}
}
