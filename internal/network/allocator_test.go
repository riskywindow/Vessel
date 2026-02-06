package network

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestNewIPAllocator(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "alloc.json")

	a, err := NewIPAllocator(path)
	if err != nil {
		t.Fatalf("NewIPAllocator failed: %v", err)
	}

	if a.gateway.String() != "10.88.0.1" {
		t.Errorf("expected gateway 10.88.0.1, got %s", a.gateway.String())
	}

	if a.nextIP.String() != "10.88.0.2" {
		t.Errorf("expected nextIP 10.88.0.2, got %s", a.nextIP.String())
	}
}

func TestAllocateSequential(t *testing.T) {
	dir := t.TempDir()
	a, err := NewIPAllocator(filepath.Join(dir, "alloc.json"))
	if err != nil {
		t.Fatalf("NewIPAllocator failed: %v", err)
	}

	expected := []string{"10.88.0.2", "10.88.0.3", "10.88.0.4", "10.88.0.5"}
	for i, exp := range expected {
		ip, err := a.Allocate(containerID(i))
		if err != nil {
			t.Fatalf("Allocate %d failed: %v", i, err)
		}
		if ip.String() != exp {
			t.Errorf("allocation %d: expected %s, got %s", i, exp, ip.String())
		}
	}
}

func TestAllocateGatewayNeverAllocated(t *testing.T) {
	dir := t.TempDir()
	a, err := NewIPAllocator(filepath.Join(dir, "alloc.json"))
	if err != nil {
		t.Fatalf("NewIPAllocator failed: %v", err)
	}

	// Allocate many IPs and ensure gateway is never returned
	gatewayStr := "10.88.0.1"
	for i := 0; i < 100; i++ {
		ip, err := a.Allocate(containerID(i))
		if err != nil {
			t.Fatalf("Allocate %d failed: %v", i, err)
		}
		if ip.String() == gatewayStr {
			t.Fatalf("gateway IP %s was allocated to container %d", gatewayStr, i)
		}
	}
}

func TestAllocateIdempotent(t *testing.T) {
	dir := t.TempDir()
	a, err := NewIPAllocator(filepath.Join(dir, "alloc.json"))
	if err != nil {
		t.Fatalf("NewIPAllocator failed: %v", err)
	}

	// First allocation
	ip1, err := a.Allocate("container-aaa-111")
	if err != nil {
		t.Fatalf("first Allocate failed: %v", err)
	}

	// Second allocation for same container should return same IP
	ip2, err := a.Allocate("container-aaa-111")
	if err != nil {
		t.Fatalf("second Allocate failed: %v", err)
	}

	if !ip1.Equal(ip2) {
		t.Errorf("idempotent allocation failed: %s != %s", ip1, ip2)
	}
}

func TestReleaseAndReallocate(t *testing.T) {
	dir := t.TempDir()
	a, err := NewIPAllocator(filepath.Join(dir, "alloc.json"))
	if err != nil {
		t.Fatalf("NewIPAllocator failed: %v", err)
	}

	// Allocate 3 IPs
	ip1, _ := a.Allocate("container-aaa-111")
	a.Allocate("container-bbb-222")
	a.Allocate("container-ccc-333")

	// Release the first one
	if err := a.Release("container-aaa-111"); err != nil {
		t.Fatalf("Release failed: %v", err)
	}

	// Verify it's gone
	_, found := a.GetIP("container-aaa-111")
	if found {
		t.Error("released IP still found")
	}

	// Allocate a new container — should get a new IP (nextIP advances, doesn't reuse immediately)
	ip4, err := a.Allocate("container-ddd-444")
	if err != nil {
		t.Fatalf("Allocate after release failed: %v", err)
	}

	// The released IP should eventually be reusable
	// ip4 will be 10.88.0.5 since nextIP advanced past 0.4
	if ip4.String() == ip1.String() {
		// This is also valid — if wrapping happened
		t.Logf("reused released IP immediately: %s", ip4)
	}
}

func TestGetIP(t *testing.T) {
	dir := t.TempDir()
	a, err := NewIPAllocator(filepath.Join(dir, "alloc.json"))
	if err != nil {
		t.Fatalf("NewIPAllocator failed: %v", err)
	}

	// Not allocated yet
	_, found := a.GetIP("container-aaa-111")
	if found {
		t.Error("GetIP returned true for unallocated container")
	}

	// Allocate
	expected, _ := a.Allocate("container-aaa-111")

	// Now it should be found
	ip, found := a.GetIP("container-aaa-111")
	if !found {
		t.Error("GetIP returned false for allocated container")
	}
	if !ip.Equal(expected) {
		t.Errorf("GetIP returned %s, expected %s", ip, expected)
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "alloc.json")

	// Create allocator and allocate some IPs
	a1, err := NewIPAllocator(path)
	if err != nil {
		t.Fatalf("NewIPAllocator failed: %v", err)
	}

	ip1, _ := a1.Allocate("container-aaa-111")
	ip2, _ := a1.Allocate("container-bbb-222")

	// Verify file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("allocations file not created")
	}

	// Create new allocator from same file
	a2, err := NewIPAllocator(path)
	if err != nil {
		t.Fatalf("NewIPAllocator (reload) failed: %v", err)
	}

	// Verify allocations were restored
	got1, found1 := a2.GetIP("container-aaa-111")
	if !found1 || !got1.Equal(ip1) {
		t.Errorf("persistence failed for container-aaa-111: found=%v, ip=%v (expected %v)", found1, got1, ip1)
	}

	got2, found2 := a2.GetIP("container-bbb-222")
	if !found2 || !got2.Equal(ip2) {
		t.Errorf("persistence failed for container-bbb-222: found=%v, ip=%v (expected %v)", found2, got2, ip2)
	}

	// Allocating a new container should not conflict with existing ones
	ip3, err := a2.Allocate("container-ccc-333")
	if err != nil {
		t.Fatalf("Allocate after reload failed: %v", err)
	}
	if ip3.Equal(ip1) || ip3.Equal(ip2) {
		t.Errorf("new allocation %s conflicts with existing: %s, %s", ip3, ip1, ip2)
	}
}

func TestReleaseNonexistent(t *testing.T) {
	dir := t.TempDir()
	a, err := NewIPAllocator(filepath.Join(dir, "alloc.json"))
	if err != nil {
		t.Fatalf("NewIPAllocator failed: %v", err)
	}

	// Releasing a non-existent container should not error
	if err := a.Release("nonexistent-container"); err != nil {
		t.Errorf("Release of non-existent container should not error: %v", err)
	}
}

func TestAllocateNoDataPath(t *testing.T) {
	// Empty dataPath means no persistence
	a, err := NewIPAllocator("")
	if err != nil {
		t.Fatalf("NewIPAllocator failed: %v", err)
	}

	ip, err := a.Allocate("container-aaa-111")
	if err != nil {
		t.Fatalf("Allocate failed: %v", err)
	}
	if ip.String() != "10.88.0.2" {
		t.Errorf("expected 10.88.0.2, got %s", ip)
	}
}

func TestIncrementIP(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"10.88.0.1", "10.88.0.2"},
		{"10.88.0.254", "10.88.0.255"},
		{"10.88.0.255", "10.88.1.0"},
		{"10.88.255.255", "10.89.0.0"},
		{"0.0.0.0", "0.0.0.1"},
	}

	for _, tt := range tests {
		ip := net.ParseIP(tt.input).To4()
		result := incrementIP(ip)
		if result.String() != tt.expected {
			t.Errorf("incrementIP(%s) = %s, expected %s", tt.input, result, tt.expected)
		}
		// Verify original is not modified
		if ip.String() != tt.input {
			t.Errorf("incrementIP modified original: %s → %s", tt.input, ip)
		}
	}
}

func TestGatewayAndSubnet(t *testing.T) {
	dir := t.TempDir()
	a, err := NewIPAllocator(filepath.Join(dir, "alloc.json"))
	if err != nil {
		t.Fatalf("NewIPAllocator failed: %v", err)
	}

	gw := a.Gateway()
	if gw.String() != "10.88.0.1" {
		t.Errorf("Gateway() = %s, expected 10.88.0.1", gw)
	}

	sn := a.Subnet()
	if sn.String() != "10.88.0.0/16" {
		t.Errorf("Subnet() = %s, expected 10.88.0.0/16", sn)
	}
}

// containerID generates a container ID for testing (must be ≥12 chars).
func containerID(n int) string {
	return fmt.Sprintf("container-%04d", n)
}

