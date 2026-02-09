package health

import (
	"context"
	"fmt"
	"io"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vessel/vessel/internal/runtime"
	"github.com/vessel/vessel/internal/store"
	"github.com/vessel/vessel/testutil"
)

// metricsTestRuntime returns configurable container stats.
type metricsTestRuntime struct {
	statsFn          func(ctx context.Context, containerID string) (*store.MetricPoint, error)
	containerCounter int64
}

func (m *metricsTestRuntime) PullImage(ctx context.Context, ref string) error { return nil }
func (m *metricsTestRuntime) CreateContainer(ctx context.Context, opts runtime.ContainerOpts) (*store.Container, error) {
	n := atomic.AddInt64(&m.containerCounter, 1)
	return &store.Container{
		ID:        fmt.Sprintf("metrics-container-%06d", n),
		Image:     opts.Image,
		State:     store.ContainerStateCreated,
		CreatedAt: time.Now(),
	}, nil
}
func (m *metricsTestRuntime) StartContainer(ctx context.Context, containerID string) error { return nil }
func (m *metricsTestRuntime) StopContainer(ctx context.Context, containerID string, gp time.Duration) error {
	return nil
}
func (m *metricsTestRuntime) RemoveContainer(ctx context.Context, containerID string) error {
	return nil
}
func (m *metricsTestRuntime) ContainerLogs(ctx context.Context, containerID string, follow bool, tail int) (io.ReadCloser, error) {
	return io.NopCloser(nil), nil
}
func (m *metricsTestRuntime) ContainerStats(ctx context.Context, containerID string) (*store.MetricPoint, error) {
	if m.statsFn != nil {
		return m.statsFn(ctx, containerID)
	}
	return &store.MetricPoint{
		Timestamp:   time.Now(),
		CPUPercent:  5.5,
		MemoryBytes: 50 * 1024 * 1024,
		MemoryLimit: 256 * 1024 * 1024,
		PidsCount:   10,
	}, nil
}
func (m *metricsTestRuntime) GetContainerPID(containerID string) (int, error) { return 12345, nil }
func (m *metricsTestRuntime) ExecInContainer(ctx context.Context, containerID string, cmd []string) error {
	return nil
}

// --- MetricsCollector Tests ---

func TestMetricsCollector_NewDefaults(t *testing.T) {
	rt := &metricsTestRuntime{}
	st := testutil.TempStore(t)
	mc := NewMetricsCollector(rt, st, 0, 0, testLogger())

	if mc.interval != 15*time.Second {
		t.Errorf("expected default interval 15s, got %v", mc.interval)
	}
	if mc.retention != 24*time.Hour {
		t.Errorf("expected default retention 24h, got %v", mc.retention)
	}
}

func TestMetricsCollector_CustomIntervals(t *testing.T) {
	rt := &metricsTestRuntime{}
	st := testutil.TempStore(t)
	mc := NewMetricsCollector(rt, st, 5*time.Second, 12*time.Hour, testLogger())

	if mc.interval != 5*time.Second {
		t.Errorf("expected 5s interval, got %v", mc.interval)
	}
	if mc.retention != 12*time.Hour {
		t.Errorf("expected 12h retention, got %v", mc.retention)
	}
}

func TestMetricsCollector_RegisterDeregister(t *testing.T) {
	rt := &metricsTestRuntime{}
	st := testutil.TempStore(t)
	mc := NewMetricsCollector(rt, st, 15*time.Second, 24*time.Hour, testLogger())

	containerID := "test-metrics-container1"
	mc.RegisterContainer(containerID)

	registered := mc.GetRegisteredContainers()
	if len(registered) != 1 {
		t.Fatalf("expected 1 registered, got %d", len(registered))
	}
	if registered[0] != containerID {
		t.Errorf("expected %s, got %s", containerID, registered[0])
	}

	mc.DeregisterContainer(containerID)
	registered = mc.GetRegisteredContainers()
	if len(registered) != 0 {
		t.Errorf("expected 0 registered after deregister, got %d", len(registered))
	}
}

func TestMetricsCollector_MultipleContainers(t *testing.T) {
	rt := &metricsTestRuntime{}
	st := testutil.TempStore(t)
	mc := NewMetricsCollector(rt, st, 15*time.Second, 24*time.Hour, testLogger())

	mc.RegisterContainer("container-aabbcc-111111")
	mc.RegisterContainer("container-ddeeff-222222")
	mc.RegisterContainer("container-gghhii-333333")

	registered := mc.GetRegisteredContainers()
	if len(registered) != 3 {
		t.Errorf("expected 3 registered, got %d", len(registered))
	}

	mc.DeregisterContainer("container-ddeeff-222222")
	registered = mc.GetRegisteredContainers()
	if len(registered) != 2 {
		t.Errorf("expected 2 registered after deregister, got %d", len(registered))
	}
}

func TestMetricsCollector_CollectStoresMetrics(t *testing.T) {
	var collectCount int64
	rt := &metricsTestRuntime{
		statsFn: func(ctx context.Context, containerID string) (*store.MetricPoint, error) {
			atomic.AddInt64(&collectCount, 1)
			return &store.MetricPoint{
				Timestamp:   time.Now(),
				CPUPercent:  12.5,
				MemoryBytes: 100 * 1024 * 1024,
				MemoryLimit: 512 * 1024 * 1024,
				PidsCount:   25,
			}, nil
		},
	}
	st := testutil.TempStore(t)
	mc := NewMetricsCollector(rt, st, 50*time.Millisecond, 24*time.Hour, testLogger())

	containerID := "container-collect-test1"
	mc.RegisterContainer(containerID)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mc.Start(ctx)
	defer mc.Stop()

	// Wait for at least 2 collection intervals
	time.Sleep(150 * time.Millisecond)

	count := atomic.LoadInt64(&collectCount)
	if count < 2 {
		t.Errorf("expected at least 2 collections, got %d", count)
	}

	// Verify metrics were stored
	metrics, err := st.GetMetrics(containerID, time.Time{}, time.Now(), 0)
	if err != nil {
		t.Fatalf("GetMetrics failed: %v", err)
	}
	if len(metrics) < 2 {
		t.Errorf("expected at least 2 stored metrics, got %d", len(metrics))
	}

	// Check metric values
	if metrics[0].CPUPercent != 12.5 {
		t.Errorf("expected CPU 12.5%%, got %.1f%%", metrics[0].CPUPercent)
	}
	if metrics[0].MemoryBytes != 100*1024*1024 {
		t.Errorf("expected memory 100MiB, got %d", metrics[0].MemoryBytes)
	}
}

func TestMetricsCollector_StatsError(t *testing.T) {
	rt := &metricsTestRuntime{
		statsFn: func(ctx context.Context, containerID string) (*store.MetricPoint, error) {
			return nil, fmt.Errorf("container not running")
		},
	}
	st := testutil.TempStore(t)
	mc := NewMetricsCollector(rt, st, 50*time.Millisecond, 24*time.Hour, testLogger())

	containerID := "container-error-test12"
	mc.RegisterContainer(containerID)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mc.Start(ctx)
	defer mc.Stop()

	time.Sleep(150 * time.Millisecond)

	// No metrics should be stored when stats fail
	metrics, err := st.GetMetrics(containerID, time.Time{}, time.Now(), 0)
	if err != nil {
		t.Fatalf("GetMetrics failed: %v", err)
	}
	if len(metrics) != 0 {
		t.Errorf("expected 0 metrics on error, got %d", len(metrics))
	}
}

func TestMetricsCollector_StartStop(t *testing.T) {
	rt := &metricsTestRuntime{}
	st := testutil.TempStore(t)
	mc := NewMetricsCollector(rt, st, 50*time.Millisecond, 24*time.Hour, testLogger())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := mc.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Stop should return promptly
	done := make(chan struct{})
	go func() {
		mc.Stop()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Error("Stop took too long")
	}
}

func TestMetricsCollector_GetLatestMetrics(t *testing.T) {
	rt := &metricsTestRuntime{}
	st := testutil.TempStore(t)
	mc := NewMetricsCollector(rt, st, 15*time.Second, 24*time.Hour, testLogger())

	containerID := "container-latest-test1"

	// No metrics yet
	_, err := mc.GetLatestMetrics(containerID)
	if err == nil {
		t.Error("expected error for no metrics")
	}

	// Store metrics in the past so they're all before time.Now()
	now := time.Now()
	for i := 0; i < 3; i++ {
		metric := &store.MetricPoint{
			Timestamp:   now.Add(-time.Duration(3-i) * time.Second), // -3s, -2s, -1s
			CPUPercent:  float64(i + 1),
			MemoryBytes: int64((i + 1) * 1024 * 1024),
		}
		if err := st.CreateMetricPoint(containerID, metric); err != nil {
			t.Fatalf("CreateMetricPoint failed: %v", err)
		}
	}

	latest, err := mc.GetLatestMetrics(containerID)
	if err != nil {
		t.Fatalf("GetLatestMetrics failed: %v", err)
	}
	if latest.CPUPercent != 3.0 {
		t.Errorf("expected latest CPU 3.0%%, got %.1f%%", latest.CPUPercent)
	}
}

func TestMetricsCollector_GetMetricsTimeRange(t *testing.T) {
	rt := &metricsTestRuntime{}
	st := testutil.TempStore(t)
	mc := NewMetricsCollector(rt, st, 15*time.Second, 24*time.Hour, testLogger())

	containerID := "container-range-test12"
	now := time.Now()

	// Store metrics spread across time
	for i := 0; i < 10; i++ {
		metric := &store.MetricPoint{
			Timestamp:   now.Add(time.Duration(i) * time.Minute),
			CPUPercent:  float64(i),
			MemoryBytes: int64(i * 1024 * 1024),
		}
		st.CreateMetricPoint(containerID, metric)
	}

	// Query a time range (minutes 3-7)
	start := now.Add(3 * time.Minute)
	end := now.Add(7 * time.Minute)
	metrics, err := mc.GetMetrics(containerID, start, end, 0)
	if err != nil {
		t.Fatalf("GetMetrics failed: %v", err)
	}

	if len(metrics) != 5 { // minutes 3, 4, 5, 6, 7
		t.Errorf("expected 5 metrics in range, got %d", len(metrics))
	}
}

func TestMetricsCollector_GetMetricsWithLimit(t *testing.T) {
	rt := &metricsTestRuntime{}
	st := testutil.TempStore(t)
	mc := NewMetricsCollector(rt, st, 15*time.Second, 24*time.Hour, testLogger())

	containerID := "container-limit-test12"
	now := time.Now()

	for i := 0; i < 10; i++ {
		st.CreateMetricPoint(containerID, &store.MetricPoint{
			Timestamp:  now.Add(time.Duration(i) * time.Second),
			CPUPercent: float64(i),
		})
	}

	metrics, err := mc.GetMetrics(containerID, time.Time{}, now.Add(1*time.Hour), 3)
	if err != nil {
		t.Fatalf("GetMetrics failed: %v", err)
	}
	if len(metrics) != 3 {
		t.Errorf("expected 3 metrics with limit, got %d", len(metrics))
	}
}

func TestMetricsCollector_DeregisterStopsCollection(t *testing.T) {
	var collectCount int64
	rt := &metricsTestRuntime{
		statsFn: func(ctx context.Context, containerID string) (*store.MetricPoint, error) {
			atomic.AddInt64(&collectCount, 1)
			return &store.MetricPoint{
				Timestamp: time.Now(),
			}, nil
		},
	}
	st := testutil.TempStore(t)
	mc := NewMetricsCollector(rt, st, 50*time.Millisecond, 24*time.Hour, testLogger())

	containerID := "container-dereg-stop12"
	mc.RegisterContainer(containerID)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mc.Start(ctx)
	defer mc.Stop()

	// Let it collect a few times
	time.Sleep(150 * time.Millisecond)
	countBefore := atomic.LoadInt64(&collectCount)

	// Deregister
	mc.DeregisterContainer(containerID)

	// Wait and verify collection stopped
	time.Sleep(150 * time.Millisecond)
	countAfter := atomic.LoadInt64(&collectCount)

	// Allow +1 tolerance for an in-flight collection that started before deregister took effect
	if countAfter > countBefore+1 {
		t.Errorf("collection continued after deregister: before=%d, after=%d", countBefore, countAfter)
	}
}

// --- Store Metrics Tests ---

func TestStoreMetrics_CreateAndGet(t *testing.T) {
	st := testutil.TempStore(t)

	containerID := "container-store-met-12"
	now := time.Now()

	metric := &store.MetricPoint{
		Timestamp:   now,
		CPUPercent:  42.5,
		MemoryBytes: 128 * 1024 * 1024,
		MemoryLimit: 512 * 1024 * 1024,
		PidsCount:   50,
	}

	if err := st.CreateMetricPoint(containerID, metric); err != nil {
		t.Fatalf("CreateMetricPoint failed: %v", err)
	}

	metrics, err := st.GetMetrics(containerID, time.Time{}, now.Add(time.Second), 0)
	if err != nil {
		t.Fatalf("GetMetrics failed: %v", err)
	}
	if len(metrics) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(metrics))
	}
	if metrics[0].CPUPercent != 42.5 {
		t.Errorf("expected CPU 42.5%%, got %.1f%%", metrics[0].CPUPercent)
	}
	if metrics[0].MemoryBytes != 128*1024*1024 {
		t.Errorf("expected 128MiB memory, got %d", metrics[0].MemoryBytes)
	}
}

func TestStoreMetrics_MultipleContainers(t *testing.T) {
	st := testutil.TempStore(t)
	now := time.Now()

	// Store metrics for two containers
	st.CreateMetricPoint("container-aaa-111111", &store.MetricPoint{
		Timestamp: now, CPUPercent: 10.0,
	})
	st.CreateMetricPoint("container-bbb-222222", &store.MetricPoint{
		Timestamp: now, CPUPercent: 20.0,
	})
	st.CreateMetricPoint("container-aaa-111111", &store.MetricPoint{
		Timestamp: now.Add(time.Second), CPUPercent: 11.0,
	})

	// Query only container-aaa
	metrics, err := st.GetMetrics("container-aaa-111111", time.Time{}, now.Add(time.Hour), 0)
	if err != nil {
		t.Fatalf("GetMetrics failed: %v", err)
	}
	if len(metrics) != 2 {
		t.Errorf("expected 2 metrics for container-aaa, got %d", len(metrics))
	}

	// Query only container-bbb
	metrics, err = st.GetMetrics("container-bbb-222222", time.Time{}, now.Add(time.Hour), 0)
	if err != nil {
		t.Fatalf("GetMetrics failed: %v", err)
	}
	if len(metrics) != 1 {
		t.Errorf("expected 1 metric for container-bbb, got %d", len(metrics))
	}
}

func TestStoreMetrics_Prune(t *testing.T) {
	st := testutil.TempStore(t)
	now := time.Now()

	containerID := "container-prune-met-12"

	// Store 10 metrics, 1 second apart
	for i := 0; i < 10; i++ {
		st.CreateMetricPoint(containerID, &store.MetricPoint{
			Timestamp:  now.Add(time.Duration(i) * time.Second),
			CPUPercent: float64(i),
		})
	}

	// Prune everything before the 5th second
	cutoff := now.Add(5 * time.Second)
	if err := st.PruneMetrics(cutoff); err != nil {
		t.Fatalf("PruneMetrics failed: %v", err)
	}

	metrics, err := st.GetMetrics(containerID, time.Time{}, now.Add(time.Hour), 0)
	if err != nil {
		t.Fatalf("GetMetrics failed: %v", err)
	}
	if len(metrics) != 5 { // seconds 5, 6, 7, 8, 9
		t.Errorf("expected 5 metrics after pruning, got %d", len(metrics))
	}
}

func TestStoreMetrics_PruneMultipleContainers(t *testing.T) {
	st := testutil.TempStore(t)
	now := time.Now()

	// Store old metrics for container-a and container-b
	st.CreateMetricPoint("container-prun-aaa-12", &store.MetricPoint{
		Timestamp: now.Add(-2 * time.Hour), CPUPercent: 1.0,
	})
	st.CreateMetricPoint("container-prun-bbb-12", &store.MetricPoint{
		Timestamp: now.Add(-2 * time.Hour), CPUPercent: 2.0,
	})
	// Store recent metrics
	st.CreateMetricPoint("container-prun-aaa-12", &store.MetricPoint{
		Timestamp: now, CPUPercent: 3.0,
	})

	// Prune older than 1 hour ago
	if err := st.PruneMetrics(now.Add(-1 * time.Hour)); err != nil {
		t.Fatalf("PruneMetrics failed: %v", err)
	}

	// container-a should have 1 metric (the recent one)
	metrics, _ := st.GetMetrics("container-prun-aaa-12", time.Time{}, now.Add(time.Hour), 0)
	if len(metrics) != 1 {
		t.Errorf("expected 1 metric for container-a, got %d", len(metrics))
	}

	// container-b should have 0 metrics (old one pruned)
	metrics, _ = st.GetMetrics("container-prun-bbb-12", time.Time{}, now.Add(time.Hour), 0)
	if len(metrics) != 0 {
		t.Errorf("expected 0 metrics for container-b, got %d", len(metrics))
	}
}

func TestStoreMetrics_EmptyResults(t *testing.T) {
	st := testutil.TempStore(t)

	metrics, err := st.GetMetrics("nonexistent-container", time.Time{}, time.Now(), 0)
	if err != nil {
		t.Fatalf("GetMetrics should not fail: %v", err)
	}
	if len(metrics) != 0 {
		t.Errorf("expected 0 metrics, got %d", len(metrics))
	}
}

func TestStoreMetrics_PruneNoop(t *testing.T) {
	st := testutil.TempStore(t)

	// Prune on empty store should not fail
	if err := st.PruneMetrics(time.Now()); err != nil {
		t.Fatalf("PruneMetrics on empty store should not fail: %v", err)
	}
}

func TestStoreMetrics_Chronological(t *testing.T) {
	st := testutil.TempStore(t)
	now := time.Now()
	containerID := "container-chrono-test1"

	// Store metrics out of order
	st.CreateMetricPoint(containerID, &store.MetricPoint{
		Timestamp: now.Add(2 * time.Second), CPUPercent: 3.0,
	})
	st.CreateMetricPoint(containerID, &store.MetricPoint{
		Timestamp: now, CPUPercent: 1.0,
	})
	st.CreateMetricPoint(containerID, &store.MetricPoint{
		Timestamp: now.Add(1 * time.Second), CPUPercent: 2.0,
	})

	metrics, err := st.GetMetrics(containerID, time.Time{}, now.Add(time.Hour), 0)
	if err != nil {
		t.Fatalf("GetMetrics failed: %v", err)
	}
	if len(metrics) != 3 {
		t.Fatalf("expected 3 metrics, got %d", len(metrics))
	}

	// Should be in chronological order (BoltDB keys are sorted)
	for i := 1; i < len(metrics); i++ {
		if metrics[i].CPUPercent < metrics[i-1].CPUPercent {
			t.Errorf("metrics not in chronological order at index %d", i)
		}
	}
}
