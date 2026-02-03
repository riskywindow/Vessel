package store

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	vessel "github.com/vessel/vessel/internal"
)

func setupTestStore(t *testing.T) *BoltStore {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open test store: %v", err)
	}

	t.Cleanup(func() {
		store.Close()
	})

	return store
}

// App tests

func TestAppCRUD(t *testing.T) {
	store := setupTestStore(t)

	// Create
	app := &App{
		ID:    "app-1",
		Name:  "myapp",
		State: AppStateRunning,
	}

	if err := store.CreateApp(app); err != nil {
		t.Fatalf("failed to create app: %v", err)
	}

	// Verify timestamps were set
	if app.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
	if app.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be set")
	}

	// Get
	retrieved, err := store.GetApp("myapp")
	if err != nil {
		t.Fatalf("failed to get app: %v", err)
	}

	if retrieved.ID != app.ID {
		t.Errorf("expected ID %q, got %q", app.ID, retrieved.ID)
	}
	if retrieved.Name != app.Name {
		t.Errorf("expected Name %q, got %q", app.Name, retrieved.Name)
	}
	if retrieved.State != app.State {
		t.Errorf("expected State %q, got %q", app.State, retrieved.State)
	}

	// Update
	retrieved.State = AppStateStopped
	if err := store.UpdateApp(retrieved); err != nil {
		t.Fatalf("failed to update app: %v", err)
	}

	updated, err := store.GetApp("myapp")
	if err != nil {
		t.Fatalf("failed to get updated app: %v", err)
	}
	if updated.State != AppStateStopped {
		t.Errorf("expected State %q, got %q", AppStateStopped, updated.State)
	}

	// List
	apps, err := store.ListApps()
	if err != nil {
		t.Fatalf("failed to list apps: %v", err)
	}
	if len(apps) != 1 {
		t.Errorf("expected 1 app, got %d", len(apps))
	}

	// Delete
	if err := store.DeleteApp("myapp"); err != nil {
		t.Fatalf("failed to delete app: %v", err)
	}

	// Verify deletion
	_, err = store.GetApp("myapp")
	if !errors.Is(err, vessel.ErrAppNotFound) {
		t.Errorf("expected ErrAppNotFound, got %v", err)
	}
}

func TestCreateAppDuplicate(t *testing.T) {
	store := setupTestStore(t)

	app := &App{
		ID:    "app-1",
		Name:  "myapp",
		State: AppStateRunning,
	}

	if err := store.CreateApp(app); err != nil {
		t.Fatalf("failed to create app: %v", err)
	}

	// Try to create duplicate
	err := store.CreateApp(app)
	if !errors.Is(err, vessel.ErrAppAlreadyExists) {
		t.Errorf("expected ErrAppAlreadyExists, got %v", err)
	}
}

func TestGetAppNotFound(t *testing.T) {
	store := setupTestStore(t)

	_, err := store.GetApp("nonexistent")
	if !errors.Is(err, vessel.ErrAppNotFound) {
		t.Errorf("expected ErrAppNotFound, got %v", err)
	}
}

func TestUpdateAppNotFound(t *testing.T) {
	store := setupTestStore(t)

	app := &App{
		ID:    "app-1",
		Name:  "nonexistent",
		State: AppStateRunning,
	}

	err := store.UpdateApp(app)
	if !errors.Is(err, vessel.ErrAppNotFound) {
		t.Errorf("expected ErrAppNotFound, got %v", err)
	}
}

func TestDeleteAppNotFound(t *testing.T) {
	store := setupTestStore(t)

	err := store.DeleteApp("nonexistent")
	if !errors.Is(err, vessel.ErrAppNotFound) {
		t.Errorf("expected ErrAppNotFound, got %v", err)
	}
}

// Container tests

func TestContainerCRUD(t *testing.T) {
	store := setupTestStore(t)

	// Create
	container := &Container{
		ID:          "container-1",
		AppID:       "app-1",
		Image:       "nginx:latest",
		ImageDigest: "sha256:abc123",
		State:       ContainerStateRunning,
		PID:         12345,
		IP:          "10.0.0.2",
		Resources: ResourceLimits{
			CPUQuota:  50000,
			CPUPeriod: 100000,
			MemoryMax: 256 * 1024 * 1024,
			PidsMax:   100,
		},
	}

	if err := store.CreateContainer(container); err != nil {
		t.Fatalf("failed to create container: %v", err)
	}

	// Verify timestamp was set
	if container.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}

	// Get
	retrieved, err := store.GetContainer("container-1")
	if err != nil {
		t.Fatalf("failed to get container: %v", err)
	}

	if retrieved.ID != container.ID {
		t.Errorf("expected ID %q, got %q", container.ID, retrieved.ID)
	}
	if retrieved.AppID != container.AppID {
		t.Errorf("expected AppID %q, got %q", container.AppID, retrieved.AppID)
	}
	if retrieved.PID != container.PID {
		t.Errorf("expected PID %d, got %d", container.PID, retrieved.PID)
	}

	// Update
	now := time.Now().UTC()
	retrieved.State = ContainerStateStopped
	retrieved.StoppedAt = &now
	exitCode := 0
	retrieved.ExitCode = &exitCode

	if err := store.UpdateContainer(retrieved); err != nil {
		t.Fatalf("failed to update container: %v", err)
	}

	updated, err := store.GetContainer("container-1")
	if err != nil {
		t.Fatalf("failed to get updated container: %v", err)
	}
	if updated.State != ContainerStateStopped {
		t.Errorf("expected State %q, got %q", ContainerStateStopped, updated.State)
	}
	if updated.ExitCode == nil || *updated.ExitCode != 0 {
		t.Errorf("expected ExitCode 0, got %v", updated.ExitCode)
	}

	// List by app
	containers, err := store.ListContainersByApp("app-1")
	if err != nil {
		t.Fatalf("failed to list containers: %v", err)
	}
	if len(containers) != 1 {
		t.Errorf("expected 1 container, got %d", len(containers))
	}

	// List by different app (should be empty)
	containers, err = store.ListContainersByApp("app-2")
	if err != nil {
		t.Fatalf("failed to list containers: %v", err)
	}
	if len(containers) != 0 {
		t.Errorf("expected 0 containers, got %d", len(containers))
	}

	// Delete
	if err := store.DeleteContainer("container-1"); err != nil {
		t.Fatalf("failed to delete container: %v", err)
	}

	// Verify deletion
	_, err = store.GetContainer("container-1")
	if !errors.Is(err, vessel.ErrContainerNotFound) {
		t.Errorf("expected ErrContainerNotFound, got %v", err)
	}
}

func TestGetContainerNotFound(t *testing.T) {
	store := setupTestStore(t)

	_, err := store.GetContainer("nonexistent")
	if !errors.Is(err, vessel.ErrContainerNotFound) {
		t.Errorf("expected ErrContainerNotFound, got %v", err)
	}
}

// Deploy tests

func TestDeployCRUD(t *testing.T) {
	store := setupTestStore(t)

	// Create
	deploy := &Deploy{
		ID:          "deploy-1",
		AppID:       "app-1",
		Image:       "myapp:v1",
		ImageDigest: "sha256:abc123",
		Strategy:    DeployStrategyRolling,
		Status:      DeployStatusPending,
		Version:     1,
	}

	if err := store.CreateDeploy(deploy); err != nil {
		t.Fatalf("failed to create deploy: %v", err)
	}

	// Verify timestamp was set
	if deploy.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}

	// Get
	retrieved, err := store.GetDeploy("deploy-1")
	if err != nil {
		t.Fatalf("failed to get deploy: %v", err)
	}

	if retrieved.ID != deploy.ID {
		t.Errorf("expected ID %q, got %q", deploy.ID, retrieved.ID)
	}
	if retrieved.Version != deploy.Version {
		t.Errorf("expected Version %d, got %d", deploy.Version, retrieved.Version)
	}

	// Create second deploy
	deploy2 := &Deploy{
		ID:          "deploy-2",
		AppID:       "app-1",
		Image:       "myapp:v2",
		ImageDigest: "sha256:def456",
		Strategy:    DeployStrategyRolling,
		Status:      DeployStatusActive,
		Version:     2,
	}

	if err := store.CreateDeploy(deploy2); err != nil {
		t.Fatalf("failed to create deploy 2: %v", err)
	}

	// List by app (should be sorted by version descending)
	deploys, err := store.ListDeploysByApp("app-1")
	if err != nil {
		t.Fatalf("failed to list deploys: %v", err)
	}
	if len(deploys) != 2 {
		t.Errorf("expected 2 deploys, got %d", len(deploys))
	}
	if deploys[0].Version != 2 {
		t.Errorf("expected first deploy version 2, got %d", deploys[0].Version)
	}

	// Get latest
	latest, err := store.GetLatestDeploy("app-1")
	if err != nil {
		t.Fatalf("failed to get latest deploy: %v", err)
	}
	if latest.Version != 2 {
		t.Errorf("expected latest version 2, got %d", latest.Version)
	}

	// Get next version
	nextVersion, err := store.GetNextDeployVersion("app-1")
	if err != nil {
		t.Fatalf("failed to get next version: %v", err)
	}
	if nextVersion != 3 {
		t.Errorf("expected next version 3, got %d", nextVersion)
	}
}

func TestGetLatestDeployNoHistory(t *testing.T) {
	store := setupTestStore(t)

	_, err := store.GetLatestDeploy("nonexistent")
	if !errors.Is(err, vessel.ErrNoDeployHistory) {
		t.Errorf("expected ErrNoDeployHistory, got %v", err)
	}
}

func TestGetNextDeployVersionNoHistory(t *testing.T) {
	store := setupTestStore(t)

	version, err := store.GetNextDeployVersion("new-app")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if version != 1 {
		t.Errorf("expected version 1, got %d", version)
	}
}

// Secret tests

func TestSecretCRUD(t *testing.T) {
	store := setupTestStore(t)

	// Set
	if err := store.SetSecret("API_KEY", []byte("encrypted-value")); err != nil {
		t.Fatalf("failed to set secret: %v", err)
	}

	// Get
	secret, err := store.GetSecret("API_KEY")
	if err != nil {
		t.Fatalf("failed to get secret: %v", err)
	}

	if secret.Key != "API_KEY" {
		t.Errorf("expected key 'API_KEY', got %q", secret.Key)
	}
	if string(secret.Value) != "encrypted-value" {
		t.Errorf("expected value 'encrypted-value', got %q", secret.Value)
	}
	if secret.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
	if secret.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be set")
	}

	// Update
	time.Sleep(10 * time.Millisecond) // Ensure different timestamp
	if err := store.SetSecret("API_KEY", []byte("new-encrypted-value")); err != nil {
		t.Fatalf("failed to update secret: %v", err)
	}

	updated, err := store.GetSecret("API_KEY")
	if err != nil {
		t.Fatalf("failed to get updated secret: %v", err)
	}
	if string(updated.Value) != "new-encrypted-value" {
		t.Errorf("expected updated value, got %q", updated.Value)
	}
	// CreatedAt should be preserved
	if !updated.CreatedAt.Equal(secret.CreatedAt) {
		t.Errorf("expected CreatedAt to be preserved")
	}

	// List keys
	keys, err := store.ListSecretKeys()
	if err != nil {
		t.Fatalf("failed to list secret keys: %v", err)
	}
	if len(keys) != 1 {
		t.Errorf("expected 1 key, got %d", len(keys))
	}
	if keys[0] != "API_KEY" {
		t.Errorf("expected key 'API_KEY', got %q", keys[0])
	}

	// Delete
	if err := store.DeleteSecret("API_KEY"); err != nil {
		t.Fatalf("failed to delete secret: %v", err)
	}

	// Verify deletion
	_, err = store.GetSecret("API_KEY")
	if !errors.Is(err, vessel.ErrSecretNotFound) {
		t.Errorf("expected ErrSecretNotFound, got %v", err)
	}
}

func TestGetSecretNotFound(t *testing.T) {
	store := setupTestStore(t)

	_, err := store.GetSecret("nonexistent")
	if !errors.Is(err, vessel.ErrSecretNotFound) {
		t.Errorf("expected ErrSecretNotFound, got %v", err)
	}
}

func TestDeleteSecretNotFound(t *testing.T) {
	store := setupTestStore(t)

	err := store.DeleteSecret("nonexistent")
	if !errors.Is(err, vessel.ErrSecretNotFound) {
		t.Errorf("expected ErrSecretNotFound, got %v", err)
	}
}

// Database lifecycle tests

func TestOpenCreateDirectory(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "nested", "dir", "test.db")

	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open store with nested path: %v", err)
	}
	defer store.Close()
}

func TestReopenDatabase(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	// Create and populate
	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open store: %v", err)
	}

	app := &App{ID: "app-1", Name: "test", State: AppStateRunning}
	if err := store.CreateApp(app); err != nil {
		t.Fatalf("failed to create app: %v", err)
	}
	store.Close()

	// Reopen and verify
	store, err = Open(dbPath)
	if err != nil {
		t.Fatalf("failed to reopen store: %v", err)
	}
	defer store.Close()

	retrieved, err := store.GetApp("test")
	if err != nil {
		t.Fatalf("failed to get app after reopen: %v", err)
	}
	if retrieved.ID != "app-1" {
		t.Errorf("expected ID 'app-1', got %q", retrieved.ID)
	}
}
