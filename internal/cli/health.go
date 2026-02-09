package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/vessel/vessel/internal/cli/styles"
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
  vessel health myapp        # Detailed health for 'myapp'
  vessel health check <id>   # Run immediate health check
  vessel health history <id> # Show health check history`,
	Args: cobra.MaximumNArgs(1),
	RunE: runHealth,
}

var healthCheckCmd = &cobra.Command{
	Use:   "check <container-id>",
	Short: "Run an immediate health check on a container",
	Args:  cobra.ExactArgs(1),
	RunE:  runHealthCheck,
}

var healthHistoryCmd = &cobra.Command{
	Use:   "history <container-id>",
	Short: "Show health check history for a container",
	Args:  cobra.ExactArgs(1),
	RunE:  runHealthHistory,
}

var healthWatchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Watch health status continuously",
	RunE:  runHealthWatch,
}

func init() {
	healthCmd.AddCommand(healthCheckCmd)
	healthCmd.AddCommand(healthHistoryCmd)
	healthCmd.AddCommand(healthWatchCmd)
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
		styles.TableHeader.Render("CONTAINER"),
		styles.TableHeader.Render("APP"),
		styles.TableHeader.Render("STATUS"),
		styles.TableHeader.Render("FAILS"),
		styles.TableHeader.Render("LAST CHECK"),
		styles.TableHeader.Render("MESSAGE"))
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

		statusStr := styledHealthStatus(string(ch.Status))
		lastCheck := "—"
		if !ch.LastCheck.IsZero() {
			lastCheck = ch.LastCheck.Format(time.RFC3339)
		}
		message := ch.Message
		if len(message) > 30 {
			message = message[:27] + "..."
		}

		fmt.Printf("%-14s %-12s %-10s %-8d %-24s %s\n",
			styles.ContainerID.Render(containerID), appID, statusStr, ch.ConsecutiveFails, lastCheck, message)
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
		Container *store.Container       `json:"container"`
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

	fmt.Printf("Health status for app '%s':\n\n", styles.AppName.Render(appName))
	fmt.Printf("%-14s %-10s %-10s %-8s %-24s %s\n",
		styles.TableHeader.Render("CONTAINER"),
		styles.TableHeader.Render("STATE"),
		styles.TableHeader.Render("HEALTH"),
		styles.TableHeader.Render("FAILS"),
		styles.TableHeader.Render("LAST CHECK"),
		styles.TableHeader.Render("MESSAGE"))
	fmt.Println(strings.Repeat("-", 80))

	for _, r := range results {
		containerID := r.Container.ID
		if len(containerID) > 12 {
			containerID = containerID[:12]
		}

		healthStr := styles.Muted.Render("—")
		fails := 0
		lastCheck := "—"
		message := "—"

		if r.Health != nil {
			healthStr = styledHealthStatus(string(r.Health.Status))
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

		stateStr := styledContainerState(string(r.Container.State))

		fmt.Printf("%-14s %-10s %-10s %-8d %-24s %s\n",
			styles.ContainerID.Render(containerID), stateStr, healthStr, fails, lastCheck, message)
	}

	return nil
}

func runHealthCheck(cmd *cobra.Command, args []string) error {
	containerID := args[0]

	client := daemon.NewClient("")

	var result store.HealthResult
	if err := client.Call("health.check_now", map[string]string{"container_id": containerID}, &result); err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}

	if jsonOutput {
		data, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	shortID := containerID
	if len(shortID) > 12 {
		shortID = shortID[:12]
	}

	fmt.Printf("Container: %s\n", styles.ContainerID.Render(shortID))
	fmt.Printf("Status:    %s\n", styledHealthStatus(string(result.Status)))
	fmt.Printf("Duration:  %v\n", result.Duration.Round(time.Millisecond))
	if result.Message != "" {
		fmt.Printf("Message:   %s\n", result.Message)
	}

	return nil
}

func runHealthHistory(cmd *cobra.Command, args []string) error {
	containerID := args[0]

	client := daemon.NewClient("")

	var results []*store.HealthResult
	if err := client.Call("health.history", map[string]interface{}{
		"container_id": containerID,
		"limit":        20,
	}, &results); err != nil {
		return fmt.Errorf("failed to get health history: %w", err)
	}

	if len(results) == 0 {
		fmt.Println("No health check history found.")
		return nil
	}

	if jsonOutput {
		data, _ := json.MarshalIndent(results, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	shortID := containerID
	if len(shortID) > 12 {
		shortID = shortID[:12]
	}

	fmt.Printf("Health history for container %s:\n\n", styles.ContainerID.Render(shortID))
	fmt.Printf("%-20s %-12s %-12s %s\n",
		styles.TableHeader.Render("TIME"),
		styles.TableHeader.Render("STATUS"),
		styles.TableHeader.Render("DURATION"),
		styles.TableHeader.Render("MESSAGE"))
	fmt.Println(strings.Repeat("-", 70))

	for _, r := range results {
		statusStr := styledHealthStatus(string(r.Status))

		msg := r.Message
		if len(msg) > 30 {
			msg = msg[:27] + "..."
		}

		fmt.Printf("%-20s %-12s %-12s %s\n",
			r.CheckedAt.Format("15:04:05"),
			statusStr,
			r.Duration.Round(time.Millisecond).String(),
			msg,
		)
	}

	return nil
}

func runHealthWatch(cmd *cobra.Command, args []string) error {
	client := daemon.NewClient("")

	fmt.Println("Watching health status (Ctrl+C to stop)...")
	fmt.Println()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		// Clear screen and redraw
		fmt.Print("\033[H\033[2J")
		fmt.Printf("Health Status (updated %s)\n\n", time.Now().Format("15:04:05"))
		if err := showAllHealth(client); err != nil {
			fmt.Printf("Error: %v\n", err)
		}

		<-ticker.C
	}
}

// humanDuration formats a duration into a human-readable string.
func humanDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	return fmt.Sprintf("%dh", int(d.Hours()))
}

// styledHealthStatus returns a colored health status string.
func styledHealthStatus(status string) string {
	switch status {
	case "healthy":
		return styles.Success.Render(status)
	case "unhealthy":
		return styles.Error.Render(status)
	default:
		return styles.Muted.Render(status)
	}
}

// styledContainerState returns a colored container state string.
func styledContainerState(state string) string {
	switch state {
	case "running":
		return styles.Success.Render(state)
	case "stopped":
		return styles.Muted.Render(state)
	case "failed":
		return styles.Error.Render(state)
	default:
		return state
	}
}
