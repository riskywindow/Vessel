package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseMinimalConfig(t *testing.T) {
	toml := `
[[app]]
name = "myapp"
image = "nginx:latest"
`
	cfg, err := ParseBytes([]byte(toml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.Apps) != 1 {
		t.Fatalf("expected 1 app, got %d", len(cfg.Apps))
	}

	if cfg.Apps[0].Name != "myapp" {
		t.Errorf("expected app name 'myapp', got %q", cfg.Apps[0].Name)
	}

	if cfg.Apps[0].Image != "nginx:latest" {
		t.Errorf("expected image 'nginx:latest', got %q", cfg.Apps[0].Image)
	}
}

func TestParseFullConfig(t *testing.T) {
	toml := `
data_dir = "/data/vessel"
log_level = "debug"
api_address = "0.0.0.0"
api_port = 8080

[[app]]
name = "web"
image = "myapp:v1.2.3"
instances = 3
command = ["/app/start", "--port=8080"]
domains = ["example.com", "www.example.com"]

[app.env]
NODE_ENV = "production"
DATABASE_URL = "postgres://localhost/db"

[app.resources]
cpu = 200
memory = "512MB"
swap = "1GB"
pids = 200

[[app.port]]
host = 8080
container = 80
protocol = "tcp"

[[app.port]]
host = 8443
container = 443
protocol = "tcp"

[app.health_check]
type = "http"
path = "/health"
port = 80
interval = "10s"
timeout = "3s"
retries = 5
start_period = "30s"

[app.deploy]
strategy = "blue-green"
grace_period = "60s"
healthy_threshold = 3
unhealthy_threshold = 5

[[app.volume]]
host = "/data/uploads"
container = "/app/uploads"
readonly = false

[[app]]
name = "worker"
image = "myworker:latest"
instances = 2

[app.health_check]
type = "command"
command = ["./healthcheck.sh"]
interval = "15s"
timeout = "5s"
retries = 3
`
	cfg, err := ParseBytes([]byte(toml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.DataDir != "/data/vessel" {
		t.Errorf("expected data_dir '/data/vessel', got %q", cfg.DataDir)
	}

	if cfg.LogLevel != "debug" {
		t.Errorf("expected log_level 'debug', got %q", cfg.LogLevel)
	}

	if cfg.APIAddress != "0.0.0.0" {
		t.Errorf("expected api_address '0.0.0.0', got %q", cfg.APIAddress)
	}

	if cfg.APIPort != 8080 {
		t.Errorf("expected api_port 8080, got %d", cfg.APIPort)
	}

	if len(cfg.Apps) != 2 {
		t.Fatalf("expected 2 apps, got %d", len(cfg.Apps))
	}

	// Test first app
	app := cfg.Apps[0]
	if app.Name != "web" {
		t.Errorf("expected app name 'web', got %q", app.Name)
	}

	if app.Instances != 3 {
		t.Errorf("expected 3 instances, got %d", app.Instances)
	}

	if len(app.Command) != 2 {
		t.Errorf("expected 2 command args, got %d", len(app.Command))
	}

	if len(app.Domains) != 2 {
		t.Errorf("expected 2 domains, got %d", len(app.Domains))
	}

	if app.Env["NODE_ENV"] != "production" {
		t.Errorf("expected NODE_ENV='production', got %q", app.Env["NODE_ENV"])
	}

	if app.Resources.CPU != 200 {
		t.Errorf("expected cpu 200, got %d", app.Resources.CPU)
	}

	if app.Resources.Memory != "512MB" {
		t.Errorf("expected memory '512MB', got %q", app.Resources.Memory)
	}

	if len(app.Ports) != 2 {
		t.Errorf("expected 2 ports, got %d", len(app.Ports))
	}

	if app.HealthCheck == nil {
		t.Fatal("expected health check to be set")
	}

	if app.HealthCheck.Type != "http" {
		t.Errorf("expected health check type 'http', got %q", app.HealthCheck.Type)
	}

	if app.HealthCheck.Interval.Duration != 10*time.Second {
		t.Errorf("expected interval 10s, got %v", app.HealthCheck.Interval.Duration)
	}

	if app.Deploy.Strategy != "blue-green" {
		t.Errorf("expected strategy 'blue-green', got %q", app.Deploy.Strategy)
	}

	if len(app.Volumes) != 1 {
		t.Errorf("expected 1 volume, got %d", len(app.Volumes))
	}

	// Test second app
	worker := cfg.Apps[1]
	if worker.HealthCheck.Type != "command" {
		t.Errorf("expected health check type 'command', got %q", worker.HealthCheck.Type)
	}
}

func TestApplyDefaults(t *testing.T) {
	cfg := &Config{
		Apps: []AppConfig{
			{
				Name:  "test",
				Image: "test:latest",
			},
		},
	}

	ApplyDefaults(cfg)

	// Global defaults
	if cfg.DataDir != DefaultDataDir {
		t.Errorf("expected data_dir %q, got %q", DefaultDataDir, cfg.DataDir)
	}

	if cfg.LogLevel != DefaultLogLevel {
		t.Errorf("expected log_level %q, got %q", DefaultLogLevel, cfg.LogLevel)
	}

	if cfg.APIAddress != DefaultAPIAddress {
		t.Errorf("expected api_address %q, got %q", DefaultAPIAddress, cfg.APIAddress)
	}

	if cfg.APIPort != DefaultAPIPort {
		t.Errorf("expected api_port %d, got %d", DefaultAPIPort, cfg.APIPort)
	}

	// App defaults
	app := cfg.Apps[0]
	if app.Instances != DefaultInstances {
		t.Errorf("expected instances %d, got %d", DefaultInstances, app.Instances)
	}

	if app.Resources.CPU != DefaultCPU {
		t.Errorf("expected cpu %d, got %d", DefaultCPU, app.Resources.CPU)
	}

	if app.Resources.Memory != DefaultMemory {
		t.Errorf("expected memory %q, got %q", DefaultMemory, app.Resources.Memory)
	}

	if app.Deploy.Strategy != DefaultDeployStrategy {
		t.Errorf("expected strategy %q, got %q", DefaultDeployStrategy, app.Deploy.Strategy)
	}

	if app.Deploy.GracePeriod.Duration != DefaultGracePeriod {
		t.Errorf("expected grace_period %v, got %v", DefaultGracePeriod, app.Deploy.GracePeriod.Duration)
	}
}

func TestValidateMinimalConfig(t *testing.T) {
	cfg := &Config{
		LogLevel: "info",
		APIPort:  9477,
		Apps: []AppConfig{
			{
				Name:      "test",
				Image:     "nginx:latest",
				Instances: 1,
				Resources: ResourceConfig{
					CPU:    100,
					Memory: "256MB",
					Pids:   100,
				},
				Deploy: DeployConfig{
					Strategy:           "rolling",
					GracePeriod:        Duration{30 * time.Second},
					HealthyThreshold:   2,
					UnhealthyThreshold: 3,
				},
			},
		},
	}

	errs := Validate(cfg)
	if len(errs) > 0 {
		t.Errorf("expected no validation errors, got: %v", errs)
	}
}

func TestValidateInvalidConfigs(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *Config
		wantErrs int
	}{
		{
			name: "no apps",
			cfg: &Config{
				LogLevel: "info",
				APIPort:  9477,
			},
			wantErrs: 1,
		},
		{
			name: "missing app name",
			cfg: &Config{
				LogLevel: "info",
				APIPort:  9477,
				Apps: []AppConfig{
					{
						Image:     "nginx:latest",
						Instances: 1,
						Resources: ResourceConfig{CPU: 100, Memory: "256MB", Pids: 100},
						Deploy:    DeployConfig{Strategy: "rolling", GracePeriod: Duration{30 * time.Second}, HealthyThreshold: 2, UnhealthyThreshold: 3},
					},
				},
			},
			wantErrs: 1,
		},
		{
			name: "missing image",
			cfg: &Config{
				LogLevel: "info",
				APIPort:  9477,
				Apps: []AppConfig{
					{
						Name:      "test",
						Instances: 1,
						Resources: ResourceConfig{CPU: 100, Memory: "256MB", Pids: 100},
						Deploy:    DeployConfig{Strategy: "rolling", GracePeriod: Duration{30 * time.Second}, HealthyThreshold: 2, UnhealthyThreshold: 3},
					},
				},
			},
			wantErrs: 1,
		},
		{
			name: "invalid app name - starts with number",
			cfg: &Config{
				LogLevel: "info",
				APIPort:  9477,
				Apps: []AppConfig{
					{
						Name:      "1app",
						Image:     "nginx:latest",
						Instances: 1,
						Resources: ResourceConfig{CPU: 100, Memory: "256MB", Pids: 100},
						Deploy:    DeployConfig{Strategy: "rolling", GracePeriod: Duration{30 * time.Second}, HealthyThreshold: 2, UnhealthyThreshold: 3},
					},
				},
			},
			wantErrs: 1,
		},
		{
			name: "invalid app name - uppercase",
			cfg: &Config{
				LogLevel: "info",
				APIPort:  9477,
				Apps: []AppConfig{
					{
						Name:      "MyApp",
						Image:     "nginx:latest",
						Instances: 1,
						Resources: ResourceConfig{CPU: 100, Memory: "256MB", Pids: 100},
						Deploy:    DeployConfig{Strategy: "rolling", GracePeriod: Duration{30 * time.Second}, HealthyThreshold: 2, UnhealthyThreshold: 3},
					},
				},
			},
			wantErrs: 1,
		},
		{
			name: "duplicate app names",
			cfg: &Config{
				LogLevel: "info",
				APIPort:  9477,
				Apps: []AppConfig{
					{Name: "app", Image: "nginx:latest", Instances: 1, Resources: ResourceConfig{CPU: 100, Memory: "256MB", Pids: 100}, Deploy: DeployConfig{Strategy: "rolling", GracePeriod: Duration{30 * time.Second}, HealthyThreshold: 2, UnhealthyThreshold: 3}},
					{Name: "app", Image: "nginx:latest", Instances: 1, Resources: ResourceConfig{CPU: 100, Memory: "256MB", Pids: 100}, Deploy: DeployConfig{Strategy: "rolling", GracePeriod: Duration{30 * time.Second}, HealthyThreshold: 2, UnhealthyThreshold: 3}},
				},
			},
			wantErrs: 1,
		},
		{
			name: "invalid log level",
			cfg: &Config{
				LogLevel: "invalid",
				APIPort:  9477,
				Apps: []AppConfig{
					{Name: "app", Image: "nginx:latest", Instances: 1, Resources: ResourceConfig{CPU: 100, Memory: "256MB", Pids: 100}, Deploy: DeployConfig{Strategy: "rolling", GracePeriod: Duration{30 * time.Second}, HealthyThreshold: 2, UnhealthyThreshold: 3}},
				},
			},
			wantErrs: 1,
		},
		{
			name: "invalid port",
			cfg: &Config{
				LogLevel: "info",
				APIPort:  99999,
				Apps: []AppConfig{
					{Name: "app", Image: "nginx:latest", Instances: 1, Resources: ResourceConfig{CPU: 100, Memory: "256MB", Pids: 100}, Deploy: DeployConfig{Strategy: "rolling", GracePeriod: Duration{30 * time.Second}, HealthyThreshold: 2, UnhealthyThreshold: 3}},
				},
			},
			wantErrs: 1,
		},
		{
			name: "invalid deploy strategy",
			cfg: &Config{
				LogLevel: "info",
				APIPort:  9477,
				Apps: []AppConfig{
					{Name: "app", Image: "nginx:latest", Instances: 1, Resources: ResourceConfig{CPU: 100, Memory: "256MB", Pids: 100}, Deploy: DeployConfig{Strategy: "invalid", GracePeriod: Duration{30 * time.Second}, HealthyThreshold: 2, UnhealthyThreshold: 3}},
				},
			},
			wantErrs: 1,
		},
		{
			name: "invalid memory format",
			cfg: &Config{
				LogLevel: "info",
				APIPort:  9477,
				Apps: []AppConfig{
					{Name: "app", Image: "nginx:latest", Instances: 1, Resources: ResourceConfig{CPU: 100, Memory: "invalid", Pids: 100}, Deploy: DeployConfig{Strategy: "rolling", GracePeriod: Duration{30 * time.Second}, HealthyThreshold: 2, UnhealthyThreshold: 3}},
				},
			},
			wantErrs: 1,
		},
		{
			name: "health check http missing path validation after fix",
			cfg: &Config{
				LogLevel: "info",
				APIPort:  9477,
				Apps: []AppConfig{
					{
						Name:      "app",
						Image:     "nginx:latest",
						Instances: 1,
						Resources: ResourceConfig{CPU: 100, Memory: "256MB", Pids: 100},
						Deploy:    DeployConfig{Strategy: "rolling", GracePeriod: Duration{30 * time.Second}, HealthyThreshold: 2, UnhealthyThreshold: 3},
						HealthCheck: &HealthCheckConfig{
							Type:     "http",
							Path:     "",
							Port:     80,
							Interval: Duration{30 * time.Second},
							Timeout:  Duration{5 * time.Second},
							Retries:  3,
						},
					},
				},
			},
			wantErrs: 1,
		},
		{
			name: "health check command missing command",
			cfg: &Config{
				LogLevel: "info",
				APIPort:  9477,
				Apps: []AppConfig{
					{
						Name:      "app",
						Image:     "nginx:latest",
						Instances: 1,
						Resources: ResourceConfig{CPU: 100, Memory: "256MB", Pids: 100},
						Deploy:    DeployConfig{Strategy: "rolling", GracePeriod: Duration{30 * time.Second}, HealthyThreshold: 2, UnhealthyThreshold: 3},
						HealthCheck: &HealthCheckConfig{
							Type:     "command",
							Interval: Duration{30 * time.Second},
							Timeout:  Duration{5 * time.Second},
							Retries:  3,
						},
					},
				},
			},
			wantErrs: 1,
		},
		{
			name: "volume with relative container path",
			cfg: &Config{
				LogLevel: "info",
				APIPort:  9477,
				Apps: []AppConfig{
					{
						Name:      "app",
						Image:     "nginx:latest",
						Instances: 1,
						Resources: ResourceConfig{CPU: 100, Memory: "256MB", Pids: 100},
						Deploy:    DeployConfig{Strategy: "rolling", GracePeriod: Duration{30 * time.Second}, HealthyThreshold: 2, UnhealthyThreshold: 3},
						Volumes: []VolumeConfig{
							{Host: "/data", Container: "relative/path"},
						},
					},
				},
			},
			wantErrs: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := Validate(tt.cfg)
			if len(errs) != tt.wantErrs {
				t.Errorf("expected %d errors, got %d: %v", tt.wantErrs, len(errs), errs)
			}
		})
	}
}

func TestLoadFromFile(t *testing.T) {
	// Create a temporary config file
	dir := t.TempDir()
	configPath := filepath.Join(dir, "vessel.toml")

	content := `
log_level = "info"
api_port = 9477

[[app]]
name = "test"
image = "nginx:latest"
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Apps[0].Name != "test" {
		t.Errorf("expected app name 'test', got %q", cfg.Apps[0].Name)
	}

	// Check defaults were applied
	if cfg.DataDir != DefaultDataDir {
		t.Errorf("expected default data_dir, got %q", cfg.DataDir)
	}

	if cfg.Apps[0].Instances != DefaultInstances {
		t.Errorf("expected default instances, got %d", cfg.Apps[0].Instances)
	}
}

func TestLoadFromFileMissing(t *testing.T) {
	_, err := Load("/nonexistent/vessel.toml")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestParseMemorySize(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
		wantErr  bool
	}{
		{"100B", 100, false},
		{"1KB", 1024, false},
		{"256MB", 256 * 1024 * 1024, false},
		{"1GB", 1024 * 1024 * 1024, false},
		{"2TB", 2 * 1024 * 1024 * 1024 * 1024, false},
		{"256mb", 256 * 1024 * 1024, false}, // lowercase
		{"invalid", 0, true},
		{"MB", 0, true},
		{"256", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := ParseMemorySize(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if result != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestDurationUnmarshal(t *testing.T) {
	toml := `
[[app]]
name = "test"
image = "nginx:latest"

[app.health_check]
type = "http"
path = "/"
port = 80
interval = "30s"
timeout = "5s"
retries = 3
`
	cfg, err := ParseBytes([]byte(toml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Apps[0].HealthCheck.Interval.Duration != 30*time.Second {
		t.Errorf("expected 30s interval, got %v", cfg.Apps[0].HealthCheck.Interval.Duration)
	}

	if cfg.Apps[0].HealthCheck.Timeout.Duration != 5*time.Second {
		t.Errorf("expected 5s timeout, got %v", cfg.Apps[0].HealthCheck.Timeout.Duration)
	}
}
