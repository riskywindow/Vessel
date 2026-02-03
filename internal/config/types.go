// Package config handles configuration parsing and validation.
package config

import "time"

// Config represents the top-level vessel.toml configuration.
type Config struct {
	// Global settings
	DataDir    string `toml:"data_dir"`
	LogLevel   string `toml:"log_level"`
	APIAddress string `toml:"api_address"`
	APIPort    int    `toml:"api_port"`

	// Apps is a list of application configurations.
	Apps []AppConfig `toml:"app"`
}

// AppConfig represents a single application definition in vessel.toml.
type AppConfig struct {
	// Name is the unique identifier for this application.
	Name string `toml:"name"`

	// Image is the OCI image reference (e.g., "nginx:latest", "ghcr.io/owner/app:v1").
	Image string `toml:"image"`

	// Instances is the number of container instances to run.
	Instances int `toml:"instances"`

	// Command overrides the image's default command.
	Command []string `toml:"command"`

	// Environment variables to set in the container.
	Env map[string]string `toml:"env"`

	// Resources defines resource limits for each container.
	Resources ResourceConfig `toml:"resources"`

	// Ports defines port mappings.
	Ports []PortConfig `toml:"port"`

	// Domains defines the hostnames that route to this app.
	Domains []string `toml:"domains"`

	// HealthCheck defines how to verify the app is healthy.
	HealthCheck *HealthCheckConfig `toml:"health_check"`

	// Deploy configures deployment behavior.
	Deploy DeployConfig `toml:"deploy"`

	// Volumes defines volume mounts.
	Volumes []VolumeConfig `toml:"volume"`
}

// ResourceConfig defines container resource limits.
type ResourceConfig struct {
	// CPU is the CPU limit as a percentage (e.g., 50 for 50%, 200 for 2 cores).
	CPU int `toml:"cpu"`

	// Memory is the memory limit (e.g., "256MB", "1GB").
	Memory string `toml:"memory"`

	// Swap is the swap limit (e.g., "512MB", "-1" for same as memory).
	Swap string `toml:"swap"`

	// Pids is the maximum number of processes.
	Pids int `toml:"pids"`
}

// PortConfig defines a port mapping.
type PortConfig struct {
	// Host is the port on the host (0 for auto-assign).
	Host int `toml:"host"`

	// Container is the port inside the container.
	Container int `toml:"container"`

	// Protocol is "tcp" or "udp".
	Protocol string `toml:"protocol"`
}

// HealthCheckConfig defines health check parameters.
type HealthCheckConfig struct {
	// Type is "http", "tcp", or "command".
	Type string `toml:"type"`

	// Path is the HTTP path for HTTP health checks.
	Path string `toml:"path"`

	// Port is the port to check for HTTP and TCP health checks.
	Port int `toml:"port"`

	// Command is the command to run for command health checks.
	Command []string `toml:"command"`

	// Interval is the time between health checks.
	Interval Duration `toml:"interval"`

	// Timeout is the maximum time to wait for a health check response.
	Timeout Duration `toml:"timeout"`

	// Retries is the number of consecutive failures before marking unhealthy.
	Retries int `toml:"retries"`

	// StartPeriod is the grace period before health checks start.
	StartPeriod Duration `toml:"start_period"`
}

// DeployConfig defines deployment behavior.
type DeployConfig struct {
	// Strategy is "rolling" or "blue-green".
	Strategy string `toml:"strategy"`

	// GracePeriod is the time to wait for graceful shutdown.
	GracePeriod Duration `toml:"grace_period"`

	// HealthyThreshold is how many successful health checks before marking healthy.
	HealthyThreshold int `toml:"healthy_threshold"`

	// UnhealthyThreshold is how many failed health checks before marking unhealthy.
	UnhealthyThreshold int `toml:"unhealthy_threshold"`
}

// VolumeConfig defines a volume mount.
type VolumeConfig struct {
	// Host is the path on the host.
	Host string `toml:"host"`

	// Container is the path inside the container.
	Container string `toml:"container"`

	// ReadOnly indicates if the mount is read-only.
	ReadOnly bool `toml:"readonly"`
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
