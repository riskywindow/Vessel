package daemon

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/vessel/vessel/internal/config"
	"github.com/vessel/vessel/internal/health"
	"github.com/vessel/vessel/internal/manager"
	"github.com/vessel/vessel/internal/network"
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
	manager          *manager.AppManager
	runtime          runtime.Runtime
	store            store.Store
	secretManager    *store.SecretManager
	network          network.NetworkManager
	healthMonitor    *health.HealthMonitor
	metricsCollector *health.MetricsCollector
	logger           *slog.Logger
}

// NewHandler creates a new request handler.
func NewHandler(mgr *manager.AppManager, rt runtime.Runtime, st store.Store, sm *store.SecretManager, net network.NetworkManager, hm *health.HealthMonitor, mc *health.MetricsCollector, logger *slog.Logger) *Handler {
	return &Handler{
		manager:          mgr,
		runtime:          rt,
		store:            st,
		secretManager:    sm,
		network:          net,
		healthMonitor:    hm,
		metricsCollector: mc,
		logger:           logger,
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
	case "apps.deploy_config":
		h.handleDeployAppConfig(ctx, conn, req.Params)
	case "apps.rollback":
		h.handleRollbackApp(ctx, conn, req.Params)
	case "apps.history":
		h.handleDeployHistory(ctx, conn, req.Params)
	case "containers.list":
		h.handleListContainers(ctx, conn, req.Params)
	case "containers.logs":
		h.handleContainerLogs(ctx, conn, req.Params)
	case "containers.stats":
		h.handleContainerStats(ctx, conn, req.Params)
	case "secrets.set":
		h.handleSecretSet(ctx, conn, req.Params)
	case "secrets.get":
		h.handleSecretGet(ctx, conn, req.Params)
	case "secrets.list":
		h.handleSecretList(ctx, conn)
	case "secrets.delete":
		h.handleSecretDelete(ctx, conn, req.Params)
	case "network.routes":
		h.handleNetworkRoutes(ctx, conn)
	case "health.status":
		h.handleHealthStatus(ctx, conn, req.Params)
	case "health.all":
		h.handleHealthAll(ctx, conn)
	case "health.check_now":
		h.handleHealthCheckNow(ctx, conn, req.Params)
	case "health.history":
		h.handleHealthHistory(ctx, conn, req.Params)
	case "metrics.get":
		h.handleMetricsGet(ctx, conn, req.Params)
	case "metrics.latest":
		h.handleMetricsLatest(ctx, conn, req.Params)
	case "cert.list":
		h.handleCertList(ctx, conn)
	case "cert.import":
		h.handleCertImport(ctx, conn, req.Params)
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

func (h *Handler) handleDeployAppConfig(ctx context.Context, conn net.Conn, params json.RawMessage) {
	var appCfg config.AppConfig
	if err := json.Unmarshal(params, &appCfg); err != nil {
		h.writeError(conn, "INVALID_PARAMS", err.Error())
		return
	}
	deploy, err := h.manager.DeployAppFromConfig(ctx, &appCfg)
	if err != nil {
		h.writeError(conn, "DEPLOY_FAILED", err.Error())
		return
	}
	h.writeData(conn, deploy)
}

func (h *Handler) handleRollbackApp(ctx context.Context, conn net.Conn, params json.RawMessage) {
	var p struct {
		Name    string `json:"name"`
		Version int    `json:"version"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		h.writeError(conn, "INVALID_PARAMS", err.Error())
		return
	}
	deploy, err := h.manager.RollbackApp(ctx, p.Name, p.Version)
	if err != nil {
		h.writeError(conn, "ROLLBACK_FAILED", err.Error())
		return
	}
	h.writeData(conn, deploy)
}

func (h *Handler) handleDeployHistory(ctx context.Context, conn net.Conn, params json.RawMessage) {
	var p struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		h.writeError(conn, "INVALID_PARAMS", err.Error())
		return
	}
	deploys, err := h.manager.GetDeployHistory(ctx, p.Name)
	if err != nil {
		h.writeError(conn, "HISTORY_FAILED", err.Error())
		return
	}
	h.writeData(conn, deploys)
}

func (h *Handler) handleListContainers(ctx context.Context, conn net.Conn, params json.RawMessage) {
	var p struct {
		AppName string `json:"app_name"`
	}
	// params may be nil for listing all containers
	if params != nil {
		json.Unmarshal(params, &p)
	}

	if p.AppName != "" {
		// List containers for a specific app
		containers, err := h.manager.GetContainers(ctx, p.AppName)
		if err != nil {
			h.writeError(conn, "LIST_FAILED", err.Error())
			return
		}
		h.writeData(conn, containers)
		return
	}

	// List all containers across all apps
	containers, err := h.manager.ListAllContainers(ctx)
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

func (h *Handler) handleSecretSet(ctx context.Context, conn net.Conn, params json.RawMessage) {
	var p struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		h.writeError(conn, "INVALID_PARAMS", err.Error())
		return
	}
	if h.secretManager == nil {
		h.writeError(conn, "SECRET_UNAVAILABLE", "secret manager not initialized")
		return
	}
	if err := h.secretManager.SetSecret(p.Key, p.Value); err != nil {
		h.writeError(conn, "SECRET_SET_FAILED", err.Error())
		return
	}
	h.writeData(conn, map[string]string{"status": "ok"})
}

func (h *Handler) handleSecretGet(ctx context.Context, conn net.Conn, params json.RawMessage) {
	var p struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		h.writeError(conn, "INVALID_PARAMS", err.Error())
		return
	}
	if h.secretManager == nil {
		h.writeError(conn, "SECRET_UNAVAILABLE", "secret manager not initialized")
		return
	}
	value, err := h.secretManager.GetSecret(p.Key)
	if err != nil {
		h.writeError(conn, "SECRET_GET_FAILED", err.Error())
		return
	}
	h.writeData(conn, map[string]string{"value": value})
}

func (h *Handler) handleSecretList(ctx context.Context, conn net.Conn) {
	if h.secretManager == nil {
		h.writeError(conn, "SECRET_UNAVAILABLE", "secret manager not initialized")
		return
	}
	keys, err := h.secretManager.ListSecretKeys()
	if err != nil {
		h.writeError(conn, "SECRET_LIST_FAILED", err.Error())
		return
	}

	// Return secret metadata (keys + timestamps, no values)
	var secrets []store.Secret
	for _, key := range keys {
		meta, err := h.secretManager.GetSecretMetadata(key)
		if err != nil {
			continue
		}
		secrets = append(secrets, store.Secret{
			Key:       meta.Key,
			CreatedAt: meta.CreatedAt,
			UpdatedAt: meta.UpdatedAt,
		})
	}
	h.writeData(conn, secrets)
}

func (h *Handler) handleSecretDelete(ctx context.Context, conn net.Conn, params json.RawMessage) {
	var p struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		h.writeError(conn, "INVALID_PARAMS", err.Error())
		return
	}
	if h.secretManager == nil {
		h.writeError(conn, "SECRET_UNAVAILABLE", "secret manager not initialized")
		return
	}
	if err := h.secretManager.DeleteSecret(p.Key); err != nil {
		h.writeError(conn, "SECRET_DELETE_FAILED", err.Error())
		return
	}
	h.writeData(conn, map[string]string{"status": "deleted"})
}

func (h *Handler) handleNetworkRoutes(ctx context.Context, conn net.Conn) {
	if h.network == nil {
		h.writeData(conn, map[string][]network.RouteTarget{})
		return
	}
	if nm, ok := h.network.(*network.LinuxNetworkManager); ok {
		h.writeData(conn, nm.GetRoutes())
		return
	}
	h.writeData(conn, map[string][]network.RouteTarget{})
}

func (h *Handler) handleHealthStatus(ctx context.Context, conn net.Conn, params json.RawMessage) {
	var p struct {
		ContainerID string `json:"container_id"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		h.writeError(conn, "INVALID_PARAMS", err.Error())
		return
	}

	if h.healthMonitor == nil {
		h.writeError(conn, "HEALTH_UNAVAILABLE", "health monitor not initialized")
		return
	}

	status, err := h.healthMonitor.GetStatus(p.ContainerID)
	if err != nil {
		h.writeError(conn, "HEALTH_FAILED", err.Error())
		return
	}
	if status == nil {
		h.writeError(conn, "NOT_MONITORED", "container not being monitored")
		return
	}
	h.writeData(conn, status)
}

func (h *Handler) handleHealthAll(ctx context.Context, conn net.Conn) {
	if h.healthMonitor == nil {
		h.writeData(conn, map[string]*health.ContainerHealth{})
		return
	}

	h.writeData(conn, h.healthMonitor.GetAllStatus())
}

func (h *Handler) handleHealthCheckNow(ctx context.Context, conn net.Conn, params json.RawMessage) {
	var p struct {
		ContainerID string `json:"container_id"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		h.writeError(conn, "INVALID_PARAMS", err.Error())
		return
	}

	if h.healthMonitor == nil {
		h.writeError(conn, "HEALTH_UNAVAILABLE", "health monitor not initialized")
		return
	}

	// Get the container's health check config from the monitor
	status, err := h.healthMonitor.GetStatus(p.ContainerID)
	if err != nil {
		h.writeError(conn, "HEALTH_FAILED", err.Error())
		return
	}
	if status == nil {
		h.writeError(conn, "NOT_MONITORED", "container not being monitored")
		return
	}

	// Execute an immediate health check
	start := time.Now()
	checkErr := health.ExecuteCheck(ctx, h.runtime, p.ContainerID, status.Check)

	result := store.HealthResult{
		ContainerID: p.ContainerID,
		CheckedAt:   start,
		Duration:    time.Since(start),
	}
	if checkErr != nil {
		result.Status = store.HealthStatusUnhealthy
		result.Message = checkErr.Error()
	} else {
		result.Status = store.HealthStatusHealthy
		result.Message = "healthy"
	}

	h.writeData(conn, result)
}

func (h *Handler) handleHealthHistory(ctx context.Context, conn net.Conn, params json.RawMessage) {
	var p struct {
		ContainerID string `json:"container_id"`
		Limit       int    `json:"limit"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		h.writeError(conn, "INVALID_PARAMS", err.Error())
		return
	}
	if p.Limit <= 0 {
		p.Limit = 20
	}

	if h.store == nil {
		h.writeError(conn, "STORE_UNAVAILABLE", "store not initialized")
		return
	}

	results, err := h.store.GetHealthResults(p.ContainerID, p.Limit)
	if err != nil {
		h.writeError(conn, "HEALTH_FAILED", err.Error())
		return
	}

	h.writeData(conn, results)
}

func (h *Handler) handleCertList(ctx context.Context, conn net.Conn) {
	result := struct {
		TLSEnabled bool     `json:"tls_enabled"`
		ACME       bool     `json:"acme"`
		Certs      []string `json:"certs"`
	}{}

	if h.network != nil {
		if nm, ok := h.network.(*network.LinuxNetworkManager); ok {
			result.TLSEnabled = nm.IsTLSEnabled()
			result.Certs = nm.GetCustomCerts()
		}
	}

	h.writeData(conn, result)
}

func (h *Handler) handleCertImport(ctx context.Context, conn net.Conn, params json.RawMessage) {
	var p struct {
		Hostname string `json:"hostname"`
		CertPath string `json:"cert_path"`
		KeyPath  string `json:"key_path"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		h.writeError(conn, "INVALID_PARAMS", err.Error())
		return
	}

	if h.network == nil {
		h.writeError(conn, "NETWORK_UNAVAILABLE", "network manager not initialized")
		return
	}

	nm, ok := h.network.(*network.LinuxNetworkManager)
	if !ok {
		h.writeError(conn, "NETWORK_UNAVAILABLE", "network manager does not support TLS")
		return
	}

	if err := nm.LoadCustomCert(p.Hostname, p.CertPath, p.KeyPath); err != nil {
		h.writeError(conn, "CERT_IMPORT_FAILED", err.Error())
		return
	}

	h.writeData(conn, map[string]string{"status": "imported"})
}

func (h *Handler) handleMetricsGet(ctx context.Context, conn net.Conn, params json.RawMessage) {
	var p struct {
		ContainerID string    `json:"container_id"`
		Start       time.Time `json:"start"`
		End         time.Time `json:"end"`
		Limit       int       `json:"limit"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		h.writeError(conn, "INVALID_PARAMS", err.Error())
		return
	}

	if h.metricsCollector == nil {
		h.writeError(conn, "METRICS_UNAVAILABLE", "metrics collector not initialized")
		return
	}

	metrics, err := h.metricsCollector.GetMetrics(p.ContainerID, p.Start, p.End, p.Limit)
	if err != nil {
		h.writeError(conn, "METRICS_FAILED", err.Error())
		return
	}
	h.writeData(conn, metrics)
}

func (h *Handler) handleMetricsLatest(ctx context.Context, conn net.Conn, params json.RawMessage) {
	var p struct {
		ContainerID string `json:"container_id"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		h.writeError(conn, "INVALID_PARAMS", err.Error())
		return
	}

	if h.metricsCollector == nil {
		h.writeError(conn, "METRICS_UNAVAILABLE", "metrics collector not initialized")
		return
	}

	metric, err := h.metricsCollector.GetLatestMetrics(p.ContainerID)
	if err != nil {
		h.writeError(conn, "METRICS_FAILED", err.Error())
		return
	}
	h.writeData(conn, metric)
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
