package config

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Valid values for various config fields.
var (
	ValidLogLevels       = []string{"debug", "info", "warn", "error"}
	ValidHealthTypes     = []string{"http", "tcp", "command"}
	ValidDeployStrategies = []string{"rolling", "blue-green"}
	ValidPortProtocols   = []string{"tcp", "udp"}
)

// Regular expressions for validation.
var (
	appNameRegex   = regexp.MustCompile(`^[a-z][a-z0-9-]*[a-z0-9]$|^[a-z]$`)
	imageRefRegex  = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._/-]*[a-zA-Z0-9](:[a-zA-Z0-9._-]+)?(@sha256:[a-f0-9]{64})?$`)
	memorySizeRegex = regexp.MustCompile(`^(\d+)(B|KB|MB|GB|TB)$`)
)

// Validate checks the configuration for errors and returns all found issues.
// This is not fail-fast; it collects all errors.
func Validate(cfg *Config) []error {
	var errs []error

	// Validate global settings
	if !contains(ValidLogLevels, cfg.LogLevel) {
		errs = append(errs, fmt.Errorf("invalid log_level %q: must be one of %v", cfg.LogLevel, ValidLogLevels))
	}

	if cfg.APIPort < 1 || cfg.APIPort > 65535 {
		errs = append(errs, fmt.Errorf("invalid api_port %d: must be between 1 and 65535", cfg.APIPort))
	}

	// Validate apps
	if len(cfg.Apps) == 0 {
		errs = append(errs, fmt.Errorf("at least one [[app]] must be defined"))
	}

	seenNames := make(map[string]bool)
	for i, app := range cfg.Apps {
		appErrs := validateApp(&app, i, seenNames)
		errs = append(errs, appErrs...)
	}

	return errs
}

func validateApp(app *AppConfig, index int, seenNames map[string]bool) []error {
	var errs []error
	prefix := fmt.Sprintf("app[%d]", index)

	// Name validation
	if app.Name == "" {
		errs = append(errs, fmt.Errorf("%s: name is required", prefix))
	} else {
		if !appNameRegex.MatchString(app.Name) {
			errs = append(errs, fmt.Errorf("%s: invalid name %q: must be lowercase alphanumeric with hyphens, starting with a letter", prefix, app.Name))
		}
		if seenNames[app.Name] {
			errs = append(errs, fmt.Errorf("%s: duplicate app name %q", prefix, app.Name))
		}
		seenNames[app.Name] = true
		prefix = fmt.Sprintf("app[%s]", app.Name)
	}

	// Image validation
	if app.Image == "" {
		errs = append(errs, fmt.Errorf("%s: image is required", prefix))
	} else if !imageRefRegex.MatchString(app.Image) {
		errs = append(errs, fmt.Errorf("%s: invalid image reference %q", prefix, app.Image))
	}

	// Instances validation
	if app.Instances < 1 {
		errs = append(errs, fmt.Errorf("%s: instances must be at least 1", prefix))
	}

	// Resources validation
	errs = append(errs, validateResources(&app.Resources, prefix)...)

	// Ports validation
	errs = append(errs, validatePorts(app.Ports, prefix)...)

	// Health check validation
	if app.HealthCheck != nil {
		errs = append(errs, validateHealthCheck(app.HealthCheck, prefix)...)
	}

	// Deploy validation
	errs = append(errs, validateDeploy(&app.Deploy, prefix)...)

	// Volumes validation
	errs = append(errs, validateVolumes(app.Volumes, prefix)...)

	return errs
}

func validateResources(res *ResourceConfig, prefix string) []error {
	var errs []error

	if res.CPU < 1 {
		errs = append(errs, fmt.Errorf("%s.resources: cpu must be at least 1", prefix))
	}

	if res.Memory != "" {
		if _, err := parseMemorySize(res.Memory); err != nil {
			errs = append(errs, fmt.Errorf("%s.resources: invalid memory %q: %v", prefix, res.Memory, err))
		}
	}

	if res.Swap != "" && res.Swap != "-1" {
		if _, err := parseMemorySize(res.Swap); err != nil {
			errs = append(errs, fmt.Errorf("%s.resources: invalid swap %q: %v", prefix, res.Swap, err))
		}
	}

	if res.Pids < 1 {
		errs = append(errs, fmt.Errorf("%s.resources: pids must be at least 1", prefix))
	}

	return errs
}

func validatePorts(ports []PortConfig, prefix string) []error {
	var errs []error

	seenHostPorts := make(map[int]bool)
	for i, port := range ports {
		portPrefix := fmt.Sprintf("%s.port[%d]", prefix, i)

		if port.Container < 1 || port.Container > 65535 {
			errs = append(errs, fmt.Errorf("%s: container port must be between 1 and 65535", portPrefix))
		}

		if port.Host != 0 && (port.Host < 1 || port.Host > 65535) {
			errs = append(errs, fmt.Errorf("%s: host port must be between 1 and 65535 or 0 for auto-assign", portPrefix))
		}

		if port.Host != 0 && seenHostPorts[port.Host] {
			errs = append(errs, fmt.Errorf("%s: duplicate host port %d", portPrefix, port.Host))
		}
		seenHostPorts[port.Host] = true

		if !contains(ValidPortProtocols, port.Protocol) {
			errs = append(errs, fmt.Errorf("%s: invalid protocol %q: must be one of %v", portPrefix, port.Protocol, ValidPortProtocols))
		}
	}

	return errs
}

func validateHealthCheck(hc *HealthCheckConfig, prefix string) []error {
	var errs []error
	hcPrefix := prefix + ".health_check"

	if !contains(ValidHealthTypes, hc.Type) {
		errs = append(errs, fmt.Errorf("%s: invalid type %q: must be one of %v", hcPrefix, hc.Type, ValidHealthTypes))
	}

	switch hc.Type {
	case "http":
		if hc.Path == "" {
			errs = append(errs, fmt.Errorf("%s: path is required for HTTP health checks", hcPrefix))
		}
		if hc.Port < 1 || hc.Port > 65535 {
			errs = append(errs, fmt.Errorf("%s: port must be between 1 and 65535", hcPrefix))
		}
	case "tcp":
		if hc.Port < 1 || hc.Port > 65535 {
			errs = append(errs, fmt.Errorf("%s: port must be between 1 and 65535", hcPrefix))
		}
	case "command":
		if len(hc.Command) == 0 {
			errs = append(errs, fmt.Errorf("%s: command is required for command health checks", hcPrefix))
		}
	}

	if hc.Interval.Duration <= 0 {
		errs = append(errs, fmt.Errorf("%s: interval must be positive", hcPrefix))
	}

	if hc.Timeout.Duration <= 0 {
		errs = append(errs, fmt.Errorf("%s: timeout must be positive", hcPrefix))
	}

	if hc.Timeout.Duration >= hc.Interval.Duration {
		errs = append(errs, fmt.Errorf("%s: timeout must be less than interval", hcPrefix))
	}

	if hc.Retries < 1 {
		errs = append(errs, fmt.Errorf("%s: retries must be at least 1", hcPrefix))
	}

	return errs
}

func validateDeploy(deploy *DeployConfig, prefix string) []error {
	var errs []error
	deployPrefix := prefix + ".deploy"

	if !contains(ValidDeployStrategies, deploy.Strategy) {
		errs = append(errs, fmt.Errorf("%s: invalid strategy %q: must be one of %v", deployPrefix, deploy.Strategy, ValidDeployStrategies))
	}

	if deploy.GracePeriod.Duration < 0 {
		errs = append(errs, fmt.Errorf("%s: grace_period cannot be negative", deployPrefix))
	}

	if deploy.HealthyThreshold < 1 {
		errs = append(errs, fmt.Errorf("%s: healthy_threshold must be at least 1", deployPrefix))
	}

	if deploy.UnhealthyThreshold < 1 {
		errs = append(errs, fmt.Errorf("%s: unhealthy_threshold must be at least 1", deployPrefix))
	}

	return errs
}

func validateVolumes(volumes []VolumeConfig, prefix string) []error {
	var errs []error

	for i, vol := range volumes {
		volPrefix := fmt.Sprintf("%s.volume[%d]", prefix, i)

		if vol.Host == "" {
			errs = append(errs, fmt.Errorf("%s: host path is required", volPrefix))
		}

		if vol.Container == "" {
			errs = append(errs, fmt.Errorf("%s: container path is required", volPrefix))
		}

		// Container path must be absolute
		if vol.Container != "" && !strings.HasPrefix(vol.Container, "/") {
			errs = append(errs, fmt.Errorf("%s: container path must be absolute", volPrefix))
		}
	}

	return errs
}

// Helper functions

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func parseMemorySize(s string) (int64, error) {
	matches := memorySizeRegex.FindStringSubmatch(strings.ToUpper(s))
	if matches == nil {
		return 0, fmt.Errorf("must be a number followed by B, KB, MB, GB, or TB")
	}

	value, err := strconv.ParseInt(matches[1], 10, 64)
	if err != nil {
		return 0, err
	}

	multipliers := map[string]int64{
		"B":  1,
		"KB": 1024,
		"MB": 1024 * 1024,
		"GB": 1024 * 1024 * 1024,
		"TB": 1024 * 1024 * 1024 * 1024,
	}

	return value * multipliers[matches[2]], nil
}

// ParseMemorySize is exported for use by other packages.
func ParseMemorySize(s string) (int64, error) {
	return parseMemorySize(s)
}
