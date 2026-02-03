package cli

import "github.com/spf13/cobra"

var psCmd = &cobra.Command{
	Use:   "ps",
	Short: "List running applications",
	Long: `List all running applications and their containers.

Examples:
  vessel ps              # List all apps
  vessel ps --all        # Include stopped apps`,
	RunE: notImplemented,
}

func init() {
	psCmd.Flags().BoolP("all", "a", false, "show all apps including stopped")
}
