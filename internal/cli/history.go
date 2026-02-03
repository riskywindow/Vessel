package cli

import "github.com/spf13/cobra"

var historyCmd = &cobra.Command{
	Use:   "history <app>",
	Short: "Show deploy history",
	Long: `Show the deploy history for an application.

Displays all previous deploys with their version, image, status, and timestamp.

Examples:
  vessel history myapp
  vessel history myapp --limit 10`,
	Args: cobra.ExactArgs(1),
	RunE: notImplemented,
}

func init() {
	historyCmd.Flags().Int("limit", 20, "maximum number of deploys to show")
}
