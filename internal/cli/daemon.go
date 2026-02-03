package cli

import "github.com/spf13/cobra"

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Start the Vessel daemon",
	Long: `Start the Vessel daemon process.

The daemon manages container lifecycles, health monitoring, and the API server.

Examples:
  vessel daemon                  # Run in foreground
  vessel daemon --detach         # Run in background`,
	RunE: notImplemented,
}

func init() {
	daemonCmd.Flags().BoolP("detach", "d", false, "run daemon in background")
	daemonCmd.Flags().String("socket", "", "Unix socket path")
	daemonCmd.Flags().String("api-address", "", "API listen address")
	daemonCmd.Flags().Int("api-port", 0, "API listen port")
}
