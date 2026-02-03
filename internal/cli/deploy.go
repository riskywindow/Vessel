package cli

import "github.com/spf13/cobra"

var deployCmd = &cobra.Command{
	Use:   "deploy [app]",
	Short: "Deploy an application",
	Long: `Deploy an application from the vessel.toml configuration file.

If an app name is provided, only that app is deployed.
If no app name is provided, all apps in the config are deployed.

Examples:
  vessel deploy              # Deploy all apps
  vessel deploy myapp        # Deploy only 'myapp'
  vessel deploy --all        # Explicitly deploy all apps`,
	RunE: notImplemented,
}

func init() {
	deployCmd.Flags().Bool("all", false, "deploy all apps in config")
}
