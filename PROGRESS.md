# PROGRESS.md — Vessel Development Progress

> **Updated after every Claude Code session.**
> This file tracks what has been completed, what's in progress, and what's next.

---

## Current Phase: PHASE 2 — App Manager & Daemon

### Status: IN PROGRESS

---

## Phase 0: Foundations (Week 1) ✅ COMPLETE

- [x] Initialize Go module (`github.com/vessel/vessel`)
- [x] Create full directory structure (all `internal/` packages with placeholder files)
- [x] Create `Makefile` with targets: `build`, `test`, `lint`, `clean`, `install`
- [ ] Create `Vagrantfile` for Ubuntu 24.04 dev VM with cgroups v2
- [x] Implement data models in `internal/store/models.go`
- [x] Implement sentinel errors in `internal/errors.go`
- [x] Implement filesystem path constants in `internal/paths/paths.go`
- [x] Set up cobra CLI skeleton in `cmd/vessel/main.go` and `internal/cli/root.go`
- [x] Implement config structs and TOML parsing in `internal/config/`
- [x] Implement config defaults in `internal/config/defaults.go`
- [x] Implement config validation in `internal/config/validate.go`
- [x] Write initial README.md with vision statement and placeholder sections
- [ ] Set up GitHub Actions CI: lint, test, build (linux/amd64, linux/arm64)
- [x] Implement BBolt store wrapper in `internal/store/db.go`
- [x] Implement App CRUD in `internal/store/apps.go`
- [x] Implement Container CRUD in `internal/store/containers.go`
- [x] Implement Deploy CRUD in `internal/store/deploys.go`
- [x] Write tests for store package
- [x] Write tests for config package

## Phase 1: Container Runtime (Weeks 2-4) ✅ COMPLETE

### Week 2: Process Isolation with Namespaces ✅
- [x] Implement namespace creation with clone flags (PID, Mount, UTS, IPC, Network)
- [x] Implement UTS namespace (hostname isolation)
- [x] Implement PID namespace with /proc mount
- [x] Implement pivot_root for filesystem isolation
- [x] Set up basic /dev nodes (null, zero, random, urandom, tty, pts, ptmx)
- [x] Implement container init process (`internal/runtime/init.go`)
- [x] Write tests verifying isolation properties

### Week 3: Filesystem and OCI Images ✅
- [x] Implement OCI image pull using go-containerregistry
- [x] Implement layer unpacking and local storage
- [x] Implement OverlayFS mount/unmount
- [x] Implement container storage lifecycle
- [x] Test: pull alpine:latest, run a command in full isolation
- [x] Implement `vessel images` command

### Week 4: cgroups v2, Resource Limits, and Lifecycle ✅
- [x] Implement cgroup v2 hierarchy creation
- [x] Implement CPU limits (cpu.max)
- [x] Implement memory limits (memory.max, memory.swap.max)
- [x] Implement PID limits (pids.max)
- [x] Implement full container lifecycle (create → start → stop → remove)
- [x] Implement `vessel run <image>` CLI command
- [x] Write integration tests for resource limits

### Week 4: Security ✅
- [x] Implement seccomp BPF filter (blocks 17 dangerous syscalls)
- [x] Multi-architecture support (x86_64 + ARM64 syscall numbers)
- [x] Implement capability dropping (Docker-compatible default set)
- [x] NO_NEW_PRIVS enforcement

## Phase 2: App Manager & Deploys (Weeks 5-7) — IN PROGRESS

### Week 5: State Management and Basic App Lifecycle ✅
- [x] Implement Vessel daemon (internal/daemon/daemon.go)
- [x] Implement Unix socket communication (internal/daemon/socket.go)
- [x] Implement daemon client (internal/daemon/client.go)
- [x] Implement reconciliation loop (internal/manager/reconciler.go)
- [x] Implement app manager (internal/manager/app.go)
- [x] Implement `vessel ps` — list running apps
- [x] Implement `vessel stop` and `vessel rm`
- [x] Implement `vessel logs` — tail container stdout/stderr
- [x] Implement `vessel daemon` — start daemon process
- [x] Write unit tests for manager (21 tests)
- [x] Write unit tests for daemon socket communication (10 tests)

### Week 6: Zero-Downtime Deploys and Rollback ✅
- [x] Implement `vessel deploy` — full flow from config to running container
- [x] Implement rolling deploy strategy
- [x] Implement blue-green deploy strategy
- [x] Implement deploy history tracking
- [x] Implement `vessel rollback`
- [x] Implement `vessel history`

### Week 7: Environment, Secrets, and Multi-App
- [ ] Implement environment variable injection
- [ ] Implement encrypted secret store
- [ ] Implement `vessel secret` subcommands
- [ ] Support multiple apps in a single vessel.toml
- [ ] Implement `vessel deploy --all`

## Phase 3: Networking & TLS (Weeks 8-9) — NOT STARTED

### Week 8: Reverse Proxy and Routing
- [ ] Implement HTTP reverse proxy with host-based routing
- [ ] Implement load balancing across instances
- [ ] Implement WebSocket proxying
- [ ] Implement container networking (bridge, veth pairs, NAT)
- [ ] Implement internal DNS

### Week 9: Automatic TLS
- [ ] Implement ACME client for Let's Encrypt
- [ ] Automatic certificate provisioning on deploy
- [ ] Certificate renewal
- [ ] HTTP → HTTPS redirect
- [ ] Custom certificate support

## Phase 4: Health Monitoring & Reliability (Week 10) — NOT STARTED
- [ ] Implement HTTP health checks
- [ ] Implement TCP health checks
- [ ] Implement command health checks
- [ ] Auto-restart with exponential backoff
- [ ] Resource monitoring (cgroup stats)
- [ ] Time-series metrics storage
- [ ] Webhook alerting
- [ ] Implement `vessel health`

## Phase 5: CLI Polish & Remote Deploy (Week 11) — NOT STARTED
- [ ] Styled terminal output (lipgloss)
- [ ] Progress indicators for image pulls and deploys
- [ ] `vessel init` — interactive setup
- [ ] `vessel exec` — run command inside container
- [ ] `vessel ssh` — remote deploy via SSH
- [ ] Shell autocomplete (bash, zsh, fish)
- [ ] `vessel doctor` — system prerequisites check
- [ ] `vessel fmt` — config validation and formatting

## Phase 6: Web Dashboard (Weeks 12-13) — NOT STARTED

### Week 12: Backend API + Dashboard Skeleton
- [ ] REST API for all operations
- [ ] WebSocket endpoints for logs and metrics
- [ ] API key authentication
- [ ] React + TypeScript + Tailwind scaffold
- [ ] App list view
- [ ] Embed frontend into Go binary

### Week 13: Dashboard Features
- [ ] Real-time log viewer
- [ ] Resource usage graphs
- [ ] Deploy history timeline with rollback
- [ ] App detail view
- [ ] Browser-based terminal (xterm.js)
- [ ] Dark mode, responsive layout

## Phase 7: Hardening & Ship (Week 14) — NOT STARTED
- [ ] Security audit (namespace escape, seccomp)
- [ ] Default seccomp profile
- [ ] Capability dropping
- [ ] Multi-distro testing
- [ ] One-line install script
- [ ] Systemd service file
- [ ] Documentation site (mdBook)
- [ ] Demo GIF/video
- [ ] Launch blog post
- [ ] Example apps
- [ ] GitHub Release with binaries
- [ ] Launch on HN, Reddit, Lobsters

---

## Session Log

### Session 1 — 2026-02-03
**Task:** Phase 0 — Project initialization

**Completed:**
- Initialized Go module as `github.com/vessel/vessel`
- Created full directory structure with all `internal/` packages
- Created Makefile with all required targets
- Implemented all data models, sentinel errors, path constants
- Implemented config system (types, parser, defaults, validation)
- Implemented BBolt store with full CRUD
- Set up cobra CLI skeleton with 15 subcommands
- Created README, examples, install script, systemd service
- Fixed Secret storage (internal `secretStorage` type for db persistence)

### Sessions 2-4 — Phase 1 (Container Runtime)
**Task:** Implement container runtime using Linux primitives

**Completed:**
- Container lifecycle (Create/Start/Stop/Remove) in `container.go` (720 lines)
- Namespace creation (PID, Mount, UTS, IPC, Network) in `namespaces.go` (149 lines)
- Container init process with pivot_root in `init.go` (271 lines)
- OCI image pulling via go-containerregistry in `image.go` (481 lines)
- OverlayFS filesystem management in `filesystem.go` (447 lines)
- cgroups v2 resource limits in `cgroups.go` (380 lines)
- Seccomp BPF filter in `seccomp.go` (156 lines)
- Capability dropping in `capabilities.go` (176 lines)
- `vessel run` command (223 lines)
- `vessel images` command (114 lines)
- `vessel stats` command (82 lines)
- Integration test suite (1000 lines, 13+ tests)

### Session 5 — 2026-02-05
**Task:** Phase 2 Start — Environment verification and Phase 2 implementation

**Environment:** OrbStack VM (Ubuntu 24.04, aarch64 kernel, amd64 Go via Rosetta)

**Completed:**
- Installed Go 1.24.13
- Verified cgroups v2, namespace support
- Fixed seccomp for multi-architecture (ARM64 syscall numbers + graceful degradation on Rosetta)
- Removed PROGRESS.md and CLAUDE.md from .gitignore
- Verified all Phase 1 tests: 7/7 manual tests, 16/16 integration tests
- Reconstructed PROGRESS.md with accurate Phase 1 status
- Started Phase 2 implementation

**Decisions:**
- Seccomp gracefully degrades on emulated architectures (logs warning, continues)
- Added ARM64 syscall number table alongside x86_64

### Session 5 (continued) — 2026-02-05
**Task:** Phase 2 — Daemon, Manager, Reconciler, CLI, Socket, Tests

**Completed:**
- Updated App model in store/models.go (added Image, Command, Env, Instances, Resources fields)
- Implemented `internal/manager/app.go` (~387 lines):
  - AppManager with ListApps, GetApp, GetContainers, StopApp, RemoveApp, RestartApp
  - DeployApp (basic replace strategy), RollbackApp (stub), GetDeployHistory
- Implemented `internal/manager/reconciler.go` (~365 lines):
  - Reconciler with Run loop, ReconcileApp, exponential backoff (1s→5min cap)
  - Process alive detection via /proc/<pid>, container restart logic
  - stopExcessContainers (oldest first), ensureNoRunning for stopped apps
- Implemented `internal/daemon/daemon.go` (~223 lines):
  - Daemon struct with NewDaemon, Start, Stop, signal handling
  - Socket listener, reconciler goroutine, graceful shutdown (30s timeout)
- Implemented `internal/daemon/socket.go` (~279 lines):
  - JSON request/response protocol, Handler dispatches 10 methods
  - Methods: apps.list, apps.get, apps.stop, apps.remove, apps.restart, apps.deploy,
    containers.list, containers.logs, containers.stats, ping
- Implemented `internal/daemon/client.go` (~80 lines):
  - Client with Call method, Ping health check
- Updated CLI commands: daemon.go, ps.go, stop.go, rm.go, logs.go, stats.go
- Created `testutil/helpers.go` with TempStore and CreateTestDir
- Created `internal/manager/manager_test.go` (21 tests):
  - Manager: ListApps, GetApp, GetContainers, StopApp, RemoveApp, BuildEnvList,
    GetDeployHistory, RollbackApp
  - Reconciler: backoff doubling, backoff cap, backoff reset, shouldRestart,
    isProcessAlive, ensureNoRunning
- Created `internal/daemon/daemon_test.go` (10 tests):
  - Serialization: Request, Response (success + error)
  - Client-server: PingPong, ErrorResponse, MultipleRequests, InvalidResponse,
    ConnectFailure, IsRunning (not running + running)

**Test Results:**
- `go build ./...` — clean
- `go test ./...` — all 5 packages pass (config, daemon, manager, runtime, store)
- `go vet ./...` — clean
- 31 new tests added for Phase 2

**Fixes:**
- `runRm` name collision between run.go and rm.go — renamed to `runRemoveApp`
- Removed duplicate statsCmd registration (root.go already registers it)
- Removed unused `os` import in ps.go
- Fixed seccomp for Rosetta/ARM64 (graceful degradation)

**Next:**
- Week 6: Implement rolling/blue-green deploy strategies, rollback, `vessel deploy`, `vessel history`

### Session 6 — 2026-02-05 ✅
**Task:** Phase 2 Week 6 — Deploy Pipeline, Rollback, History

**Completed:**
- Updated Store interface (`internal/store/db.go`):
  - Added UpdateDeploy, GetNextDeployVersion, GetDeployByVersion, DeleteDeploy to Store interface
- Implemented new store methods (`internal/store/deploys.go`):
  - GetDeployByVersion, DeleteDeploy, PruneDeployHistory (retention management)
- Added JSON tags to config types (`internal/config/types.go`):
  - All Config/AppConfig/HealthCheckConfig/DeployConfig structs now have `json:"..."` tags
  - Enables serialization over daemon socket protocol
- Implemented full deploy pipeline (`internal/manager/deploy.go`, ~465 lines):
  - DeployAppFromConfig: main entry point accepting config.AppConfig
  - deployNewApp: first deploy — create app, pull image, start containers, health check
  - deployRolling: rolling update — replace containers one at a time with drain timeout
  - deployBlueGreen: blue-green — start all new, health check all, atomic swap, remove old
  - createDeployRecord: monotonic versioning per app
  - finishDeploy/failDeploy: deploy status management
  - configToResourceLimits: config.ResourceConfig → store.ResourceLimits conversion
  - Container lifecycle helpers: createAndStartManagedContainer, cleanupContainers, etc.
- Implemented deploy-time health checking (`internal/manager/health.go`, ~190 lines):
  - waitForHealthy: polls health check until healthy or timeout (immediate first check)
  - HTTP health checks (GET, expect 2xx)
  - TCP health checks (connection test)
  - Command health checks (exec in container, check exit code)
  - Default health check when none configured (command: true)
  - buildHealthCheck: converts config.HealthCheckConfig → store.HealthCheck
- Implemented rollback (`internal/manager/rollback.go`, ~100 lines):
  - RollbackApp: find target version, validate status, redeploy with target's image
  - Supports version=0 (previous) and explicit version number
  - Marks new deploy with RollbackOf pointer
  - Marks old deploy as rolled_back
  - Validates target deploy was successful (active or rolled_back status)
- Updated daemon socket handler (`internal/daemon/socket.go`):
  - Added apps.deploy_config method (accepts full config.AppConfig)
  - Added apps.rollback method (name + version)
  - Added apps.history method (returns deploy list)
- Implemented CLI commands:
  - `vessel deploy` (`internal/cli/deploy.go`): parse TOML, send to daemon, show progress
    - Supports --app, --all, --image, --strategy, --config flags
  - `vessel rollback` (`internal/cli/rollback.go`): rollback with --version flag
  - `vessel history` (`internal/cli/history.go`): formatted table with version, status, image, strategy, duration, notes
- Refactored AppManager/Reconciler/Daemon to use `runtime.Runtime` interface:
  - Changed from concrete `*runtime.LinuxRuntime` to `runtime.Runtime` interface
  - Enables mock runtime injection for testing
  - Updated daemon.go, socket.go, app.go, reconciler.go, health.go, deploy.go
- Created comprehensive test suite (`internal/manager/deploy_test.go`, ~500 lines):
  - mockRuntime: full mock implementing runtime.Runtime interface
  - Deploy tests: new app happy path, multiple instances, health check failure, image pull failure
  - Rolling update tests: container replacement, failure keeps old containers
  - Blue-green tests: happy path, failure keeps old containers
  - Versioning test: 3 sequential deploys with correct version numbers
  - Rollback tests: previous version, specific version, no history, failed deploy, app not found
  - Store tests: GetDeployByVersion, GetNextDeployVersion
  - configToResourceLimits: unit tests with table-driven tests

**Test Results:**
- `go build ./...` — clean
- `go test ./...` — all 5 packages pass (config, daemon, manager, runtime, store)
- `go vet ./...` — clean
- 18 new tests added in deploy_test.go
- manager package total: 36 tests (was 21, now 36)
- Overall test count: ~77 tests across all packages

**Decisions:**
- Refactored AppManager/Reconciler/Daemon to use `runtime.Runtime` interface (enables mock injection for testing)
- Deploy-time health checks are separate from continuous health monitoring (Phase 4)
- PruneDeployHistory keeps last 10 deploys by default
- DeployApp backward-compat wrapper kept for existing socket handler
- waitForHealthy does immediate first health check before waiting for ticker (faster tests)

**Next:**
- Week 7: Environment variables, encrypted secrets, multi-app deploys
