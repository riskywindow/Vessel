// Package network handles container networking, reverse proxy routing, and TLS.
package network

import (
	"context"
	"net"
)

// NetworkManager handles container networking and reverse proxy routing.
type NetworkManager interface {
	// SetupContainerNetwork creates a veth pair, assigns an IP, and configures
	// networking inside the container's network namespace.
	SetupContainerNetwork(ctx context.Context, containerID string, pid int) (*ContainerNetwork, error)

	// TeardownContainerNetwork removes the veth pair and releases the IP.
	TeardownContainerNetwork(ctx context.Context, containerID string) error

	// RegisterRoute adds a hostname → container mapping to the reverse proxy.
	RegisterRoute(hostname string, target RouteTarget) error

	// DeregisterRoute removes a container from routing for the given hostname.
	// If hostname is empty, the container is removed from all routes.
	DeregisterRoute(hostname string, containerID string) error

	// Start initializes the network bridge and any required infrastructure.
	Start(ctx context.Context) error

	// Stop gracefully shuts down networking (does not tear down the bridge).
	Stop() error
}

// ContainerNetwork holds the network configuration assigned to a container.
type ContainerNetwork struct {
	ContainerID   string `json:"container_id"`
	IP            net.IP `json:"ip"`
	Gateway       net.IP `json:"gateway"`
	Bridge        string `json:"bridge"`
	VethHost      string `json:"veth_host"`      // Host-side veth name
	VethContainer string `json:"veth_container"` // Container-side veth name (always "eth0")
}

// RouteTarget represents where to route traffic for a hostname.
type RouteTarget struct {
	ContainerID string `json:"container_id"`
	IP          net.IP `json:"ip"`
	Port        int    `json:"port"`
	Weight      int    `json:"weight"` // For load balancing (default 1)
}
