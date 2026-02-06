# CLAUDE.md — Vessel Project Master Document

> **This is the single source of truth for the Vessel project.**
> Every coding session MUST begin by reading this file in full.
> Every change MUST be consistent with the conventions defined here.
> If a decision isn't covered here, ASK before assuming.

---

## Project Overview

**Vessel** is a single-binary container orchestrator written in Go that deploys apps on any Linux server using raw Linux primitives (namespaces, cgroups v2, OverlayFS). It does NOT depend on Docker. It speaks OCI for image compatibility but implements its own container runtime from scratch.

**One-sentence pitch:** "Kubernetes is for Google. Vessel is for the rest of us."

**Repository:** `vessel` (Go module: `github.com/[owner]/vessel` — replace [owner] when repo is created)

---

## Critical Rules — Read Before Every Session

### 1. NEVER overwrite existing working code
- Before modifying ANY file, read the current contents first using `cat` or your file reading tool.
- If a file exists and has working code, you MUST use targeted edits (sed, patch, or specific line replacements). NEVER rewrite an entire file from scratch unless it is broken beyond repair or you are explicitly asked to.
- If you are adding a new function to an existing file, append it or insert it at the appropriate location. Do not rewrite the whole file.

### 2. NEVER create duplicate implementations
- Before creating a new file, check if a file for that purpose already exists in the directory structure.
- Before implementing a function, grep the codebase to see if it already exists: `grep -r "func SomeName" internal/`
- If similar functionality exists, extend it rather than creating a parallel implementation.

### 3. ALWAYS maintain interface contracts
- All component interactions go through explicitly defined Go interfaces (see Architecture section).
- If you need to change an interface, you MUST update ALL implementations and ALL callers in the same session.
- Never leave the codebase in a state where interfaces and implementations are out of sync.

### 4. ALWAYS run tests after changes
- After modifying any `.go` file, run `go build ./...` to verify compilation.
- After modifying any file in a package, run `go test ./internal/<package>/...` for that package.
- After any significant change, run `go vet ./...` and `golangci-lint run` if available.
- Never consider a task complete until tests pass.

### 5. ALWAYS update the progress tracker
- After completing any task, update `PROGRESS.md` to reflect what was done.
- Mark completed items, note any deviations from the plan, and document any new decisions.

### 6. Commit discipline
- Each logical unit of work should be a single commit.
- Commit messages follow conventional commits: `feat(runtime): implement PID namespace isolation`
- Prefixes: `feat`, `fix`, `refactor`, `test`, `docs`, `chore`
- Scopes: `runtime`, `manager`, `network`, `health`, `store`, `config`, `api`, `cli`, `dashboard`

---

## Architecture & Directory Structure

```
vessel/
├── CLAUDE.md                        # THIS FILE — master document
├── PROGRESS.md                      # Progress tracker — update after every session
├── ARCHITECTURE.md                  # Detailed architecture decisions log
├── cmd/
│   └── vessel/
│       └── main.go                  # CLI entrypoint — uses cobra
├── internal/
│   ├── runtime/                     # Container runtime (Linux primitives)
│   │   ├── container.go             # Container struct + lifecycle (Create/Start/Stop/Remove)
│   │   ├── namespaces.go            # Namespace creation and configuration
│   │   ├── cgroups.go               # cgroups v2 resource limit management
│   │   ├── filesystem.go            # OverlayFS mount/unmount, rootfs setup
│   │   ├── image.go                 # OCI image pull, unpack, layer storage
│   │   ├── seccomp.go               # Seccomp profile generation and loading
│   │   ├── init.go                  # Container init process (runs inside namespace)
│   │   └── runtime_test.go          # Integration tests for the runtime
│   ├── manager/                     # App lifecycle & deploy orchestration
│   │   ├── app.go                   # App struct, CRUD operations
│   │   ├── deploy.go                # Deploy strategies (rolling, blue-green)
│   │   ├── rollback.go              # Rollback logic
│   │   ├── reconciler.go            # Desired → actual state reconciliation loop
│   │   └── manager_test.go
│   ├── network/                     # Networking, proxy, TLS
│   │   ├── proxy.go                 # HTTP(S) reverse proxy with host-based routing
│   │   ├── tls.go                   # ACME / Let's Encrypt automatic certificates
│   │   ├── bridge.go                # Linux bridge, veth pairs, container networking
│   │   ├── dns.go                   # Internal DNS for container-to-container resolution
│   │   └── network_test.go
│   ├── health/                      # Health checking & monitoring
│   │   ├── checker.go               # Health check execution (HTTP, TCP, command)
│   │   ├── monitor.go               # Continuous monitoring loop with restart logic
│   │   ├── alerts.go                # Webhook alerting on failures
│   │   └── health_test.go
│   ├── store/                       # Persistent state (BBolt)
│   │   ├── db.go                    # BBolt wrapper — open, close, migrations
│   │   ├── models.go                # ALL data models (App, Container, Deploy, Secret, etc.)
│   │   ├── apps.go                  # App CRUD operations on the store
│   │   ├── containers.go            # Container state persistence
│   │   ├── deploys.go               # Deploy history persistence
│   │   ├── secrets.go               # Encrypted secret storage
│   │   └── store_test.go
│   ├── config/                      # Configuration parsing
│   │   ├── parser.go                # TOML parsing into Go structs
│   │   ├── validate.go              # Config validation rules
│   │   ├── defaults.go              # Default values for all optional fields
│   │   └── config_test.go
│   ├── api/                         # REST + WebSocket API
│   │   ├── server.go                # HTTP server setup, middleware chain
│   │   ├── routes.go                # Route registration
│   │   ├── handlers_apps.go         # App-related API handlers
│   │   ├── handlers_deploys.go      # Deploy-related API handlers
│   │   ├── handlers_logs.go         # Log streaming handlers
│   │   ├── handlers_metrics.go      # Metrics handlers
│   │   ├── websocket.go             # WebSocket upgrade and streaming
│   │   ├── middleware.go            # Auth, logging, CORS, rate limiting
│   │   └── api_test.go
│   ├── cli/                         # CLI command implementations
│   │   ├── root.go                  # Root cobra command, global flags
│   │   ├── deploy.go                # `vessel deploy`
│   │   ├── ps.go                    # `vessel ps`
│   │   ├── logs.go                  # `vessel logs`
│   │   ├── exec.go                  # `vessel exec`
│   │   ├── stop.go                  # `vessel stop`
│   │   ├── rm.go                    # `vessel rm`
│   │   ├── rollback.go              # `vessel rollback`
│   │   ├── history.go               # `vessel history`
│   │   ├── health.go                # `vessel health`
│   │   ├── stats.go                 # `vessel stats`
│   │   ├── secrets.go               # `vessel secret` subcommands
│   │   ├── init_cmd.go              # `vessel init`
│   │   ├── doctor.go                # `vessel doctor`
│   │   ├── daemon.go                # `vessel daemon` (starts the daemon)
│   │   └── fmt.go                   # `vessel fmt`
│   ├── metrics/                     # Resource metrics collection
│   │   ├── collector.go             # cgroup stats polling
│   │   └── store.go                 # Ring buffer time-series storage
│   └── daemon/                      # Daemon process management
│       ├── daemon.go                # Main daemon loop, signal handling
│       └── socket.go                # Unix socket for CLI ↔ daemon communication
├── dashboard/                       # React frontend (embedded in binary)
│   ├── src/
│   │   ├── App.tsx
│   │   ├── components/
│   │   └── hooks/
│   ├── package.json
│   ├── tsconfig.json
│   └── vite.config.ts
├── docs/                            # mdBook documentation
├── examples/                        # Example vessel.toml configs
├── scripts/
│   ├── install.sh
│   └── vessel.service               # Systemd unit file
├── testutil/                        # Shared test utilities
│   ├── helpers.go                   # Test helper functions
│   └── fixtures/                    # Test fixture files (configs, images)
├── Vagrantfile                      # Development VM
├── Makefile
├── go.mod
├── go.sum
└── README.md
```

### Rules for directory structure:
- **NEVER create files outside this structure** without updating this document first.
- **NEVER put implementation code in `cmd/`** — `cmd/vessel/main.go` only initializes and calls into `internal/cli`.
- **NEVER import between sibling packages horizontally** at the same level without going through defined interfaces. The dependency graph flows DOWN:
  ```
  cli → manager → runtime
  cli → api → manager → runtime
  cli → config
  manager → store
  manager → network
  manager → health
  health → runtime (read-only, for health checks)
  api → manager
  api → store (read-only, for queries)
  metrics → runtime (read-only, for stats)
  ```
- **NEVER let `runtime` import from `manager`, `api`, `cli`, or `network`**. The runtime is the lowest layer and has no upward dependencies.

---

## Data Models — Single Source of Truth

All data models live in `internal/store/models.go`. NOWHERE ELSE. Every other package imports from there.

```go
package store

import "time"

// AppState represents the current state of an application.
type AppState string

const (
    AppStateRunning  AppState = "running"
    AppStateStopped  AppState = "stopped"
    AppStateDeploying AppState = "deploying"
    AppStateFailed   AppState = "failed"
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
    DeployStatusPending  DeployStatus = "pending"
    DeployStatusActive   DeployStatus = "active"
    DeployStatusFailed   DeployStatus = "failed"
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
    ID          string     `json:"id"`
    Name        string     `json:"name"`
    State       AppState   `json:"state"`
    CreatedAt   time.Time  `json:"created_at"`
    UpdatedAt   time.Time  `json:"updated_at"`
}

// Container represents a single running (or stopped) container instance.
type Container struct {
    ID          string          `json:"id"`
    AppID       string          `json:"app_id"`
    Image       string          `json:"image"`       // Full image reference including digest
    ImageDigest string          `json:"image_digest"` // SHA256 digest
    State       ContainerState  `json:"state"`
    PID         int             `json:"pid"`          // Host PID of the container init process
    IP          string          `json:"ip"`           // Container IP on the vessel bridge
    Resources   ResourceLimits  `json:"resources"`
    CreatedAt   time.Time       `json:"created_at"`
    StartedAt   *time.Time      `json:"started_at,omitempty"`
    StoppedAt   *time.Time      `json:"stopped_at,omitempty"`
    ExitCode    *int            `json:"exit_code,omitempty"`
    RestartCount int            `json:"restart_count"`
}

// ResourceLimits defines the resource constraints for a container.
type ResourceLimits struct {
    CPUQuota   int64  `json:"cpu_quota"`    // Microseconds per period (e.g., 50000 for 50%)
    CPUPeriod  int64  `json:"cpu_period"`   // Period in microseconds (default 100000)
    MemoryMax  int64  `json:"memory_max"`   // Bytes
    SwapMax    int64  `json:"swap_max"`     // Bytes (-1 for same as memory)
    PidsMax    int64  `json:"pids_max"`     // Max number of processes
}

// Deploy records a single deploy event.
type Deploy struct {
    ID          string         `json:"id"`
    AppID       string         `json:"app_id"`
    Image       string         `json:"image"`
    ImageDigest string         `json:"image_digest"`
    Strategy    DeployStrategy `json:"strategy"`
    Status      DeployStatus   `json:"status"`
    Version     int            `json:"version"`       // Monotonically increasing per app
    CreatedAt   time.Time      `json:"created_at"`
    FinishedAt  *time.Time     `json:"finished_at,omitempty"`
    Error       string         `json:"error,omitempty"`
    RollbackOf  *int           `json:"rollback_of,omitempty"` // Version this rolled back from
}

// HealthCheck defines how to check if a container is healthy.
type HealthCheck struct {
    Type     HealthCheckType `json:"type"`
    Target   string          `json:"target"`    // URL path, TCP address, or command
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
    Value     []byte    `json:"-"`           // Encrypted, never serialized to JSON
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}

// MetricPoint is a single time-series data point for resource metrics.
type MetricPoint struct {
    Timestamp    time.Time `json:"timestamp"`
    CPUPercent   float64   `json:"cpu_percent"`
    MemoryBytes  int64     `json:"memory_bytes"`
    MemoryLimit  int64     `json:"memory_limit"`
    NetworkRxBytes int64   `json:"network_rx_bytes"`
    NetworkTxBytes int64   `json:"network_tx_bytes"`
    PidsCount    int64     `json:"pids_count"`
}
```

### Rules for data models:
- **ALL models live in `internal/store/models.go`**. No exceptions.
- **NEVER define shadow types** in other packages. If you need the type, import it from `store`.
- **NEVER use `map[string]interface{}`** where a typed struct should be used.
- **ALL JSON serialization tags must be defined** on every exported field.
- **Use pointer types for optional/nullable fields** (`*time.Time`, `*int`).
- **Use typed constants (iota or string constants) for all enums**, never raw strings.
- When you need to add a new model, add it to `models.go` and document it in this file.

---

## Interface Contracts

These are the core interfaces that define component boundaries. Implementations MUST satisfy these exactly.

```go
// internal/runtime/runtime.go
package runtime

type Runtime interface {
    // PullImage downloads an OCI image from a registry.
    PullImage(ctx context.Context, ref string) (digest string, err error)

    // CreateContainer sets up the filesystem and config for a new container.
    // It does NOT start the container.
    CreateContainer(ctx context.Context, opts ContainerOpts) (*store.Container, error)

    // StartContainer starts an existing container by forking + execing into namespaces.
    StartContainer(ctx context.Context, containerID string) error

    // StopContainer sends SIGTERM, waits for grace period, then SIGKILL.
    StopContainer(ctx context.Context, containerID string, gracePeriod time.Duration) error

    // RemoveContainer cleans up all resources (cgroups, filesystem, network).
    RemoveContainer(ctx context.Context, containerID string) error

    // ExecInContainer runs a command inside an existing container's namespaces.
    ExecInContainer(ctx context.Context, containerID string, cmd []string) (io.Reader, error)

    // ContainerLogs returns a reader for the container's stdout/stderr.
    ContainerLogs(ctx context.Context, containerID string, follow bool, tail int) (io.ReadCloser, error)

    // ContainerStats returns current resource usage for a container.
    ContainerStats(ctx context.Context, containerID string) (*store.MetricPoint, error)
}

// ContainerOpts configures a new container.
type ContainerOpts struct {
    AppID       string
    Image       string
    ImageDigest string
    Command     []string
    Env         map[string]string
    Resources   store.ResourceLimits
    Hostname    string
}
```

```go
// internal/manager/manager.go
package manager

type Manager interface {
    // DeployApp performs a full deploy: pull image, create container, health check, swap traffic.
    DeployApp(ctx context.Context, appCfg *config.AppConfig) (*store.Deploy, error)

    // StopApp stops all containers for an app.
    StopApp(ctx context.Context, appName string) error

    // RemoveApp stops and removes all containers and state for an app.
    RemoveApp(ctx context.Context, appName string) error

    // RollbackApp reverts to a previous deploy version.
    RollbackApp(ctx context.Context, appName string, version int) (*store.Deploy, error)

    // ListApps returns all known apps with their current state.
    ListApps(ctx context.Context) ([]*store.App, error)

    // GetApp returns a single app by name.
    GetApp(ctx context.Context, appName string) (*store.App, error)

    // GetContainers returns all containers for an app.
    GetContainers(ctx context.Context, appName string) ([]*store.Container, error)

    // GetDeployHistory returns deploy history for an app.
    GetDeployHistory(ctx context.Context, appName string) ([]*store.Deploy, error)

    // RestartApp restarts all containers for an app.
    RestartApp(ctx context.Context, appName string) error
}
```

```go
// internal/store/store.go
package store

type Store interface {
    // App operations
    CreateApp(app *App) error
    GetApp(name string) (*App, error)
    ListApps() ([]*App, error)
    UpdateApp(app *App) error
    DeleteApp(name string) error

    // Container operations
    CreateContainer(c *Container) error
    GetContainer(id string) (*Container, error)
    ListContainersByApp(appID string) ([]*Container, error)
    UpdateContainer(c *Container) error
    DeleteContainer(id string) error

    // Deploy operations
    CreateDeploy(d *Deploy) error
    GetDeploy(id string) (*Deploy, error)
    ListDeploysByApp(appID string) ([]*Deploy, error)
    GetLatestDeploy(appID string) (*Deploy, error)

    // Secret operations
    SetSecret(key string, encryptedValue []byte) error
    GetSecret(key string) (*Secret, error)
    ListSecretKeys() ([]string, error)
    DeleteSecret(key string) error

    // Lifecycle
    Close() error
}
```

```go
// internal/network/network.go
package network

type NetworkManager interface {
    // SetupContainerNetwork creates veth pair, assigns IP, attaches to bridge.
    SetupContainerNetwork(containerID string, pid int) (ip string, err error)

    // TeardownContainerNetwork removes networking for a container.
    TeardownContainerNetwork(containerID string) error

    // RegisterRoute adds a hostname → container mapping to the reverse proxy.
    RegisterRoute(hostname string, containerAddr string) error

    // DeregisterRoute removes a hostname → container mapping.
    DeregisterRoute(hostname string, containerAddr string) error

    // EnsureTLS provisions a TLS certificate for the hostname if needed.
    EnsureTLS(hostname string) error
}
```

```go
// internal/health/health.go
package health

type HealthMonitor interface {
    // RegisterCheck starts health checking for a container.
    RegisterCheck(containerID string, check store.HealthCheck) error

    // DeregisterCheck stops health checking for a container.
    DeregisterCheck(containerID string) error

    // GetStatus returns the current health status of a container.
    GetStatus(containerID string) (*store.HealthResult, error)

    // Subscribe returns a channel that receives health status changes.
    Subscribe(containerID string) (<-chan store.HealthResult, func())
}
```

### Rules for interfaces:
- **Interfaces are defined in the package that USES them**, not the package that implements them (Go convention).
- **NEVER add methods to an interface without updating all implementations.**
- **ALWAYS accept `context.Context` as the first parameter** for any operation that does I/O or could be cancelled.
- **ALWAYS return `error` as the last return value** for any fallible operation.
- **Mock implementations for testing go in `testutil/`** or as `_test.go` files in the consuming package.

---

## Coding Standards

### Go conventions
- **Go version:** 1.22+ (use latest stable)
- **Formatting:** `gofmt` (non-negotiable, enforced by CI)
- **Linting:** `golangci-lint` with default config + `gocritic`, `errcheck`, `gosec`
- **Naming:**
  - Packages: lowercase, single word when possible (`runtime`, `manager`, `store`)
  - Interfaces: verb-noun or noun (`Runtime`, `Store`, `NetworkManager`, `HealthMonitor`)
  - Implementations: descriptive (`LinuxRuntime`, `BoltStore`, `BridgeNetworkManager`)
  - Test files: `<package>_test.go` for unit tests, `<package>_integration_test.go` for integration tests
  - Test functions: `TestFunctionName_Scenario_Expected` (e.g., `TestCreateContainer_InvalidImage_ReturnsError`)
- **Error handling:**
  - ALWAYS wrap errors with context: `fmt.Errorf("failed to start container %s: %w", id, err)`
  - NEVER use `panic()` in library code. Only acceptable in `main.go` for truly unrecoverable situations.
  - Define package-level sentinel errors: `var ErrContainerNotFound = errors.New("container not found")`
  - Use `errors.Is()` and `errors.As()` for error checking, never string comparison.
- **Logging:**
  - Use `log/slog` (stdlib structured logging, available since Go 1.21)
  - Log levels: `Debug` (verbose internal state), `Info` (normal operations), `Warn` (recoverable issues), `Error` (failures)
  - ALWAYS include relevant context: `slog.Error("failed to stop container", "container_id", id, "error", err)`
  - NEVER use `fmt.Println` for operational logging. Only for CLI user-facing output.
- **Comments:**
  - Every exported type, function, and method MUST have a doc comment.
  - Comments explain WHY, not WHAT. The code shows what.
  - Use `// TODO(vessel):` for known improvements. Never leave uncommented TODOs.

### Configuration conventions
- Config format: TOML
- Config parsing: `github.com/BurntSushi/toml`
- Config structs live in `internal/config/`
- EVERY optional field has a default defined in `internal/config/defaults.go`
- Config validation happens in `internal/config/validate.go`, returns ALL errors at once (not fail-fast)

### Testing conventions
- Unit tests: test individual functions in isolation, use mocks for dependencies
- Integration tests: test real Linux primitives, guarded by build tag `//go:build integration`
- Table-driven tests: use for any function with multiple input/output scenarios
- Test helpers: shared utilities go in `testutil/helpers.go`
- NEVER test private functions directly. Test through the public API.
- Aim for tests that verify BEHAVIOR, not implementation details.

---

## Error Types

Define these in `internal/errors.go` (project-level error package):

```go
package vessel

import "errors"

// Sentinel errors used across the project.
var (
    // Runtime errors
    ErrContainerNotFound    = errors.New("container not found")
    ErrContainerNotRunning  = errors.New("container is not running")
    ErrContainerAlreadyRunning = errors.New("container is already running")
    ErrImagePullFailed      = errors.New("failed to pull image")
    ErrNamespaceSetupFailed = errors.New("failed to set up namespaces")
    ErrCgroupSetupFailed    = errors.New("failed to set up cgroups")
    ErrFilesystemSetupFailed = errors.New("failed to set up filesystem")

    // App errors
    ErrAppNotFound          = errors.New("app not found")
    ErrAppAlreadyExists     = errors.New("app already exists")
    ErrDeployInProgress     = errors.New("deploy already in progress")
    ErrNoDeployHistory      = errors.New("no deploy history found")
    ErrVersionNotFound      = errors.New("deploy version not found")

    // Config errors
    ErrInvalidConfig        = errors.New("invalid configuration")
    ErrConfigNotFound       = errors.New("vessel.toml not found")

    // Network errors
    ErrPortConflict         = errors.New("port already in use")
    ErrTLSProvisionFailed   = errors.New("failed to provision TLS certificate")

    // Store errors
    ErrStoreCorrupted       = errors.New("data store corrupted")
    ErrSecretNotFound       = errors.New("secret not found")
)
```

### Rules for errors:
- **ALL sentinel errors are defined in ONE place** (`internal/errors.go`).
- **NEVER create ad-hoc error strings** that duplicate sentinel errors. Use `fmt.Errorf("context: %w", vessel.ErrContainerNotFound)`.
- **Wrap errors at every layer** to build a clear error chain.

---

## Filesystem Paths

All paths used by Vessel at runtime:

```
/var/lib/vessel/                     # Root data directory
├── images/                          # OCI image layer store
│   └── <digest>/                    # Unpacked layers for one image
│       ├── layer-0/
│       ├── layer-1/
│       └── manifest.json
├── containers/                      # Container filesystems
│   └── <container-id>/
│       ├── lower/                   # Symlinks to image layers
│       ├── upper/                   # Writable layer
│       ├── work/                    # OverlayFS work directory
│       ├── merged/                  # OverlayFS merged mount point (the rootfs)
│       ├── config.json              # Container configuration
│       └── logs/
│           ├── stdout.log
│           └── stderr.log
├── state/
│   └── vessel.db                    # BBolt database
├── secrets/
│   └── vault.enc                    # Encrypted secrets file
├── tls/
│   └── <domain>/
│       ├── cert.pem
│       └── key.pem
└── run/
    └── vessel.sock                  # Unix socket for CLI ↔ daemon
```

### Rules for paths:
- **NEVER hardcode paths as string literals** scattered through code. Define them as constants in a `paths.go` file within the relevant package, or in a central `internal/paths/paths.go`.
- **ALWAYS use `filepath.Join()`**, never string concatenation for paths.
- **ALWAYS create directories with appropriate permissions** (0755 for dirs, 0644 for files, 0600 for secrets).

---

## CLI Framework

- Use `github.com/spf13/cobra` for CLI structure.
- Use `github.com/spf13/viper` for configuration binding (env vars, flags, config file).
- CLI output formatting:
  - Use `github.com/charmbracelet/lipgloss` for styled terminal output.
  - Tables: use `github.com/charmbracelet/lipgloss/table` or `github.com/olekukonez/tablewriter`.
  - Spinners/progress: use `github.com/charmbracelet/bubbles` spinner.
  - Colors: green for success, red for errors, yellow for warnings, blue for info.
- NEVER print raw struct output. Always format for humans.
- JSON output mode: every command supports `--json` flag for machine-readable output.
- Every command has:
  - A one-line `Short` description
  - A multi-line `Long` description with examples
  - `RunE` (not `Run`) to properly propagate errors
  - Proper flag definitions with shorthand, description, and defaults

---

## API Design

- REST API prefix: `/api/v1/`
- Use standard HTTP methods: GET (read), POST (create/action), PUT (replace), PATCH (partial update), DELETE (remove)
- Response format:
  ```json
  {
    "data": { ... },
    "error": null
  }
  ```
  or on error:
  ```json
  {
    "data": null,
    "error": {
      "code": "APP_NOT_FOUND",
      "message": "App 'my-api' not found"
    }
  }
  ```
- Use `net/http` standard library with a lightweight router (e.g., `chi` or Go 1.22's enhanced `ServeMux`).
- Prefer Go 1.22's `http.ServeMux` with method-based routing if available. Avoid heavy frameworks.
- WebSocket endpoints use `/ws/` prefix.
- Authentication: API key in `Authorization: Bearer <key>` header.

---

## Dependencies — Approved List

Only use these dependencies. If you need something not on this list, document why in ARCHITECTURE.md before adding it.

| Dependency | Purpose | Package |
|---|---|---|
| Cobra | CLI framework | `github.com/spf13/cobra` |
| Viper | Configuration | `github.com/spf13/viper` |
| BurntSushi/toml | TOML parsing | `github.com/BurntSushi/toml` |
| BBolt | Embedded KV store | `go.etcd.io/bbolt` |
| go-containerregistry | OCI image operations | `github.com/google/go-containerregistry` |
| lipgloss | Terminal styling | `github.com/charmbracelet/lipgloss` |
| slog | Structured logging | `log/slog` (stdlib) |
| autocert | ACME/TLS automation | `golang.org/x/crypto/acme/autocert` |
| golang.org/x/sys | Linux syscall wrappers | `golang.org/x/sys/unix` |
| golang.org/x/crypto | Argon2id + ACME autocert | `golang.org/x/crypto` |
| google/uuid | UUID generation | `github.com/google/uuid` |
| gorilla/websocket | WebSocket support | `github.com/gorilla/websocket` |
| miekg/dns | Internal DNS server | `github.com/miekg/dns` |

### Rules for dependencies:
- **Prefer stdlib** over third-party when the stdlib solution is adequate.
- **NEVER add a dependency for trivial functionality** (e.g., don't import a library for string utilities).
- **NEVER add a dependency without checking its license** (MIT, BSD, Apache 2.0 are OK).
- **Pin all dependency versions** in `go.mod`.

---

## Git & Branching

- Main branch: `main`
- Feature branches: `feat/<description>` (e.g., `feat/cgroups-v2-support`)
- Fix branches: `fix/<description>`
- Commit messages: conventional commits (see above)
- NEVER commit broken code to `main`. Every commit on `main` must compile and pass tests.

---

## Session Workflow

Every Claude Code session MUST follow this workflow:

1. **Read `CLAUDE.md`** (this file) in full.
2. **Read `PROGRESS.md`** to understand what's been done and what's next.
3. **Identify the task** for this session from the progress tracker or user instruction.
4. **Check existing code** before writing anything. `cat` or read any files you'll modify.
5. **Implement** following all conventions above.
6. **Test**: `go build ./...`, `go test ./...`, `go vet ./...`
7. **Update `PROGRESS.md`** with what was completed.
8. **Summarize** what was done and any decisions made.

---

## Current Phase

**PHASE 4 — Health Monitoring & Reliability** (Phases 0, 1, 2, and 3 COMPLETE)

See PROGRESS.md for detailed status.

## Architecture Decisions Log

### Secrets Architecture (Phase 2 Week 7)
- Secrets use **AES-256-GCM** encryption with **Argon2id** key derivation
- Argon2id params: time=1, memory=64MB, threads=4, key=32 bytes
- Salt generated once, persisted to `/var/lib/vessel/secrets/salt`
- Master password: `VESSEL_MASTER_PASSWORD` env var (default for dev)
- SecretManager wraps Store operations with encrypt/decrypt
- Secret references: `${secret:key-name}` syntax in env values
- Resolution happens during deploy (before image pull)

### Multi-App Config (Phase 2 Week 7)
- Uses `[[app]]` TOML array of tables syntax
- Per-app env via `[app.env]` sections
- `vessel deploy --all` deploys in config order
- `--continue-on-error` flag for fault-tolerant multi-app deploys
- Deploy summary shows version transitions and timing

### Environment Variable Merge Order (Phase 2 Week 7)
- Image config env (lowest priority)
- Config file env (`[env]` section in vessel.toml)
- .env file (`--env-file` flag)
- CLI flags (`--env KEY=VALUE`, highest priority)

### Deploy Strategies (Phase 2 Week 6)
- Rolling: replace containers one at a time with drain timeout
- Blue-Green: start all new, health check, atomic swap, remove old
- Deploy-time health checks separate from continuous monitoring (Phase 4)

### Container Networking (Phase 3)
- Linux bridge `vessel0` with `10.88.0.0/16` subnet, gateway `10.88.0.1`
- Veth pairs connect containers to bridge; host-side `veth-<8chars>`, container-side `eth0`
- IP allocator: sequential allocation, persists to `/var/lib/vessel/network/allocations.json`
- NAT via iptables MASQUERADE for outbound traffic
- Network setup is best-effort (deploy succeeds even if networking fails)
- Bridge persists across daemon restarts

### Reverse Proxy (Phase 3)
- Host-based routing using `net/http/httputil.NewSingleHostReverseProxy`
- Random load balancing across multiple container backends
- Routes registered after health check passes during deploy
- Routes deregistered BEFORE network teardown on stop
- Sets `X-Forwarded-Host`, `X-Forwarded-Proto`, `X-Real-IP` headers

### TLS (Phase 3)
- Automatic TLS via `golang.org/x/crypto/acme/autocert`
- Custom certificates checked first, ACME as fallback
- `hostPolicy` restricts ACME certs to registered route hostnames only
- HTTP server: health checks + ACME challenges + 301 redirect to HTTPS
- HTTPS server: TLS 1.2 minimum, `GetCertificate` callback
- `vessel cert list` / `vessel cert import` CLI commands

### Internal DNS (Phase 3)
- DNS server on `10.88.0.1:53` using `github.com/miekg/dns`
- `<app-name>.vessel.internal` resolves to container IPs (A records, 60s TTL)
- External queries forwarded to upstream DNS (default 8.8.8.8:53)
- Container `/etc/resolv.conf`: primary=10.88.0.1, secondary=8.8.8.8, search=vessel.internal
- DNS records updated after successful deploy; deregistered on app removal
