package health

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/vessel/vessel/internal/runtime"
	"github.com/vessel/vessel/internal/store"
)

// MetricsCollector periodically collects resource metrics from container cgroups
// and stores them as time-series data for querying.
type MetricsCollector struct {
	mu         sync.RWMutex
	containers map[string]bool // containerIDs being monitored
	runtime    runtime.Runtime
	store      store.Store
	interval   time.Duration // Collection interval (default 15s)
	retention  time.Duration // How long to keep metrics (default 24h)
	logger     *slog.Logger

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewMetricsCollector creates a new MetricsCollector.
func NewMetricsCollector(rt runtime.Runtime, st store.Store, interval, retention time.Duration, logger *slog.Logger) *MetricsCollector {
	if interval <= 0 {
		interval = 15 * time.Second
	}
	if retention <= 0 {
		retention = 24 * time.Hour
	}

	return &MetricsCollector{
		containers: make(map[string]bool),
		runtime:    rt,
		store:      st,
		interval:   interval,
		retention:  retention,
		logger:     logger,
	}
}

// Start begins the metrics collection loop and pruning goroutine.
func (m *MetricsCollector) Start(ctx context.Context) error {
	m.ctx, m.cancel = context.WithCancel(ctx)

	m.wg.Add(1)
	go m.runLoop()

	m.wg.Add(1)
	go m.runPruner()

	m.logger.Info("metrics collector started", "interval", m.interval, "retention", m.retention)
	return nil
}

// Stop gracefully shuts down the collector and waits for goroutines to finish.
func (m *MetricsCollector) Stop() error {
	if m.cancel != nil {
		m.cancel()
	}
	m.wg.Wait()
	return nil
}

// RegisterContainer adds a container to metrics collection.
func (m *MetricsCollector) RegisterContainer(containerID string) {
	m.mu.Lock()
	m.containers[containerID] = true
	m.mu.Unlock()
	m.logger.Debug("registered container for metrics collection", "container", containerID[:12])
}

// DeregisterContainer removes a container from metrics collection.
func (m *MetricsCollector) DeregisterContainer(containerID string) {
	m.mu.Lock()
	delete(m.containers, containerID)
	m.mu.Unlock()
	m.logger.Debug("deregistered container from metrics collection", "container", containerID[:12])
}

// GetRegisteredContainers returns the IDs of all containers being monitored.
func (m *MetricsCollector) GetRegisteredContainers() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := make([]string, 0, len(m.containers))
	for id := range m.containers {
		ids = append(ids, id)
	}
	return ids
}

// runLoop is the main collection loop.
func (m *MetricsCollector) runLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.collectAll()
		}
	}
}

// collectAll collects metrics for all registered containers.
func (m *MetricsCollector) collectAll() {
	m.mu.RLock()
	containerIDs := make([]string, 0, len(m.containers))
	for id := range m.containers {
		containerIDs = append(containerIDs, id)
	}
	m.mu.RUnlock()

	for _, id := range containerIDs {
		m.collectContainer(id)
	}
}

// collectContainer collects and stores metrics for a single container.
func (m *MetricsCollector) collectContainer(containerID string) {
	stats, err := m.runtime.ContainerStats(m.ctx, containerID)
	if err != nil {
		m.logger.Debug("failed to collect metrics", "container", containerID[:12], "error", err)
		return
	}

	metric := &store.MetricPoint{
		Timestamp:      time.Now(),
		CPUPercent:     stats.CPUPercent,
		MemoryBytes:    stats.MemoryBytes,
		MemoryLimit:    stats.MemoryLimit,
		NetworkRxBytes: stats.NetworkRxBytes,
		NetworkTxBytes: stats.NetworkTxBytes,
		PidsCount:      stats.PidsCount,
	}

	if err := m.store.CreateMetricPoint(containerID, metric); err != nil {
		m.logger.Warn("failed to store metric", "container", containerID[:12], "error", err)
	}
}

// runPruner periodically removes old metrics beyond the retention period.
func (m *MetricsCollector) runPruner() {
	defer m.wg.Done()

	// Prune old metrics every hour
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			cutoff := time.Now().Add(-m.retention)
			if err := m.store.PruneMetrics(cutoff); err != nil {
				m.logger.Warn("failed to prune metrics", "error", err)
			}
		}
	}
}

// GetMetrics returns metrics for a container within a time range.
func (m *MetricsCollector) GetMetrics(containerID string, start, end time.Time, limit int) ([]*store.MetricPoint, error) {
	return m.store.GetMetrics(containerID, start, end, limit)
}

// GetLatestMetrics returns the most recent metric point for a container.
func (m *MetricsCollector) GetLatestMetrics(containerID string) (*store.MetricPoint, error) {
	metrics, err := m.store.GetMetrics(containerID, time.Time{}, time.Now(), -1)
	if err != nil {
		return nil, err
	}
	if len(metrics) == 0 {
		return nil, fmt.Errorf("no metrics for container %s", containerID)
	}
	return metrics[len(metrics)-1], nil
}
