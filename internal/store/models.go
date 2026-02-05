// Package store provides persistent state storage using BBolt.
package store

import "time"

// AppState represents the current state of an application.
type AppState string

const (
	AppStateRunning   AppState = "running"
	AppStateStopped   AppState = "stopped"
	AppStateDeploying AppState = "deploying"
	AppStateFailed    AppState = "failed"
)

// ContainerState represents the current state of a container.
type ContainerState string

const (
	ContainerStateCreated  ContainerState = "created"
	ContainerStateRunning  ContainerState = "running"
	ContainerStateStopped  ContainerState = "stopped"
	ContainerStateFailed   ContainerState = "failed"
	ContainerStateRemoving ContainerState = "removing"
)

// DeployStatus represents the result of a deploy operation.
type DeployStatus string

const (
	DeployStatusPending    DeployStatus = "pending"
	DeployStatusActive     DeployStatus = "active"
	DeployStatusFailed     DeployStatus = "failed"
	DeployStatusRolledBack DeployStatus = "rolled_back"
)

// HealthStatus represents the health state of a container.
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
	HealthStatusUnknown   HealthStatus = "unknown"
)

// DeployStrategy represents how deploys are performed.
type DeployStrategy string

const (
	DeployStrategyRolling   DeployStrategy = "rolling"
	DeployStrategyBlueGreen DeployStrategy = "blue-green"
)

// HealthCheckType represents the kind of health check.
type HealthCheckType string

const (
	HealthCheckHTTP    HealthCheckType = "http"
	HealthCheckTCP     HealthCheckType = "tcp"
	HealthCheckCommand HealthCheckType = "command"
)

// App is the top-level unit of deployment.
type App struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Image     string    `json:"image"`               // OCI image reference
	Command   []string  `json:"command,omitempty"`    // Override image command
	Env       map[string]string `json:"env,omitempty"` // Environment variables
	Instances int       `json:"instances"`            // Desired instance count
	Resources ResourceLimits `json:"resources"`       // Resource limits per container
	State     AppState  `json:"state"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Container represents a single running (or stopped) container instance.
type Container struct {
	ID           string         `json:"id"`
	AppID        string         `json:"app_id"`
	Image        string         `json:"image"`        // Full image reference including digest
	ImageDigest  string         `json:"image_digest"` // SHA256 digest
	State        ContainerState `json:"state"`
	PID          int            `json:"pid"`          // Host PID of the container init process
	IP           string         `json:"ip"`           // Container IP on the vessel bridge
	Resources    ResourceLimits `json:"resources"`
	CreatedAt    time.Time      `json:"created_at"`
	StartedAt    *time.Time     `json:"started_at,omitempty"`
	StoppedAt    *time.Time     `json:"stopped_at,omitempty"`
	ExitCode     *int           `json:"exit_code,omitempty"`
	RestartCount int            `json:"restart_count"`
}

// ResourceLimits defines the resource constraints for a container.
type ResourceLimits struct {
	CPUQuota  int64 `json:"cpu_quota"`  // Microseconds per period (e.g., 50000 for 50%)
	CPUPeriod int64 `json:"cpu_period"` // Period in microseconds (default 100000)
	MemoryMax int64 `json:"memory_max"` // Bytes
	SwapMax   int64 `json:"swap_max"`   // Bytes (-1 for same as memory)
	PidsMax   int64 `json:"pids_max"`   // Max number of processes
}

// Deploy records a single deploy event.
type Deploy struct {
	ID          string         `json:"id"`
	AppID       string         `json:"app_id"`
	Image       string         `json:"image"`
	ImageDigest string         `json:"image_digest"`
	Strategy    DeployStrategy `json:"strategy"`
	Status      DeployStatus   `json:"status"`
	Version     int            `json:"version"`               // Monotonically increasing per app
	CreatedAt   time.Time      `json:"created_at"`
	FinishedAt  *time.Time     `json:"finished_at,omitempty"`
	Error       string         `json:"error,omitempty"`
	RollbackOf  *int           `json:"rollback_of,omitempty"` // Version this rolled back from
}

// HealthCheck defines how to check if a container is healthy.
type HealthCheck struct {
	Type     HealthCheckType `json:"type"`
	Target   string          `json:"target"` // URL path, TCP address, or command
	Interval time.Duration   `json:"interval"`
	Timeout  time.Duration   `json:"timeout"`
	Retries  int             `json:"retries"`
}

// HealthResult is the outcome of a single health check execution.
type HealthResult struct {
	ContainerID string       `json:"container_id"`
	Status      HealthStatus `json:"status"`
	Message     string       `json:"message,omitempty"`
	Duration    time.Duration `json:"duration"`
	CheckedAt   time.Time    `json:"checked_at"`
}

// Secret is an encrypted key-value pair.
type Secret struct {
	Key       string    `json:"key"`
	Value     []byte    `json:"-"` // Encrypted, never serialized to JSON
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// MetricPoint is a single time-series data point for resource metrics.
type MetricPoint struct {
	Timestamp      time.Time `json:"timestamp"`
	CPUPercent     float64   `json:"cpu_percent"`
	MemoryBytes    int64     `json:"memory_bytes"`
	MemoryLimit    int64     `json:"memory_limit"`
	NetworkRxBytes int64     `json:"network_rx_bytes"`
	NetworkTxBytes int64     `json:"network_tx_bytes"`
	PidsCount      int64     `json:"pids_count"`
}
