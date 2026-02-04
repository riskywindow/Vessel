package runtime

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"golang.org/x/sys/unix"
)

// VesselBinaryPath can be set to override the path to the vessel binary.
// This is useful for testing where os.Executable() returns the test binary.
var VesselBinaryPath string

// CreateNamespacedProcess creates a new process in isolated namespaces.
// It re-executes the vessel binary with the "init" subcommand, which triggers
// the container init code path.
func CreateNamespacedProcess(config NamespaceConfig, stdout, stderr *os.File) (*os.Process, error) {
	// Create a pipe for parent → child communication
	readPipe, writePipe, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create pipe: %w", err)
	}

	// Get the vessel binary path
	var self string
	if VesselBinaryPath != "" {
		self = VesselBinaryPath
	} else {
		self, err = os.Executable()
		if err != nil {
			readPipe.Close()
			writePipe.Close()
			return nil, fmt.Errorf("failed to get executable path: %w", err)
		}
	}

	// Create the command to re-execute vessel with "init" argument
	cmd := exec.Command(self, "__init__")

	// Set up namespaces via clone flags
	// Note: we use syscall.SysProcAttr (not unix.SysProcAttr) because os/exec expects it
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWPID | // New PID namespace
			syscall.CLONE_NEWNS | // New mount namespace
			syscall.CLONE_NEWUTS | // New UTS namespace (hostname)
			syscall.CLONE_NEWIPC | // New IPC namespace
			syscall.CLONE_NEWNET, // New network namespace
	}

	// Pass the read end of the pipe as FD 3
	cmd.ExtraFiles = []*os.File{readPipe}

	// Set up stdout/stderr
	if stdout != nil {
		cmd.Stdout = stdout
	}
	if stderr != nil {
		cmd.Stderr = stderr
	}

	// Set environment variables for the child
	cmd.Env = []string{
		"PATH=/bin:/sbin:/usr/bin:/usr/sbin:/usr/local/bin",
	}

	// Start the process
	if err := cmd.Start(); err != nil {
		readPipe.Close()
		writePipe.Close()
		return nil, fmt.Errorf("failed to start process: %w", err)
	}

	// Close the read end in the parent (child has it)
	readPipe.Close()

	// Send the configuration to the child
	encoder := json.NewEncoder(writePipe)
	if err := encoder.Encode(&config); err != nil {
		writePipe.Close()
		// Kill the child if we can't send config
		cmd.Process.Kill()
		return nil, fmt.Errorf("failed to send config: %w", err)
	}

	// Close the write end (signals EOF to child)
	writePipe.Close()

	return cmd.Process, nil
}

// SetupNetworkNamespace configures the network namespace after the container starts.
// This is called from the host side to set up veth pairs, etc.
// For now, this is a stub that will be implemented when we add network support.
func SetupNetworkNamespace(pid int, containerIP string) error {
	// Get the network namespace path for the container
	nsPath := fmt.Sprintf("/proc/%d/ns/net", pid)

	// Verify the namespace exists
	if _, err := os.Stat(nsPath); err != nil {
		return fmt.Errorf("network namespace not found: %w", err)
	}

	// Network setup will be implemented in a future session
	// This includes:
	// 1. Creating a veth pair
	// 2. Moving one end into the container's network namespace
	// 3. Configuring IP addresses
	// 4. Setting up routing

	return nil
}

// GetNamespaceIDs returns the namespace IDs for a given PID.
// Useful for debugging and verification.
func GetNamespaceIDs(pid int) (map[string]uint64, error) {
	namespaces := []string{"pid", "mnt", "uts", "ipc", "net", "user"}
	ids := make(map[string]uint64)

	for _, ns := range namespaces {
		path := fmt.Sprintf("/proc/%d/ns/%s", pid, ns)
		var stat unix.Stat_t
		if err := unix.Stat(path, &stat); err != nil {
			return nil, fmt.Errorf("failed to stat %s: %w", path, err)
		}
		ids[ns] = stat.Ino
	}

	return ids, nil
}

// IsInNamespace checks if the current process is running in a namespace
// different from the init process (PID 1).
func IsInNamespace(nsType string) (bool, error) {
	selfPath := fmt.Sprintf("/proc/self/ns/%s", nsType)
	initPath := fmt.Sprintf("/proc/1/ns/%s", nsType)

	var selfStat, initStat unix.Stat_t
	if err := unix.Stat(selfPath, &selfStat); err != nil {
		return false, fmt.Errorf("failed to stat self namespace: %w", err)
	}
	if err := unix.Stat(initPath, &initStat); err != nil {
		return false, fmt.Errorf("failed to stat init namespace: %w", err)
	}

	return selfStat.Ino != initStat.Ino, nil
}
