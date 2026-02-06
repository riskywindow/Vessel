package config

import (
	"strings"
	"testing"
)

func TestParseMultiAppConfig(t *testing.T) {
	toml := `
[[app]]
name = "frontend"
image = "myorg/frontend:v2.1"

[app.env]
NODE_ENV = "production"

[[app]]
name = "api"
image = "myorg/api:v3.0"

[app.env]
DATABASE_URL = "postgres://localhost/db"
LOG_LEVEL = "info"

[[app]]
name = "worker"
image = "myorg/worker:v3.0"
instances = 3
`
	cfg, err := ParseBytes([]byte(toml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.Apps) != 3 {
		t.Fatalf("expected 3 apps, got %d", len(cfg.Apps))
	}

	// Verify app names
	names := []string{"frontend", "api", "worker"}
	for i, name := range names {
		if cfg.Apps[i].Name != name {
			t.Errorf("app[%d]: expected name %q, got %q", i, name, cfg.Apps[i].Name)
		}
	}

	// Verify env vars
	if cfg.Apps[0].Env["NODE_ENV"] != "production" {
		t.Errorf("frontend: expected NODE_ENV=production, got %q", cfg.Apps[0].Env["NODE_ENV"])
	}
	if cfg.Apps[1].Env["DATABASE_URL"] != "postgres://localhost/db" {
		t.Errorf("api: expected DATABASE_URL, got %q", cfg.Apps[1].Env["DATABASE_URL"])
	}

	// Verify instances
	if cfg.Apps[2].Instances != 3 {
		t.Errorf("worker: expected 3 instances, got %d", cfg.Apps[2].Instances)
	}
}

func TestParseMultiApp_WithDefaults(t *testing.T) {
	toml := `
[[app]]
name = "a"
image = "img:v1"

[[app]]
name = "b"
image = "img:v2"
`
	cfg, err := ParseBytes([]byte(toml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ApplyDefaults(cfg)

	for i, app := range cfg.Apps {
		if app.Instances != DefaultInstances {
			t.Errorf("app[%d]: expected default instances %d, got %d", i, DefaultInstances, app.Instances)
		}
		if app.Resources.CPU != DefaultCPU {
			t.Errorf("app[%d]: expected default CPU %d, got %d", i, DefaultCPU, app.Resources.CPU)
		}
		if app.Resources.Memory != DefaultMemory {
			t.Errorf("app[%d]: expected default memory %q, got %q", i, DefaultMemory, app.Resources.Memory)
		}
		if app.Deploy.Strategy != DefaultDeployStrategy {
			t.Errorf("app[%d]: expected default strategy %q, got %q", i, DefaultDeployStrategy, app.Deploy.Strategy)
		}
	}
}

func TestValidateMultiApp_DuplicateNames(t *testing.T) {
	cfg := &Config{
		LogLevel: "info",
		APIPort:  9477,
		Apps: []AppConfig{
			{Name: "myapp", Image: "img:v1", Instances: 1, Resources: ResourceConfig{CPU: 100, Memory: "256MB", Pids: 100}, Deploy: validDeploy()},
			{Name: "myapp", Image: "img:v2", Instances: 1, Resources: ResourceConfig{CPU: 100, Memory: "256MB", Pids: 100}, Deploy: validDeploy()},
		},
	}

	errs := Validate(cfg)
	if len(errs) == 0 {
		t.Error("expected validation errors for duplicate app names, got none")
		return
	}
	found := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "duplicate app name") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected duplicate name error, got: %v", errs)
	}
}

func TestValidateMultiApp_MissingRequiredFields(t *testing.T) {
	cfg := &Config{
		LogLevel: "info",
		APIPort:  9477,
		Apps: []AppConfig{
			{Name: "a", Image: "", Instances: 1, Resources: ResourceConfig{CPU: 100, Memory: "256MB", Pids: 100}, Deploy: validDeploy()},
			{Name: "", Image: "img:v1", Instances: 1, Resources: ResourceConfig{CPU: 100, Memory: "256MB", Pids: 100}, Deploy: validDeploy()},
		},
	}

	errs := Validate(cfg)
	if len(errs) < 2 {
		t.Errorf("expected at least 2 errors (missing image + missing name), got %d: %v", len(errs), errs)
	}
}

func TestParseMultiApp_WithSecretRefs(t *testing.T) {
	toml := `
[[app]]
name = "a"
image = "img:v1"

[app.env]
API_KEY = "${secret:api-key}"
DB_URL = "${secret:db-url}"
PLAIN = "hello"
`
	cfg, err := ParseBytes([]byte(toml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	env := cfg.Apps[0].Env
	if env["API_KEY"] != "${secret:api-key}" {
		t.Errorf("expected secret ref preserved, got %q", env["API_KEY"])
	}
	if env["DB_URL"] != "${secret:db-url}" {
		t.Errorf("expected secret ref preserved, got %q", env["DB_URL"])
	}
	if env["PLAIN"] != "hello" {
		t.Errorf("expected PLAIN=hello, got %q", env["PLAIN"])
	}
}

func TestParseMultiApp_DifferentStrategies(t *testing.T) {
	toml := `
[[app]]
name = "a"
image = "img:v1"

[app.deploy]
strategy = "rolling"

[[app]]
name = "b"
image = "img:v2"

[app.deploy]
strategy = "blue-green"
`
	cfg, err := ParseBytes([]byte(toml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ApplyDefaults(cfg)

	if cfg.Apps[0].Deploy.Strategy != "rolling" {
		t.Errorf("app a: expected rolling, got %q", cfg.Apps[0].Deploy.Strategy)
	}
	if cfg.Apps[1].Deploy.Strategy != "blue-green" {
		t.Errorf("app b: expected blue-green, got %q", cfg.Apps[1].Deploy.Strategy)
	}
}

// validDeploy returns a valid DeployConfig for use in tests.
func validDeploy() DeployConfig {
	return DeployConfig{
		Strategy:           "rolling",
		GracePeriod:        Duration{30e9}, // 30s
		HealthyThreshold:   2,
		UnhealthyThreshold: 3,
	}
}
