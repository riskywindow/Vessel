package manager

import (
	"context"
	"fmt"
	"net"
	"regexp"
	"time"

	"github.com/vessel/vessel/internal/config"
	"github.com/vessel/vessel/internal/network"
	"github.com/vessel/vessel/internal/runtime"
	"github.com/vessel/vessel/internal/store"
)

// secretRefPattern matches ${secret:key-name} references in env values.
var secretRefPattern = regexp.MustCompile(`\$\{secret:([^}]+)\}`)

const (
	// defaultDrainTimeout is how long to wait for in-flight requests before
	// stopping an old container during rolling updates.
	defaultDrainTimeout = 5 * time.Second

	// defaultDeployHealthTimeout is the maximum time to wait for a container
	// to become healthy during a deploy.
	defaultDeployHealthTimeout = 60 * time.Second

	// defaultDeployRetention is the number of deploy records to keep per app.
	defaultDeployRetention = 10
)

// DeployAppFromConfig performs a full deploy from a parsed AppConfig.
// For a NEW app: creates the app, pulls the image, starts containers, health checks.
// For an EXISTING app: performs a rolling or blue-green update.
func (m *AppManager) DeployAppFromConfig(ctx context.Context, appCfg *config.AppConfig) (*store.Deploy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if appCfg.Instances <= 0 {
		appCfg.Instances = 1
	}

	m.logger.Info("starting deploy", "app", appCfg.Name, "image", appCfg.Image,
		"instances", appCfg.Instances, "strategy", appCfg.Deploy.Strategy)

	// Resolve secret references in env vars
	if err := m.resolveSecretRefs(appCfg); err != nil {
		return nil, fmt.Errorf("failed to resolve secrets: %w", err)
	}

	// Pull the image
	if err := m.runtime.PullImage(ctx, appCfg.Image); err != nil {
		return nil, fmt.Errorf("failed to pull image %s: %w", appCfg.Image, err)
	}

	// Convert resource config to store.ResourceLimits
	resources, err := configToResourceLimits(&appCfg.Resources)
	if err != nil {
		return nil, fmt.Errorf("invalid resource config: %w", err)
	}

	// Check if app exists
	app, err := m.store.GetApp(appCfg.Name)
	if err != nil {
		// New app — first deploy
		return m.deployNewApp(ctx, appCfg, resources)
	}

	// Existing app — update deploy
	switch appCfg.Deploy.Strategy {
	case "blue-green":
		return m.deployBlueGreen(ctx, app, appCfg, resources)
	default:
		return m.deployRolling(ctx, app, appCfg, resources)
	}
}

// deployNewApp handles the first deploy of a brand-new application.
func (m *AppManager) deployNewApp(ctx context.Context, appCfg *config.AppConfig, resources store.ResourceLimits) (*store.Deploy, error) {
	// Create the app in the store
	app := &store.App{
		ID:        appCfg.Name,
		Name:      appCfg.Name,
		Image:     appCfg.Image,
		Command:   appCfg.Command,
		Instances: appCfg.Instances,
		Env:       appCfg.Env,
		Resources: resources,
		State:     store.AppStateDeploying,
	}
	if err := m.store.CreateApp(app); err != nil {
		return nil, fmt.Errorf("failed to create app: %w", err)
	}

	// Create a deploy record
	deploy, err := m.createDeployRecord(app.ID, appCfg)
	if err != nil {
		app.State = store.AppStateFailed
		m.store.UpdateApp(app)
		return nil, err
	}

	// Start all containers
	envList := buildEnvList(appCfg.Env)
	healthTimeout := m.healthTimeout(appCfg)

	var startedContainers []*store.Container
	for i := 0; i < appCfg.Instances; i++ {
		c, err := m.createAndStartManagedContainer(ctx, app, appCfg, envList, resources)
		if err != nil {
			m.logger.Error("failed to start container", "app", appCfg.Name, "instance", i, "error", err)
			m.cleanupContainers(ctx, startedContainers)
			return m.failDeploy(app, deploy, fmt.Sprintf("failed to start instance %d: %v", i, err))
		}

		// Health check
		if err := waitForHealthy(ctx, m.runtime, c, appCfg.HealthCheck, healthTimeout, m.logger); err != nil {
			m.logger.Error("health check failed", "container", c.ID[:12], "error", err)
			startedContainers = append(startedContainers, c)
			m.cleanupContainers(ctx, startedContainers)
			return m.failDeploy(app, deploy, fmt.Sprintf("health check failed for instance %d: %v", i, err))
		}

		startedContainers = append(startedContainers, c)
		m.logger.Info("instance healthy", "app", appCfg.Name, "container", c.ID[:12],
			"instance", i+1, "of", appCfg.Instances)

		// Register routes for each configured domain
		m.registerContainerRoutes(c, appCfg)
	}

	// Success: update app and deploy
	app.State = store.AppStateRunning
	app.UpdatedAt = time.Now()
	m.store.UpdateApp(app)

	return m.finishDeploy(deploy)
}

// deployRolling performs a rolling update: replaces containers one at a time.
func (m *AppManager) deployRolling(ctx context.Context, app *store.App, appCfg *config.AppConfig, resources store.ResourceLimits) (*store.Deploy, error) {
	// Update app metadata
	app.Image = appCfg.Image
	app.Command = appCfg.Command
	app.Instances = appCfg.Instances
	app.Env = appCfg.Env
	app.Resources = resources
	app.State = store.AppStateDeploying
	m.store.UpdateApp(app)

	// Create deploy record
	deploy, err := m.createDeployRecord(app.ID, appCfg)
	if err != nil {
		app.State = store.AppStateFailed
		m.store.UpdateApp(app)
		return nil, err
	}

	// Get current running containers
	oldContainers, err := m.getRunningContainers(app.ID)
	if err != nil {
		return m.failDeploy(app, deploy, fmt.Sprintf("failed to list containers: %v", err))
	}

	envList := buildEnvList(appCfg.Env)
	healthTimeout := m.healthTimeout(appCfg)
	drainTimeout := defaultDrainTimeout
	if appCfg.Deploy.GracePeriod.Duration > 0 {
		drainTimeout = appCfg.Deploy.GracePeriod.Duration
	}

	// Replace containers one at a time
	var newContainers []*store.Container
	for i := 0; i < appCfg.Instances; i++ {
		// Start a new container
		newC, err := m.createAndStartManagedContainer(ctx, app, appCfg, envList, resources)
		if err != nil {
			m.logger.Error("rolling update: failed to start new container", "instance", i, "error", err)
			m.cleanupContainers(ctx, newContainers)
			return m.failDeploy(app, deploy, fmt.Sprintf("failed to start instance %d: %v", i, err))
		}

		// Health check the new container
		if err := waitForHealthy(ctx, m.runtime, newC, appCfg.HealthCheck, healthTimeout, m.logger); err != nil {
			m.logger.Error("rolling update: health check failed", "container", newC.ID[:12], "error", err)
			m.cleanupContainers(ctx, []*store.Container{newC})
			m.cleanupContainers(ctx, newContainers)
			return m.failDeploy(app, deploy, fmt.Sprintf("health check failed for instance %d: %v", i, err))
		}

		newContainers = append(newContainers, newC)
		m.logger.Info("rolling update: new instance healthy", "container", newC.ID[:12],
			"instance", i+1, "of", appCfg.Instances)

		// Register routes for the new container
		m.registerContainerRoutes(newC, appCfg)

		// Remove one old container (if available)
		if i < len(oldContainers) {
			old := oldContainers[i]
			m.logger.Info("rolling update: draining old container",
				"container", old.ID[:12], "drain_timeout", drainTimeout)
			time.Sleep(drainTimeout)
			m.stopAndRemoveContainer(ctx, old)
		}
	}

	// If we have more old containers than new instances, remove the extras
	if len(oldContainers) > appCfg.Instances {
		for _, old := range oldContainers[appCfg.Instances:] {
			m.stopAndRemoveContainer(ctx, old)
		}
	}

	// Success
	app.State = store.AppStateRunning
	app.UpdatedAt = time.Now()
	m.store.UpdateApp(app)

	return m.finishDeploy(deploy)
}

// deployBlueGreen performs a blue-green deploy: starts all new containers,
// health checks all of them, then atomically swaps traffic and removes old ones.
func (m *AppManager) deployBlueGreen(ctx context.Context, app *store.App, appCfg *config.AppConfig, resources store.ResourceLimits) (*store.Deploy, error) {
	// Update app metadata
	app.Image = appCfg.Image
	app.Command = appCfg.Command
	app.Instances = appCfg.Instances
	app.Env = appCfg.Env
	app.Resources = resources
	app.State = store.AppStateDeploying
	m.store.UpdateApp(app)

	// Create deploy record
	deploy, err := m.createDeployRecord(app.ID, appCfg)
	if err != nil {
		app.State = store.AppStateFailed
		m.store.UpdateApp(app)
		return nil, err
	}

	// Get old containers
	oldContainers, err := m.getRunningContainers(app.ID)
	if err != nil {
		return m.failDeploy(app, deploy, fmt.Sprintf("failed to list containers: %v", err))
	}

	envList := buildEnvList(appCfg.Env)
	healthTimeout := m.healthTimeout(appCfg)
	drainTimeout := defaultDrainTimeout
	if appCfg.Deploy.GracePeriod.Duration > 0 {
		drainTimeout = appCfg.Deploy.GracePeriod.Duration
	}

	// Start ALL new containers
	var newContainers []*store.Container
	for i := 0; i < appCfg.Instances; i++ {
		newC, err := m.createAndStartManagedContainer(ctx, app, appCfg, envList, resources)
		if err != nil {
			m.logger.Error("blue-green: failed to start new container", "instance", i, "error", err)
			m.cleanupContainers(ctx, newContainers)
			return m.failDeploy(app, deploy, fmt.Sprintf("failed to start instance %d: %v", i, err))
		}
		newContainers = append(newContainers, newC)
	}

	// Health check ALL new containers
	for i, newC := range newContainers {
		if err := waitForHealthy(ctx, m.runtime, newC, appCfg.HealthCheck, healthTimeout, m.logger); err != nil {
			m.logger.Error("blue-green: health check failed", "container", newC.ID[:12], "error", err)
			m.cleanupContainers(ctx, newContainers)
			return m.failDeploy(app, deploy, fmt.Sprintf("health check failed for instance %d: %v", i, err))
		}
		m.logger.Info("blue-green: instance healthy", "container", newC.ID[:12],
			"instance", i+1, "of", appCfg.Instances)
	}

	// All new containers healthy — atomic swap: register new routes
	m.logger.Info("blue-green: all instances healthy, swapping traffic", "app", appCfg.Name)
	for _, newC := range newContainers {
		m.registerContainerRoutes(newC, appCfg)
	}

	// Wait drain timeout for in-flight requests to old containers
	time.Sleep(drainTimeout)

	// Stop and remove all old containers
	for _, old := range oldContainers {
		m.stopAndRemoveContainer(ctx, old)
	}

	// Success
	app.State = store.AppStateRunning
	app.UpdatedAt = time.Now()
	m.store.UpdateApp(app)

	return m.finishDeploy(deploy)
}

// createDeployRecord creates a new Deploy in the store with a monotonic version.
func (m *AppManager) createDeployRecord(appID string, appCfg *config.AppConfig) (*store.Deploy, error) {
	version, err := m.store.GetNextDeployVersion(appID)
	if err != nil {
		return nil, fmt.Errorf("failed to get next deploy version: %w", err)
	}

	strategy := store.DeployStrategyRolling
	if appCfg.Deploy.Strategy == "blue-green" {
		strategy = store.DeployStrategyBlueGreen
	}

	deploy := &store.Deploy{
		ID:       fmt.Sprintf("%s-v%d", appID, version),
		AppID:    appID,
		Image:    appCfg.Image,
		Strategy: strategy,
		Status:   store.DeployStatusPending,
		Version:  version,
	}

	if err := m.store.CreateDeploy(deploy); err != nil {
		return nil, fmt.Errorf("failed to create deploy record: %w", err)
	}

	return deploy, nil
}

// finishDeploy marks a deploy as active and prunes old deploy history.
func (m *AppManager) finishDeploy(deploy *store.Deploy) (*store.Deploy, error) {
	now := time.Now()
	deploy.Status = store.DeployStatusActive
	deploy.FinishedAt = &now
	if err := m.store.UpdateDeploy(deploy); err != nil {
		m.logger.Error("failed to update deploy status", "deploy", deploy.ID, "error", err)
	}

	// Prune old deploys (best-effort)
	if bs, ok := m.store.(*store.BoltStore); ok {
		bs.PruneDeployHistory(deploy.AppID, defaultDeployRetention)
	}

	m.logger.Info("deploy completed successfully", "app", deploy.AppID, "version", deploy.Version, "image", deploy.Image)
	return deploy, nil
}

// failDeploy marks a deploy as failed and updates the app state.
func (m *AppManager) failDeploy(app *store.App, deploy *store.Deploy, errMsg string) (*store.Deploy, error) {
	now := time.Now()
	deploy.Status = store.DeployStatusFailed
	deploy.Error = errMsg
	deploy.FinishedAt = &now
	m.store.UpdateDeploy(deploy)

	app.State = store.AppStateFailed
	m.store.UpdateApp(app)

	m.logger.Error("deploy failed", "app", app.Name, "version", deploy.Version, "error", errMsg)
	return deploy, fmt.Errorf("deploy failed: %s", errMsg)
}

// createAndStartManagedContainer creates a container, persists it, and starts it.
func (m *AppManager) createAndStartManagedContainer(ctx context.Context, app *store.App, appCfg *config.AppConfig, envList []string, resources store.ResourceLimits) (*store.Container, error) {
	c, err := m.runtime.CreateContainer(ctx, runtime.ContainerOpts{
		Name:      fmt.Sprintf("%s-%d", app.Name, time.Now().UnixNano()%10000),
		Image:     appCfg.Image,
		Command:   appCfg.Command,
		Env:       envList,
		Resources: resources,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create container: %w", err)
	}
	c.AppID = app.ID

	if err := m.store.CreateContainer(c); err != nil {
		m.runtime.RemoveContainer(ctx, c.ID)
		return nil, fmt.Errorf("failed to persist container: %w", err)
	}

	if err := m.runtime.StartContainer(ctx, c.ID); err != nil {
		m.store.DeleteContainer(c.ID)
		m.runtime.RemoveContainer(ctx, c.ID)
		return nil, fmt.Errorf("failed to start container: %w", err)
	}

	// Set up container networking (best-effort — deploy succeeds even if networking fails)
	if m.network != nil {
		pid, err := m.runtime.GetContainerPID(c.ID)
		if err != nil {
			m.logger.Warn("failed to get container PID for networking", "container", c.ID[:12], "error", err)
		} else {
			netInfo, err := m.network.SetupContainerNetwork(ctx, c.ID, pid)
			if err != nil {
				m.logger.Error("failed to setup container network", "container", c.ID[:12], "error", err)
			} else {
				c.IP = netInfo.IP.String()
				c.PID = pid
				m.logger.Info("container network configured", "container", c.ID[:12], "ip", c.IP)
			}
		}
	}

	now := time.Now()
	c.State = store.ContainerStateRunning
	c.StartedAt = &now
	m.store.UpdateContainer(c)

	return c, nil
}

// getRunningContainers returns all running containers for an app.
func (m *AppManager) getRunningContainers(appID string) ([]*store.Container, error) {
	containers, err := m.store.ListContainersByApp(appID)
	if err != nil {
		return nil, err
	}

	var running []*store.Container
	for _, c := range containers {
		if c.State == store.ContainerStateRunning {
			running = append(running, c)
		}
	}
	return running, nil
}

// stopAndRemoveContainer stops and removes a single container.
func (m *AppManager) stopAndRemoveContainer(ctx context.Context, c *store.Container) {
	// Deregister routes before teardown so proxy stops sending traffic
	if m.network != nil {
		if err := m.network.DeregisterRoute("", c.ID); err != nil {
			m.logger.Warn("failed to deregister container routes", "container", c.ID[:12], "error", err)
		}
	}

	// Teardown container networking
	if m.network != nil {
		if err := m.network.TeardownContainerNetwork(ctx, c.ID); err != nil {
			m.logger.Warn("failed to teardown container network", "container", c.ID[:12], "error", err)
		}
	}

	if c.State == store.ContainerStateRunning {
		if err := m.runtime.StopContainer(ctx, c.ID, 10*time.Second); err != nil {
			m.logger.Error("failed to stop old container", "container", c.ID[:12], "error", err)
		}
	}
	if err := m.runtime.RemoveContainer(ctx, c.ID); err != nil {
		m.logger.Error("failed to remove old container", "container", c.ID[:12], "error", err)
	}
	if err := m.store.DeleteContainer(c.ID); err != nil {
		m.logger.Error("failed to delete container from store", "container", c.ID[:12], "error", err)
	}
}

// cleanupContainers stops and removes a slice of containers.
func (m *AppManager) cleanupContainers(ctx context.Context, containers []*store.Container) {
	for _, c := range containers {
		m.stopAndRemoveContainer(ctx, c)
	}
}

// healthTimeout returns the deploy health check timeout for an app config.
func (m *AppManager) healthTimeout(appCfg *config.AppConfig) time.Duration {
	if appCfg.HealthCheck != nil && appCfg.HealthCheck.Timeout.Duration > 0 {
		retries := appCfg.HealthCheck.Retries
		if retries <= 0 {
			retries = 3
		}
		interval := appCfg.HealthCheck.Interval.Duration
		if interval <= 0 {
			interval = defaultHealthInterval
		}
		return time.Duration(retries+2) * interval
	}
	return defaultDeployHealthTimeout
}

// resolveSecretRefs scans env values for ${secret:key} patterns and resolves them
// using the secret manager. The resolved plaintext values replace the references.
func (m *AppManager) resolveSecretRefs(appCfg *config.AppConfig) error {
	if len(appCfg.Env) == 0 {
		return nil
	}

	for key, value := range appCfg.Env {
		resolved, err := m.resolveSecretValue(value)
		if err != nil {
			return fmt.Errorf("env %s: %w", key, err)
		}
		appCfg.Env[key] = resolved
	}
	return nil
}

// resolveSecretValue replaces all ${secret:name} references in a string.
func (m *AppManager) resolveSecretValue(value string) (string, error) {
	matches := secretRefPattern.FindAllStringSubmatchIndex(value, -1)
	if len(matches) == 0 {
		return value, nil
	}

	if m.secretManager == nil {
		return "", fmt.Errorf("secret references found but secret manager is not initialized")
	}

	result := value
	// Process matches in reverse order to preserve indices
	for i := len(matches) - 1; i >= 0; i-- {
		match := matches[i]
		fullStart, fullEnd := match[0], match[1]
		nameStart, nameEnd := match[2], match[3]

		secretName := value[nameStart:nameEnd]
		secretValue, err := m.secretManager.GetSecret(secretName)
		if err != nil {
			return "", fmt.Errorf("failed to resolve secret %q: %w", secretName, err)
		}

		result = result[:fullStart] + secretValue + result[fullEnd:]
	}

	return result, nil
}

// ResolveSecretRefs is exported for testing.
func ResolveSecretRefs(env map[string]string, sm *store.SecretManager) (map[string]string, error) {
	if len(env) == 0 {
		return env, nil
	}
	result := make(map[string]string, len(env))
	for k, v := range env {
		matches := secretRefPattern.FindAllStringSubmatchIndex(v, -1)
		if len(matches) == 0 {
			result[k] = v
			continue
		}
		if sm == nil {
			return nil, fmt.Errorf("env %s: secret references found but secret manager is not initialized", k)
		}
		resolved := v
		for i := len(matches) - 1; i >= 0; i-- {
			match := matches[i]
			secretName := v[match[2]:match[3]]
			secretValue, err := sm.GetSecret(secretName)
			if err != nil {
				return nil, fmt.Errorf("env %s: failed to resolve secret %q: %w", k, secretName, err)
			}
			resolved = resolved[:match[0]] + secretValue + resolved[match[1]:]
		}
		result[k] = resolved
	}
	return result, nil
}

// registerContainerRoutes registers reverse proxy routes for all configured domains.
func (m *AppManager) registerContainerRoutes(c *store.Container, appCfg *config.AppConfig) {
	if m.network == nil || len(appCfg.Domains) == 0 || c.IP == "" {
		return
	}

	// Determine the container port to route to.
	// Use the first port mapping's container port, or default to 80.
	port := 80
	if len(appCfg.Ports) > 0 {
		port = appCfg.Ports[0].Container
	}

	for _, domain := range appCfg.Domains {
		target := network.RouteTarget{
			ContainerID: c.ID,
			IP:          net.ParseIP(c.IP),
			Port:        port,
		}
		if err := m.network.RegisterRoute(domain, target); err != nil {
			m.logger.Warn("failed to register route", "domain", domain, "container", c.ID[:12], "error", err)
		}
	}
}

// configToResourceLimits converts a config.ResourceConfig to store.ResourceLimits.
func configToResourceLimits(rc *config.ResourceConfig) (store.ResourceLimits, error) {
	var r store.ResourceLimits

	// CPU: config uses percentage (100 = 1 core), convert to microseconds
	if rc.CPU > 0 {
		r.CPUPeriod = 100000 // 100ms
		r.CPUQuota = int64(rc.CPU) * 1000
	}

	// Memory
	if rc.Memory != "" {
		memBytes, err := config.ParseMemorySize(rc.Memory)
		if err != nil {
			return r, fmt.Errorf("invalid memory: %w", err)
		}
		r.MemoryMax = memBytes
	}

	// Swap
	if rc.Swap == "-1" {
		r.SwapMax = -1
	} else if rc.Swap != "" {
		swapBytes, err := config.ParseMemorySize(rc.Swap)
		if err != nil {
			return r, fmt.Errorf("invalid swap: %w", err)
		}
		r.SwapMax = swapBytes
	}

	// Pids
	if rc.Pids > 0 {
		r.PidsMax = int64(rc.Pids)
	}

	return r, nil
}
