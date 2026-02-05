package manager

import (
	"context"
	"fmt"

	vessel "github.com/vessel/vessel/internal"
	"github.com/vessel/vessel/internal/config"
	"github.com/vessel/vessel/internal/store"
)

// RollbackApp reverts an app to a previous deploy version.
// If version is 0, rolls back to the previous version (current - 1).
func (m *AppManager) RollbackApp(ctx context.Context, appName string, version int) (*store.Deploy, error) {
	// Get the app
	app, err := m.store.GetApp(appName)
	if err != nil {
		return nil, fmt.Errorf("app %q not found: %w", appName, err)
	}

	// Get deploy history
	deploys, err := m.store.ListDeploysByApp(app.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get deploy history: %w", err)
	}
	if len(deploys) == 0 {
		return nil, vessel.ErrNoDeployHistory
	}

	// Find the current version (highest version)
	currentVersion := deploys[0].Version // deploys sorted newest-first

	// Determine target version
	targetVersion := version
	if targetVersion == 0 {
		// Roll back to previous version
		targetVersion = currentVersion - 1
		if targetVersion < 1 {
			return nil, fmt.Errorf("no previous version to roll back to")
		}
	}

	// Find the target deploy
	targetDeploy, err := m.store.GetDeployByVersion(app.ID, targetVersion)
	if err != nil {
		return nil, fmt.Errorf("version %d not found: %w", targetVersion, vessel.ErrVersionNotFound)
	}

	// Validate: target must have been a successful deploy
	if targetDeploy.Status != store.DeployStatusActive && targetDeploy.Status != store.DeployStatusRolledBack {
		return nil, fmt.Errorf("cannot roll back to version %d: deploy status was %q (must be active or rolled_back)",
			targetVersion, targetDeploy.Status)
	}

	m.logger.Info("rolling back app", "app", appName,
		"from_version", currentVersion, "to_version", targetVersion,
		"image", targetDeploy.Image)

	// Build a config from the target deploy's parameters
	appCfg := &config.AppConfig{
		Name:      app.Name,
		Image:     targetDeploy.Image,
		Instances: app.Instances,
		Command:   app.Command,
		Env:       app.Env,
	}

	// Use the same strategy as the target deploy
	switch targetDeploy.Strategy {
	case store.DeployStrategyBlueGreen:
		appCfg.Deploy.Strategy = "blue-green"
	default:
		appCfg.Deploy.Strategy = "rolling"
	}

	// Apply defaults for any missing config fields
	config.ApplyDefaults(&config.Config{Apps: []config.AppConfig{*appCfg}})

	// Perform the deploy using the standard pipeline
	newDeploy, err := m.DeployAppFromConfig(ctx, appCfg)
	if err != nil {
		return nil, fmt.Errorf("rollback deploy failed: %w", err)
	}

	// Mark the new deploy as a rollback
	newDeploy.RollbackOf = &currentVersion
	m.store.UpdateDeploy(newDeploy)

	// Mark the current deploy as rolled_back
	if currentDeploy, err := m.store.GetDeployByVersion(app.ID, currentVersion); err == nil {
		currentDeploy.Status = store.DeployStatusRolledBack
		m.store.UpdateDeploy(currentDeploy)
	}

	m.logger.Info("rollback completed", "app", appName,
		"new_version", newDeploy.Version, "rolled_back_from", currentVersion)

	return newDeploy, nil
}
