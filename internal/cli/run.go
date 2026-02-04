package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/vessel/vessel/internal/runtime"
	"github.com/vessel/vessel/internal/store"
)

var (
	runName    string
	runEnv     []string
	runWorkDir string
	runRm      bool
	runMemory  string
	runCPU     float64
	runPids    int64
)

var runCmd = &cobra.Command{
	Use:   "run [flags] IMAGE [-- COMMAND [ARGS...]]",
	Short: "Run a container",
	Long: `Run a container with the specified image and optional command.

If no command is specified, the image's default entrypoint/cmd is used.

Examples:
  vessel run alpine:latest -- echo hello
  vessel run --name mycontainer alpine:latest -- sh -c "echo hello"
  vessel run --rm --env FOO=bar alpine:latest -- sh -c 'echo $FOO'
  vessel run nginx:alpine

  # Resource limits
  vessel run --memory 128MB alpine:latest -- sh -c 'echo hello'
  vessel run --cpu 0.5 alpine:latest -- sh -c 'while true; do :; done'
  vessel run --pids 10 alpine:latest -- sh -c 'ps aux'`,
	Args: cobra.MinimumNArgs(1),
	RunE: runContainer,
}

func init() {
	runCmd.Flags().StringVarP(&runName, "name", "n", "", "container name")
	runCmd.Flags().StringArrayVarP(&runEnv, "env", "e", nil, "environment variables (KEY=VALUE)")
	runCmd.Flags().StringVarP(&runWorkDir, "workdir", "w", "", "working directory inside container")
	runCmd.Flags().BoolVar(&runRm, "rm", false, "automatically remove container when it exits")
	runCmd.Flags().StringVarP(&runMemory, "memory", "m", "", "memory limit (e.g., 128MB, 1GB)")
	runCmd.Flags().Float64Var(&runCPU, "cpu", 0, "CPU limit (e.g., 0.5 for 50% of one core, 2.0 for two cores)")
	runCmd.Flags().Int64Var(&runPids, "pids", 0, "maximum number of processes")

	rootCmd.AddCommand(runCmd)
}

func runContainer(cmd *cobra.Command, args []string) error {
	// Parse image and command from args
	// First arg is the image, remaining args are the command
	image := args[0]
	var runCommand []string
	if len(args) > 1 {
		runCommand = args[1:]
	}

	// Check for root privileges
	if os.Getuid() != 0 {
		return fmt.Errorf("vessel run requires root privileges")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle SIGINT/SIGTERM for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Fprintln(os.Stderr, "\nReceived signal, stopping container...")
		cancel()
	}()

	// Create runtime
	rt, err := runtime.NewLinuxRuntime()
	if err != nil {
		return fmt.Errorf("failed to create runtime: %w", err)
	}

	// Set up environment - add TERM from host, then user-provided vars
	env := []string{
		"TERM=" + os.Getenv("TERM"),
	}
	env = append(env, runEnv...)

	// Parse resource limits
	resources := store.ResourceLimits{}
	if runMemory != "" {
		memBytes, err := parseMemoryLimit(runMemory)
		if err != nil {
			return fmt.Errorf("invalid memory limit: %w", err)
		}
		resources.MemoryMax = memBytes
	}
	if runCPU > 0 {
		// Convert CPU percentage to quota/period
		// CPU limit of 0.5 means 50000 microseconds per 100000 microseconds period
		resources.CPUPeriod = 100000 // 100ms
		resources.CPUQuota = int64(runCPU * 100000)
	}
	if runPids > 0 {
		resources.PidsMax = runPids
	}

	// Create container (this pulls the image if needed)
	fmt.Printf("Creating container with image '%s'...\n", image)
	container, err := rt.CreateContainer(ctx, runtime.ContainerOpts{
		Name:      runName,
		Image:     image,
		Command:   runCommand,
		Env:       env,
		WorkDir:   runWorkDir,
		Resources: resources,
	})
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}

	containerID := container.ID
	fmt.Printf("Container ID: %s\n", containerID)

	// Ensure cleanup
	if runRm {
		defer func() {
			fmt.Printf("Removing container %s...\n", containerID[:12])
			rt.RemoveContainer(context.Background(), containerID)
		}()
	}

	// Start container
	fmt.Printf("Starting container...\n")
	if err := rt.StartContainer(ctx, containerID); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	fmt.Printf("Container running.\n\n")

	// Stream logs to stdout
	logs, err := rt.ContainerLogs(ctx, containerID, true, 0)
	if err != nil {
		return fmt.Errorf("failed to get logs: %w", err)
	}
	defer logs.Close()

	// Copy logs to stdout
	done := make(chan error, 1)
	go func() {
		_, err := io.Copy(os.Stdout, logs)
		done <- err
	}()

	// Wait for logs to finish or context to be cancelled
	select {
	case err := <-done:
		if err != nil && err != context.Canceled {
			fmt.Fprintf(os.Stderr, "error reading logs: %v\n", err)
		}
	case <-ctx.Done():
		// Context cancelled, stop container
		rt.StopContainer(context.Background(), containerID, 10*time.Second)
		<-done
	}

	// Get final state
	fmt.Printf("\nContainer stopped.\n")

	return nil
}

// parseMemoryLimit parses a memory limit string like "128MB" or "1GB" into bytes.
func parseMemoryLimit(s string) (int64, error) {
	s = strings.TrimSpace(strings.ToUpper(s))

	// Match number followed by optional unit
	re := regexp.MustCompile(`^(\d+(?:\.\d+)?)\s*(B|K|KB|KIB|M|MB|MIB|G|GB|GIB|T|TB|TIB)?$`)
	matches := re.FindStringSubmatch(s)
	if matches == nil {
		return 0, fmt.Errorf("invalid format: %s (expected e.g., 128MB, 1GB, 512K)", s)
	}

	value, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return 0, fmt.Errorf("invalid number: %s", matches[1])
	}

	unit := matches[2]
	if unit == "" {
		unit = "B"
	}

	var multiplier float64
	switch unit {
	case "B":
		multiplier = 1
	case "K", "KB", "KIB":
		multiplier = 1024
	case "M", "MB", "MIB":
		multiplier = 1024 * 1024
	case "G", "GB", "GIB":
		multiplier = 1024 * 1024 * 1024
	case "T", "TB", "TIB":
		multiplier = 1024 * 1024 * 1024 * 1024
	default:
		return 0, fmt.Errorf("unknown unit: %s", unit)
	}

	return int64(value * multiplier), nil
}
