package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	"github.com/vessel/vessel/internal/daemon"
)

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Start the Vessel daemon",
	Long: `Start the Vessel daemon process.

The daemon manages container lifecycles, health monitoring, and the API server.

Examples:
  vessel daemon                  # Run in foreground`,
	RunE: runDaemon,
}

func init() {
	daemonCmd.Flags().String("socket", "", "Unix socket path")
}

func runDaemon(cmd *cobra.Command, args []string) error {
	if os.Getuid() != 0 {
		return fmt.Errorf("vessel daemon requires root privileges")
	}

	// Set up logger
	logLevel := slog.LevelInfo
	if verbose {
		logLevel = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel}))

	// Check if daemon is already running
	if daemon.IsRunning() {
		return fmt.Errorf("vessel daemon is already running")
	}

	d, err := daemon.NewDaemon(logger)
	if err != nil {
		return fmt.Errorf("failed to create daemon: %w", err)
	}

	ctx := context.Background()
	return d.Start(ctx)
}
