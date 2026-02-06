//go:build integration

package network

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"testing"
)

func integrationLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

func requireRoot(t *testing.T) {
	t.Helper()
	if os.Getuid() != 0 {
		t.Skip("integration test requires root")
	}
}

func TestIntegration_BridgeSetupTeardown(t *testing.T) {
	requireRoot(t)

	logger := integrationLogger()
	bridge := NewBridgeNetwork("vessel-test0", "10.99.0.1/16", logger)

	// Clean up in case a previous test left it
	bridge.Teardown()

	// Setup
	if err := bridge.Setup(); err != nil {
		t.Fatalf("bridge Setup failed: %v", err)
	}
	defer bridge.Teardown()

	// Verify bridge exists
	if !bridge.bridgeExists() {
		t.Fatal("bridge does not exist after setup")
	}

	// Verify IP is assigned
	output, err := exec.Command("ip", "addr", "show", "vessel-test0").CombinedOutput()
	if err != nil {
		t.Fatalf("failed to show bridge addr: %v", err)
	}
	if !contains(string(output), "10.99.0.1") {
		t.Errorf("bridge does not have expected IP, output: %s", output)
	}

	// Verify bridge is UP
	if !contains(string(output), "UP") {
		t.Errorf("bridge is not UP, output: %s", output)
	}

	// Teardown
	if err := bridge.Teardown(); err != nil {
		t.Fatalf("bridge Teardown failed: %v", err)
	}

	if bridge.bridgeExists() {
		t.Error("bridge still exists after teardown")
	}
}

func TestIntegration_BridgeIdempotent(t *testing.T) {
	requireRoot(t)

	logger := integrationLogger()
	bridge := NewBridgeNetwork("vessel-test1", "10.98.0.1/16", logger)

	bridge.Teardown() // Clean up

	if err := bridge.Setup(); err != nil {
		t.Fatalf("first Setup failed: %v", err)
	}
	defer bridge.Teardown()

	// Second setup should succeed (idempotent)
	if err := bridge.Setup(); err != nil {
		t.Fatalf("second Setup failed: %v", err)
	}
}

func TestIntegration_ManagerStartStop(t *testing.T) {
	requireRoot(t)

	logger := integrationLogger()
	dir := t.TempDir()

	allocator, err := NewIPAllocator(dir + "/alloc.json")
	if err != nil {
		t.Fatalf("NewIPAllocator failed: %v", err)
	}

	bridge := NewBridgeNetwork("vessel-test2", "10.97.0.1/16", logger)
	bridge.Teardown() // Clean up

	m := &LinuxNetworkManager{
		bridge:    bridge,
		allocator: allocator,
		routes:    make(map[string][]RouteTarget),
		logger:    logger,
	}

	ctx := context.Background()
	if err := m.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer bridge.Teardown()

	if !bridge.bridgeExists() {
		t.Error("bridge does not exist after Start")
	}

	if err := m.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	// Bridge should still exist after Stop (intentional)
	if !bridge.bridgeExists() {
		t.Error("bridge should persist after Stop")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
