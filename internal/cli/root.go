// Package cli implements the Vessel command-line interface.
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	// Global flags
	configFile string
	jsonOutput bool
	verbose    bool
)

// rootCmd is the base command for vessel.
var rootCmd = &cobra.Command{
	Use:   "vessel",
	Short: "A single-binary container orchestrator",
	Long: `Vessel is a single-binary container orchestrator that deploys apps
on any Linux server using raw Linux primitives (namespaces, cgroups v2, OverlayFS).

Kubernetes is for Google. Vessel is for the rest of us.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "vessel.toml", "config file path")
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "output in JSON format")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	// Register subcommands
	rootCmd.AddCommand(deployCmd)
	rootCmd.AddCommand(psCmd)
	rootCmd.AddCommand(logsCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(rmCmd)
	rootCmd.AddCommand(rollbackCmd)
	rootCmd.AddCommand(historyCmd)
	rootCmd.AddCommand(healthCmd)
	rootCmd.AddCommand(statsCmd)
	rootCmd.AddCommand(secretCmd)
	rootCmd.AddCommand(execCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(doctorCmd)
	rootCmd.AddCommand(daemonCmd)
	rootCmd.AddCommand(fmtCmd)
	rootCmd.AddCommand(networkCmd)
	rootCmd.AddCommand(certCmd)

	// Register network subcommands
	networkCmd.AddCommand(networkStatusCmd)
	networkCmd.AddCommand(networkListCmd)
}

// notImplemented returns a RunE function that prints "not yet implemented".
func notImplemented(cmd *cobra.Command, args []string) error {
	fmt.Println("not yet implemented")
	return nil
}
