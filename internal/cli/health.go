package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/vessel/vessel/internal/daemon"
	"github.com/vessel/vessel/internal/health"
	"github.com/vessel/vessel/internal/store"
)

var healthCmd = &cobra.Command{
	Use:   "health [app]",
	Short: "Show health status",
	Long: `Show health check status for applications.

If an app name is provided, shows detailed health info for that app's containers.
Otherwise, shows a summary for all monitored containers.

Examples:
  vessel health              # Summary for all monitored containers
  vessel health myapp        # Detailed health for 'myapp'`,
	Args: cobra.MaximumNArgs(1),
	RunE: runHealth,
}

func runHealth(cmd *cobra.Command, args []string) error {
	client := daemon.NewClient("")

	if len(args) == 1 {
		return showAppHealth(client, args[0])
	}

	return showAllHealth(client)
}

func showAllHealth(client *daemon.Client) error {
	// Get all health statuses
	var statuses map[string]*health.ContainerHealth
	if err := client.Call("health.all", nil, &statuses); err != nil {
		return fmt.Errorf("failed to get health status: %w", err)
	}

	if len(statuses) == 0 {
		fmt.Println("No containers being monitored.")
		return nil
	}

	if jsonOutput {
		data, _ := json.MarshalIndent(statuses, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	fmt.Printf("%-14s %-12s %-10s %-8s %-24s %s\n",
		"CONTAINER", "APP", "STATUS", "FAILS", "LAST CHECK", "MESSAGE")
	fmt.Println(strings.Repeat("-", 85))

	for _, ch := range statuses {
		containerID := ch.ContainerID
		if len(containerID) > 12 {
			containerID = containerID[:12]
		}
		appID := ch.AppID
		if len(appID) > 10 {
			appID = appID[:10]
		}

		statusStr := string(ch.Status)
		lastCheck := "—"
		if !ch.LastCheck.IsZero() {
			lastCheck = ch.LastCheck.Format(time.RFC3339)
		}
		message := ch.Message
		if len(message) > 30 {
			message = message[:27] + "..."
		}

		fmt.Printf("%-14s %-12s %-10s %-8d %-24s %s\n",
			containerID, appID, statusStr, ch.ConsecutiveFails, lastCheck, message)
	}

	return nil
}

func showAppHealth(client *daemon.Client, appName string) error {
	// Get containers for the app
	var containers []*store.Container
	if err := client.Call("containers.list", map[string]string{"app_name": appName}, &containers); err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	if len(containers) == 0 {
		return fmt.Errorf("no containers found for app '%s'", appName)
	}

	type containerHealthInfo struct {
		Container *store.Container        `json:"container"`
		Health    *health.ContainerHealth `json:"health,omitempty"`
	}

	var results []containerHealthInfo
	for _, c := range containers {
		info := containerHealthInfo{Container: c}

		var ch health.ContainerHealth
		if err := client.Call("health.status", map[string]string{"container_id": c.ID}, &ch); err == nil {
			info.Health = &ch
		}

		results = append(results, info)
	}

	if jsonOutput {
		data, _ := json.MarshalIndent(results, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	fmt.Printf("Health status for app '%s':\n\n", appName)
	fmt.Printf("%-14s %-10s %-10s %-8s %-24s %s\n",
		"CONTAINER", "STATE", "HEALTH", "FAILS", "LAST CHECK", "MESSAGE")
	fmt.Println(strings.Repeat("-", 80))

	for _, r := range results {
		containerID := r.Container.ID
		if len(containerID) > 12 {
			containerID = containerID[:12]
		}

		healthStr := "—"
		fails := 0
		lastCheck := "—"
		message := "—"

		if r.Health != nil {
			healthStr = string(r.Health.Status)
			fails = r.Health.ConsecutiveFails
			if !r.Health.LastCheck.IsZero() {
				lastCheck = r.Health.LastCheck.Format(time.RFC3339)
			}
			if r.Health.Message != "" {
				message = r.Health.Message
				if len(message) > 30 {
					message = message[:27] + "..."
				}
			}
		}

		fmt.Printf("%-14s %-10s %-10s %-8d %-24s %s\n",
			containerID, r.Container.State, healthStr, fails, lastCheck, message)
	}

	return nil
}
