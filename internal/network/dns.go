package network

import (
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
)

// DNSSuffix is the suffix used for internal service discovery.
const DNSSuffix = ".vessel.internal."

// DefaultUpstreamDNS is the upstream DNS server for external domains.
const DefaultUpstreamDNS = "8.8.8.8:53"

// DNSServer provides internal DNS resolution for container-to-container communication.
// It resolves <app-name>.vessel.internal to container IPs and forwards all other
// queries to an upstream DNS server.
type DNSServer struct {
	mu       sync.RWMutex
	records  map[string][]net.IP // FQDN (with trailing dot) → IPs
	server   *dns.Server
	upstream string
	logger   *slog.Logger
}

// NewDNSServer creates a new DNSServer.
func NewDNSServer(logger *slog.Logger, upstream string) *DNSServer {
	if upstream == "" {
		upstream = DefaultUpstreamDNS
	}
	return &DNSServer{
		records:  make(map[string][]net.IP),
		upstream: upstream,
		logger:   logger,
	}
}

// RegisterApp adds DNS records for an app name mapping to the given IPs.
// The app becomes resolvable as <appName>.vessel.internal.
func (d *DNSServer) RegisterApp(appName string, ips []net.IP) {
	d.mu.Lock()
	defer d.mu.Unlock()

	hostname := strings.ToLower(appName) + DNSSuffix

	// Copy IPs to avoid mutation
	copied := make([]net.IP, len(ips))
	for i, ip := range ips {
		copied[i] = make(net.IP, len(ip))
		copy(copied[i], ip)
	}

	d.records[hostname] = copied
	d.logger.Info("DNS record registered", "hostname", hostname, "ips", ips)
}

// DeregisterApp removes DNS records for an app.
func (d *DNSServer) DeregisterApp(appName string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	hostname := strings.ToLower(appName) + DNSSuffix
	delete(d.records, hostname)
	d.logger.Info("DNS record deregistered", "hostname", hostname)
}

// GetRecords returns the current DNS records (for inspection/testing).
func (d *DNSServer) GetRecords() map[string][]net.IP {
	d.mu.RLock()
	defer d.mu.RUnlock()

	result := make(map[string][]net.IP, len(d.records))
	for k, v := range d.records {
		copied := make([]net.IP, len(v))
		for i, ip := range v {
			copied[i] = make(net.IP, len(ip))
			copy(copied[i], ip)
		}
		result[k] = copied
	}
	return result
}

// Resolve looks up IPs for the given FQDN (must include trailing dot).
func (d *DNSServer) Resolve(name string) ([]net.IP, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	ips, ok := d.records[strings.ToLower(name)]
	return ips, ok
}

// handleDNS processes DNS queries.
func (d *DNSServer) handleDNS(w dns.ResponseWriter, r *dns.Msg) {
	m := new(dns.Msg)
	m.SetReply(r)
	m.Authoritative = true

	for _, q := range r.Question {
		name := strings.ToLower(q.Name)

		if q.Qtype == dns.TypeA {
			d.mu.RLock()
			ips, ok := d.records[name]
			d.mu.RUnlock()

			if ok {
				for _, ip := range ips {
					ipv4 := ip.To4()
					if ipv4 == nil {
						continue
					}
					rr := &dns.A{
						Hdr: dns.RR_Header{
							Name:   q.Name,
							Rrtype: dns.TypeA,
							Class:  dns.ClassINET,
							Ttl:    60,
						},
						A: ipv4,
					}
					m.Answer = append(m.Answer, rr)
				}
			} else if strings.HasSuffix(name, DNSSuffix) {
				// Known domain but no record — NXDOMAIN
				m.Rcode = dns.RcodeNameError
			} else {
				// External domain — forward to upstream
				d.forwardUpstream(w, r)
				return
			}
		} else {
			// Non-A queries
			if strings.HasSuffix(name, DNSSuffix) {
				m.Rcode = dns.RcodeNameError
			} else {
				d.forwardUpstream(w, r)
				return
			}
		}
	}

	if err := w.WriteMsg(m); err != nil {
		d.logger.Error("failed to write DNS response", "error", err)
	}
}

// forwardUpstream forwards a DNS query to the upstream server.
func (d *DNSServer) forwardUpstream(w dns.ResponseWriter, r *dns.Msg) {
	c := new(dns.Client)
	c.Timeout = 5 * time.Second
	resp, _, err := c.Exchange(r, d.upstream)
	if err != nil {
		d.logger.Debug("upstream DNS query failed", "error", err)
		m := new(dns.Msg)
		m.SetReply(r)
		m.Rcode = dns.RcodeServerFailure
		w.WriteMsg(m)
		return
	}
	if resp != nil {
		resp.Id = r.Id
		if err := w.WriteMsg(resp); err != nil {
			d.logger.Error("failed to write upstream DNS response", "error", err)
		}
	}
}

// Start begins serving DNS on the given address (e.g., "10.88.0.1:53").
func (d *DNSServer) Start(addr string) error {
	mux := dns.NewServeMux()
	mux.HandleFunc(".", d.handleDNS)

	d.server = &dns.Server{
		Addr:    addr,
		Net:     "udp",
		Handler: mux,
	}

	d.logger.Info("starting DNS server", "addr", addr)
	return d.server.ListenAndServe()
}

// Stop gracefully shuts down the DNS server.
func (d *DNSServer) Stop() error {
	if d.server != nil {
		return d.server.Shutdown()
	}
	return nil
}
