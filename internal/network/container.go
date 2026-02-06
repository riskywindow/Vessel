package network

import (
	"context"
	"fmt"
	"net"
	"os/exec"
)

// SetupContainerNetwork creates a veth pair, assigns an IP from the allocator,
// attaches the host-side veth to the bridge, moves the container-side veth into
// the container's network namespace, and configures the interface inside the namespace.
func (m *LinuxNetworkManager) SetupContainerNetwork(ctx context.Context, containerID string, pid int) (*ContainerNetwork, error) {
	// Allocate an IP
	ip, err := m.allocator.Allocate(containerID)
	if err != nil {
		return nil, fmt.Errorf("failed to allocate IP: %w", err)
	}

	shortID := containerID[:12]
	vethHost := "veth-" + shortID[:8]
	vethContainer := "eth0"

	// Create veth pair
	if err := m.run("ip", "link", "add", vethHost, "type", "veth", "peer", "name", vethContainer); err != nil {
		m.allocator.Release(containerID)
		return nil, fmt.Errorf("failed to create veth pair: %w", err)
	}

	// Attach host side to bridge
	if err := m.run("ip", "link", "set", vethHost, "master", m.bridge.name); err != nil {
		m.run("ip", "link", "delete", vethHost)
		m.allocator.Release(containerID)
		return nil, fmt.Errorf("failed to attach to bridge: %w", err)
	}

	// Move container side into container's network namespace
	pidStr := fmt.Sprintf("%d", pid)
	if err := m.run("ip", "link", "set", vethContainer, "netns", pidStr); err != nil {
		m.run("ip", "link", "delete", vethHost)
		m.allocator.Release(containerID)
		return nil, fmt.Errorf("failed to move veth to namespace: %w", err)
	}

	// Helper to run commands inside the container's network namespace
	nsenter := func(args ...string) error {
		fullArgs := append([]string{"-t", pidStr, "-n", "--"}, args...)
		return m.run("nsenter", fullArgs...)
	}

	// Assign IP inside the container
	if err := nsenter("ip", "addr", "add", ip.String()+"/16", "dev", vethContainer); err != nil {
		m.run("ip", "link", "delete", vethHost)
		m.allocator.Release(containerID)
		return nil, fmt.Errorf("failed to assign IP in container: %w", err)
	}

	// Bring up eth0 inside container
	if err := nsenter("ip", "link", "set", vethContainer, "up"); err != nil {
		m.run("ip", "link", "delete", vethHost)
		m.allocator.Release(containerID)
		return nil, fmt.Errorf("failed to bring up eth0: %w", err)
	}

	// Bring up loopback inside container
	if err := nsenter("ip", "link", "set", "lo", "up"); err != nil {
		m.logger.Warn("failed to bring up loopback", "container", shortID, "error", err)
	}

	// Add default route inside container
	gateway := m.allocator.Gateway()
	if err := nsenter("ip", "route", "add", "default", "via", gateway.String()); err != nil {
		m.logger.Warn("failed to add default route", "container", shortID, "error", err)
	}

	// Bring up the host-side veth
	if err := m.run("ip", "link", "set", vethHost, "up"); err != nil {
		m.run("ip", "link", "delete", vethHost)
		m.allocator.Release(containerID)
		return nil, fmt.Errorf("failed to bring up host veth: %w", err)
	}

	netInfo := &ContainerNetwork{
		ContainerID:   containerID,
		IP:            ip,
		Gateway:       gateway,
		Bridge:        m.bridge.name,
		VethHost:      vethHost,
		VethContainer: vethContainer,
	}

	m.logger.Info("container network setup complete",
		"container", shortID, "ip", ip.String(), "veth", vethHost)

	return netInfo, nil
}

// TeardownContainerNetwork removes the veth pair and releases the IP for a container.
func (m *LinuxNetworkManager) TeardownContainerNetwork(ctx context.Context, containerID string) error {
	shortID := containerID[:12]
	vethHost := "veth-" + shortID[:8]

	// Delete veth pair (deleting one side deletes both)
	if err := m.run("ip", "link", "delete", vethHost); err != nil {
		m.logger.Warn("failed to delete veth", "veth", vethHost, "error", err)
	}

	// Release the IP
	if err := m.allocator.Release(containerID); err != nil {
		m.logger.Warn("failed to release IP", "container", shortID, "error", err)
	}

	m.logger.Info("container network teardown complete", "container", shortID)
	return nil
}

// run executes a command and returns any error.
func (m *LinuxNetworkManager) run(cmd string, args ...string) error {
	c := exec.Command(cmd, args...)
	output, err := c.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %v failed: %w: %s", cmd, args, err, output)
	}
	return nil
}

// GetContainerIP returns the IP address allocated to a container, if any.
func (m *LinuxNetworkManager) GetContainerIP(containerID string) (net.IP, bool) {
	return m.allocator.GetIP(containerID)
}
