package health

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/vessel/vessel/internal/runtime"
	"github.com/vessel/vessel/internal/store"
)

// ContainerHealth holds the current health state of a monitored container.
type ContainerHealth struct {
	ContainerID      string             `json:"container_id"`
	AppID            string             `json:"app_id"`
	Status           store.HealthStatus `json:"status"`
	LastCheck        time.Time          `json:"last_check"`
	LastSuccess      *time.Time         `json:"last_success,omitempty"`
	LastFailure      *time.Time         `json:"last_failure,omitempty"`
	ConsecutiveFails int                `json:"consecutive_fails"`
	Message          string             `json:"message,omitempty"`
	Check            store.HealthCheck  `json:"check"`
}

// HealthEvent is emitted when container health status changes.
type HealthEvent struct {
	ContainerID string             `json:"container_id"`
	AppID       string             `json:"app_id"`
	OldStatus   store.HealthStatus `json:"old_status"`
	NewStatus   store.HealthStatus `json:"new_status"`
	Message     string             `json:"message"`
	Timestamp   time.Time          `json:"timestamp"`
}

// HealthMonitorConfig configures the health monitor behavior.
type HealthMonitorConfig struct {
	CheckInterval      time.Duration // How often to run health checks (default 10s)
	UnhealthyThreshold int           // Consecutive failures before marking unhealthy (default 3)
	HealthyThreshold   int           // Consecutive successes before marking healthy (default 1)
}

// monitoredContainer tracks a container being health-checked.
type monitoredContainer struct {
	containerID      string
	appID            string
	check            store.HealthCheck
	status           store.HealthStatus
	lastCheck        time.Time
	lastSuccess      *time.Time
	lastFailure      *time.Time
	consecutiveFails int
	consecutiveOK    int
	message          string
}

// HealthMonitor continuously checks container health and triggers recovery actions.
type HealthMonitor struct {
	mu         sync.RWMutex
	containers map[string]*monitoredContainer
	runtime    runtime.Runtime
	store      store.Store
	restarter  *AutoRestarter
	alerter    *Alerter
	config     HealthMonitorConfig
	logger     *slog.Logger

	// Event subscribers
	subMu       sync.RWMutex
	subscribers []chan HealthEvent

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewHealthMonitor creates a new HealthMonitor.
func NewHealthMonitor(rt runtime.Runtime, st store.Store, restarter *AutoRestarter, alerter *Alerter, cfg HealthMonitorConfig, logger *slog.Logger) *HealthMonitor {
	if cfg.CheckInterval <= 0 {
		cfg.CheckInterval = 10 * time.Second
	}
	if cfg.UnhealthyThreshold <= 0 {
		cfg.UnhealthyThreshold = 3
	}
	if cfg.HealthyThreshold <= 0 {
		cfg.HealthyThreshold = 1
	}

	return &HealthMonitor{
		containers:  make(map[string]*monitoredContainer),
		runtime:     rt,
		store:       st,
		restarter:   restarter,
		alerter:     alerter,
		config:      cfg,
		logger:      logger,
		subscribers: make([]chan HealthEvent, 0),
	}
}

// Start begins the health monitoring loop.
func (m *HealthMonitor) Start(ctx context.Context) error {
	m.ctx, m.cancel = context.WithCancel(ctx)

	m.wg.Add(1)
	go m.runLoop()

	m.logger.Info("health monitor started", "interval", m.config.CheckInterval)
	return nil
}

// Stop gracefully shuts down the monitor.
func (m *HealthMonitor) Stop() error {
	if m.cancel != nil {
		m.cancel()
	}
	m.wg.Wait()

	// Close all subscriber channels
	m.subMu.Lock()
	for _, ch := range m.subscribers {
		close(ch)
	}
	m.subscribers = nil
	m.subMu.Unlock()

	return nil
}

// RegisterContainer adds a container to be monitored.
func (m *HealthMonitor) RegisterContainer(containerID, appID string, check store.HealthCheck) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.containers[containerID] = &monitoredContainer{
		containerID: containerID,
		appID:       appID,
		check:       check,
		status:      store.HealthStatusUnknown,
	}

	m.logger.Info("registered container for health monitoring",
		"container", containerID[:12], "app", appID, "type", check.Type)
	return nil
}

// DeregisterContainer removes a container from monitoring.
func (m *HealthMonitor) DeregisterContainer(containerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.containers[containerID]; ok {
		delete(m.containers, containerID)
		m.logger.Info("deregistered container from health monitoring", "container", containerID[:12])
	}
	return nil
}

// GetStatus returns the current health status of a container.
func (m *HealthMonitor) GetStatus(containerID string) (*ContainerHealth, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	mc, ok := m.containers[containerID]
	if !ok {
		return nil, nil
	}

	return m.toContainerHealth(mc), nil
}

// GetAllStatus returns health status for all monitored containers.
func (m *HealthMonitor) GetAllStatus() map[string]*ContainerHealth {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*ContainerHealth, len(m.containers))
	for id, mc := range m.containers {
		result[id] = m.toContainerHealth(mc)
	}
	return result
}

// Subscribe returns a channel that receives health events.
func (m *HealthMonitor) Subscribe() <-chan HealthEvent {
	ch := make(chan HealthEvent, 100)
	m.subMu.Lock()
	m.subscribers = append(m.subscribers, ch)
	m.subMu.Unlock()
	return ch
}

// Unsubscribe removes a subscription channel.
func (m *HealthMonitor) Unsubscribe(ch <-chan HealthEvent) {
	m.subMu.Lock()
	defer m.subMu.Unlock()

	for i, sub := range m.subscribers {
		if sub == ch {
			close(sub)
			m.subscribers = append(m.subscribers[:i], m.subscribers[i+1:]...)
			return
		}
	}
}

// runLoop is the main monitoring loop.
func (m *HealthMonitor) runLoop() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.checkAll()
		}
	}
}

// checkAll runs health checks for all registered containers.
func (m *HealthMonitor) checkAll() {
	m.mu.RLock()
	containers := make([]*monitoredContainer, 0, len(m.containers))
	for _, c := range m.containers {
		containers = append(containers, c)
	}
	m.mu.RUnlock()

	for _, c := range containers {
		m.checkContainer(c)
	}
}

// checkContainer performs a health check on a single container.
func (m *HealthMonitor) checkContainer(mc *monitoredContainer) {
	err := ExecuteCheck(m.ctx, m.runtime, mc.containerID, mc.check)
	now := time.Now()

	m.mu.Lock()

	// Verify container is still registered
	current, ok := m.containers[mc.containerID]
	if !ok {
		m.mu.Unlock()
		return
	}

	oldStatus := current.status
	current.lastCheck = now

	if err == nil {
		current.consecutiveOK++
		current.consecutiveFails = 0
		current.lastSuccess = &now
		current.message = "healthy"

		if current.consecutiveOK >= m.config.HealthyThreshold {
			current.status = store.HealthStatusHealthy
		}
	} else {
		current.consecutiveFails++
		current.consecutiveOK = 0
		current.lastFailure = &now
		current.message = err.Error()

		if current.consecutiveFails >= m.config.UnhealthyThreshold {
			current.status = store.HealthStatusUnhealthy
		}
	}

	newStatus := current.status
	appID := current.appID
	message := current.message
	consecutiveFails := current.consecutiveFails
	containerID := current.containerID
	m.mu.Unlock()

	// Store health result (best-effort, outside lock)
	result := &store.HealthResult{
		ContainerID: containerID,
		Status:      newStatus,
		Message:     message,
		Duration:    time.Since(now),
		CheckedAt:   now,
	}
	if m.store != nil {
		m.store.CreateHealthResult(result)
	}

	// Emit event and trigger restart if status changed (outside lock)
	if oldStatus != newStatus {
		event := HealthEvent{
			ContainerID: containerID,
			AppID:       appID,
			OldStatus:   oldStatus,
			NewStatus:   newStatus,
			Message:     message,
			Timestamp:   now,
		}
		m.emitEvent(event)

		if newStatus == store.HealthStatusUnhealthy && m.restarter != nil {
			m.logger.Warn("container became unhealthy, requesting restart",
				"container", containerID[:12], "app", appID, "failures", consecutiveFails)
			m.restarter.RequestRestart(containerID, appID, message)
		}

		if newStatus == store.HealthStatusHealthy && oldStatus == store.HealthStatusUnhealthy && m.restarter != nil {
			m.restarter.ResetBackoff(containerID)
		}
	}
}

// emitEvent sends a health event to all subscribers and the alerter.
func (m *HealthMonitor) emitEvent(event HealthEvent) {
	m.subMu.RLock()
	defer m.subMu.RUnlock()

	for _, ch := range m.subscribers {
		select {
		case ch <- event:
		default:
			m.logger.Warn("dropping health event, subscriber channel full")
		}
	}

	// Send alert via webhook
	if m.alerter != nil {
		go m.alerter.HandleHealthEvent(m.ctx, event)
	}
}

// toContainerHealth converts internal state to the public ContainerHealth type.
func (m *HealthMonitor) toContainerHealth(mc *monitoredContainer) *ContainerHealth {
	return &ContainerHealth{
		ContainerID:      mc.containerID,
		AppID:            mc.appID,
		Status:           mc.status,
		LastCheck:        mc.lastCheck,
		LastSuccess:      mc.lastSuccess,
		LastFailure:      mc.lastFailure,
		ConsecutiveFails: mc.consecutiveFails,
		Message:          mc.message,
		Check:            mc.check,
	}
}
