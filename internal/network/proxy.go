// Package network handles networking, proxy, and TLS.
package network

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"math/rand"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/acme/autocert"
)

// ProxyConfig configures the reverse proxy.
type ProxyConfig struct {
	HTTPAddr   string // e.g., ":80"
	HTTPSAddr  string // e.g., ":443"
	TLSEnabled bool
	ACMEEmail  string
	CertDir    string // e.g., "/var/lib/vessel/tls"
}

// ReverseProxy routes incoming HTTP requests to containers by hostname.
type ReverseProxy struct {
	mu          sync.RWMutex
	routes      map[string][]RouteTarget
	httpServer  *http.Server
	httpsServer *http.Server
	certManager *autocert.Manager
	customCerts map[string]*tls.Certificate
	logger      *slog.Logger
	tlsEnabled  bool
}

// NewReverseProxy creates a new ReverseProxy without TLS.
func NewReverseProxy(logger *slog.Logger) *ReverseProxy {
	return &ReverseProxy{
		routes:      make(map[string][]RouteTarget),
		customCerts: make(map[string]*tls.Certificate),
		logger:      logger,
	}
}

// NewReverseProxyWithTLS creates a new ReverseProxy with TLS support.
func NewReverseProxyWithTLS(logger *slog.Logger, cfg ProxyConfig) *ReverseProxy {
	p := &ReverseProxy{
		routes:      make(map[string][]RouteTarget),
		customCerts: make(map[string]*tls.Certificate),
		logger:      logger,
		tlsEnabled:  cfg.TLSEnabled,
	}

	if cfg.TLSEnabled && cfg.ACMEEmail != "" && cfg.CertDir != "" {
		p.certManager = &autocert.Manager{
			Prompt:     autocert.AcceptTOS,
			Email:      cfg.ACMEEmail,
			Cache:      autocert.DirCache(cfg.CertDir),
			HostPolicy: p.hostPolicy,
		}
	}

	return p
}

// hostPolicy only allows certificates for registered routes.
func (p *ReverseProxy) hostPolicy(_ context.Context, host string) error {
	p.mu.RLock()
	_, ok := p.routes[host]
	p.mu.RUnlock()

	if !ok {
		return fmt.Errorf("host %s not in routes", host)
	}
	return nil
}

// getCertificate checks custom certs first, then falls back to ACME.
func (p *ReverseProxy) getCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	p.mu.RLock()
	if cert, ok := p.customCerts[hello.ServerName]; ok {
		p.mu.RUnlock()
		return cert, nil
	}
	p.mu.RUnlock()

	if p.certManager != nil {
		return p.certManager.GetCertificate(hello)
	}

	return nil, fmt.Errorf("no certificate for %s", hello.ServerName)
}

// LoadCustomCert loads a custom TLS certificate for a hostname.
func (p *ReverseProxy) LoadCustomCert(hostname, certPath, keyPath string) error {
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return fmt.Errorf("failed to load certificate: %w", err)
	}

	p.mu.Lock()
	p.customCerts[hostname] = &cert
	p.mu.Unlock()

	p.logger.Info("loaded custom certificate", "hostname", hostname)
	return nil
}

// GetCustomCerts returns the hostnames that have custom certificates loaded.
func (p *ReverseProxy) GetCustomCerts() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var hosts []string
	for h := range p.customCerts {
		hosts = append(hosts, h)
	}
	return hosts
}

// IsTLSEnabled returns whether TLS is enabled.
func (p *ReverseProxy) IsTLSEnabled() bool {
	return p.tlsEnabled
}

// HasACME returns whether ACME certificate management is configured.
func (p *ReverseProxy) HasACME() bool {
	return p.certManager != nil
}

// ServeHTTP implements http.Handler and dispatches requests to the correct backend.
func (p *ReverseProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Health check endpoint for the proxy itself
	if r.URL.Path == "/__vessel/health" {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
		return
	}

	// Extract hostname (strip port if present)
	hostname := r.Host
	if h, _, err := net.SplitHostPort(hostname); err == nil {
		hostname = h
	}

	p.mu.RLock()
	targets := p.routes[hostname]
	p.mu.RUnlock()

	if len(targets) == 0 {
		p.logger.Warn("no route for host", "host", hostname)
		http.Error(w, "No route for host: "+hostname, http.StatusBadGateway)
		return
	}

	// Simple random selection for load balancing
	target := targets[rand.Intn(len(targets))]

	targetURL := &url.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("%s:%d", target.IP.String(), target.Port),
	}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		p.logger.Error("proxy error", "host", hostname, "target", targetURL.String(), "error", err)
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
	}

	// Set forwarded headers for backends
	r.Header.Set("X-Forwarded-Host", r.Host)
	if r.TLS != nil {
		r.Header.Set("X-Forwarded-Proto", "https")
	} else {
		r.Header.Set("X-Forwarded-Proto", "http")
	}
	if clientIP, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		r.Header.Set("X-Real-IP", clientIP)
	} else {
		r.Header.Set("X-Real-IP", r.RemoteAddr)
	}

	p.logger.Debug("proxying request", "host", hostname, "target", targetURL.String(), "path", r.URL.Path)
	proxy.ServeHTTP(w, r)
}

// RegisterRoute adds a container as a backend for a hostname.
// Duplicate container IDs for the same hostname are ignored.
func (p *ReverseProxy) RegisterRoute(hostname string, target RouteTarget) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Check for duplicate
	for _, t := range p.routes[hostname] {
		if t.ContainerID == target.ContainerID {
			return
		}
	}

	p.routes[hostname] = append(p.routes[hostname], target)
	shortID := target.ContainerID
	if len(shortID) > 12 {
		shortID = shortID[:12]
	}
	p.logger.Info("route registered", "hostname", hostname, "container", shortID, "ip", target.IP.String(), "port", target.Port)
}

// DeregisterRoute removes a container from routing.
// If hostname is empty, the container is removed from all hostnames.
func (p *ReverseProxy) DeregisterRoute(hostname string, containerID string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	shortID := containerID
	if len(shortID) > 12 {
		shortID = shortID[:12]
	}

	if hostname == "" {
		// Remove from all routes
		for h, targets := range p.routes {
			var filtered []RouteTarget
			for _, t := range targets {
				if t.ContainerID != containerID {
					filtered = append(filtered, t)
				}
			}
			if len(filtered) > 0 {
				p.routes[h] = filtered
			} else {
				delete(p.routes, h)
			}
		}
		p.logger.Info("container deregistered from all routes", "container", shortID)
	} else {
		targets := p.routes[hostname]
		var filtered []RouteTarget
		for _, t := range targets {
			if t.ContainerID != containerID {
				filtered = append(filtered, t)
			}
		}
		if len(filtered) > 0 {
			p.routes[hostname] = filtered
		} else {
			delete(p.routes, hostname)
		}
		p.logger.Info("route deregistered", "hostname", hostname, "container", shortID)
	}
}

// GetRoutes returns a copy of the current route table.
func (p *ReverseProxy) GetRoutes() map[string][]RouteTarget {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make(map[string][]RouteTarget, len(p.routes))
	for k, v := range p.routes {
		result[k] = append([]RouteTarget{}, v...)
	}
	return result
}

// Start begins serving HTTP traffic on the given address (non-TLS mode).
func (p *ReverseProxy) Start(addr string) error {
	p.httpServer = &http.Server{
		Addr:         addr,
		Handler:      p,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	p.logger.Info("starting reverse proxy", "addr", addr)
	return p.httpServer.ListenAndServe()
}

// StartTLS starts both HTTP and HTTPS servers with TLS support.
// The HTTP server handles ACME challenges and redirects to HTTPS.
// The HTTPS server terminates TLS and proxies to backends.
func (p *ReverseProxy) StartTLS(httpAddr, httpsAddr string) error {
	// HTTP handler — serves ACME challenges, health checks, and redirects to HTTPS
	httpHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Health check endpoint
		if r.URL.Path == "/__vessel/health" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
			return
		}

		// ACME challenge
		if p.certManager != nil && strings.HasPrefix(r.URL.Path, "/.well-known/acme-challenge/") {
			p.certManager.HTTPHandler(nil).ServeHTTP(w, r)
			return
		}

		// Redirect to HTTPS
		if p.tlsEnabled && httpsAddr != "" {
			target := "https://" + r.Host + r.URL.RequestURI()
			http.Redirect(w, r, target, http.StatusMovedPermanently)
			return
		}

		// Fallback to normal proxy
		p.ServeHTTP(w, r)
	})

	p.httpServer = &http.Server{
		Addr:         httpAddr,
		Handler:      httpHandler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	errCh := make(chan error, 2)

	go func() {
		p.logger.Info("starting HTTP server", "addr", httpAddr)
		if err := p.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			p.logger.Error("HTTP server error", "error", err)
			errCh <- err
		}
	}()

	if httpsAddr != "" {
		tlsConfig := &tls.Config{
			MinVersion:     tls.VersionTLS12,
			GetCertificate: p.getCertificate,
		}

		p.httpsServer = &http.Server{
			Addr:         httpsAddr,
			Handler:      p,
			TLSConfig:    tlsConfig,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  120 * time.Second,
		}

		go func() {
			p.logger.Info("starting HTTPS server", "addr", httpsAddr)
			if err := p.httpsServer.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
				p.logger.Error("HTTPS server error", "error", err)
				errCh <- err
			}
		}()
	}

	return nil
}

// Stop gracefully shuts down all servers.
func (p *ReverseProxy) Stop(ctx context.Context) error {
	var errs []error
	if p.httpServer != nil {
		if err := p.httpServer.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("http shutdown: %w", err))
		}
	}
	if p.httpsServer != nil {
		if err := p.httpsServer.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("https shutdown: %w", err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("shutdown errors: %v", errs)
	}
	return nil
}
