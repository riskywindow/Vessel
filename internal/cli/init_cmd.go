package cli

import "github.com/spf13/cobra"

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new vessel.toml",
	Long: `Initialize a new vessel.toml configuration file.

Creates a starter configuration file with example settings.

Examples:
  vessel init                    # Create vessel.toml in current directory
  vessel init --output app.toml  # Create with custom filename`,
	RunE: notImplemented,
}

func init() {
	initCmd.Flags().StringP("output", "o", "vessel.toml", "output file path")
}
