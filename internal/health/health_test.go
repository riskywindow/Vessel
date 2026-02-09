package health

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vessel/vessel/internal/runtime"
	"github.com/vessel/vessel/internal/store"
	"github.com/vessel/vessel/testutil"
)

// testLogger returns a logger that discards all output.
func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// mockRuntime implements runtime.Runtime for health check testing.
type mockRuntime struct {
	execFn           func(ctx context.Context, containerID string, cmd []string) error
	containerCounter int64
}

func (m *mockRuntime) PullImage(ctx context.Context, ref string) error      { return nil }
func (m *mockRuntime) CreateContainer(ctx context.Context, opts runtime.ContainerOpts) (*store.Container, error) {
	n := atomic.AddInt64(&m.containerCounter, 1)
	return &store.Container{
		ID:        fmt.Sprintf("mock-container-%012d", n),
		Image:     opts.Image,
		State:     store.ContainerStateCreated,
		CreatedAt: time.Now(),
	}, nil
}
func (m *mockRuntime) StartContainer(ctx context.Context, containerID string) error      { return nil }
func (m *mockRuntime) StopContainer(ctx context.Context, containerID string, gp time.Duration) error {
	return nil
}
func (m *mockRuntime) RemoveContainer(ctx context.Context, containerID string) error     { return nil }
func (m *mockRuntime) ContainerLogs(ctx context.Context, containerID string, follow bool, tail int) (io.ReadCloser, error) {
	return io.NopCloser(nil), nil
}
func (m *mockRuntime) ContainerStats(ctx context.Context, containerID string) (*store.MetricPoint, error) {
	return &store.MetricPoint{}, nil
}
func (m *mockRuntime) GetContainerPID(containerID string) (int, error) { return 12345, nil }
func (m *mockRuntime) ExecInContainer(ctx context.Context, containerID string, cmd []string) error {
	if m.execFn != nil {
		return m.execFn(ctx, containerID, cmd)
	}
	return nil
}

// --- Health Check Tests ---

func TestExecuteHTTPCheck_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	err := ExecuteHTTPCheck(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestExecuteHTTPCheck_Failure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	err := ExecuteHTTPCheck(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestExecuteTCPCheck_Success(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	err = ExecuteTCPCheck(context.Background(), ln.Addr().String())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestExecuteTCPCheck_Failure(t *testing.T) {
	// Use a port that's almost certainly not listening
	err := ExecuteTCPCheck(context.Background(), "127.0.0.1:1")
	if err == nil {
		t.Fatal("expected error for connection refused")
	}
}

func TestExecuteCommandCheck_Success(t *testing.T) {
	rt := &mockRuntime{}
	err := ExecuteCommandCheck(context.Background(), rt, "container-123456", "true")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestExecuteCommandCheck_Failure(t *testing.T) {
	rt := &mockRuntime{
		execFn: func(ctx context.Context, containerID string, cmd []string) error {
			return fmt.Errorf("exec failed")
		},
	}
	err := ExecuteCommandCheck(context.Background(), rt, "container-123456", "false")
	if err == nil {
		t.Fatal("expected error for failed command")
	}
}

func TestExecuteCheck_Dispatch(t *testing.T) {
	rt := &mockRuntime{}

	// Command check succeeds
	check := store.HealthCheck{
		Type:    store.HealthCheckCommand,
		Target:  "true",
		Timeout: 5 * time.Second,
	}
	err := ExecuteCheck(context.Background(), rt, "container-123456", check)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Unknown type fails
	check.Type = "unknown"
	err = ExecuteCheck(context.Background(), rt, "container-123456", check)
	if err == nil {
		t.Fatal("expected error for unknown check type")
	}
}

// --- Health Monitor Tests ---

func TestMonitor_RegisterDeregister(t *testing.T) {
	rt := &mockRuntime{}
	st := testutil.TempStore(t)
	mon := NewHealthMonitor(rt, st, nil, HealthMonitorConfig{}, testLogger())

	containerID := "test-container-123456"
	check := store.HealthCheck{
		Type:    store.HealthCheckCommand,
		Target:  "true",
		Timeout: 5 * time.Second,
	}

	if err := mon.RegisterContainer(containerID, "app-1", check); err != nil {
		t.Fatalf("register failed: %v", err)
	}

	status, err := mon.GetStatus(containerID)
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}
	if status == nil {
		t.Fatal("expected status, got nil")
	}
	if status.Status != store.HealthStatusUnknown {
		t.Errorf("expected unknown status, got %s", status.Status)
	}

	if err := mon.DeregisterContainer(containerID); err != nil {
		t.Fatalf("deregister failed: %v", err)
	}

	status, _ = mon.GetStatus(containerID)
	if status != nil {
		t.Error("expected nil after deregister")
	}
}

func TestMonitor_GetAllStatus(t *testing.T) {
	rt := &mockRuntime{}
	st := testutil.TempStore(t)
	mon := NewHealthMonitor(rt, st, nil, HealthMonitorConfig{}, testLogger())

	check := store.HealthCheck{Type: store.HealthCheckCommand, Target: "true", Timeout: 5 * time.Second}
	mon.RegisterContainer("container-aaaaaa-111111", "app-1", check)
	mon.RegisterContainer("container-bbbbbb-222222", "app-2", check)

	all := mon.GetAllStatus()
	if len(all) != 2 {
		t.Errorf("expected 2 statuses, got %d", len(all))
	}
}

func TestMonitor_UnhealthyThreshold(t *testing.T) {
	rt := &mockRuntime{
		execFn: func(ctx context.Context, containerID string, cmd []string) error {
			return fmt.Errorf("process dead")
		},
	}
	st := testutil.TempStore(t)
	cfg := HealthMonitorConfig{
		CheckInterval:      50 * time.Millisecond,
		UnhealthyThreshold: 3,
		HealthyThreshold:   1,
	}
	mon := NewHealthMonitor(rt, st, nil, cfg, testLogger())

	containerID := "test-container-unhealthy"
	check := store.HealthCheck{
		Type:    store.HealthCheckCommand,
		Target:  "false",
		Timeout: 5 * time.Second,
	}
	mon.RegisterContainer(containerID, "app-1", check)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mon.Start(ctx)
	defer mon.Stop()

	// Wait for enough checks to trigger unhealthy
	time.Sleep(250 * time.Millisecond)

	status, _ := mon.GetStatus(containerID)
	if status == nil {
		t.Fatal("expected status, got nil")
	}
	if status.Status != store.HealthStatusUnhealthy {
		t.Errorf("expected unhealthy, got %s (consecutive fails: %d)", status.Status, status.ConsecutiveFails)
	}
}

func TestMonitor_HealthyThreshold(t *testing.T) {
	rt := &mockRuntime{} // exec succeeds by default
	st := testutil.TempStore(t)
	cfg := HealthMonitorConfig{
		CheckInterval:      50 * time.Millisecond,
		UnhealthyThreshold: 3,
		HealthyThreshold:   2, // Need 2 consecutive successes
	}
	mon := NewHealthMonitor(rt, st, nil, cfg, testLogger())

	containerID := "test-container-healthy12"
	check := store.HealthCheck{
		Type:    store.HealthCheckCommand,
		Target:  "true",
		Timeout: 5 * time.Second,
	}
	mon.RegisterContainer(containerID, "app-1", check)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mon.Start(ctx)
	defer mon.Stop()

	// Wait for 2+ check intervals
	time.Sleep(200 * time.Millisecond)

	status, _ := mon.GetStatus(containerID)
	if status == nil {
		t.Fatal("expected status, got nil")
	}
	if status.Status != store.HealthStatusHealthy {
		t.Errorf("expected healthy, got %s", status.Status)
	}
}

func TestMonitor_EventEmission(t *testing.T) {
	var healthy atomic.Bool
	rt := &mockRuntime{
		execFn: func(ctx context.Context, containerID string, cmd []string) error {
			if healthy.Load() {
				return nil
			}
			return fmt.Errorf("unhealthy")
		},
	}
	st := testutil.TempStore(t)
	cfg := HealthMonitorConfig{
		CheckInterval:      50 * time.Millisecond,
		UnhealthyThreshold: 2,
		HealthyThreshold:   1,
	}
	mon := NewHealthMonitor(rt, st, nil, cfg, testLogger())

	containerID := "test-container-events12"
	check := store.HealthCheck{
		Type:    store.HealthCheckCommand,
		Target:  "check",
		Timeout: 5 * time.Second,
	}
	mon.RegisterContainer(containerID, "app-1", check)

	ch := mon.Subscribe()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mon.Start(ctx)
	defer mon.Stop()

	// Wait for unhealthy event
	var events []HealthEvent
	timer := time.NewTimer(500 * time.Millisecond)
	for {
		select {
		case evt := <-ch:
			events = append(events, evt)
			if evt.NewStatus == store.HealthStatusUnhealthy {
				goto checkEvents
			}
		case <-timer.C:
			goto checkEvents
		}
	}

checkEvents:
	if len(events) == 0 {
		t.Fatal("expected at least one event")
	}

	found := false
	for _, e := range events {
		if e.NewStatus == store.HealthStatusUnhealthy {
			found = true
			if e.ContainerID != containerID {
				t.Errorf("wrong container in event: %s", e.ContainerID)
			}
		}
	}
	if !found {
		t.Error("did not receive unhealthy event")
	}
}

func TestMonitor_Subscription(t *testing.T) {
	rt := &mockRuntime{}
	st := testutil.TempStore(t)
	mon := NewHealthMonitor(rt, st, nil, HealthMonitorConfig{}, testLogger())

	ch := mon.Subscribe()
	if ch == nil {
		t.Fatal("subscribe returned nil")
	}

	mon.Unsubscribe(ch)

	// Verify unsubscribed (channel should be closed)
	_, ok := <-ch
	if ok {
		t.Error("expected channel to be closed after unsubscribe")
	}
}

func TestMonitor_DeregisterDuringCheck(t *testing.T) {
	rt := &mockRuntime{}
	st := testutil.TempStore(t)
	cfg := HealthMonitorConfig{
		CheckInterval:      50 * time.Millisecond,
		UnhealthyThreshold: 3,
		HealthyThreshold:   1,
	}
	mon := NewHealthMonitor(rt, st, nil, cfg, testLogger())

	containerID := "test-container-deregist"
	check := store.HealthCheck{
		Type:    store.HealthCheckCommand,
		Target:  "true",
		Timeout: 5 * time.Second,
	}
	mon.RegisterContainer(containerID, "app-1", check)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mon.Start(ctx)
	defer mon.Stop()

	// Deregister while running — should not panic
	time.Sleep(100 * time.Millisecond)
	mon.DeregisterContainer(containerID)
	time.Sleep(100 * time.Millisecond)

	status, _ := mon.GetStatus(containerID)
	if status != nil {
		t.Error("expected nil status after deregister")
	}
}

func TestMonitor_StopCleanup(t *testing.T) {
	rt := &mockRuntime{}
	st := testutil.TempStore(t)
	mon := NewHealthMonitor(rt, st, nil, HealthMonitorConfig{CheckInterval: 50 * time.Millisecond}, testLogger())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mon.Start(ctx)

	ch := mon.Subscribe()
	mon.Stop()

	// Subscriber channel should be closed
	_, ok := <-ch
	if ok {
		t.Error("expected subscriber channel to be closed on Stop")
	}
}

func TestMonitor_DefaultConfig(t *testing.T) {
	rt := &mockRuntime{}
	st := testutil.TempStore(t)
	mon := NewHealthMonitor(rt, st, nil, HealthMonitorConfig{}, testLogger())

	if mon.config.CheckInterval != 10*time.Second {
		t.Errorf("expected default 10s interval, got %v", mon.config.CheckInterval)
	}
	if mon.config.UnhealthyThreshold != 3 {
		t.Errorf("expected default threshold 3, got %d", mon.config.UnhealthyThreshold)
	}
	if mon.config.HealthyThreshold != 1 {
		t.Errorf("expected default threshold 1, got %d", mon.config.HealthyThreshold)
	}
}

func TestMonitor_RestarterIntegration(t *testing.T) {
	rt := &mockRuntime{
		execFn: func(ctx context.Context, containerID string, cmd []string) error {
			return fmt.Errorf("dead")
		},
	}
	st := testutil.TempStore(t)

	var restartRequested atomic.Bool
	restarter := NewAutoRestarter(st, nil, testLogger())
	// Override to capture restart
	restarter.pending = make(chan RestartRequest, 10)

	cfg := HealthMonitorConfig{
		CheckInterval:      50 * time.Millisecond,
		UnhealthyThreshold: 2,
		HealthyThreshold:   1,
	}
	mon := NewHealthMonitor(rt, st, restarter, cfg, testLogger())

	containerID := "test-container-restrt12"
	check := store.HealthCheck{
		Type:    store.HealthCheckCommand,
		Target:  "check",
		Timeout: 5 * time.Second,
	}
	mon.RegisterContainer(containerID, "app-1", check)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mon.Start(ctx)
	defer mon.Stop()

	// Wait for unhealthy + restart request
	timer := time.NewTimer(500 * time.Millisecond)
	select {
	case req := <-restarter.pending:
		restartRequested.Store(true)
		if req.ContainerID != containerID {
			t.Errorf("wrong container in restart: %s", req.ContainerID)
		}
	case <-timer.C:
	}

	if !restartRequested.Load() {
		t.Error("expected restart to be requested")
	}
}

// --- Auto-Restarter Tests ---

func TestRestarter_CalculateDelay(t *testing.T) {
	r := &AutoRestarter{
		InitialDelay: 1 * time.Second,
		MaxDelay:     5 * time.Minute,
		Multiplier:   2.0,
	}

	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{1, 1 * time.Second},
		{2, 2 * time.Second},
		{3, 4 * time.Second},
		{4, 8 * time.Second},
		{5, 16 * time.Second},
		{10, 5 * time.Minute}, // Capped
		{20, 5 * time.Minute}, // Still capped
	}

	for _, tt := range tests {
		got := r.CalculateDelay(tt.attempt)
		if got != tt.expected {
			t.Errorf("attempt %d: expected %v, got %v", tt.attempt, tt.expected, got)
		}
	}
}

func TestRestarter_BackoffReset(t *testing.T) {
	st := testutil.TempStore(t)
	r := NewAutoRestarter(st, nil, testLogger())

	containerID := "container-backoff-reset"

	// Simulate some backoff state
	r.mu.Lock()
	r.backoffs[containerID] = &containerBackoff{
		attempts:    5,
		nextAllowed: time.Now().Add(5 * time.Minute),
	}
	r.mu.Unlock()

	attempts, _, ok := r.GetBackoffState(containerID)
	if !ok || attempts != 5 {
		t.Fatalf("expected 5 attempts, got %d (ok=%v)", attempts, ok)
	}

	r.ResetBackoff(containerID)

	_, _, ok = r.GetBackoffState(containerID)
	if ok {
		t.Error("expected backoff state to be cleared")
	}
}

func TestRestarter_SetManager(t *testing.T) {
	st := testutil.TempStore(t)
	r := NewAutoRestarter(st, nil, testLogger())

	if r.manager != nil {
		t.Error("expected nil manager initially")
	}

	mock := &mockAppRestarter{}
	r.SetManager(mock)

	if r.manager != mock {
		t.Error("expected manager to be set")
	}
}

// mockAppRestarter implements AppRestarter for testing.
type mockAppRestarter struct {
	restartFn func(ctx context.Context, containerID, appID string) error
	calls     int32
}

func (m *mockAppRestarter) RestartContainer(ctx context.Context, containerID, appID string) error {
	atomic.AddInt32(&m.calls, 1)
	if m.restartFn != nil {
		return m.restartFn(ctx, containerID, appID)
	}
	return nil
}

func TestRestarter_RequestRestart(t *testing.T) {
	st := testutil.TempStore(t)
	mgr := &mockAppRestarter{}
	r := NewAutoRestarter(st, mgr, testLogger())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r.Start(ctx)
	defer r.Stop()

	containerID := "container-restart-test"

	// Create a container in the store for restart count increment
	container := &store.Container{
		ID:    containerID,
		AppID: "app-1",
		State: store.ContainerStateRunning,
	}
	st.CreateContainer(container)

	r.RequestRestart(containerID, "app-1", "unhealthy")

	// Wait for restart to be processed
	time.Sleep(200 * time.Millisecond)

	if atomic.LoadInt32(&mgr.calls) != 1 {
		t.Errorf("expected 1 restart call, got %d", mgr.calls)
	}

	// Check restart count was incremented
	updated, err := st.GetContainer(containerID)
	if err != nil {
		t.Fatalf("failed to get container: %v", err)
	}
	if updated.RestartCount != 1 {
		t.Errorf("expected restart count 1, got %d", updated.RestartCount)
	}
}

func TestRestarter_BackoffDelaysRestart(t *testing.T) {
	st := testutil.TempStore(t)
	mgr := &mockAppRestarter{}
	r := NewAutoRestarter(st, mgr, testLogger())
	r.InitialDelay = 100 * time.Millisecond
	r.Multiplier = 2.0
	r.MaxDelay = 1 * time.Second

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r.Start(ctx)
	defer r.Stop()

	containerID := "container-backoff-test"
	st.CreateContainer(&store.Container{
		ID:    containerID,
		AppID: "app-1",
		State: store.ContainerStateRunning,
	})

	// First restart — should be immediate
	start := time.Now()
	r.RequestRestart(containerID, "app-1", "fail1")
	time.Sleep(50 * time.Millisecond)

	// Second restart — should be delayed by backoff
	r.RequestRestart(containerID, "app-1", "fail2")
	time.Sleep(500 * time.Millisecond)

	elapsed := time.Since(start)
	calls := atomic.LoadInt32(&mgr.calls)

	if calls < 2 {
		t.Errorf("expected at least 2 restart calls, got %d (elapsed: %v)", calls, elapsed)
	}

	// Check backoff state
	attempts, _, ok := r.GetBackoffState(containerID)
	if !ok {
		t.Error("expected backoff state to exist")
	}
	if attempts < 2 {
		t.Errorf("expected at least 2 attempts, got %d", attempts)
	}
}

func TestRestarter_QueueFull(t *testing.T) {
	st := testutil.TempStore(t)
	r := NewAutoRestarter(st, nil, testLogger())
	r.pending = make(chan RestartRequest, 1) // Tiny buffer

	containerID := "container-queue-full12"
	r.RequestRestart(containerID, "app-1", "test1") // Should succeed
	r.RequestRestart(containerID, "app-1", "test2") // Should be dropped (non-blocking)
}

func TestRestarter_StopDuringBackoff(t *testing.T) {
	st := testutil.TempStore(t)
	mgr := &mockAppRestarter{}
	r := NewAutoRestarter(st, mgr, testLogger())
	r.InitialDelay = 10 * time.Second // Long backoff

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r.Start(ctx)

	containerID := "container-stop-backoff"
	st.CreateContainer(&store.Container{
		ID:    containerID,
		AppID: "app-1",
		State: store.ContainerStateRunning,
	})

	// Force a backoff state
	r.mu.Lock()
	r.backoffs[containerID] = &containerBackoff{
		attempts:    3,
		nextAllowed: time.Now().Add(10 * time.Second),
	}
	r.mu.Unlock()

	r.RequestRestart(containerID, "app-1", "test")

	// Stop should return quickly even though restart is waiting in backoff
	done := make(chan struct{})
	go func() {
		r.Stop()
		close(done)
	}()

	select {
	case <-done:
		// Good — stopped promptly
	case <-time.After(2 * time.Second):
		t.Error("Stop took too long — may be stuck in backoff wait")
	}
}

// --- Store Health Result Tests ---

func TestStoreHealthResult_CreateAndGet(t *testing.T) {
	st := testutil.TempStore(t)

	result := &store.HealthResult{
		ContainerID: "container-store-test12",
		Status:      store.HealthStatusHealthy,
		Message:     "OK",
		Duration:    5 * time.Millisecond,
		CheckedAt:   time.Now(),
	}

	if err := st.CreateHealthResult(result); err != nil {
		t.Fatalf("CreateHealthResult failed: %v", err)
	}

	latest, err := st.GetLatestHealthResult("container-store-test12")
	if err != nil {
		t.Fatalf("GetLatestHealthResult failed: %v", err)
	}
	if latest.Status != store.HealthStatusHealthy {
		t.Errorf("expected healthy, got %s", latest.Status)
	}
}

func TestStoreHealthResult_GetMultiple(t *testing.T) {
	st := testutil.TempStore(t)

	containerID := "container-multi-result"
	for i := 0; i < 5; i++ {
		result := &store.HealthResult{
			ContainerID: containerID,
			Status:      store.HealthStatusHealthy,
			Message:     fmt.Sprintf("check-%d", i),
			Duration:    time.Millisecond,
			CheckedAt:   time.Now().Add(time.Duration(i) * time.Second),
		}
		if err := st.CreateHealthResult(result); err != nil {
			t.Fatalf("CreateHealthResult %d failed: %v", i, err)
		}
	}

	results, err := st.GetHealthResults(containerID, 3)
	if err != nil {
		t.Fatalf("GetHealthResults failed: %v", err)
	}
	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}

	// Should be in reverse chronological order (most recent first)
	if len(results) >= 2 && results[0].CheckedAt.Before(results[1].CheckedAt) {
		t.Error("results not in reverse chronological order")
	}
}

func TestStoreHealthResult_Prune(t *testing.T) {
	st := testutil.TempStore(t)

	containerID := "container-prune-result"
	for i := 0; i < 10; i++ {
		result := &store.HealthResult{
			ContainerID: containerID,
			Status:      store.HealthStatusHealthy,
			Message:     fmt.Sprintf("check-%d", i),
			Duration:    time.Millisecond,
			CheckedAt:   time.Now().Add(time.Duration(i) * time.Second),
		}
		st.CreateHealthResult(result)
	}

	// Prune to keep only 3
	if err := st.PruneHealthResults(containerID, 3); err != nil {
		t.Fatalf("PruneHealthResults failed: %v", err)
	}

	results, err := st.GetHealthResults(containerID, 0)
	if err != nil {
		t.Fatalf("GetHealthResults failed: %v", err)
	}
	if len(results) != 3 {
		t.Errorf("expected 3 results after pruning, got %d", len(results))
	}
}

func TestStoreHealthResult_NoResults(t *testing.T) {
	st := testutil.TempStore(t)

	_, err := st.GetLatestHealthResult("nonexistent-container")
	if err == nil {
		t.Error("expected error for nonexistent container")
	}

	results, err := st.GetHealthResults("nonexistent-container", 10)
	if err != nil {
		t.Fatalf("GetHealthResults should not fail: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestStoreHealthResult_PruneNoop(t *testing.T) {
	st := testutil.TempStore(t)

	// Prune nonexistent container — should not fail
	if err := st.PruneHealthResults("nonexistent", 5); err != nil {
		t.Fatalf("prune of nonexistent should not fail: %v", err)
	}
}
