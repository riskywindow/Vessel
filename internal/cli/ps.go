package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/vessel/vessel/internal/cli/styles"
	"github.com/vessel/vessel/internal/daemon"
	"github.com/vessel/vessel/internal/store"
)

var psCmd = &cobra.Command{
	Use:   "ps",
	Short: "List running applications",
	Long: `List all running applications and their containers.

Examples:
  vessel ps              # List all apps
  vessel ps --all        # Include stopped apps`,
	RunE: runPs,
}

func init() {
	psCmd.Flags().BoolP("all", "a", false, "show all apps including stopped")
}

func runPs(cmd *cobra.Command, args []string) error {
	showAll, _ := cmd.Flags().GetBool("all")

	client := daemon.NewClient("")
	var apps []*store.App
	if err := client.Call("apps.list", nil, &apps); err != nil {
		return fmt.Errorf("failed to list apps: %w", err)
	}

	if len(apps) == 0 {
		fmt.Println("No applications found.")
		return nil
	}

	if jsonOutput {
		filtered := filterApps(apps, showAll)
		data, err := json.MarshalIndent(filtered, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	// Table header
	fmt.Printf("%-16s %-12s %-12s %-16s %-24s %-12s\n",
		styles.TableHeader.Render("NAME"),
		styles.TableHeader.Render("STATUS"),
		styles.TableHeader.Render("INSTANCES"),
		styles.TableHeader.Render("IP"),
		styles.TableHeader.Render("IMAGE"),
		styles.TableHeader.Render("UPTIME"))
	fmt.Println(strings.Repeat("-", 96))

	for _, app := range apps {
		if !showAll && app.State == store.AppStateStopped {
			continue
		}

		// Get container info for instance count and IPs
		var containers []*store.Container
		client.Call("containers.list", map[string]string{"app_name": app.Name}, &containers)

		running := 0
		for _, c := range containers {
			if c.State == store.ContainerStateRunning {
				running++
			}
		}

		// Status with color
		var statusStr string
		switch app.State {
		case store.AppStateRunning:
			statusStr = styles.Success.Render(string(app.State))
		case store.AppStateStopped:
			statusStr = styles.Muted.Render(string(app.State))
		case store.AppStateFailed:
			statusStr = styles.Error.Render(string(app.State))
		case store.AppStateDeploying:
			statusStr = styles.Warning.Render(string(app.State))
		default:
			statusStr = string(app.State)
		}

		// Instance count with color
		instanceStr := fmt.Sprintf("%d/%d", running, app.Instances)
		if running == app.Instances && app.Instances > 0 {
			instanceStr = styles.Success.Render(instanceStr)
		} else if running == 0 {
			instanceStr = styles.Error.Render(instanceStr)
		} else {
			instanceStr = styles.Warning.Render(instanceStr)
		}

		// App name styled
		appNameStr := styles.AppName.Render(truncate(app.Name, 16))

		// Collect container IP for display
		ip := "-"
		if len(containers) > 0 && containers[0].IP != "" {
			ip = containers[0].IP
			if len(containers) > 1 {
				ip = fmt.Sprintf("%s (+%d)", ip, len(containers)-1)
			}
		}

		uptime := formatUptime(app.CreatedAt)
		if app.State == store.AppStateStopped {
			uptime = "—"
		}

		image := app.Image
		if len(image) > 24 {
			image = image[:21] + "..."
		}

		fmt.Printf("%-16s %-12s %-12s %-16s %-24s %-12s\n",
			appNameStr,
			statusStr,
			instanceStr,
			ip,
			image,
			uptime,
		)
	}

	return nil
}

func filterApps(apps []*store.App, showAll bool) []*store.App {
	if showAll {
		return apps
	}
	var filtered []*store.App
	for _, app := range apps {
		if app.State != store.AppStateStopped {
			filtered = append(filtered, app)
		}
	}
	return filtered
}

func formatUptime(created time.Time) string {
	dur := time.Since(created)
	if dur < time.Minute {
		return fmt.Sprintf("%ds", int(dur.Seconds()))
	}
	if dur < time.Hour {
		return fmt.Sprintf("%dm", int(dur.Minutes()))
	}
	if dur < 24*time.Hour {
		hours := int(dur.Hours())
		mins := int(dur.Minutes()) % 60
		return fmt.Sprintf("%dh %dm", hours, mins)
	}
	days := int(dur.Hours()) / 24
	hours := int(dur.Hours()) % 24
	return fmt.Sprintf("%dd %dh", days, hours)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
