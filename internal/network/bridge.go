package network

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
)

// BridgeName is the default bridge interface name.
const BridgeName = "vessel0"

// BridgeSubnet is the bridge IP and subnet in CIDR notation.
const BridgeSubnet = "10.88.0.1/16"

// BridgeNetwork manages the Linux bridge for container networking.
type BridgeNetwork struct {
	name   string // Bridge interface name (e.g., "vessel0")
	subnet string // Bridge IP/mask in CIDR notation (e.g., "10.88.0.1/16")
	logger *slog.Logger
}

// NewBridgeNetwork creates a new BridgeNetwork with the given name and subnet.
func NewBridgeNetwork(name, subnet string, logger *slog.Logger) *BridgeNetwork {
	return &BridgeNetwork{
		name:   name,
		subnet: subnet,
		logger: logger,
	}
}

// Setup creates and configures the Linux bridge, enables IP forwarding,
// and sets up NAT/forwarding iptables rules.
func (b *BridgeNetwork) Setup() error {
	// If bridge already exists, nothing to do
	if b.bridgeExists() {
		b.logger.Info("bridge already exists", "name", b.name)
		return nil
	}

	// Create the bridge interface
	if err := b.run("ip", "link", "add", b.name, "type", "bridge"); err != nil {
		return fmt.Errorf("failed to create bridge: %w", err)
	}

	// Assign IP address to the bridge
	if err := b.run("ip", "addr", "add", b.subnet, "dev", b.name); err != nil {
		// Clean up on failure
		b.run("ip", "link", "delete", b.name)
		return fmt.Errorf("failed to assign IP to bridge: %w", err)
	}

	// Bring up the bridge
	if err := b.run("ip", "link", "set", b.name, "up"); err != nil {
		b.run("ip", "link", "delete", b.name)
		return fmt.Errorf("failed to bring up bridge: %w", err)
	}

	// Enable IP forwarding
	if err := os.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte("1"), 0644); err != nil {
		b.logger.Warn("failed to enable IP forwarding", "error", err)
	}

	// Set up NAT (masquerade) for outbound traffic from the container subnet
	b.setupNAT()

	// Set up forwarding rules
	b.setupForwarding()

	b.logger.Info("bridge setup complete", "name", b.name, "subnet", b.subnet)
	return nil
}

// Teardown removes the bridge interface and associated iptables rules.
func (b *BridgeNetwork) Teardown() error {
	// Remove iptables rules first
	b.cleanupIPTables()

	// Bring down and delete the bridge
	b.run("ip", "link", "set", b.name, "down")
	if err := b.run("ip", "link", "delete", b.name); err != nil {
		return fmt.Errorf("failed to delete bridge: %w", err)
	}

	b.logger.Info("bridge teardown complete", "name", b.name)
	return nil
}

// bridgeExists checks if the bridge interface exists.
func (b *BridgeNetwork) bridgeExists() bool {
	_, err := os.Stat(filepath.Join("/sys/class/net", b.name))
	return err == nil
}

// setupNAT adds a MASQUERADE rule for outbound traffic from the container subnet.
func (b *BridgeNetwork) setupNAT() {
	// Check if rule already exists
	if err := b.run("iptables", "-t", "nat", "-C", "POSTROUTING",
		"-s", DefaultSubnet, "!", "-o", b.name, "-j", "MASQUERADE"); err != nil {
		// Rule doesn't exist, add it
		if err := b.run("iptables", "-t", "nat", "-A", "POSTROUTING",
			"-s", DefaultSubnet, "!", "-o", b.name, "-j", "MASQUERADE"); err != nil {
			b.logger.Warn("failed to add NAT rule", "error", err)
		}
	}
}

// setupForwarding adds iptables FORWARD rules to allow traffic through the bridge.
func (b *BridgeNetwork) setupForwarding() {
	// Allow traffic from bridge
	if err := b.run("iptables", "-C", "FORWARD", "-i", b.name, "-j", "ACCEPT"); err != nil {
		if err := b.run("iptables", "-A", "FORWARD", "-i", b.name, "-j", "ACCEPT"); err != nil {
			b.logger.Warn("failed to add forward rule (inbound)", "error", err)
		}
	}

	// Allow traffic to bridge
	if err := b.run("iptables", "-C", "FORWARD", "-o", b.name, "-j", "ACCEPT"); err != nil {
		if err := b.run("iptables", "-A", "FORWARD", "-o", b.name, "-j", "ACCEPT"); err != nil {
			b.logger.Warn("failed to add forward rule (outbound)", "error", err)
		}
	}
}

// cleanupIPTables removes iptables rules associated with the bridge.
func (b *BridgeNetwork) cleanupIPTables() {
	// Remove NAT rule
	b.run("iptables", "-t", "nat", "-D", "POSTROUTING",
		"-s", DefaultSubnet, "!", "-o", b.name, "-j", "MASQUERADE")

	// Remove forwarding rules
	b.run("iptables", "-D", "FORWARD", "-i", b.name, "-j", "ACCEPT")
	b.run("iptables", "-D", "FORWARD", "-o", b.name, "-j", "ACCEPT")
}

// run executes a command and returns any error.
func (b *BridgeNetwork) run(cmd string, args ...string) error {
	c := exec.Command(cmd, args...)
	output, err := c.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %v failed: %w: %s", cmd, args, err, output)
	}
	return nil
}
