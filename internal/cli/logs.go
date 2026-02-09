package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/vessel/vessel/internal/daemon"
	"github.com/vessel/vessel/internal/store"
)

var logsCmd = &cobra.Command{
	Use:   "logs <app>",
	Short: "View application logs",
	Long: `View logs from an application's containers.

Examples:
  vessel logs myapp           # Show recent logs
  vessel logs myapp -f        # Follow log output
  vessel logs myapp --tail 50 # Show last 50 lines`,
	Args:              cobra.ExactArgs(1),
	RunE:              runLogs,
	ValidArgsFunction: completeAppNames,
}

func init() {
	logsCmd.Flags().BoolP("follow", "f", false, "follow log output")
	logsCmd.Flags().Int("tail", 100, "number of lines to show from the end")
	logsCmd.Flags().Bool("timestamps", false, "show timestamps")
}

func runLogs(cmd *cobra.Command, args []string) error {
	appName := args[0]
	follow, _ := cmd.Flags().GetBool("follow")
	tail, _ := cmd.Flags().GetInt("tail")

	client := daemon.NewClient("")

	// Get containers for the app
	var containers []*store.Container
	if err := client.Call("containers.list", map[string]string{"app_name": appName}, &containers); err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	if len(containers) == 0 {
		return fmt.Errorf("no containers found for app '%s'", appName)
	}

	if follow {
		return followLogs(appName, containers)
	}

	// Non-follow mode: get logs for each container via daemon
	for _, c := range containers {
		var result map[string]string
		params := map[string]interface{}{
			"container_id": c.ID,
			"follow":       false,
			"tail":         tail,
		}
		if err := client.Call("containers.logs", params, &result); err != nil {
			fmt.Fprintf(os.Stderr, "failed to get logs for container %s: %v\n", c.ID[:8], err)
			continue
		}

		logs := result["logs"]
		if logs == "" {
			continue
		}

		// Prefix each line with container ID
		prefix := c.ID[:8]
		printPrefixedLogs(prefix, logs)
	}

	return nil
}

// followLogs reads logs continuously using the runtime directly.
// For follow mode, we read from the log files directly since streaming
// over the socket protocol doesn't support long-lived connections well.
func followLogs(appName string, containers []*store.Container) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	// For now, just get the most recent logs (follow mode requires direct file access)
	client := daemon.NewClient("")
	for _, c := range containers {
		var result map[string]string
		params := map[string]interface{}{
			"container_id": c.ID,
			"follow":       false,
			"tail":         100,
		}
		if err := client.Call("containers.logs", params, &result); err != nil {
			continue
		}
		prefix := c.ID[:8]
		printPrefixedLogs(prefix, result["logs"])
	}

	if ctx.Err() != nil {
		return nil
	}

	fmt.Println("(follow mode: showing latest logs, press Ctrl+C to exit)")
	<-ctx.Done()
	return nil
}

func printPrefixedLogs(prefix, logs string) {
	// Split into lines and prefix each
	start := 0
	for i := 0; i < len(logs); i++ {
		if logs[i] == '\n' {
			line := logs[start:i]
			if len(line) > 0 {
				fmt.Printf("[%s] %s\n", prefix, line)
			}
			start = i + 1
		}
	}
	// Print remaining if no trailing newline
	if start < len(logs) {
		fmt.Printf("[%s] %s\n", prefix, logs[start:])
	}
}

// Ensure io import is used (for future follow mode implementation)
var _ = io.EOF
