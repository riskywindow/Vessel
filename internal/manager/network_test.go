package manager

import (
	"context"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/vessel/vessel/internal/config"
	"github.com/vessel/vessel/internal/network"
	"github.com/vessel/vessel/internal/runtime"
	"github.com/vessel/vessel/internal/store"
	"github.com/vessel/vessel/testutil"
)

// routeCall records a RegisterRoute or DeregisterRoute call.
type routeCall struct {
	Hostname    string
	ContainerID string
	Target      network.RouteTarget
}

// mockNetworkManager implements network.NetworkManager for testing.
type mockNetworkManager struct {
	setupFn    func(ctx context.Context, containerID string, pid int) (*network.ContainerNetwork, error)
	teardownFn func(ctx context.Context, containerID string) error

	mu               sync.Mutex
	setupCalls       []string // container IDs
	teardownCalls    []string // container IDs
	registerCalls    []routeCall
	deregisterCalls  []routeCall
	allocatedIPs     map[string]net.IP
	ipCounter        int
}

func newMockNetworkManager() *mockNetworkManager {
	return &mockNetworkManager{
		allocatedIPs: make(map[string]net.IP),
	}
}

func (m *mockNetworkManager) SetupContainerNetwork(ctx context.Context, containerID string, pid int) (*network.ContainerNetwork, error) {
	if m.setupFn != nil {
		return m.setupFn(ctx, containerID, pid)
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	m.setupCalls = append(m.setupCalls, containerID)
	m.ipCounter++
	ip := net.IPv4(10, 88, 0, byte(m.ipCounter+1))
	m.allocatedIPs[containerID] = ip

	return &network.ContainerNetwork{
		ContainerID:   containerID,
		IP:            ip,
		Gateway:       net.IPv4(10, 88, 0, 1),
		Bridge:        "vessel0",
		VethHost:      "veth-" + containerID[:8],
		VethContainer: "eth0",
	}, nil
}

func (m *mockNetworkManager) TeardownContainerNetwork(ctx context.Context, containerID string) error {
	if m.teardownFn != nil {
		return m.teardownFn(ctx, containerID)
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	m.teardownCalls = append(m.teardownCalls, containerID)
	delete(m.allocatedIPs, containerID)
	return nil
}

func (m *mockNetworkManager) RegisterRoute(hostname string, target network.RouteTarget) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.registerCalls = append(m.registerCalls, routeCall{Hostname: hostname, Target: target})
	return nil
}

func (m *mockNetworkManager) DeregisterRoute(hostname string, containerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deregisterCalls = append(m.deregisterCalls, routeCall{Hostname: hostname, ContainerID: containerID})
	return nil
}

func (m *mockNetworkManager) Start(ctx context.Context) error {
	return nil
}

func (m *mockNetworkManager) Stop() error {
	return nil
}

// newTestManagerWithNetwork creates an AppManager with mock runtime and mock network for testing.
func newTestManagerWithNetwork(t *testing.T, rt runtime.Runtime, net network.NetworkManager) (*AppManager, store.Store) {
	t.Helper()
	st := testutil.TempStore(t)
	logger := testLogger()
	mgr := NewAppManager(rt, st, net, logger)
	return mgr, st
}

// --- Deploy with Networking Tests ---

func TestDeploy_SetsUpNetworking(t *testing.T) {
	rt := &mockRuntime{}
	netMgr := newMockNetworkManager()
	mgr, st := newTestManagerWithNetwork(t, rt, netMgr)

	appCfg := newTestAppConfig("net-app", "alpine:latest")
	deploy, err := mgr.DeployAppFromConfig(context.Background(), appCfg)
	if err != nil {
		t.Fatalf("deploy failed: %v", err)
	}
	if deploy.Status != store.DeployStatusActive {
		t.Errorf("expected active status, got %s", deploy.Status)
	}

	// Verify network setup was called
	netMgr.mu.Lock()
	setupCount := len(netMgr.setupCalls)
	netMgr.mu.Unlock()

	if setupCount != 1 {
		t.Errorf("expected 1 SetupContainerNetwork call, got %d", setupCount)
	}

	// Verify container has an IP in the store
	containers, err := st.ListContainersByApp("net-app")
	if err != nil {
		t.Fatalf("failed to list containers: %v", err)
	}
	if len(containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(containers))
	}
	if containers[0].IP == "" {
		t.Error("expected container to have an IP assigned")
	}
	if containers[0].IP != "10.88.0.2" {
		t.Errorf("expected IP 10.88.0.2, got %s", containers[0].IP)
	}
}

func TestDeploy_MultipleInstances_EachGetsIP(t *testing.T) {
	rt := &mockRuntime{}
	netMgr := newMockNetworkManager()
	mgr, st := newTestManagerWithNetwork(t, rt, netMgr)

	appCfg := newTestAppConfig("multi-app", "alpine:latest")
	appCfg.Instances = 3

	deploy, err := mgr.DeployAppFromConfig(context.Background(), appCfg)
	if err != nil {
		t.Fatalf("deploy failed: %v", err)
	}
	if deploy.Status != store.DeployStatusActive {
		t.Errorf("expected active status, got %s", deploy.Status)
	}

	// Verify 3 network setup calls
	netMgr.mu.Lock()
	setupCount := len(netMgr.setupCalls)
	netMgr.mu.Unlock()

	if setupCount != 3 {
		t.Errorf("expected 3 SetupContainerNetwork calls, got %d", setupCount)
	}

	// Verify each container has a unique IP
	containers, err := st.ListContainersByApp("multi-app")
	if err != nil {
		t.Fatalf("failed to list containers: %v", err)
	}
	if len(containers) != 3 {
		t.Fatalf("expected 3 containers, got %d", len(containers))
	}

	ips := make(map[string]bool)
	for _, c := range containers {
		if c.IP == "" {
			t.Error("expected container to have an IP")
		}
		if ips[c.IP] {
			t.Errorf("duplicate IP %s assigned to containers", c.IP)
		}
		ips[c.IP] = true
	}
}

func TestDeploy_NetworkFailure_BestEffort(t *testing.T) {
	rt := &mockRuntime{}
	netMgr := newMockNetworkManager()
	netMgr.setupFn = func(ctx context.Context, containerID string, pid int) (*network.ContainerNetwork, error) {
		return nil, fmt.Errorf("network setup failed: bridge not available")
	}
	mgr, st := newTestManagerWithNetwork(t, rt, netMgr)

	appCfg := newTestAppConfig("fail-net-app", "alpine:latest")
	deploy, err := mgr.DeployAppFromConfig(context.Background(), appCfg)
	if err != nil {
		t.Fatalf("deploy should succeed even when network fails, got: %v", err)
	}
	if deploy.Status != store.DeployStatusActive {
		t.Errorf("expected active status, got %s", deploy.Status)
	}

	// Container should exist but without IP
	containers, err := st.ListContainersByApp("fail-net-app")
	if err != nil {
		t.Fatalf("failed to list containers: %v", err)
	}
	if len(containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(containers))
	}
	if containers[0].IP != "" {
		t.Errorf("expected empty IP when network setup fails, got %s", containers[0].IP)
	}
}

func TestDeploy_NoNetworkManager_Succeeds(t *testing.T) {
	rt := &mockRuntime{}
	// nil network manager
	mgr, st := newTestManagerWithNetwork(t, rt, nil)

	appCfg := newTestAppConfig("no-net-app", "alpine:latest")
	deploy, err := mgr.DeployAppFromConfig(context.Background(), appCfg)
	if err != nil {
		t.Fatalf("deploy failed: %v", err)
	}
	if deploy.Status != store.DeployStatusActive {
		t.Errorf("expected active status, got %s", deploy.Status)
	}

	containers, err := st.ListContainersByApp("no-net-app")
	if err != nil {
		t.Fatalf("failed to list containers: %v", err)
	}
	if len(containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(containers))
	}
	if containers[0].IP != "" {
		t.Errorf("expected empty IP with nil network manager, got %s", containers[0].IP)
	}
}

func TestRollingDeploy_TearsDownOldNetwork(t *testing.T) {
	rt := &mockRuntime{}
	netMgr := newMockNetworkManager()
	mgr, _ := newTestManagerWithNetwork(t, rt, netMgr)

	// First deploy
	appCfg := newTestAppConfig("rolling-net", "alpine:v1")
	appCfg.Deploy.GracePeriod = config.Duration{Duration: 10 * time.Millisecond}
	_, err := mgr.DeployAppFromConfig(context.Background(), appCfg)
	if err != nil {
		t.Fatalf("first deploy failed: %v", err)
	}

	// Rolling update
	appCfg2 := newTestAppConfig("rolling-net", "alpine:v2")
	appCfg2.Deploy.GracePeriod = config.Duration{Duration: 10 * time.Millisecond}
	_, err = mgr.DeployAppFromConfig(context.Background(), appCfg2)
	if err != nil {
		t.Fatalf("rolling deploy failed: %v", err)
	}

	// Verify teardown was called for old container
	netMgr.mu.Lock()
	teardownCount := len(netMgr.teardownCalls)
	setupCount := len(netMgr.setupCalls)
	netMgr.mu.Unlock()

	if teardownCount < 1 {
		t.Errorf("expected at least 1 TeardownContainerNetwork call, got %d", teardownCount)
	}
	// Should have 2 setups (v1 + v2) and 1 teardown (v1)
	if setupCount != 2 {
		t.Errorf("expected 2 SetupContainerNetwork calls, got %d", setupCount)
	}
}

func TestBlueGreenDeploy_TearsDownOldNetwork(t *testing.T) {
	rt := &mockRuntime{}
	netMgr := newMockNetworkManager()
	mgr, _ := newTestManagerWithNetwork(t, rt, netMgr)

	// First deploy (2 instances)
	appCfg := newTestAppConfig("bg-net", "alpine:v1")
	appCfg.Instances = 2
	appCfg.Deploy.GracePeriod = config.Duration{Duration: 10 * time.Millisecond}
	_, err := mgr.DeployAppFromConfig(context.Background(), appCfg)
	if err != nil {
		t.Fatalf("first deploy failed: %v", err)
	}

	// Blue-green update
	appCfg2 := newTestAppConfig("bg-net", "alpine:v2")
	appCfg2.Instances = 2
	appCfg2.Deploy.Strategy = "blue-green"
	appCfg2.Deploy.GracePeriod = config.Duration{Duration: 10 * time.Millisecond}
	_, err = mgr.DeployAppFromConfig(context.Background(), appCfg2)
	if err != nil {
		t.Fatalf("blue-green deploy failed: %v", err)
	}

	netMgr.mu.Lock()
	teardownCount := len(netMgr.teardownCalls)
	setupCount := len(netMgr.setupCalls)
	netMgr.mu.Unlock()

	// Should have 4 setups (2 v1 + 2 v2) and 2 teardowns (v1 containers)
	if setupCount != 4 {
		t.Errorf("expected 4 SetupContainerNetwork calls, got %d", setupCount)
	}
	if teardownCount != 2 {
		t.Errorf("expected 2 TeardownContainerNetwork calls for old containers, got %d", teardownCount)
	}
}

func TestStopAndRemoveContainer_TearsDownNetwork(t *testing.T) {
	rt := &mockRuntime{}
	netMgr := newMockNetworkManager()
	st := testutil.TempStore(t)
	logger := testLogger()
	mgr := NewAppManager(rt, st, netMgr, logger)

	containerID := "test-container-123456789012"
	c := &store.Container{
		ID:    containerID,
		AppID: "test-app",
		Image: "alpine:latest",
		State: store.ContainerStateRunning,
		IP:    "10.88.0.5",
	}
	st.CreateContainer(c)

	mgr.stopAndRemoveContainer(context.Background(), c)

	netMgr.mu.Lock()
	teardownCount := len(netMgr.teardownCalls)
	netMgr.mu.Unlock()

	if teardownCount != 1 {
		t.Errorf("expected 1 TeardownContainerNetwork call, got %d", teardownCount)
	}
}

func TestDeploy_ContainerPID_StoredOnNetwork(t *testing.T) {
	expectedPID := 42
	rt := &mockRuntime{
		getContainerPIDFn: func(containerID string) (int, error) {
			return expectedPID, nil
		},
	}
	netMgr := newMockNetworkManager()
	mgr, st := newTestManagerWithNetwork(t, rt, netMgr)

	appCfg := newTestAppConfig("pid-app", "alpine:latest")
	_, err := mgr.DeployAppFromConfig(context.Background(), appCfg)
	if err != nil {
		t.Fatalf("deploy failed: %v", err)
	}

	containers, err := st.ListContainersByApp("pid-app")
	if err != nil {
		t.Fatalf("failed to list containers: %v", err)
	}
	if len(containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(containers))
	}
	if containers[0].PID != expectedPID {
		t.Errorf("expected PID %d, got %d", expectedPID, containers[0].PID)
	}
}

func TestDeploy_GetPIDFails_NoNetworkSetup(t *testing.T) {
	rt := &mockRuntime{
		getContainerPIDFn: func(containerID string) (int, error) {
			return 0, fmt.Errorf("container not running")
		},
	}
	netMgr := newMockNetworkManager()
	mgr, st := newTestManagerWithNetwork(t, rt, netMgr)

	appCfg := newTestAppConfig("pid-fail-app", "alpine:latest")
	deploy, err := mgr.DeployAppFromConfig(context.Background(), appCfg)
	if err != nil {
		t.Fatalf("deploy should succeed, got: %v", err)
	}
	if deploy.Status != store.DeployStatusActive {
		t.Errorf("expected active status, got %s", deploy.Status)
	}

	// Verify no network setup was called (PID retrieval failed)
	netMgr.mu.Lock()
	setupCount := len(netMgr.setupCalls)
	netMgr.mu.Unlock()

	if setupCount != 0 {
		t.Errorf("expected 0 SetupContainerNetwork calls when PID fails, got %d", setupCount)
	}

	// Container should exist but without IP
	containers, err := st.ListContainersByApp("pid-fail-app")
	if err != nil {
		t.Fatalf("failed to list containers: %v", err)
	}
	if len(containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(containers))
	}
	if containers[0].IP != "" {
		t.Errorf("expected empty IP when PID fails, got %s", containers[0].IP)
	}
}

// --- Store Tests ---

func TestListAllContainers(t *testing.T) {
	st := testutil.TempStore(t)
	mgr := &AppManager{
		store: st,
	}

	// Create containers for different apps
	for i, appID := range []string{"app-a", "app-b", "app-c"} {
		c := &store.Container{
			ID:    fmt.Sprintf("container-%012d", i),
			AppID: appID,
			Image: "alpine:latest",
			State: store.ContainerStateRunning,
			IP:    fmt.Sprintf("10.88.0.%d", i+2),
		}
		if err := st.CreateContainer(c); err != nil {
			t.Fatalf("failed to create container: %v", err)
		}
	}

	containers, err := mgr.ListAllContainers(context.Background())
	if err != nil {
		t.Fatalf("ListAllContainers failed: %v", err)
	}
	if len(containers) != 3 {
		t.Errorf("expected 3 containers, got %d", len(containers))
	}
}

func TestListContainers_StoreMethod(t *testing.T) {
	st := testutil.TempStore(t)

	// Create containers
	for i := 0; i < 5; i++ {
		c := &store.Container{
			ID:    fmt.Sprintf("container-%012d", i),
			AppID: "test-app",
			Image: "alpine:latest",
			State: store.ContainerStateRunning,
			IP:    fmt.Sprintf("10.88.0.%d", i+2),
		}
		if err := st.CreateContainer(c); err != nil {
			t.Fatalf("failed to create container: %v", err)
		}
	}

	containers, err := st.ListContainers()
	if err != nil {
		t.Fatalf("ListContainers failed: %v", err)
	}
	if len(containers) != 5 {
		t.Errorf("expected 5 containers, got %d", len(containers))
	}

	// Verify IPs are preserved
	for _, c := range containers {
		if c.IP == "" {
			t.Errorf("expected container %s to have IP, got empty", c.ID)
		}
	}
}

func TestListContainers_Empty(t *testing.T) {
	st := testutil.TempStore(t)

	containers, err := st.ListContainers()
	if err != nil {
		t.Fatalf("ListContainers failed: %v", err)
	}
	if len(containers) != 0 {
		t.Errorf("expected 0 containers, got %d", len(containers))
	}
}

// --- Route Registration Tests ---

func TestDeploy_RegistersRoutes_ForDomains(t *testing.T) {
	rt := &mockRuntime{}
	netMgr := newMockNetworkManager()
	mgr, _ := newTestManagerWithNetwork(t, rt, netMgr)

	appCfg := newTestAppConfig("route-app", "alpine:latest")
	appCfg.Domains = []string{"app.example.com", "www.example.com"}
	appCfg.Ports = []config.PortConfig{{Container: 8080}}

	_, err := mgr.DeployAppFromConfig(context.Background(), appCfg)
	if err != nil {
		t.Fatalf("deploy failed: %v", err)
	}

	netMgr.mu.Lock()
	registerCount := len(netMgr.registerCalls)
	calls := append([]routeCall{}, netMgr.registerCalls...)
	netMgr.mu.Unlock()

	// 1 container × 2 domains = 2 route registrations
	if registerCount != 2 {
		t.Fatalf("expected 2 RegisterRoute calls, got %d", registerCount)
	}

	hostnames := map[string]bool{}
	for _, c := range calls {
		hostnames[c.Hostname] = true
		if c.Target.Port != 8080 {
			t.Errorf("expected port 8080, got %d", c.Target.Port)
		}
	}
	if !hostnames["app.example.com"] || !hostnames["www.example.com"] {
		t.Errorf("expected both domains to be registered, got: %v", hostnames)
	}
}

func TestDeploy_RegistersRoutes_DefaultPort(t *testing.T) {
	rt := &mockRuntime{}
	netMgr := newMockNetworkManager()
	mgr, _ := newTestManagerWithNetwork(t, rt, netMgr)

	appCfg := newTestAppConfig("default-port-app", "alpine:latest")
	appCfg.Domains = []string{"test.local"}
	// No ports configured — should default to 80

	_, err := mgr.DeployAppFromConfig(context.Background(), appCfg)
	if err != nil {
		t.Fatalf("deploy failed: %v", err)
	}

	netMgr.mu.Lock()
	calls := append([]routeCall{}, netMgr.registerCalls...)
	netMgr.mu.Unlock()

	if len(calls) != 1 {
		t.Fatalf("expected 1 RegisterRoute call, got %d", len(calls))
	}
	if calls[0].Target.Port != 80 {
		t.Errorf("expected default port 80, got %d", calls[0].Target.Port)
	}
}

func TestDeploy_NoDomains_NoRoutes(t *testing.T) {
	rt := &mockRuntime{}
	netMgr := newMockNetworkManager()
	mgr, _ := newTestManagerWithNetwork(t, rt, netMgr)

	appCfg := newTestAppConfig("no-domain-app", "alpine:latest")
	// No domains configured

	_, err := mgr.DeployAppFromConfig(context.Background(), appCfg)
	if err != nil {
		t.Fatalf("deploy failed: %v", err)
	}

	netMgr.mu.Lock()
	registerCount := len(netMgr.registerCalls)
	netMgr.mu.Unlock()

	if registerCount != 0 {
		t.Errorf("expected 0 RegisterRoute calls when no domains, got %d", registerCount)
	}
}

func TestDeploy_MultipleInstances_RegistersRoutePerInstance(t *testing.T) {
	rt := &mockRuntime{}
	netMgr := newMockNetworkManager()
	mgr, _ := newTestManagerWithNetwork(t, rt, netMgr)

	appCfg := newTestAppConfig("multi-route-app", "alpine:latest")
	appCfg.Instances = 3
	appCfg.Domains = []string{"multi.test"}

	_, err := mgr.DeployAppFromConfig(context.Background(), appCfg)
	if err != nil {
		t.Fatalf("deploy failed: %v", err)
	}

	netMgr.mu.Lock()
	registerCount := len(netMgr.registerCalls)
	netMgr.mu.Unlock()

	// 3 containers × 1 domain = 3 route registrations
	if registerCount != 3 {
		t.Errorf("expected 3 RegisterRoute calls, got %d", registerCount)
	}
}

func TestRollingDeploy_RegistersNewRoutes(t *testing.T) {
	rt := &mockRuntime{}
	netMgr := newMockNetworkManager()
	mgr, _ := newTestManagerWithNetwork(t, rt, netMgr)

	// First deploy with domain
	appCfg := newTestAppConfig("rolling-route", "alpine:v1")
	appCfg.Domains = []string{"rolling.test"}
	appCfg.Deploy.GracePeriod = config.Duration{Duration: 10 * time.Millisecond}
	_, err := mgr.DeployAppFromConfig(context.Background(), appCfg)
	if err != nil {
		t.Fatalf("first deploy failed: %v", err)
	}

	// Rolling update
	appCfg2 := newTestAppConfig("rolling-route", "alpine:v2")
	appCfg2.Domains = []string{"rolling.test"}
	appCfg2.Deploy.GracePeriod = config.Duration{Duration: 10 * time.Millisecond}
	_, err = mgr.DeployAppFromConfig(context.Background(), appCfg2)
	if err != nil {
		t.Fatalf("rolling deploy failed: %v", err)
	}

	netMgr.mu.Lock()
	registerCount := len(netMgr.registerCalls)
	deregisterCount := len(netMgr.deregisterCalls)
	netMgr.mu.Unlock()

	// v1 registers 1 route, v2 registers 1 route = 2 total
	if registerCount != 2 {
		t.Errorf("expected 2 RegisterRoute calls, got %d", registerCount)
	}
	// Old container deregistered (empty hostname = all routes)
	if deregisterCount < 1 {
		t.Errorf("expected at least 1 DeregisterRoute call, got %d", deregisterCount)
	}
}

func TestBlueGreenDeploy_RegistersNewRoutes(t *testing.T) {
	rt := &mockRuntime{}
	netMgr := newMockNetworkManager()
	mgr, _ := newTestManagerWithNetwork(t, rt, netMgr)

	// First deploy
	appCfg := newTestAppConfig("bg-route", "alpine:v1")
	appCfg.Domains = []string{"bg.test"}
	appCfg.Deploy.GracePeriod = config.Duration{Duration: 10 * time.Millisecond}
	_, err := mgr.DeployAppFromConfig(context.Background(), appCfg)
	if err != nil {
		t.Fatalf("first deploy failed: %v", err)
	}

	// Blue-green update
	appCfg2 := newTestAppConfig("bg-route", "alpine:v2")
	appCfg2.Domains = []string{"bg.test"}
	appCfg2.Deploy.Strategy = "blue-green"
	appCfg2.Deploy.GracePeriod = config.Duration{Duration: 10 * time.Millisecond}
	_, err = mgr.DeployAppFromConfig(context.Background(), appCfg2)
	if err != nil {
		t.Fatalf("blue-green deploy failed: %v", err)
	}

	netMgr.mu.Lock()
	registerCount := len(netMgr.registerCalls)
	deregisterCount := len(netMgr.deregisterCalls)
	netMgr.mu.Unlock()

	// v1 registers 1, v2 registers 1 = 2 total
	if registerCount != 2 {
		t.Errorf("expected 2 RegisterRoute calls, got %d", registerCount)
	}
	// Old container deregistered
	if deregisterCount < 1 {
		t.Errorf("expected at least 1 DeregisterRoute call, got %d", deregisterCount)
	}
}

func TestStopAndRemoveContainer_DeregistersRoutes(t *testing.T) {
	rt := &mockRuntime{}
	netMgr := newMockNetworkManager()
	st := testutil.TempStore(t)
	logger := testLogger()
	mgr := NewAppManager(rt, st, netMgr, logger)

	c := &store.Container{
		ID:    "route-teardown-container-123",
		AppID: "test-app",
		Image: "alpine:latest",
		State: store.ContainerStateRunning,
		IP:    "10.88.0.5",
	}
	st.CreateContainer(c)

	mgr.stopAndRemoveContainer(context.Background(), c)

	netMgr.mu.Lock()
	deregisterCount := len(netMgr.deregisterCalls)
	netMgr.mu.Unlock()

	// DeregisterRoute should be called with empty hostname (all routes)
	if deregisterCount != 1 {
		t.Errorf("expected 1 DeregisterRoute call, got %d", deregisterCount)
	}
}

