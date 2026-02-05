package manager

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/vessel/vessel/internal/runtime"
	"github.com/vessel/vessel/internal/store"
)

// Reconciler continuously ensures actual state matches desired state.
type Reconciler struct {
	runtime  *runtime.LinuxRuntime
	store    store.Store
	logger   *slog.Logger
	interval time.Duration

	// backoff tracks restart backoff per container
	mu       sync.Mutex
	backoffs map[string]*backoffState
}

// backoffState tracks exponential backoff for container restarts.
type backoffState struct {
	delay     time.Duration
	nextRetry time.Time
	failures  int
}

const (
	initialBackoff = 1 * time.Second
	maxBackoff     = 5 * time.Minute
)

// NewReconciler creates a new Reconciler.
func NewReconciler(rt *runtime.LinuxRuntime, st store.Store, logger *slog.Logger) *Reconciler {
	return &Reconciler{
		runtime:  rt,
		store:    st,
		logger:   logger,
		interval: 5 * time.Second,
		backoffs: make(map[string]*backoffState),
	}
}

// Run starts the reconciliation loop. It blocks until ctx is cancelled.
func (r *Reconciler) Run(ctx context.Context) {
	r.logger.Info("reconciler started", "interval", r.interval)
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			r.logger.Info("reconciler stopped")
			return
		case <-ticker.C:
			if err := r.reconcileAll(ctx); err != nil {
				r.logger.Error("reconciliation cycle failed", "error", err)
			}
		}
	}
}

// reconcileAll iterates over all apps and reconciles each one.
func (r *Reconciler) reconcileAll(ctx context.Context) error {
	apps, err := r.store.ListApps()
	if err != nil {
		return fmt.Errorf("failed to list apps: %w", err)
	}

	for _, app := range apps {
		if err := r.ReconcileApp(ctx, app); err != nil {
			r.logger.Error("failed to reconcile app", "app", app.Name, "error", err)
		}
	}

	return nil
}

// ReconcileApp ensures the actual state of an app matches its desired state.
// This method is idempotent — calling it multiple times produces the same result.
func (r *Reconciler) ReconcileApp(ctx context.Context, app *store.App) error {
	// Skip stopped apps — they should have no running containers
	if app.State == store.AppStateStopped {
		return r.ensureNoRunning(ctx, app)
	}

	containers, err := r.store.ListContainersByApp(app.ID)
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	// Detect dead containers: check if PIDs are still alive
	var running []*store.Container
	for _, c := range containers {
		if c.State == store.ContainerStateRunning {
			if r.isProcessAlive(c.PID) {
				running = append(running, c)
			} else {
				// Container died but store still says running
				r.logger.Warn("detected dead container", "container", c.ID[:12], "app", app.Name, "pid", c.PID)
				c.State = store.ContainerStateFailed
				now := time.Now()
				c.StoppedAt = &now
				exitCode := 137 // Assume killed
				c.ExitCode = &exitCode
				if err := r.store.UpdateContainer(c); err != nil {
					r.logger.Error("failed to update dead container state", "container", c.ID[:12], "error", err)
				}
			}
		}
	}

	// Re-fetch to get updated states
	containers, err = r.store.ListContainersByApp(app.ID)
	if err != nil {
		return fmt.Errorf("failed to re-list containers: %w", err)
	}

	// Count containers by state
	runningCount := 0
	var failedContainers []*store.Container
	for _, c := range containers {
		switch c.State {
		case store.ContainerStateRunning:
			runningCount++
		case store.ContainerStateFailed:
			failedContainers = append(failedContainers, c)
		}
	}

	desired := app.Instances
	if desired <= 0 {
		desired = 1
	}

	// If actual < desired: start new containers
	if runningCount < desired {
		needed := desired - runningCount

		// First try to restart failed containers
		restarted := 0
		for _, c := range failedContainers {
			if restarted >= needed {
				break
			}
			if r.shouldRestart(c) {
				if err := r.restartContainer(ctx, app, c); err != nil {
					r.logger.Error("failed to restart container", "container", c.ID[:12], "error", err)
					continue
				}
				restarted++
			}
		}

		// If we still need more, create new containers
		for i := restarted; i < needed; i++ {
			if err := r.createAndStartContainer(ctx, app); err != nil {
				r.logger.Error("failed to create container for scaling", "app", app.Name, "error", err)
			}
		}
	}

	// If actual > desired: stop excess containers (oldest first)
	if runningCount > desired {
		excess := runningCount - desired
		r.stopExcessContainers(ctx, app, containers, excess)
	}

	return nil
}

// ensureNoRunning stops any containers that shouldn't be running.
func (r *Reconciler) ensureNoRunning(ctx context.Context, app *store.App) error {
	containers, err := r.store.ListContainersByApp(app.ID)
	if err != nil {
		return err
	}

	for _, c := range containers {
		if c.State == store.ContainerStateRunning {
			r.logger.Info("stopping orphaned container for stopped app", "container", c.ID[:12], "app", app.Name)
			if err := r.runtime.StopContainer(ctx, c.ID, 10*time.Second); err != nil {
				r.logger.Error("failed to stop orphaned container", "container", c.ID[:12], "error", err)
			}
			now := time.Now()
			c.State = store.ContainerStateStopped
			c.StoppedAt = &now
			r.store.UpdateContainer(c)
		}
	}
	return nil
}

// isProcessAlive checks if a process with the given PID is still alive.
func (r *Reconciler) isProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	// Check /proc/<pid> exists
	_, err := os.Stat(fmt.Sprintf("/proc/%d", pid))
	return err == nil
}

// shouldRestart checks whether a failed container should be restarted,
// respecting exponential backoff.
func (r *Reconciler) shouldRestart(c *store.Container) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	bs, ok := r.backoffs[c.ID]
	if !ok {
		// First failure — restart immediately
		r.backoffs[c.ID] = &backoffState{
			delay:     initialBackoff,
			nextRetry: time.Now(),
			failures:  1,
		}
		return true
	}

	if time.Now().Before(bs.nextRetry) {
		return false
	}

	return true
}

// updateBackoff updates the backoff state after a restart attempt.
func (r *Reconciler) updateBackoff(containerID string, success bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if success {
		delete(r.backoffs, containerID)
		return
	}

	bs, ok := r.backoffs[containerID]
	if !ok {
		bs = &backoffState{delay: initialBackoff}
		r.backoffs[containerID] = bs
	}

	bs.failures++
	bs.delay *= 2
	if bs.delay > maxBackoff {
		bs.delay = maxBackoff
	}
	bs.nextRetry = time.Now().Add(bs.delay)
}

// restartContainer removes a failed container and starts a fresh one.
func (r *Reconciler) restartContainer(ctx context.Context, app *store.App, c *store.Container) error {
	r.logger.Info("restarting failed container", "container", c.ID[:12], "app", app.Name, "restart_count", c.RestartCount)

	// Remove the old container
	r.runtime.RemoveContainer(ctx, c.ID)
	r.store.DeleteContainer(c.ID)

	// Create a new one
	envList := buildEnvList(app.Env)
	newC, err := r.runtime.CreateContainer(ctx, runtime.ContainerOpts{
		Name:      fmt.Sprintf("%s-%d", app.Name, time.Now().UnixNano()%1000),
		Image:     app.Image,
		Command:   app.Command,
		Env:       envList,
		Resources: app.Resources,
	})
	if err != nil {
		r.updateBackoff(c.ID, false)
		return fmt.Errorf("failed to create replacement container: %w", err)
	}
	newC.AppID = app.ID
	newC.RestartCount = c.RestartCount + 1

	if err := r.store.CreateContainer(newC); err != nil {
		r.updateBackoff(c.ID, false)
		return fmt.Errorf("failed to persist replacement container: %w", err)
	}

	if err := r.runtime.StartContainer(ctx, newC.ID); err != nil {
		r.updateBackoff(c.ID, false)
		return fmt.Errorf("failed to start replacement container: %w", err)
	}

	now := time.Now()
	newC.State = store.ContainerStateRunning
	newC.StartedAt = &now
	r.store.UpdateContainer(newC)

	r.updateBackoff(c.ID, true)
	r.logger.Info("container restarted", "old_container", c.ID[:12], "new_container", newC.ID[:12], "app", app.Name)
	return nil
}

// createAndStartContainer creates and starts a new container for an app.
func (r *Reconciler) createAndStartContainer(ctx context.Context, app *store.App) error {
	envList := buildEnvList(app.Env)

	c, err := r.runtime.CreateContainer(ctx, runtime.ContainerOpts{
		Name:      fmt.Sprintf("%s-%d", app.Name, time.Now().UnixNano()%1000),
		Image:     app.Image,
		Command:   app.Command,
		Env:       envList,
		Resources: app.Resources,
	})
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}
	c.AppID = app.ID

	if err := r.store.CreateContainer(c); err != nil {
		return fmt.Errorf("failed to persist container: %w", err)
	}

	if err := r.runtime.StartContainer(ctx, c.ID); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	now := time.Now()
	c.State = store.ContainerStateRunning
	c.StartedAt = &now
	r.store.UpdateContainer(c)

	r.logger.Info("container created by reconciler", "container", c.ID[:12], "app", app.Name)
	return nil
}

// stopExcessContainers stops the oldest containers when actual > desired.
func (r *Reconciler) stopExcessContainers(ctx context.Context, app *store.App, containers []*store.Container, count int) {
	// Sort running containers by creation time (oldest first)
	var running []*store.Container
	for _, c := range containers {
		if c.State == store.ContainerStateRunning {
			running = append(running, c)
		}
	}
	sort.Slice(running, func(i, j int) bool {
		return running[i].CreatedAt.Before(running[j].CreatedAt)
	})

	stopped := 0
	for _, c := range running {
		if stopped >= count {
			break
		}
		r.logger.Info("stopping excess container", "container", c.ID[:12], "app", app.Name)
		if err := r.runtime.StopContainer(ctx, c.ID, 10*time.Second); err != nil {
			r.logger.Error("failed to stop excess container", "container", c.ID[:12], "error", err)
			continue
		}
		now := time.Now()
		c.State = store.ContainerStateStopped
		c.StoppedAt = &now
		r.store.UpdateContainer(c)
		stopped++
	}
}
