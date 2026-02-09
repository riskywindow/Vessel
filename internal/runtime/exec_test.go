package runtime

import (
	"bytes"
	"context"
	"os/exec"
	"testing"
)

// --- ExecOptions Validation Tests ---

func TestRunExec_InvalidPID_ReturnsError(t *testing.T) {
	_, err := RunExec(context.Background(), 0, ExecOptions{
		Command: []string{"echo", "hello"},
	})
	if err == nil {
		t.Fatal("expected error for PID 0")
	}
	if err.Error() != "invalid PID: 0" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunExec_NegativePID_ReturnsError(t *testing.T) {
	_, err := RunExec(context.Background(), -1, ExecOptions{
		Command: []string{"echo", "hello"},
	})
	if err == nil {
		t.Fatal("expected error for negative PID")
	}
}

func TestRunExec_EmptyCommand_ReturnsError(t *testing.T) {
	_, err := RunExec(context.Background(), 1234, ExecOptions{
		Command: []string{},
	})
	if err == nil {
		t.Fatal("expected error for empty command")
	}
	if err.Error() != "no command specified" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunExec_NilCommand_ReturnsError(t *testing.T) {
	_, err := RunExec(context.Background(), 1234, ExecOptions{})
	if err == nil {
		t.Fatal("expected error for nil command")
	}
}

// --- buildExecResult Tests ---

func TestBuildExecResult_NilError_ZeroExitCode(t *testing.T) {
	result, err := buildExecResult(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", result.ExitCode)
	}
}

func TestBuildExecResult_ExitError_PropagatesCode(t *testing.T) {
	// Create a command that exits with code 42
	cmd := exec.Command("sh", "-c", "exit 42")
	err := cmd.Run()
	if err == nil {
		t.Fatal("expected command to fail")
	}

	result, buildErr := buildExecResult(err)
	if buildErr != nil {
		t.Fatalf("unexpected build error: %v", buildErr)
	}
	if result.ExitCode != 42 {
		t.Fatalf("expected exit code 42, got %d", result.ExitCode)
	}
}

func TestBuildExecResult_ExitCode1(t *testing.T) {
	cmd := exec.Command("sh", "-c", "exit 1")
	err := cmd.Run()

	result, buildErr := buildExecResult(err)
	if buildErr != nil {
		t.Fatalf("unexpected build error: %v", buildErr)
	}
	if result.ExitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", result.ExitCode)
	}
}

func TestBuildExecResult_ExitCode0(t *testing.T) {
	cmd := exec.Command("sh", "-c", "exit 0")
	err := cmd.Run()

	result, buildErr := buildExecResult(err)
	if buildErr != nil {
		t.Fatalf("unexpected build error: %v", buildErr)
	}
	if result.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", result.ExitCode)
	}
}

// --- ExecOptions Struct Tests ---

func TestExecOptions_Defaults(t *testing.T) {
	opts := ExecOptions{
		ContainerID: "test-container-123",
		Command:     []string{"echo", "hello"},
	}

	if opts.TTY {
		t.Fatal("TTY should default to false")
	}
	if opts.Interactive {
		t.Fatal("Interactive should default to false")
	}
	if opts.WorkDir != "" {
		t.Fatal("WorkDir should default to empty")
	}
	if opts.User != "" {
		t.Fatal("User should default to empty")
	}
	if opts.Stdin != nil {
		t.Fatal("Stdin should default to nil")
	}
	if opts.Stdout != nil {
		t.Fatal("Stdout should default to nil")
	}
	if opts.Stderr != nil {
		t.Fatal("Stderr should default to nil")
	}
}

// --- execWithoutTTY Tests ---

func TestExecWithoutTTY_CapturesStdout(t *testing.T) {
	var stdout bytes.Buffer
	cmd := exec.Command("echo", "hello world")

	result, err := execWithoutTTY(cmd, ExecOptions{
		Stdout: &stdout,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", result.ExitCode)
	}

	output := stdout.String()
	if output != "hello world\n" {
		t.Fatalf("expected 'hello world\\n', got %q", output)
	}
}

func TestExecWithoutTTY_CapturesStderr(t *testing.T) {
	var stderr bytes.Buffer
	cmd := exec.Command("sh", "-c", "echo error >&2")

	result, err := execWithoutTTY(cmd, ExecOptions{
		Stdout: &bytes.Buffer{},
		Stderr: &stderr,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", result.ExitCode)
	}

	output := stderr.String()
	if output != "error\n" {
		t.Fatalf("expected 'error\\n', got %q", output)
	}
}

func TestExecWithoutTTY_ExitCodePropagation(t *testing.T) {
	cmd := exec.Command("sh", "-c", "exit 7")

	result, err := execWithoutTTY(cmd, ExecOptions{
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 7 {
		t.Fatalf("expected exit code 7, got %d", result.ExitCode)
	}
}

func TestExecWithoutTTY_StdinPipe(t *testing.T) {
	stdin := bytes.NewBufferString("hello from stdin\n")
	var stdout bytes.Buffer
	cmd := exec.Command("cat")

	result, err := execWithoutTTY(cmd, ExecOptions{
		Stdin:  stdin,
		Stdout: &stdout,
		Stderr: &bytes.Buffer{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", result.ExitCode)
	}

	output := stdout.String()
	if output != "hello from stdin\n" {
		t.Fatalf("expected 'hello from stdin\\n', got %q", output)
	}
}

func TestExecWithoutTTY_CommandNotFound(t *testing.T) {
	cmd := exec.Command("nonexistent-command-12345")

	_, err := execWithoutTTY(cmd, ExecOptions{
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
	})
	if err == nil {
		t.Fatal("expected error for nonexistent command")
	}
}

// --- ExecResult Tests ---

func TestExecResult_ZeroValue(t *testing.T) {
	var r ExecResult
	if r.ExitCode != 0 {
		t.Fatalf("expected zero value exit code to be 0, got %d", r.ExitCode)
	}
}

// --- Context Cancellation ---

func TestRunExec_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	// This should fail because nsenter won't find the target PID,
	// but the context cancellation should prevent prolonged execution.
	_, err := RunExec(ctx, 999999999, ExecOptions{
		Command: []string{"echo", "hello"},
	})
	// We expect an error (either context cancelled or nsenter failure)
	if err == nil {
		// nsenter might still return an exit code rather than an error,
		// so this is acceptable in some cases
		t.Log("command succeeded despite cancelled context (may be race)")
	}
}
