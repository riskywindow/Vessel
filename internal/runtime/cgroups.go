// Package runtime implements the container runtime using Linux primitives.
package runtime

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/vessel/vessel/internal/store"
)

const (
	// cgroupRoot is the root of the cgroup v2 hierarchy.
	cgroupRoot = "/sys/fs/cgroup"

	// vesselCgroupDir is the parent cgroup for all vessel containers.
	vesselCgroupDir = "/sys/fs/cgroup/vessel"
)

// CreateCgroup creates a cgroup for a container with the specified resource limits.
func CreateCgroup(containerID string, limits store.ResourceLimits) error {
	// First, ensure the parent vessel cgroup directory exists
	if err := ensureVesselCgroup(); err != nil {
		return fmt.Errorf("failed to ensure vessel cgroup: %w", err)
	}

	// Create the container's cgroup directory
	containerCgroup := filepath.Join(vesselCgroupDir, containerID)
	if err := os.MkdirAll(containerCgroup, 0755); err != nil {
		return fmt.Errorf("failed to create container cgroup: %w", err)
	}

	// Apply resource limits
	if err := applyCPULimits(containerCgroup, limits); err != nil {
		RemoveCgroup(containerID)
		return fmt.Errorf("failed to apply CPU limits: %w", err)
	}

	if err := applyMemoryLimits(containerCgroup, limits); err != nil {
		RemoveCgroup(containerID)
		return fmt.Errorf("failed to apply memory limits: %w", err)
	}

	if err := applyPIDLimits(containerCgroup, limits); err != nil {
		RemoveCgroup(containerID)
		return fmt.Errorf("failed to apply PID limits: %w", err)
	}

	return nil
}

// ensureVesselCgroup creates the parent vessel cgroup and enables required controllers.
func ensureVesselCgroup() error {
	// Ensure controllers are enabled at the root level first
	if err := enableControllersAtRoot(); err != nil {
		return fmt.Errorf("failed to enable controllers at root: %w", err)
	}

	// Create the vessel parent cgroup
	if err := os.MkdirAll(vesselCgroupDir, 0755); err != nil {
		return fmt.Errorf("failed to create vessel cgroup dir: %w", err)
	}

	// Enable controllers in the vessel cgroup for child cgroups
	subtreeControl := filepath.Join(vesselCgroupDir, "cgroup.subtree_control")
	controllers := "+cpu +memory +pids +io"
	if err := os.WriteFile(subtreeControl, []byte(controllers), 0644); err != nil {
		// Some controllers might not be available, try a minimal set
		minimalControllers := "+cpu +memory +pids"
		if err := os.WriteFile(subtreeControl, []byte(minimalControllers), 0644); err != nil {
			// If that also fails, log but continue - some environments have restrictions
			// We'll still be able to create the cgroup, just without all limits
		}
	}

	return nil
}

// enableControllersAtRoot ensures the required controllers are enabled at the root cgroup level.
func enableControllersAtRoot() error {
	subtreeControl := filepath.Join(cgroupRoot, "cgroup.subtree_control")

	// Read current controllers
	data, err := os.ReadFile(subtreeControl)
	if err != nil {
		return fmt.Errorf("failed to read root subtree_control: %w", err)
	}

	currentControllers := string(data)
	neededControllers := []string{"cpu", "memory", "pids", "io"}
	var missingControllers []string

	for _, controller := range neededControllers {
		if !strings.Contains(currentControllers, controller) {
			missingControllers = append(missingControllers, "+"+controller)
		}
	}

	if len(missingControllers) > 0 {
		enableStr := strings.Join(missingControllers, " ")
		if err := os.WriteFile(subtreeControl, []byte(enableStr), 0644); err != nil {
			// Not all controllers may be available, try one at a time
			for _, controller := range missingControllers {
				os.WriteFile(subtreeControl, []byte(controller), 0644)
			}
		}
	}

	return nil
}

// applyCPULimits writes CPU limits to the cgroup.
func applyCPULimits(cgroupPath string, limits store.ResourceLimits) error {
	if limits.CPUQuota <= 0 {
		// No CPU limit, set to "max" (unlimited)
		return writeCgroupFile(cgroupPath, "cpu.max", "max 100000")
	}

	period := limits.CPUPeriod
	if period <= 0 {
		period = 100000 // Default: 100ms in microseconds
	}

	quota := limits.CPUQuota
	// cpu.max format: "$QUOTA $PERIOD"
	value := fmt.Sprintf("%d %d", quota, period)
	return writeCgroupFile(cgroupPath, "cpu.max", value)
}

// applyMemoryLimits writes memory limits to the cgroup.
func applyMemoryLimits(cgroupPath string, limits store.ResourceLimits) error {
	if limits.MemoryMax > 0 {
		if err := writeCgroupFile(cgroupPath, "memory.max", strconv.FormatInt(limits.MemoryMax, 10)); err != nil {
			return fmt.Errorf("failed to set memory.max: %w", err)
		}
	}

	// Set swap limit
	swapMax := limits.SwapMax
	if swapMax == 0 {
		// Disable swap
		swapMax = 0
	} else if swapMax < 0 {
		// -1 means same as memory
		swapMax = limits.MemoryMax
	}

	if swapMax >= 0 {
		if err := writeCgroupFile(cgroupPath, "memory.swap.max", strconv.FormatInt(swapMax, 10)); err != nil {
			// Some systems don't have swap cgroup, ignore error
		}
	}

	return nil
}

// applyPIDLimits writes PID limits to the cgroup.
func applyPIDLimits(cgroupPath string, limits store.ResourceLimits) error {
	if limits.PidsMax <= 0 {
		// No limit
		return writeCgroupFile(cgroupPath, "pids.max", "max")
	}
	return writeCgroupFile(cgroupPath, "pids.max", strconv.FormatInt(limits.PidsMax, 10))
}

// writeCgroupFile writes a value to a cgroup control file.
func writeCgroupFile(cgroupPath, filename, value string) error {
	path := filepath.Join(cgroupPath, filename)
	return os.WriteFile(path, []byte(value), 0644)
}

// AddProcessToCgroup adds a process to a container's cgroup.
func AddProcessToCgroup(containerID string, pid int) error {
	cgroupPath := filepath.Join(vesselCgroupDir, containerID)
	procsFile := filepath.Join(cgroupPath, "cgroup.procs")

	return os.WriteFile(procsFile, []byte(strconv.Itoa(pid)), 0644)
}

// GetCgroupStats reads resource usage statistics from a container's cgroup.
func GetCgroupStats(containerID string) (*store.MetricPoint, error) {
	cgroupPath := filepath.Join(vesselCgroupDir, containerID)

	// Check if cgroup exists
	if _, err := os.Stat(cgroupPath); err != nil {
		return nil, fmt.Errorf("cgroup not found: %w", err)
	}

	stats := &store.MetricPoint{
		Timestamp: time.Now(),
	}

	// Read CPU stats
	cpuUsage, err := readCPUStats(cgroupPath)
	if err == nil {
		stats.CPUPercent = cpuUsage
	}

	// Read memory stats
	memCurrent, err := readCgroupUint64(cgroupPath, "memory.current")
	if err == nil {
		stats.MemoryBytes = int64(memCurrent)
	}

	memMax, err := readCgroupUint64(cgroupPath, "memory.max")
	if err == nil {
		stats.MemoryLimit = int64(memMax)
	}

	// Read PID count
	pidsCurrent, err := readCgroupUint64(cgroupPath, "pids.current")
	if err == nil {
		stats.PidsCount = int64(pidsCurrent)
	}

	return stats, nil
}

// readCPUStats reads CPU statistics from the cgroup.
func readCPUStats(cgroupPath string) (float64, error) {
	statFile := filepath.Join(cgroupPath, "cpu.stat")
	file, err := os.Open(statFile)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	var usageUsec uint64
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "usage_usec") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				usageUsec, _ = strconv.ParseUint(parts[1], 10, 64)
			}
		}
	}

	// Convert to a percentage-like value (usage in seconds)
	// This is cumulative, so for real-time percentage you'd need two samples
	return float64(usageUsec) / 1000000.0, nil
}

// readCgroupUint64 reads a uint64 value from a cgroup file.
func readCgroupUint64(cgroupPath, filename string) (uint64, error) {
	path := filepath.Join(cgroupPath, filename)
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}

	valueStr := strings.TrimSpace(string(data))
	if valueStr == "max" {
		return 0, nil // Return 0 for "max" (unlimited)
	}

	return strconv.ParseUint(valueStr, 10, 64)
}

// RemoveCgroup removes a container's cgroup.
// The cgroup must be empty (no processes) before removal.
func RemoveCgroup(containerID string) error {
	cgroupPath := filepath.Join(vesselCgroupDir, containerID)

	// Check if cgroup exists
	if _, err := os.Stat(cgroupPath); err != nil {
		if os.IsNotExist(err) {
			return nil // Already removed
		}
		return err
	}

	// Verify no processes are in the cgroup
	procsFile := filepath.Join(cgroupPath, "cgroup.procs")
	data, err := os.ReadFile(procsFile)
	if err == nil && len(strings.TrimSpace(string(data))) > 0 {
		// There are still processes in the cgroup
		// This shouldn't happen if StopContainer was called properly
		return fmt.Errorf("cgroup still has processes")
	}

	// Remove the cgroup directory (must use rmdir semantics, not RemoveAll)
	return os.Remove(cgroupPath)
}

// CleanupVesselCgroup removes the parent vessel cgroup if it's empty.
func CleanupVesselCgroup() error {
	// Check if vessel cgroup exists and is empty
	entries, err := os.ReadDir(vesselCgroupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	// Only remove if no child cgroups exist
	for _, entry := range entries {
		if entry.IsDir() {
			// Still has child cgroups
			return nil
		}
	}

	// Try to remove (will fail if not empty of cgroup control files, which is fine)
	os.Remove(vesselCgroupDir)
	return nil
}

// GetCgroupMemoryEvents returns memory event counts (like OOM kills).
func GetCgroupMemoryEvents(containerID string) (map[string]uint64, error) {
	cgroupPath := filepath.Join(vesselCgroupDir, containerID)
	eventsFile := filepath.Join(cgroupPath, "memory.events")

	file, err := os.Open(eventsFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	events := make(map[string]uint64)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			value, err := strconv.ParseUint(parts[1], 10, 64)
			if err == nil {
				events[parts[0]] = value
			}
		}
	}

	return events, scanner.Err()
}

// ValidateResourceLimits checks that resource limits are reasonable and sets defaults.
func ValidateResourceLimits(limits *store.ResourceLimits) error {
	// Set defaults if not specified
	if limits.MemoryMax == 0 {
		limits.MemoryMax = 256 * 1024 * 1024 // 256MB default
	}

	if limits.PidsMax == 0 {
		limits.PidsMax = 512 // 512 processes default
	}

	if limits.CPUPeriod == 0 {
		limits.CPUPeriod = 100000 // 100ms default period
	}

	// Validate ranges
	if limits.MemoryMax < 0 {
		return fmt.Errorf("memory limit cannot be negative")
	}

	if limits.PidsMax < 0 {
		return fmt.Errorf("PID limit cannot be negative")
	}

	if limits.CPUQuota < 0 {
		return fmt.Errorf("CPU quota cannot be negative")
	}

	// Reasonable minimums
	if limits.MemoryMax > 0 && limits.MemoryMax < 4*1024*1024 {
		return fmt.Errorf("memory limit too low (minimum 4MB)")
	}

	if limits.PidsMax > 0 && limits.PidsMax < 1 {
		return fmt.Errorf("PID limit too low (minimum 1)")
	}

	return nil
}
