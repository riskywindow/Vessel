package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/vessel/vessel/internal/runtime"
)

var (
	runName    string
	runEnv     []string
	runWorkDir string
	runRm      bool
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
  vessel run nginx:alpine`,
	Args: cobra.MinimumNArgs(1),
	RunE: runContainer,
}

func init() {
	runCmd.Flags().StringVarP(&runName, "name", "n", "", "container name")
	runCmd.Flags().StringArrayVarP(&runEnv, "env", "e", nil, "environment variables (KEY=VALUE)")
	runCmd.Flags().StringVarP(&runWorkDir, "workdir", "w", "", "working directory inside container")
	runCmd.Flags().BoolVar(&runRm, "rm", false, "automatically remove container when it exits")

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

	// Create container (this pulls the image if needed)
	fmt.Printf("Creating container with image '%s'...\n", image)
	container, err := rt.CreateContainer(ctx, runtime.ContainerOpts{
		Name:    runName,
		Image:   image,
		Command: runCommand,
		Env:     env,
		WorkDir: runWorkDir,
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
