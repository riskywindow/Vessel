package cli

import "github.com/spf13/cobra"

var rollbackCmd = &cobra.Command{
	Use:   "rollback <app> [version]",
	Short: "Rollback to a previous deploy",
	Long: `Rollback an application to a previous deploy version.

If no version is specified, rolls back to the previous version.

Examples:
  vessel rollback myapp       # Rollback to previous version
  vessel rollback myapp 3     # Rollback to version 3`,
	Args: cobra.RangeArgs(1, 2),
	RunE: notImplemented,
}
