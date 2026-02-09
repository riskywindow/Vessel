package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/vessel/vessel/internal/daemon"
	"github.com/vessel/vessel/internal/runtime"
	"github.com/vessel/vessel/internal/store"
	"golang.org/x/term"
)

var execCmd = &cobra.Command{
	Use:   "exec [flags] <app> [--] <command> [args...]",
	Short: "Execute a command in a container",
	Long: `Execute a command inside a running container.

By default, executes in the first running container of the app.
Use -c/--container to specify a specific container ID.

Examples:
  vessel exec my-app -- sh                    # Open a shell
  vessel exec my-app -- ls -la /app           # List files
  vessel exec -it my-app -- /bin/bash         # Interactive bash session
  vessel exec my-app -c abc123 -- ps aux      # Exec in specific container
  vessel exec my-app -- env                   # View environment variables
  vessel exec my-app -e MY_VAR=hello -- sh -c 'echo $MY_VAR'
  vessel exec my-app -w /tmp -- pwd           # Exec with working directory`,
	Args:               cobra.MinimumNArgs(1),
	RunE:               runExec,
	ValidArgsFunction:  completeAppNames,
	DisableFlagParsing: false,
}

var (
	execInteractive bool
	execTTY         bool
	execContainer   string
	execEnv         []string
	execWorkDir     string
	execUser        string
)

func init() {
	execCmd.Flags().BoolVarP(&execInteractive, "interactive", "i", false, "Keep STDIN open")
	execCmd.Flags().BoolVarP(&execTTY, "tty", "t", false, "Allocate a pseudo-TTY")
	execCmd.Flags().StringVar(&execContainer, "container", "", "Container ID (default: first running container)")
	execCmd.Flags().StringArrayVarP(&execEnv, "env", "e", nil, "Set environment variables (KEY=VALUE)")
	execCmd.Flags().StringVarP(&execWorkDir, "workdir", "w", "", "Working directory inside the container")
	execCmd.Flags().StringVarP(&execUser, "user", "u", "", "Username or UID (format: <name|uid>[:<group|gid>])")
}

func runExec(cmd *cobra.Command, args []string) error {
	appName := args[0]

	// Determine the command to execute.
	// Cobra splits on "--" automatically: args before -- are cobra args,
	// args after -- go into ArgsAfterDash. But since app name is first arg,
	// the command is everything after the app name.
	cmdArgs := args[1:]
	if len(cmdArgs) == 0 {
		cmdArgs = []string{"/bin/sh"}
	}

	// Auto-detect TTY if stdin is a terminal and -t not explicitly set
	if !cmd.Flags().Changed("tty") && term.IsTerminal(int(os.Stdin.Fd())) && len(cmdArgs) == 1 && isShellCommand(cmdArgs[0]) {
		execTTY = true
	}

	// -i implies -t in most interactive scenarios
	if execInteractive && !cmd.Flags().Changed("tty") {
		execTTY = true
	}

	// Look up the container PID
	containerID, containerPID, err := resolveContainer(appName, execContainer)
	if err != nil {
		return err
	}

	if containerPID == 0 {
		return fmt.Errorf("container %s has no PID recorded (is it running?)", containerID[:12])
	}

	// Run exec directly via nsenter
	result, err := runtime.RunExec(context.Background(), containerPID, runtime.ExecOptions{
		ContainerID: containerID,
		Command:     cmdArgs,
		TTY:         execTTY,
		Interactive: execInteractive,
		Env:         execEnv,
		WorkDir:     execWorkDir,
		User:        execUser,
	})
	if err != nil {
		return err
	}

	// Exit with the same code as the executed command
	if result.ExitCode != 0 {
		os.Exit(result.ExitCode)
	}

	return nil
}

// resolveContainer finds the container ID and PID for exec.
// If containerID is specified, looks it up directly.
// Otherwise, finds the first running container for the given app.
func resolveContainer(appName string, containerID string) (string, int, error) {
	client := daemon.NewClient("")

	if containerID != "" {
		// Look up specific container by listing all for this app
		var containers []*store.Container
		if err := client.Call("containers.list", map[string]string{"app_name": appName}, &containers); err != nil {
			return "", 0, fmt.Errorf("failed to list containers: %w", err)
		}

		for _, c := range containers {
			if c.ID == containerID || (len(c.ID) >= 12 && c.ID[:12] == containerID) {
				if c.State != store.ContainerStateRunning {
					return "", 0, fmt.Errorf("container %s is not running (state: %s)", containerID, c.State)
				}
				return c.ID, c.PID, nil
			}
		}
		return "", 0, fmt.Errorf("container %q not found in app %q", containerID, appName)
	}

	// Find first running container for this app
	var containers []*store.Container
	if err := client.Call("containers.list", map[string]string{"app_name": appName}, &containers); err != nil {
		return "", 0, fmt.Errorf("failed to list containers: %w", err)
	}

	for _, c := range containers {
		if c.State == store.ContainerStateRunning {
			return c.ID, c.PID, nil
		}
	}

	return "", 0, fmt.Errorf("no running containers found for app %q", appName)
}

// isShellCommand returns true if the command is a common shell.
func isShellCommand(cmd string) bool {
	switch cmd {
	case "sh", "/bin/sh", "bash", "/bin/bash", "zsh", "/bin/zsh", "ash", "/bin/ash":
		return true
	}
	return false
}
