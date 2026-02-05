package daemon

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRequestSerialization(t *testing.T) {
	tests := []struct {
		name   string
		method string
		params interface{}
	}{
		{"no params", "ping", nil},
		{"with string params", "apps.get", map[string]string{"name": "myapp"}},
		{"with complex params", "apps.deploy", map[string]interface{}{
			"name":      "myapp",
			"image":     "alpine:latest",
			"instances": 2,
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var paramsJSON json.RawMessage
			if tt.params != nil {
				data, err := json.Marshal(tt.params)
				if err != nil {
					t.Fatalf("failed to marshal params: %v", err)
				}
				paramsJSON = data
			}

			req := Request{
				Method: tt.method,
				Params: paramsJSON,
			}

			// Marshal and unmarshal
			data, err := json.Marshal(req)
			if err != nil {
				t.Fatalf("failed to marshal request: %v", err)
			}

			var decoded Request
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("failed to unmarshal request: %v", err)
			}

			if decoded.Method != tt.method {
				t.Errorf("expected method %q, got %q", tt.method, decoded.Method)
			}
		})
	}
}

func TestResponseSerialization_Success(t *testing.T) {
	data, _ := json.Marshal(map[string]string{"status": "ok"})
	resp := Response{
		Data: data,
	}

	encoded, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal response: %v", err)
	}

	var decoded Response
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if decoded.Error != nil {
		t.Error("expected no error in response")
	}
	if decoded.Data == nil {
		t.Fatal("expected data in response")
	}

	var result map[string]string
	if err := json.Unmarshal(decoded.Data, &result); err != nil {
		t.Fatalf("failed to unmarshal data: %v", err)
	}
	if result["status"] != "ok" {
		t.Errorf("expected status 'ok', got %q", result["status"])
	}
}

func TestResponseSerialization_Error(t *testing.T) {
	resp := Response{
		Error: &ErrorResponse{
			Code:    "APP_NOT_FOUND",
			Message: "app 'myapp' not found",
		},
	}

	encoded, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal response: %v", err)
	}

	var decoded Response
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if decoded.Error == nil {
		t.Fatal("expected error in response")
	}
	if decoded.Error.Code != "APP_NOT_FOUND" {
		t.Errorf("expected code 'APP_NOT_FOUND', got %q", decoded.Error.Code)
	}
	if decoded.Error.Message != "app 'myapp' not found" {
		t.Errorf("expected message 'app 'myapp' not found', got %q", decoded.Error.Message)
	}
}

func TestClientCall_ConnectFailure(t *testing.T) {
	// Use a non-existent socket path
	client := NewClient("/tmp/vessel-test-nonexistent-" + t.Name() + ".sock")
	var result map[string]string
	err := client.Call("ping", nil, &result)
	if err == nil {
		t.Fatal("expected error when connecting to non-existent socket")
	}
}

func TestClientServer_PingPong(t *testing.T) {
	// Create a temp socket
	dir := t.TempDir()
	sockPath := filepath.Join(dir, "test.sock")

	// Start a test server that handles ping
	listener, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer listener.Close()

	// Server goroutine
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		conn.SetDeadline(time.Now().Add(5 * time.Second))

		// Read request
		var req Request
		decoder := json.NewDecoder(conn)
		if err := decoder.Decode(&req); err != nil {
			return
		}

		// Write response
		data, _ := json.Marshal(map[string]string{"status": "ok"})
		resp := Response{Data: data}
		encoder := json.NewEncoder(conn)
		encoder.Encode(resp)
	}()

	// Client sends ping
	client := NewClient(sockPath)
	err = client.Ping()
	if err != nil {
		t.Fatalf("Ping failed: %v", err)
	}

	<-serverDone
}

func TestClientServer_ErrorResponse(t *testing.T) {
	dir := t.TempDir()
	sockPath := filepath.Join(dir, "test.sock")

	listener, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer listener.Close()

	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		conn.SetDeadline(time.Now().Add(5 * time.Second))

		// Read request
		var req Request
		decoder := json.NewDecoder(conn)
		if err := decoder.Decode(&req); err != nil {
			return
		}

		// Write error response
		resp := Response{
			Error: &ErrorResponse{
				Code:    "APP_NOT_FOUND",
				Message: "app not found",
			},
		}
		encoder := json.NewEncoder(conn)
		encoder.Encode(resp)
	}()

	client := NewClient(sockPath)
	var result map[string]string
	err = client.Call("apps.get", map[string]string{"name": "nonexistent"}, &result)
	if err == nil {
		t.Fatal("expected error for not-found app")
	}

	<-serverDone
}

func TestClientServer_MultipleRequests(t *testing.T) {
	dir := t.TempDir()
	sockPath := filepath.Join(dir, "test.sock")

	listener, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer listener.Close()

	// Server handles multiple sequential connections
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				c.SetDeadline(time.Now().Add(5 * time.Second))

				var req Request
				decoder := json.NewDecoder(c)
				if err := decoder.Decode(&req); err != nil {
					return
				}

				data, _ := json.Marshal(map[string]string{"method": req.Method})
				resp := Response{Data: data}
				encoder := json.NewEncoder(c)
				encoder.Encode(resp)
			}(conn)
		}
	}()

	client := NewClient(sockPath)

	// Send multiple sequential requests
	methods := []string{"ping", "apps.list", "apps.get"}
	for _, method := range methods {
		var result map[string]string
		err := client.Call(method, nil, &result)
		if err != nil {
			t.Fatalf("Call(%s) failed: %v", method, err)
		}
		if result["method"] != method {
			t.Errorf("expected method %q echoed back, got %q", method, result["method"])
		}

		select {
		case <-ctx.Done():
			t.Fatal("test timed out")
		default:
		}
	}
}

func TestClientCall_InvalidResponse(t *testing.T) {
	dir := t.TempDir()
	sockPath := filepath.Join(dir, "test.sock")

	listener, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer listener.Close()

	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		conn.SetDeadline(time.Now().Add(5 * time.Second))

		// Read request
		var req Request
		decoder := json.NewDecoder(conn)
		if err := decoder.Decode(&req); err != nil {
			return
		}

		// Write invalid JSON
		conn.Write([]byte("not-json\n"))
	}()

	client := NewClient(sockPath)
	var result map[string]string
	err = client.Call("ping", nil, &result)
	if err == nil {
		t.Fatal("expected error for invalid response")
	}

	<-serverDone
}

// isRunningAt checks if a daemon is listening at a given socket path.
// Test-only helper since the production IsRunning() uses the hardcoded path.
func isRunningAt(socketPath string) bool {
	conn, err := net.DialTimeout("unix", socketPath, 1*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func TestIsRunning_NotRunning(t *testing.T) {
	dir := t.TempDir()
	sockPath := filepath.Join(dir, "test.sock")

	// Socket file doesn't exist
	if isRunningAt(sockPath) {
		t.Error("expected isRunningAt to return false when socket doesn't exist")
	}

	// Create socket file but no listener
	os.Create(sockPath)
	if isRunningAt(sockPath) {
		t.Error("expected isRunningAt to return false when no listener")
	}
}

func TestIsRunning_Running(t *testing.T) {
	dir := t.TempDir()
	sockPath := filepath.Join(dir, "test.sock")

	listener, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer listener.Close()

	// Accept connections in background
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()

	if !isRunningAt(sockPath) {
		t.Error("expected isRunningAt to return true when daemon is listening")
	}
}
