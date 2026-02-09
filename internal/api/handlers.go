package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/vessel/vessel/internal/config"
	"github.com/vessel/vessel/internal/health"
	"github.com/vessel/vessel/internal/manager"
	"github.com/vessel/vessel/internal/network"
	"github.com/vessel/vessel/internal/runtime"
	"github.com/vessel/vessel/internal/store"
)

// Handler contains all API endpoint handlers.
type Handler struct {
	manager          *manager.AppManager
	runtime          runtime.Runtime
	store            store.Store
	secretManager    *store.SecretManager
	network          network.NetworkManager
	healthMonitor    *health.HealthMonitor
	metricsCollector *health.MetricsCollector
}

// NewHandler creates a new API handler.
func NewHandler(
	mgr *manager.AppManager,
	rt runtime.Runtime,
	st store.Store,
	sm *store.SecretManager,
	net network.NetworkManager,
	hm *health.HealthMonitor,
	mc *health.MetricsCollector,
) *Handler {
	return &Handler{
		manager:          mgr,
		runtime:          rt,
		store:            st,
		secretManager:    sm,
		network:          net,
		healthMonitor:    hm,
		metricsCollector: mc,
	}
}

// Ping returns server status.
func (h *Handler) Ping(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ListApps returns all apps.
func (h *Handler) ListApps(w http.ResponseWriter, r *http.Request) {
	apps, err := h.manager.ListApps(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, apps)
}

// GetApp returns a single app by name.
func (h *Handler) GetApp(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	app, err := h.manager.GetApp(r.Context(), name)
	if err != nil {
		writeError(w, http.StatusNotFound, "APP_NOT_FOUND", err.Error())
		return
	}

	// Include containers for app detail view
	containers, _ := h.manager.GetContainers(r.Context(), name)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"app":        app,
		"containers": containers,
	})
}

// StopApp stops an app.
func (h *Handler) StopApp(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if err := h.manager.StopApp(r.Context(), name); err != nil {
		writeError(w, http.StatusInternalServerError, "STOP_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "stopped"})
}

// RemoveApp removes an app.
func (h *Handler) RemoveApp(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if err := h.manager.RemoveApp(r.Context(), name); err != nil {
		writeError(w, http.StatusInternalServerError, "REMOVE_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "removed"})
}

// RestartApp restarts an app.
func (h *Handler) RestartApp(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if err := h.manager.RestartApp(r.Context(), name); err != nil {
		writeError(w, http.StatusInternalServerError, "RESTART_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "restarted"})
}

// DeployApp deploys or updates an app.
func (h *Handler) DeployApp(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	var appCfg config.AppConfig
	if err := json.NewDecoder(r.Body).Decode(&appCfg); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	appCfg.Name = name

	deploy, err := h.manager.DeployAppFromConfig(r.Context(), &appCfg)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DEPLOY_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, deploy)
}

// RollbackApp rolls back to a previous version.
func (h *Handler) RollbackApp(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	var req struct {
		Version int `json:"version"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	deploy, err := h.manager.RollbackApp(r.Context(), name, req.Version)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "ROLLBACK_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, deploy)
}

// GetDeployHistory returns deploy history for an app.
func (h *Handler) GetDeployHistory(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	deploys, err := h.manager.GetDeployHistory(r.Context(), name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "HISTORY_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, deploys)
}

// ListContainers returns all containers, optionally filtered by app.
func (h *Handler) ListContainers(w http.ResponseWriter, r *http.Request) {
	appName := r.URL.Query().Get("app")

	if appName != "" {
		containers, err := h.manager.GetContainers(r.Context(), appName)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "LIST_FAILED", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, containers)
		return
	}

	containers, err := h.store.ListContainers()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, containers)
}

// GetContainer returns a single container.
func (h *Handler) GetContainer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	container, err := h.store.GetContainer(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "CONTAINER_NOT_FOUND", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, container)
}

// GetContainerLogs returns container logs (non-streaming).
func (h *Handler) GetContainerLogs(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	tail, _ := strconv.Atoi(r.URL.Query().Get("tail"))
	if tail <= 0 {
		tail = 100
	}

	logs, err := h.runtime.ContainerLogs(r.Context(), id, false, tail)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "LOGS_FAILED", err.Error())
		return
	}
	defer logs.Close()

	buf := make([]byte, 1024*1024) // 1MB max
	n, _ := logs.Read(buf)

	writeJSON(w, http.StatusOK, map[string]string{
		"logs": string(buf[:n]),
	})
}

// GetContainerStats returns current container resource stats.
func (h *Handler) GetContainerStats(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	stats, err := h.runtime.ContainerStats(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "STATS_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

// GetAllHealth returns health status for all containers.
func (h *Handler) GetAllHealth(w http.ResponseWriter, r *http.Request) {
	if h.healthMonitor == nil {
		writeJSON(w, http.StatusOK, map[string]*health.ContainerHealth{})
		return
	}
	writeJSON(w, http.StatusOK, h.healthMonitor.GetAllStatus())
}

// GetHealthStatus returns health status for a container.
func (h *Handler) GetHealthStatus(w http.ResponseWriter, r *http.Request) {
	containerId := chi.URLParam(r, "containerId")
	if h.healthMonitor == nil {
		writeError(w, http.StatusServiceUnavailable, "HEALTH_UNAVAILABLE", "Health monitor not initialized")
		return
	}

	status, err := h.healthMonitor.GetStatus(containerId)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, status)
}

// CheckHealthNow runs an immediate health check on a container.
func (h *Handler) CheckHealthNow(w http.ResponseWriter, r *http.Request) {
	containerId := chi.URLParam(r, "containerId")
	if h.healthMonitor == nil {
		writeError(w, http.StatusServiceUnavailable, "HEALTH_UNAVAILABLE", "Health monitor not initialized")
		return
	}

	status, err := h.healthMonitor.GetStatus(containerId)
	if err != nil {
		writeError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}

	start := time.Now()
	checkErr := health.ExecuteCheck(r.Context(), h.runtime, containerId, status.Check)

	result := store.HealthResult{
		ContainerID: containerId,
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

	writeJSON(w, http.StatusOK, result)
}

// GetHealthHistory returns health check history for a container.
func (h *Handler) GetHealthHistory(w http.ResponseWriter, r *http.Request) {
	containerId := chi.URLParam(r, "containerId")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 20
	}

	results, err := h.store.GetHealthResults(containerId, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "HISTORY_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, results)
}

// GetMetrics returns metrics for a container within a time range.
func (h *Handler) GetMetrics(w http.ResponseWriter, r *http.Request) {
	containerId := chi.URLParam(r, "containerId")

	start := time.Time{}
	end := time.Now()
	limit := 100

	if s := r.URL.Query().Get("start"); s != "" {
		start, _ = time.Parse(time.RFC3339, s)
	}
	if e := r.URL.Query().Get("end"); e != "" {
		end, _ = time.Parse(time.RFC3339, e)
	}
	if l := r.URL.Query().Get("limit"); l != "" {
		limit, _ = strconv.Atoi(l)
	}

	if h.metricsCollector == nil {
		writeError(w, http.StatusServiceUnavailable, "METRICS_UNAVAILABLE", "Metrics collector not initialized")
		return
	}

	metrics, err := h.metricsCollector.GetMetrics(containerId, start, end, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "METRICS_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, metrics)
}

// GetLatestMetrics returns the latest metrics for a container.
func (h *Handler) GetLatestMetrics(w http.ResponseWriter, r *http.Request) {
	containerId := chi.URLParam(r, "containerId")

	if h.metricsCollector == nil {
		writeError(w, http.StatusServiceUnavailable, "METRICS_UNAVAILABLE", "Metrics collector not initialized")
		return
	}

	metric, err := h.metricsCollector.GetLatestMetrics(containerId)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "METRICS_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, metric)
}

// ListSecrets returns all secret keys (not values).
func (h *Handler) ListSecrets(w http.ResponseWriter, r *http.Request) {
	if h.secretManager == nil {
		writeError(w, http.StatusServiceUnavailable, "SECRETS_UNAVAILABLE", "Secret manager not initialized")
		return
	}

	keys, err := h.secretManager.ListSecretKeys()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "LIST_FAILED", err.Error())
		return
	}

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
	if secrets == nil {
		secrets = []store.Secret{}
	}
	writeJSON(w, http.StatusOK, secrets)
}

// SetSecret creates or updates a secret.
func (h *Handler) SetSecret(w http.ResponseWriter, r *http.Request) {
	if h.secretManager == nil {
		writeError(w, http.StatusServiceUnavailable, "SECRETS_UNAVAILABLE", "Secret manager not initialized")
		return
	}

	var req struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	if req.Key == "" {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "key is required")
		return
	}

	if err := h.secretManager.SetSecret(req.Key, req.Value); err != nil {
		writeError(w, http.StatusInternalServerError, "SET_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"status": "created"})
}

// DeleteSecret deletes a secret.
func (h *Handler) DeleteSecret(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	if h.secretManager == nil {
		writeError(w, http.StatusServiceUnavailable, "SECRETS_UNAVAILABLE", "Secret manager not initialized")
		return
	}

	if err := h.secretManager.DeleteSecret(key); err != nil {
		writeError(w, http.StatusInternalServerError, "DELETE_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// GetNetworkRoutes returns the current routing table.
func (h *Handler) GetNetworkRoutes(w http.ResponseWriter, r *http.Request) {
	if h.network == nil {
		writeJSON(w, http.StatusOK, map[string][]network.RouteTarget{})
		return
	}
	if nm, ok := h.network.(*network.LinuxNetworkManager); ok {
		writeJSON(w, http.StatusOK, nm.GetRoutes())
		return
	}
	writeJSON(w, http.StatusOK, map[string][]network.RouteTarget{})
}

// ListCerts returns TLS certificate information.
func (h *Handler) ListCerts(w http.ResponseWriter, r *http.Request) {
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

	if result.Certs == nil {
		result.Certs = []string{}
	}

	writeJSON(w, http.StatusOK, result)
}

// ImportCert imports a custom TLS certificate.
func (h *Handler) ImportCert(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Hostname string `json:"hostname"`
		CertPath string `json:"cert_path"`
		KeyPath  string `json:"key_path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	if h.network == nil {
		writeError(w, http.StatusServiceUnavailable, "NETWORK_UNAVAILABLE", "Network manager not initialized")
		return
	}

	nm, ok := h.network.(*network.LinuxNetworkManager)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "NETWORK_UNAVAILABLE", "Network manager does not support TLS")
		return
	}

	if err := nm.LoadCustomCert(req.Hostname, req.CertPath, req.KeyPath); err != nil {
		writeError(w, http.StatusInternalServerError, "IMPORT_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "imported"})
}

// GetSystemInfo returns system information.
func (h *Handler) GetSystemInfo(w http.ResponseWriter, r *http.Request) {
	apps, _ := h.manager.ListApps(r.Context())
	containers, _ := h.store.ListContainers()

	var runningContainers int
	for _, c := range containers {
		if c.State == store.ContainerStateRunning {
			runningContainers++
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"version":            "0.1.0",
		"apps":               len(apps),
		"containers":         len(containers),
		"running_containers": runningContainers,
	})
}
