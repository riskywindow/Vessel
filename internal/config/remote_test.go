package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRemoteConfig_Parse(t *testing.T) {
	content := `
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
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "vessel.toml")
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	if len(cfg.Remotes) != 2 {
		t.Fatalf("expected 2 remotes, got %d", len(cfg.Remotes))
	}

	// First remote
	r := cfg.Remotes[0]
	if r.Name != "prod" {
		t.Errorf("expected name 'prod', got %q", r.Name)
	}
	if r.Host != "prod.example.com" {
		t.Errorf("expected host 'prod.example.com', got %q", r.Host)
	}
	if r.User != "deploy" {
		t.Errorf("expected user 'deploy', got %q", r.User)
	}
	if r.Port != 2222 {
		t.Errorf("expected port 2222, got %d", r.Port)
	}
	if r.KeyFile != "~/.ssh/prod_key" {
		t.Errorf("expected key_file '~/.ssh/prod_key', got %q", r.KeyFile)
	}

	// Second remote
	r2 := cfg.Remotes[1]
	if r2.Name != "staging" {
		t.Errorf("expected name 'staging', got %q", r2.Name)
	}
	if r2.Host != "staging.example.com" {
		t.Errorf("expected host 'staging.example.com', got %q", r2.Host)
	}
	if r2.User != "admin" {
		t.Errorf("expected user 'admin', got %q", r2.User)
	}
	if r2.Port != 0 {
		t.Errorf("expected port 0 (default), got %d", r2.Port)
	}
}

func TestRemoteConfig_Empty(t *testing.T) {
	content := `
[[app]]
name = "myapp"
image = "nginx:latest"
`
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "vessel.toml")
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	if len(cfg.Remotes) != 0 {
		t.Errorf("expected 0 remotes, got %d", len(cfg.Remotes))
	}
}

func TestRemoteConfig_WithApps(t *testing.T) {
	content := `
[[app]]
name = "myapp"
image = "nginx:latest"

[[remote]]
name = "prod"
host = "prod.example.com"
user = "deploy"
`
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "vessel.toml")
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	if len(cfg.Apps) != 1 {
		t.Errorf("expected 1 app, got %d", len(cfg.Apps))
	}
	if len(cfg.Remotes) != 1 {
		t.Errorf("expected 1 remote, got %d", len(cfg.Remotes))
	}
}

func TestRemoteConfig_PortDefault(t *testing.T) {
	content := `
[[app]]
name = "myapp"
image = "nginx:latest"

[[remote]]
name = "dev"
host = "192.168.1.100"
user = "root"
`
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "vessel.toml")
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	if len(cfg.Remotes) != 1 {
		t.Fatalf("expected 1 remote, got %d", len(cfg.Remotes))
	}

	// Port should be 0 (caller applies default 22)
	if cfg.Remotes[0].Port != 0 {
		t.Errorf("expected port 0 (unset), got %d", cfg.Remotes[0].Port)
	}
}
