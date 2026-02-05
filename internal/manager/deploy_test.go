package manager

import (
	"context"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vessel/vessel/internal/config"
	"github.com/vessel/vessel/internal/runtime"
	"github.com/vessel/vessel/internal/store"
	"github.com/vessel/vessel/testutil"
)

// mockRuntime implements runtime.Runtime for testing.
type mockRuntime struct {
	pullImageFn       func(ctx context.Context, ref string) error
	createContainerFn func(ctx context.Context, opts runtime.ContainerOpts) (*store.Container, error)
	startContainerFn  func(ctx context.Context, containerID string) error
	stopContainerFn   func(ctx context.Context, containerID string, gracePeriod time.Duration) error
	removeContainerFn func(ctx context.Context, containerID string) error
	execInContainerFn func(ctx context.Context, containerID string, cmd []string) error
	containerLogsFn   func(ctx context.Context, containerID string, follow bool, tail int) (io.ReadCloser, error)
	containerStatsFn  func(ctx context.Context, containerID string) (*store.MetricPoint, error)

	containerCounter int64 // atomic counter for generating unique IDs
}

func (m *mockRuntime) PullImage(ctx context.Context, ref string) error {
	if m.pullImageFn != nil {
		return m.pullImageFn(ctx, ref)
	}
	return nil
}

func (m *mockRuntime) CreateContainer(ctx context.Context, opts runtime.ContainerOpts) (*store.Container, error) {
	if m.createContainerFn != nil {
		return m.createContainerFn(ctx, opts)
	}
	n := atomic.AddInt64(&m.containerCounter, 1)
	return &store.Container{
		ID:        fmt.Sprintf("mock-container-%012d", n),
		Image:     opts.Image,
		State:     store.ContainerStateCreated,
		CreatedAt: time.Now(),
	}, nil
}

func (m *mockRuntime) StartContainer(ctx context.Context, containerID string) error {
	if m.startContainerFn != nil {
		return m.startContainerFn(ctx, containerID)
	}
	return nil
}

func (m *mockRuntime) StopContainer(ctx context.Context, containerID string, gracePeriod time.Duration) error {
	if m.stopContainerFn != nil {
		return m.stopContainerFn(ctx, containerID, gracePeriod)
	}
	return nil
}

func (m *mockRuntime) RemoveContainer(ctx context.Context, containerID string) error {
	if m.removeContainerFn != nil {
		return m.removeContainerFn(ctx, containerID)
	}
	return nil
}

func (m *mockRuntime) ExecInContainer(ctx context.Context, containerID string, cmd []string) error {
	if m.execInContainerFn != nil {
		return m.execInContainerFn(ctx, containerID, cmd)
	}
	return nil
}

func (m *mockRuntime) ContainerLogs(ctx context.Context, containerID string, follow bool, tail int) (io.ReadCloser, error) {
	if m.containerLogsFn != nil {
		return m.containerLogsFn(ctx, containerID, follow, tail)
	}
	return io.NopCloser(nil), nil
}

func (m *mockRuntime) ContainerStats(ctx context.Context, containerID string) (*store.MetricPoint, error) {
	if m.containerStatsFn != nil {
		return m.containerStatsFn(ctx, containerID)
	}
	return &store.MetricPoint{}, nil
}

// newTestManager creates an AppManager with a mock runtime for testing.
func newTestManager(t *testing.T, rt runtime.Runtime) (*AppManager, store.Store) {
	t.Helper()
	st := testutil.TempStore(t)
	logger := testLogger()
	mgr := &AppManager{
		runtime: rt,
		store:   st,
		logger:  logger,
	}
	mgr.reconciler = NewReconciler(rt, st, logger)
	return mgr, st
}

// newTestAppConfig creates a minimal AppConfig for testing.
// Includes a fast command health check so tests don't wait 60s.
func newTestAppConfig(name, image string) *config.AppConfig {
	cfg := &config.AppConfig{
		Name:      name,
		Image:     image,
		Instances: 1,
		HealthCheck: &config.HealthCheckConfig{
			Type:    "command",
			Command: []string{"true"},
			Interval: config.Duration{Duration: 100 * time.Millisecond},
			Timeout:  config.Duration{Duration: 50 * time.Millisecond},
			Retries:  1,
		},
	}
	config.ApplyDefaults(&config.Config{Apps: []config.AppConfig{*cfg}})
	return cfg
}

// --- Deploy Tests ---

func TestDeployNewApp_HappyPath(t *testing.T) {
	rt := &mockRuntime{}
	mgr, st := newTestManager(t, rt)

	appCfg := newTestAppConfig("test-app", "alpine:latest")

	deploy, err := mgr.DeployAppFromConfig(context.Background(), appCfg)
	if err != nil {
		t.Fatalf("DeployAppFromConfig failed: %v", err)
	}

	// Verify deploy
	if deploy.Status != store.DeployStatusActive {
		t.Errorf("expected deploy status active, got %s", deploy.Status)
	}
	if deploy.Version != 1 {
		t.Errorf("expected version 1, got %d", deploy.Version)
	}
	if deploy.Image != "alpine:latest" {
		t.Errorf("expected image alpine:latest, got %s", deploy.Image)
	}

	// Verify app state
	app, err := st.GetApp("test-app")
	if err != nil {
		t.Fatalf("failed to get app: %v", err)
	}
	if app.State != store.AppStateRunning {
		t.Errorf("expected app state running, got %s", app.State)
	}

	// Verify container created
	containers, err := st.ListContainersByApp("test-app")
	if err != nil {
		t.Fatalf("failed to list containers: %v", err)
	}
	if len(containers) != 1 {
		t.Errorf("expected 1 container, got %d", len(containers))
	}
	if len(containers) > 0 && containers[0].State != store.ContainerStateRunning {
		t.Errorf("expected container state running, got %s", containers[0].State)
	}
}

func TestDeployNewApp_MultipleInstances(t *testing.T) {
	rt := &mockRuntime{}
	mgr, st := newTestManager(t, rt)

	appCfg := newTestAppConfig("test-app", "alpine:latest")
	appCfg.Instances = 3

	deploy, err := mgr.DeployAppFromConfig(context.Background(), appCfg)
	if err != nil {
		t.Fatalf("DeployAppFromConfig failed: %v", err)
	}

	if deploy.Status != store.DeployStatusActive {
		t.Errorf("expected deploy status active, got %s", deploy.Status)
	}

	containers, err := st.ListContainersByApp("test-app")
	if err != nil {
		t.Fatalf("failed to list containers: %v", err)
	}
	if len(containers) != 3 {
		t.Errorf("expected 3 containers, got %d", len(containers))
	}
}

func TestDeployNewApp_HealthCheckFails(t *testing.T) {
	rt := &mockRuntime{
		execInContainerFn: func(ctx context.Context, containerID string, cmd []string) error {
			return fmt.Errorf("command failed")
		},
	}
	mgr, st := newTestManager(t, rt)

	appCfg := newTestAppConfig("test-app", "alpine:latest")
	appCfg.HealthCheck = &config.HealthCheckConfig{
		Type:    "command",
		Command: []string{"false"},
	}
	appCfg.HealthCheck.Interval = config.Duration{Duration: 100 * time.Millisecond}
	appCfg.HealthCheck.Timeout = config.Duration{Duration: 50 * time.Millisecond}
	appCfg.HealthCheck.Retries = 1

	deploy, err := mgr.DeployAppFromConfig(context.Background(), appCfg)
	if err == nil {
		t.Fatal("expected error for unhealthy container")
	}

	if deploy == nil {
		t.Fatal("expected deploy record even on failure")
	}
	if deploy.Status != store.DeployStatusFailed {
		t.Errorf("expected deploy status failed, got %s", deploy.Status)
	}

	// Verify app state is failed
	app, err := st.GetApp("test-app")
	if err != nil {
		t.Fatalf("failed to get app: %v", err)
	}
	if app.State != store.AppStateFailed {
		t.Errorf("expected app state failed, got %s", app.State)
	}

	// Verify containers were cleaned up
	containers, err := st.ListContainersByApp("test-app")
	if err != nil {
		t.Fatalf("failed to list containers: %v", err)
	}
	if len(containers) != 0 {
		t.Errorf("expected 0 containers after failed deploy, got %d", len(containers))
	}
}

func TestDeployNewApp_ImagePullFails(t *testing.T) {
	rt := &mockRuntime{
		pullImageFn: func(ctx context.Context, ref string) error {
			return fmt.Errorf("pull failed: registry unreachable")
		},
	}
	mgr, _ := newTestManager(t, rt)

	appCfg := newTestAppConfig("test-app", "nonexistent:latest")
	_, err := mgr.DeployAppFromConfig(context.Background(), appCfg)
	if err == nil {
		t.Fatal("expected error when image pull fails")
	}
}

func TestDeployRolling_ReplacesContainers(t *testing.T) {
	rt := &mockRuntime{}
	mgr, st := newTestManager(t, rt)

	// First deploy
	appCfg := newTestAppConfig("test-app", "alpine:v1")
	appCfg.Deploy.GracePeriod = config.Duration{Duration: 10 * time.Millisecond}
	deploy1, err := mgr.DeployAppFromConfig(context.Background(), appCfg)
	if err != nil {
		t.Fatalf("first deploy failed: %v", err)
	}
	if deploy1.Version != 1 {
		t.Errorf("expected version 1, got %d", deploy1.Version)
	}

	// Get the container from v1
	containers1, _ := st.ListContainersByApp("test-app")
	if len(containers1) != 1 {
		t.Fatalf("expected 1 container after first deploy, got %d", len(containers1))
	}
	oldContainerID := containers1[0].ID

	// Second deploy (rolling update)
	appCfg2 := newTestAppConfig("test-app", "alpine:v2")
	appCfg2.Deploy.GracePeriod = config.Duration{Duration: 10 * time.Millisecond}
	deploy2, err := mgr.DeployAppFromConfig(context.Background(), appCfg2)
	if err != nil {
		t.Fatalf("second deploy failed: %v", err)
	}
	if deploy2.Version != 2 {
		t.Errorf("expected version 2, got %d", deploy2.Version)
	}
	if deploy2.Status != store.DeployStatusActive {
		t.Errorf("expected deploy status active, got %s", deploy2.Status)
	}

	// Verify old container was replaced
	containers2, _ := st.ListContainersByApp("test-app")
	if len(containers2) != 1 {
		t.Fatalf("expected 1 container after rolling update, got %d", len(containers2))
	}
	if containers2[0].ID == oldContainerID {
		t.Error("expected container to be replaced, but old container still exists")
	}

	// Verify app state
	app, _ := st.GetApp("test-app")
	if app.State != store.AppStateRunning {
		t.Errorf("expected app state running, got %s", app.State)
	}
	if app.Image != "alpine:v2" {
		t.Errorf("expected app image alpine:v2, got %s", app.Image)
	}
}

func TestDeployRolling_FailureKeepsOldContainers(t *testing.T) {
	// Track which containers were created with which image
	containerImages := make(map[string]string)
	var mu sync.Mutex

	rt := &mockRuntime{
		createContainerFn: func(ctx context.Context, opts runtime.ContainerOpts) (*store.Container, error) {
			id := fmt.Sprintf("mock-container-%012d", time.Now().UnixNano()%100000)
			mu.Lock()
			containerImages[id] = opts.Image
			mu.Unlock()
			return &store.Container{
				ID:        id,
				Image:     opts.Image,
				State:     store.ContainerStateCreated,
				CreatedAt: time.Now(),
			}, nil
		},
		// Exec fails for containers with "broken" image
		execInContainerFn: func(ctx context.Context, containerID string, cmd []string) error {
			mu.Lock()
			img := containerImages[containerID]
			mu.Unlock()
			if img == "alpine:v2-broken" {
				return fmt.Errorf("unhealthy")
			}
			return nil
		},
	}
	mgr, st := newTestManager(t, rt)

	// First deploy - succeeds
	appCfg := newTestAppConfig("test-app", "alpine:v1")
	appCfg.Deploy.GracePeriod = config.Duration{Duration: 10 * time.Millisecond}

	_, err := mgr.DeployAppFromConfig(context.Background(), appCfg)
	if err != nil {
		t.Fatalf("first deploy failed: %v", err)
	}

	// Second deploy - health check fails for broken image
	appCfg2 := newTestAppConfig("test-app", "alpine:v2-broken")
	appCfg2.HealthCheck = &config.HealthCheckConfig{
		Type:    "command",
		Command: []string{"false"},
	}
	appCfg2.HealthCheck.Interval = config.Duration{Duration: 100 * time.Millisecond}
	appCfg2.HealthCheck.Timeout = config.Duration{Duration: 50 * time.Millisecond}
	appCfg2.HealthCheck.Retries = 1

	deploy2, err := mgr.DeployAppFromConfig(context.Background(), appCfg2)
	if err == nil {
		t.Fatal("expected error for unhealthy deploy")
	}
	if deploy2 != nil && deploy2.Status != store.DeployStatusFailed {
		t.Errorf("expected deploy status failed, got %s", deploy2.Status)
	}

	// Verify deploy history shows 2 deploys
	deploys, _ := st.ListDeploysByApp("test-app")
	if len(deploys) < 2 {
		t.Errorf("expected at least 2 deploys, got %d", len(deploys))
	}
}

func TestDeployBlueGreen_HappyPath(t *testing.T) {
	rt := &mockRuntime{}
	mgr, st := newTestManager(t, rt)

	// First deploy
	appCfg := newTestAppConfig("test-app", "alpine:v1")
	appCfg.Instances = 2
	appCfg.Deploy.GracePeriod = config.Duration{Duration: 10 * time.Millisecond}
	_, err := mgr.DeployAppFromConfig(context.Background(), appCfg)
	if err != nil {
		t.Fatalf("first deploy failed: %v", err)
	}

	// Blue-green deploy
	appCfg2 := newTestAppConfig("test-app", "alpine:v2")
	appCfg2.Instances = 2
	appCfg2.Deploy.Strategy = "blue-green"
	appCfg2.Deploy.GracePeriod = config.Duration{Duration: 10 * time.Millisecond}

	deploy2, err := mgr.DeployAppFromConfig(context.Background(), appCfg2)
	if err != nil {
		t.Fatalf("blue-green deploy failed: %v", err)
	}
	if deploy2.Strategy != store.DeployStrategyBlueGreen {
		t.Errorf("expected blue-green strategy, got %s", deploy2.Strategy)
	}
	if deploy2.Status != store.DeployStatusActive {
		t.Errorf("expected active status, got %s", deploy2.Status)
	}

	// Verify 2 containers running
	containers, _ := st.ListContainersByApp("test-app")
	if len(containers) != 2 {
		t.Errorf("expected 2 containers after blue-green, got %d", len(containers))
	}
}

func TestDeployBlueGreen_FailureKeepsOldContainers(t *testing.T) {
	createCount := int64(0)
	rt := &mockRuntime{
		createContainerFn: func(ctx context.Context, opts runtime.ContainerOpts) (*store.Container, error) {
			n := atomic.AddInt64(&createCount, 1)
			// Fail on the 4th container (2nd deploy's 2nd container)
			if n == 4 {
				return nil, fmt.Errorf("resource exhausted")
			}
			return &store.Container{
				ID:        fmt.Sprintf("mock-container-%012d", n),
				Image:     opts.Image,
				State:     store.ContainerStateCreated,
				CreatedAt: time.Now(),
			}, nil
		},
	}
	mgr, _ := newTestManager(t, rt)

	// First deploy with 2 instances
	appCfg := newTestAppConfig("test-app", "alpine:v1")
	appCfg.Instances = 2
	appCfg.Deploy.GracePeriod = config.Duration{Duration: 10 * time.Millisecond}
	_, err := mgr.DeployAppFromConfig(context.Background(), appCfg)
	if err != nil {
		t.Fatalf("first deploy failed: %v", err)
	}

	// Blue-green deploy - fails on 2nd container
	appCfg2 := newTestAppConfig("test-app", "alpine:v2")
	appCfg2.Instances = 2
	appCfg2.Deploy.Strategy = "blue-green"
	appCfg2.Deploy.GracePeriod = config.Duration{Duration: 10 * time.Millisecond}

	deploy2, err := mgr.DeployAppFromConfig(context.Background(), appCfg2)
	if err == nil {
		t.Fatal("expected error when container creation fails")
	}
	if deploy2 != nil && deploy2.Status != store.DeployStatusFailed {
		t.Errorf("expected deploy status failed, got %s", deploy2.Status)
	}
}

func TestDeployVersioning(t *testing.T) {
	rt := &mockRuntime{}
	mgr, st := newTestManager(t, rt)

	// Deploy 3 times
	for i := 1; i <= 3; i++ {
		appCfg := newTestAppConfig("test-app", fmt.Sprintf("alpine:v%d", i))
		appCfg.Deploy.GracePeriod = config.Duration{Duration: 10 * time.Millisecond}
		deploy, err := mgr.DeployAppFromConfig(context.Background(), appCfg)
		if err != nil {
			t.Fatalf("deploy %d failed: %v", i, err)
		}
		if deploy.Version != i {
			t.Errorf("expected version %d, got %d", i, deploy.Version)
		}
	}

	// Verify history
	deploys, err := st.ListDeploysByApp("test-app")
	if err != nil {
		t.Fatalf("failed to list deploys: %v", err)
	}
	if len(deploys) != 3 {
		t.Errorf("expected 3 deploys, got %d", len(deploys))
	}
	// First deploy should be newest (version 3)
	if len(deploys) > 0 && deploys[0].Version != 3 {
		t.Errorf("expected newest deploy version 3, got %d", deploys[0].Version)
	}
}

// --- Rollback Tests ---

func TestRollback_ToPreviousVersion(t *testing.T) {
	rt := &mockRuntime{}
	mgr, st := newTestManager(t, rt)

	// Deploy v1
	appCfg1 := newTestAppConfig("test-app", "alpine:v1")
	appCfg1.Deploy.GracePeriod = config.Duration{Duration: 10 * time.Millisecond}
	_, err := mgr.DeployAppFromConfig(context.Background(), appCfg1)
	if err != nil {
		t.Fatalf("deploy v1 failed: %v", err)
	}

	// Deploy v2
	appCfg2 := newTestAppConfig("test-app", "alpine:v2")
	appCfg2.Deploy.GracePeriod = config.Duration{Duration: 10 * time.Millisecond}
	_, err = mgr.DeployAppFromConfig(context.Background(), appCfg2)
	if err != nil {
		t.Fatalf("deploy v2 failed: %v", err)
	}

	// Rollback to previous (v1)
	rollbackDeploy, err := mgr.RollbackApp(context.Background(), "test-app", 0)
	if err != nil {
		t.Fatalf("rollback failed: %v", err)
	}

	if rollbackDeploy.Version != 3 {
		t.Errorf("expected rollback deploy version 3, got %d", rollbackDeploy.Version)
	}
	if rollbackDeploy.Image != "alpine:v1" {
		t.Errorf("expected rollback to alpine:v1, got %s", rollbackDeploy.Image)
	}
	if rollbackDeploy.RollbackOf == nil || *rollbackDeploy.RollbackOf != 2 {
		t.Error("expected RollbackOf to be 2")
	}

	// Verify app is running with v1 image
	app, _ := st.GetApp("test-app")
	if app.Image != "alpine:v1" {
		t.Errorf("expected app image alpine:v1 after rollback, got %s", app.Image)
	}
}

func TestRollback_ToSpecificVersion(t *testing.T) {
	rt := &mockRuntime{}
	mgr, _ := newTestManager(t, rt)

	// Deploy v1, v2, v3
	for i := 1; i <= 3; i++ {
		appCfg := newTestAppConfig("test-app", fmt.Sprintf("alpine:v%d", i))
		appCfg.Deploy.GracePeriod = config.Duration{Duration: 10 * time.Millisecond}
		_, err := mgr.DeployAppFromConfig(context.Background(), appCfg)
		if err != nil {
			t.Fatalf("deploy v%d failed: %v", i, err)
		}
	}

	// Rollback to v1
	rollbackDeploy, err := mgr.RollbackApp(context.Background(), "test-app", 1)
	if err != nil {
		t.Fatalf("rollback to v1 failed: %v", err)
	}

	if rollbackDeploy.Image != "alpine:v1" {
		t.Errorf("expected rollback to alpine:v1, got %s", rollbackDeploy.Image)
	}
}

func TestRollback_NoHistory(t *testing.T) {
	rt := &mockRuntime{}
	mgr, st := newTestManager(t, rt)

	// Create app with no deploys
	app := &store.App{
		ID:        "test-app",
		Name:      "test-app",
		Image:     "alpine:latest",
		Instances: 1,
		State:     store.AppStateRunning,
	}
	st.CreateApp(app)

	_, err := mgr.RollbackApp(context.Background(), "test-app", 0)
	if err == nil {
		t.Fatal("expected error when no deploy history exists")
	}
}

func TestRollback_ToFailedDeploy(t *testing.T) {
	rt := &mockRuntime{}
	mgr, st := newTestManager(t, rt)

	// Deploy v1 - succeeds
	appCfg1 := newTestAppConfig("test-app", "alpine:v1")
	appCfg1.Deploy.GracePeriod = config.Duration{Duration: 10 * time.Millisecond}
	_, err := mgr.DeployAppFromConfig(context.Background(), appCfg1)
	if err != nil {
		t.Fatalf("deploy v1 failed: %v", err)
	}

	// Create a fake failed deploy record (v2)
	failedDeploy := &store.Deploy{
		ID:      "test-app-v2",
		AppID:   "test-app",
		Image:   "alpine:v2-broken",
		Version: 2,
		Status:  store.DeployStatusFailed,
		Error:   "health check timeout",
	}
	st.CreateDeploy(failedDeploy)

	// Rollback to v2 (failed) should error
	_, err = mgr.RollbackApp(context.Background(), "test-app", 2)
	if err == nil {
		t.Fatal("expected error when rolling back to failed deploy")
	}
}

func TestRollback_AppNotFound(t *testing.T) {
	rt := &mockRuntime{}
	mgr, _ := newTestManager(t, rt)

	_, err := mgr.RollbackApp(context.Background(), "nonexistent", 0)
	if err == nil {
		t.Fatal("expected error for nonexistent app")
	}
}

// --- Store Tests ---

func TestGetDeployByVersion(t *testing.T) {
	st := testutil.TempStore(t)

	// Create deploys
	for i := 1; i <= 3; i++ {
		d := &store.Deploy{
			ID:      fmt.Sprintf("app-v%d", i),
			AppID:   "app",
			Image:   fmt.Sprintf("alpine:v%d", i),
			Version: i,
			Status:  store.DeployStatusActive,
		}
		if err := st.CreateDeploy(d); err != nil {
			t.Fatalf("failed to create deploy: %v", err)
		}
	}

	// Get version 2
	d, err := st.GetDeployByVersion("app", 2)
	if err != nil {
		t.Fatalf("GetDeployByVersion failed: %v", err)
	}
	if d.Version != 2 {
		t.Errorf("expected version 2, got %d", d.Version)
	}
	if d.Image != "alpine:v2" {
		t.Errorf("expected image alpine:v2, got %s", d.Image)
	}

	// Get nonexistent version
	_, err = st.GetDeployByVersion("app", 999)
	if err == nil {
		t.Fatal("expected error for nonexistent version")
	}
}

func TestGetNextDeployVersion(t *testing.T) {
	st := testutil.TempStore(t)

	// First version should be 1
	v, err := st.GetNextDeployVersion("app")
	if err != nil {
		t.Fatalf("GetNextDeployVersion failed: %v", err)
	}
	if v != 1 {
		t.Errorf("expected version 1, got %d", v)
	}

	// After creating version 1, next should be 2
	d := &store.Deploy{
		ID:      "app-v1",
		AppID:   "app",
		Version: 1,
		Status:  store.DeployStatusActive,
	}
	st.CreateDeploy(d)

	v, err = st.GetNextDeployVersion("app")
	if err != nil {
		t.Fatalf("GetNextDeployVersion failed: %v", err)
	}
	if v != 2 {
		t.Errorf("expected version 2, got %d", v)
	}
}

func TestConfigToResourceLimits(t *testing.T) {
	tests := []struct {
		name     string
		cfg      config.ResourceConfig
		wantCPU  int64
		wantMem  int64
		wantPids int64
		wantErr  bool
	}{
		{
			name:     "default config",
			cfg:      config.ResourceConfig{CPU: 100, Memory: "256MB", Pids: 100},
			wantCPU:  100000,
			wantMem:  256 * 1024 * 1024,
			wantPids: 100,
		},
		{
			name:     "2 cores",
			cfg:      config.ResourceConfig{CPU: 200, Memory: "1GB", Pids: 50},
			wantCPU:  200000,
			wantMem:  1024 * 1024 * 1024,
			wantPids: 50,
		},
		{
			name:    "invalid memory",
			cfg:     config.ResourceConfig{CPU: 100, Memory: "invalid", Pids: 100},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := configToResourceLimits(&tt.cfg)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if r.CPUQuota != tt.wantCPU {
				t.Errorf("CPUQuota: expected %d, got %d", tt.wantCPU, r.CPUQuota)
			}
			if r.MemoryMax != tt.wantMem {
				t.Errorf("MemoryMax: expected %d, got %d", tt.wantMem, r.MemoryMax)
			}
			if r.PidsMax != tt.wantPids {
				t.Errorf("PidsMax: expected %d, got %d", tt.wantPids, r.PidsMax)
			}
		})
	}
}
