package manager

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	vessel "github.com/vessel/vessel/internal"
	"github.com/vessel/vessel/internal/store"
	"github.com/vessel/vessel/testutil"
)

// testLogger returns a logger that discards all output (for quiet tests).
func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestListApps_Empty(t *testing.T) {
	st := testutil.TempStore(t)
	mgr := &AppManager{
		store: st,
	}

	apps, err := mgr.ListApps(context.Background())
	if err != nil {
		t.Fatalf("ListApps failed: %v", err)
	}
	if len(apps) != 0 {
		t.Errorf("expected 0 apps, got %d", len(apps))
	}
}

func TestListApps_WithApps(t *testing.T) {
	st := testutil.TempStore(t)
	mgr := &AppManager{
		store: st,
	}

	// Create some apps
	for _, name := range []string{"app-a", "app-b", "app-c"} {
		app := &store.App{
			ID:        name,
			Name:      name,
			Image:     "alpine:latest",
			Instances: 1,
			State:     store.AppStateRunning,
		}
		if err := st.CreateApp(app); err != nil {
			t.Fatalf("failed to create app %s: %v", name, err)
		}
	}

	apps, err := mgr.ListApps(context.Background())
	if err != nil {
		t.Fatalf("ListApps failed: %v", err)
	}
	if len(apps) != 3 {
		t.Errorf("expected 3 apps, got %d", len(apps))
	}
}

func TestGetApp_Exists(t *testing.T) {
	st := testutil.TempStore(t)
	mgr := &AppManager{
		store: st,
	}

	app := &store.App{
		ID:        "myapp",
		Name:      "myapp",
		Image:     "nginx:latest",
		Instances: 2,
		State:     store.AppStateRunning,
	}
	if err := st.CreateApp(app); err != nil {
		t.Fatalf("failed to create app: %v", err)
	}

	got, err := mgr.GetApp(context.Background(), "myapp")
	if err != nil {
		t.Fatalf("GetApp failed: %v", err)
	}
	if got.Name != "myapp" {
		t.Errorf("expected name 'myapp', got '%s'", got.Name)
	}
	if got.Image != "nginx:latest" {
		t.Errorf("expected image 'nginx:latest', got '%s'", got.Image)
	}
	if got.Instances != 2 {
		t.Errorf("expected 2 instances, got %d", got.Instances)
	}
}

func TestGetApp_NotFound(t *testing.T) {
	st := testutil.TempStore(t)
	mgr := &AppManager{
		store: st,
	}

	_, err := mgr.GetApp(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent app")
	}
}

func TestGetContainers_NoContainers(t *testing.T) {
	st := testutil.TempStore(t)
	mgr := &AppManager{
		store: st,
	}

	app := &store.App{
		ID:        "myapp",
		Name:      "myapp",
		Image:     "alpine:latest",
		Instances: 1,
		State:     store.AppStateRunning,
	}
	if err := st.CreateApp(app); err != nil {
		t.Fatalf("failed to create app: %v", err)
	}

	containers, err := mgr.GetContainers(context.Background(), "myapp")
	if err != nil {
		t.Fatalf("GetContainers failed: %v", err)
	}
	if len(containers) != 0 {
		t.Errorf("expected 0 containers, got %d", len(containers))
	}
}

func TestGetContainers_WithContainers(t *testing.T) {
	st := testutil.TempStore(t)
	mgr := &AppManager{
		store: st,
	}

	app := &store.App{
		ID:        "myapp",
		Name:      "myapp",
		Image:     "alpine:latest",
		Instances: 2,
		State:     store.AppStateRunning,
	}
	if err := st.CreateApp(app); err != nil {
		t.Fatalf("failed to create app: %v", err)
	}

	// Create containers
	for i := 0; i < 2; i++ {
		c := &store.Container{
			ID:    "container-" + string(rune('a'+i)),
			AppID: "myapp",
			Image: "alpine:latest",
			State: store.ContainerStateRunning,
		}
		if err := st.CreateContainer(c); err != nil {
			t.Fatalf("failed to create container: %v", err)
		}
	}

	containers, err := mgr.GetContainers(context.Background(), "myapp")
	if err != nil {
		t.Fatalf("GetContainers failed: %v", err)
	}
	if len(containers) != 2 {
		t.Errorf("expected 2 containers, got %d", len(containers))
	}
}

func TestStopApp_UpdatesState(t *testing.T) {
	st := testutil.TempStore(t)
	mgr := &AppManager{
		store:  st,
		logger: testLogger(),
	}

	app := &store.App{
		ID:        "myapp",
		Name:      "myapp",
		Image:     "alpine:latest",
		Instances: 1,
		State:     store.AppStateStopped, // Already stopped
	}
	if err := st.CreateApp(app); err != nil {
		t.Fatalf("failed to create app: %v", err)
	}

	// Stopping an already-stopped app should be a no-op
	err := mgr.StopApp(context.Background(), "myapp")
	if err != nil {
		t.Fatalf("StopApp failed: %v", err)
	}

	got, _ := st.GetApp("myapp")
	if got.State != store.AppStateStopped {
		t.Errorf("expected state stopped, got %s", got.State)
	}
}

func TestStopApp_NotFound(t *testing.T) {
	st := testutil.TempStore(t)
	mgr := &AppManager{
		store:  st,
		logger: testLogger(),
	}

	err := mgr.StopApp(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent app")
	}
}

func TestRemoveApp_NotFound(t *testing.T) {
	st := testutil.TempStore(t)
	mgr := &AppManager{
		store:  st,
		logger: testLogger(),
	}

	err := mgr.RemoveApp(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent app")
	}
}

func TestBuildEnvList(t *testing.T) {
	tests := []struct {
		name     string
		env      map[string]string
		expected int
	}{
		{"nil map", nil, 0},
		{"empty map", map[string]string{}, 0},
		{"single var", map[string]string{"KEY": "val"}, 1},
		{"multiple vars", map[string]string{"A": "1", "B": "2", "C": "3"}, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildEnvList(tt.env)
			if len(result) != tt.expected {
				t.Errorf("expected %d items, got %d", tt.expected, len(result))
			}
		})
	}
}

func TestGetDeployHistory_AppNotFound(t *testing.T) {
	st := testutil.TempStore(t)
	mgr := &AppManager{
		store: st,
	}

	_, err := mgr.GetDeployHistory(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent app")
	}
}

func TestGetDeployHistory_WithDeploys(t *testing.T) {
	st := testutil.TempStore(t)
	mgr := &AppManager{
		store: st,
	}

	app := &store.App{
		ID:        "myapp",
		Name:      "myapp",
		Image:     "alpine:latest",
		Instances: 1,
		State:     store.AppStateRunning,
	}
	if err := st.CreateApp(app); err != nil {
		t.Fatalf("failed to create app: %v", err)
	}

	// Create deploys
	for i := 1; i <= 3; i++ {
		deploy := &store.Deploy{
			ID:      "deploy-" + string(rune('0'+i)),
			AppID:   "myapp",
			Image:   "alpine:latest",
			Version: i,
			Status:  store.DeployStatusActive,
		}
		if err := st.CreateDeploy(deploy); err != nil {
			t.Fatalf("failed to create deploy: %v", err)
		}
	}

	deploys, err := mgr.GetDeployHistory(context.Background(), "myapp")
	if err != nil {
		t.Fatalf("GetDeployHistory failed: %v", err)
	}
	if len(deploys) != 3 {
		t.Errorf("expected 3 deploys, got %d", len(deploys))
	}
}

func TestRollbackApp_AppNotFound(t *testing.T) {
	st := testutil.TempStore(t)
	mgr := &AppManager{
		store:  st,
		logger: testLogger(),
	}

	_, err := mgr.RollbackApp(context.Background(), "nonexistent", 1)
	if err == nil {
		t.Fatal("expected error for nonexistent app")
	}
}

// Reconciler tests

func TestReconciler_BackoffDoubles(t *testing.T) {
	r := &Reconciler{
		backoffs: make(map[string]*backoffState),
	}

	containerID := "test-container-123456789012"

	// First failure
	r.updateBackoff(containerID, false)
	bs := r.backoffs[containerID]
	if bs.delay != 2*initialBackoff {
		t.Errorf("expected delay %v after first failure, got %v", 2*initialBackoff, bs.delay)
	}

	// Second failure
	r.updateBackoff(containerID, false)
	bs = r.backoffs[containerID]
	if bs.delay != 4*initialBackoff {
		t.Errorf("expected delay %v after second failure, got %v", 4*initialBackoff, bs.delay)
	}

	// Third failure
	r.updateBackoff(containerID, false)
	bs = r.backoffs[containerID]
	if bs.delay != 8*initialBackoff {
		t.Errorf("expected delay %v after third failure, got %v", 8*initialBackoff, bs.delay)
	}
}

func TestReconciler_BackoffCapped(t *testing.T) {
	r := &Reconciler{
		backoffs: make(map[string]*backoffState),
	}

	containerID := "test-container-123456789012"

	// Simulate many failures to hit the cap
	for i := 0; i < 20; i++ {
		r.updateBackoff(containerID, false)
	}

	bs := r.backoffs[containerID]
	if bs.delay > maxBackoff {
		t.Errorf("backoff delay %v exceeded max %v", bs.delay, maxBackoff)
	}
	if bs.delay != maxBackoff {
		t.Errorf("expected delay to be capped at %v, got %v", maxBackoff, bs.delay)
	}
}

func TestReconciler_BackoffReset(t *testing.T) {
	r := &Reconciler{
		backoffs: make(map[string]*backoffState),
	}

	containerID := "test-container-123456789012"

	// Add some failures
	r.updateBackoff(containerID, false)
	r.updateBackoff(containerID, false)

	// Success should clear backoff
	r.updateBackoff(containerID, true)
	if _, ok := r.backoffs[containerID]; ok {
		t.Error("expected backoff to be cleared after success")
	}
}

func TestReconciler_ShouldRestart_FirstFailure(t *testing.T) {
	r := &Reconciler{
		backoffs: make(map[string]*backoffState),
	}

	c := &store.Container{
		ID:    "test-container-123456789012",
		State: store.ContainerStateFailed,
	}

	// First failure should restart immediately
	if !r.shouldRestart(c) {
		t.Error("expected shouldRestart to return true for first failure")
	}
}

func TestReconciler_ShouldRestart_RespectsBackoff(t *testing.T) {
	r := &Reconciler{
		backoffs: make(map[string]*backoffState),
	}

	containerID := "test-container-123456789012"
	c := &store.Container{
		ID:    containerID,
		State: store.ContainerStateFailed,
	}

	// First restart
	r.shouldRestart(c)

	// Set backoff with future retry time
	r.mu.Lock()
	r.backoffs[containerID].nextRetry = time.Now().Add(1 * time.Hour)
	r.mu.Unlock()

	// Should not restart during backoff
	if r.shouldRestart(c) {
		t.Error("expected shouldRestart to return false during backoff")
	}
}

func TestReconciler_IsProcessAlive_DeadPID(t *testing.T) {
	r := &Reconciler{}

	// PID 0 or negative should be dead
	if r.isProcessAlive(0) {
		t.Error("expected PID 0 to be dead")
	}
	if r.isProcessAlive(-1) {
		t.Error("expected PID -1 to be dead")
	}

	// Very high PID should not exist
	if r.isProcessAlive(999999999) {
		t.Error("expected very high PID to be dead")
	}
}

func TestReconciler_IsProcessAlive_CurrentPID(t *testing.T) {
	r := &Reconciler{}

	// PID 1 should always exist
	if !r.isProcessAlive(1) {
		t.Error("expected PID 1 to be alive")
	}
}

func TestReconciler_EnsureNoRunning_StoppedApp(t *testing.T) {
	st := testutil.TempStore(t)
	r := &Reconciler{
		store:    st,
		logger:   testLogger(),
		backoffs: make(map[string]*backoffState),
	}

	app := &store.App{
		ID:        "myapp",
		Name:      "myapp",
		Image:     "alpine:latest",
		Instances: 1,
		State:     store.AppStateStopped,
	}
	if err := st.CreateApp(app); err != nil {
		t.Fatalf("failed to create app: %v", err)
	}

	// No containers — should succeed
	err := r.ensureNoRunning(context.Background(), app)
	if err != nil {
		t.Fatalf("ensureNoRunning failed: %v", err)
	}
}

// Suppress the unused import of vessel
var _ = vessel.ErrAppNotFound
