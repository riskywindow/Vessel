package cli

import "github.com/spf13/cobra"

var stopCmd = &cobra.Command{
	Use:   "stop <app>",
	Short: "Stop an application",
	Long: `Stop all containers for an application.

The containers are gracefully stopped with SIGTERM, then SIGKILL after the grace period.

Examples:
  vessel stop myapp
  vessel stop myapp --grace-period 60s`,
	Args: cobra.ExactArgs(1),
	RunE: notImplemented,
}

func init() {
	stopCmd.Flags().Duration("grace-period", 0, "override the configured grace period")
}
