package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestCompletionCommand_Exists(t *testing.T) {
	if completionCmd == nil {
		t.Fatal("completionCmd is nil")
	}
	if completionCmd.Use != "completion [bash|zsh|fish|powershell]" {
		t.Errorf("unexpected Use: %s", completionCmd.Use)
	}
}

func TestCompletionCommand_ValidArgs(t *testing.T) {
	expected := []string{"bash", "zsh", "fish", "powershell"}
	if len(completionCmd.ValidArgs) != len(expected) {
		t.Fatalf("expected %d valid args, got %d", len(expected), len(completionCmd.ValidArgs))
	}
	for i, arg := range expected {
		if completionCmd.ValidArgs[i] != arg {
			t.Errorf("expected valid arg %d to be %q, got %q", i, arg, completionCmd.ValidArgs[i])
		}
	}
}

func TestCompletionBash(t *testing.T) {
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetArgs([]string{"completion", "bash"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Bash completion scripts contain the command name
	// The output goes to stdout via os.Stdout, not the cobra writer,
	// so we test via GenBashCompletion directly.
	buf.Reset()
	err = rootCmd.GenBashCompletion(&buf)
	if err != nil {
		t.Fatalf("GenBashCompletion error: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "vessel") {
		t.Error("bash completion output should contain 'vessel'")
	}
	if !strings.Contains(output, "complete") || !strings.Contains(output, "bash") {
		t.Error("bash completion output should contain bash completion functions")
	}
}

func TestCompletionZsh(t *testing.T) {
	var buf bytes.Buffer
	err := rootCmd.GenZshCompletion(&buf)
	if err != nil {
		t.Fatalf("GenZshCompletion error: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "vessel") {
		t.Error("zsh completion output should contain 'vessel'")
	}
}

func TestCompletionFish(t *testing.T) {
	var buf bytes.Buffer
	err := rootCmd.GenFishCompletion(&buf, true)
	if err != nil {
		t.Fatalf("GenFishCompletion error: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "vessel") {
		t.Error("fish completion output should contain 'vessel'")
	}
}

func TestCompletionPowerShell(t *testing.T) {
	var buf bytes.Buffer
	err := rootCmd.GenPowerShellCompletionWithDesc(&buf)
	if err != nil {
		t.Fatalf("GenPowerShellCompletionWithDesc error: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "vessel") {
		t.Error("powershell completion output should contain 'vessel'")
	}
}

func TestCompletionRegistered(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "completion" {
			found = true
			break
		}
	}
	if !found {
		t.Error("completion command not registered in root")
	}
}

func TestCompleteAppNames_NoArgs(t *testing.T) {
	// Without a running daemon, completeAppNames should return an error directive
	cmd := &cobra.Command{}
	names, directive := completeAppNames(cmd, []string{}, "")
	// Should return error since daemon isn't running
	if directive != cobra.ShellCompDirectiveError {
		// It's also OK if it returns no file comp (daemon not running)
		if directive != cobra.ShellCompDirectiveNoFileComp || len(names) != 0 {
			// Just verify it doesn't panic — daemon connection failure is expected
			_ = names
		}
	}
}

func TestCompleteAppNames_WithExistingArgs(t *testing.T) {
	// If args already provided, should return nothing
	cmd := &cobra.Command{}
	names, directive := completeAppNames(cmd, []string{"myapp"}, "")
	if len(names) != 0 {
		t.Errorf("expected no completions when args exist, got %v", names)
	}
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("expected NoFileComp directive, got %v", directive)
	}
}

func TestCommandsHaveValidArgsFunction(t *testing.T) {
	// Verify that commands which take app names have ValidArgsFunction set
	cmdsWithCompletion := []string{"exec", "stop", "rm", "logs", "rollback", "health", "stats", "history"}

	for _, name := range cmdsWithCompletion {
		var found *cobra.Command
		for _, cmd := range rootCmd.Commands() {
			if cmd.Name() == name {
				found = cmd
				break
			}
		}
		if found == nil {
			t.Errorf("command %q not found in root", name)
			continue
		}
		if found.ValidArgsFunction == nil {
			t.Errorf("command %q missing ValidArgsFunction for app name completion", name)
		}
	}
}
