package manager

import (
	"context"
	"testing"
	"time"

	"github.com/vessel/vessel/internal/config"
	"github.com/vessel/vessel/internal/health"
	"github.com/vessel/vessel/internal/store"
	"github.com/vessel/vessel/testutil"
)

// TestDeploy_RegistersWithMetricsCollector verifies that deployed containers
// are registered with the metrics collector.
func TestDeploy_RegistersWithMetricsCollector(t *testing.T) {
	rt := &mockRuntime{}
	st := testutil.TempStore(t)
	mgr := NewAppManager(rt, st, nil, testLogger())

	mc := health.NewMetricsCollector(rt, st, 15*time.Second, 24*time.Hour, testLogger())
	mgr.SetMetricsCollector(mc)

	appCfg := &config.AppConfig{
		Name:      "metrics-test-app",
		Image:     "alpine:latest",
		Instances: 2,
	}
	config.ApplyDefaults(&config.Config{Apps: []config.AppConfig{*appCfg}})

	deploy, err := mgr.DeployAppFromConfig(context.Background(), appCfg)
	if err != nil {
		t.Fatalf("deploy failed: %v", err)
	}
	if deploy.Status != store.DeployStatusActive {
		t.Errorf("expected active deploy, got %s", deploy.Status)
	}

	// Verify containers are registered with metrics collector
	registered := mc.GetRegisteredContainers()
	if len(registered) != 2 {
		t.Errorf("expected 2 containers registered with metrics collector, got %d", len(registered))
	}
}

// TestDeploy_NoMetricsCollector_Succeeds verifies deploy works without metrics collector.
func TestDeploy_NoMetricsCollector_Succeeds(t *testing.T) {
	rt := &mockRuntime{}
	st := testutil.TempStore(t)
	mgr := NewAppManager(rt, st, nil, testLogger())
	// No metrics collector set — should work fine

	appCfg := &config.AppConfig{
		Name:      "no-metrics-app",
		Image:     "alpine:latest",
		Instances: 1,
	}
	config.ApplyDefaults(&config.Config{Apps: []config.AppConfig{*appCfg}})

	deploy, err := mgr.DeployAppFromConfig(context.Background(), appCfg)
	if err != nil {
		t.Fatalf("deploy failed without metrics collector: %v", err)
	}
	if deploy.Status != store.DeployStatusActive {
		t.Errorf("expected active deploy, got %s", deploy.Status)
	}
}

// TestDeploy_DeregistersMetricsOnStop verifies that stopping a container deregisters from metrics.
func TestDeploy_DeregistersMetricsOnStop(t *testing.T) {
	rt := &mockRuntime{}
	st := testutil.TempStore(t)
	mgr := NewAppManager(rt, st, nil, testLogger())

	mc := health.NewMetricsCollector(rt, st, 15*time.Second, 24*time.Hour, testLogger())
	mgr.SetMetricsCollector(mc)

	appCfg := &config.AppConfig{
		Name:      "dereg-metrics-app",
		Image:     "alpine:latest",
		Instances: 1,
	}
	config.ApplyDefaults(&config.Config{Apps: []config.AppConfig{*appCfg}})

	_, err := mgr.DeployAppFromConfig(context.Background(), appCfg)
	if err != nil {
		t.Fatalf("deploy failed: %v", err)
	}

	// Verify registered
	if len(mc.GetRegisteredContainers()) != 1 {
		t.Fatal("expected 1 registered container")
	}

	// Remove app
	if err := mgr.RemoveApp(context.Background(), "dereg-metrics-app"); err != nil {
		t.Fatalf("remove failed: %v", err)
	}

	// Verify deregistered
	if len(mc.GetRegisteredContainers()) != 0 {
		t.Error("expected 0 registered containers after remove")
	}
}

// TestRollingDeploy_RegistersNewDeregistersOld verifies rolling deploy
// registers new containers and deregisters old ones with metrics collector.
func TestRollingDeploy_RegistersNewDeregistersOld(t *testing.T) {
	rt := &mockRuntime{}
	st := testutil.TempStore(t)
	mgr := NewAppManager(rt, st, nil, testLogger())

	mc := health.NewMetricsCollector(rt, st, 15*time.Second, 24*time.Hour, testLogger())
	mgr.SetMetricsCollector(mc)

	// First deploy
	appCfg := &config.AppConfig{
		Name:      "rolling-metrics-app",
		Image:     "alpine:latest",
		Instances: 2,
		Deploy:    config.DeployConfig{Strategy: "rolling"},
	}
	config.ApplyDefaults(&config.Config{Apps: []config.AppConfig{*appCfg}})

	_, err := mgr.DeployAppFromConfig(context.Background(), appCfg)
	if err != nil {
		t.Fatalf("first deploy failed: %v", err)
	}

	oldRegistered := mc.GetRegisteredContainers()
	if len(oldRegistered) != 2 {
		t.Fatalf("expected 2 containers after first deploy, got %d", len(oldRegistered))
	}

	// Rolling deploy with new image
	appCfg.Image = "alpine:3.19"
	_, err = mgr.DeployAppFromConfig(context.Background(), appCfg)
	if err != nil {
		t.Fatalf("rolling deploy failed: %v", err)
	}

	// Should still have 2 containers registered (new ones)
	newRegistered := mc.GetRegisteredContainers()
	if len(newRegistered) != 2 {
		t.Errorf("expected 2 containers after rolling deploy, got %d", len(newRegistered))
	}

	// Old containers should be deregistered (new ones should be different)
	for _, old := range oldRegistered {
		for _, new := range newRegistered {
			if old == new {
				t.Errorf("old container %s still registered after rolling deploy", old)
			}
		}
	}
}

// TestBlueGreenDeploy_RegistersNewDeregistersOld verifies blue-green deploy
// registers new and deregisters old containers with metrics collector.
func TestBlueGreenDeploy_RegistersNewDeregistersOld(t *testing.T) {
	rt := &mockRuntime{}
	st := testutil.TempStore(t)
	mgr := NewAppManager(rt, st, nil, testLogger())

	mc := health.NewMetricsCollector(rt, st, 15*time.Second, 24*time.Hour, testLogger())
	mgr.SetMetricsCollector(mc)

	// First deploy
	appCfg := &config.AppConfig{
		Name:      "bluegreen-met-app",
		Image:     "alpine:latest",
		Instances: 1,
		Deploy:    config.DeployConfig{Strategy: "blue-green"},
	}
	config.ApplyDefaults(&config.Config{Apps: []config.AppConfig{*appCfg}})

	_, err := mgr.DeployAppFromConfig(context.Background(), appCfg)
	if err != nil {
		t.Fatalf("first deploy failed: %v", err)
	}

	if len(mc.GetRegisteredContainers()) != 1 {
		t.Fatal("expected 1 container after first deploy")
	}

	// Blue-green deploy
	appCfg.Image = "alpine:3.19"
	_, err = mgr.DeployAppFromConfig(context.Background(), appCfg)
	if err != nil {
		t.Fatalf("blue-green deploy failed: %v", err)
	}

	if len(mc.GetRegisteredContainers()) != 1 {
		t.Errorf("expected 1 container after blue-green, got %d", len(mc.GetRegisteredContainers()))
	}
}

// TestSetGetMetricsCollector verifies the setter/getter for metrics collector.
func TestSetGetMetricsCollector(t *testing.T) {
	rt := &mockRuntime{}
	st := testutil.TempStore(t)
	mgr := NewAppManager(rt, st, nil, testLogger())

	if mgr.GetMetricsCollector() != nil {
		t.Error("expected nil metrics collector initially")
	}

	mc := health.NewMetricsCollector(rt, st, 15*time.Second, 24*time.Hour, testLogger())
	mgr.SetMetricsCollector(mc)

	if mgr.GetMetricsCollector() != mc {
		t.Error("expected metrics collector to be set")
	}
}
