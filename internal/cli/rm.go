package cli

import "github.com/spf13/cobra"

var rmCmd = &cobra.Command{
	Use:   "rm <app>",
	Short: "Remove an application",
	Long: `Remove an application and all its containers.

This stops any running containers and removes all state for the app.

Examples:
  vessel rm myapp
  vessel rm myapp --force     # Remove without confirmation`,
	Args: cobra.ExactArgs(1),
	RunE: notImplemented,
}

func init() {
	rmCmd.Flags().BoolP("force", "f", false, "force removal without confirmation")
}
