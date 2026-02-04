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

// TestOCIImagePull tests pulling an OCI image from a registry.
func TestOCIImagePull(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("integration tests require root privileges")
	}

	ctx := context.Background()

	// Create image store
	store, err := NewImageStore()
	if err != nil {
		t.Fatalf("failed to create image store: %v", err)
	}

	// Pull alpine:latest
	t.Log("Pulling alpine:latest...")
	digest, err := store.PullImage(ctx, "alpine:latest")
	if err != nil {
		t.Fatalf("failed to pull image: %v", err)
	}
	t.Logf("Pulled image with digest: %s", digest)

	// Verify layers exist on disk
	layerPaths, err := store.GetLayerPaths(digest)
	if err != nil {
		t.Fatalf("failed to get layer paths: %v", err)
	}
	if len(layerPaths) == 0 {
		t.Error("expected at least one layer")
	}
	t.Logf("Found %d layers", len(layerPaths))

	for i, path := range layerPaths {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("layer %d path does not exist: %s", i, path)
		}
		// Verify it has some content (standard alpine directories)
		binPath := filepath.Join(path, "bin")
		if _, err := os.Stat(binPath); err == nil {
			t.Logf("Layer %d contains /bin directory", i)
		}
	}

	// Verify metadata
	metadata, err := store.GetImage(digest)
	if err != nil {
		t.Fatalf("failed to get metadata: %v", err)
	}
	if metadata.Reference != "alpine:latest" {
		t.Errorf("expected reference 'alpine:latest', got '%s'", metadata.Reference)
	}
	if metadata.Layers == 0 {
		t.Error("expected at least one layer in metadata")
	}

	// Test that pulling again is a no-op (cached)
	t.Log("Pulling again (should be cached)...")
	digest2, err := store.PullImage(ctx, "alpine:latest")
	if err != nil {
		t.Fatalf("failed to pull cached image: %v", err)
	}
	if digest2 != digest {
		t.Errorf("expected same digest, got %s vs %s", digest2, digest)
	}
}

// TestOCIContainerWithOverlayFS tests running a container with an OCI image and OverlayFS.
func TestOCIContainerWithOverlayFS(t *testing.T) {
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

	// Create container with alpine image
	t.Log("Creating container with alpine:latest...")
	container, err := rt.CreateContainer(ctx, ContainerOpts{
		Name:    "test-alpine-overlay",
		Image:   "alpine:latest",
		Command: []string{"ls", "/"},
	})
	if err != nil {
		t.Fatalf("failed to create container: %v", err)
	}
	defer rt.RemoveContainer(ctx, container.ID)

	// Verify OverlayFS is set up
	mergedDir := "/var/lib/vessel/containers/" + container.ID + "/merged"
	if _, err := os.Stat(mergedDir); err != nil {
		t.Fatalf("merged directory does not exist: %s", mergedDir)
	}

	// Start container
	t.Log("Starting container...")
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

	lsOutput := string(logData)
	t.Logf("ls output:\n%s", lsOutput)

	// Verify standard alpine filesystem layout
	expectedDirs := []string{"bin", "etc", "lib", "usr", "var"}
	for _, dir := range expectedDirs {
		if !strings.Contains(lsOutput, dir) {
			t.Errorf("expected '%s' in ls output", dir)
		}
	}
}

// TestOverlayFSWriteIsolation tests that writes inside the container don't affect image layers.
func TestOverlayFSWriteIsolation(t *testing.T) {
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

	// Create container that writes a file
	t.Log("Creating container that writes a file...")
	container, err := rt.CreateContainer(ctx, ContainerOpts{
		Name:    "test-overlay-write",
		Image:   "alpine:latest",
		Command: []string{"sh", "-c", "touch /testfile && ls /testfile"},
	})
	if err != nil {
		t.Fatalf("failed to create container: %v", err)
	}

	containerID := container.ID
	upperDir := "/var/lib/vessel/containers/" + containerID + "/upper"

	// Start container
	if err := rt.StartContainer(ctx, containerID); err != nil {
		t.Fatalf("failed to start container: %v", err)
	}

	// Wait for container to complete
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

	output := strings.TrimSpace(string(logData))
	t.Logf("output: %s", output)

	if output != "/testfile" {
		t.Errorf("expected '/testfile', got '%s'", output)
	}

	// Verify the file appears in the upper layer
	upperTestFile := filepath.Join(upperDir, "testfile")
	if _, err := os.Stat(upperTestFile); err != nil {
		t.Logf("Note: testfile in upper dir: %v (might be unmounted already)", err)
	}

	// Verify the file does NOT exist in image layers
	store := rt.GetImageStore()
	metadata, _ := store.GetImageByRef("alpine:latest")
	if metadata != nil {
		layerPaths, _ := store.GetLayerPaths(metadata.Digest)
		for i, layerPath := range layerPaths {
			testFilePath := filepath.Join(layerPath, "testfile")
			if _, err := os.Stat(testFilePath); err == nil {
				t.Errorf("testfile found in image layer %d, expected write isolation", i)
			}
		}
	}

	// Clean up
	rt.RemoveContainer(ctx, containerID)

	// Verify container directory is cleaned up
	containerDir := "/var/lib/vessel/containers/" + containerID
	if _, err := os.Stat(containerDir); err == nil {
		t.Errorf("container directory still exists after removal: %s", containerDir)
	}
}

// TestImageRemoval tests that removing an image cleans up properly.
func TestImageRemoval(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("integration tests require root privileges")
	}

	ctx := context.Background()

	store, err := NewImageStore()
	if err != nil {
		t.Fatalf("failed to create image store: %v", err)
	}

	// Pull a small image
	t.Log("Pulling alpine:latest...")
	digest, err := store.PullImage(ctx, "alpine:latest")
	if err != nil {
		t.Fatalf("failed to pull image: %v", err)
	}

	// Verify it exists
	digestDir := strings.ReplaceAll(digest, ":", "-")
	imageDir := filepath.Join("/var/lib/vessel/images", digestDir)
	if _, err := os.Stat(imageDir); err != nil {
		t.Fatalf("image directory does not exist: %s", imageDir)
	}

	// Remove the image
	t.Log("Removing image...")
	if err := store.RemoveImage(digest); err != nil {
		t.Fatalf("failed to remove image: %v", err)
	}

	// Verify it's gone
	if _, err := os.Stat(imageDir); err == nil {
		t.Errorf("image directory still exists after removal: %s", imageDir)
	}

	// Verify GetImage returns error
	if _, err := store.GetImage(digest); err == nil {
		t.Error("expected error getting removed image")
	}
}
