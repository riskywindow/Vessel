package network

import (
	"net"
	"testing"
	"time"

	"github.com/miekg/dns"
)

func TestDNS_NewDNSServer(t *testing.T) {
	d := NewDNSServer(testLogger(), "")
	if d.upstream != DefaultUpstreamDNS {
		t.Errorf("expected default upstream %s, got %s", DefaultUpstreamDNS, d.upstream)
	}

	d2 := NewDNSServer(testLogger(), "1.1.1.1:53")
	if d2.upstream != "1.1.1.1:53" {
		t.Errorf("expected custom upstream 1.1.1.1:53, got %s", d2.upstream)
	}
}

func TestDNS_RegisterDeregisterApp(t *testing.T) {
	d := NewDNSServer(testLogger(), "")

	ips := []net.IP{net.ParseIP("10.88.0.2"), net.ParseIP("10.88.0.3")}
	d.RegisterApp("myapp", ips)

	records := d.GetRecords()
	got, ok := records["myapp.vessel.internal."]
	if !ok {
		t.Fatal("expected record for myapp.vessel.internal.")
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 IPs, got %d", len(got))
	}
	if !got[0].Equal(net.ParseIP("10.88.0.2")) {
		t.Errorf("expected 10.88.0.2, got %s", got[0])
	}

	d.DeregisterApp("myapp")
	records = d.GetRecords()
	if _, ok := records["myapp.vessel.internal."]; ok {
		t.Error("expected record to be removed after deregister")
	}
}

func TestDNS_RegisterApp_CaseInsensitive(t *testing.T) {
	d := NewDNSServer(testLogger(), "")

	d.RegisterApp("MyApp", []net.IP{net.ParseIP("10.88.0.2")})

	// Lookup should be case-insensitive
	ips, ok := d.Resolve("myapp.vessel.internal.")
	if !ok {
		t.Fatal("expected record for myapp.vessel.internal.")
	}
	if len(ips) != 1 {
		t.Fatalf("expected 1 IP, got %d", len(ips))
	}
}

func TestDNS_Resolve(t *testing.T) {
	d := NewDNSServer(testLogger(), "")

	d.RegisterApp("web", []net.IP{net.ParseIP("10.88.0.5")})

	ips, ok := d.Resolve("web.vessel.internal.")
	if !ok {
		t.Fatal("expected to resolve web.vessel.internal.")
	}
	if len(ips) != 1 || !ips[0].Equal(net.ParseIP("10.88.0.5")) {
		t.Errorf("unexpected IPs: %v", ips)
	}

	_, ok = d.Resolve("nonexistent.vessel.internal.")
	if ok {
		t.Error("should not resolve nonexistent app")
	}
}

func TestDNS_GetRecords_Isolation(t *testing.T) {
	d := NewDNSServer(testLogger(), "")

	d.RegisterApp("app1", []net.IP{net.ParseIP("10.88.0.2")})

	// Modify the returned records
	records := d.GetRecords()
	records["app1.vessel.internal."][0] = net.ParseIP("99.99.99.99")

	// Original should be unaffected
	original := d.GetRecords()
	if original["app1.vessel.internal."][0].Equal(net.ParseIP("99.99.99.99")) {
		t.Error("GetRecords did not return an isolated copy")
	}
}

func TestDNS_RegisterApp_OverwritesPrevious(t *testing.T) {
	d := NewDNSServer(testLogger(), "")

	d.RegisterApp("api", []net.IP{net.ParseIP("10.88.0.2")})
	d.RegisterApp("api", []net.IP{net.ParseIP("10.88.0.3"), net.ParseIP("10.88.0.4")})

	records := d.GetRecords()
	got := records["api.vessel.internal."]
	if len(got) != 2 {
		t.Fatalf("expected 2 IPs after overwrite, got %d", len(got))
	}
}

func TestDNS_HandleDNS_InternalRecord(t *testing.T) {
	d := NewDNSServer(testLogger(), "")
	d.RegisterApp("web", []net.IP{net.ParseIP("10.88.0.5").To4()})

	// Start DNS server on a random port
	listener, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	addr := listener.LocalAddr().String()
	listener.Close()

	go d.Start(addr)
	time.Sleep(100 * time.Millisecond)
	defer d.Stop()

	// Query for the app
	c := new(dns.Client)
	c.Timeout = 2 * time.Second
	m := new(dns.Msg)
	m.SetQuestion("web.vessel.internal.", dns.TypeA)

	resp, _, err := c.Exchange(m, addr)
	if err != nil {
		t.Fatalf("DNS query failed: %v", err)
	}

	if len(resp.Answer) != 1 {
		t.Fatalf("expected 1 answer, got %d", len(resp.Answer))
	}

	aRecord, ok := resp.Answer[0].(*dns.A)
	if !ok {
		t.Fatalf("expected A record, got %T", resp.Answer[0])
	}
	if !aRecord.A.Equal(net.ParseIP("10.88.0.5")) {
		t.Errorf("expected 10.88.0.5, got %s", aRecord.A)
	}
}

func TestDNS_HandleDNS_NXDOMAIN(t *testing.T) {
	d := NewDNSServer(testLogger(), "")

	listener, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	addr := listener.LocalAddr().String()
	listener.Close()

	go d.Start(addr)
	time.Sleep(100 * time.Millisecond)
	defer d.Stop()

	c := new(dns.Client)
	c.Timeout = 2 * time.Second
	m := new(dns.Msg)
	m.SetQuestion("nonexistent.vessel.internal.", dns.TypeA)

	resp, _, err := c.Exchange(m, addr)
	if err != nil {
		t.Fatalf("DNS query failed: %v", err)
	}

	if resp.Rcode != dns.RcodeNameError {
		t.Errorf("expected NXDOMAIN, got rcode %d", resp.Rcode)
	}
}

func TestDNS_HandleDNS_MultipleIPs(t *testing.T) {
	d := NewDNSServer(testLogger(), "")
	d.RegisterApp("lb", []net.IP{
		net.ParseIP("10.88.0.10").To4(),
		net.ParseIP("10.88.0.11").To4(),
		net.ParseIP("10.88.0.12").To4(),
	})

	listener, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	addr := listener.LocalAddr().String()
	listener.Close()

	go d.Start(addr)
	time.Sleep(100 * time.Millisecond)
	defer d.Stop()

	c := new(dns.Client)
	c.Timeout = 2 * time.Second
	m := new(dns.Msg)
	m.SetQuestion("lb.vessel.internal.", dns.TypeA)

	resp, _, err := c.Exchange(m, addr)
	if err != nil {
		t.Fatalf("DNS query failed: %v", err)
	}

	if len(resp.Answer) != 3 {
		t.Fatalf("expected 3 A records, got %d", len(resp.Answer))
	}
}

func TestDNS_HandleDNS_ForwardExternal(t *testing.T) {
	// Create a DNS server with a mock upstream that we control
	upstream := startMockUpstreamDNS(t)
	defer upstream.Shutdown()

	d := NewDNSServer(testLogger(), upstream.PacketConn.LocalAddr().String())

	listener, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	addr := listener.LocalAddr().String()
	listener.Close()

	go d.Start(addr)
	time.Sleep(100 * time.Millisecond)
	defer d.Stop()

	// Query for an external domain — should be forwarded to upstream
	c := new(dns.Client)
	c.Timeout = 2 * time.Second
	m := new(dns.Msg)
	m.SetQuestion("example.com.", dns.TypeA)

	resp, _, err := c.Exchange(m, addr)
	if err != nil {
		t.Fatalf("DNS query failed: %v", err)
	}

	if len(resp.Answer) != 1 {
		t.Fatalf("expected 1 answer from upstream, got %d", len(resp.Answer))
	}

	aRecord, ok := resp.Answer[0].(*dns.A)
	if !ok {
		t.Fatalf("expected A record, got %T", resp.Answer[0])
	}
	if !aRecord.A.Equal(net.ParseIP("93.184.216.34")) {
		t.Errorf("expected upstream response 93.184.216.34, got %s", aRecord.A)
	}
}

func TestDNS_StartStop(t *testing.T) {
	d := NewDNSServer(testLogger(), "")

	listener, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	addr := listener.LocalAddr().String()
	listener.Close()

	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Start(addr)
	}()
	time.Sleep(100 * time.Millisecond)

	if err := d.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestDNS_StopNil(t *testing.T) {
	d := NewDNSServer(testLogger(), "")
	// Stop without starting should not error
	if err := d.Stop(); err != nil {
		t.Errorf("Stop on unstarted server should not error: %v", err)
	}
}

// startMockUpstreamDNS starts a mock upstream DNS server that responds to
// example.com with a fixed IP.
func startMockUpstreamDNS(t *testing.T) *dns.Server {
	t.Helper()

	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen for mock upstream: %v", err)
	}

	mux := dns.NewServeMux()
	mux.HandleFunc(".", func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)
		for _, q := range r.Question {
			if q.Qtype == dns.TypeA {
				m.Answer = append(m.Answer, &dns.A{
					Hdr: dns.RR_Header{
						Name:   q.Name,
						Rrtype: dns.TypeA,
						Class:  dns.ClassINET,
						Ttl:    60,
					},
					A: net.ParseIP("93.184.216.34").To4(),
				})
			}
		}
		w.WriteMsg(m)
	})

	server := &dns.Server{
		PacketConn: pc,
		Handler:    mux,
	}

	go server.ActivateAndServe()
	time.Sleep(50 * time.Millisecond)

	return server
}
