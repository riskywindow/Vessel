package runtime

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/creack/pty"
	"golang.org/x/term"
)

// ExecOptions configures how a command is executed in a container.
type ExecOptions struct {
	ContainerID string    // Used by LinuxRuntime.Exec to look up PID
	Command     []string  // Command and arguments to run
	Stdin       io.Reader // Stdin override (nil = no stdin unless Interactive)
	Stdout      io.Writer // Stdout override (nil = os.Stdout)
	Stderr      io.Writer // Stderr override (nil = os.Stderr)
	TTY         bool      // Allocate a pseudo-TTY
	Interactive bool      // Keep stdin open
	Env         []string  // Additional environment variables (KEY=VALUE)
	WorkDir     string    // Working directory inside container
	User        string    // User to run as (e.g., "root", "1000:1000")
}

// ExecResult contains the result of an exec operation.
type ExecResult struct {
	ExitCode int
}

// Exec executes a command in a running container with full TTY support.
// It looks up the container PID from the runtime's container state.
func (r *LinuxRuntime) Exec(ctx context.Context, opts ExecOptions) (*ExecResult, error) {
	pid, err := r.GetContainerPID(opts.ContainerID)
	if err != nil {
		return nil, fmt.Errorf("container not running: %w", err)
	}

	return RunExec(ctx, pid, opts)
}

// RunExec executes a command inside a container's namespaces using nsenter.
// This is a standalone function that accepts a PID directly, so it can be
// called without a full LinuxRuntime instance (e.g., from the CLI after
// looking up the PID via the daemon socket).
func RunExec(ctx context.Context, pid int, opts ExecOptions) (*ExecResult, error) {
	if pid <= 0 {
		return nil, fmt.Errorf("invalid PID: %d", pid)
	}
	if len(opts.Command) == 0 {
		return nil, fmt.Errorf("no command specified")
	}

	// Build nsenter command
	// -t <pid>: target process
	// -p: enter PID namespace
	// -m: enter mount namespace
	// -u: enter UTS namespace (hostname)
	// -i: enter IPC namespace
	// -n: enter network namespace
	args := []string{
		"-t", fmt.Sprintf("%d", pid),
		"-p", "-m", "-u", "-i", "-n",
	}

	// Add working directory if specified
	if opts.WorkDir != "" {
		args = append(args, fmt.Sprintf("--wd=%s", opts.WorkDir))
	}

	args = append(args, "--")
	args = append(args, opts.Command...)

	cmd := exec.CommandContext(ctx, "nsenter", args...)

	// Set environment variables
	if len(opts.Env) > 0 {
		cmd.Env = append(os.Environ(), opts.Env...)
	}

	if opts.TTY {
		return execWithTTY(cmd)
	}

	return execWithoutTTY(cmd, opts)
}

// execWithTTY runs the command with a pseudo-terminal.
func execWithTTY(cmd *exec.Cmd) (*ExecResult, error) {
	// Start the command with a pty
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to start pty: %w", err)
	}
	defer ptmx.Close()

	// Handle terminal resize signals
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGWINCH)
	go func() {
		for range ch {
			_ = pty.InheritSize(os.Stdin, ptmx)
		}
	}()
	ch <- syscall.SIGWINCH // Initial resize
	defer func() {
		signal.Stop(ch)
		close(ch)
	}()

	// Set stdin to raw mode so keystrokes pass through directly
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return nil, fmt.Errorf("failed to set raw terminal mode: %w", err)
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	// Copy stdin to pty in background
	go func() {
		_, _ = io.Copy(ptmx, os.Stdin)
	}()

	// Copy pty output to stdout (blocks until pty closes)
	_, _ = io.Copy(os.Stdout, ptmx)

	// Wait for command to finish
	err = cmd.Wait()

	return buildExecResult(err)
}

// execWithoutTTY runs the command without a pseudo-terminal.
func execWithoutTTY(cmd *exec.Cmd, opts ExecOptions) (*ExecResult, error) {
	if opts.Stdin != nil {
		cmd.Stdin = opts.Stdin
	} else if opts.Interactive {
		cmd.Stdin = os.Stdin
	}

	if opts.Stdout != nil {
		cmd.Stdout = opts.Stdout
	} else {
		cmd.Stdout = os.Stdout
	}

	if opts.Stderr != nil {
		cmd.Stderr = opts.Stderr
	} else {
		cmd.Stderr = os.Stderr
	}

	err := cmd.Run()
	return buildExecResult(err)
}

// buildExecResult extracts exit code from a command error.
func buildExecResult(err error) (*ExecResult, error) {
	if err == nil {
		return &ExecResult{ExitCode: 0}, nil
	}

	if exitErr, ok := err.(*exec.ExitError); ok {
		return &ExecResult{ExitCode: exitErr.ExitCode()}, nil
	}

	return nil, err
}
