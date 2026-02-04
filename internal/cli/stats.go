package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/vessel/vessel/internal/runtime"
)

var statsCmd = &cobra.Command{
	Use:   "stats CONTAINER",
	Short: "Show resource usage statistics for a container",
	Long: `Show resource usage statistics for a running container.

Displays CPU, memory, and process count metrics from cgroups v2.

Examples:
  vessel stats mycontainer     # Stats for container named 'mycontainer'
  vessel stats abc123def456    # Stats for container by ID`,
	Args: cobra.ExactArgs(1),
	RunE: showContainerStats,
}

func init() {
	rootCmd.AddCommand(statsCmd)
}

func showContainerStats(cmd *cobra.Command, args []string) error {
	containerID := args[0]

	// Check for root privileges
	if os.Getuid() != 0 {
		return fmt.Errorf("vessel stats requires root privileges")
	}

	// Create runtime
	rt, err := runtime.NewLinuxRuntime()
	if err != nil {
		return fmt.Errorf("failed to create runtime: %w", err)
	}

	ctx := context.Background()

	// Get container stats
	stats, err := rt.ContainerStats(ctx, containerID)
	if err != nil {
		return fmt.Errorf("failed to get stats: %w", err)
	}

	// Display stats
	fmt.Printf("Container: %s\n", containerID)
	fmt.Printf("Timestamp: %s\n\n", stats.Timestamp.Format(time.RFC3339))

	// CPU
	fmt.Printf("CPU:\n")
	fmt.Printf("  Usage: %.2f seconds (cumulative)\n", stats.CPUPercent)

	// Memory
	fmt.Printf("\nMemory:\n")
	fmt.Printf("  Current: %s\n", formatBytes(stats.MemoryBytes))
	if stats.MemoryLimit > 0 {
		fmt.Printf("  Limit:   %s\n", formatBytes(stats.MemoryLimit))
		pct := float64(stats.MemoryBytes) / float64(stats.MemoryLimit) * 100
		fmt.Printf("  Usage:   %.1f%%\n", pct)
	}

	// PIDs
	fmt.Printf("\nProcesses:\n")
	fmt.Printf("  Count: %d\n", stats.PidsCount)

	// Network (if available)
	if stats.NetworkRxBytes > 0 || stats.NetworkTxBytes > 0 {
		fmt.Printf("\nNetwork:\n")
		fmt.Printf("  RX: %s\n", formatBytes(stats.NetworkRxBytes))
		fmt.Printf("  TX: %s\n", formatBytes(stats.NetworkTxBytes))
	}

	return nil
}
