package ssh

import (
	"os"
	"testing"
)

func TestNewClient_Defaults(t *testing.T) {
	c := NewClient(Config{Host: "example.com"})
	if c.host != "example.com" {
		t.Errorf("expected host example.com, got %s", c.host)
	}
	if c.port != 22 {
		t.Errorf("expected port 22, got %d", c.port)
	}
	// User should fall back to $USER env var
	expectedUser := os.Getenv("USER")
	if c.user != expectedUser {
		t.Errorf("expected user %s, got %s", expectedUser, c.user)
	}
}

func TestNewClient_CustomConfig(t *testing.T) {
	c := NewClient(Config{
		Host:    "prod.example.com",
		User:    "deploy",
		Port:    2222,
		KeyFile: "/home/user/.ssh/prod_key",
	})

	if c.host != "prod.example.com" {
		t.Errorf("expected host prod.example.com, got %s", c.host)
	}
	if c.user != "deploy" {
		t.Errorf("expected user deploy, got %s", c.user)
	}
	if c.port != 2222 {
		t.Errorf("expected port 2222, got %d", c.port)
	}
	if c.keyFile != "/home/user/.ssh/prod_key" {
		t.Errorf("expected key file /home/user/.ssh/prod_key, got %s", c.keyFile)
	}
}

func TestNewClient_ZeroPort(t *testing.T) {
	c := NewClient(Config{Host: "host.com", Port: 0})
	if c.port != 22 {
		t.Errorf("expected port 22 for zero-value port, got %d", c.port)
	}
}

func TestNewClient_EmptyUser(t *testing.T) {
	c := NewClient(Config{Host: "host.com", User: ""})
	expectedUser := os.Getenv("USER")
	if c.user != expectedUser {
		t.Errorf("expected user %s, got %s", expectedUser, c.user)
	}
}

func TestParseTarget_UserAtHost(t *testing.T) {
	tests := []struct {
		input    string
		wantUser string
		wantHost string
	}{
		{"root@example.com", "root", "example.com"},
		{"deploy@prod.server.com", "deploy", "prod.server.com"},
		{"admin@192.168.1.1", "admin", "192.168.1.1"},
		{"user@host", "user", "host"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			user, host := ParseTarget(tt.input)
			if user != tt.wantUser {
				t.Errorf("ParseTarget(%q) user = %q, want %q", tt.input, user, tt.wantUser)
			}
			if host != tt.wantHost {
				t.Errorf("ParseTarget(%q) host = %q, want %q", tt.input, host, tt.wantHost)
			}
		})
	}
}

func TestParseTarget_HostOnly(t *testing.T) {
	user, host := ParseTarget("example.com")
	if user != "" {
		t.Errorf("expected empty user, got %q", user)
	}
	if host != "example.com" {
		t.Errorf("expected host example.com, got %q", host)
	}
}

func TestParseTarget_MultipleAt(t *testing.T) {
	// Should split on first @
	user, host := ParseTarget("user@host@extra")
	if user != "user" {
		t.Errorf("expected user 'user', got %q", user)
	}
	if host != "host@extra" {
		t.Errorf("expected host 'host@extra', got %q", host)
	}
}

func TestClient_Host(t *testing.T) {
	c := NewClient(Config{Host: "myhost.com"})
	if c.Host() != "myhost.com" {
		t.Errorf("expected Host() = myhost.com, got %s", c.Host())
	}
}

func TestClient_User(t *testing.T) {
	c := NewClient(Config{Host: "h", User: "testuser"})
	if c.User() != "testuser" {
		t.Errorf("expected User() = testuser, got %s", c.User())
	}
}

func TestClient_Port(t *testing.T) {
	c := NewClient(Config{Host: "h", Port: 3333})
	if c.Port() != 3333 {
		t.Errorf("expected Port() = 3333, got %d", c.Port())
	}
}

func TestClient_SocketPath_Empty(t *testing.T) {
	c := NewClient(Config{Host: "h"})
	if c.SocketPath() != "" {
		t.Errorf("expected empty SocketPath before connect, got %s", c.SocketPath())
	}
}

func TestClient_Close_NoTunnel(t *testing.T) {
	c := NewClient(Config{Host: "h"})
	// Close should not panic when there's no tunnel
	if err := c.Close(); err != nil {
		t.Errorf("Close() should not error with no tunnel: %v", err)
	}
}

func TestBuildSSHArgs_NoKey(t *testing.T) {
	c := NewClient(Config{Host: "h", Port: 22})
	args := c.buildSSHArgs()

	if len(args) != 2 {
		t.Fatalf("expected 2 args, got %d: %v", len(args), args)
	}
	if args[0] != "-p" || args[1] != "22" {
		t.Errorf("expected [-p 22], got %v", args)
	}
}

func TestBuildSSHArgs_WithKey(t *testing.T) {
	c := NewClient(Config{Host: "h", Port: 2222, KeyFile: "/path/to/key"})
	args := c.buildSSHArgs()

	if len(args) != 4 {
		t.Fatalf("expected 4 args, got %d: %v", len(args), args)
	}
	if args[0] != "-i" || args[1] != "/path/to/key" {
		t.Errorf("expected first args [-i /path/to/key], got %v", args[:2])
	}
	if args[2] != "-p" || args[3] != "2222" {
		t.Errorf("expected last args [-p 2222], got %v", args[2:])
	}
}

func TestConnect_EmptyHost(t *testing.T) {
	c := NewClient(Config{Host: ""})
	_, err := c.Connect(nil)
	if err == nil {
		t.Error("expected error for empty host")
	}
}
