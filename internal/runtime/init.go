// Package runtime implements the container runtime using Linux primitives.
package runtime

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// NamespaceConfig contains the configuration sent from parent to child process.
type NamespaceConfig struct {
	RootFS   string   `json:"rootfs"`   // Path to the merged rootfs
	Command  []string `json:"command"`  // Command + args to execute
	Env      []string `json:"env"`      // Environment variables
	Hostname string   `json:"hostname"` // Container hostname
	WorkDir  string   `json:"workdir"`  // Working directory inside the container
}

// Device node specifications (major, minor, mode).
var deviceNodes = []struct {
	Path  string
	Major uint32
	Minor uint32
	Mode  uint32
}{
	{"/dev/null", 1, 3, unix.S_IFCHR | 0666},
	{"/dev/zero", 1, 5, unix.S_IFCHR | 0666},
	{"/dev/random", 1, 8, unix.S_IFCHR | 0444},
	{"/dev/urandom", 1, 9, unix.S_IFCHR | 0444},
	{"/dev/tty", 5, 0, unix.S_IFCHR | 0666},
}

// ContainerInit is the init process that runs inside the new namespaces.
// It is invoked when the vessel binary is re-executed with the "init" argument.
// The pipe file descriptor is passed as FD 3 (after stdin, stdout, stderr).
func ContainerInit() error {
	// Read configuration from pipe (FD 3)
	pipe := os.NewFile(3, "pipe")
	if pipe == nil {
		return fmt.Errorf("failed to get pipe file descriptor")
	}
	defer pipe.Close()

	var config NamespaceConfig
	decoder := json.NewDecoder(pipe)
	if err := decoder.Decode(&config); err != nil {
		return fmt.Errorf("failed to decode config: %w", err)
	}

	// Set the hostname
	if err := unix.Sethostname([]byte(config.Hostname)); err != nil {
		return fmt.Errorf("failed to set hostname: %w", err)
	}

	// Set up the mount namespace
	if err := setupMountNamespace(config.RootFS); err != nil {
		return fmt.Errorf("failed to setup mount namespace: %w", err)
	}

	// Perform pivot_root
	if err := pivotRoot(config.RootFS); err != nil {
		return fmt.Errorf("failed to pivot_root: %w", err)
	}

	// Set up /etc files
	if err := setupEtcFiles(config.Hostname); err != nil {
		return fmt.Errorf("failed to setup /etc files: %w", err)
	}

	// Change to working directory
	workDir := config.WorkDir
	if workDir == "" {
		workDir = "/"
	}
	if err := os.Chdir(workDir); err != nil {
		return fmt.Errorf("failed to chdir to %s: %w", workDir, err)
	}

	// Find the executable
	execPath, err := findExecutable(config.Command[0])
	if err != nil {
		return fmt.Errorf("failed to find executable %s: %w", config.Command[0], err)
	}

	// Drop capabilities (keep only a safe subset)
	if err := DropCapabilities(); err != nil {
		return fmt.Errorf("failed to drop capabilities: %w", err)
	}

	// Apply seccomp filter (block dangerous syscalls)
	if err := ApplySeccompFilter(); err != nil {
		return fmt.Errorf("failed to apply seccomp filter: %w", err)
	}

	// Execute the container command (replaces this process)
	if err := unix.Exec(execPath, config.Command, config.Env); err != nil {
		return fmt.Errorf("failed to exec %s: %w", config.Command[0], err)
	}

	// Should never reach here
	return nil
}

// setupMountNamespace configures the mount namespace before pivot_root.
func setupMountNamespace(newroot string) error {
	// Make the mount namespace private to prevent propagation to host
	if err := unix.Mount("", "/", "", unix.MS_PRIVATE|unix.MS_REC, ""); err != nil {
		return fmt.Errorf("failed to make root private: %w", err)
	}

	// Bind mount the new root onto itself (required for pivot_root)
	if err := unix.Mount(newroot, newroot, "", unix.MS_BIND|unix.MS_REC, ""); err != nil {
		return fmt.Errorf("failed to bind mount newroot: %w", err)
	}

	// Create mount point directories
	mountPoints := []string{"proc", "dev", "dev/pts", "sys", "tmp"}
	for _, mp := range mountPoints {
		path := filepath.Join(newroot, mp)
		if err := os.MkdirAll(path, 0755); err != nil {
			return fmt.Errorf("failed to create %s: %w", path, err)
		}
	}

	// Mount proc
	procPath := filepath.Join(newroot, "proc")
	if err := unix.Mount("proc", procPath, "proc", 0, ""); err != nil {
		return fmt.Errorf("failed to mount proc: %w", err)
	}

	// Mount tmpfs on /dev
	devPath := filepath.Join(newroot, "dev")
	if err := unix.Mount("tmpfs", devPath, "tmpfs", unix.MS_NOSUID|unix.MS_STRICTATIME, "mode=755"); err != nil {
		return fmt.Errorf("failed to mount tmpfs on dev: %w", err)
	}

	// Create device nodes
	for _, dev := range deviceNodes {
		devNodePath := filepath.Join(newroot, dev.Path)
		devNum := unix.Mkdev(dev.Major, dev.Minor)
		if err := unix.Mknod(devNodePath, dev.Mode, int(devNum)); err != nil {
			return fmt.Errorf("failed to create device %s: %w", dev.Path, err)
		}
	}

	// Create /dev/pts directory and mount devpts
	ptsPath := filepath.Join(newroot, "dev/pts")
	if err := os.MkdirAll(ptsPath, 0755); err != nil {
		return fmt.Errorf("failed to create /dev/pts: %w", err)
	}
	if err := unix.Mount("devpts", ptsPath, "devpts", 0, ""); err != nil {
		return fmt.Errorf("failed to mount devpts: %w", err)
	}

	// Create /dev/ptmx symlink
	ptmxPath := filepath.Join(newroot, "dev/ptmx")
	if err := os.Symlink("pts/ptmx", ptmxPath); err != nil && !os.IsExist(err) {
		return fmt.Errorf("failed to create /dev/ptmx symlink: %w", err)
	}

	// Create standard file descriptors
	if err := os.Symlink("/proc/self/fd", filepath.Join(newroot, "dev/fd")); err != nil && !os.IsExist(err) {
		return fmt.Errorf("failed to create /dev/fd symlink: %w", err)
	}
	if err := os.Symlink("/proc/self/fd/0", filepath.Join(newroot, "dev/stdin")); err != nil && !os.IsExist(err) {
		return fmt.Errorf("failed to create /dev/stdin symlink: %w", err)
	}
	if err := os.Symlink("/proc/self/fd/1", filepath.Join(newroot, "dev/stdout")); err != nil && !os.IsExist(err) {
		return fmt.Errorf("failed to create /dev/stdout symlink: %w", err)
	}
	if err := os.Symlink("/proc/self/fd/2", filepath.Join(newroot, "dev/stderr")); err != nil && !os.IsExist(err) {
		return fmt.Errorf("failed to create /dev/stderr symlink: %w", err)
	}

	// Mount sysfs (read-only)
	sysPath := filepath.Join(newroot, "sys")
	if err := unix.Mount("sysfs", sysPath, "sysfs", unix.MS_RDONLY, ""); err != nil {
		return fmt.Errorf("failed to mount sysfs: %w", err)
	}

	// Mount tmpfs on /tmp
	tmpPath := filepath.Join(newroot, "tmp")
	if err := unix.Mount("tmpfs", tmpPath, "tmpfs", unix.MS_NOSUID|unix.MS_NODEV, "mode=1777"); err != nil {
		return fmt.Errorf("failed to mount tmpfs on /tmp: %w", err)
	}

	return nil
}

// pivotRoot switches the root filesystem to newroot.
func pivotRoot(newroot string) error {
	// Create old_root directory inside the new root
	oldroot := filepath.Join(newroot, ".old_root")
	if err := os.MkdirAll(oldroot, 0700); err != nil {
		return fmt.Errorf("failed to create old_root: %w", err)
	}

	// pivot_root swaps the root mount with old_root
	if err := unix.PivotRoot(newroot, oldroot); err != nil {
		return fmt.Errorf("pivot_root failed: %w", err)
	}

	// Change to new root
	if err := os.Chdir("/"); err != nil {
		return fmt.Errorf("failed to chdir to /: %w", err)
	}

	// Unmount old root
	oldroot = "/.old_root"
	if err := unix.Unmount(oldroot, unix.MNT_DETACH); err != nil {
		return fmt.Errorf("failed to unmount old root: %w", err)
	}

	// Remove old root directory
	if err := os.RemoveAll(oldroot); err != nil {
		return fmt.Errorf("failed to remove old root: %w", err)
	}

	return nil
}

// setupEtcFiles creates essential /etc files inside the container.
func setupEtcFiles(hostname string) error {
	// Ensure /etc exists
	if err := os.MkdirAll("/etc", 0755); err != nil {
		return fmt.Errorf("failed to create /etc: %w", err)
	}

	// /etc/hostname
	if err := os.WriteFile("/etc/hostname", []byte(hostname+"\n"), 0644); err != nil {
		return fmt.Errorf("failed to write /etc/hostname: %w", err)
	}

	// /etc/hosts
	hosts := fmt.Sprintf("127.0.0.1\tlocalhost\n127.0.0.1\t%s\n::1\tlocalhost ip6-localhost ip6-loopback\n", hostname)
	if err := os.WriteFile("/etc/hosts", []byte(hosts), 0644); err != nil {
		return fmt.Errorf("failed to write /etc/hosts: %w", err)
	}

	// /etc/resolv.conf - use Google DNS as default
	resolv := "nameserver 8.8.8.8\nnameserver 8.8.4.4\n"
	if err := os.WriteFile("/etc/resolv.conf", []byte(resolv), 0644); err != nil {
		return fmt.Errorf("failed to write /etc/resolv.conf: %w", err)
	}

	return nil
}

// findExecutable looks for an executable in PATH or returns the path directly if absolute.
func findExecutable(name string) (string, error) {
	if filepath.IsAbs(name) {
		if _, err := os.Stat(name); err != nil {
			return "", err
		}
		return name, nil
	}

	// Search in common paths
	paths := []string{"/bin", "/sbin", "/usr/bin", "/usr/sbin", "/usr/local/bin"}
	for _, dir := range paths {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("executable %s not found", name)
}
