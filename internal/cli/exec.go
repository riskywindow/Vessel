package cli

import "github.com/spf13/cobra"

var execCmd = &cobra.Command{
	Use:   "exec <app> -- <command>",
	Short: "Execute a command in a container",
	Long: `Execute a command inside a running container.

Examples:
  vessel exec myapp -- /bin/sh
  vessel exec myapp -- ls -la /app
  vessel exec myapp -it -- /bin/bash`,
	Args: cobra.MinimumNArgs(1),
	RunE: notImplemented,
}

func init() {
	execCmd.Flags().BoolP("interactive", "i", false, "keep stdin open")
	execCmd.Flags().BoolP("tty", "t", false, "allocate a TTY")
}
