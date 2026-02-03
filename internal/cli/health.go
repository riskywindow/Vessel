package cli

import "github.com/spf13/cobra"

var healthCmd = &cobra.Command{
	Use:   "health [app]",
	Short: "Show health status",
	Long: `Show health check status for applications.

If an app name is provided, shows detailed health info for that app.
Otherwise, shows a summary for all apps.

Examples:
  vessel health              # Summary for all apps
  vessel health myapp        # Detailed health for 'myapp'`,
	RunE: notImplemented,
}
