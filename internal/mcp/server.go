package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"

	"github.com/vessel/vessel/internal/daemon"
)

// Server is the MCP server for Vessel.
type Server struct {
	client    *daemon.Client
	logger    *slog.Logger
	tools     map[string]ToolHandler
	resources map[string]ResourceHandler

	reader *bufio.Reader
	writer io.Writer
	mu     sync.Mutex

	initialized bool
}

// ToolHandler handles a tool call.
type ToolHandler func(ctx context.Context, args json.RawMessage) (interface{}, error)

// ResourceHandler handles a resource read.
type ResourceHandler func(ctx context.Context) (string, error)

// NewServer creates a new MCP server that communicates over stdin/stdout.
func NewServer(socketPath string, logger *slog.Logger) *Server {
	client := daemon.NewClient(socketPath)

	s := &Server{
		client:    client,
		logger:    logger,
		tools:     make(map[string]ToolHandler),
		resources: make(map[string]ResourceHandler),
		reader:    bufio.NewReader(os.Stdin),
		writer:    os.Stdout,
	}

	s.registerTools()
	s.registerResources()

	return s
}

// NewServerWithIO creates a new MCP server with custom IO (for testing).
func NewServerWithIO(socketPath string, reader io.Reader, writer io.Writer, logger *slog.Logger) *Server {
	client := daemon.NewClient(socketPath)

	s := &Server{
		client:    client,
		logger:    logger,
		tools:     make(map[string]ToolHandler),
		resources: make(map[string]ResourceHandler),
		reader:    bufio.NewReader(reader),
		writer:    writer,
	}

	s.registerTools()
	s.registerResources()

	return s
}

// Run starts the MCP server loop, reading JSON-RPC messages from stdin.
func (s *Server) Run(ctx context.Context) error {
	s.logger.Info("starting MCP server")

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line, err := s.reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("failed to read: %w", err)
		}

		// Skip empty lines
		if len(line) <= 1 {
			continue
		}

		var req Request
		if err := json.Unmarshal(line, &req); err != nil {
			s.sendError(nil, ParseError, "Parse error", err.Error())
			continue
		}

		s.handleRequest(ctx, &req)
	}
}

func (s *Server) handleRequest(ctx context.Context, req *Request) {
	s.logger.Debug("handling request", "method", req.Method)

	switch req.Method {
	case "initialize":
		s.handleInitialize(req)
	case "initialized":
		s.initialized = true
	case "tools/list":
		s.handleToolsList(req)
	case "tools/call":
		s.handleToolsCall(ctx, req)
	case "resources/list":
		s.handleResourcesList(req)
	case "resources/read":
		s.handleResourcesRead(ctx, req)
	case "ping":
		s.sendResult(req.ID, map[string]string{})
	default:
		s.sendError(req.ID, MethodNotFound, "Method not found", req.Method)
	}
}

func (s *Server) handleInitialize(req *Request) {
	result := InitializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities: ServerCapabilities{
			Tools:     &ToolsCapability{},
			Resources: &ResourcesCapability{},
		},
		ServerInfo: ServerInfo{
			Name:    "vessel",
			Version: "0.1.0",
		},
	}
	s.sendResult(req.ID, result)
}

func (s *Server) handleToolsList(req *Request) {
	tools := s.getToolDefinitions()
	s.sendResult(req.ID, map[string]interface{}{"tools": tools})
}

func (s *Server) handleToolsCall(ctx context.Context, req *Request) {
	var call ToolCall
	if err := json.Unmarshal(req.Params, &call); err != nil {
		s.sendError(req.ID, InvalidParams, "Invalid params", err.Error())
		return
	}

	handler, ok := s.tools[call.Name]
	if !ok {
		s.sendError(req.ID, MethodNotFound, "Unknown tool", call.Name)
		return
	}

	result, err := handler(ctx, call.Arguments)
	if err != nil {
		s.sendResult(req.ID, ToolResult{
			Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("Error: %v", err)}},
			IsError: true,
		})
		return
	}

	// Format result as JSON text
	text, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		text = []byte(fmt.Sprintf("%v", result))
	}

	s.sendResult(req.ID, ToolResult{
		Content: []ContentBlock{{Type: "text", Text: string(text)}},
	})
}

func (s *Server) handleResourcesList(req *Request) {
	resources := []Resource{
		{
			URI:         "vessel://apps",
			Name:        "Applications",
			Description: "List of all deployed applications",
			MimeType:    "application/json",
		},
		{
			URI:         "vessel://health",
			Name:        "Health Status",
			Description: "Health status of all containers",
			MimeType:    "application/json",
		},
		{
			URI:         "vessel://system",
			Name:        "System Info",
			Description: "Vessel system information",
			MimeType:    "application/json",
		},
	}
	s.sendResult(req.ID, map[string]interface{}{"resources": resources})
}

func (s *Server) handleResourcesRead(ctx context.Context, req *Request) {
	var params struct {
		URI string `json:"uri"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		s.sendError(req.ID, InvalidParams, "Invalid params", err.Error())
		return
	}

	handler, ok := s.resources[params.URI]
	if !ok {
		s.sendError(req.ID, InvalidParams, "Unknown resource", params.URI)
		return
	}

	content, err := handler(ctx)
	if err != nil {
		s.sendError(req.ID, InternalError, "Resource error", err.Error())
		return
	}

	s.sendResult(req.ID, map[string]interface{}{
		"contents": []ResourceContents{{
			URI:      params.URI,
			MimeType: "application/json",
			Text:     content,
		}},
	})
}

func (s *Server) sendResult(id json.RawMessage, result interface{}) {
	resp := Response{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	s.send(resp)
}

func (s *Server) sendError(id json.RawMessage, code int, message, data string) {
	resp := Response{
		JSONRPC: "2.0",
		ID:      id,
		Error: &Error{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
	s.send(resp)
}

func (s *Server) send(resp Response) {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.Marshal(resp)
	if err != nil {
		s.logger.Error("failed to marshal response", "error", err)
		return
	}

	s.writer.Write(data)
	s.writer.Write([]byte("\n"))
}
