// Package ssh implements SSH tunneling for remote Vessel daemon access.
package ssh

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Client manages SSH connections to remote Vessel daemons.
type Client struct {
	host       string
	user       string
	port       int
	keyFile    string
	socketPath string
	tunnel     *exec.Cmd
}

// Config for SSH connection.
type Config struct {
	Host    string // hostname or IP
	User    string // SSH username
	Port    int    // SSH port (default 22)
	KeyFile string // Path to SSH private key (optional)
}

// NewClient creates a new SSH client.
func NewClient(cfg Config) *Client {
	if cfg.Port == 0 {
		cfg.Port = 22
	}
	if cfg.User == "" {
		cfg.User = os.Getenv("USER")
	}

	return &Client{
		host:    cfg.Host,
		user:    cfg.User,
		port:    cfg.Port,
		keyFile: cfg.KeyFile,
	}
}

// Connect establishes an SSH tunnel to the remote Vessel daemon.
// Returns the local socket path to connect to.
func (c *Client) Connect(ctx context.Context) (string, error) {
	if c.host == "" {
		return "", fmt.Errorf("SSH host is required")
	}

	// Create local socket for forwarding
	localSocket := filepath.Join(os.TempDir(), fmt.Sprintf("vessel-remote-%d.sock", os.Getpid()))
	os.Remove(localSocket) // Clean up any existing socket

	remoteSocket := "/var/lib/vessel/run/vessel.sock"

	// Build SSH command for socket forwarding
	args := c.buildSSHArgs()
	args = append(args,
		"-N",                                           // No remote command
		"-L", localSocket+":"+remoteSocket,             // Forward Unix socket
		"-o", "ExitOnForwardFailure=yes",               // Fail if forward can't be established
		"-o", "ServerAliveInterval=30",                 // Keep connection alive
		"-o", "ServerAliveCountMax=3",                  // Disconnect after 3 missed keepalives
		"-o", "StrictHostKeyChecking=accept-new",       // Accept new host keys
		"-o", "ConnectTimeout=10",                      // 10 second connect timeout
		fmt.Sprintf("%s@%s", c.user, c.host),
	)

	c.tunnel = exec.CommandContext(ctx, "ssh", args...)
	c.tunnel.Stdout = io.Discard
	c.tunnel.Stderr = os.Stderr

	if err := c.tunnel.Start(); err != nil {
		return "", fmt.Errorf("failed to start SSH tunnel: %w", err)
	}

	// Wait for socket to become available
	for i := 0; i < 50; i++ { // 5 seconds timeout
		if _, err := os.Stat(localSocket); err == nil {
			c.socketPath = localSocket
			return localSocket, nil
		}

		// Check if the tunnel process has already exited (error case)
		if c.tunnel.ProcessState != nil {
			return "", fmt.Errorf("SSH tunnel exited unexpectedly")
		}

		time.Sleep(100 * time.Millisecond)
	}

	c.tunnel.Process.Kill()
	return "", fmt.Errorf("timeout waiting for SSH tunnel to %s:%d", c.host, c.port)
}

// Close terminates the SSH tunnel and cleans up.
func (c *Client) Close() error {
	if c.tunnel != nil && c.tunnel.Process != nil {
		c.tunnel.Process.Kill()
		c.tunnel.Wait() // Prevent zombie process
	}
	if c.socketPath != "" {
		os.Remove(c.socketPath)
	}
	return nil
}

// CopyFile copies a local file to the remote server using scp.
func (c *Client) CopyFile(ctx context.Context, localPath, remotePath string) error {
	args := []string{}
	if c.keyFile != "" {
		args = append(args, "-i", c.keyFile)
	}
	args = append(args,
		"-P", fmt.Sprintf("%d", c.port),
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "ConnectTimeout=10",
		localPath,
		fmt.Sprintf("%s@%s:%s", c.user, c.host, remotePath),
	)

	cmd := exec.CommandContext(ctx, "scp", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("scp to %s:%s failed: %w", c.host, remotePath, err)
	}
	return nil
}

// RunCommand runs a command on the remote server via SSH.
func (c *Client) RunCommand(ctx context.Context, command string) error {
	args := c.buildSSHArgs()
	args = append(args,
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "ConnectTimeout=10",
		fmt.Sprintf("%s@%s", c.user, c.host),
		command,
	)

	cmd := exec.CommandContext(ctx, "ssh", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// RunCommandOutput runs a command on the remote server and returns the output.
func (c *Client) RunCommandOutput(ctx context.Context, command string) (string, error) {
	args := c.buildSSHArgs()
	args = append(args,
		"-o", "StrictHostKeyChecking=accept-new",
		"-o", "ConnectTimeout=10",
		fmt.Sprintf("%s@%s", c.user, c.host),
		command,
	)

	cmd := exec.CommandContext(ctx, "ssh", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("remote command failed: %w\nOutput: %s", err, strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
}

// CheckVessel verifies that Vessel is installed on the remote server.
func (c *Client) CheckVessel(ctx context.Context) error {
	output, err := c.RunCommandOutput(ctx, "vessel version 2>/dev/null || vessel --version 2>/dev/null || echo VESSEL_NOT_FOUND")
	if err != nil || strings.Contains(output, "VESSEL_NOT_FOUND") {
		return fmt.Errorf("vessel not found on remote server %s@%s", c.user, c.host)
	}
	return nil
}

// CheckDaemon verifies that the Vessel daemon is running on the remote server.
func (c *Client) CheckDaemon(ctx context.Context) error {
	_, err := c.RunCommandOutput(ctx, "test -S /var/lib/vessel/run/vessel.sock && echo OK || echo NOT_RUNNING")
	if err != nil {
		return fmt.Errorf("vessel daemon not running on remote server %s@%s: %w", c.user, c.host, err)
	}
	return nil
}

// Host returns the SSH host.
func (c *Client) Host() string {
	return c.host
}

// User returns the SSH user.
func (c *Client) User() string {
	return c.user
}

// Port returns the SSH port.
func (c *Client) Port() int {
	return c.port
}

// SocketPath returns the local forwarded socket path (empty if not connected).
func (c *Client) SocketPath() string {
	return c.socketPath
}

// buildSSHArgs returns the common SSH arguments.
func (c *Client) buildSSHArgs() []string {
	args := []string{}
	if c.keyFile != "" {
		args = append(args, "-i", c.keyFile)
	}
	args = append(args, "-p", fmt.Sprintf("%d", c.port))
	return args
}

// ParseTarget parses a "user@host" or "host" string into user and host parts.
func ParseTarget(target string) (user, host string) {
	if strings.Contains(target, "@") {
		parts := strings.SplitN(target, "@", 2)
		return parts[0], parts[1]
	}
	return "", target
}
