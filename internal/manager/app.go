// Package manager handles app lifecycle and deploy orchestration.
package manager

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	vessel "github.com/vessel/vessel/internal"
	"github.com/vessel/vessel/internal/runtime"
	"github.com/vessel/vessel/internal/store"
)

// AppManager orchestrates application lifecycle operations.
type AppManager struct {
	runtime    *runtime.LinuxRuntime
	store      store.Store
	reconciler *Reconciler
	logger     *slog.Logger
	mu         sync.Mutex
}

// NewAppManager creates a new AppManager.
func NewAppManager(rt *runtime.LinuxRuntime, st store.Store, logger *slog.Logger) *AppManager {
	m := &AppManager{
		runtime: rt,
		store:   st,
		logger:  logger,
	}
	m.reconciler = NewReconciler(rt, st, logger)
	return m
}

// GetReconciler returns the manager's reconciler for use by the daemon.
func (m *AppManager) GetReconciler() *Reconciler {
	return m.reconciler
}

// ListApps returns all known apps with their current state.
func (m *AppManager) ListApps(ctx context.Context) ([]*store.App, error) {
	apps, err := m.store.ListApps()
	if err != nil {
		return nil, fmt.Errorf("failed to list apps: %w", err)
	}
	return apps, nil
}

// GetApp returns a single app by name.
func (m *AppManager) GetApp(ctx context.Context, appName string) (*store.App, error) {
	app, err := m.store.GetApp(appName)
	if err != nil {
		return nil, err
	}
	return app, nil
}

// GetContainers returns all containers for an app.
func (m *AppManager) GetContainers(ctx context.Context, appName string) ([]*store.Container, error) {
	app, err := m.store.GetApp(appName)
	if err != nil {
		return nil, err
	}

	containers, err := m.store.ListContainersByApp(app.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}
	return containers, nil
}

// StopApp stops all containers for an app.
func (m *AppManager) StopApp(ctx context.Context, appName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	app, err := m.store.GetApp(appName)
	if err != nil {
		return err
	}

	if app.State == store.AppStateStopped {
		return nil
	}

	m.logger.Info("stopping app", "app", appName)

	containers, err := m.store.ListContainersByApp(app.ID)
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	gracePeriod := 30 * time.Second

	for _, c := range containers {
		if c.State != store.ContainerStateRunning {
			continue
		}

		m.logger.Info("stopping container", "container", c.ID[:12], "app", appName)
		if err := m.runtime.StopContainer(ctx, c.ID, gracePeriod); err != nil {
			m.logger.Error("failed to stop container", "container", c.ID[:12], "error", err)
			continue
		}

		now := time.Now()
		c.State = store.ContainerStateStopped
		c.StoppedAt = &now
		if err := m.store.UpdateContainer(c); err != nil {
			m.logger.Error("failed to update container state", "container", c.ID[:12], "error", err)
		}
	}

	app.State = store.AppStateStopped
	if err := m.store.UpdateApp(app); err != nil {
		return fmt.Errorf("failed to update app state: %w", err)
	}

	m.logger.Info("app stopped", "app", appName)
	return nil
}

// RemoveApp stops and removes all containers and state for an app.
func (m *AppManager) RemoveApp(ctx context.Context, appName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	app, err := m.store.GetApp(appName)
	if err != nil {
		return err
	}

	m.logger.Info("removing app", "app", appName)

	containers, err := m.store.ListContainersByApp(app.ID)
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	// Stop and remove each container
	for _, c := range containers {
		if c.State == store.ContainerStateRunning {
			if err := m.runtime.StopContainer(ctx, c.ID, 10*time.Second); err != nil {
				m.logger.Error("failed to stop container during removal", "container", c.ID[:12], "error", err)
			}
		}

		if err := m.runtime.RemoveContainer(ctx, c.ID); err != nil {
			m.logger.Error("failed to remove container", "container", c.ID[:12], "error", err)
		}

		if err := m.store.DeleteContainer(c.ID); err != nil {
			m.logger.Error("failed to delete container from store", "container", c.ID[:12], "error", err)
		}
	}

	// Delete the app from the store
	if err := m.store.DeleteApp(appName); err != nil {
		return fmt.Errorf("failed to delete app: %w", err)
	}

	m.logger.Info("app removed", "app", appName)
	return nil
}

// RestartApp restarts all containers for an app.
func (m *AppManager) RestartApp(ctx context.Context, appName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	app, err := m.store.GetApp(appName)
	if err != nil {
		return err
	}

	m.logger.Info("restarting app", "app", appName)

	containers, err := m.store.ListContainersByApp(app.ID)
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	// Stop existing containers
	for _, c := range containers {
		if c.State == store.ContainerStateRunning {
			if err := m.runtime.StopContainer(ctx, c.ID, 10*time.Second); err != nil {
				m.logger.Error("failed to stop container during restart", "container", c.ID[:12], "error", err)
			}
		}
		if err := m.runtime.RemoveContainer(ctx, c.ID); err != nil {
			m.logger.Error("failed to remove container during restart", "container", c.ID[:12], "error", err)
		}
		if err := m.store.DeleteContainer(c.ID); err != nil {
			m.logger.Error("failed to delete container from store", "container", c.ID[:12], "error", err)
		}
	}

	// Build environment list
	envList := buildEnvList(app.Env)

	// Start new containers
	for i := 0; i < app.Instances; i++ {
		c, err := m.runtime.CreateContainer(ctx, runtime.ContainerOpts{
			Name:      fmt.Sprintf("%s-%d", app.Name, i),
			Image:     app.Image,
			Command:   app.Command,
			Env:       envList,
			Resources: app.Resources,
		})
		if err != nil {
			m.logger.Error("failed to create container during restart", "app", appName, "error", err)
			continue
		}
		c.AppID = app.ID

		if err := m.store.CreateContainer(c); err != nil {
			m.logger.Error("failed to persist container", "container", c.ID[:12], "error", err)
			continue
		}

		if err := m.runtime.StartContainer(ctx, c.ID); err != nil {
			m.logger.Error("failed to start container during restart", "container", c.ID[:12], "error", err)
			continue
		}

		now := time.Now()
		c.State = store.ContainerStateRunning
		c.StartedAt = &now
		if err := m.store.UpdateContainer(c); err != nil {
			m.logger.Error("failed to update container state", "container", c.ID[:12], "error", err)
		}
	}

	app.State = store.AppStateRunning
	if err := m.store.UpdateApp(app); err != nil {
		return fmt.Errorf("failed to update app state: %w", err)
	}

	m.logger.Info("app restarted", "app", appName)
	return nil
}

// DeployApp performs a full deploy. Stubbed for now — implemented in Phase 2 Session 2.
func (m *AppManager) DeployApp(ctx context.Context, appName string, image string, instances int, env map[string]string, resources store.ResourceLimits) (*store.Deploy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if app exists, create if not
	app, err := m.store.GetApp(appName)
	if err != nil {
		// Create a new app
		app = &store.App{
			ID:        appName, // Use name as ID for simplicity
			Name:      appName,
			Image:     image,
			Instances: instances,
			Env:       env,
			Resources: resources,
			State:     store.AppStateDeploying,
		}
		if err := m.store.CreateApp(app); err != nil {
			return nil, fmt.Errorf("failed to create app: %w", err)
		}
	} else {
		// Update existing app
		app.Image = image
		app.Instances = instances
		app.Env = env
		app.Resources = resources
		app.State = store.AppStateDeploying
		if err := m.store.UpdateApp(app); err != nil {
			return nil, fmt.Errorf("failed to update app: %w", err)
		}
	}

	m.logger.Info("deploying app", "app", appName, "image", image, "instances", instances)

	// Pull the image first
	if err := m.runtime.PullImage(ctx, image); err != nil {
		app.State = store.AppStateFailed
		m.store.UpdateApp(app)
		return nil, fmt.Errorf("failed to pull image: %w", err)
	}

	// Stop and remove existing containers
	existingContainers, _ := m.store.ListContainersByApp(app.ID)
	for _, c := range existingContainers {
		if c.State == store.ContainerStateRunning {
			m.runtime.StopContainer(ctx, c.ID, 10*time.Second)
		}
		m.runtime.RemoveContainer(ctx, c.ID)
		m.store.DeleteContainer(c.ID)
	}

	// Build environment list
	envList := buildEnvList(env)

	// Create and start new containers
	var lastErr error
	for i := 0; i < instances; i++ {
		c, err := m.runtime.CreateContainer(ctx, runtime.ContainerOpts{
			Name:      fmt.Sprintf("%s-%d", appName, i),
			Image:     image,
			Env:       envList,
			Resources: resources,
		})
		if err != nil {
			m.logger.Error("failed to create container", "app", appName, "instance", i, "error", err)
			lastErr = err
			continue
		}
		c.AppID = app.ID

		if err := m.store.CreateContainer(c); err != nil {
			m.logger.Error("failed to persist container", "container", c.ID[:12], "error", err)
			lastErr = err
			continue
		}

		if err := m.runtime.StartContainer(ctx, c.ID); err != nil {
			m.logger.Error("failed to start container", "container", c.ID[:12], "error", err)
			lastErr = err
			continue
		}

		now := time.Now()
		c.State = store.ContainerStateRunning
		c.StartedAt = &now
		if err := m.store.UpdateContainer(c); err != nil {
			m.logger.Error("failed to update container state", "container", c.ID[:12], "error", err)
		}
	}

	// Update app state
	if lastErr != nil {
		app.State = store.AppStateFailed
	} else {
		app.State = store.AppStateRunning
	}
	m.store.UpdateApp(app)

	// Create deploy record
	deploy := &store.Deploy{
		ID:       fmt.Sprintf("%s-%d", appName, time.Now().UnixNano()),
		AppID:    app.ID,
		Image:    image,
		Strategy: store.DeployStrategyRolling,
		Status:   store.DeployStatusActive,
	}
	if lastErr != nil {
		deploy.Status = store.DeployStatusFailed
		deploy.Error = lastErr.Error()
	}
	now := time.Now()
	deploy.FinishedAt = &now
	m.store.CreateDeploy(deploy)

	m.logger.Info("deploy completed", "app", appName, "status", deploy.Status)
	return deploy, lastErr
}

// RollbackApp reverts to a previous deploy version. Stubbed for now.
func (m *AppManager) RollbackApp(ctx context.Context, appName string, version int) (*store.Deploy, error) {
	return nil, fmt.Errorf("%w: rollback not yet implemented", vessel.ErrNoDeployHistory)
}

// GetDeployHistory returns deploy history for an app.
func (m *AppManager) GetDeployHistory(ctx context.Context, appName string) ([]*store.Deploy, error) {
	app, err := m.store.GetApp(appName)
	if err != nil {
		return nil, err
	}
	return m.store.ListDeploysByApp(app.ID)
}

// buildEnvList converts a map of env vars to a slice of KEY=VALUE strings.
func buildEnvList(env map[string]string) []string {
	if len(env) == 0 {
		return nil
	}
	list := make([]string, 0, len(env))
	for k, v := range env {
		list = append(list, fmt.Sprintf("%s=%s", k, v))
	}
	return list
}
