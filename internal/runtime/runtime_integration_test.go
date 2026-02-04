//go:build integration

package runtime

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func init() {
	// Build the vessel binary for testing if not already set
	if VesselBinaryPath == "" {
		// Find the project root by looking for go.mod
		wd, _ := os.Getwd()
		for dir := wd; dir != "/"; dir = filepath.Dir(dir) {
			if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
				binaryPath := filepath.Join(dir, "bin", "vessel")
				// Build the binary if it doesn't exist or rebuild to be sure
				cmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/vessel")
				cmd.Dir = dir
				if err := cmd.Run(); err == nil {
					VesselBinaryPath = binaryPath
				}
				break
			}
		}
	}
}

// TestContainerLifecycle tests the full container lifecycle.
func TestContainerLifecycle(t *testing.T) {
	// Skip if not running as root
	if os.Getuid() != 0 {
		t.Skip("integration tests require root privileges")
	}

	if VesselBinaryPath == "" {
		t.Skip("vessel binary not found")
	}
	t.Logf("Using vessel binary: %s", VesselBinaryPath)

	ctx := context.Background()

	// Create runtime
	rt, err := NewLinuxRuntime()
	if err != nil {
		t.Fatalf("failed to create runtime: %v", err)
	}

	// Create container
	container, err := rt.CreateContainer(ctx, ContainerOpts{
		Name:    "test-lifecycle",
		Image:   "busybox",
		Command: []string{"sh", "-c", "echo hello-from-vessel && sleep 1 && echo goodbye"},
	})
	if err != nil {
		t.Fatalf("failed to create container: %v", err)
	}

	containerID := container.ID
	t.Logf("created container: %s", containerID)

	// Verify container was created
	state, err := rt.GetContainerState(containerID)
	if err != nil {
		t.Fatalf("failed to get container state: %v", err)
	}
	if state != "created" {
		t.Errorf("expected state 'created', got '%s'", state)
	}

	// Start container
	if err := rt.StartContainer(ctx, containerID); err != nil {
		t.Fatalf("failed to start container: %v", err)
	}
	t.Log("container started")

	// Wait for container to run
	time.Sleep(2 * time.Second)

	// Read logs
	logs, err := rt.ContainerLogs(ctx, containerID, false, 0)
	if err != nil {
		t.Fatalf("failed to get logs: %v", err)
	}
	logData, err := io.ReadAll(logs)
	logs.Close()
	if err != nil {
		t.Fatalf("failed to read logs: %v", err)
	}
	logStr := string(logData)
	t.Logf("logs: %s", logStr)

	if !strings.Contains(logStr, "hello-from-vessel") {
		t.Errorf("expected logs to contain 'hello-from-vessel', got: %s", logStr)
	}

	// Stop container
	if err := rt.StopContainer(ctx, containerID, 5*time.Second); err != nil {
		t.Fatalf("failed to stop container: %v", err)
	}
	t.Log("container stopped")

	// Remove container
	if err := rt.RemoveContainer(ctx, containerID); err != nil {
		t.Fatalf("failed to remove container: %v", err)
	}
	t.Log("container removed")

	// Verify container directory is cleaned up
	containerDir := "/var/lib/vessel/containers/" + containerID
	if _, err := os.Stat(containerDir); err == nil {
		t.Errorf("container directory still exists after removal: %s", containerDir)
	}
}

// TestHostnameIsolation verifies that the container has its own hostname.
func TestHostnameIsolation(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("integration tests require root privileges")
	}
	if VesselBinaryPath == "" {
		t.Skip("vessel binary not found")
	}

	ctx := context.Background()

	rt, err := NewLinuxRuntime()
	if err != nil {
		t.Fatalf("failed to create runtime: %v", err)
	}

	// Create container with a specific hostname
	container, err := rt.CreateContainer(ctx, ContainerOpts{
		Name:    "test-hostname",
		Image:   "busybox",
		Command: []string{"hostname"},
	})
	if err != nil {
		t.Fatalf("failed to create container: %v", err)
	}
	defer rt.RemoveContainer(ctx, container.ID)

	// Start container
	if err := rt.StartContainer(ctx, container.ID); err != nil {
		t.Fatalf("failed to start container: %v", err)
	}

	// Wait for container to complete
	time.Sleep(2 * time.Second)

	// Read logs
	logs, err := rt.ContainerLogs(ctx, container.ID, false, 0)
	if err != nil {
		t.Fatalf("failed to get logs: %v", err)
	}
	logData, err := io.ReadAll(logs)
	logs.Close()
	if err != nil {
		t.Fatalf("failed to read logs: %v", err)
	}

	containerHostname := strings.TrimSpace(string(logData))
	t.Logf("container hostname: %s", containerHostname)

	// Get host hostname
	hostHostname, err := os.Hostname()
	if err != nil {
		t.Fatalf("failed to get host hostname: %v", err)
	}
	t.Logf("host hostname: %s", hostHostname)

	// Container hostname should be different from host (it should be "test-hostname")
	if containerHostname == hostHostname {
		t.Errorf("container hostname matches host hostname, expected isolation")
	}

	if containerHostname != "test-hostname" {
		t.Errorf("expected hostname 'test-hostname', got '%s'", containerHostname)
	}
}

// TestPIDIsolation verifies that the container has its own PID namespace.
func TestPIDIsolation(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("integration tests require root privileges")
	}
	if VesselBinaryPath == "" {
		t.Skip("vessel binary not found")
	}

	ctx := context.Background()

	rt, err := NewLinuxRuntime()
	if err != nil {
		t.Fatalf("failed to create runtime: %v", err)
	}

	// Create container that runs ps
	container, err := rt.CreateContainer(ctx, ContainerOpts{
		Name:    "test-pid",
		Image:   "busybox",
		Command: []string{"ps", "aux"},
	})
	if err != nil {
		t.Fatalf("failed to create container: %v", err)
	}
	defer rt.RemoveContainer(ctx, container.ID)

	// Start container
	if err := rt.StartContainer(ctx, container.ID); err != nil {
		t.Fatalf("failed to start container: %v", err)
	}

	// Wait for container to complete
	time.Sleep(2 * time.Second)

	// Read logs
	logs, err := rt.ContainerLogs(ctx, container.ID, false, 0)
	if err != nil {
		t.Fatalf("failed to get logs: %v", err)
	}
	logData, err := io.ReadAll(logs)
	logs.Close()
	if err != nil {
		t.Fatalf("failed to read logs: %v", err)
	}

	psOutput := string(logData)
	t.Logf("ps output:\n%s", psOutput)

	// Count the number of processes (lines with PID numbers)
	lines := strings.Split(psOutput, "\n")
	processCount := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "PID") || strings.HasPrefix(line, "USER") {
			continue
		}
		processCount++
	}

	t.Logf("process count: %d", processCount)

	// Should only have 1-2 processes (init/ps), not the full host process list
	if processCount > 5 {
		t.Errorf("expected isolated PID namespace with few processes, got %d", processCount)
	}
}

// TestFilesystemIsolation verifies that the container has its own filesystem.
func TestFilesystemIsolation(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("integration tests require root privileges")
	}
	if VesselBinaryPath == "" {
		t.Skip("vessel binary not found")
	}

	ctx := context.Background()

	rt, err := NewLinuxRuntime()
	if err != nil {
		t.Fatalf("failed to create runtime: %v", err)
	}

	testFile := "/tmp/vessel-test-file-" + time.Now().Format("20060102150405")

	// Create container that creates a file
	container, err := rt.CreateContainer(ctx, ContainerOpts{
		Name:    "test-fs",
		Image:   "busybox",
		Command: []string{"sh", "-c", "echo test > " + testFile + " && cat " + testFile},
	})
	if err != nil {
		t.Fatalf("failed to create container: %v", err)
	}
	defer rt.RemoveContainer(ctx, container.ID)

	// Start container
	if err := rt.StartContainer(ctx, container.ID); err != nil {
		t.Fatalf("failed to start container: %v", err)
	}

	// Wait for container to complete
	time.Sleep(2 * time.Second)

	// Read logs to verify file was created inside container
	logs, err := rt.ContainerLogs(ctx, container.ID, false, 0)
	if err != nil {
		t.Fatalf("failed to get logs: %v", err)
	}
	logData, err := io.ReadAll(logs)
	logs.Close()
	if err != nil {
		t.Fatalf("failed to read logs: %v", err)
	}

	logStr := strings.TrimSpace(string(logData))
	t.Logf("container output: %s", logStr)

	if logStr != "test" {
		t.Errorf("expected 'test', got '%s'", logStr)
	}

	// Verify the file does NOT exist on the host
	if _, err := os.Stat(testFile); err == nil {
		t.Errorf("file %s exists on host, expected filesystem isolation", testFile)
		os.Remove(testFile)
	}
}

// TestMultipleContainers tests running multiple containers simultaneously.
func TestMultipleContainers(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("integration tests require root privileges")
	}
	if VesselBinaryPath == "" {
		t.Skip("vessel binary not found")
	}

	ctx := context.Background()

	rt, err := NewLinuxRuntime()
	if err != nil {
		t.Fatalf("failed to create runtime: %v", err)
	}

	// Create multiple containers
	containers := make([]string, 3)
	for i := 0; i < 3; i++ {
		container, err := rt.CreateContainer(ctx, ContainerOpts{
			Name:    "test-multi-" + string(rune('a'+i)),
			Image:   "busybox",
			Command: []string{"sh", "-c", "echo container-" + string(rune('a'+i)) + " && sleep 2"},
		})
		if err != nil {
			t.Fatalf("failed to create container %d: %v", i, err)
		}
		containers[i] = container.ID
	}

	// Start all containers
	for i, id := range containers {
		if err := rt.StartContainer(ctx, id); err != nil {
			t.Fatalf("failed to start container %d: %v", i, err)
		}
	}

	// Wait for them to run
	time.Sleep(3 * time.Second)

	// Verify logs from each container
	for i, id := range containers {
		logs, err := rt.ContainerLogs(ctx, id, false, 0)
		if err != nil {
			t.Errorf("failed to get logs for container %d: %v", i, err)
			continue
		}
		logData, _ := io.ReadAll(logs)
		logs.Close()

		expected := "container-" + string(rune('a'+i))
		if !strings.Contains(string(logData), expected) {
			t.Errorf("container %d logs don't contain '%s': %s", i, expected, string(logData))
		}
	}

	// Clean up
	for _, id := range containers {
		rt.RemoveContainer(ctx, id)
	}
}
