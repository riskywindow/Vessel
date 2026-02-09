package health

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/vessel/vessel/internal/store"
)

// AlertConfig configures webhook alerting behavior.
type AlertConfig struct {
	WebhookURL      string        `json:"webhook_url"`
	Enabled         bool          `json:"enabled"`
	MinInterval     time.Duration `json:"min_interval"`     // Minimum time between alerts for same container
	IncludeRecovers bool          `json:"include_recovers"` // Alert on recovery too
}

// Alert is the payload sent to the webhook endpoint.
type Alert struct {
	Type        string    `json:"type"`         // "unhealthy", "recovered"
	AppID       string    `json:"app_id"`
	ContainerID string    `json:"container_id"`
	Message     string    `json:"message"`
	Timestamp   time.Time `json:"timestamp"`
}

// Alerter sends webhook notifications on health status changes.
type Alerter struct {
	config    AlertConfig
	client    *http.Client
	logger    *slog.Logger

	// Rate limiting per container
	mu        sync.Mutex
	lastAlert map[string]time.Time
}

// NewAlerter creates a new Alerter with the given configuration.
func NewAlerter(cfg AlertConfig, logger *slog.Logger) *Alerter {
	if cfg.MinInterval <= 0 {
		cfg.MinInterval = 5 * time.Minute
	}

	return &Alerter{
		config:    cfg,
		client:    &http.Client{Timeout: 10 * time.Second},
		logger:    logger,
		lastAlert: make(map[string]time.Time),
	}
}

// SendAlert sends an alert to the configured webhook endpoint.
func (a *Alerter) SendAlert(ctx context.Context, alert Alert) error {
	if !a.config.Enabled || a.config.WebhookURL == "" {
		return nil
	}

	// Rate limit check
	a.mu.Lock()
	lastTime, ok := a.lastAlert[alert.ContainerID]
	if ok && time.Since(lastTime) < a.config.MinInterval {
		a.mu.Unlock()
		a.logger.Debug("alert rate limited", "container", alert.ContainerID[:12])
		return nil
	}
	a.lastAlert[alert.ContainerID] = time.Now()
	a.mu.Unlock()

	// Skip recovery alerts if not configured
	if alert.Type == "recovered" && !a.config.IncludeRecovers {
		return nil
	}

	// Send webhook
	payload, err := json.Marshal(alert)
	if err != nil {
		return fmt.Errorf("failed to marshal alert: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", a.config.WebhookURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Vessel/1.0")

	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send alert: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook returned %d", resp.StatusCode)
	}

	a.logger.Info("alert sent", "type", alert.Type, "container", alert.ContainerID[:12])
	return nil
}

// HandleHealthEvent converts a HealthEvent to an Alert and sends it.
func (a *Alerter) HandleHealthEvent(ctx context.Context, event HealthEvent) {
	var alertType string
	switch event.NewStatus {
	case store.HealthStatusUnhealthy:
		alertType = "unhealthy"
	case store.HealthStatusHealthy:
		if event.OldStatus == store.HealthStatusUnhealthy {
			alertType = "recovered"
		} else {
			return // Don't alert on initial healthy
		}
	default:
		return
	}

	alert := Alert{
		Type:        alertType,
		AppID:       event.AppID,
		ContainerID: event.ContainerID,
		Message:     event.Message,
		Timestamp:   event.Timestamp,
	}

	if err := a.SendAlert(ctx, alert); err != nil {
		a.logger.Warn("failed to send alert", "error", err)
	}
}
