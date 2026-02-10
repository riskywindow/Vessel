package cli

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/vessel/vessel/internal/mcp"
	"github.com/vessel/vessel/internal/paths"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Run MCP server for AI assistant integration",
	Long: `Start the Model Context Protocol (MCP) server for Vessel.

This allows AI assistants like Claude to manage Vessel deployments
through natural language. The MCP server communicates over stdio.

To use with Claude Desktop, add to claude_desktop_config.json:

  {
    "mcpServers": {
      "vessel": {
        "command": "/path/to/vessel",
        "args": ["mcp"]
      }
    }
  }

Available tools:
  vessel_list_apps      - List all deployed applications
  vessel_get_app        - Get detailed app information
  vessel_deploy         - Deploy a new application
  vessel_stop           - Stop an application
  vessel_restart        - Restart an application
  vessel_remove         - Remove an application
  vessel_rollback       - Rollback to a previous version
  vessel_logs           - Get container logs
  vessel_health         - Get health status
  vessel_metrics        - Get resource usage metrics
  vessel_system_info    - Get system information
  vessel_deploy_history - Get deployment history`,
	RunE: runMCP,
}

var mcpSocketPath string

func init() {
	mcpCmd.Flags().StringVar(&mcpSocketPath, "socket", "", "Daemon socket path")
}

func runMCP(cmd *cobra.Command, args []string) error {
	// Log to stderr (stdout is reserved for MCP protocol)
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	}))

	socketPath := mcpSocketPath
	if socketPath == "" {
		socketPath = paths.SocketPath
	}

	server := mcp.NewServer(socketPath, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	return server.Run(ctx)
}
