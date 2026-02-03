package cli

import "github.com/spf13/cobra"

var fmtCmd = &cobra.Command{
	Use:   "fmt [file]",
	Short: "Format and validate configuration",
	Long: `Format and validate a vessel.toml configuration file.

By default, operates on vessel.toml in the current directory.

Examples:
  vessel fmt                     # Format vessel.toml
  vessel fmt app.toml            # Format specific file
  vessel fmt --check             # Check without modifying`,
	RunE: notImplemented,
}

func init() {
	fmtCmd.Flags().Bool("check", false, "check formatting without modifying")
	fmtCmd.Flags().Bool("diff", false, "show diff of changes")
}
