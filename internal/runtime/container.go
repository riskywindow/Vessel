// Package runtime implements the container runtime using Linux primitives.
package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/vessel/vessel/internal/paths"
	"github.com/vessel/vessel/internal/store"
	"golang.org/x/sys/unix"
)

// ContainerOpts contains options for creating a container.
type ContainerOpts struct {
	Name      string              // Human-readable name
	Image     string              // Image reference (for now, "busybox" or path to rootfs)
	Command   []string            // Command to run
	Env       []string            // Environment variables
	WorkDir   string              // Working directory
	Resources store.ResourceLimits // Resource limits
}

// Runtime is the interface for container runtime operations.
type Runtime interface {
	CreateContainer(ctx context.Context, opts ContainerOpts) (*store.Container, error)
	StartContainer(ctx context.Context, containerID string) error
	StopContainer(ctx context.Context, containerID string, gracePeriod time.Duration) error
	RemoveContainer(ctx context.Context, containerID string) error
	ContainerLogs(ctx context.Context, containerID string, follow bool, tail int) (io.ReadCloser, error)
	PullImage(ctx context.Context, ref string) error
	ExecInContainer(ctx context.Context, containerID string, cmd []string) error
	ContainerStats(ctx context.Context, containerID string) (*store.MetricPoint, error)
}

// LinuxRuntime implements the Runtime interface using Linux primitives.
type LinuxRuntime struct {
	mu         sync.RWMutex
	containers map[string]*containerState
}

// containerState holds the runtime state of a container.
type containerState struct {
	config    *containerConfig
	process   *os.Process
	waitCh    chan struct{} // closed when process exits
	exitCode  int
	exitError error
	started   bool // true if container has been started at least once
}

// containerConfig is persisted to disk.
type containerConfig struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Image     string            `json:"image"`
	Command   []string          `json:"command"`
	Env       []string          `json:"env"`
	WorkDir   string            `json:"workdir"`
	Hostname  string            `json:"hostname"`
	RootFS    string            `json:"rootfs"`
	CreatedAt time.Time         `json:"created_at"`
	Resources store.ResourceLimits `json:"resources"`
}

// NewLinuxRuntime creates a new Linux container runtime.
func NewLinuxRuntime() (*LinuxRuntime, error) {
	// Ensure base directories exist
	for _, dir := range []string{paths.ContainersDir, paths.ImagesDir, paths.RunDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create %s: %w", dir, err)
		}
	}

	return &LinuxRuntime{
		containers: make(map[string]*containerState),
	}, nil
}

// CreateContainer creates a new container without starting it.
func (r *LinuxRuntime) CreateContainer(ctx context.Context, opts ContainerOpts) (*store.Container, error) {
	// Generate container ID
	containerID := generateContainerID()

	// Create container directory structure
	containerDir := paths.ContainerDir(containerID)
	logsDir := paths.ContainerLogsDir(containerID)
	rootfsDir := paths.ContainerMergedDir(containerID)

	for _, dir := range []string{containerDir, logsDir, rootfsDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create %s: %w", dir, err)
		}
	}

	// Create the rootfs
	// For now, we only support "busybox" image which creates a minimal rootfs
	if opts.Image == "busybox" || opts.Image == "" {
		if err := CreateMinimalRootfs(rootfsDir); err != nil {
			os.RemoveAll(containerDir)
			return nil, fmt.Errorf("failed to create rootfs: %w", err)
		}
	} else {
		// Treat image as a path to an existing rootfs
		// In the future, this will pull OCI images
		if _, err := os.Stat(opts.Image); err != nil {
			os.RemoveAll(containerDir)
			return nil, fmt.Errorf("image not found: %s", opts.Image)
		}
		// Copy the rootfs
		if err := copyDir(opts.Image, rootfsDir); err != nil {
			os.RemoveAll(containerDir)
			return nil, fmt.Errorf("failed to copy rootfs: %w", err)
		}
	}

	// Default command if not specified
	if len(opts.Command) == 0 {
		opts.Command = []string{"/bin/sh"}
	}

	// Default environment
	if len(opts.Env) == 0 {
		opts.Env = []string{
			"PATH=/bin:/sbin:/usr/bin:/usr/sbin:/usr/local/bin",
			"HOME=/root",
			"TERM=xterm",
		}
	}

	// Generate hostname from container ID
	hostname := containerID[:12]
	if opts.Name != "" {
		hostname = opts.Name
	}

	// Create container config
	config := &containerConfig{
		ID:        containerID,
		Name:      opts.Name,
		Image:     opts.Image,
		Command:   opts.Command,
		Env:       opts.Env,
		WorkDir:   opts.WorkDir,
		Hostname:  hostname,
		RootFS:    rootfsDir,
		CreatedAt: time.Now(),
		Resources: opts.Resources,
	}

	// Save config to disk
	configPath := paths.ContainerConfigPath(containerID)
	configData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		os.RemoveAll(containerDir)
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}
	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		os.RemoveAll(containerDir)
		return nil, fmt.Errorf("failed to write config: %w", err)
	}

	// Store in memory
	r.mu.Lock()
	r.containers[containerID] = &containerState{
		config: config,
		waitCh: make(chan struct{}),
	}
	r.mu.Unlock()

	// Return container info
	return &store.Container{
		ID:        containerID,
		Image:     opts.Image,
		State:     store.ContainerStateCreated,
		Resources: opts.Resources,
		CreatedAt: config.CreatedAt,
	}, nil
}

// StartContainer starts a previously created container.
func (r *LinuxRuntime) StartContainer(ctx context.Context, containerID string) error {
	r.mu.Lock()
	state, ok := r.containers[containerID]
	if !ok {
		// Try to load from disk
		config, err := r.loadContainerConfig(containerID)
		if err != nil {
			r.mu.Unlock()
			return fmt.Errorf("container not found: %s", containerID)
		}
		state = &containerState{
			config: config,
			waitCh: make(chan struct{}),
		}
		r.containers[containerID] = state
	}
	r.mu.Unlock()

	if state.process != nil {
		return fmt.Errorf("container already running")
	}

	// Open log files
	stdoutPath := paths.ContainerStdoutLog(containerID)
	stderrPath := paths.ContainerStderrLog(containerID)

	stdout, err := os.OpenFile(stdoutPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open stdout log: %w", err)
	}

	stderr, err := os.OpenFile(stderrPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		stdout.Close()
		return fmt.Errorf("failed to open stderr log: %w", err)
	}

	// Create namespace config
	nsConfig := NamespaceConfig{
		RootFS:   state.config.RootFS,
		Command:  state.config.Command,
		Env:      state.config.Env,
		Hostname: state.config.Hostname,
		WorkDir:  state.config.WorkDir,
	}

	// Start the process in new namespaces
	process, err := CreateNamespacedProcess(nsConfig, stdout, stderr)
	if err != nil {
		stdout.Close()
		stderr.Close()
		return fmt.Errorf("failed to create namespaced process: %w", err)
	}

	r.mu.Lock()
	state.process = process
	state.started = true
	state.waitCh = make(chan struct{})
	r.mu.Unlock()

	// Start goroutine to wait for process exit
	go r.waitForExit(containerID, process, stdout, stderr)

	return nil
}

// StopContainer stops a running container.
func (r *LinuxRuntime) StopContainer(ctx context.Context, containerID string, gracePeriod time.Duration) error {
	r.mu.RLock()
	state, ok := r.containers[containerID]
	r.mu.RUnlock()

	if !ok {
		return fmt.Errorf("container not found: %s", containerID)
	}

	if state.process == nil {
		return nil // Already stopped
	}

	// Send SIGTERM
	if err := state.process.Signal(unix.SIGTERM); err != nil {
		// Process might already be dead
		if err == os.ErrProcessDone {
			return nil
		}
	}

	// Wait for graceful shutdown or timeout
	select {
	case <-state.waitCh:
		return nil
	case <-time.After(gracePeriod):
		// Force kill with SIGKILL
		state.process.Signal(unix.SIGKILL)
		<-state.waitCh
		return nil
	case <-ctx.Done():
		// Context cancelled, force kill
		state.process.Signal(unix.SIGKILL)
		return ctx.Err()
	}
}

// RemoveContainer removes a container and its resources.
func (r *LinuxRuntime) RemoveContainer(ctx context.Context, containerID string) error {
	r.mu.Lock()
	state, ok := r.containers[containerID]
	if ok {
		// Ensure container is stopped
		if state.process != nil {
			r.mu.Unlock()
			if err := r.StopContainer(ctx, containerID, 10*time.Second); err != nil {
				return err
			}
			r.mu.Lock()
		}
		delete(r.containers, containerID)
	}
	r.mu.Unlock()

	// Remove container directory
	containerDir := paths.ContainerDir(containerID)
	if err := os.RemoveAll(containerDir); err != nil {
		return fmt.Errorf("failed to remove container directory: %w", err)
	}

	return nil
}

// ContainerLogs returns a reader for container logs.
func (r *LinuxRuntime) ContainerLogs(ctx context.Context, containerID string, follow bool, tail int) (io.ReadCloser, error) {
	stdoutPath := paths.ContainerStdoutLog(containerID)

	// Check if log file exists
	if _, err := os.Stat(stdoutPath); err != nil {
		return nil, fmt.Errorf("logs not found for container: %s", containerID)
	}

	if follow {
		// For following logs, we return a file that will be read continuously
		file, err := os.Open(stdoutPath)
		if err != nil {
			return nil, err
		}
		return &followReader{
			file:        file,
			containerID: containerID,
			ctx:         ctx,
			runtime:     r,
		}, nil
	}

	// For non-follow mode, just return the file
	return os.Open(stdoutPath)
}

// followReader implements io.ReadCloser for following logs.
type followReader struct {
	file        *os.File
	containerID string
	ctx         context.Context
	runtime     *LinuxRuntime
}

func (f *followReader) Read(p []byte) (int, error) {
	for {
		n, err := f.file.Read(p)
		if n > 0 {
			return n, nil
		}
		if err != nil && err != io.EOF {
			return 0, err
		}

		// Check if context is cancelled
		select {
		case <-f.ctx.Done():
			return 0, f.ctx.Err()
		default:
		}

		// Check if container is still running
		f.runtime.mu.RLock()
		state, ok := f.runtime.containers[f.containerID]
		f.runtime.mu.RUnlock()

		if !ok || state.process == nil {
			// Container stopped, check for remaining data
			n, readErr := f.file.Read(p)
			if n > 0 {
				return n, readErr
			}
			return 0, io.EOF
		}

		// Wait a bit before retrying
		time.Sleep(100 * time.Millisecond)
	}
}

func (f *followReader) Close() error {
	return f.file.Close()
}

// PullImage pulls an OCI image from a registry.
// TODO: Implement in next session.
func (r *LinuxRuntime) PullImage(ctx context.Context, ref string) error {
	return fmt.Errorf("not yet implemented")
}

// ExecInContainer executes a command in a running container.
// TODO: Implement in future session.
func (r *LinuxRuntime) ExecInContainer(ctx context.Context, containerID string, cmd []string) error {
	return fmt.Errorf("not yet implemented")
}

// ContainerStats returns resource usage statistics for a container.
// TODO: Implement in future session.
func (r *LinuxRuntime) ContainerStats(ctx context.Context, containerID string) (*store.MetricPoint, error) {
	return nil, fmt.Errorf("not yet implemented")
}

// waitForExit waits for a container process to exit and updates state.
func (r *LinuxRuntime) waitForExit(containerID string, process *os.Process, stdout, stderr *os.File) {
	defer stdout.Close()
	defer stderr.Close()

	// Wait for process to exit
	state, err := process.Wait()

	r.mu.Lock()
	if cs, ok := r.containers[containerID]; ok {
		if state != nil {
			cs.exitCode = state.ExitCode()
		}
		cs.exitError = err
		cs.process = nil
		close(cs.waitCh)
	}
	r.mu.Unlock()
}

// loadContainerConfig loads container configuration from disk.
func (r *LinuxRuntime) loadContainerConfig(containerID string) (*containerConfig, error) {
	configPath := paths.ContainerConfigPath(containerID)
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var config containerConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// GetContainerState returns the current state of a container.
func (r *LinuxRuntime) GetContainerState(containerID string) (store.ContainerState, error) {
	r.mu.RLock()
	state, ok := r.containers[containerID]
	r.mu.RUnlock()

	if !ok {
		// Try to load from disk
		configPath := paths.ContainerConfigPath(containerID)
		if _, err := os.Stat(configPath); err == nil {
			return store.ContainerStateCreated, nil
		}
		return "", fmt.Errorf("container not found: %s", containerID)
	}

	if state.process != nil {
		return store.ContainerStateRunning, nil
	}

	if !state.started {
		return store.ContainerStateCreated, nil
	}

	return store.ContainerStateStopped, nil
}

// GetContainerPID returns the PID of a running container.
func (r *LinuxRuntime) GetContainerPID(containerID string) (int, error) {
	r.mu.RLock()
	state, ok := r.containers[containerID]
	r.mu.RUnlock()

	if !ok || state.process == nil {
		return 0, fmt.Errorf("container not running: %s", containerID)
	}

	return state.process.Pid, nil
}

// generateContainerID generates a unique container ID.
func generateContainerID() string {
	// Use a combination of timestamp and random bytes
	now := time.Now().UnixNano()
	random := make([]byte, 8)
	// Read from /dev/urandom for randomness
	f, _ := os.Open("/dev/urandom")
	if f != nil {
		f.Read(random)
		f.Close()
	}
	return fmt.Sprintf("%x%x", now, random)
}

// copyDir recursively copies a directory.
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		// Handle symlinks
		if info.Mode()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(link, dstPath)
		}

		// Copy regular file
		return copyFile(path, dstPath)
	})
}
