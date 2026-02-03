package cli

import "github.com/spf13/cobra"

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check system prerequisites",
	Long: `Check that the system meets all prerequisites for running Vessel.

Checks:
  - Linux kernel version
  - cgroups v2 support
  - Namespace support
  - OverlayFS support
  - Required permissions

Examples:
  vessel doctor`,
	RunE: notImplemented,
}
