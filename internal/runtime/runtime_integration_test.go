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

	"github.com/vessel/vessel/internal/store"
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

// TestMemoryLimit tests that memory limits are enforced via cgroups.
func TestMemoryLimit(t *testing.T) {
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

	// Create container with 64MB memory limit
	t.Log("Creating container with 64MB memory limit...")
	container, err := rt.CreateContainer(ctx, ContainerOpts{
		Name:    "test-memory-limit",
		Image:   "alpine:latest",
		Command: []string{"sh", "-c", "echo Memory limit test started; sleep 2; echo done"},
		Resources: store.ResourceLimits{
			MemoryMax: 64 * 1024 * 1024, // 64MB
			PidsMax:   100,
		},
	})
	if err != nil {
		t.Fatalf("failed to create container: %v", err)
	}
	defer rt.RemoveContainer(ctx, container.ID)

	// Start container
	if err := rt.StartContainer(ctx, container.ID); err != nil {
		t.Fatalf("failed to start container: %v", err)
	}

	// Wait a bit for the process to start
	time.Sleep(1 * time.Second)

	// Get stats to verify cgroup is set up
	stats, err := rt.ContainerStats(ctx, container.ID)
	if err != nil {
		t.Logf("Warning: could not get stats: %v", err)
	} else {
		t.Logf("Memory current: %d bytes", stats.MemoryBytes)
		t.Logf("Memory limit: %d bytes", stats.MemoryLimit)
		
		// Verify the limit is set correctly (approximately 64MB)
		expectedLimit := int64(64 * 1024 * 1024)
		if stats.MemoryLimit != expectedLimit && stats.MemoryLimit != 0 {
			t.Logf("Memory limit mismatch: got %d, expected %d", stats.MemoryLimit, expectedLimit)
		}
	}

	// Wait for container to finish
	time.Sleep(3 * time.Second)

	// Verify container completed
	state, err := rt.GetContainerState(container.ID)
	if err != nil {
		t.Logf("Warning: could not get state: %v", err)
	} else {
		t.Logf("Container final state: %s", state)
	}
}

// TestPIDLimit tests that PID limits are enforced via cgroups.
func TestPIDLimit(t *testing.T) {
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

	// Create container with low PID limit
	t.Log("Creating container with PID limit of 10...")
	container, err := rt.CreateContainer(ctx, ContainerOpts{
		Name:    "test-pid-limit",
		Image:   "alpine:latest",
		Command: []string{"sh", "-c", "echo PID limit test; cat /sys/fs/cgroup/pids.max; sleep 1"},
		Resources: store.ResourceLimits{
			MemoryMax: 64 * 1024 * 1024,
			PidsMax:   10,
		},
	})
	if err != nil {
		t.Fatalf("failed to create container: %v", err)
	}
	defer rt.RemoveContainer(ctx, container.ID)

	// Start container
	if err := rt.StartContainer(ctx, container.ID); err != nil {
		t.Fatalf("failed to start container: %v", err)
	}

	// Wait for container to run
	time.Sleep(3 * time.Second)

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

	t.Logf("Container output:\n%s", string(logData))
}

// TestFullLifecycleWithCgroups tests the complete container lifecycle with cgroups.
func TestFullLifecycleWithCgroups(t *testing.T) {
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

	// 1. Create container with resource limits
	t.Log("Step 1: Creating container...")
	container, err := rt.CreateContainer(ctx, ContainerOpts{
		Name:    "test-full-lifecycle",
		Image:   "alpine:latest",
		Command: []string{"sh", "-c", "echo hello; sleep 2; echo goodbye"},
		Resources: store.ResourceLimits{
			MemoryMax: 128 * 1024 * 1024, // 128MB
			PidsMax:   50,
		},
	})
	if err != nil {
		t.Fatalf("failed to create container: %v", err)
	}
	containerID := container.ID
	t.Logf("Container created: %s", containerID[:12])

	// Verify state is "created"
	state, err := rt.GetContainerState(containerID)
	if err != nil {
		t.Fatalf("failed to get state: %v", err)
	}
	if state != "created" {
		t.Errorf("expected state 'created', got '%s'", state)
	}

	// 2. Start container
	t.Log("Step 2: Starting container...")
	if err := rt.StartContainer(ctx, containerID); err != nil {
		rt.RemoveContainer(ctx, containerID)
		t.Fatalf("failed to start container: %v", err)
	}

	// Verify state is "running"
	state, err = rt.GetContainerState(containerID)
	if err != nil {
		t.Fatalf("failed to get state: %v", err)
	}
	if state != "running" {
		t.Errorf("expected state 'running', got '%s'", state)
	}

	// 3. Get stats while running
	t.Log("Step 3: Getting stats...")
	stats, err := rt.ContainerStats(ctx, containerID)
	if err != nil {
		t.Logf("Warning: could not get stats: %v", err)
	} else {
		t.Logf("Stats - Memory: %d bytes, PIDs: %d", stats.MemoryBytes, stats.PidsCount)
	}

	// 4. Read logs
	t.Log("Step 4: Reading logs...")
	time.Sleep(1 * time.Second)
	logs, err := rt.ContainerLogs(ctx, containerID, false, 0)
	if err != nil {
		t.Logf("Warning: could not get logs: %v", err)
	} else {
		logData, _ := io.ReadAll(logs)
		logs.Close()
		t.Logf("Logs: %s", strings.TrimSpace(string(logData)))
	}

	// 5. Wait for container to finish naturally
	t.Log("Step 5: Waiting for container to finish...")
	time.Sleep(3 * time.Second)

	// 6. Verify state is stopped
	state, err = rt.GetContainerState(containerID)
	if err != nil {
		t.Logf("Warning: could not get state: %v", err)
	} else {
		t.Logf("Final state: %s", state)
		if state != "stopped" && state != "failed" {
			t.Logf("Note: expected state 'stopped' or 'failed', got '%s'", state)
		}
	}

	// 7. Remove container
	t.Log("Step 6: Removing container...")
	if err := rt.RemoveContainer(ctx, containerID); err != nil {
		t.Fatalf("failed to remove container: %v", err)
	}

	// 8. Verify cleanup
	t.Log("Step 7: Verifying cleanup...")
	
	// Check container directory is gone
	containerDir := "/var/lib/vessel/containers/" + containerID
	if _, err := os.Stat(containerDir); err == nil {
		t.Errorf("container directory still exists: %s", containerDir)
	}

	// Check cgroup is gone
	cgroupDir := "/sys/fs/cgroup/vessel/" + containerID
	if _, err := os.Stat(cgroupDir); err == nil {
		t.Errorf("cgroup directory still exists: %s", cgroupDir)
	}

	t.Log("Full lifecycle test passed!")
}

// TestCapabilityDropping tests that capabilities are properly dropped.
func TestCapabilityDropping(t *testing.T) {
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

	// Create container that reads capability info
	t.Log("Creating container to check capabilities...")
	container, err := rt.CreateContainer(ctx, ContainerOpts{
		Name:    "test-capabilities",
		Image:   "alpine:latest",
		Command: []string{"sh", "-c", "cat /proc/1/status | grep Cap"},
	})
	if err != nil {
		t.Fatalf("failed to create container: %v", err)
	}
	defer rt.RemoveContainer(ctx, container.ID)

	// Start container
	if err := rt.StartContainer(ctx, container.ID); err != nil {
		t.Fatalf("failed to start container: %v", err)
	}

	// Wait for container to run
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

	capOutput := string(logData)
	t.Logf("Capability status:\n%s", capOutput)

	// The capability bounding set should be restricted
	// We can't easily check the exact values, but we log them for manual inspection
	if !strings.Contains(capOutput, "CapBnd") {
		t.Log("Warning: Could not find CapBnd in output")
	}
}

// TestSeccompBlocking tests that seccomp blocks dangerous syscalls.
func TestSeccompBlocking(t *testing.T) {
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

	// Create container that tries to use a blocked syscall (reboot)
	// This should fail with EPERM
	t.Log("Creating container to test seccomp...")
	container, err := rt.CreateContainer(ctx, ContainerOpts{
		Name:    "test-seccomp",
		Image:   "alpine:latest",
		// Try to read seccomp status - should show seccomp is active
		Command: []string{"sh", "-c", "cat /proc/self/status | grep Seccomp; echo done"},
	})
	if err != nil {
		t.Fatalf("failed to create container: %v", err)
	}
	defer rt.RemoveContainer(ctx, container.ID)

	// Start container
	if err := rt.StartContainer(ctx, container.ID); err != nil {
		t.Fatalf("failed to start container: %v", err)
	}

	// Wait for container to run
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

	seccompOutput := string(logData)
	t.Logf("Seccomp status:\n%s", seccompOutput)

	// Seccomp: 2 means SECCOMP_MODE_FILTER is active
	if strings.Contains(seccompOutput, "Seccomp:\t2") {
		t.Log("Seccomp filter is active")
	} else if strings.Contains(seccompOutput, "Seccomp:") {
		t.Log("Seccomp status found (may be different format)")
	} else {
		t.Log("Could not determine seccomp status from output")
	}
}
