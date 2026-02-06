package network

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"

	vessel "github.com/vessel/vessel/internal"
)

// IPAllocator manages IP address allocation for containers within a subnet.
type IPAllocator struct {
	mu        sync.Mutex
	subnet    *net.IPNet        // 10.88.0.0/16
	gateway   net.IP            // 10.88.0.1
	allocated map[string]net.IP // containerID → IP
	used      map[string]bool   // IP string → in use
	nextIP    net.IP            // Next IP to try
	dataPath  string            // Path to persist allocations
}

// allocatorState is the on-disk format for IP allocations.
type allocatorState struct {
	Allocated map[string]string `json:"allocated"` // containerID → IP string
	NextIP    string            `json:"next_ip"`
}

// DefaultSubnet is the default subnet for container networking.
const DefaultSubnet = "10.88.0.0/16"

// DefaultGateway is the gateway IP for the container bridge.
const DefaultGateway = "10.88.0.1"

// NewIPAllocator creates a new IP allocator for the given subnet.
// It loads existing allocations from dataPath if the file exists.
func NewIPAllocator(dataPath string) (*IPAllocator, error) {
	_, subnet, err := net.ParseCIDR(DefaultSubnet)
	if err != nil {
		return nil, fmt.Errorf("failed to parse subnet: %w", err)
	}

	gateway := net.ParseIP(DefaultGateway).To4()
	startIP := net.ParseIP("10.88.0.2").To4()

	a := &IPAllocator{
		subnet:    subnet,
		gateway:   gateway,
		allocated: make(map[string]net.IP),
		used:      make(map[string]bool),
		nextIP:    startIP,
		dataPath:  dataPath,
	}

	// Reserve gateway
	a.used[gateway.String()] = true

	// Load existing allocations from disk
	if err := a.load(); err != nil {
		// Not fatal — start fresh if file is missing or corrupt
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to load allocations: %w", err)
		}
	}

	return a, nil
}

// Allocate assigns the next available IP to the given container.
func (a *IPAllocator) Allocate(containerID string) (net.IP, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Check if already allocated
	if ip, ok := a.allocated[containerID]; ok {
		return ip, nil
	}

	// Find next available IP
	ip, err := a.findNextAvailable()
	if err != nil {
		return nil, err
	}

	// Record allocation
	result := make(net.IP, len(ip))
	copy(result, ip)
	a.allocated[containerID] = result
	a.used[result.String()] = true

	// Advance nextIP
	a.nextIP = incrementIP(ip)

	// Persist to disk
	if err := a.persist(); err != nil {
		// Allocation succeeded in memory, log the persist error but don't fail
		return result, nil
	}

	return result, nil
}

// Release frees the IP allocated to the given container.
func (a *IPAllocator) Release(containerID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	ip, ok := a.allocated[containerID]
	if !ok {
		return nil // Nothing to release
	}

	delete(a.allocated, containerID)
	delete(a.used, ip.String())

	return a.persist()
}

// GetIP returns the IP allocated to the given container, if any.
func (a *IPAllocator) GetIP(containerID string) (net.IP, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()

	ip, ok := a.allocated[containerID]
	return ip, ok
}

// Gateway returns the gateway IP for the subnet.
func (a *IPAllocator) Gateway() net.IP {
	return a.gateway
}

// Subnet returns the subnet CIDR.
func (a *IPAllocator) Subnet() *net.IPNet {
	return a.subnet
}

// findNextAvailable finds the next available IP in the subnet.
// Must be called with mu held.
func (a *IPAllocator) findNextAvailable() (net.IP, error) {
	// Start from nextIP and scan forward
	candidate := make(net.IP, len(a.nextIP))
	copy(candidate, a.nextIP)

	// We'll scan the entire subnet once
	startIP := make(net.IP, len(candidate))
	copy(startIP, candidate)
	wrapped := false

	for {
		// Check if we're still in the subnet
		if !a.subnet.Contains(candidate) {
			// Wrap around to 10.88.0.2
			candidate = net.ParseIP("10.88.0.2").To4()
			if wrapped {
				return nil, fmt.Errorf("%w: subnet %s", vessel.ErrIPExhausted, a.subnet.String())
			}
			wrapped = true
		}

		// Check if wrapped back to start
		if wrapped && candidate.Equal(startIP) {
			return nil, fmt.Errorf("%w: subnet %s", vessel.ErrIPExhausted, a.subnet.String())
		}

		// Skip if in use
		if !a.used[candidate.String()] {
			return candidate, nil
		}

		candidate = incrementIP(candidate)
	}
}

// persist writes the current allocation state to disk.
func (a *IPAllocator) persist() error {
	if a.dataPath == "" {
		return nil
	}

	state := allocatorState{
		Allocated: make(map[string]string, len(a.allocated)),
		NextIP:    a.nextIP.String(),
	}
	for id, ip := range a.allocated {
		state.Allocated[id] = ip.String()
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal allocations: %w", err)
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(a.dataPath), 0755); err != nil {
		return fmt.Errorf("failed to create allocations dir: %w", err)
	}

	if err := os.WriteFile(a.dataPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write allocations: %w", err)
	}

	return nil
}

// load reads the allocation state from disk.
func (a *IPAllocator) load() error {
	if a.dataPath == "" {
		return nil
	}

	data, err := os.ReadFile(a.dataPath)
	if err != nil {
		return err
	}

	var state allocatorState
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("failed to unmarshal allocations: %w", err)
	}

	// Restore allocations
	for id, ipStr := range state.Allocated {
		ip := net.ParseIP(ipStr).To4()
		if ip == nil {
			continue
		}
		a.allocated[id] = ip
		a.used[ip.String()] = true
	}

	// Restore nextIP
	if state.NextIP != "" {
		if ip := net.ParseIP(state.NextIP).To4(); ip != nil {
			a.nextIP = ip
		}
	}

	return nil
}

// incrementIP returns the next IP address after the given one.
func incrementIP(ip net.IP) net.IP {
	result := make(net.IP, len(ip))
	copy(result, ip)
	for i := len(result) - 1; i >= 0; i-- {
		result[i]++
		if result[i] != 0 {
			break
		}
	}
	return result
}
