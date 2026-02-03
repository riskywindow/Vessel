package config

import "time"

// Default configuration values.
const (
	DefaultDataDir    = "/var/lib/vessel"
	DefaultLogLevel   = "info"
	DefaultAPIAddress = "127.0.0.1"
	DefaultAPIPort    = 9477

	DefaultInstances          = 1
	DefaultCPU                = 100 // 100% = 1 core
	DefaultMemory             = "256MB"
	DefaultSwap               = "-1" // Same as memory
	DefaultPids               = 100
	DefaultPortProtocol       = "tcp"
	DefaultHealthCheckType    = "http"
	DefaultHealthCheckPath    = "/"
	DefaultHealthCheckPort    = 80
	DefaultHealthInterval     = 30 * time.Second
	DefaultHealthTimeout      = 5 * time.Second
	DefaultHealthRetries      = 3
	DefaultHealthStartPeriod  = 0
	DefaultDeployStrategy     = "rolling"
	DefaultGracePeriod        = 30 * time.Second
	DefaultHealthyThreshold   = 2
	DefaultUnhealthyThreshold = 3
)

// ApplyDefaults fills in default values for any unset configuration fields.
func ApplyDefaults(cfg *Config) {
	// Global defaults
	if cfg.DataDir == "" {
		cfg.DataDir = DefaultDataDir
	}
	if cfg.LogLevel == "" {
		cfg.LogLevel = DefaultLogLevel
	}
	if cfg.APIAddress == "" {
		cfg.APIAddress = DefaultAPIAddress
	}
	if cfg.APIPort == 0 {
		cfg.APIPort = DefaultAPIPort
	}

	// App defaults
	for i := range cfg.Apps {
		applyAppDefaults(&cfg.Apps[i])
	}
}

func applyAppDefaults(app *AppConfig) {
	// Instances
	if app.Instances == 0 {
		app.Instances = DefaultInstances
	}

	// Resources
	if app.Resources.CPU == 0 {
		app.Resources.CPU = DefaultCPU
	}
	if app.Resources.Memory == "" {
		app.Resources.Memory = DefaultMemory
	}
	if app.Resources.Swap == "" {
		app.Resources.Swap = DefaultSwap
	}
	if app.Resources.Pids == 0 {
		app.Resources.Pids = DefaultPids
	}

	// Port defaults
	for j := range app.Ports {
		if app.Ports[j].Protocol == "" {
			app.Ports[j].Protocol = DefaultPortProtocol
		}
	}

	// Health check defaults
	if app.HealthCheck != nil {
		applyHealthCheckDefaults(app.HealthCheck)
	}

	// Deploy defaults
	applyDeployDefaults(&app.Deploy)
}

func applyHealthCheckDefaults(hc *HealthCheckConfig) {
	if hc.Type == "" {
		hc.Type = DefaultHealthCheckType
	}
	if hc.Path == "" && hc.Type == "http" {
		hc.Path = DefaultHealthCheckPath
	}
	if hc.Port == 0 {
		hc.Port = DefaultHealthCheckPort
	}
	if hc.Interval.Duration == 0 {
		hc.Interval.Duration = DefaultHealthInterval
	}
	if hc.Timeout.Duration == 0 {
		hc.Timeout.Duration = DefaultHealthTimeout
	}
	if hc.Retries == 0 {
		hc.Retries = DefaultHealthRetries
	}
	// StartPeriod defaults to 0, which is valid
}

func applyDeployDefaults(deploy *DeployConfig) {
	if deploy.Strategy == "" {
		deploy.Strategy = DefaultDeployStrategy
	}
	if deploy.GracePeriod.Duration == 0 {
		deploy.GracePeriod.Duration = DefaultGracePeriod
	}
	if deploy.HealthyThreshold == 0 {
		deploy.HealthyThreshold = DefaultHealthyThreshold
	}
	if deploy.UnhealthyThreshold == 0 {
		deploy.UnhealthyThreshold = DefaultUnhealthyThreshold
	}
}
