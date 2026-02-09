package health

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/vessel/vessel/internal/store"
)

// AppRestarter is the interface for triggering container restarts via the manager.
// Implemented by manager.AppManager to avoid circular imports.
type AppRestarter interface {
	RestartContainer(ctx context.Context, containerID string, appID string) error
}

// RestartRequest represents a pending container restart.
type RestartRequest struct {
	ContainerID string
	AppID       string
	Reason      string
	RequestedAt time.Time
}

// containerBackoff tracks exponential backoff state for a container.
type containerBackoff struct {
	attempts    int
	lastAttempt time.Time
	nextAllowed time.Time
}

// AutoRestarter handles automatic container restarts with exponential backoff.
type AutoRestarter struct {
	mu       sync.Mutex
	backoffs map[string]*containerBackoff
	pending  chan RestartRequest
	store    store.Store
	manager  AppRestarter
	logger   *slog.Logger

	// Backoff configuration
	InitialDelay time.Duration
	MaxDelay     time.Duration
	Multiplier   float64

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewAutoRestarter creates a new AutoRestarter.
// The manager can be nil initially and set later via SetManager.
func NewAutoRestarter(st store.Store, mgr AppRestarter, logger *slog.Logger) *AutoRestarter {
	return &AutoRestarter{
		backoffs:     make(map[string]*containerBackoff),
		pending:      make(chan RestartRequest, 100),
		store:        st,
		manager:      mgr,
		logger:       logger,
		InitialDelay: 1 * time.Second,
		MaxDelay:     5 * time.Minute,
		Multiplier:   2.0,
	}
}

// SetManager sets the AppRestarter implementation (called after manager is created).
func (r *AutoRestarter) SetManager(mgr AppRestarter) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.manager = mgr
}

// Start begins processing restart requests.
func (r *AutoRestarter) Start(ctx context.Context) error {
	r.ctx, r.cancel = context.WithCancel(ctx)

	r.wg.Add(1)
	go r.processRestarts()

	r.logger.Info("auto-restarter started")
	return nil
}

// Stop gracefully shuts down the restarter.
func (r *AutoRestarter) Stop() error {
	if r.cancel != nil {
		r.cancel()
	}
	r.wg.Wait()
	return nil
}

// RequestRestart queues a container restart request.
func (r *AutoRestarter) RequestRestart(containerID, appID, reason string) {
	req := RestartRequest{
		ContainerID: containerID,
		AppID:       appID,
		Reason:      reason,
		RequestedAt: time.Now(),
	}

	select {
	case r.pending <- req:
		r.logger.Info("restart requested", "container", containerID[:12], "reason", reason)
	default:
		r.logger.Warn("restart queue full, dropping request", "container", containerID[:12])
	}
}

// processRestarts drains the pending queue and handles restarts with backoff.
func (r *AutoRestarter) processRestarts() {
	defer r.wg.Done()

	for {
		select {
		case <-r.ctx.Done():
			return
		case req := <-r.pending:
			r.handleRestart(req)
		}
	}
}

// handleRestart performs a single restart with backoff enforcement.
func (r *AutoRestarter) handleRestart(req RestartRequest) {
	r.mu.Lock()
	backoff, ok := r.backoffs[req.ContainerID]
	if !ok {
		backoff = &containerBackoff{}
		r.backoffs[req.ContainerID] = backoff
	}

	now := time.Now()

	// Check if we're still in backoff period
	if now.Before(backoff.nextAllowed) {
		waitTime := backoff.nextAllowed.Sub(now)
		r.mu.Unlock()

		r.logger.Info("restart delayed by backoff",
			"container", req.ContainerID[:12],
			"wait", waitTime,
			"attempt", backoff.attempts)

		// Wait for backoff period
		select {
		case <-r.ctx.Done():
			return
		case <-time.After(waitTime):
		}

		r.mu.Lock()
	}

	// Calculate next backoff
	backoff.attempts++
	backoff.lastAttempt = time.Now()
	delay := r.CalculateDelay(backoff.attempts)
	backoff.nextAllowed = time.Now().Add(delay)

	mgr := r.manager
	r.mu.Unlock()

	if mgr == nil {
		r.logger.Error("restart failed: no manager configured", "container", req.ContainerID[:12])
		return
	}

	// Perform the restart
	r.logger.Info("restarting container",
		"container", req.ContainerID[:12],
		"app", req.AppID,
		"attempt", backoff.attempts,
		"next_backoff", delay)

	ctx, cancel := context.WithTimeout(r.ctx, 30*time.Second)
	defer cancel()

	if err := mgr.RestartContainer(ctx, req.ContainerID, req.AppID); err != nil {
		r.logger.Error("restart failed",
			"container", req.ContainerID[:12],
			"error", err)
	} else {
		r.logger.Info("restart succeeded", "container", req.ContainerID[:12])
		// Update container restart count in store
		r.incrementRestartCount(req.ContainerID)
	}
}

// CalculateDelay computes the backoff delay for a given attempt number.
func (r *AutoRestarter) CalculateDelay(attempts int) time.Duration {
	delay := float64(r.InitialDelay)
	for i := 1; i < attempts; i++ {
		delay *= r.Multiplier
		if delay > float64(r.MaxDelay) {
			delay = float64(r.MaxDelay)
			break
		}
	}
	return time.Duration(delay)
}

// incrementRestartCount bumps the restart count for a container in the store.
func (r *AutoRestarter) incrementRestartCount(containerID string) {
	container, err := r.store.GetContainer(containerID)
	if err != nil {
		return
	}
	container.RestartCount++
	r.store.UpdateContainer(container)
}

// ResetBackoff clears backoff state for a container (called when container recovers).
func (r *AutoRestarter) ResetBackoff(containerID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.backoffs, containerID)
}

// GetBackoffState returns current backoff info for a container.
func (r *AutoRestarter) GetBackoffState(containerID string) (attempts int, nextAllowed time.Time, ok bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if b, exists := r.backoffs[containerID]; exists {
		return b.attempts, b.nextAllowed, true
	}
	return 0, time.Time{}, false
}
