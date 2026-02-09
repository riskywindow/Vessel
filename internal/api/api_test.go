package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/vessel/vessel/internal/manager"
	"github.com/vessel/vessel/internal/runtime"
	"github.com/vessel/vessel/internal/store"
	"github.com/vessel/vessel/testutil"
)

// mockRuntime implements runtime.Runtime for testing.
type mockRuntime struct {
	pullImageFn       func(ctx context.Context, ref string) error
	createContainerFn func(ctx context.Context, opts runtime.ContainerOpts) (*store.Container, error)
	startContainerFn  func(ctx context.Context, containerID string) error
	stopContainerFn   func(ctx context.Context, containerID string, gracePeriod time.Duration) error
	removeContainerFn func(ctx context.Context, containerID string) error
	execInContainerFn func(ctx context.Context, containerID string, cmd []string) error
	containerLogsFn   func(ctx context.Context, containerID string, follow bool, tail int) (io.ReadCloser, error)
	containerStatsFn  func(ctx context.Context, containerID string) (*store.MetricPoint, error)
	getContainerPIDFn func(containerID string) (int, error)

	containerCounter int64
}

func (m *mockRuntime) PullImage(ctx context.Context, ref string) error {
	if m.pullImageFn != nil {
		return m.pullImageFn(ctx, ref)
	}
	return nil
}

func (m *mockRuntime) CreateContainer(ctx context.Context, opts runtime.ContainerOpts) (*store.Container, error) {
	if m.createContainerFn != nil {
		return m.createContainerFn(ctx, opts)
	}
	n := atomic.AddInt64(&m.containerCounter, 1)
	return &store.Container{
		ID:        fmt.Sprintf("mock-container-%012d", n),
		Image:     opts.Image,
		State:     store.ContainerStateCreated,
		CreatedAt: time.Now(),
	}, nil
}

func (m *mockRuntime) StartContainer(ctx context.Context, containerID string) error {
	if m.startContainerFn != nil {
		return m.startContainerFn(ctx, containerID)
	}
	return nil
}

func (m *mockRuntime) StopContainer(ctx context.Context, containerID string, gracePeriod time.Duration) error {
	if m.stopContainerFn != nil {
		return m.stopContainerFn(ctx, containerID, gracePeriod)
	}
	return nil
}

func (m *mockRuntime) RemoveContainer(ctx context.Context, containerID string) error {
	if m.removeContainerFn != nil {
		return m.removeContainerFn(ctx, containerID)
	}
	return nil
}

func (m *mockRuntime) ExecInContainer(ctx context.Context, containerID string, cmd []string) error {
	if m.execInContainerFn != nil {
		return m.execInContainerFn(ctx, containerID, cmd)
	}
	return nil
}

func (m *mockRuntime) ContainerLogs(ctx context.Context, containerID string, follow bool, tail int) (io.ReadCloser, error) {
	if m.containerLogsFn != nil {
		return m.containerLogsFn(ctx, containerID, follow, tail)
	}
	return io.NopCloser(strings.NewReader("test log line\n")), nil
}

func (m *mockRuntime) ContainerStats(ctx context.Context, containerID string) (*store.MetricPoint, error) {
	if m.containerStatsFn != nil {
		return m.containerStatsFn(ctx, containerID)
	}
	return &store.MetricPoint{
		Timestamp:   time.Now(),
		CPUPercent:  25.0,
		MemoryBytes: 1024 * 1024 * 100,
		MemoryLimit: 1024 * 1024 * 512,
		PidsCount:   5,
	}, nil
}

func (m *mockRuntime) GetContainerPID(containerID string) (int, error) {
	if m.getContainerPIDFn != nil {
		return m.getContainerPIDFn(containerID)
	}
	return 12345, nil
}

// setupTestServer creates a test API server with the given options.
func setupTestServer(t *testing.T, apiKeys ...string) (*httptest.Server, *manager.AppManager, store.Store) {
	t.Helper()

	st := testutil.TempStore(t)
	rt := &mockRuntime{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	mgr := manager.NewAppManager(rt, st, nil, logger)

	handler := NewHandler(mgr, rt, st, nil, nil, nil, nil)

	cfg := Config{
		APIKeys: apiKeys,
	}
	srv := NewServer(handler, cfg, logger)

	ts := httptest.NewServer(srv.Router().(http.Handler))
	t.Cleanup(ts.Close)

	return ts, mgr, st
}

// --- Ping Tests ---

func TestPing_ReturnsOK(t *testing.T) {
	ts, _, _ := setupTestServer(t)

	resp, err := http.Get(ts.URL + "/api/ping")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	if result["status"] != "ok" {
		t.Errorf("expected status=ok, got %q", result["status"])
	}
}

func TestPing_NoAuthRequired(t *testing.T) {
	ts, _, _ := setupTestServer(t, "test-key-123")

	// Ping should work without API key even when auth is configured
	resp, err := http.Get(ts.URL + "/api/ping")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

// --- Authentication Tests ---

func TestAuth_BlocksUnauthenticatedRequests(t *testing.T) {
	ts, _, _ := setupTestServer(t, "test-key-123")

	// Request without API key should be rejected
	resp, err := http.Get(ts.URL + "/api/apps")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	errObj := result["error"].(map[string]interface{})
	if errObj["code"] != "UNAUTHORIZED" {
		t.Errorf("expected code=UNAUTHORIZED, got %q", errObj["code"])
	}
}

func TestAuth_AcceptsXAPIKeyHeader(t *testing.T) {
	ts, _, _ := setupTestServer(t, "test-key-123")

	req, _ := http.NewRequest("GET", ts.URL+"/api/apps", nil)
	req.Header.Set("X-API-Key", "test-key-123")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestAuth_AcceptsBearerToken(t *testing.T) {
	ts, _, _ := setupTestServer(t, "test-key-123")

	req, _ := http.NewRequest("GET", ts.URL+"/api/apps", nil)
	req.Header.Set("Authorization", "Bearer test-key-123")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestAuth_RejectsInvalidKey(t *testing.T) {
	ts, _, _ := setupTestServer(t, "test-key-123")

	req, _ := http.NewRequest("GET", ts.URL+"/api/apps", nil)
	req.Header.Set("X-API-Key", "wrong-key")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestAuth_NoKeysConfigured_AllowsAll(t *testing.T) {
	ts, _, _ := setupTestServer(t) // no API keys

	resp, err := http.Get(ts.URL + "/api/apps")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

// --- CORS Tests ---

func TestCORS_HeadersPresent(t *testing.T) {
	ts, _, _ := setupTestServer(t)

	req, _ := http.NewRequest("OPTIONS", ts.URL+"/api/apps", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", "GET")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	allowOrigin := resp.Header.Get("Access-Control-Allow-Origin")
	if allowOrigin == "" {
		t.Error("expected Access-Control-Allow-Origin header to be set")
	}
}

// --- App Tests ---

func TestListApps_Empty(t *testing.T) {
	ts, _, _ := setupTestServer(t)

	resp, err := http.Get(ts.URL + "/api/apps")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestListApps_WithApps(t *testing.T) {
	ts, _, st := setupTestServer(t)

	st.CreateApp(&store.App{
		ID:        "app-test-001",
		Name:      "myapp",
		State:     store.AppStateRunning,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})

	resp, err := http.Get(ts.URL + "/api/apps")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var apps []*store.App
	json.NewDecoder(resp.Body).Decode(&apps)
	if len(apps) != 1 {
		t.Errorf("expected 1 app, got %d", len(apps))
	}
	if len(apps) > 0 && apps[0].Name != "myapp" {
		t.Errorf("expected app name=myapp, got %q", apps[0].Name)
	}
}

func TestGetApp_NotFound(t *testing.T) {
	ts, _, _ := setupTestServer(t)

	resp, err := http.Get(ts.URL + "/api/apps/nonexistent")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestGetApp_Found(t *testing.T) {
	ts, _, st := setupTestServer(t)

	st.CreateApp(&store.App{
		ID:        "app-test-002",
		Name:      "myapp",
		State:     store.AppStateRunning,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})

	resp, err := http.Get(ts.URL + "/api/apps/myapp")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	if result["app"] == nil {
		t.Error("expected app in response")
	}
}

// --- Container Tests ---

func TestListContainers_Empty(t *testing.T) {
	ts, _, _ := setupTestServer(t)

	resp, err := http.Get(ts.URL + "/api/containers")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestListContainers_WithContainers(t *testing.T) {
	ts, _, st := setupTestServer(t)

	st.CreateContainer(&store.Container{
		ID:        "container-test-001",
		AppID:     "app1",
		Image:     "alpine:latest",
		State:     store.ContainerStateRunning,
		CreatedAt: time.Now(),
	})

	resp, err := http.Get(ts.URL + "/api/containers")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var containers []*store.Container
	json.NewDecoder(resp.Body).Decode(&containers)
	if len(containers) != 1 {
		t.Errorf("expected 1 container, got %d", len(containers))
	}
}

func TestListContainers_FilterByApp(t *testing.T) {
	ts, _, st := setupTestServer(t)

	st.CreateApp(&store.App{
		ID:        "app1",
		Name:      "app1",
		State:     store.AppStateRunning,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})
	st.CreateContainer(&store.Container{
		ID:        "container-filter-001",
		AppID:     "app1",
		Image:     "alpine:latest",
		State:     store.ContainerStateRunning,
		CreatedAt: time.Now(),
	})
	st.CreateContainer(&store.Container{
		ID:        "container-filter-002",
		AppID:     "app2",
		Image:     "nginx:latest",
		State:     store.ContainerStateRunning,
		CreatedAt: time.Now(),
	})

	resp, err := http.Get(ts.URL + "/api/containers?app=app1")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var containers []*store.Container
	json.NewDecoder(resp.Body).Decode(&containers)
	if len(containers) != 1 {
		t.Errorf("expected 1 filtered container, got %d", len(containers))
	}
}

func TestGetContainer_NotFound(t *testing.T) {
	ts, _, _ := setupTestServer(t)

	resp, err := http.Get(ts.URL + "/api/containers/nonexistent")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestGetContainer_Found(t *testing.T) {
	ts, _, st := setupTestServer(t)

	st.CreateContainer(&store.Container{
		ID:        "container-test-002",
		AppID:     "app1",
		Image:     "alpine:latest",
		State:     store.ContainerStateRunning,
		CreatedAt: time.Now(),
	})

	resp, err := http.Get(ts.URL + "/api/containers/container-test-002")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

// --- Health Tests ---

func TestGetAllHealth_NoMonitor(t *testing.T) {
	ts, _, _ := setupTestServer(t)

	resp, err := http.Get(ts.URL + "/api/health")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestGetHealthStatus_NoMonitor(t *testing.T) {
	ts, _, _ := setupTestServer(t)

	resp, err := http.Get(ts.URL + "/api/health/some-container")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", resp.StatusCode)
	}
}

func TestGetHealthHistory(t *testing.T) {
	ts, _, st := setupTestServer(t)

	st.CreateHealthResult(&store.HealthResult{
		ContainerID: "container-health-001",
		Status:      store.HealthStatusHealthy,
		Message:     "OK",
		Duration:    100 * time.Millisecond,
		CheckedAt:   time.Now(),
	})

	resp, err := http.Get(ts.URL + "/api/health/container-health-001/history")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

// --- Metrics Tests ---

func TestGetMetrics_NoCollector(t *testing.T) {
	ts, _, _ := setupTestServer(t)

	resp, err := http.Get(ts.URL + "/api/metrics/some-container")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", resp.StatusCode)
	}
}

func TestGetLatestMetrics_NoCollector(t *testing.T) {
	ts, _, _ := setupTestServer(t)

	resp, err := http.Get(ts.URL + "/api/metrics/some-container/latest")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", resp.StatusCode)
	}
}

// --- Secrets Tests ---

func TestListSecrets_NoManager(t *testing.T) {
	ts, _, _ := setupTestServer(t)

	resp, err := http.Get(ts.URL + "/api/secrets")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", resp.StatusCode)
	}
}

func TestSetSecret_NoManager(t *testing.T) {
	ts, _, _ := setupTestServer(t)

	body := bytes.NewBufferString(`{"key":"test","value":"secret"}`)
	resp, err := http.Post(ts.URL+"/api/secrets", "application/json", body)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", resp.StatusCode)
	}
}

func TestDeleteSecret_NoManager(t *testing.T) {
	ts, _, _ := setupTestServer(t)

	req, _ := http.NewRequest("DELETE", ts.URL+"/api/secrets/mykey", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", resp.StatusCode)
	}
}

// --- Network Tests ---

func TestGetNetworkRoutes_NoManager(t *testing.T) {
	ts, _, _ := setupTestServer(t)

	resp, err := http.Get(ts.URL + "/api/network/routes")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

// --- Certs Tests ---

func TestListCerts_NoNetwork(t *testing.T) {
	ts, _, _ := setupTestServer(t)

	resp, err := http.Get(ts.URL + "/api/certs")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var result struct {
		TLSEnabled bool     `json:"tls_enabled"`
		Certs      []string `json:"certs"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	if result.TLSEnabled {
		t.Error("expected tls_enabled=false")
	}
}

// --- System Tests ---

func TestGetSystemInfo(t *testing.T) {
	ts, _, st := setupTestServer(t)

	st.CreateApp(&store.App{
		ID:        "app-sys-001",
		Name:      "webapp",
		State:     store.AppStateRunning,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})
	st.CreateContainer(&store.Container{
		ID:        "container-sys-001",
		AppID:     "webapp",
		Image:     "alpine:latest",
		State:     store.ContainerStateRunning,
		CreatedAt: time.Now(),
	})
	st.CreateContainer(&store.Container{
		ID:        "container-sys-002",
		AppID:     "webapp",
		Image:     "alpine:latest",
		State:     store.ContainerStateStopped,
		CreatedAt: time.Now(),
	})

	resp, err := http.Get(ts.URL + "/api/system")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if result["version"] != "0.1.0" {
		t.Errorf("expected version=0.1.0, got %q", result["version"])
	}
	if result["apps"].(float64) != 1 {
		t.Errorf("expected apps=1, got %v", result["apps"])
	}
	if result["containers"].(float64) != 2 {
		t.Errorf("expected containers=2, got %v", result["containers"])
	}
	if result["running_containers"].(float64) != 1 {
		t.Errorf("expected running_containers=1, got %v", result["running_containers"])
	}
}

// --- Content Type Tests ---

func TestContentType_JSON(t *testing.T) {
	ts, _, _ := setupTestServer(t)

	resp, err := http.Get(ts.URL + "/api/ping")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}
}

func TestErrorResponse_Format(t *testing.T) {
	ts, _, _ := setupTestServer(t)

	resp, err := http.Get(ts.URL + "/api/apps/nonexistent")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	errObj, ok := result["error"].(map[string]interface{})
	if !ok {
		t.Fatal("expected error object in response")
	}
	if errObj["code"] == nil || errObj["code"] == "" {
		t.Error("expected non-empty error code")
	}
	if errObj["message"] == nil || errObj["message"] == "" {
		t.Error("expected non-empty error message")
	}
}

// --- Server Lifecycle Tests ---

func TestServerConfig_DefaultAddr(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	st := testutil.TempStore(t)
	rt := &mockRuntime{}
	mgr := manager.NewAppManager(rt, st, nil, logger)
	handler := NewHandler(mgr, rt, st, nil, nil, nil, nil)

	srv := NewServer(handler, Config{}, logger)
	if srv.server.Addr != ":8080" {
		t.Errorf("expected default addr :8080, got %q", srv.server.Addr)
	}
}

func TestServerConfig_CustomAddr(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	st := testutil.TempStore(t)
	rt := &mockRuntime{}
	mgr := manager.NewAppManager(rt, st, nil, logger)
	handler := NewHandler(mgr, rt, st, nil, nil, nil, nil)

	srv := NewServer(handler, Config{Addr: ":9090"}, logger)
	if srv.server.Addr != ":9090" {
		t.Errorf("expected addr :9090, got %q", srv.server.Addr)
	}
}

func TestServer_StartStop(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	st := testutil.TempStore(t)
	rt := &mockRuntime{}
	mgr := manager.NewAppManager(rt, st, nil, logger)
	handler := NewHandler(mgr, rt, st, nil, nil, nil, nil)

	srv := NewServer(handler, Config{Addr: ":0"}, logger)

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Start()
	}()

	time.Sleep(50 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Stop(ctx); err != nil {
		t.Fatalf("stop failed: %v", err)
	}

	if err := <-errCh; err != nil {
		t.Errorf("unexpected start error: %v", err)
	}
}

// --- Deploy/Rollback Tests ---

func TestDeployApp_InvalidJSON(t *testing.T) {
	ts, _, st := setupTestServer(t)

	st.CreateApp(&store.App{
		ID:        "app-deploy-001",
		Name:      "myapp",
		State:     store.AppStateRunning,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})

	body := bytes.NewBufferString(`{invalid json}`)
	resp, err := http.Post(ts.URL+"/api/apps/myapp/deploy", "application/json", body)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestRollbackApp_InvalidJSON(t *testing.T) {
	ts, _, _ := setupTestServer(t)

	body := bytes.NewBufferString(`{invalid json}`)
	resp, err := http.Post(ts.URL+"/api/apps/myapp/rollback", "application/json", body)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

// --- Logs Tests ---

func TestGetContainerLogs(t *testing.T) {
	ts, _, st := setupTestServer(t)

	st.CreateContainer(&store.Container{
		ID:        "container-logs-001",
		AppID:     "app1",
		Image:     "alpine:latest",
		State:     store.ContainerStateRunning,
		CreatedAt: time.Now(),
	})

	resp, err := http.Get(ts.URL + "/api/containers/container-logs-001/logs?tail=50")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	if !strings.Contains(result["logs"], "test log line") {
		t.Errorf("expected log output, got %q", result["logs"])
	}
}

// --- Stats Tests ---

func TestGetContainerStats(t *testing.T) {
	ts, _, st := setupTestServer(t)

	st.CreateContainer(&store.Container{
		ID:        "container-stats-001",
		AppID:     "app1",
		Image:     "alpine:latest",
		State:     store.ContainerStateRunning,
		CreatedAt: time.Now(),
	})

	resp, err := http.Get(ts.URL + "/api/containers/container-stats-001/stats")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var stats store.MetricPoint
	json.NewDecoder(resp.Body).Decode(&stats)
	if stats.CPUPercent != 25.0 {
		t.Errorf("expected CPU=25.0, got %f", stats.CPUPercent)
	}
}

// --- Config Tests ---

func TestAPIConfig_Parse(t *testing.T) {
	tomlData := `
[[app]]
name = "test"
image = "alpine:latest"

[api]
enabled = true
addr = ":9090"
api_keys = ["key1", "key2"]
cors_hosts = ["http://localhost:5173"]
`
	configPath := t.TempDir() + "/vessel.toml"
	if err := os.WriteFile(configPath, []byte(tomlData), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	var cfg struct {
		API struct {
			Enabled   bool     `toml:"enabled"`
			Addr      string   `toml:"addr"`
			APIKeys   []string `toml:"api_keys"`
			CORSHosts []string `toml:"cors_hosts"`
		} `toml:"api"`
		Apps []struct {
			Name  string `toml:"name"`
			Image string `toml:"image"`
		} `toml:"app"`
	}

	if _, err := toml.DecodeFile(configPath, &cfg); err != nil {
		t.Fatalf("failed to parse config: %v", err)
	}

	if !cfg.API.Enabled {
		t.Error("expected api.enabled=true")
	}
	if cfg.API.Addr != ":9090" {
		t.Errorf("expected addr=:9090, got %q", cfg.API.Addr)
	}
	if len(cfg.API.APIKeys) != 2 {
		t.Errorf("expected 2 API keys, got %d", len(cfg.API.APIKeys))
	}
	if len(cfg.API.CORSHosts) != 1 {
		t.Errorf("expected 1 CORS host, got %d", len(cfg.API.CORSHosts))
	}
}

func TestNewHandler_NilComponents(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	st := testutil.TempStore(t)
	rt := &mockRuntime{}
	mgr := manager.NewAppManager(rt, st, nil, logger)

	// All optional components nil — should not panic
	h := NewHandler(mgr, rt, st, nil, nil, nil, nil)
	if h == nil {
		t.Fatal("expected non-nil handler")
	}
	if h.manager != mgr {
		t.Error("expected manager to be set")
	}
	if h.secretManager != nil {
		t.Error("expected secretManager to be nil")
	}
	if h.healthMonitor != nil {
		t.Error("expected healthMonitor to be nil")
	}
	if h.metricsCollector != nil {
		t.Error("expected metricsCollector to be nil")
	}
	if h.network != nil {
		t.Error("expected network to be nil")
	}
}

func TestStopApp_NotFound(t *testing.T) {
	ts, _, _ := setupTestServer(t)

	resp, err := http.Post(ts.URL+"/api/apps/nonexistent/stop", "application/json", nil)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", resp.StatusCode)
	}
}

func TestRemoveApp_NotFound(t *testing.T) {
	ts, _, _ := setupTestServer(t)

	req, _ := http.NewRequest("DELETE", ts.URL+"/api/apps/nonexistent", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", resp.StatusCode)
	}
}

func TestCheckHealthNow_NoMonitor(t *testing.T) {
	ts, _, _ := setupTestServer(t)

	resp, err := http.Post(ts.URL+"/api/health/some-container/check", "application/json", nil)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", resp.StatusCode)
	}
}

func TestSetSecret_InvalidJSON(t *testing.T) {
	// Use a server with a secret manager to test JSON validation
	st := testutil.TempStore(t)
	rt := &mockRuntime{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	mgr := manager.NewAppManager(rt, st, nil, logger)

	sm, _ := store.NewSecretManager(st, "test-password", nil)
	handler := NewHandler(mgr, rt, st, sm, nil, nil, nil)
	srv := NewServer(handler, Config{}, logger)
	ts := httptest.NewServer(srv.Router().(http.Handler))
	defer ts.Close()

	body := bytes.NewBufferString(`{bad json}`)
	resp, err := http.Post(ts.URL+"/api/secrets", "application/json", body)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestSetSecret_EmptyKey(t *testing.T) {
	st := testutil.TempStore(t)
	rt := &mockRuntime{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	mgr := manager.NewAppManager(rt, st, nil, logger)

	sm, _ := store.NewSecretManager(st, "test-password", nil)
	handler := NewHandler(mgr, rt, st, sm, nil, nil, nil)
	srv := NewServer(handler, Config{}, logger)
	ts := httptest.NewServer(srv.Router().(http.Handler))
	defer ts.Close()

	body := bytes.NewBufferString(`{"key":"","value":"secret"}`)
	resp, err := http.Post(ts.URL+"/api/secrets", "application/json", body)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestImportCert_NoNetwork(t *testing.T) {
	ts, _, _ := setupTestServer(t)

	body := bytes.NewBufferString(`{"hostname":"example.com","cert_path":"/tmp/cert.pem","key_path":"/tmp/key.pem"}`)
	resp, err := http.Post(ts.URL+"/api/certs", "application/json", body)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", resp.StatusCode)
	}
}
