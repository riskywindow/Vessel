package manager

import (
	"context"
	"testing"

	"github.com/vessel/vessel/internal/config"
	"github.com/vessel/vessel/internal/store"
	"github.com/vessel/vessel/testutil"
)

func TestResolveSecretRefs_NoRefs(t *testing.T) {
	env := map[string]string{
		"PLAIN": "hello",
		"NUM":   "42",
	}

	result, err := ResolveSecretRefs(env, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result["PLAIN"] != "hello" {
		t.Errorf("expected PLAIN=hello, got %q", result["PLAIN"])
	}
	if result["NUM"] != "42" {
		t.Errorf("expected NUM=42, got %q", result["NUM"])
	}
}

func TestResolveSecretRefs_WithRefs(t *testing.T) {
	st := testutil.TempStore(t)

	sm, err := store.NewSecretManager(st, "test-password", nil)
	if err != nil {
		t.Fatalf("failed to create SecretManager: %v", err)
	}

	sm.SetSecret("api-key", "sk-12345")
	sm.SetSecret("db-password", "p@ssw0rd")

	env := map[string]string{
		"API_KEY":      "${secret:api-key}",
		"DB_PASSWORD":  "${secret:db-password}",
		"PLAIN":        "hello",
		"MIXED":        "prefix-${secret:api-key}-suffix",
	}

	result, err := ResolveSecretRefs(env, sm)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result["API_KEY"] != "sk-12345" {
		t.Errorf("expected API_KEY=sk-12345, got %q", result["API_KEY"])
	}
	if result["DB_PASSWORD"] != "p@ssw0rd" {
		t.Errorf("expected DB_PASSWORD=p@ssw0rd, got %q", result["DB_PASSWORD"])
	}
	if result["PLAIN"] != "hello" {
		t.Errorf("expected PLAIN=hello, got %q", result["PLAIN"])
	}
	if result["MIXED"] != "prefix-sk-12345-suffix" {
		t.Errorf("expected MIXED=prefix-sk-12345-suffix, got %q", result["MIXED"])
	}
}

func TestResolveSecretRefs_SecretNotFound(t *testing.T) {
	st := testutil.TempStore(t)

	sm, err := store.NewSecretManager(st, "test-password", nil)
	if err != nil {
		t.Fatalf("failed to create SecretManager: %v", err)
	}

	env := map[string]string{
		"KEY": "${secret:nonexistent}",
	}

	_, err = ResolveSecretRefs(env, sm)
	if err == nil {
		t.Error("expected error for nonexistent secret")
	}
}

func TestResolveSecretRefs_NoSecretManager(t *testing.T) {
	env := map[string]string{
		"KEY": "${secret:api-key}",
	}

	_, err := ResolveSecretRefs(env, nil)
	if err == nil {
		t.Error("expected error when secret manager is nil")
	}
}

func TestResolveSecretRefs_EmptyEnv(t *testing.T) {
	result, err := ResolveSecretRefs(nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil for nil input, got %v", result)
	}
}

func TestDeployWithSecrets(t *testing.T) {
	rt := &mockRuntime{}
	mgr, st := newTestManager(t, rt)

	// Create a secret manager and wire it in
	sm, err := store.NewSecretManager(st, "test-password", nil)
	if err != nil {
		t.Fatalf("failed to create SecretManager: %v", err)
	}
	mgr.SetSecretManager(sm)

	// Set a secret
	sm.SetSecret("my-api-key", "sk-test-12345")

	// Create an app config with secret reference
	appCfg := newTestAppConfig("secret-app", "alpine:latest")
	appCfg.Env = map[string]string{
		"API_KEY":   "${secret:my-api-key}",
		"PLAIN_VAR": "hello",
	}

	// Deploy should succeed and resolve secrets
	deploy, err := mgr.DeployAppFromConfig(context.Background(), appCfg)
	if err != nil {
		t.Fatalf("deploy failed: %v", err)
	}

	if deploy.Status != store.DeployStatusActive {
		t.Errorf("expected active deploy, got %q", deploy.Status)
	}

	// Verify the app was stored with resolved env vars
	app, err := st.GetApp("secret-app")
	if err != nil {
		t.Fatalf("failed to get app: %v", err)
	}

	// The env should have resolved secret values
	if app.Env["API_KEY"] != "sk-test-12345" {
		t.Errorf("expected resolved API_KEY=sk-test-12345, got %q", app.Env["API_KEY"])
	}
	if app.Env["PLAIN_VAR"] != "hello" {
		t.Errorf("expected PLAIN_VAR=hello, got %q", app.Env["PLAIN_VAR"])
	}
}

func TestDeployWithSecrets_MissingSecret(t *testing.T) {
	rt := &mockRuntime{}
	mgr, st := newTestManager(t, rt)

	sm, err := store.NewSecretManager(st, "test-password", nil)
	if err != nil {
		t.Fatalf("failed to create SecretManager: %v", err)
	}
	mgr.SetSecretManager(sm)

	// Config references a secret that doesn't exist
	appCfg := newTestAppConfig("fail-app", "alpine:latest")
	appCfg.Env = map[string]string{
		"KEY": "${secret:nonexistent}",
	}

	_, err = mgr.DeployAppFromConfig(context.Background(), appCfg)
	if err == nil {
		t.Error("expected error for missing secret reference")
	}
}

func TestMergeEnvOrder(t *testing.T) {
	// Test that merge order is correct: image < config < .env < CLI flags
	imageEnv := map[string]string{
		"A": "image",
		"B": "image",
		"C": "image",
		"D": "image",
	}

	configEnv := map[string]string{
		"B": "config",
		"C": "config",
		"D": "config",
	}

	envFileEnv := map[string]string{
		"C": "envfile",
		"D": "envfile",
	}

	cliEnv := map[string]string{
		"D": "cli",
	}

	result := config.MergeEnv(imageEnv, configEnv, envFileEnv, cliEnv)

	if result["A"] != "image" {
		t.Errorf("A: expected 'image', got %q", result["A"])
	}
	if result["B"] != "config" {
		t.Errorf("B: expected 'config', got %q", result["B"])
	}
	if result["C"] != "envfile" {
		t.Errorf("C: expected 'envfile', got %q", result["C"])
	}
	if result["D"] != "cli" {
		t.Errorf("D: expected 'cli', got %q", result["D"])
	}
}
