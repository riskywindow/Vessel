package health

import (
	"context"
	"encoding/json"
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
	mon := NewHealthMonitor(rt, st, nil, nil, HealthMonitorConfig{}, testLogger())

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
	mon := NewHealthMonitor(rt, st, nil, nil, HealthMonitorConfig{}, testLogger())

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
	mon := NewHealthMonitor(rt, st, nil, nil, cfg, testLogger())

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
	mon := NewHealthMonitor(rt, st, nil, nil, cfg, testLogger())

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
	mon := NewHealthMonitor(rt, st, nil, nil, cfg, testLogger())

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
	mon := NewHealthMonitor(rt, st, nil, nil, HealthMonitorConfig{}, testLogger())

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
	mon := NewHealthMonitor(rt, st, nil, nil, cfg, testLogger())

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
	mon := NewHealthMonitor(rt, st, nil, nil, HealthMonitorConfig{CheckInterval: 50 * time.Millisecond}, testLogger())

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
	mon := NewHealthMonitor(rt, st, nil, nil, HealthMonitorConfig{}, testLogger())

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
	mon := NewHealthMonitor(rt, st, restarter, nil, cfg, testLogger())

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

// --- Alerter Tests ---

func TestAlerter_NewAlerter_Defaults(t *testing.T) {
	a := NewAlerter(AlertConfig{}, testLogger())
	if a.config.MinInterval != 5*time.Minute {
		t.Errorf("expected default 5m min interval, got %v", a.config.MinInterval)
	}
}

func TestAlerter_SendAlert_Disabled(t *testing.T) {
	a := NewAlerter(AlertConfig{Enabled: false, WebhookURL: "http://example.com"}, testLogger())

	err := a.SendAlert(context.Background(), Alert{
		Type:        "unhealthy",
		ContainerID: "container-disabled-12",
		Timestamp:   time.Now(),
	})
	if err != nil {
		t.Fatalf("expected no error for disabled alerter, got: %v", err)
	}
}

func TestAlerter_SendAlert_EmptyURL(t *testing.T) {
	a := NewAlerter(AlertConfig{Enabled: true, WebhookURL: ""}, testLogger())

	err := a.SendAlert(context.Background(), Alert{
		Type:        "unhealthy",
		ContainerID: "container-nourl-12345",
		Timestamp:   time.Now(),
	})
	if err != nil {
		t.Fatalf("expected no error for empty URL, got: %v", err)
	}
}

func TestAlerter_SendAlert_Success(t *testing.T) {
	var receivedPayload []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		receivedPayload = body
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}
		if r.Header.Get("User-Agent") != "Vessel/1.0" {
			t.Errorf("expected User-Agent Vessel/1.0, got %s", r.Header.Get("User-Agent"))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	a := NewAlerter(AlertConfig{
		Enabled:    true,
		WebhookURL: srv.URL,
	}, testLogger())

	alert := Alert{
		Type:        "unhealthy",
		AppID:       "my-app",
		ContainerID: "container-alert-test1",
		Message:     "health check failed",
		Timestamp:   time.Now(),
	}

	err := a.SendAlert(context.Background(), alert)
	if err != nil {
		t.Fatalf("SendAlert failed: %v", err)
	}

	// Verify payload
	var received Alert
	if err := json.Unmarshal(receivedPayload, &received); err != nil {
		t.Fatalf("failed to unmarshal received payload: %v", err)
	}
	if received.Type != "unhealthy" {
		t.Errorf("expected type 'unhealthy', got '%s'", received.Type)
	}
	if received.AppID != "my-app" {
		t.Errorf("expected app_id 'my-app', got '%s'", received.AppID)
	}
	if received.ContainerID != "container-alert-test1" {
		t.Errorf("expected container_id, got '%s'", received.ContainerID)
	}
	if received.Message != "health check failed" {
		t.Errorf("expected message 'health check failed', got '%s'", received.Message)
	}
}

func TestAlerter_SendAlert_WebhookError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	a := NewAlerter(AlertConfig{
		Enabled:    true,
		WebhookURL: srv.URL,
	}, testLogger())

	err := a.SendAlert(context.Background(), Alert{
		Type:        "unhealthy",
		ContainerID: "container-webhook-err",
		Timestamp:   time.Now(),
	})
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestAlerter_RateLimiting(t *testing.T) {
	var callCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	a := NewAlerter(AlertConfig{
		Enabled:     true,
		WebhookURL:  srv.URL,
		MinInterval: 1 * time.Hour, // Very long interval for testing
	}, testLogger())

	containerID := "container-ratelimit-1"

	// First alert should go through
	a.SendAlert(context.Background(), Alert{
		Type: "unhealthy", ContainerID: containerID, Timestamp: time.Now(),
	})

	// Second alert for same container should be rate-limited
	a.SendAlert(context.Background(), Alert{
		Type: "unhealthy", ContainerID: containerID, Timestamp: time.Now(),
	})

	if atomic.LoadInt32(&callCount) != 1 {
		t.Errorf("expected 1 webhook call (second should be rate-limited), got %d", callCount)
	}

	// Different container should still go through
	a.SendAlert(context.Background(), Alert{
		Type: "unhealthy", ContainerID: "container-ratelimit-2", Timestamp: time.Now(),
	})

	if atomic.LoadInt32(&callCount) != 2 {
		t.Errorf("expected 2 webhook calls (different container), got %d", callCount)
	}
}

func TestAlerter_RecoveryAlerts_Disabled(t *testing.T) {
	var callCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	a := NewAlerter(AlertConfig{
		Enabled:         true,
		WebhookURL:      srv.URL,
		IncludeRecovers: false,
	}, testLogger())

	// Recovery alert should be skipped
	a.SendAlert(context.Background(), Alert{
		Type: "recovered", ContainerID: "container-recover-off", Timestamp: time.Now(),
	})

	if atomic.LoadInt32(&callCount) != 0 {
		t.Errorf("expected 0 webhook calls (recovery disabled), got %d", callCount)
	}
}

func TestAlerter_RecoveryAlerts_Enabled(t *testing.T) {
	var callCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	a := NewAlerter(AlertConfig{
		Enabled:         true,
		WebhookURL:      srv.URL,
		IncludeRecovers: true,
	}, testLogger())

	a.SendAlert(context.Background(), Alert{
		Type: "recovered", ContainerID: "container-recover-on1", Timestamp: time.Now(),
	})

	if atomic.LoadInt32(&callCount) != 1 {
		t.Errorf("expected 1 webhook call (recovery enabled), got %d", callCount)
	}
}

func TestAlerter_HandleHealthEvent_Unhealthy(t *testing.T) {
	var receivedType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var alert Alert
		json.Unmarshal(body, &alert)
		receivedType = alert.Type
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	a := NewAlerter(AlertConfig{
		Enabled:    true,
		WebhookURL: srv.URL,
	}, testLogger())

	event := HealthEvent{
		ContainerID: "container-event-unhlt",
		AppID:       "test-app",
		OldStatus:   store.HealthStatusHealthy,
		NewStatus:   store.HealthStatusUnhealthy,
		Message:     "check failed",
		Timestamp:   time.Now(),
	}

	a.HandleHealthEvent(context.Background(), event)

	// Give async webhook time
	time.Sleep(50 * time.Millisecond)

	if receivedType != "unhealthy" {
		t.Errorf("expected type 'unhealthy', got '%s'", receivedType)
	}
}

func TestAlerter_HandleHealthEvent_Recovered(t *testing.T) {
	var receivedType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var alert Alert
		json.Unmarshal(body, &alert)
		receivedType = alert.Type
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	a := NewAlerter(AlertConfig{
		Enabled:         true,
		WebhookURL:      srv.URL,
		IncludeRecovers: true,
	}, testLogger())

	event := HealthEvent{
		ContainerID: "container-event-recov",
		AppID:       "test-app",
		OldStatus:   store.HealthStatusUnhealthy,
		NewStatus:   store.HealthStatusHealthy,
		Message:     "recovered",
		Timestamp:   time.Now(),
	}

	a.HandleHealthEvent(context.Background(), event)

	time.Sleep(50 * time.Millisecond)

	if receivedType != "recovered" {
		t.Errorf("expected type 'recovered', got '%s'", receivedType)
	}
}

func TestAlerter_HandleHealthEvent_InitialHealthy_Ignored(t *testing.T) {
	var callCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	a := NewAlerter(AlertConfig{
		Enabled:         true,
		WebhookURL:      srv.URL,
		IncludeRecovers: true,
	}, testLogger())

	// Unknown -> Healthy transition should NOT trigger alert
	event := HealthEvent{
		ContainerID: "container-init-health",
		AppID:       "test-app",
		OldStatus:   store.HealthStatusUnknown,
		NewStatus:   store.HealthStatusHealthy,
		Timestamp:   time.Now(),
	}

	a.HandleHealthEvent(context.Background(), event)
	time.Sleep(50 * time.Millisecond)

	if atomic.LoadInt32(&callCount) != 0 {
		t.Errorf("expected 0 calls for initial healthy, got %d", callCount)
	}
}

func TestAlerter_HandleHealthEvent_UnknownStatus_Ignored(t *testing.T) {
	var callCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	a := NewAlerter(AlertConfig{
		Enabled:    true,
		WebhookURL: srv.URL,
	}, testLogger())

	event := HealthEvent{
		ContainerID: "container-unknown-sts",
		AppID:       "test-app",
		OldStatus:   store.HealthStatusHealthy,
		NewStatus:   store.HealthStatusUnknown,
		Timestamp:   time.Now(),
	}

	a.HandleHealthEvent(context.Background(), event)
	time.Sleep(50 * time.Millisecond)

	if atomic.LoadInt32(&callCount) != 0 {
		t.Errorf("expected 0 calls for unknown status, got %d", callCount)
	}
}

func TestAlerter_MonitorIntegration(t *testing.T) {
	// Test that alerts are sent when the health monitor detects status changes
	var alertReceived atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		alertReceived.Store(true)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	rt := &mockRuntime{
		execFn: func(ctx context.Context, containerID string, cmd []string) error {
			return fmt.Errorf("process dead")
		},
	}
	st := testutil.TempStore(t)

	alerter := NewAlerter(AlertConfig{
		Enabled:    true,
		WebhookURL: srv.URL,
	}, testLogger())

	cfg := HealthMonitorConfig{
		CheckInterval:      50 * time.Millisecond,
		UnhealthyThreshold: 2,
		HealthyThreshold:   1,
	}
	mon := NewHealthMonitor(rt, st, nil, alerter, cfg, testLogger())

	containerID := "container-alert-integ"
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

	// Wait for unhealthy threshold to trigger alert
	time.Sleep(300 * time.Millisecond)

	if !alertReceived.Load() {
		t.Error("expected alert to be sent when container becomes unhealthy")
	}
}

func TestHumanDuration(t *testing.T) {
	tests := []struct {
		input    time.Duration
		expected string
	}{
		{5 * time.Second, "5s"},
		{30 * time.Second, "30s"},
		{2 * time.Minute, "2m"},
		{45 * time.Minute, "45m"},
		{3 * time.Hour, "3h"},
		{24 * time.Hour, "24h"},
	}

	for _, tt := range tests {
		got := humanDuration(tt.input)
		if got != tt.expected {
			t.Errorf("humanDuration(%v) = %s, want %s", tt.input, got, tt.expected)
		}
	}
}

func humanDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	return fmt.Sprintf("%dh", int(d.Hours()))
}
