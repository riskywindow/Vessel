package network

import (
	"log/slog"
	"net"
	"os"
	"testing"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func testManager(t *testing.T) *LinuxNetworkManager {
	t.Helper()
	dir := t.TempDir()

	allocator, err := NewIPAllocator(dir + "/alloc.json")
	if err != nil {
		t.Fatalf("NewIPAllocator failed: %v", err)
	}

	proxy := NewReverseProxy(testLogger())

	return &LinuxNetworkManager{
		bridge:    NewBridgeNetwork("vessel-test", "10.88.0.1/16", testLogger()),
		allocator: allocator,
		proxy:     proxy,
		logger:    testLogger(),
		proxyAddr: ":0",
	}
}

func TestRegisterRoute(t *testing.T) {
	m := testManager(t)

	target := RouteTarget{
		ContainerID: "container-aaa-111",
		IP:          net.ParseIP("10.88.0.2"),
		Port:        8080,
		Weight:      1,
	}

	if err := m.RegisterRoute("example.com", target); err != nil {
		t.Fatalf("RegisterRoute failed: %v", err)
	}

	routes := m.GetRoutes()
	targets, ok := routes["example.com"]
	if !ok {
		t.Fatal("route not found for example.com")
	}
	if len(targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(targets))
	}
	if targets[0].ContainerID != "container-aaa-111" {
		t.Errorf("expected container-aaa-111, got %s", targets[0].ContainerID)
	}
}

func TestRegisterMultipleTargets(t *testing.T) {
	m := testManager(t)

	t1 := RouteTarget{ContainerID: "container-aaa-111", IP: net.ParseIP("10.88.0.2"), Port: 8080, Weight: 1}
	t2 := RouteTarget{ContainerID: "container-bbb-222", IP: net.ParseIP("10.88.0.3"), Port: 8080, Weight: 1}

	m.RegisterRoute("example.com", t1)
	m.RegisterRoute("example.com", t2)

	routes := m.GetRoutes()
	if len(routes["example.com"]) != 2 {
		t.Errorf("expected 2 targets, got %d", len(routes["example.com"]))
	}
}

func TestDeregisterRoute_ByHostname(t *testing.T) {
	m := testManager(t)

	t1 := RouteTarget{ContainerID: "container-aaa-111", IP: net.ParseIP("10.88.0.2"), Port: 8080, Weight: 1}
	t2 := RouteTarget{ContainerID: "container-bbb-222", IP: net.ParseIP("10.88.0.3"), Port: 8080, Weight: 1}

	m.RegisterRoute("example.com", t1)
	m.RegisterRoute("example.com", t2)

	// Remove just one container
	if err := m.DeregisterRoute("example.com", "container-aaa-111"); err != nil {
		t.Fatalf("DeregisterRoute failed: %v", err)
	}

	routes := m.GetRoutes()
	targets := routes["example.com"]
	if len(targets) != 1 {
		t.Fatalf("expected 1 target after deregister, got %d", len(targets))
	}
	if targets[0].ContainerID != "container-bbb-222" {
		t.Errorf("wrong container remaining: %s", targets[0].ContainerID)
	}
}

func TestDeregisterRoute_RemovesEmptyHostname(t *testing.T) {
	m := testManager(t)

	t1 := RouteTarget{ContainerID: "container-aaa-111", IP: net.ParseIP("10.88.0.2"), Port: 8080, Weight: 1}
	m.RegisterRoute("example.com", t1)

	m.DeregisterRoute("example.com", "container-aaa-111")

	routes := m.GetRoutes()
	if _, ok := routes["example.com"]; ok {
		t.Error("empty hostname entry should be removed")
	}
}

func TestDeregisterRoute_AllHostnames(t *testing.T) {
	m := testManager(t)

	t1 := RouteTarget{ContainerID: "container-aaa-111", IP: net.ParseIP("10.88.0.2"), Port: 8080, Weight: 1}

	m.RegisterRoute("a.com", t1)
	m.RegisterRoute("b.com", t1)

	// Empty hostname = remove from all routes
	m.DeregisterRoute("", "container-aaa-111")

	routes := m.GetRoutes()
	if len(routes) != 0 {
		t.Errorf("expected 0 routes after deregister all, got %d", len(routes))
	}
}

func TestGetRoutes_Isolation(t *testing.T) {
	m := testManager(t)

	t1 := RouteTarget{ContainerID: "container-aaa-111", IP: net.ParseIP("10.88.0.2"), Port: 8080, Weight: 1}
	m.RegisterRoute("example.com", t1)

	// GetRoutes should return a copy
	routes := m.GetRoutes()
	routes["example.com"] = nil // Modify the copy

	// Original should be unaffected
	original := m.GetRoutes()
	if len(original["example.com"]) != 1 {
		t.Error("GetRoutes did not return an isolated copy")
	}
}

func TestRegisterRoute_DuplicateIgnored(t *testing.T) {
	m := testManager(t)

	t1 := RouteTarget{ContainerID: "container-aaa-111", IP: net.ParseIP("10.88.0.2"), Port: 8080, Weight: 1}

	m.RegisterRoute("example.com", t1)
	m.RegisterRoute("example.com", t1) // Duplicate

	routes := m.GetRoutes()
	if len(routes["example.com"]) != 1 {
		t.Errorf("expected 1 target (duplicate should be ignored), got %d", len(routes["example.com"]))
	}
}
