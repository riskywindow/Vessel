package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/vessel/vessel/internal/cli/progress"
	"github.com/vessel/vessel/internal/daemon"
)

var stopCmd = &cobra.Command{
	Use:   "stop <app>",
	Short: "Stop an application",
	Long: `Stop all containers for an application.

The containers are gracefully stopped with SIGTERM, then SIGKILL after the grace period.

Examples:
  vessel stop myapp
  vessel stop myapp --grace-period 60s`,
	Args:              cobra.ExactArgs(1),
	RunE:              runStop,
	ValidArgsFunction: completeAppNames,
}

func init() {
	stopCmd.Flags().Duration("grace-period", 0, "override the configured grace period")
}

func runStop(cmd *cobra.Command, args []string) error {
	appName := args[0]

	client := daemon.NewClient("")

	spin := progress.NewSpinner(fmt.Sprintf("Stopping %s", appName))
	spin.Start()

	var result map[string]string
	if err := client.Call("apps.stop", map[string]string{"name": appName}, &result); err != nil {
		spin.Fail(fmt.Sprintf("Failed to stop %s: %v", appName, err))
		return err
	}

	spin.Success(fmt.Sprintf("%s stopped", appName))
	return nil
}
