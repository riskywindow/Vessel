package network

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestBridgeExists_NotPresent(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	b := NewBridgeNetwork("vessel-test-nonexistent", "10.99.0.1/16", logger)

	if b.bridgeExists() {
		t.Error("bridgeExists returned true for non-existent bridge")
	}
}

func TestBridgeExists_MockPresent(t *testing.T) {
	// Create a fake /sys/class/net entry to test detection
	// We use a temp dir and override the check path indirectly
	// Since bridgeExists uses /sys/class/net directly, we test with a known interface
	dir := filepath.Join("/sys/class/net", "lo")
	if _, err := os.Stat(dir); err != nil {
		t.Skip("no /sys/class/net/lo, skipping")
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	// "lo" always exists as the loopback interface
	b := &BridgeNetwork{
		name:   "lo",
		subnet: "127.0.0.1/8",
		logger: logger,
	}

	if !b.bridgeExists() {
		t.Error("bridgeExists returned false for loopback interface")
	}
}

func TestNewBridgeNetwork(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	b := NewBridgeNetwork("test0", "10.99.0.1/16", logger)

	if b.name != "test0" {
		t.Errorf("name = %s, expected test0", b.name)
	}
	if b.subnet != "10.99.0.1/16" {
		t.Errorf("subnet = %s, expected 10.99.0.1/16", b.subnet)
	}
}
