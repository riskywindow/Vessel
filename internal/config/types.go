// Package config handles configuration parsing and validation.
package config

import "time"

// Config represents the top-level vessel.toml configuration.
type Config struct {
	// Global settings
	DataDir    string `toml:"data_dir" json:"data_dir"`
	LogLevel   string `toml:"log_level" json:"log_level"`
	APIAddress string `toml:"api_address" json:"api_address"`
	APIPort    int    `toml:"api_port" json:"api_port"`

	// Alerting configures webhook alerting for health status changes.
	Alerting AlertingConfig `toml:"alerting" json:"alerting"`

	// Apps is a list of application configurations.
	Apps []AppConfig `toml:"app" json:"apps"`

	// Remotes is a list of remote server configurations for SSH deployment.
	Remotes []RemoteConfig `toml:"remote" json:"remotes,omitempty"`
}

// AlertingConfig configures webhook alerting behavior.
type AlertingConfig struct {
	// Enabled controls whether webhook alerting is active.
	Enabled bool `toml:"enabled" json:"enabled"`

	// WebhookURL is the endpoint to POST alert payloads to.
	WebhookURL string `toml:"webhook_url" json:"webhook_url"`

	// MinInterval is the minimum time between alerts for the same container.
	MinInterval Duration `toml:"min_interval" json:"min_interval"`

	// IncludeRecovers controls whether recovery alerts are sent.
	IncludeRecovers bool `toml:"include_recovers" json:"include_recovers"`
}

// AppConfig represents a single application definition in vessel.toml.
type AppConfig struct {
	// Name is the unique identifier for this application.
	Name string `toml:"name" json:"name"`

	// Image is the OCI image reference (e.g., "nginx:latest", "ghcr.io/owner/app:v1").
	Image string `toml:"image" json:"image"`

	// Instances is the number of container instances to run.
	Instances int `toml:"instances" json:"instances"`

	// Command overrides the image's default command.
	Command []string `toml:"command" json:"command,omitempty"`

	// Environment variables to set in the container.
	Env map[string]string `toml:"env" json:"env,omitempty"`

	// Resources defines resource limits for each container.
	Resources ResourceConfig `toml:"resources" json:"resources"`

	// Ports defines port mappings.
	Ports []PortConfig `toml:"port" json:"ports,omitempty"`

	// Domains defines the hostnames that route to this app.
	Domains []string `toml:"domains" json:"domains,omitempty"`

	// HealthCheck defines how to verify the app is healthy.
	HealthCheck *HealthCheckConfig `toml:"health_check" json:"health_check,omitempty"`

	// Deploy configures deployment behavior.
	Deploy DeployConfig `toml:"deploy" json:"deploy"`

	// Volumes defines volume mounts.
	Volumes []VolumeConfig `toml:"volume" json:"volumes,omitempty"`
}

// ResourceConfig defines container resource limits.
type ResourceConfig struct {
	// CPU is the CPU limit as a percentage (e.g., 50 for 50%, 200 for 2 cores).
	CPU int `toml:"cpu" json:"cpu"`

	// Memory is the memory limit (e.g., "256MB", "1GB").
	Memory string `toml:"memory" json:"memory"`

	// Swap is the swap limit (e.g., "512MB", "-1" for same as memory).
	Swap string `toml:"swap" json:"swap"`

	// Pids is the maximum number of processes.
	Pids int `toml:"pids" json:"pids"`
}

// PortConfig defines a port mapping.
type PortConfig struct {
	// Host is the port on the host (0 for auto-assign).
	Host int `toml:"host" json:"host"`

	// Container is the port inside the container.
	Container int `toml:"container" json:"container"`

	// Protocol is "tcp" or "udp".
	Protocol string `toml:"protocol" json:"protocol"`
}

// HealthCheckConfig defines health check parameters.
type HealthCheckConfig struct {
	// Type is "http", "tcp", or "command".
	Type string `toml:"type" json:"type"`

	// Path is the HTTP path for HTTP health checks.
	Path string `toml:"path" json:"path,omitempty"`

	// Port is the port to check for HTTP and TCP health checks.
	Port int `toml:"port" json:"port,omitempty"`

	// Command is the command to run for command health checks.
	Command []string `toml:"command" json:"command,omitempty"`

	// Interval is the time between health checks.
	Interval Duration `toml:"interval" json:"interval"`

	// Timeout is the maximum time to wait for a health check response.
	Timeout Duration `toml:"timeout" json:"timeout"`

	// Retries is the number of consecutive failures before marking unhealthy.
	Retries int `toml:"retries" json:"retries"`

	// StartPeriod is the grace period before health checks start.
	StartPeriod Duration `toml:"start_period" json:"start_period"`
}

// DeployConfig defines deployment behavior.
type DeployConfig struct {
	// Strategy is "rolling" or "blue-green".
	Strategy string `toml:"strategy" json:"strategy"`

	// GracePeriod is the time to wait for graceful shutdown.
	GracePeriod Duration `toml:"grace_period" json:"grace_period"`

	// HealthyThreshold is how many successful health checks before marking healthy.
	HealthyThreshold int `toml:"healthy_threshold" json:"healthy_threshold"`

	// UnhealthyThreshold is how many failed health checks before marking unhealthy.
	UnhealthyThreshold int `toml:"unhealthy_threshold" json:"unhealthy_threshold"`
}

// VolumeConfig defines a volume mount.
type VolumeConfig struct {
	// Host is the path on the host.
	Host string `toml:"host" json:"host"`

	// Container is the path inside the container.
	Container string `toml:"container" json:"container"`

	// ReadOnly indicates if the mount is read-only.
	ReadOnly bool `toml:"readonly" json:"readonly"`
}

// RemoteConfig defines a remote server for SSH deployment.
type RemoteConfig struct {
	// Name is the alias for this remote (e.g., "prod", "staging").
	Name string `toml:"name" json:"name"`

	// Host is the SSH hostname or IP address.
	Host string `toml:"host" json:"host"`

	// User is the SSH username.
	User string `toml:"user" json:"user"`

	// Port is the SSH port (default 22).
	Port int `toml:"port" json:"port,omitempty"`

	// KeyFile is the path to the SSH private key (optional).
	KeyFile string `toml:"key_file" json:"key_file,omitempty"`
}

// Duration is a wrapper around time.Duration for TOML parsing.
type Duration struct {
	time.Duration
}

// UnmarshalText implements encoding.TextUnmarshaler for Duration.
func (d *Duration) UnmarshalText(text []byte) error {
	var err error
	d.Duration, err = time.ParseDuration(string(text))
	return err
}

// MarshalText implements encoding.TextMarshaler for Duration.
func (d Duration) MarshalText() ([]byte, error) {
	return []byte(d.Duration.String()), nil
}
