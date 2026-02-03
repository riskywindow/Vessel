package cli

import "github.com/spf13/cobra"

var logsCmd = &cobra.Command{
	Use:   "logs <app>",
	Short: "View application logs",
	Long: `View logs from an application's containers.

Examples:
  vessel logs myapp           # Show recent logs
  vessel logs myapp -f        # Follow log output
  vessel logs myapp --tail 50 # Show last 50 lines`,
	Args: cobra.ExactArgs(1),
	RunE: notImplemented,
}

func init() {
	logsCmd.Flags().BoolP("follow", "f", false, "follow log output")
	logsCmd.Flags().Int("tail", 100, "number of lines to show from the end")
	logsCmd.Flags().Bool("timestamps", false, "show timestamps")
}
