package cli

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/vessel/vessel/internal/cli/progress"
	"github.com/vessel/vessel/internal/daemon"
	"github.com/vessel/vessel/internal/store"
)

var rollbackCmd = &cobra.Command{
	Use:   "rollback <app>",
	Short: "Rollback to a previous deploy",
	Long: `Rollback an application to a previous deploy version.

If no version is specified, rolls back to the previous version.

Examples:
  vessel rollback myapp            # Rollback to previous version
  vessel rollback myapp --version 2  # Rollback to version 2`,
	Args:              cobra.ExactArgs(1),
	RunE:              runRollback,
	ValidArgsFunction: completeAppNames,
}

func init() {
	rollbackCmd.Flags().Int("version", 0, "target version to rollback to (0 = previous)")
}

func runRollback(cmd *cobra.Command, args []string) error {
	appName := args[0]
	version, _ := cmd.Flags().GetInt("version")

	client := daemon.NewClient("")

	spinMsg := fmt.Sprintf("Rolling back %s to previous version", appName)
	if version > 0 {
		spinMsg = fmt.Sprintf("Rolling back %s to v%d", appName, version)
	}

	if !jsonOutput {
		spin := progress.NewSpinner(spinMsg)
		spin.Start()
		defer spin.Stop()

		params := map[string]interface{}{
			"name":    appName,
			"version": version,
		}

		var deploy store.Deploy
		err := client.Call("apps.rollback", params, &deploy)
		if err != nil {
			spin.Fail(fmt.Sprintf("Rollback failed: %v", err))
			return err
		}

		rollbackFrom := "previous"
		if deploy.RollbackOf != nil {
			rollbackFrom = "v" + strconv.Itoa(*deploy.RollbackOf)
		}

		spin.Success(fmt.Sprintf("%s rolled back to v%d (from %s, now v%d)",
			appName, deploy.Version-1, rollbackFrom, deploy.Version))
		return nil
	}

	params := map[string]interface{}{
		"name":    appName,
		"version": version,
	}

	var deploy store.Deploy
	err := client.Call("apps.rollback", params, &deploy)
	if err != nil {
		return err
	}

	data, _ := json.MarshalIndent(deploy, "", "  ")
	fmt.Println(string(data))
	return nil
}
