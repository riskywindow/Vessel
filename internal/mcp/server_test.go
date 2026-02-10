package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"
)

func TestRequestMarshal(t *testing.T) {
	req := Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "tools/list",
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	if !bytes.Contains(data, []byte(`"method":"tools/list"`)) {
		t.Errorf("missing method: %s", data)
	}
}

func TestRequestUnmarshal(t *testing.T) {
	input := `{"jsonrpc":"2.0","id":42,"method":"initialize","params":{"protocolVersion":"2024-11-05"}}`
	var req Request
	if err := json.Unmarshal([]byte(input), &req); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if req.Method != "initialize" {
		t.Errorf("expected initialize, got %s", req.Method)
	}
	if req.JSONRPC != "2.0" {
		t.Errorf("expected 2.0, got %s", req.JSONRPC)
	}
}

func TestResponseMarshal(t *testing.T) {
	resp := Response{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Result:  map[string]string{"status": "ok"},
	}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	if !bytes.Contains(data, []byte(`"result":`)) {
		t.Errorf("missing result: %s", data)
	}
}

func TestResponseErrorMarshal(t *testing.T) {
	resp := Response{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Error: &Error{
			Code:    MethodNotFound,
			Message: "Method not found",
			Data:    "unknown",
		},
	}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	if !bytes.Contains(data, []byte(`"error":`)) {
		t.Errorf("missing error: %s", data)
	}
	if !bytes.Contains(data, []byte(`-32601`)) {
		t.Errorf("missing error code: %s", data)
	}
}

func TestErrorCodes(t *testing.T) {
	tests := []struct {
		name     string
		code     int
		expected int
	}{
		{"ParseError", ParseError, -32700},
		{"InvalidRequest", InvalidRequest, -32600},
		{"MethodNotFound", MethodNotFound, -32601},
		{"InvalidParams", InvalidParams, -32602},
		{"InternalError", InternalError, -32603},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.code != tt.expected {
				t.Errorf("got %d, want %d", tt.code, tt.expected)
			}
		})
	}
}

func TestToolDefinitionCount(t *testing.T) {
	s := &Server{
		tools:     make(map[string]ToolHandler),
		resources: make(map[string]ResourceHandler),
	}
	defs := s.getToolDefinitions()
	if len(defs) != 12 {
		t.Errorf("expected 12 tools, got %d", len(defs))
	}
}

func TestToolDefinitionsValid(t *testing.T) {
	s := &Server{
		tools:     make(map[string]ToolHandler),
		resources: make(map[string]ResourceHandler),
	}
	for _, tool := range s.getToolDefinitions() {
		t.Run(tool.Name, func(t *testing.T) {
			if tool.Name == "" {
				t.Error("tool missing name")
			}
			if tool.Description == "" {
				t.Errorf("tool %s missing description", tool.Name)
			}
			if tool.InputSchema.Type != "object" {
				t.Errorf("tool %s schema type should be object, got %s", tool.Name, tool.InputSchema.Type)
			}
		})
	}
}

func TestToolDefinitionsHaveRequiredFields(t *testing.T) {
	s := &Server{
		tools:     make(map[string]ToolHandler),
		resources: make(map[string]ResourceHandler),
	}
	requireName := map[string]bool{
		"vessel_get_app":        true,
		"vessel_deploy":         true,
		"vessel_stop":           true,
		"vessel_restart":        true,
		"vessel_remove":         true,
		"vessel_logs":           true,
		"vessel_metrics":        true,
		"vessel_deploy_history": true,
	}
	for _, tool := range s.getToolDefinitions() {
		if requireName[tool.Name] {
			found := false
			for _, r := range tool.InputSchema.Required {
				if r == "name" {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("tool %s should require 'name'", tool.Name)
			}
		}
	}
}

func TestInitializeResult(t *testing.T) {
	result := InitializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities: ServerCapabilities{
			Tools:     &ToolsCapability{},
			Resources: &ResourcesCapability{},
		},
		ServerInfo: ServerInfo{Name: "vessel", Version: "0.1.0"},
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	if !strings.Contains(string(data), "2024-11-05") {
		t.Error("missing protocol version")
	}
	if !strings.Contains(string(data), "vessel") {
		t.Error("missing server name")
	}
}

func TestToolResultFormat(t *testing.T) {
	result := ToolResult{
		Content: []ContentBlock{{Type: "text", Text: "hello world"}},
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	if !strings.Contains(string(data), "hello world") {
		t.Error("missing content text")
	}
	if !strings.Contains(string(data), `"type":"text"`) {
		t.Error("missing content type")
	}
}

func TestToolResultErrorFormat(t *testing.T) {
	result := ToolResult{
		Content: []ContentBlock{{Type: "text", Text: "Error: something failed"}},
		IsError: true,
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	if !strings.Contains(string(data), `"isError":true`) {
		t.Error("missing isError flag")
	}
}

func TestResourceDefinitions(t *testing.T) {
	resources := []Resource{
		{URI: "vessel://apps", Name: "Applications", MimeType: "application/json"},
		{URI: "vessel://health", Name: "Health Status", MimeType: "application/json"},
		{URI: "vessel://system", Name: "System Info", MimeType: "application/json"},
	}
	for _, r := range resources {
		if r.URI == "" {
			t.Error("resource missing URI")
		}
		if r.Name == "" {
			t.Error("resource missing name")
		}
	}
}

// newTestServer creates an MCP server with custom IO for testing.
// It registers test tools that don't need a daemon connection.
func newTestServer(input string) (*Server, *bytes.Buffer) {
	writer := &bytes.Buffer{}
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))

	s := &Server{
		client:    nil,
		logger:    logger,
		tools:     make(map[string]ToolHandler),
		resources: make(map[string]ResourceHandler),
		reader:    bufio.NewReader(strings.NewReader(input)),
		writer:    writer,
	}

	// Register test tools that don't need daemon
	s.tools["test_echo"] = func(ctx context.Context, args json.RawMessage) (interface{}, error) {
		var params map[string]string
		json.Unmarshal(args, &params)
		return params, nil
	}
	s.tools["test_error"] = func(ctx context.Context, args json.RawMessage) (interface{}, error) {
		return nil, fmt.Errorf("test error occurred")
	}

	s.resources["vessel://test"] = func(ctx context.Context) (string, error) {
		return `{"test": true}`, nil
	}

	return s, writer
}

func TestServerInitialize(t *testing.T) {
	input := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}` + "\n"
	s, output := newTestServer(input)

	err := s.Run(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var resp Response
	if err := json.Unmarshal(output.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v (raw: %s)", err, output.String())
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
	if resp.JSONRPC != "2.0" {
		t.Errorf("expected jsonrpc 2.0, got %s", resp.JSONRPC)
	}

	resultJSON, _ := json.Marshal(resp.Result)
	if !strings.Contains(string(resultJSON), "2024-11-05") {
		t.Errorf("missing protocol version in result: %s", resultJSON)
	}
}

func TestServerPing(t *testing.T) {
	input := `{"jsonrpc":"2.0","id":1,"method":"ping"}` + "\n"
	s, output := newTestServer(input)

	s.Run(context.Background())

	var resp Response
	if err := json.Unmarshal(output.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
}

func TestServerMethodNotFound(t *testing.T) {
	input := `{"jsonrpc":"2.0","id":1,"method":"nonexistent"}` + "\n"
	s, output := newTestServer(input)

	s.Run(context.Background())

	var resp Response
	if err := json.Unmarshal(output.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected error response")
	}
	if resp.Error.Code != MethodNotFound {
		t.Errorf("expected code %d, got %d", MethodNotFound, resp.Error.Code)
	}
}

func TestServerToolsCall(t *testing.T) {
	input := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"test_echo","arguments":{"msg":"hello"}}}` + "\n"
	s, output := newTestServer(input)

	s.Run(context.Background())

	var resp Response
	if err := json.Unmarshal(output.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	resultJSON, _ := json.Marshal(resp.Result)
	if !strings.Contains(string(resultJSON), "hello") {
		t.Errorf("expected echo content, got: %s", resultJSON)
	}
}

func TestServerToolsCallError(t *testing.T) {
	input := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"test_error","arguments":{}}}` + "\n"
	s, output := newTestServer(input)

	s.Run(context.Background())

	var resp Response
	if err := json.Unmarshal(output.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	// Tool errors come back as result with isError, not as JSON-RPC error
	if resp.Error != nil {
		t.Fatalf("tool errors should not be JSON-RPC errors: %v", resp.Error)
	}

	resultJSON, _ := json.Marshal(resp.Result)
	if !strings.Contains(string(resultJSON), "isError") {
		t.Errorf("expected isError in result: %s", resultJSON)
	}
	if !strings.Contains(string(resultJSON), "test error occurred") {
		t.Errorf("expected error message in result: %s", resultJSON)
	}
}

func TestServerToolsCallUnknown(t *testing.T) {
	input := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"nonexistent","arguments":{}}}` + "\n"
	s, output := newTestServer(input)

	s.Run(context.Background())

	var resp Response
	json.Unmarshal(output.Bytes(), &resp)
	if resp.Error == nil {
		t.Fatal("expected error for unknown tool")
	}
	if resp.Error.Code != MethodNotFound {
		t.Errorf("expected code %d, got %d", MethodNotFound, resp.Error.Code)
	}
}

func TestServerResourcesRead(t *testing.T) {
	input := `{"jsonrpc":"2.0","id":1,"method":"resources/read","params":{"uri":"vessel://test"}}` + "\n"
	s, output := newTestServer(input)

	s.Run(context.Background())

	var resp Response
	if err := json.Unmarshal(output.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	resultJSON, _ := json.Marshal(resp.Result)
	if !strings.Contains(string(resultJSON), "vessel://test") {
		t.Errorf("expected resource URI in result: %s", resultJSON)
	}
}

func TestServerResourcesReadUnknown(t *testing.T) {
	input := `{"jsonrpc":"2.0","id":1,"method":"resources/read","params":{"uri":"vessel://nonexistent"}}` + "\n"
	s, output := newTestServer(input)

	s.Run(context.Background())

	var resp Response
	json.Unmarshal(output.Bytes(), &resp)
	if resp.Error == nil {
		t.Fatal("expected error for unknown resource")
	}
}

func TestServerResourcesList(t *testing.T) {
	input := `{"jsonrpc":"2.0","id":1,"method":"resources/list"}` + "\n"
	s, output := newTestServer(input)

	s.Run(context.Background())

	var resp Response
	if err := json.Unmarshal(output.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	resultJSON, _ := json.Marshal(resp.Result)
	if !strings.Contains(string(resultJSON), "vessel://apps") {
		t.Errorf("expected apps resource: %s", resultJSON)
	}
}

func TestServerToolsList(t *testing.T) {
	input := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}` + "\n"
	s, output := newTestServer(input)
	s.registerTools()

	s.Run(context.Background())

	var resp Response
	if err := json.Unmarshal(output.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	resultJSON, _ := json.Marshal(resp.Result)
	if !strings.Contains(string(resultJSON), "vessel_list_apps") {
		t.Errorf("expected vessel_list_apps tool: %s", resultJSON)
	}
}

func TestServerParseError(t *testing.T) {
	input := "not json\n"
	s, output := newTestServer(input)

	s.Run(context.Background())

	var resp Response
	if err := json.Unmarshal(output.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Error == nil {
		t.Fatal("expected parse error")
	}
	if resp.Error.Code != ParseError {
		t.Errorf("expected code %d, got %d", ParseError, resp.Error.Code)
	}
}

func TestServerMultipleRequests(t *testing.T) {
	input := `{"jsonrpc":"2.0","id":1,"method":"ping"}` + "\n" +
		`{"jsonrpc":"2.0","id":2,"method":"ping"}` + "\n"
	s, output := newTestServer(input)

	s.Run(context.Background())

	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 responses, got %d: %s", len(lines), output.String())
	}

	for i, line := range lines {
		var resp Response
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			t.Fatalf("response %d parse failed: %v", i, err)
		}
		if resp.Error != nil {
			t.Errorf("response %d unexpected error: %v", i, resp.Error)
		}
	}
}

func TestServerInitializedNotification(t *testing.T) {
	input := `{"jsonrpc":"2.0","method":"initialized"}` + "\n"
	s, output := newTestServer(input)

	s.Run(context.Background())

	if output.Len() > 0 {
		t.Errorf("expected no response for initialized notification, got: %s", output.String())
	}
	if !s.initialized {
		t.Error("server should be marked as initialized")
	}
}

func TestServerSkipsEmptyLines(t *testing.T) {
	input := "\n\n" + `{"jsonrpc":"2.0","id":1,"method":"ping"}` + "\n"
	s, output := newTestServer(input)

	s.Run(context.Background())

	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 response, got %d", len(lines))
	}
}

func TestContentBlockJSON(t *testing.T) {
	block := ContentBlock{Type: "text", Text: "test content"}
	data, err := json.Marshal(block)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	if !strings.Contains(string(data), `"type":"text"`) {
		t.Error("missing type field")
	}
	if !strings.Contains(string(data), `"text":"test content"`) {
		t.Error("missing text field")
	}
}

func TestServerInfoJSON(t *testing.T) {
	info := ServerInfo{Name: "vessel", Version: "0.1.0"}
	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	if !strings.Contains(string(data), `"name":"vessel"`) {
		t.Error("missing name")
	}
	if !strings.Contains(string(data), `"version":"0.1.0"`) {
		t.Error("missing version")
	}
}

func TestResourceContentsJSON(t *testing.T) {
	rc := ResourceContents{
		URI:      "vessel://apps",
		MimeType: "application/json",
		Text:     `[{"name":"web"}]`,
	}
	data, err := json.Marshal(rc)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	if !strings.Contains(string(data), "vessel://apps") {
		t.Error("missing URI")
	}
}

func TestToolCallInvalidParams(t *testing.T) {
	input := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":"bad"}` + "\n"
	s, output := newTestServer(input)

	s.Run(context.Background())

	var resp Response
	json.Unmarshal(output.Bytes(), &resp)
	if resp.Error == nil {
		t.Fatal("expected error for invalid params")
	}
	if resp.Error.Code != InvalidParams {
		t.Errorf("expected code %d, got %d", InvalidParams, resp.Error.Code)
	}
}

func TestResourceReadInvalidParams(t *testing.T) {
	input := `{"jsonrpc":"2.0","id":1,"method":"resources/read","params":"bad"}` + "\n"
	s, output := newTestServer(input)

	s.Run(context.Background())

	var resp Response
	json.Unmarshal(output.Bytes(), &resp)
	if resp.Error == nil {
		t.Fatal("expected error for invalid params")
	}
	if resp.Error.Code != InvalidParams {
		t.Errorf("expected code %d, got %d", InvalidParams, resp.Error.Code)
	}
}

func TestToolRegistration(t *testing.T) {
	s := &Server{
		tools:     make(map[string]ToolHandler),
		resources: make(map[string]ResourceHandler),
		logger:    slog.New(slog.NewJSONHandler(io.Discard, nil)),
	}
	s.registerTools()

	expectedTools := []string{
		"vessel_list_apps",
		"vessel_get_app",
		"vessel_health",
		"vessel_logs",
		"vessel_metrics",
		"vessel_system_info",
		"vessel_deploy_history",
		"vessel_deploy",
		"vessel_stop",
		"vessel_restart",
		"vessel_remove",
		"vessel_rollback",
	}
	for _, name := range expectedTools {
		if _, ok := s.tools[name]; !ok {
			t.Errorf("tool %s not registered", name)
		}
	}
}

func TestResourceRegistration(t *testing.T) {
	s := &Server{
		tools:     make(map[string]ToolHandler),
		resources: make(map[string]ResourceHandler),
		logger:    slog.New(slog.NewJSONHandler(io.Discard, nil)),
	}
	s.registerResources()

	expectedResources := []string{
		"vessel://apps",
		"vessel://health",
		"vessel://system",
	}
	for _, uri := range expectedResources {
		if _, ok := s.resources[uri]; !ok {
			t.Errorf("resource %s not registered", uri)
		}
	}
}

func TestNotImplementedTools(t *testing.T) {
	s := &Server{
		tools:     make(map[string]ToolHandler),
		resources: make(map[string]ResourceHandler),
		logger:    slog.New(slog.NewJSONHandler(io.Discard, nil)),
	}
	s.registerTools()

	stubs := []string{"vessel_deploy", "vessel_stop", "vessel_restart", "vessel_remove", "vessel_rollback"}
	for _, name := range stubs {
		t.Run(name, func(t *testing.T) {
			handler := s.tools[name]
			_, err := handler(context.Background(), json.RawMessage(`{}`))
			if err == nil {
				t.Errorf("expected error from stub %s", name)
			}
			if !strings.Contains(err.Error(), "Session 2") {
				t.Errorf("expected 'Session 2' in error, got: %s", err.Error())
			}
		})
	}
}

func TestInitializeHandshake(t *testing.T) {
	input := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"claude-desktop","version":"1.0"}}}` + "\n" +
		`{"jsonrpc":"2.0","method":"initialized"}` + "\n" +
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}` + "\n"
	s, output := newTestServer(input)
	s.registerTools()

	s.Run(context.Background())

	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	// Should have 2 responses (initialize result + tools/list result)
	// "initialized" is a notification, no response
	if len(lines) != 2 {
		t.Fatalf("expected 2 responses, got %d: %s", len(lines), output.String())
	}

	// First response: initialize
	var initResp Response
	json.Unmarshal([]byte(lines[0]), &initResp)
	if initResp.Error != nil {
		t.Fatalf("initialize error: %v", initResp.Error)
	}

	// Second response: tools/list
	var toolsResp Response
	json.Unmarshal([]byte(lines[1]), &toolsResp)
	if toolsResp.Error != nil {
		t.Fatalf("tools/list error: %v", toolsResp.Error)
	}

	if !s.initialized {
		t.Error("server should be initialized after handshake")
	}
}
