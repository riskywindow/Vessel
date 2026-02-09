package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vessel/vessel/internal/cli/styles"
	"github.com/vessel/vessel/internal/daemon"
	"github.com/vessel/vessel/internal/paths"
	"golang.org/x/sys/unix"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check system prerequisites",
	Long: `Check that the system meets all prerequisites for running Vessel.

Checks:
  - Linux operating system
  - Kernel version (5.8+ recommended for cgroups v2)
  - cgroups v2 unified hierarchy
  - OverlayFS support
  - Required commands (nsenter, iptables, ip, mount, umount)
  - User permissions (root required for container operations)
  - Vessel data directories
  - Daemon status

Examples:
  vessel doctor
  vessel doctor --json`,
	RunE: runDoctor,
}

// CheckResult represents the outcome of a single diagnostic check.
type CheckResult struct {
	Name    string `json:"name"`
	Status  string `json:"status"` // "pass", "fail", "warn"
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

func runDoctor(cmd *cobra.Command, args []string) error {
	var results []CheckResult

	results = append(results, checkOS())
	results = append(results, checkKernel())
	results = append(results, checkCgroupsV2())
	results = append(results, checkOverlayFS())
	results = append(results, checkRequiredCommands()...)
	results = append(results, checkNetworkCapabilities())
	results = append(results, checkUserPermissions())
	results = append(results, checkVesselDirectories())
	results = append(results, checkDaemonStatus())

	if jsonOutput {
		data, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	fmt.Println(styles.Header.Render("Vessel System Diagnostics"))
	fmt.Println()

	passCount := 0
	failCount := 0
	warnCount := 0

	for _, r := range results {
		var icon string
		switch r.Status {
		case "pass":
			icon = styles.CheckPassIcon()
			passCount++
		case "fail":
			icon = styles.CheckFailIcon()
			failCount++
		case "warn":
			icon = styles.CheckWarnIcon()
			warnCount++
		}

		fmt.Printf("%s %s: %s\n", icon, r.Name, r.Message)
		if r.Details != "" && verbose {
			fmt.Printf("    %s\n", styles.Muted.Render(r.Details))
		}
	}

	// Summary
	fmt.Println()
	summary := fmt.Sprintf("%d passed", passCount)
	if warnCount > 0 {
		summary += fmt.Sprintf(", %d warnings", warnCount)
	}
	if failCount > 0 {
		summary += fmt.Sprintf(", %d failed", failCount)
	}

	if failCount > 0 {
		fmt.Println(styles.Error.Render("✗ " + summary))
		fmt.Println()
		fmt.Println("Some checks failed. Vessel may not work correctly.")
		return fmt.Errorf("doctor found %d issues", failCount)
	}

	if warnCount > 0 {
		fmt.Println(styles.Warning.Render("! " + summary))
	} else {
		fmt.Println(styles.Success.Render("✓ " + summary))
	}

	fmt.Println()
	fmt.Println("Vessel is ready to run!")

	return nil
}

func checkOS() CheckResult {
	if runtime.GOOS != "linux" {
		return CheckResult{
			Name:    "Operating System",
			Status:  "fail",
			Message: fmt.Sprintf("%s is not supported (requires Linux)", runtime.GOOS),
		}
	}
	return CheckResult{
		Name:    "Operating System",
		Status:  "pass",
		Message: fmt.Sprintf("Linux (%s)", runtime.GOARCH),
	}
}

func checkKernel() CheckResult {
	var uname unix.Utsname
	if err := unix.Uname(&uname); err != nil {
		return CheckResult{
			Name:    "Kernel Version",
			Status:  "warn",
			Message: "Could not determine kernel version",
			Details: err.Error(),
		}
	}

	release := unix.ByteSliceToString(uname.Release[:])

	// Parse major.minor
	parts := strings.Split(release, ".")
	if len(parts) >= 2 {
		var major, minor int
		fmt.Sscanf(parts[0], "%d", &major)
		fmt.Sscanf(parts[1], "%d", &minor)

		if major < 5 || (major == 5 && minor < 8) {
			return CheckResult{
				Name:    "Kernel Version",
				Status:  "warn",
				Message: fmt.Sprintf("%s (5.8+ recommended for full cgroups v2 support)", release),
			}
		}
	}

	return CheckResult{
		Name:    "Kernel Version",
		Status:  "pass",
		Message: release,
	}
}

func checkCgroupsV2() CheckResult {
	data, err := os.ReadFile("/proc/filesystems")
	if err != nil {
		return CheckResult{
			Name:    "cgroups v2",
			Status:  "fail",
			Message: "Could not read /proc/filesystems",
			Details: err.Error(),
		}
	}

	if !strings.Contains(string(data), "cgroup2") {
		return CheckResult{
			Name:    "cgroups v2",
			Status:  "fail",
			Message: "cgroup2 filesystem not available",
			Details: "Kernel may need CONFIG_CGROUP2 enabled",
		}
	}

	// Check if unified hierarchy is in use
	if _, err := os.Stat("/sys/fs/cgroup/cgroup.controllers"); err != nil {
		return CheckResult{
			Name:    "cgroups v2",
			Status:  "fail",
			Message: "cgroups v2 unified hierarchy not mounted",
			Details: "Add 'systemd.unified_cgroup_hierarchy=1' to kernel cmdline",
		}
	}

	return CheckResult{
		Name:    "cgroups v2",
		Status:  "pass",
		Message: "Unified hierarchy active",
	}
}

func checkOverlayFS() CheckResult {
	data, err := os.ReadFile("/proc/filesystems")
	if err != nil {
		return CheckResult{
			Name:    "OverlayFS",
			Status:  "fail",
			Message: "Could not read /proc/filesystems",
		}
	}

	if !strings.Contains(string(data), "overlay") {
		return CheckResult{
			Name:    "OverlayFS",
			Status:  "fail",
			Message: "overlay filesystem not available",
			Details: "Load the overlay kernel module: modprobe overlay",
		}
	}

	return CheckResult{
		Name:    "OverlayFS",
		Status:  "pass",
		Message: "Available",
	}
}

func checkRequiredCommands() []CheckResult {
	commands := []struct {
		name     string
		required bool
	}{
		{"nsenter", true},
		{"iptables", true},
		{"ip", true},
		{"mount", true},
		{"umount", true},
		{"tar", false},
		{"gzip", false},
	}

	var results []CheckResult
	for _, c := range commands {
		path, err := exec.LookPath(c.name)
		if err != nil {
			status := "warn"
			if c.required {
				status = "fail"
			}
			results = append(results, CheckResult{
				Name:    fmt.Sprintf("Command: %s", c.name),
				Status:  status,
				Message: "Not found in PATH",
			})
		} else {
			results = append(results, CheckResult{
				Name:    fmt.Sprintf("Command: %s", c.name),
				Status:  "pass",
				Message: "Found",
				Details: path,
			})
		}
	}

	return results
}

func checkNetworkCapabilities() CheckResult {
	if os.Geteuid() != 0 {
		return CheckResult{
			Name:    "Network Capabilities",
			Status:  "warn",
			Message: "Not running as root",
			Details: "Network features require root privileges",
		}
	}

	// Check if vessel0 bridge exists
	if _, err := os.Stat("/sys/class/net/vessel0"); err == nil {
		return CheckResult{
			Name:    "Network Capabilities",
			Status:  "pass",
			Message: "Bridge network configured (vessel0)",
		}
	}

	return CheckResult{
		Name:    "Network Capabilities",
		Status:  "pass",
		Message: "Available (bridge not yet created)",
	}
}

func checkUserPermissions() CheckResult {
	if os.Geteuid() == 0 {
		return CheckResult{
			Name:    "User Permissions",
			Status:  "pass",
			Message: "Running as root",
		}
	}

	return CheckResult{
		Name:    "User Permissions",
		Status:  "warn",
		Message: "Not running as root",
		Details: "Vessel requires root for container operations",
	}
}

func checkVesselDirectories() CheckResult {
	dirs := []string{
		paths.RootDir,
		paths.ContainersDir,
		paths.ImagesDir,
	}

	existing := 0
	for _, dir := range dirs {
		if _, err := os.Stat(dir); err == nil {
			existing++
		}
	}

	if existing == 0 {
		return CheckResult{
			Name:    "Vessel Directories",
			Status:  "pass",
			Message: "Will be created on first run",
			Details: paths.RootDir,
		}
	}

	if existing == len(dirs) {
		return CheckResult{
			Name:    "Vessel Directories",
			Status:  "pass",
			Message: "Configured",
			Details: paths.RootDir,
		}
	}

	return CheckResult{
		Name:    "Vessel Directories",
		Status:  "pass",
		Message: fmt.Sprintf("Partially configured (%d/%d)", existing, len(dirs)),
		Details: paths.RootDir,
	}
}

func checkDaemonStatus() CheckResult {
	if _, err := os.Stat(paths.SocketPath); err != nil {
		return CheckResult{
			Name:    "Daemon Status",
			Status:  "warn",
			Message: "Not running",
			Details: "Start with: vessel daemon start",
		}
	}

	// Try to ping
	client := daemon.NewClient("")
	if err := client.Ping(); err != nil {
		return CheckResult{
			Name:    "Daemon Status",
			Status:  "warn",
			Message: "Socket exists but daemon not responding",
			Details: err.Error(),
		}
	}

	return CheckResult{
		Name:    "Daemon Status",
		Status:  "pass",
		Message: "Running",
	}
}
