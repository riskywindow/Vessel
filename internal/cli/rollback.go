package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"
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
	Args: cobra.ExactArgs(1),
	RunE: runRollback,
}

func init() {
	rollbackCmd.Flags().Int("version", 0, "target version to rollback to (0 = previous)")
}

func runRollback(cmd *cobra.Command, args []string) error {
	appName := args[0]
	version, _ := cmd.Flags().GetInt("version")

	client := daemon.NewClient("")

	if version > 0 {
		fmt.Printf("Rolling back %s to v%d...\n", appName, version)
	} else {
		fmt.Printf("Rolling back %s to previous version...\n", appName)
	}

	params := map[string]interface{}{
		"name":    appName,
		"version": version,
	}

	var deploy store.Deploy
	err := client.Call("apps.rollback", params, &deploy)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  x Rollback failed: %v\n", err)
		return err
	}

	if jsonOutput {
		data, _ := json.MarshalIndent(deploy, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	rollbackFrom := "previous"
	if deploy.RollbackOf != nil {
		rollbackFrom = "v" + strconv.Itoa(*deploy.RollbackOf)
	}

	fmt.Printf("  > %s rolled back to v%d (from %s, now v%d)\n",
		appName, deploy.Version-1, rollbackFrom, deploy.Version)
	return nil
}
