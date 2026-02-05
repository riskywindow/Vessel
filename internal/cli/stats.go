package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vessel/vessel/internal/daemon"
	"github.com/vessel/vessel/internal/store"
)

var statsCmd = &cobra.Command{
	Use:   "stats <app>",
	Short: "Show resource usage statistics",
	Long: `Show resource usage statistics for an application's containers.

Displays CPU, memory, and process count metrics from cgroups v2.

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
			var stats store.MetricPoint
			if err := client.Call("containers.stats", map[string]string{"container_id": c.ID}, &stats); err != nil {
				continue
			}
			results = append(results, map[string]interface{}{
				"container_id": c.ID[:8],
				"stats":        stats,
			})
		}
		data, _ := json.MarshalIndent(results, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	// Table header
	fmt.Printf("%-12s %-8s %-20s %-8s\n", "CONTAINER", "CPU", "MEMORY", "PIDS")
	fmt.Println(strings.Repeat("-", 52))

	for _, c := range containers {
		if c.State != store.ContainerStateRunning {
			continue
		}

		var stats store.MetricPoint
		if err := client.Call("containers.stats", map[string]string{"container_id": c.ID}, &stats); err != nil {
			fmt.Printf("%-12s %-8s %-20s %-8s\n", c.ID[:8], "—", "—", "—")
			continue
		}

		cpuStr := fmt.Sprintf("%.1f%%", stats.CPUPercent)
		memStr := formatMemory(stats.MemoryBytes, stats.MemoryLimit)
		pidsStr := fmt.Sprintf("%d", stats.PidsCount)
		if c.Resources.PidsMax > 0 {
			pidsStr = fmt.Sprintf("%d/%d", stats.PidsCount, c.Resources.PidsMax)
		}

		fmt.Printf("%-12s %-8s %-20s %-8s\n", c.ID[:8], cpuStr, memStr, pidsStr)
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
