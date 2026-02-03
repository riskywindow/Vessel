package cli

import "github.com/spf13/cobra"

var statsCmd = &cobra.Command{
	Use:   "stats [app]",
	Short: "Show resource usage statistics",
	Long: `Show resource usage statistics for applications.

Displays CPU, memory, network, and process count metrics.

Examples:
  vessel stats              # Stats for all apps
  vessel stats myapp        # Stats for 'myapp'
  vessel stats --watch      # Continuously update stats`,
	RunE: notImplemented,
}

func init() {
	statsCmd.Flags().BoolP("watch", "w", false, "continuously update stats")
	statsCmd.Flags().Duration("interval", 0, "update interval when watching")
}
