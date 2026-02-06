package network

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestProxy_HealthEndpoint(t *testing.T) {
	proxy := NewReverseProxy(testLogger())

	req := httptest.NewRequest("GET", "http://localhost/__vessel/health", nil)
	w := httptest.NewRecorder()

	proxy.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "OK" {
		t.Errorf("expected 'OK', got %q", string(body))
	}
}

func TestProxy_UnknownHost_Returns502(t *testing.T) {
	proxy := NewReverseProxy(testLogger())

	req := httptest.NewRequest("GET", "http://unknown.local/", nil)
	req.Host = "unknown.local"
	w := httptest.NewRecorder()

	proxy.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("expected 502, got %d", resp.StatusCode)
	}
}

func TestProxy_RoutesToBackend(t *testing.T) {
	// Start a test backend
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "hello from backend")
	}))
	defer backend.Close()

	// Parse backend address
	backendHost := backend.Listener.Addr().String()
	host, portStr, _ := net.SplitHostPort(backendHost)
	port := 0
	fmt.Sscanf(portStr, "%d", &port)
	if host == "" {
		host = "127.0.0.1"
	}

	proxy := NewReverseProxy(testLogger())
	proxy.RegisterRoute("app.test", RouteTarget{
		ContainerID: "container-test-123",
		IP:          net.ParseIP(host),
		Port:        port,
	})

	req := httptest.NewRequest("GET", "http://app.test/", nil)
	req.Host = "app.test"
	w := httptest.NewRecorder()

	proxy.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "hello from backend" {
		t.Errorf("expected 'hello from backend', got %q", string(body))
	}
}

func TestProxy_SetsForwardedHeaders(t *testing.T) {
	var receivedHeaders http.Header

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	backendHost := backend.Listener.Addr().String()
	host, portStr, _ := net.SplitHostPort(backendHost)
	port := 0
	fmt.Sscanf(portStr, "%d", &port)
	if host == "" {
		host = "127.0.0.1"
	}

	proxy := NewReverseProxy(testLogger())
	proxy.RegisterRoute("headers.test", RouteTarget{
		ContainerID: "container-hdr-1234",
		IP:          net.ParseIP(host),
		Port:        port,
	})

	req := httptest.NewRequest("GET", "http://headers.test/path", nil)
	req.Host = "headers.test:8080"
	req.RemoteAddr = "192.168.1.100:12345"
	w := httptest.NewRecorder()

	proxy.ServeHTTP(w, req)

	if receivedHeaders.Get("X-Forwarded-Host") != "headers.test:8080" {
		t.Errorf("X-Forwarded-Host: expected 'headers.test:8080', got %q", receivedHeaders.Get("X-Forwarded-Host"))
	}
	if receivedHeaders.Get("X-Forwarded-Proto") != "http" {
		t.Errorf("X-Forwarded-Proto: expected 'http', got %q", receivedHeaders.Get("X-Forwarded-Proto"))
	}
	if receivedHeaders.Get("X-Real-IP") != "192.168.1.100" {
		t.Errorf("X-Real-IP: expected '192.168.1.100', got %q", receivedHeaders.Get("X-Real-IP"))
	}
}

func TestProxy_HostnameStripsPort(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "reached")
	}))
	defer backend.Close()

	backendHost := backend.Listener.Addr().String()
	host, portStr, _ := net.SplitHostPort(backendHost)
	port := 0
	fmt.Sscanf(portStr, "%d", &port)
	if host == "" {
		host = "127.0.0.1"
	}

	proxy := NewReverseProxy(testLogger())
	proxy.RegisterRoute("strip.test", RouteTarget{
		ContainerID: "container-strip-123",
		IP:          net.ParseIP(host),
		Port:        port,
	})

	// Request with port in Host header — should still match "strip.test"
	req := httptest.NewRequest("GET", "http://strip.test:9999/", nil)
	req.Host = "strip.test:9999"
	w := httptest.NewRecorder()

	proxy.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 (hostname port should be stripped), got %d", resp.StatusCode)
	}
}

func TestProxy_LoadBalancing_MultipleTargets(t *testing.T) {
	// Create two backends that respond differently
	backend1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "backend1")
	}))
	defer backend1.Close()

	backend2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "backend2")
	}))
	defer backend2.Close()

	parseAddr := func(addr string) (net.IP, int) {
		host, portStr, _ := net.SplitHostPort(addr)
		port := 0
		fmt.Sscanf(portStr, "%d", &port)
		if host == "" {
			host = "127.0.0.1"
		}
		return net.ParseIP(host), port
	}

	ip1, port1 := parseAddr(backend1.Listener.Addr().String())
	ip2, port2 := parseAddr(backend2.Listener.Addr().String())

	proxy := NewReverseProxy(testLogger())
	proxy.RegisterRoute("lb.test", RouteTarget{
		ContainerID: "container-lb1-1234",
		IP:          ip1,
		Port:        port1,
	})
	proxy.RegisterRoute("lb.test", RouteTarget{
		ContainerID: "container-lb2-5678",
		IP:          ip2,
		Port:        port2,
	})

	// Make multiple requests and check that both backends are hit
	seen := map[string]bool{}
	for i := 0; i < 20; i++ {
		req := httptest.NewRequest("GET", "http://lb.test/", nil)
		req.Host = "lb.test"
		w := httptest.NewRecorder()

		proxy.ServeHTTP(w, req)

		body, _ := io.ReadAll(w.Result().Body)
		seen[string(body)] = true
	}

	if !seen["backend1"] || !seen["backend2"] {
		t.Errorf("expected both backends to be hit, got: %v", seen)
	}
}

func TestProxy_RegisterRoute_Deduplication(t *testing.T) {
	proxy := NewReverseProxy(testLogger())

	target := RouteTarget{
		ContainerID: "container-dup-1234",
		IP:          net.ParseIP("10.88.0.2"),
		Port:        80,
	}

	proxy.RegisterRoute("dup.test", target)
	proxy.RegisterRoute("dup.test", target) // duplicate

	routes := proxy.GetRoutes()
	if len(routes["dup.test"]) != 1 {
		t.Errorf("expected 1 target (duplicate ignored), got %d", len(routes["dup.test"]))
	}
}

func TestProxy_DeregisterRoute_ByHostname(t *testing.T) {
	proxy := NewReverseProxy(testLogger())

	proxy.RegisterRoute("dereg.test", RouteTarget{ContainerID: "container-a-123456", IP: net.ParseIP("10.88.0.2"), Port: 80})
	proxy.RegisterRoute("dereg.test", RouteTarget{ContainerID: "container-b-789012", IP: net.ParseIP("10.88.0.3"), Port: 80})

	proxy.DeregisterRoute("dereg.test", "container-a-123456")

	routes := proxy.GetRoutes()
	if len(routes["dereg.test"]) != 1 {
		t.Fatalf("expected 1 target, got %d", len(routes["dereg.test"]))
	}
	if routes["dereg.test"][0].ContainerID != "container-b-789012" {
		t.Errorf("wrong container remaining: %s", routes["dereg.test"][0].ContainerID)
	}
}

func TestProxy_DeregisterRoute_AllHostnames(t *testing.T) {
	proxy := NewReverseProxy(testLogger())

	target := RouteTarget{ContainerID: "container-all-123456", IP: net.ParseIP("10.88.0.2"), Port: 80}
	proxy.RegisterRoute("a.test", target)
	proxy.RegisterRoute("b.test", target)

	proxy.DeregisterRoute("", "container-all-123456")

	routes := proxy.GetRoutes()
	if len(routes) != 0 {
		t.Errorf("expected 0 routes after deregister all, got %d", len(routes))
	}
}

func TestProxy_DeregisterRoute_RemovesEmptyHostname(t *testing.T) {
	proxy := NewReverseProxy(testLogger())

	proxy.RegisterRoute("empty.test", RouteTarget{ContainerID: "container-emp-123456", IP: net.ParseIP("10.88.0.2"), Port: 80})
	proxy.DeregisterRoute("empty.test", "container-emp-123456")

	routes := proxy.GetRoutes()
	if _, ok := routes["empty.test"]; ok {
		t.Error("empty hostname entry should be removed from map")
	}
}

func TestProxy_GetRoutes_Isolation(t *testing.T) {
	proxy := NewReverseProxy(testLogger())

	proxy.RegisterRoute("iso.test", RouteTarget{ContainerID: "container-iso-123456", IP: net.ParseIP("10.88.0.2"), Port: 80})

	routes := proxy.GetRoutes()
	routes["iso.test"] = nil

	original := proxy.GetRoutes()
	if len(original["iso.test"]) != 1 {
		t.Error("GetRoutes did not return an isolated copy")
	}
}

func TestProxy_BackendError_Returns502(t *testing.T) {
	proxy := NewReverseProxy(testLogger())

	// Route to a port that's not listening
	proxy.RegisterRoute("down.test", RouteTarget{
		ContainerID: "container-down-123456",
		IP:          net.ParseIP("127.0.0.1"),
		Port:        19999, // nothing listening
	})

	req := httptest.NewRequest("GET", "http://down.test/", nil)
	req.Host = "down.test"
	w := httptest.NewRecorder()

	proxy.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("expected 502 for unreachable backend, got %d", resp.StatusCode)
	}
}

func TestProxy_ConcurrentAccess(t *testing.T) {
	proxy := NewReverseProxy(testLogger())

	var wg sync.WaitGroup
	// Concurrent register/deregister/get
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			target := RouteTarget{
				ContainerID: fmt.Sprintf("container-cc-%06d", i),
				IP:          net.ParseIP("10.88.0.2"),
				Port:        80,
			}
			proxy.RegisterRoute("concurrent.test", target)
			proxy.GetRoutes()
			proxy.DeregisterRoute("concurrent.test", target.ContainerID)
		}(i)
	}
	wg.Wait()
}

func TestProxy_StartStop(t *testing.T) {
	proxy := NewReverseProxy(testLogger())

	// Use port 0 to get a free port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	addr := listener.Addr().String()
	listener.Close()

	// Start proxy in background
	errCh := make(chan error, 1)
	go func() {
		errCh <- proxy.Start(addr)
	}()

	// Wait for it to be ready
	time.Sleep(100 * time.Millisecond)

	// Test health endpoint via real HTTP
	resp, err := http.Get("http://" + addr + "/__vessel/health")
	if err != nil {
		t.Fatalf("failed to GET health: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("health check: expected 200, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Test unknown host via real HTTP
	req, _ := http.NewRequest("GET", "http://"+addr+"/", nil)
	req.Host = "unknown.local"
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to GET: %v", err)
	}
	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("unknown host: expected 502, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Stop
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := proxy.Stop(ctx); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	startErr := <-errCh
	if startErr != nil && startErr != http.ErrServerClosed {
		t.Errorf("unexpected start error: %v", startErr)
	}
}

func TestProxy_NewReverseProxyWithTLS(t *testing.T) {
	cfg := ProxyConfig{
		TLSEnabled: true,
		ACMEEmail:  "admin@example.com",
		CertDir:    t.TempDir(),
	}
	proxy := NewReverseProxyWithTLS(testLogger(), cfg)

	if !proxy.IsTLSEnabled() {
		t.Error("expected TLS to be enabled")
	}
	if !proxy.HasACME() {
		t.Error("expected ACME to be configured")
	}
}

func TestProxy_NewReverseProxyWithTLS_NoACME(t *testing.T) {
	cfg := ProxyConfig{
		TLSEnabled: true,
	}
	proxy := NewReverseProxyWithTLS(testLogger(), cfg)

	if !proxy.IsTLSEnabled() {
		t.Error("expected TLS to be enabled")
	}
	if proxy.HasACME() {
		t.Error("expected ACME to NOT be configured (no email/certdir)")
	}
}

func TestProxy_LoadCustomCert(t *testing.T) {
	certDir := t.TempDir()
	certPath, keyPath := generateSelfSignedCert(t, certDir, "test.local")

	proxy := NewReverseProxy(testLogger())
	if err := proxy.LoadCustomCert("test.local", certPath, keyPath); err != nil {
		t.Fatalf("LoadCustomCert failed: %v", err)
	}

	certs := proxy.GetCustomCerts()
	if len(certs) != 1 || certs[0] != "test.local" {
		t.Errorf("expected [test.local], got %v", certs)
	}
}

func TestProxy_LoadCustomCert_InvalidFiles(t *testing.T) {
	proxy := NewReverseProxy(testLogger())
	err := proxy.LoadCustomCert("test.local", "/nonexistent/cert.pem", "/nonexistent/key.pem")
	if err == nil {
		t.Error("expected error for non-existent cert files")
	}
}

func TestProxy_GetCertificate_CustomCert(t *testing.T) {
	certDir := t.TempDir()
	certPath, keyPath := generateSelfSignedCert(t, certDir, "custom.local")

	proxy := NewReverseProxy(testLogger())
	proxy.LoadCustomCert("custom.local", certPath, keyPath)

	hello := &tls.ClientHelloInfo{ServerName: "custom.local"}
	cert, err := proxy.getCertificate(hello)
	if err != nil {
		t.Fatalf("getCertificate failed: %v", err)
	}
	if cert == nil {
		t.Fatal("expected certificate, got nil")
	}
}

func TestProxy_GetCertificate_NoCert(t *testing.T) {
	proxy := NewReverseProxy(testLogger())
	hello := &tls.ClientHelloInfo{ServerName: "unknown.local"}
	_, err := proxy.getCertificate(hello)
	if err == nil {
		t.Error("expected error for unknown hostname")
	}
}

func TestProxy_HostPolicy(t *testing.T) {
	proxy := NewReverseProxy(testLogger())
	proxy.RegisterRoute("allowed.example.com", RouteTarget{
		ContainerID: "container-hp-123456",
		IP:          net.ParseIP("10.88.0.2"),
		Port:        80,
	})

	if err := proxy.hostPolicy(context.Background(), "allowed.example.com"); err != nil {
		t.Errorf("expected no error for registered host, got: %v", err)
	}

	if err := proxy.hostPolicy(context.Background(), "denied.example.com"); err == nil {
		t.Error("expected error for unregistered host")
	}
}

func TestProxy_ForwardedProto_HTTPS(t *testing.T) {
	var receivedProto string

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedProto = r.Header.Get("X-Forwarded-Proto")
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	backendHost := backend.Listener.Addr().String()
	host, portStr, _ := net.SplitHostPort(backendHost)
	port := 0
	fmt.Sscanf(portStr, "%d", &port)
	if host == "" {
		host = "127.0.0.1"
	}

	proxy := NewReverseProxy(testLogger())
	proxy.RegisterRoute("secure.test", RouteTarget{
		ContainerID: "container-sec-123456",
		IP:          net.ParseIP(host),
		Port:        port,
	})

	// Simulate HTTPS request (with TLS state set)
	req := httptest.NewRequest("GET", "https://secure.test/", nil)
	req.Host = "secure.test"
	req.TLS = &tls.ConnectionState{} // Marks this as TLS
	w := httptest.NewRecorder()

	proxy.ServeHTTP(w, req)

	if receivedProto != "https" {
		t.Errorf("expected X-Forwarded-Proto=https, got %q", receivedProto)
	}
}

func TestProxy_StartTLS_HTTPRedirect(t *testing.T) {
	proxy := NewReverseProxyWithTLS(testLogger(), ProxyConfig{TLSEnabled: true})

	// Get free ports
	httpListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	httpAddr := httpListener.Addr().String()
	httpListener.Close()

	httpsListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	httpsAddr := httpsListener.Addr().String()
	httpsListener.Close()

	if err := proxy.StartTLS(httpAddr, httpsAddr); err != nil {
		t.Fatalf("StartTLS failed: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		proxy.Stop(ctx)
	}()

	// Test that HTTP returns redirect to HTTPS
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // Don't follow redirects
		},
	}

	req, _ := http.NewRequest("GET", "http://"+httpAddr+"/test", nil)
	req.Host = "app.example.com"
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("HTTP request failed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusMovedPermanently {
		t.Errorf("expected 301, got %d", resp.StatusCode)
	}

	location := resp.Header.Get("Location")
	if !strings.HasPrefix(location, "https://") {
		t.Errorf("expected redirect to https://, got %q", location)
	}
}

func TestProxy_StartTLS_HealthCheck(t *testing.T) {
	proxy := NewReverseProxyWithTLS(testLogger(), ProxyConfig{TLSEnabled: true})

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	httpAddr := listener.Addr().String()
	listener.Close()

	if err := proxy.StartTLS(httpAddr, ""); err != nil {
		t.Fatalf("StartTLS failed: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		proxy.Stop(ctx)
	}()

	// Health check should still work on HTTP
	resp, err := http.Get("http://" + httpAddr + "/__vessel/health")
	if err != nil {
		t.Fatalf("health check request failed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestProxy_StopBothServers(t *testing.T) {
	proxy := NewReverseProxyWithTLS(testLogger(), ProxyConfig{TLSEnabled: true})

	httpListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	httpAddr := httpListener.Addr().String()
	httpListener.Close()

	proxy.StartTLS(httpAddr, "")
	time.Sleep(100 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := proxy.Stop(ctx); err != nil {
		t.Errorf("Stop failed: %v", err)
	}
}

// generateSelfSignedCert generates a self-signed certificate for testing.
func generateSelfSignedCert(t *testing.T, dir, hostname string) (certPath, keyPath string) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: hostname,
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{hostname},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("failed to create certificate: %v", err)
	}

	certPath = filepath.Join(dir, "cert.pem")
	keyPath = filepath.Join(dir, "key.pem")

	certFile, err := os.Create(certPath)
	if err != nil {
		t.Fatalf("failed to create cert file: %v", err)
	}
	pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	certFile.Close()

	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("failed to marshal key: %v", err)
	}

	keyFile, err := os.Create(keyPath)
	if err != nil {
		t.Fatalf("failed to create key file: %v", err)
	}
	pem.Encode(keyFile, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	keyFile.Close()

	return certPath, keyPath
}

func TestProxy_Integration_FullLifecycle(t *testing.T) {
	// Start a test backend
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "path=%s", r.URL.Path)
	}))
	defer backend.Close()

	backendHost := backend.Listener.Addr().String()
	host, portStr, _ := net.SplitHostPort(backendHost)
	port := 0
	fmt.Sscanf(portStr, "%d", &port)
	if host == "" {
		host = "127.0.0.1"
	}

	// Start proxy on a free port
	proxy := NewReverseProxy(testLogger())
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	proxyAddr := listener.Addr().String()
	listener.Close()

	go proxy.Start(proxyAddr)
	time.Sleep(100 * time.Millisecond)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		proxy.Stop(ctx)
	}()

	// Register route
	proxy.RegisterRoute("lifecycle.test", RouteTarget{
		ContainerID: "container-lc-123456",
		IP:          net.ParseIP(host),
		Port:        port,
	})

	// Request should be proxied
	req, _ := http.NewRequest("GET", "http://"+proxyAddr+"/hello", nil)
	req.Host = "lifecycle.test"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("proxied request failed: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if !strings.Contains(string(body), "path=/hello") {
		t.Errorf("expected proxied response with path=/hello, got %q", string(body))
	}

	// Deregister route
	proxy.DeregisterRoute("lifecycle.test", "container-lc-123456")

	// Request should now return 502
	req, _ = http.NewRequest("GET", "http://"+proxyAddr+"/hello", nil)
	req.Host = "lifecycle.test"
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post-deregister request failed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("after deregister: expected 502, got %d", resp.StatusCode)
	}
}
