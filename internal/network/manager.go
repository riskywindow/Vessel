package network

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/vessel/vessel/internal/paths"
)

// DefaultProxyAddr is the default address the reverse proxy listens on.
const DefaultProxyAddr = ":80"

// LinuxNetworkManager implements NetworkManager using Linux bridge networking.
type LinuxNetworkManager struct {
	bridge    *BridgeNetwork
	allocator *IPAllocator
	proxy     *ReverseProxy
	mu        sync.RWMutex
	logger    *slog.Logger
	proxyAddr string
}

// NewLinuxNetworkManager creates a new LinuxNetworkManager.
func NewLinuxNetworkManager(logger *slog.Logger) (*LinuxNetworkManager, error) {
	return NewLinuxNetworkManagerWithProxy(logger, DefaultProxyAddr)
}

// NewLinuxNetworkManagerWithProxy creates a new LinuxNetworkManager with a custom proxy address.
func NewLinuxNetworkManagerWithProxy(logger *slog.Logger, proxyAddr string) (*LinuxNetworkManager, error) {
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
	proxy := NewReverseProxy(logger)

	return &LinuxNetworkManager{
		bridge:    bridge,
		allocator: allocator,
		proxy:     proxy,
		logger:    logger,
		proxyAddr: proxyAddr,
	}, nil
}

// Start initializes the network bridge and starts the reverse proxy.
func (m *LinuxNetworkManager) Start(ctx context.Context) error {
	if err := m.bridge.Setup(); err != nil {
		return err
	}

	// Start proxy in background
	go func() {
		if err := m.proxy.Start(m.proxyAddr); err != nil && err != http.ErrServerClosed {
			m.logger.Error("reverse proxy error", "error", err)
		}
	}()

	m.logger.Info("reverse proxy started", "addr", m.proxyAddr)
	return nil
}

// Stop gracefully shuts down the reverse proxy.
// The bridge is intentionally NOT torn down — containers may still be using it,
// and it persists across daemon restarts.
func (m *LinuxNetworkManager) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return m.proxy.Stop(ctx)
}

// RegisterRoute adds a hostname → container routing entry via the reverse proxy.
func (m *LinuxNetworkManager) RegisterRoute(hostname string, target RouteTarget) error {
	m.proxy.RegisterRoute(hostname, target)
	return nil
}

// DeregisterRoute removes a container from routing for a given hostname.
// If hostname is empty, the container is removed from all routes.
func (m *LinuxNetworkManager) DeregisterRoute(hostname string, containerID string) error {
	m.proxy.DeregisterRoute(hostname, containerID)
	return nil
}

// GetRoutes returns the current route table from the reverse proxy.
func (m *LinuxNetworkManager) GetRoutes() map[string][]RouteTarget {
	return m.proxy.GetRoutes()
}

// GetProxy returns the underlying reverse proxy for direct access (e.g., testing).
func (m *LinuxNetworkManager) GetProxy() *ReverseProxy {
	return m.proxy
}
