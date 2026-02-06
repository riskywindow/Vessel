package network

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/vessel/vessel/internal/paths"
)

// LinuxNetworkManager implements NetworkManager using Linux bridge networking.
type LinuxNetworkManager struct {
	bridge    *BridgeNetwork
	allocator *IPAllocator
	routes    map[string][]RouteTarget // hostname → targets (for Session 3)
	mu        sync.RWMutex
	logger    *slog.Logger
}

// NewLinuxNetworkManager creates a new LinuxNetworkManager.
func NewLinuxNetworkManager(logger *slog.Logger) (*LinuxNetworkManager, error) {
	// Ensure network directory exists
	netDir := paths.NetworkDir
	if err := os.MkdirAll(netDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create network dir: %w", err)
	}

	allocator, err := NewIPAllocator(filepath.Join(netDir, "allocations.json"))
	if err != nil {
		return nil, fmt.Errorf("failed to create IP allocator: %w", err)
	}

	bridge := NewBridgeNetwork(BridgeName, BridgeSubnet, logger)

	return &LinuxNetworkManager{
		bridge:    bridge,
		allocator: allocator,
		routes:    make(map[string][]RouteTarget),
		logger:    logger,
	}, nil
}

// Start initializes the network bridge.
func (m *LinuxNetworkManager) Start(ctx context.Context) error {
	return m.bridge.Setup()
}

// Stop gracefully shuts down networking.
// The bridge is intentionally NOT torn down — containers may still be using it,
// and it persists across daemon restarts.
func (m *LinuxNetworkManager) Stop() error {
	return nil
}

// RegisterRoute adds a hostname → container routing entry.
// Stub implementation — full reverse proxy comes in Session 3.
func (m *LinuxNetworkManager) RegisterRoute(hostname string, target RouteTarget) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.routes[hostname] = append(m.routes[hostname], target)
	m.logger.Info("route registered",
		"hostname", hostname,
		"container", target.ContainerID,
		"ip", target.IP.String(),
		"port", target.Port)
	return nil
}

// DeregisterRoute removes a container from routing for a given hostname.
// If hostname is empty, the container is removed from all routes.
func (m *LinuxNetworkManager) DeregisterRoute(hostname string, containerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if hostname == "" {
		// Remove container from all routes
		for h, targets := range m.routes {
			var filtered []RouteTarget
			for _, t := range targets {
				if t.ContainerID != containerID {
					filtered = append(filtered, t)
				}
			}
			if len(filtered) > 0 {
				m.routes[h] = filtered
			} else {
				delete(m.routes, h)
			}
		}
	} else {
		targets := m.routes[hostname]
		var filtered []RouteTarget
		for _, t := range targets {
			if t.ContainerID != containerID {
				filtered = append(filtered, t)
			}
		}
		if len(filtered) > 0 {
			m.routes[hostname] = filtered
		} else {
			delete(m.routes, hostname)
		}
	}

	m.logger.Info("route deregistered", "hostname", hostname, "container", containerID)
	return nil
}

// GetRoutes returns the current route table. Used for debugging/testing.
func (m *LinuxNetworkManager) GetRoutes() map[string][]RouteTarget {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string][]RouteTarget, len(m.routes))
	for k, v := range m.routes {
		targets := make([]RouteTarget, len(v))
		copy(targets, v)
		result[k] = targets
	}
	return result
}
