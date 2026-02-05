package daemon

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/vessel/vessel/internal/manager"
	"github.com/vessel/vessel/internal/runtime"
	"github.com/vessel/vessel/internal/store"
)

// Request is the message format from CLI to daemon.
type Request struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

// Response is the message format from daemon to CLI.
type Response struct {
	Data  json.RawMessage `json:"data,omitempty"`
	Error *ErrorResponse  `json:"error,omitempty"`
}

// ErrorResponse describes an error in the response.
type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Handler dispatches requests to the appropriate manager methods.
type Handler struct {
	manager *manager.AppManager
	runtime *runtime.LinuxRuntime
	logger  *slog.Logger
}

// NewHandler creates a new request handler.
func NewHandler(mgr *manager.AppManager, rt *runtime.LinuxRuntime, logger *slog.Logger) *Handler {
	return &Handler{
		manager: mgr,
		runtime: rt,
		logger:  logger,
	}
}

// Handle reads a request from the connection and writes a response.
func (h *Handler) Handle(ctx context.Context, conn net.Conn) {
	conn.SetDeadline(time.Now().Add(60 * time.Second))

	reader := bufio.NewReader(conn)
	var req Request
	decoder := json.NewDecoder(reader)
	if err := decoder.Decode(&req); err != nil {
		h.writeError(conn, "INVALID_REQUEST", fmt.Sprintf("failed to decode request: %v", err))
		return
	}

	h.logger.Debug("handling request", "method", req.Method)

	switch req.Method {
	case "apps.list":
		h.handleListApps(ctx, conn)
	case "apps.get":
		h.handleGetApp(ctx, conn, req.Params)
	case "apps.stop":
		h.handleStopApp(ctx, conn, req.Params)
	case "apps.remove":
		h.handleRemoveApp(ctx, conn, req.Params)
	case "apps.restart":
		h.handleRestartApp(ctx, conn, req.Params)
	case "apps.deploy":
		h.handleDeployApp(ctx, conn, req.Params)
	case "containers.list":
		h.handleListContainers(ctx, conn, req.Params)
	case "containers.logs":
		h.handleContainerLogs(ctx, conn, req.Params)
	case "containers.stats":
		h.handleContainerStats(ctx, conn, req.Params)
	case "ping":
		h.writeData(conn, map[string]string{"status": "ok"})
	default:
		h.writeError(conn, "UNKNOWN_METHOD", fmt.Sprintf("unknown method: %s", req.Method))
	}
}

func (h *Handler) handleListApps(ctx context.Context, conn net.Conn) {
	apps, err := h.manager.ListApps(ctx)
	if err != nil {
		h.writeError(conn, "INTERNAL_ERROR", err.Error())
		return
	}
	h.writeData(conn, apps)
}

func (h *Handler) handleGetApp(ctx context.Context, conn net.Conn, params json.RawMessage) {
	var p struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		h.writeError(conn, "INVALID_PARAMS", err.Error())
		return
	}
	app, err := h.manager.GetApp(ctx, p.Name)
	if err != nil {
		h.writeError(conn, "APP_NOT_FOUND", err.Error())
		return
	}
	h.writeData(conn, app)
}

func (h *Handler) handleStopApp(ctx context.Context, conn net.Conn, params json.RawMessage) {
	var p struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		h.writeError(conn, "INVALID_PARAMS", err.Error())
		return
	}
	if err := h.manager.StopApp(ctx, p.Name); err != nil {
		h.writeError(conn, "STOP_FAILED", err.Error())
		return
	}
	h.writeData(conn, map[string]string{"status": "stopped"})
}

func (h *Handler) handleRemoveApp(ctx context.Context, conn net.Conn, params json.RawMessage) {
	var p struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		h.writeError(conn, "INVALID_PARAMS", err.Error())
		return
	}
	if err := h.manager.RemoveApp(ctx, p.Name); err != nil {
		h.writeError(conn, "REMOVE_FAILED", err.Error())
		return
	}
	h.writeData(conn, map[string]string{"status": "removed"})
}

func (h *Handler) handleRestartApp(ctx context.Context, conn net.Conn, params json.RawMessage) {
	var p struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		h.writeError(conn, "INVALID_PARAMS", err.Error())
		return
	}
	if err := h.manager.RestartApp(ctx, p.Name); err != nil {
		h.writeError(conn, "RESTART_FAILED", err.Error())
		return
	}
	h.writeData(conn, map[string]string{"status": "restarted"})
}

func (h *Handler) handleDeployApp(ctx context.Context, conn net.Conn, params json.RawMessage) {
	var p struct {
		Name      string               `json:"name"`
		Image     string               `json:"image"`
		Instances int                  `json:"instances"`
		Env       map[string]string    `json:"env"`
		Resources store.ResourceLimits `json:"resources"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		h.writeError(conn, "INVALID_PARAMS", err.Error())
		return
	}
	if p.Instances <= 0 {
		p.Instances = 1
	}
	deploy, err := h.manager.DeployApp(ctx, p.Name, p.Image, p.Instances, p.Env, p.Resources)
	if err != nil {
		h.writeError(conn, "DEPLOY_FAILED", err.Error())
		return
	}
	h.writeData(conn, deploy)
}

func (h *Handler) handleListContainers(ctx context.Context, conn net.Conn, params json.RawMessage) {
	var p struct {
		AppName string `json:"app_name"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		h.writeError(conn, "INVALID_PARAMS", err.Error())
		return
	}
	containers, err := h.manager.GetContainers(ctx, p.AppName)
	if err != nil {
		h.writeError(conn, "LIST_FAILED", err.Error())
		return
	}
	h.writeData(conn, containers)
}

func (h *Handler) handleContainerLogs(ctx context.Context, conn net.Conn, params json.RawMessage) {
	var p struct {
		ContainerID string `json:"container_id"`
		Follow      bool   `json:"follow"`
		Tail        int    `json:"tail"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		h.writeError(conn, "INVALID_PARAMS", err.Error())
		return
	}
	if p.Tail <= 0 {
		p.Tail = 100
	}

	reader, err := h.runtime.ContainerLogs(ctx, p.ContainerID, false, p.Tail)
	if err != nil {
		h.writeError(conn, "LOGS_FAILED", err.Error())
		return
	}
	defer reader.Close()

	buf := make([]byte, 4096)
	var logs []byte
	for {
		n, readErr := reader.Read(buf)
		if n > 0 {
			logs = append(logs, buf[:n]...)
		}
		if readErr != nil {
			break
		}
	}

	h.writeData(conn, map[string]string{"logs": string(logs)})
}

func (h *Handler) handleContainerStats(ctx context.Context, conn net.Conn, params json.RawMessage) {
	var p struct {
		ContainerID string `json:"container_id"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		h.writeError(conn, "INVALID_PARAMS", err.Error())
		return
	}
	stats, err := h.runtime.ContainerStats(ctx, p.ContainerID)
	if err != nil {
		h.writeError(conn, "STATS_FAILED", err.Error())
		return
	}
	h.writeData(conn, stats)
}

// writeData sends a successful response.
func (h *Handler) writeData(conn net.Conn, data interface{}) {
	dataJSON, err := json.Marshal(data)
	if err != nil {
		h.writeError(conn, "MARSHAL_ERROR", err.Error())
		return
	}

	resp := Response{Data: dataJSON}
	encoder := json.NewEncoder(conn)
	if err := encoder.Encode(resp); err != nil {
		h.logger.Error("failed to write response", "error", err)
	}
}

// writeError sends an error response.
func (h *Handler) writeError(conn net.Conn, code string, message string) {
	resp := Response{
		Error: &ErrorResponse{
			Code:    code,
			Message: message,
		},
	}
	encoder := json.NewEncoder(conn)
	if err := encoder.Encode(resp); err != nil {
		h.logger.Error("failed to write error response", "error", err)
	}
}
