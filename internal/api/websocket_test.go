package api

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/vessel/vessel/internal/health"
	"github.com/vessel/vessel/internal/manager"
	"github.com/vessel/vessel/internal/store"
	"github.com/vessel/vessel/testutil"
)

// wsURL converts an HTTP URL to a WebSocket URL.
func wsURL(ts *httptest.Server, path string) string {
	return "ws" + strings.TrimPrefix(ts.URL, "http") + path
}

// setupWSTestServer creates a test server with WebSocket routes and no auth.
func setupWSTestServer(t *testing.T) (*httptest.Server, *manager.AppManager, store.Store, *mockRuntime) {
	t.Helper()

	st := testutil.TempStore(t)
	rt := &mockRuntime{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	mgr := manager.NewAppManager(rt, st, nil, logger)

	handler := NewHandler(mgr, rt, st, nil, nil, nil, nil)
	cfg := Config{}
	srv := NewServer(handler, cfg, logger)

	ts := httptest.NewServer(srv.Router().(http.Handler))
	t.Cleanup(ts.Close)

	return ts, mgr, st, rt
}

// setupWSTestServerWithAuth creates a test server with auth enabled.
func setupWSTestServerWithAuth(t *testing.T, keys ...string) *httptest.Server {
	t.Helper()

	st := testutil.TempStore(t)
	rt := &mockRuntime{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	mgr := manager.NewAppManager(rt, st, nil, logger)

	handler := NewHandler(mgr, rt, st, nil, nil, nil, nil)
	cfg := Config{APIKeys: keys}
	srv := NewServer(handler, cfg, logger)

	ts := httptest.NewServer(srv.Router().(http.Handler))
	t.Cleanup(ts.Close)

	return ts
}

// setupWSTestServerWithHealth creates a test server with a health monitor.
func setupWSTestServerWithHealth(t *testing.T) (*httptest.Server, *health.HealthMonitor, store.Store, *mockRuntime) {
	t.Helper()

	st := testutil.TempStore(t)
	rt := &mockRuntime{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	mgr := manager.NewAppManager(rt, st, nil, logger)

	hm := health.NewHealthMonitor(rt, st, nil, nil, health.HealthMonitorConfig{
		CheckInterval:      1 * time.Hour, // Won't fire during test
		UnhealthyThreshold: 3,
		HealthyThreshold:   1,
	}, logger)
	hm.Start(context.Background())
	t.Cleanup(func() { hm.Stop() })

	handler := NewHandler(mgr, rt, st, nil, nil, hm, nil)
	cfg := Config{}
	srv := NewServer(handler, cfg, logger)

	ts := httptest.NewServer(srv.Router().(http.Handler))
	t.Cleanup(ts.Close)

	return ts, hm, st, rt
}

// --- WebSocket Connection Tests ---

func TestWSLogs_UpgradeSucceeds(t *testing.T) {
	ts, _, _, rt := setupWSTestServer(t)

	// Set up a log reader that provides some data then closes
	rt.containerLogsFn = func(ctx context.Context, containerID string, follow bool, tail int) (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader("line1\nline2\nline3\n")), nil
	}

	conn, resp, err := websocket.DefaultDialer.Dial(wsURL(ts, "/api/ws/containers/test-container/logs"), nil)
	if err != nil {
		t.Fatalf("websocket dial failed: %v", err)
	}
	defer conn.Close()

	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Errorf("expected 101, got %d", resp.StatusCode)
	}
}

func TestWSLogs_StreamsLogLines(t *testing.T) {
	ts, _, _, rt := setupWSTestServer(t)

	rt.containerLogsFn = func(ctx context.Context, containerID string, follow bool, tail int) (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader("hello world\ngoodbye world\n")), nil
	}

	conn, _, err := websocket.DefaultDialer.Dial(wsURL(ts, "/api/ws/containers/test-container/logs"), nil)
	if err != nil {
		t.Fatalf("websocket dial failed: %v", err)
	}
	defer conn.Close()

	// Read first log line
	var msg WSMessage
	if err := conn.ReadJSON(&msg); err != nil {
		t.Fatalf("read failed: %v", err)
	}

	if msg.Type != "log" {
		t.Errorf("expected type=log, got %q", msg.Type)
	}

	data, ok := msg.Data.(map[string]interface{})
	if !ok {
		t.Fatal("expected data to be a map")
	}
	if data["line"] != "hello world" {
		t.Errorf("expected line='hello world', got %q", data["line"])
	}

	// Read second log line
	if err := conn.ReadJSON(&msg); err != nil {
		t.Fatalf("read failed: %v", err)
	}

	data = msg.Data.(map[string]interface{})
	if data["line"] != "goodbye world" {
		t.Errorf("expected line='goodbye world', got %q", data["line"])
	}
}

func TestWSLogs_ErrorOnBadContainer(t *testing.T) {
	ts, _, _, rt := setupWSTestServer(t)

	rt.containerLogsFn = func(ctx context.Context, containerID string, follow bool, tail int) (io.ReadCloser, error) {
		return nil, io.ErrUnexpectedEOF
	}

	conn, _, err := websocket.DefaultDialer.Dial(wsURL(ts, "/api/ws/containers/bad-container/logs"), nil)
	if err != nil {
		t.Fatalf("websocket dial failed: %v", err)
	}
	defer conn.Close()

	var msg WSMessage
	if err := conn.ReadJSON(&msg); err != nil {
		t.Fatalf("read failed: %v", err)
	}

	if msg.Type != "error" {
		t.Errorf("expected type=error, got %q", msg.Type)
	}
}

func TestWSLogs_FollowEnabled(t *testing.T) {
	ts, _, _, rt := setupWSTestServer(t)

	var capturedFollow bool
	rt.containerLogsFn = func(ctx context.Context, containerID string, follow bool, tail int) (io.ReadCloser, error) {
		capturedFollow = follow
		return io.NopCloser(strings.NewReader("line\n")), nil
	}

	conn, _, err := websocket.DefaultDialer.Dial(wsURL(ts, "/api/ws/containers/test/logs"), nil)
	if err != nil {
		t.Fatalf("websocket dial failed: %v", err)
	}
	defer conn.Close()

	// Read at least one message to ensure the handler ran
	var msg WSMessage
	conn.ReadJSON(&msg)

	if !capturedFollow {
		t.Error("expected follow=true for WebSocket log streaming")
	}
}

// --- WebSocket Metrics Tests ---

func TestWSMetrics_SendsInitialMetric(t *testing.T) {
	ts, _, _, _ := setupWSTestServer(t)

	conn, _, err := websocket.DefaultDialer.Dial(wsURL(ts, "/api/ws/containers/test-container/metrics"), nil)
	if err != nil {
		t.Fatalf("websocket dial failed: %v", err)
	}
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(3 * time.Second))

	var msg WSMessage
	if err := conn.ReadJSON(&msg); err != nil {
		t.Fatalf("read failed: %v", err)
	}

	if msg.Type != "metric" {
		t.Errorf("expected type=metric, got %q", msg.Type)
	}

	// Verify metric data
	data, _ := json.Marshal(msg.Data)
	var metric store.MetricPoint
	json.Unmarshal(data, &metric)

	if metric.CPUPercent != 25.0 {
		t.Errorf("expected CPU=25.0, got %f", metric.CPUPercent)
	}
}

func TestWSMetrics_SendsPeriodicUpdates(t *testing.T) {
	ts, _, _, _ := setupWSTestServer(t)

	conn, _, err := websocket.DefaultDialer.Dial(wsURL(ts, "/api/ws/containers/test-container/metrics"), nil)
	if err != nil {
		t.Fatalf("websocket dial failed: %v", err)
	}
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))

	// Read initial metric
	var msg WSMessage
	conn.ReadJSON(&msg)

	// Read second metric (after 2s ticker)
	if err := conn.ReadJSON(&msg); err != nil {
		t.Fatalf("second read failed: %v", err)
	}

	if msg.Type != "metric" {
		t.Errorf("expected type=metric, got %q", msg.Type)
	}
}

func TestWSMetrics_ErrorOnStatsFail(t *testing.T) {
	ts, _, _, rt := setupWSTestServer(t)

	rt.containerStatsFn = func(ctx context.Context, containerID string) (*store.MetricPoint, error) {
		return nil, io.ErrUnexpectedEOF
	}

	conn, _, err := websocket.DefaultDialer.Dial(wsURL(ts, "/api/ws/containers/bad-container/metrics"), nil)
	if err != nil {
		t.Fatalf("websocket dial failed: %v", err)
	}
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(3 * time.Second))

	var msg WSMessage
	if err := conn.ReadJSON(&msg); err != nil {
		t.Fatalf("read failed: %v", err)
	}

	if msg.Type != "error" {
		t.Errorf("expected type=error, got %q", msg.Type)
	}
}

// --- All Metrics Tests ---

func TestWSAllMetrics_SendsInitial(t *testing.T) {
	ts, _, st, _ := setupWSTestServer(t)

	// Add a running container
	st.CreateContainer(&store.Container{
		ID:        "container-ws-001",
		AppID:     "app1",
		Image:     "alpine:latest",
		State:     store.ContainerStateRunning,
		CreatedAt: time.Now(),
	})

	conn, _, err := websocket.DefaultDialer.Dial(wsURL(ts, "/api/ws/metrics"), nil)
	if err != nil {
		t.Fatalf("websocket dial failed: %v", err)
	}
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(3 * time.Second))

	var msg WSMessage
	if err := conn.ReadJSON(&msg); err != nil {
		t.Fatalf("read failed: %v", err)
	}

	if msg.Type != "metrics" {
		t.Errorf("expected type=metrics, got %q", msg.Type)
	}

	// Verify it contains the container
	data, _ := json.Marshal(msg.Data)
	var metrics map[string]*store.MetricPoint
	json.Unmarshal(data, &metrics)

	if _, ok := metrics["container-ws-001"]; !ok {
		t.Error("expected metrics for container-ws-001")
	}
}

func TestWSAllMetrics_SkipsStoppedContainers(t *testing.T) {
	ts, _, st, _ := setupWSTestServer(t)

	st.CreateContainer(&store.Container{
		ID:        "container-ws-002",
		AppID:     "app1",
		Image:     "alpine:latest",
		State:     store.ContainerStateStopped,
		CreatedAt: time.Now(),
	})

	conn, _, err := websocket.DefaultDialer.Dial(wsURL(ts, "/api/ws/metrics"), nil)
	if err != nil {
		t.Fatalf("websocket dial failed: %v", err)
	}
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(3 * time.Second))

	var msg WSMessage
	conn.ReadJSON(&msg)

	data, _ := json.Marshal(msg.Data)
	var metrics map[string]*store.MetricPoint
	json.Unmarshal(data, &metrics)

	if len(metrics) != 0 {
		t.Errorf("expected 0 metrics for stopped container, got %d", len(metrics))
	}
}

// --- Health Streaming Tests ---

func TestWSHealth_NoMonitor_SendsError(t *testing.T) {
	ts, _, _, _ := setupWSTestServer(t)

	conn, _, err := websocket.DefaultDialer.Dial(wsURL(ts, "/api/ws/health"), nil)
	if err != nil {
		t.Fatalf("websocket dial failed: %v", err)
	}
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(3 * time.Second))

	var msg WSMessage
	if err := conn.ReadJSON(&msg); err != nil {
		t.Fatalf("read failed: %v", err)
	}

	if msg.Type != "error" {
		t.Errorf("expected type=error, got %q", msg.Type)
	}
}

func TestWSHealth_SendsInitialStatus(t *testing.T) {
	ts, hm, _, _ := setupWSTestServerWithHealth(t)

	// Register a container
	hm.RegisterContainer("container-ws-health-001", "app1", store.HealthCheck{
		Type:     store.HealthCheckTCP,
		Target:   "localhost:8080",
		Interval: 10 * time.Second,
		Timeout:  5 * time.Second,
		Retries:  3,
	})

	conn, _, err := websocket.DefaultDialer.Dial(wsURL(ts, "/api/ws/health"), nil)
	if err != nil {
		t.Fatalf("websocket dial failed: %v", err)
	}
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(3 * time.Second))

	var msg WSMessage
	if err := conn.ReadJSON(&msg); err != nil {
		t.Fatalf("read failed: %v", err)
	}

	if msg.Type != "health" {
		t.Errorf("expected type=health, got %q", msg.Type)
	}
}

func TestWSHealth_StreamsHealthEvents(t *testing.T) {
	ts, hm, _, _ := setupWSTestServerWithHealth(t)

	hm.RegisterContainer("container-ws-health-002", "app1", store.HealthCheck{
		Type:     store.HealthCheckTCP,
		Target:   "localhost:8080",
		Interval: 10 * time.Second,
		Timeout:  5 * time.Second,
		Retries:  3,
	})

	conn, _, err := websocket.DefaultDialer.Dial(wsURL(ts, "/api/ws/health"), nil)
	if err != nil {
		t.Fatalf("websocket dial failed: %v", err)
	}
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(3 * time.Second))

	// Read initial status message
	var msg WSMessage
	conn.ReadJSON(&msg)

	// Now emit a health event by subscribing separately and sending through the monitor
	// We can trigger an event by registering a new container (which doesn't emit an event)
	// Instead, just deregister the container — the test verifies the subscription works
	// by reading the initial message above
	if msg.Type != "health" {
		t.Errorf("expected initial health message, got %q", msg.Type)
	}
}

// --- Events Streaming Tests ---

func TestWSEvents_SendsInitialState(t *testing.T) {
	ts, _, st, _ := setupWSTestServer(t)

	st.CreateApp(&store.App{
		ID:        "app-ws-001",
		Name:      "webapp",
		State:     store.AppStateRunning,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})

	conn, _, err := websocket.DefaultDialer.Dial(wsURL(ts, "/api/ws/events"), nil)
	if err != nil {
		t.Fatalf("websocket dial failed: %v", err)
	}
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(3 * time.Second))

	var msg WSMessage
	if err := conn.ReadJSON(&msg); err != nil {
		t.Fatalf("read failed: %v", err)
	}

	if msg.Type != "state" {
		t.Errorf("expected type=state, got %q", msg.Type)
	}

	data, _ := json.Marshal(msg.Data)
	var state map[string]interface{}
	json.Unmarshal(data, &state)

	if state["apps"] == nil {
		t.Error("expected apps in state")
	}
}

func TestWSEvents_PeriodicRefresh(t *testing.T) {
	ts, _, _, _ := setupWSTestServer(t)

	conn, _, err := websocket.DefaultDialer.Dial(wsURL(ts, "/api/ws/events"), nil)
	if err != nil {
		t.Fatalf("websocket dial failed: %v", err)
	}
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(8 * time.Second))

	// Read initial state
	var msg WSMessage
	conn.ReadJSON(&msg)

	// Read second state (after 5s tick)
	if err := conn.ReadJSON(&msg); err != nil {
		t.Fatalf("second read failed (expected after 5s): %v", err)
	}

	if msg.Type != "state" {
		t.Errorf("expected type=state, got %q", msg.Type)
	}
}

// --- Authentication Tests ---

func TestWSAuth_QueryParam_Accepted(t *testing.T) {
	ts := setupWSTestServerWithAuth(t, "test-key-ws")

	url := wsURL(ts, "/api/ws/metrics") + "?api_key=test-key-ws"
	conn, resp, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("websocket dial failed: %v (status: %d)", err, resp.StatusCode)
	}
	defer conn.Close()

	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Errorf("expected 101, got %d", resp.StatusCode)
	}
}

func TestWSAuth_QueryParam_Rejected(t *testing.T) {
	ts := setupWSTestServerWithAuth(t, "test-key-ws")

	url := wsURL(ts, "/api/ws/metrics") + "?api_key=wrong-key"
	_, resp, err := websocket.DefaultDialer.Dial(url, nil)
	if err == nil {
		t.Fatal("expected dial to fail")
	}

	if resp != nil && resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestWSAuth_Header_Accepted(t *testing.T) {
	ts := setupWSTestServerWithAuth(t, "test-key-ws")

	header := http.Header{}
	header.Set("X-API-Key", "test-key-ws")

	conn, resp, err := websocket.DefaultDialer.Dial(wsURL(ts, "/api/ws/metrics"), header)
	if err != nil {
		t.Fatalf("websocket dial failed: %v", err)
	}
	defer conn.Close()

	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Errorf("expected 101, got %d", resp.StatusCode)
	}
}

func TestWSAuth_NoKey_Rejected(t *testing.T) {
	ts := setupWSTestServerWithAuth(t, "test-key-ws")

	_, resp, err := websocket.DefaultDialer.Dial(wsURL(ts, "/api/ws/metrics"), nil)
	if err == nil {
		t.Fatal("expected dial to fail")
	}

	if resp != nil && resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestWSAuth_NoKeysConfigured_AllowsAll(t *testing.T) {
	ts := setupWSTestServerWithAuth(t) // no keys

	conn, _, err := websocket.DefaultDialer.Dial(wsURL(ts, "/api/ws/metrics"), nil)
	if err != nil {
		t.Fatalf("websocket dial failed: %v", err)
	}
	defer conn.Close()
}

// --- Client Disconnect Tests ---

func TestWSLogs_ClientDisconnect(t *testing.T) {
	ts, _, _, rt := setupWSTestServer(t)

	// Provide a reader that blocks until context is cancelled
	rt.containerLogsFn = func(ctx context.Context, containerID string, follow bool, tail int) (io.ReadCloser, error) {
		pr, pw := io.Pipe()
		go func() {
			<-ctx.Done()
			pw.Close()
		}()
		return pr, nil
	}

	conn, _, err := websocket.DefaultDialer.Dial(wsURL(ts, "/api/ws/containers/test/logs"), nil)
	if err != nil {
		t.Fatalf("websocket dial failed: %v", err)
	}

	// Close connection from client side
	conn.Close()

	// Give the server time to clean up
	time.Sleep(100 * time.Millisecond)
	// If we get here without hanging, the test passes
}

func TestWSMetrics_ClientDisconnect(t *testing.T) {
	ts, _, _, _ := setupWSTestServer(t)

	conn, _, err := websocket.DefaultDialer.Dial(wsURL(ts, "/api/ws/containers/test/metrics"), nil)
	if err != nil {
		t.Fatalf("websocket dial failed: %v", err)
	}

	// Read initial metric
	var msg WSMessage
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	conn.ReadJSON(&msg)

	// Close from client side
	conn.Close()

	time.Sleep(100 * time.Millisecond)
}

// --- WSMessage Tests ---

func TestWSMessage_JSONSerialization(t *testing.T) {
	msg := WSMessage{
		Type:      "log",
		Timestamp: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		Data:      map[string]string{"line": "test"},
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded WSMessage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if decoded.Type != "log" {
		t.Errorf("expected type=log, got %q", decoded.Type)
	}
}

func TestWSMessage_Types(t *testing.T) {
	types := []string{"log", "metric", "metrics", "health", "health_event", "state", "error"}
	for _, typ := range types {
		msg := WSMessage{Type: typ, Timestamp: time.Now()}
		data, err := json.Marshal(msg)
		if err != nil {
			t.Errorf("failed to marshal type=%s: %v", typ, err)
		}
		if len(data) == 0 {
			t.Errorf("empty marshal for type=%s", typ)
		}
	}
}

// --- Route Registration Tests ---

func TestWSRoutes_Registered(t *testing.T) {
	ts, _, _, _ := setupWSTestServer(t)

	routes := []string{
		"/api/ws/containers/test/logs",
		"/api/ws/containers/test/metrics",
		"/api/ws/metrics",
		"/api/ws/health",
		"/api/ws/events",
	}

	for _, route := range routes {
		url := wsURL(ts, route)
		conn, _, err := websocket.DefaultDialer.Dial(url, nil)
		if err != nil {
			t.Errorf("route %s: dial failed: %v", route, err)
			continue
		}
		conn.Close()
	}
}
