package api

import (
	"bufio"
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	"github.com/vessel/vessel/internal/health"
	"github.com/vessel/vessel/internal/store"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// WSMessage is the format for WebSocket messages.
type WSMessage struct {
	Type      string      `json:"type"`
	Timestamp time.Time   `json:"timestamp"`
	Data      interface{} `json:"data"`
}

// StreamContainerLogs streams container logs via WebSocket.
func (h *Handler) StreamContainerLogs(w http.ResponseWriter, r *http.Request) {
	containerID := chi.URLParam(r, "id")

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Handle client disconnect
	go func() {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				cancel()
				return
			}
		}
	}()

	// Get log reader with follow=true
	reader, err := h.runtime.ContainerLogs(ctx, containerID, true, 100)
	if err != nil {
		conn.WriteJSON(WSMessage{
			Type:      "error",
			Timestamp: time.Now(),
			Data:      map[string]string{"message": err.Error()},
		})
		return
	}
	defer reader.Close()

	// Stream logs line by line
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
			msg := WSMessage{
				Type:      "log",
				Timestamp: time.Now(),
				Data: map[string]string{
					"line": scanner.Text(),
				},
			}
			if err := conn.WriteJSON(msg); err != nil {
				return
			}
		}
	}
}

// StreamContainerMetrics streams metrics for a single container via WebSocket.
func (h *Handler) StreamContainerMetrics(w http.ResponseWriter, r *http.Request) {
	containerID := chi.URLParam(r, "id")

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Handle client disconnect
	go func() {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				cancel()
				return
			}
		}
	}()

	// Stream metrics every 2 seconds
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	// Send initial metric immediately
	if err := h.sendMetric(conn, containerID); err != nil {
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := h.sendMetric(conn, containerID); err != nil {
				return
			}
		}
	}
}

func (h *Handler) sendMetric(conn *websocket.Conn, containerID string) error {
	metric, err := h.runtime.ContainerStats(context.Background(), containerID)
	if err != nil {
		return conn.WriteJSON(WSMessage{
			Type:      "error",
			Timestamp: time.Now(),
			Data:      map[string]string{"message": err.Error()},
		})
	}

	return conn.WriteJSON(WSMessage{
		Type:      "metric",
		Timestamp: time.Now(),
		Data:      metric,
	})
}

// StreamAllMetrics streams metrics for all running containers via WebSocket.
func (h *Handler) StreamAllMetrics(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Handle client disconnect
	go func() {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				cancel()
				return
			}
		}
	}()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	// Send initial metrics
	if err := h.sendAllMetrics(conn); err != nil {
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := h.sendAllMetrics(conn); err != nil {
				return
			}
		}
	}
}

func (h *Handler) sendAllMetrics(conn *websocket.Conn) error {
	containers, err := h.store.ListContainers()
	if err != nil {
		return err
	}

	metrics := make(map[string]*store.MetricPoint)
	for _, c := range containers {
		if c.State == store.ContainerStateRunning {
			if m, err := h.runtime.ContainerStats(context.Background(), c.ID); err == nil {
				metrics[c.ID] = m
			}
		}
	}

	return conn.WriteJSON(WSMessage{
		Type:      "metrics",
		Timestamp: time.Now(),
		Data:      metrics,
	})
}

// StreamHealth streams health status updates via WebSocket.
func (h *Handler) StreamHealth(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	if h.healthMonitor == nil {
		conn.WriteJSON(WSMessage{
			Type:      "error",
			Timestamp: time.Now(),
			Data:      map[string]string{"message": "health monitor not available"},
		})
		return
	}

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Handle client disconnect
	go func() {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				cancel()
				return
			}
		}
	}()

	// Subscribe to health events
	eventCh := h.healthMonitor.Subscribe()
	defer h.healthMonitor.Unsubscribe(eventCh)

	// Send initial status
	conn.WriteJSON(WSMessage{
		Type:      "health",
		Timestamp: time.Now(),
		Data:      h.healthMonitor.GetAllStatus(),
	})

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-eventCh:
			if !ok {
				return
			}
			if err := conn.WriteJSON(WSMessage{
				Type:      "health_event",
				Timestamp: event.Timestamp,
				Data:      event,
			}); err != nil {
				return
			}
		}
	}
}

// StreamEvents streams all daemon events (apps, containers, health).
func (h *Handler) StreamEvents(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Handle client disconnect
	go func() {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				cancel()
				return
			}
		}
	}()

	// Subscribe to health events if monitor is available
	var healthCh <-chan health.HealthEvent
	if h.healthMonitor != nil {
		healthCh = h.healthMonitor.Subscribe()
		defer h.healthMonitor.Unsubscribe(healthCh)
	}

	// Periodic state refresh (every 5s)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// Send initial state
	h.sendFullState(conn)

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-healthCh:
			if !ok {
				healthCh = nil
				continue
			}
			conn.WriteJSON(WSMessage{
				Type:      "health_event",
				Timestamp: event.Timestamp,
				Data:      event,
			})
		case <-ticker.C:
			h.sendFullState(conn)
		}
	}
}

func (h *Handler) sendFullState(conn *websocket.Conn) {
	apps, _ := h.manager.ListApps(context.Background())

	var healthStatus map[string]*health.ContainerHealth
	if h.healthMonitor != nil {
		healthStatus = h.healthMonitor.GetAllStatus()
	}

	conn.WriteJSON(WSMessage{
		Type:      "state",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"apps":   apps,
			"health": healthStatus,
		},
	})
}
