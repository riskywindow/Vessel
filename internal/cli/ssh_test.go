package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSSHCommand_Exists(t *testing.T) {
	if sshCmd == nil {
		t.Fatal("sshCmd is nil")
	}
	if sshCmd.Use != "ssh <user@host> [command] [args...]" {
		t.Errorf("unexpected Use: %s", sshCmd.Use)
	}
}

func TestSSHCommand_Registered(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "ssh" {
			found = true
			break
		}
	}
	if !found {
		t.Error("ssh command not registered in root")
	}
}

func TestRemoteCommand_Registered(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "remote" {
			found = true
			break
		}
	}
	if !found {
		t.Error("remote command not registered in root")
	}
}

func TestRemoteCommand_Subcommands(t *testing.T) {
	expected := []string{"list", "add", "remove"}
	for _, name := range expected {
		found := false
		for _, cmd := range remoteCmd.Commands() {
			if cmd.Name() == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("remote subcommand %q not found", name)
		}
	}
}

func TestNeedsDirectSSH(t *testing.T) {
	tests := []struct {
		args []string
		want bool
	}{
		{[]string{"exec", "my-app", "--", "/bin/sh"}, true},
		{[]string{"logs", "-f", "my-app"}, true},
		{[]string{"logs", "--follow", "my-app"}, true},
		{[]string{"logs", "my-app"}, false},
		{[]string{"ps"}, false},
		{[]string{"deploy"}, false},
		{[]string{"stop", "my-app"}, false},
		{[]string{"health", "watch"}, true},
		{[]string{"health", "my-app"}, false},
		{[]string{}, false},
	}

	for _, tt := range tests {
		name := strings.Join(tt.args, " ")
		if name == "" {
			name = "(empty)"
		}
		t.Run(name, func(t *testing.T) {
			got := needsDirectSSH(tt.args)
			if got != tt.want {
				t.Errorf("needsDirectSSH(%v) = %v, want %v", tt.args, got, tt.want)
			}
		})
	}
}

func TestShouldSyncConfig(t *testing.T) {
	tests := []struct {
		args []string
		want bool
	}{
		{[]string{"deploy", "--config", "app.toml"}, true},
		{[]string{"deploy", "-c", "app.toml"}, true},
		{[]string{"deploy"}, false},
		{[]string{"ps"}, false},
		{[]string{"deploy", "--all"}, false},
	}

	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			got := shouldSyncConfig(tt.args)
			if got != tt.want {
				t.Errorf("shouldSyncConfig(%v) = %v, want %v", tt.args, got, tt.want)
			}
		})
	}
}

func TestGetConfigFromArgs(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{[]string{"--config", "app.toml"}, "app.toml"},
		{[]string{"-c", "my-config.toml"}, "my-config.toml"},
		{[]string{"deploy", "--config", "/path/to/app.toml"}, "/path/to/app.toml"},
		{[]string{"deploy", "--all"}, ""},
		{[]string{}, ""},
		{[]string{"--config"}, ""}, // Missing value
	}

	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			got := getConfigFromArgs(tt.args)
			if got != tt.want {
				t.Errorf("getConfigFromArgs(%v) = %q, want %q", tt.args, got, tt.want)
			}
		})
	}
}

func TestReplaceConfigPath(t *testing.T) {
	args := []string{"deploy", "--config", "/local/path.toml", "--all"}
	result := replaceConfigPath(args, "/remote/path.toml")

	if result[2] != "/remote/path.toml" {
		t.Errorf("expected /remote/path.toml at index 2, got %s", result[2])
	}

	// Original should not be modified
	if args[2] != "/local/path.toml" {
		t.Errorf("original args were modified: %s", args[2])
	}
}

func TestReplaceConfigPath_ShortFlag(t *testing.T) {
	args := []string{"deploy", "-c", "app.toml"}
	result := replaceConfigPath(args, "/tmp/remote.toml")

	if result[2] != "/tmp/remote.toml" {
		t.Errorf("expected /tmp/remote.toml, got %s", result[2])
	}
}

func TestReplaceConfigPath_NoConfigFlag(t *testing.T) {
	args := []string{"deploy", "--all"}
	result := replaceConfigPath(args, "/tmp/remote.toml")

	if len(result) != len(args) {
		t.Errorf("expected same length, got %d vs %d", len(result), len(args))
	}
	if result[0] != "deploy" || result[1] != "--all" {
		t.Errorf("unexpected args: %v", result)
	}
}

func TestResolveSSHTarget_UserAtHost(t *testing.T) {
	// Point configFile to a nonexistent file so it falls through to ParseTarget
	old := configFile
	configFile = "/tmp/vessel-nonexistent.toml"
	defer func() { configFile = old }()

	cfg, err := resolveSSHTarget("root@prod.example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.User != "root" {
		t.Errorf("expected user root, got %s", cfg.User)
	}
	if cfg.Host != "prod.example.com" {
		t.Errorf("expected host prod.example.com, got %s", cfg.Host)
	}
	if cfg.Port != 22 {
		t.Errorf("expected port 22, got %d", cfg.Port)
	}
}

func TestResolveSSHTarget_HostOnly(t *testing.T) {
	old := configFile
	configFile = "/tmp/vessel-nonexistent.toml"
	defer func() { configFile = old }()

	cfg, err := resolveSSHTarget("server.example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.User != "" {
		t.Errorf("expected empty user, got %s", cfg.User)
	}
	if cfg.Host != "server.example.com" {
		t.Errorf("expected host server.example.com, got %s", cfg.Host)
	}
}

func TestResolveSSHTarget_NamedRemote(t *testing.T) {
	// Create a temp config file with a named remote
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "vessel.toml")
	cfgContent := `
[[app]]
name = "myapp"
image = "nginx:latest"

[[remote]]
name = "prod"
host = "prod.example.com"
user = "deploy"
port = 2222
key_file = "~/.ssh/prod_key"

[[remote]]
name = "staging"
host = "staging.example.com"
user = "admin"
`
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	old := configFile
	configFile = cfgPath
	defer func() { configFile = old }()

	cfg, err := resolveSSHTarget("prod")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Host != "prod.example.com" {
		t.Errorf("expected host prod.example.com, got %s", cfg.Host)
	}
	if cfg.User != "deploy" {
		t.Errorf("expected user deploy, got %s", cfg.User)
	}
	if cfg.Port != 2222 {
		t.Errorf("expected port 2222, got %d", cfg.Port)
	}
	if cfg.KeyFile != "~/.ssh/prod_key" {
		t.Errorf("expected key file ~/.ssh/prod_key, got %s", cfg.KeyFile)
	}
}

func TestResolveSSHTarget_NamedRemoteNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "vessel.toml")
	cfgContent := `
[[app]]
name = "myapp"
image = "nginx:latest"

[[remote]]
name = "prod"
host = "prod.example.com"
user = "deploy"
`
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	old := configFile
	configFile = cfgPath
	defer func() { configFile = old }()

	// "staging" is not defined, so it falls through to ParseTarget
	cfg, err := resolveSSHTarget("staging")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should treat "staging" as a hostname
	if cfg.Host != "staging" {
		t.Errorf("expected host 'staging', got %s", cfg.Host)
	}
}

func TestSSHCommand_Flags(t *testing.T) {
	// Verify flags exist
	portFlag := sshCmd.Flags().Lookup("port")
	if portFlag == nil {
		t.Error("port flag not registered")
	}
	if portFlag.Shorthand != "p" {
		t.Errorf("expected port shorthand 'p', got %q", portFlag.Shorthand)
	}

	identityFlag := sshCmd.Flags().Lookup("identity")
	if identityFlag == nil {
		t.Error("identity flag not registered")
	}
	if identityFlag.Shorthand != "i" {
		t.Errorf("expected identity shorthand 'i', got %q", identityFlag.Shorthand)
	}
}
