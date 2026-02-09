package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vessel/vessel/internal/cli/styles"
	"github.com/vessel/vessel/internal/daemon"
	"github.com/vessel/vessel/internal/store"
)

var statsCmd = &cobra.Command{
	Use:   "stats <app>",
	Short: "Show resource usage statistics",
	Long: `Show resource usage statistics for an application's containers.

Displays CPU, memory, process count, and network metrics.
Uses time-series metrics collected by the metrics collector.
Falls back to live cgroup stats if metrics are unavailable.

Examples:
  vessel stats myapp           # Stats for all containers of an app`,
	Args: cobra.ExactArgs(1),
	RunE: runStats,
}

func init() {
	// statsCmd is registered in root.go
}

func runStats(cmd *cobra.Command, args []string) error {
	appName := args[0]

	client := daemon.NewClient("")

	// Get containers for the app
	var containers []*store.Container
	if err := client.Call("containers.list", map[string]string{"app_name": appName}, &containers); err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	if len(containers) == 0 {
		return fmt.Errorf("no containers found for app '%s'", appName)
	}

	if jsonOutput {
		var results []map[string]interface{}
		for _, c := range containers {
			if c.State != store.ContainerStateRunning {
				continue
			}
			stats := getContainerStats(client, c.ID)
			if stats == nil {
				continue
			}
			results = append(results, map[string]interface{}{
				"container_id": c.ID[:12],
				"stats":        stats,
			})
		}
		data, _ := json.MarshalIndent(results, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	// Table header
	fmt.Printf("%-14s %-8s %-20s %-8s %-10s %-10s\n",
		styles.TableHeader.Render("CONTAINER"),
		styles.TableHeader.Render("CPU"),
		styles.TableHeader.Render("MEMORY"),
		styles.TableHeader.Render("PIDS"),
		styles.TableHeader.Render("NET RX"),
		styles.TableHeader.Render("NET TX"))
	fmt.Println(strings.Repeat("-", 74))

	for _, c := range containers {
		if c.State != store.ContainerStateRunning {
			continue
		}

		stats := getContainerStats(client, c.ID)
		if stats == nil {
			fmt.Printf("%-14s %-8s %-20s %-8s %-10s %-10s\n", c.ID[:12], "—", "—", "—", "—", "—")
			continue
		}

		cpuStr := fmt.Sprintf("%.1f%%", stats.CPUPercent)
		memStr := formatMemory(stats.MemoryBytes, stats.MemoryLimit)
		pidsStr := fmt.Sprintf("%d", stats.PidsCount)
		if c.Resources.PidsMax > 0 {
			pidsStr = fmt.Sprintf("%d/%d", stats.PidsCount, c.Resources.PidsMax)
		}
		rxStr := humanBytes(stats.NetworkRxBytes)
		txStr := humanBytes(stats.NetworkTxBytes)

		fmt.Printf("%-14s %-8s %-20s %-8s %-10s %-10s\n",
			styles.ContainerID.Render(c.ID[:12]), cpuStr, memStr, pidsStr, rxStr, txStr)
	}

	return nil
}

// getContainerStats tries metrics.latest first, falls back to containers.stats (live cgroup).
func getContainerStats(client *daemon.Client, containerID string) *store.MetricPoint {
	// Try stored metrics first
	var metric store.MetricPoint
	if err := client.Call("metrics.latest", map[string]string{"container_id": containerID}, &metric); err == nil {
		return &metric
	}

	// Fall back to live cgroup stats
	if err := client.Call("containers.stats", map[string]string{"container_id": containerID}, &metric); err == nil {
		return &metric
	}

	return nil
}

func formatMemory(current, limit int64) string {
	currentStr := formatBytes(current)
	if limit > 0 {
		return fmt.Sprintf("%s/%s", currentStr, formatBytes(limit))
	}
	return currentStr
}

// humanBytes formats byte counts in human-readable form.
func humanBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}
