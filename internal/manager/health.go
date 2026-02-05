package manager

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/vessel/vessel/internal/config"
	"github.com/vessel/vessel/internal/runtime"
	"github.com/vessel/vessel/internal/store"
)

// Default health check parameters used when no health check is configured.
const (
	defaultHealthTimeout  = 30 * time.Second
	defaultHealthInterval = 2 * time.Second
	defaultHealthRetries  = 3
)

// waitForHealthy polls the health check for a container until it reports healthy
// or the timeout is reached. Returns nil on success, error on timeout/failure.
func waitForHealthy(ctx context.Context, rt runtime.Runtime, container *store.Container, hcCfg *config.HealthCheckConfig, timeout time.Duration, logger *slog.Logger) error {
	if timeout <= 0 {
		timeout = defaultHealthTimeout
	}

	check := buildHealthCheck(hcCfg, container)
	interval := check.Interval
	if interval <= 0 {
		interval = defaultHealthInterval
	}

	deadline := time.After(timeout)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	requiredSuccess := 1
	consecutiveSuccess := 0
	attempt := 0

	// Check immediately before waiting for first tick
	attempt++
	if err := executeHealthCheck(ctx, rt, container, check, logger); err == nil {
		logger.Info("health check passed", "container", container.ID[:12], "attempt", attempt)
		return nil
	} else {
		logger.Debug("health check failed", "container", container.ID[:12], "attempt", attempt, "error", err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline:
			return fmt.Errorf("health check timeout after %s (%d attempts)", timeout, attempt)
		case <-ticker.C:
			attempt++
			err := executeHealthCheck(ctx, rt, container, check, logger)
			if err == nil {
				consecutiveSuccess++
				if consecutiveSuccess >= requiredSuccess {
					logger.Info("health check passed", "container", container.ID[:12], "attempt", attempt)
					return nil
				}
			} else {
				consecutiveSuccess = 0
				logger.Debug("health check failed", "container", container.ID[:12], "attempt", attempt, "error", err)
			}
		}
	}
}

// buildHealthCheck converts a config.HealthCheckConfig into a store.HealthCheck.
// If hcCfg is nil, returns a default health check (process alive check).
func buildHealthCheck(hcCfg *config.HealthCheckConfig, container *store.Container) store.HealthCheck {
	if hcCfg == nil {
		// Default: just check that the process is still running
		return store.HealthCheck{
			Type:     store.HealthCheckCommand,
			Target:   "true", // Always succeeds — just verifies exec works
			Interval: defaultHealthInterval,
			Timeout:  5 * time.Second,
			Retries:  defaultHealthRetries,
		}
	}

	hc := store.HealthCheck{
		Interval: hcCfg.Interval.Duration,
		Timeout:  hcCfg.Timeout.Duration,
		Retries:  hcCfg.Retries,
	}

	if hc.Interval <= 0 {
		hc.Interval = defaultHealthInterval
	}
	if hc.Timeout <= 0 {
		hc.Timeout = 5 * time.Second
	}
	if hc.Retries <= 0 {
		hc.Retries = defaultHealthRetries
	}

	switch hcCfg.Type {
	case "http":
		hc.Type = store.HealthCheckHTTP
		hc.Target = fmt.Sprintf("http://%s:%d%s", containerAddr(container), hcCfg.Port, hcCfg.Path)
	case "tcp":
		hc.Type = store.HealthCheckTCP
		hc.Target = fmt.Sprintf("%s:%d", containerAddr(container), hcCfg.Port)
	case "command":
		hc.Type = store.HealthCheckCommand
		if len(hcCfg.Command) > 0 {
			hc.Target = hcCfg.Command[0]
		}
	default:
		// Fall back to command check
		hc.Type = store.HealthCheckCommand
		hc.Target = "true"
	}

	return hc
}

// containerAddr returns the address to use for health checks on a container.
// Uses the container IP if available, otherwise falls back to localhost.
func containerAddr(c *store.Container) string {
	if c.IP != "" {
		return c.IP
	}
	return "127.0.0.1"
}

// executeHealthCheck performs a single health check against a container.
func executeHealthCheck(ctx context.Context, rt runtime.Runtime, container *store.Container, check store.HealthCheck, logger *slog.Logger) error {
	checkCtx, cancel := context.WithTimeout(ctx, check.Timeout)
	defer cancel()

	switch check.Type {
	case store.HealthCheckHTTP:
		return executeHTTPCheck(checkCtx, check.Target)
	case store.HealthCheckTCP:
		return executeTCPCheck(checkCtx, check.Target)
	case store.HealthCheckCommand:
		return executeCommandCheck(checkCtx, rt, container.ID, check.Target)
	default:
		return fmt.Errorf("unknown health check type: %s", check.Type)
	}
}

// executeHTTPCheck performs an HTTP GET and expects a 2xx response.
func executeHTTPCheck(ctx context.Context, target string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP check returned status %d", resp.StatusCode)
	}

	return nil
}

// executeTCPCheck attempts to connect to a TCP address.
func executeTCPCheck(ctx context.Context, target string) error {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", target)
	if err != nil {
		return fmt.Errorf("TCP check failed: %w", err)
	}
	conn.Close()
	return nil
}

// executeCommandCheck runs a command inside the container and checks exit code.
func executeCommandCheck(ctx context.Context, rt runtime.Runtime, containerID string, command string) error {
	err := rt.ExecInContainer(ctx, containerID, []string{"sh", "-c", command})
	if err != nil {
		return fmt.Errorf("command check failed: %w", err)
	}
	return nil
}
