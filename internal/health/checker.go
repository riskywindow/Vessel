// Package health provides continuous health monitoring and auto-restart for containers.
package health

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/vessel/vessel/internal/runtime"
	"github.com/vessel/vessel/internal/store"
)

// ExecuteCheck performs a single health check against a container.
// It dispatches to the appropriate check type (HTTP, TCP, or command).
func ExecuteCheck(ctx context.Context, rt runtime.Runtime, containerID string, check store.HealthCheck) error {
	checkCtx, cancel := context.WithTimeout(ctx, check.Timeout)
	defer cancel()

	switch check.Type {
	case store.HealthCheckHTTP:
		return ExecuteHTTPCheck(checkCtx, check.Target)
	case store.HealthCheckTCP:
		return ExecuteTCPCheck(checkCtx, check.Target)
	case store.HealthCheckCommand:
		return ExecuteCommandCheck(checkCtx, rt, containerID, check.Target)
	default:
		return fmt.Errorf("unknown health check type: %s", check.Type)
	}
}

// ExecuteHTTPCheck performs an HTTP GET and expects a 2xx response.
func ExecuteHTTPCheck(ctx context.Context, target string) error {
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

// ExecuteTCPCheck attempts to connect to a TCP address.
func ExecuteTCPCheck(ctx context.Context, target string) error {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", target)
	if err != nil {
		return fmt.Errorf("TCP check failed: %w", err)
	}
	conn.Close()
	return nil
}

// ExecuteCommandCheck runs a command inside the container and checks exit code.
func ExecuteCommandCheck(ctx context.Context, rt runtime.Runtime, containerID string, command string) error {
	err := rt.ExecInContainer(ctx, containerID, []string{"sh", "-c", command})
	if err != nil {
		return fmt.Errorf("command check failed: %w", err)
	}
	return nil
}
