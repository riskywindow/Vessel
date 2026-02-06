package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/vessel/vessel/internal/config"
)

var fmtCmd = &cobra.Command{
	Use:   "fmt [file]",
	Short: "Validate a configuration file",
	Long: `Validate a vessel.toml configuration file and report any errors.

By default, operates on vessel.toml in the current directory.

Examples:
  vessel fmt                     # Validate vessel.toml
  vessel fmt app.toml            # Validate specific file`,
	RunE: runFmt,
}

func init() {
	// No flags needed for validation-only mode
}

func runFmt(cmd *cobra.Command, args []string) error {
	filePath := configFile
	if len(args) > 0 {
		filePath = args[0]
	}

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("%s not found", filePath)
	}

	// Parse without defaults/validation first to check TOML syntax
	cfg, err := config.Parse(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s has errors:\n", filePath)
		fmt.Fprintf(os.Stderr, "  %v\n", err)
		return fmt.Errorf("validation failed")
	}

	// Apply defaults so validation can check fully resolved values
	config.ApplyDefaults(cfg)

	// Validate
	errs := config.Validate(cfg)
	if len(errs) > 0 {
		fmt.Fprintf(os.Stderr, "%s has errors:\n", filePath)
		for _, e := range errs {
			fmt.Fprintf(os.Stderr, "  %v\n", e)
		}
		return fmt.Errorf("validation failed with %d errors", len(errs))
	}

	fmt.Printf("%s is valid\n", filePath)
	return nil
}
