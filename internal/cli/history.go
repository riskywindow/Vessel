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

var historyCmd = &cobra.Command{
	Use:   "history <app>",
	Short: "Show deploy history",
	Long: `Show the deploy history for an application.

Displays all previous deploys with their version, image, status, and timestamp.

Examples:
  vessel history myapp
  vessel history myapp --limit 10`,
	Args: cobra.ExactArgs(1),
	RunE: runHistory,
}

func init() {
	historyCmd.Flags().Int("limit", 20, "maximum number of deploys to show")
}

func runHistory(cmd *cobra.Command, args []string) error {
	appName := args[0]
	limit, _ := cmd.Flags().GetInt("limit")

	client := daemon.NewClient("")

	var deploys []*store.Deploy
	err := client.Call("apps.history", map[string]string{"name": appName}, &deploys)
	if err != nil {
		return fmt.Errorf("failed to get deploy history: %w", err)
	}

	if len(deploys) == 0 {
		fmt.Printf("No deploy history for %s.\n", appName)
		return nil
	}

	// Apply limit
	if limit > 0 && len(deploys) > limit {
		deploys = deploys[:limit]
	}

	if jsonOutput {
		data, _ := json.MarshalIndent(deploys, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	// Table header
	fmt.Printf("%-8s %-12s %-30s %-10s %-16s %-8s %s\n",
		styles.TableHeader.Render("VERSION"),
		styles.TableHeader.Render("STATUS"),
		styles.TableHeader.Render("IMAGE"),
		styles.TableHeader.Render("STRATEGY"),
		styles.TableHeader.Render("DEPLOYED"),
		styles.TableHeader.Render("DURATION"),
		styles.TableHeader.Render("NOTES"))
	fmt.Println(strings.Repeat("-", 100))

	for _, d := range deploys {
		image := d.Image
		if len(image) > 30 {
			image = image[:27] + "..."
		}

		deployed := formatTimeAgo(d.CreatedAt)
		duration := "—"
		if d.FinishedAt != nil {
			dur := d.FinishedAt.Sub(d.CreatedAt)
			duration = formatDuration(dur)
		}

		notes := ""
		if d.RollbackOf != nil {
			notes = fmt.Sprintf("(rollback of v%d)", *d.RollbackOf)
		}
		if d.Status == store.DeployStatusFailed && d.Error != "" {
			errMsg := d.Error
			if len(errMsg) > 40 {
				errMsg = errMsg[:37] + "..."
			}
			notes = styles.Error.Render(fmt.Sprintf("(%s)", errMsg))
		}

		// Version with color
		versionStr := styles.Version.Render(fmt.Sprintf("v%d", d.Version))

		// Status with color
		var statusStr string
		switch d.Status {
		case store.DeployStatusActive:
			statusStr = styles.Success.Render(string(d.Status))
		case store.DeployStatusFailed:
			statusStr = styles.Error.Render(string(d.Status))
		case store.DeployStatusRolledBack:
			statusStr = styles.Warning.Render(string(d.Status))
		case store.DeployStatusPending:
			statusStr = styles.Muted.Render(string(d.Status))
		default:
			statusStr = string(d.Status)
		}

		fmt.Printf("%-8s %-12s %-30s %-10s %-16s %-8s %s\n",
			versionStr,
			statusStr,
			image,
			string(d.Strategy),
			deployed,
			duration,
			notes,
		)
	}

	return nil
}

// formatDuration returns a human-readable duration string.
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	mins := int(d.Minutes())
	secs := int(d.Seconds()) % 60
	return fmt.Sprintf("%dm%ds", mins, secs)
}
