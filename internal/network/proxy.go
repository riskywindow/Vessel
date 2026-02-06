// Package network handles networking, proxy, and TLS.
package network

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"
)

// ReverseProxy routes incoming HTTP requests to containers by hostname.
type ReverseProxy struct {
	mu     sync.RWMutex
	routes map[string][]RouteTarget
	server *http.Server
	logger *slog.Logger
}

// NewReverseProxy creates a new ReverseProxy.
func NewReverseProxy(logger *slog.Logger) *ReverseProxy {
	return &ReverseProxy{
		routes: make(map[string][]RouteTarget),
		logger: logger,
	}
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
	r.Header.Set("X-Forwarded-Proto", "http")
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

// Start begins serving HTTP traffic on the given address.
func (p *ReverseProxy) Start(addr string) error {
	p.server = &http.Server{
		Addr:         addr,
		Handler:      p,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	p.logger.Info("starting reverse proxy", "addr", addr)
	return p.server.ListenAndServe()
}

// Stop gracefully shuts down the proxy.
func (p *ReverseProxy) Stop(ctx context.Context) error {
	if p.server != nil {
		return p.server.Shutdown(ctx)
	}
	return nil
}
