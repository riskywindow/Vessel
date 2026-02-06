package network

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/vessel/vessel/internal/paths"
)

// DefaultProxyAddr is the default address the reverse proxy listens on.
const DefaultProxyAddr = ":80"

// DefaultDNSAddr is the address the internal DNS server listens on.
const DefaultDNSAddr = "10.88.0.1:53"

// LinuxNetworkManager implements NetworkManager using Linux bridge networking.
type LinuxNetworkManager struct {
	bridge    *BridgeNetwork
	allocator *IPAllocator
	proxy     *ReverseProxy
	dns       *DNSServer
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
	dnsServer := NewDNSServer(logger, "")

	return &LinuxNetworkManager{
		bridge:    bridge,
		allocator: allocator,
		proxy:     proxy,
		dns:       dnsServer,
		logger:    logger,
		proxyAddr: proxyAddr,
	}, nil
}

// Start initializes the network bridge, starts the DNS server, and starts the reverse proxy.
func (m *LinuxNetworkManager) Start(ctx context.Context) error {
	if err := m.bridge.Setup(); err != nil {
		return err
	}

	// Start DNS server in background
	if m.dns != nil {
		go func() {
			if err := m.dns.Start(DefaultDNSAddr); err != nil {
				m.logger.Error("DNS server error", "error", err)
			}
		}()
		m.logger.Info("DNS server started", "addr", DefaultDNSAddr)
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

// Stop gracefully shuts down the reverse proxy and DNS server.
// The bridge is intentionally NOT torn down — containers may still be using it,
// and it persists across daemon restarts.
func (m *LinuxNetworkManager) Stop() error {
	var errs []error

	// Stop DNS server
	if m.dns != nil {
		if err := m.dns.Stop(); err != nil {
			errs = append(errs, fmt.Errorf("dns shutdown: %w", err))
		}
	}

	// Stop reverse proxy
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := m.proxy.Stop(ctx); err != nil {
		errs = append(errs, fmt.Errorf("proxy shutdown: %w", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("shutdown errors: %v", errs)
	}
	return nil
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

// GetDNS returns the underlying DNS server for direct access (e.g., testing).
func (m *LinuxNetworkManager) GetDNS() *DNSServer {
	return m.dns
}

// RegisterAppDNS registers DNS records for an app.
func (m *LinuxNetworkManager) RegisterAppDNS(appName string, ips []net.IP) {
	if m.dns != nil {
		m.dns.RegisterApp(appName, ips)
	}
}

// DeregisterAppDNS removes DNS records for an app.
func (m *LinuxNetworkManager) DeregisterAppDNS(appName string) {
	if m.dns != nil {
		m.dns.DeregisterApp(appName)
	}
}

// LoadCustomCert loads a custom TLS certificate for a hostname.
func (m *LinuxNetworkManager) LoadCustomCert(hostname, certPath, keyPath string) error {
	return m.proxy.LoadCustomCert(hostname, certPath, keyPath)
}

// GetCustomCerts returns hostnames with custom certificates.
func (m *LinuxNetworkManager) GetCustomCerts() []string {
	return m.proxy.GetCustomCerts()
}

// IsTLSEnabled returns whether TLS is enabled on the proxy.
func (m *LinuxNetworkManager) IsTLSEnabled() bool {
	return m.proxy.IsTLSEnabled()
}
