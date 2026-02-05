package cli

import (
	"fmt"

	"github.com/spf13/cobra"
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
	Args: cobra.ExactArgs(1),
	RunE: runStop,
}

func init() {
	stopCmd.Flags().Duration("grace-period", 0, "override the configured grace period")
}

func runStop(cmd *cobra.Command, args []string) error {
	appName := args[0]

	client := daemon.NewClient("")
	fmt.Printf("Stopping %s... ", appName)

	var result map[string]string
	if err := client.Call("apps.stop", map[string]string{"name": appName}, &result); err != nil {
		fmt.Println("failed")
		return err
	}

	fmt.Println("done")
	return nil
}
