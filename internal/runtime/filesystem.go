package runtime

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/vessel/vessel/internal/paths"
	"golang.org/x/sys/unix"
)

// Standard Linux directory structure for containers.
var standardDirs = []string{
	"bin",
	"dev",
	"etc",
	"home",
	"lib",
	"lib64",
	"proc",
	"root",
	"run",
	"sbin",
	"sys",
	"tmp",
	"usr",
	"usr/bin",
	"usr/lib",
	"usr/lib64",
	"usr/sbin",
	"usr/share",
	"usr/local",
	"usr/local/bin",
	"var",
	"var/log",
	"var/run",
	"var/tmp",
}

// BusyboxApplets are the common busybox commands to symlink.
var BusyboxApplets = []string{
	"sh", "ash", "ls", "cat", "echo", "ps", "pwd", "mkdir", "rm", "cp", "mv",
	"ln", "chmod", "chown", "touch", "head", "tail", "grep", "sed", "awk",
	"date", "hostname", "id", "whoami", "uname", "env", "sleep", "test",
	"true", "false", "yes", "kill", "wc", "sort", "uniq", "tr", "cut",
	"xargs", "find", "which", "printf", "expr", "basename", "dirname",
	"readlink", "realpath", "stat", "df", "du", "free", "mount", "umount",
	"dmesg", "ping", "netstat", "ifconfig", "route", "wget", "vi", "more",
	"less", "tar", "gzip", "gunzip", "zcat",
}

// CreateMinimalRootfs creates a minimal rootfs directory structure.
// It tries busybox first (preferred), then falls back to copying host binaries.
func CreateMinimalRootfs(path string) error {
	// Try busybox first
	if err := CreateBusyboxRootfs(path); err == nil {
		return nil
	}

	// Fall back to host-based rootfs
	return CreateHostRootfs(path)
}

// CreateBusyboxRootfs creates a minimal rootfs using busybox-static.
// This is the preferred method as it avoids shared library dependencies.
func CreateBusyboxRootfs(path string) error {
	// Find busybox-static
	busyboxPath, err := findBusybox()
	if err != nil {
		return err
	}

	// Create directory structure
	if err := createDirectoryStructure(path); err != nil {
		return err
	}

	// Copy busybox
	busyboxDest := filepath.Join(path, "bin", "busybox")
	if err := copyFile(busyboxPath, busyboxDest); err != nil {
		return fmt.Errorf("failed to copy busybox: %w", err)
	}

	// Make busybox executable
	if err := os.Chmod(busyboxDest, 0755); err != nil {
		return fmt.Errorf("failed to chmod busybox: %w", err)
	}

	// Create symlinks for all applets
	for _, applet := range BusyboxApplets {
		linkPath := filepath.Join(path, "bin", applet)
		// Remove existing file/link if present
		os.Remove(linkPath)
		if err := os.Symlink("busybox", linkPath); err != nil {
			// Don't fail on symlink errors, just continue
			continue
		}
	}

	// Also create links in /usr/bin for common utilities
	usrBinApplets := []string{"env", "head", "tail", "wc", "sort", "uniq", "tr", "cut"}
	for _, applet := range usrBinApplets {
		linkPath := filepath.Join(path, "usr", "bin", applet)
		os.Remove(linkPath)
		os.Symlink("/bin/busybox", linkPath)
	}

	// Create essential /etc files
	if err := createEtcFiles(path); err != nil {
		return err
	}

	return nil
}

// CreateHostRootfs creates a rootfs by copying binaries from the host.
// This is a fallback when busybox is not available.
func CreateHostRootfs(path string) error {
	// Create directory structure
	if err := createDirectoryStructure(path); err != nil {
		return err
	}

	// List of essential binaries to copy
	binaries := []string{
		"/bin/sh",
		"/bin/ls",
		"/bin/cat",
		"/bin/echo",
		"/bin/ps",
		"/bin/hostname",
		"/bin/pwd",
		"/bin/mkdir",
		"/bin/rm",
	}

	// Copy each binary and its dependencies
	for _, bin := range binaries {
		if _, err := os.Stat(bin); err != nil {
			continue // Skip if not found
		}

		// Copy the binary
		destPath := filepath.Join(path, bin)
		if err := copyFile(bin, destPath); err != nil {
			continue
		}
		os.Chmod(destPath, 0755)

		// Copy shared library dependencies
		if err := copyLibraries(bin, path); err != nil {
			// Continue even if library copying fails
			continue
		}
	}

	// Create essential /etc files
	if err := createEtcFiles(path); err != nil {
		return err
	}

	return nil
}

// findBusybox looks for busybox-static in common locations.
func findBusybox() (string, error) {
	paths := []string{
		"/bin/busybox-static",
		"/usr/bin/busybox-static",
		"/bin/busybox",
		"/usr/bin/busybox",
	}

	for _, p := range paths {
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			// Check if it's statically linked
			out, err := exec.Command("file", p).Output()
			if err == nil && strings.Contains(string(out), "statically linked") {
				return p, nil
			}
			// Even if not static, use it as fallback
			return p, nil
		}
	}

	return "", fmt.Errorf("busybox not found")
}

// createDirectoryStructure creates the standard Linux directory structure.
func createDirectoryStructure(root string) error {
	for _, dir := range standardDirs {
		path := filepath.Join(root, dir)
		if err := os.MkdirAll(path, 0755); err != nil {
			return fmt.Errorf("failed to create %s: %w", path, err)
		}
	}

	// Set /tmp permissions
	tmpPath := filepath.Join(root, "tmp")
	if err := os.Chmod(tmpPath, 1777); err != nil {
		return fmt.Errorf("failed to chmod /tmp: %w", err)
	}

	return nil
}

// createEtcFiles creates essential /etc files.
func createEtcFiles(root string) error {
	etcPath := filepath.Join(root, "etc")

	// /etc/passwd
	passwd := `root:x:0:0:root:/root:/bin/sh
nobody:x:65534:65534:nobody:/nonexistent:/bin/false
`
	if err := os.WriteFile(filepath.Join(etcPath, "passwd"), []byte(passwd), 0644); err != nil {
		return fmt.Errorf("failed to write /etc/passwd: %w", err)
	}

	// /etc/group
	group := `root:x:0:
nobody:x:65534:
`
	if err := os.WriteFile(filepath.Join(etcPath, "group"), []byte(group), 0644); err != nil {
		return fmt.Errorf("failed to write /etc/group: %w", err)
	}

	// /etc/nsswitch.conf
	nsswitch := `passwd: files
group: files
shadow: files
hosts: files dns
networks: files
protocols: files
services: files
ethers: files
rpc: files
`
	if err := os.WriteFile(filepath.Join(etcPath, "nsswitch.conf"), []byte(nsswitch), 0644); err != nil {
		return fmt.Errorf("failed to write /etc/nsswitch.conf: %w", err)
	}

	return nil
}

// copyFile copies a file from src to dst, creating parent directories as needed.
func copyFile(src, dst string) error {
	// Create parent directories
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// copyLibraries uses ldd to find and copy shared library dependencies.
func copyLibraries(binary, rootPath string) error {
	// Run ldd to find dependencies
	out, err := exec.Command("ldd", binary).Output()
	if err != nil {
		return err
	}

	// Parse ldd output
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()

		// Parse lines like: "libfoo.so.1 => /lib/x86_64-linux-gnu/libfoo.so.1 (0x...)"
		// or: "/lib64/ld-linux-x86-64.so.2 (0x...)"
		parts := strings.Fields(line)

		var libPath string
		for _, part := range parts {
			if strings.HasPrefix(part, "/") && !strings.HasPrefix(part, "(") {
				libPath = part
				break
			}
		}

		if libPath == "" {
			continue
		}

		// Copy the library
		destPath := filepath.Join(rootPath, libPath)
		if _, err := os.Stat(destPath); err == nil {
			// Already copied
			continue
		}

		if err := copyFile(libPath, destPath); err != nil {
			// Continue on error
			continue
		}
	}

	return nil
}

// MountOverlay mounts an OverlayFS at the specified path.
// lowerDirs should be ordered from bottom to top layer.
func MountOverlay(lowerDirs []string, upperDir, workDir, mergedDir string) error {
	// Create directories
	for _, dir := range []string{upperDir, workDir, mergedDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create %s: %w", dir, err)
		}
	}

	// Build the mount options
	// lowerdir format: dir1:dir2:dir3 (rightmost is bottom layer)
	lowerOpt := strings.Join(lowerDirs, ":")
	data := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", lowerOpt, upperDir, workDir)

	// Mount the overlay
	if err := unix.Mount("overlay", mergedDir, "overlay", 0, data); err != nil {
		return fmt.Errorf("failed to mount overlay: %w", err)
	}

	return nil
}

// UnmountOverlay unmounts an OverlayFS.
func UnmountOverlay(mergedDir string) error {
	if err := unix.Unmount(mergedDir, 0); err != nil {
		// Try lazy unmount if normal unmount fails
		if err := unix.Unmount(mergedDir, unix.MNT_DETACH); err != nil {
			return fmt.Errorf("failed to unmount overlay: %w", err)
		}
	}
	return nil
}

// SetupOverlayFS creates the OverlayFS mount for a container using image layers.
// It returns the merged directory path which becomes the container's rootfs.
func SetupOverlayFS(containerID string, layerPaths []string) (string, error) {
	containerDir := paths.ContainerDir(containerID)
	upperDir := paths.ContainerUpperDir(containerID)
	workDir := paths.ContainerWorkDir(containerID)
	mergedDir := paths.ContainerMergedDir(containerID)

	// Create the container directory structure
	for _, dir := range []string{containerDir, upperDir, workDir, mergedDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return "", fmt.Errorf("failed to create %s: %w", dir, err)
		}
	}

	// Ensure workdir is empty (required by OverlayFS)
	workEntries, _ := os.ReadDir(workDir)
	if len(workEntries) > 0 {
		os.RemoveAll(workDir)
		if err := os.MkdirAll(workDir, 0755); err != nil {
			return "", fmt.Errorf("failed to recreate workdir: %w", err)
		}
	}

	// Build lowerdir string - OverlayFS wants layers from TOP to BOTTOM
	// Our layerPaths are ordered from bottom (0) to top (n-1), so we reverse
	reversedLayers := make([]string, len(layerPaths))
	for i, path := range layerPaths {
		reversedLayers[len(layerPaths)-1-i] = path
	}
	lowerDir := strings.Join(reversedLayers, ":")

	// Build mount options
	data := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", lowerDir, upperDir, workDir)

	// Mount the overlay
	if err := unix.Mount("overlay", mergedDir, "overlay", 0, data); err != nil {
		return "", fmt.Errorf("failed to mount overlay: %w", err)
	}

	return mergedDir, nil
}

// TeardownOverlayFS unmounts the OverlayFS for a container.
func TeardownOverlayFS(containerID string) error {
	mergedDir := paths.ContainerMergedDir(containerID)

	// Check if it's actually mounted
	if _, err := os.Stat(mergedDir); os.IsNotExist(err) {
		return nil // Nothing to unmount
	}

	// Try normal unmount first
	if err := unix.Unmount(mergedDir, 0); err != nil {
		// Try lazy unmount if normal unmount fails (e.g., busy)
		if err := unix.Unmount(mergedDir, unix.MNT_DETACH); err != nil {
			// It might not be mounted, which is fine
			return nil
		}
	}

	return nil
}

// CleanupContainer tears down OverlayFS and removes the container directory.
func CleanupContainer(containerID string) error {
	// First, teardown the overlay mount
	if err := TeardownOverlayFS(containerID); err != nil {
		return fmt.Errorf("failed to teardown overlay: %w", err)
	}

	// Remove the entire container directory
	containerDir := paths.ContainerDir(containerID)
	if err := os.RemoveAll(containerDir); err != nil {
		return fmt.Errorf("failed to remove container directory: %w", err)
	}

	return nil
}

// CleanupRootfs removes a rootfs directory and any associated mounts.
func CleanupRootfs(path string) error {
	// Try to unmount if it's a mount point
	unix.Unmount(path, unix.MNT_DETACH)

	// Remove the directory
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("failed to remove rootfs: %w", err)
	}

	return nil
}
