# PROGRESS.md — Vessel Development Progress

> **Updated after every Claude Code session.**
> This file tracks what has been completed, what's in progress, and what's next.

---

## Current Phase: PHASE 6 — Web Dashboard

### Status: IN PROGRESS (Session 2 — WebSocket Streaming)

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

## Phase 2: App Manager & Deploys (Weeks 5-7) ✅ COMPLETE

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

### Week 7: Environment, Secrets, and Multi-App ✅
- [x] Implement environment variable injection
- [x] Implement .env file parser (internal/config/envfile.go)
- [x] Implement --env and --env-file CLI flags for deploy
- [x] Implement env merge order: image < config < .env < CLI flags
- [x] Implement encrypted secret store (AES-256-GCM + Argon2id)
- [x] Implement `vessel secret set/get/list/rm` subcommands
- [x] Implement secret reference resolution (${secret:key}) during deploy
- [x] Support multiple apps in a single vessel.toml ([[app]] syntax)
- [x] Implement `vessel deploy --all` with --continue-on-error
- [x] Implement deploy summary for multi-app deploys
- [x] Implement `vessel init` — interactive config generator
- [x] Implement `vessel fmt` — config validator

## Phase 3: Networking & TLS (Weeks 8-9) ✅ COMPLETE

### Week 8: Reverse Proxy and Routing
- [x] Implement HTTP reverse proxy with host-based routing — COMPLETE (Session 10)
- [x] Implement load balancing across instances (random selection) — COMPLETE (Session 10)
- [ ] Implement WebSocket proxying (deferred to Phase 5)
- [x] Implement container networking (bridge, veth pairs, NAT) — COMPLETE (Session 8: foundation, Session 9: lifecycle integration)
- [x] Integrate networking into container lifecycle (deploy, stop, remove)
- [x] Route registration/deregistration in deploy pipeline — COMPLETE (Session 10)
- [x] `vessel network status` shows routes — COMPLETE (Session 10)
- [x] `vessel network status` and `vessel network list` CLI commands
- [x] `vessel ps` shows IP column
- [x] Implement internal DNS — COMPLETE (Session 11)

### Week 9: Automatic TLS
- [x] Implement ACME client for Let's Encrypt (autocert) — COMPLETE (Session 11)
- [x] Automatic certificate provisioning on deploy — COMPLETE (Session 11)
- [x] Certificate renewal (handled by autocert) — COMPLETE (Session 11)
- [x] HTTP → HTTPS redirect — COMPLETE (Session 11)
- [x] Custom certificate support — COMPLETE (Session 11)
- [x] `vessel cert list` and `vessel cert import` CLI commands — COMPLETE (Session 11)

## Phase 4: Health Monitoring & Reliability (Week 10) — COMPLETE
### Session 1: Continuous Health Monitor & Auto-Restart ✅
- [x] Implement HTTP health checks (shared in health/checker.go)
- [x] Implement TCP health checks (shared in health/checker.go)
- [x] Implement command health checks (shared in health/checker.go)
- [x] Continuous health monitor (health/monitor.go)
- [x] Auto-restart with exponential backoff (health/restarter.go)
- [x] Health result persistence (store/health.go)
- [x] Health monitor integration in deploy pipeline
- [x] RestartContainer in AppManager
- [x] Health status socket handlers (health.status, health.all)
- [x] 41 new tests (29 health + 12 manager)
### Session 2: Resource Monitoring & Time-Series Metrics ✅
- [x] Time-series metrics storage (store/metrics.go)
- [x] MetricsCollector with periodic collection (health/metrics.go)
- [x] Metrics pruning (hourly, configurable retention)
- [x] Metrics integration in deploy pipeline (register/deregister)
- [x] Metrics socket handlers (metrics.get, metrics.latest)
- [x] Updated `vessel stats` with NET RX/TX columns, metrics.latest fallback
- [x] Implemented `vessel health` CLI command (summary + per-app detail)
- [x] 25 new tests (18 health/metrics + 7 manager/metrics)
### Session 3: Webhook Alerting & CLI Enhancements ✅
- [x] Webhook alerter (health/alerter.go) with rate limiting
- [x] AlertConfig, Alert types with JSON payload
- [x] Rate limiting per container (configurable MinInterval, default 5m)
- [x] Recovery alert support (configurable IncludeRecovers)
- [x] Alerter integration with health monitor (emitEvent calls alerter)
- [x] AlertingConfig added to config types (vessel.toml [alerting] section)
- [x] `vessel health check <container-id>` — immediate health check
- [x] `vessel health history <container-id>` — health check history
- [x] `vessel health watch` — continuous health status display
- [x] Socket handlers: health.check_now, health.history
- [x] Store reference added to socket Handler for health history queries
- [x] 15 new tests (alerter unit tests + monitor integration)

## Phase 5: CLI Polish & Remote Deploy (Week 11) — COMPLETE
- [x] Styled terminal output (lipgloss) — Session 16
- [x] Progress indicators for image pulls and deploys — Session 17
- [x] `vessel init` — interactive setup (moved to Phase 2 Week 7)
- [x] `vessel exec` — run command inside container (Session 15)
- [x] `vessel ssh` — remote deploy via SSH — Session 18
- [x] `vessel remote` — manage remote server configs — Session 18
- [x] Shell autocomplete (bash, zsh, fish) — Session 17
- [x] `vessel doctor` — system prerequisites check (Session 16)
- [x] `vessel fmt` — config validation and formatting (moved to Phase 2 Week 7)

## Phase 6: Web Dashboard (Weeks 12-13) — IN PROGRESS

### Week 12: Backend API + Dashboard Skeleton
- [x] REST API for all operations (Session 19)
- [x] WebSocket endpoints for logs and metrics (Session 20)
- [x] API key authentication (Session 19)
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

### Session 7 — 2026-02-05 ✅
**Task:** Phase 2 Week 7 — Environment Variables, Secrets, Multi-App, CLI Tools

**Completed:**
- Implemented .env file parser (`internal/config/envfile.go`, ~140 lines):
  - Standard KEY=VALUE format, comments, blank lines
  - Quoted values (single and double quotes)
  - Variable references (${VAR}, resolved from same file or OS env)
  - Secret references (${secret:key}) preserved for deploy-time resolution
  - MergeEnv() for merge-order: image < config < .env < CLI flags
  - ParseCLIEnvFlags() for --env KEY=VALUE parsing
- Implemented encrypted secret store (`internal/store/crypto.go`, ~140 lines):
  - AES-256-GCM encryption with random nonces
  - Argon2id key derivation from master password + salt
  - SecretManager wraps Store with encrypt/decrypt
  - Salt generation and persistence to /var/lib/vessel/secrets/salt
- Implemented `vessel secret` CLI commands (`internal/cli/secrets.go`, ~190 lines):
  - `vessel secret set <key> <value>` — encrypt and store
  - `vessel secret get <key>` — decrypt and display
  - `vessel secret list` — tabular display with timestamps
  - `vessel secret rm <key>` — delete
- Implemented secret reference resolution in deploy pipeline:
  - `resolveSecretRefs()` scans env values for ${secret:key} patterns
  - Called before image pull in DeployAppFromConfig
  - Fails deploy if secret not found or secret manager unavailable
- Updated daemon for secrets:
  - Daemon initializes SecretManager on startup
  - Master password from VESSEL_MASTER_PASSWORD env var (or default)
  - Salt persisted to disk, loaded on daemon restart
  - Socket handler: secrets.set, secrets.get, secrets.list, secrets.delete methods
- Updated deploy CLI (`internal/cli/deploy.go`):
  - Added --env KEY=VALUE (repeatable) and --env-file .env flags
  - Added --continue-on-error for multi-app deploys
  - Deploy summary with version transitions and timing
- Implemented `vessel init` (`internal/cli/init_cmd.go`, ~170 lines):
  - Interactive prompts: name, image, port, hostname, memory, instances, env vars, strategy
  - Generates valid TOML config
  - Option to deploy immediately after generation
- Implemented `vessel fmt` (`internal/cli/fmt.go`, ~60 lines):
  - Validates TOML syntax and config semantics
  - Reports all validation errors with context
- Wrote comprehensive tests (34 new tests):
  - config/envfile_test.go: 13 tests (parsing, quotes, var refs, merge, CLI flags)
  - config/multiapp_test.go: 6 tests (multi-app parsing, defaults, validation, secret refs)
  - store/crypto_test.go: 11 tests (salt, key derivation, encrypt/decrypt, SecretManager operations)
  - manager/secrets_test.go: 8 tests (secret resolution, deploy with secrets, merge order)

**Test Results:**
- `go build ./...` — clean
- `go test ./...` — all 5 packages pass
- `go vet ./...` — clean
- config: 24 tests (was 9, +15 new)
- daemon: 10 tests
- manager: 44 tests (was 36, +8 new)
- runtime: 7 tests
- store: 26 tests (was 15, +11 new)
- **Total: ~111 tests across all packages**

**Dependencies Added:**
- `golang.org/x/crypto` for Argon2id key derivation

**Decisions:**
- Secrets use AES-256-GCM with Argon2id key derivation (time=1, memory=64MB, threads=4)
- Master password from VESSEL_MASTER_PASSWORD env var, defaults to built-in key for dev
- Salt persisted to /var/lib/vessel/secrets/salt (generated once)
- Secret references use ${secret:key} syntax in env values
- Multi-app configs use [[app]] TOML array syntax (already supported by parser)
- Deploy merge order: image config < vessel.toml [env] < .env file < --env CLI flags
- vessel init and vessel fmt moved from Phase 5 to Phase 2 Week 7

**Phase 2 is COMPLETE. Next: Phase 3 — Networking & TLS**

### Session 8 — 2026-02-06 ✅
**Task:** Phase 3 Session 1 — Linux Bridge Network & IP Allocation

**Completed:**
- Added network path constants to `internal/paths/paths.go`:
  - `NetworkDir` (/var/lib/vessel/network)
  - `AllocationsPath` (/var/lib/vessel/network/allocations.json)
- Added network sentinel errors to `internal/errors.go`:
  - `ErrIPExhausted`, `ErrBridgeSetupFailed`, `ErrNetworkSetupFailed`, `ErrContainerIPNotFound`
- Created `internal/network/network.go` (~50 lines):
  - `NetworkManager` interface with 6 methods (SetupContainerNetwork, TeardownContainerNetwork, RegisterRoute, DeregisterRoute, Start, Stop)
  - `ContainerNetwork` struct (IP, gateway, bridge, veth names)
  - `RouteTarget` struct (container ID, IP, port, weight)
- Created `internal/network/allocator.go` (~200 lines):
  - `IPAllocator` with 10.88.0.0/16 subnet, gateway 10.88.0.1, starting at 10.88.0.2
  - `Allocate()`: sequential allocation with idempotency, skips gateway
  - `Release()`: returns IP to pool
  - `GetIP()`: lookup by container ID
  - `persist()`/`load()`: JSON persistence to disk
  - `incrementIP()`: safe IP increment with carry
- Created `internal/network/bridge.go` (~145 lines):
  - `BridgeNetwork` struct with name/subnet/logger
  - `Setup()`: create bridge, assign IP, bring up, enable IP forwarding, NAT masquerade, forwarding rules
  - `Teardown()`: remove iptables rules, delete bridge
  - `bridgeExists()`: checks /sys/class/net
  - `setupNAT()`/`setupForwarding()`/`cleanupIPTables()`: iptables management
- Created `internal/network/container.go` (~110 lines):
  - `SetupContainerNetwork()`: allocate IP, create veth pair, attach to bridge, move to namespace via nsenter, configure IP/routes inside container
  - `TeardownContainerNetwork()`: delete veth pair, release IP
  - `GetContainerIP()`: lookup helper
  - Full cleanup on failure at each step
- Created `internal/network/manager.go` (~120 lines):
  - `LinuxNetworkManager` implementing `NetworkManager` interface
  - `NewLinuxNetworkManager()`: creates allocator, bridge, route table
  - `Start()`: sets up bridge
  - `Stop()`: no-op (bridge persists across restarts)
  - `RegisterRoute()`/`DeregisterRoute()`: stub route table management for Session 3
  - `GetRoutes()`: returns isolated copy of route table
- Wrote comprehensive tests:
  - `allocator_test.go`: 11 tests (new, sequential, gateway skip, idempotent, release/realloc, getIP, persistence, release nonexistent, no data path, incrementIP, gateway/subnet)
  - `bridge_test.go`: 3 tests (not present, mock present via loopback, constructor)
  - `manager_test.go`: 6 tests (register, multiple targets, deregister by hostname, remove empty, deregister all, isolation)
  - `network_integration_test.go`: 3 integration tests (bridge setup/teardown, idempotent setup, manager start/stop) — require root

**Test Results:**
- `go build ./...` — clean
- `go test ./...` — all 6 packages pass
- `go vet ./...` — clean
- network: 20 unit tests pass (+ 3 integration tests, require root)
- config: 24, daemon: 10, manager: 44, runtime: 7, store: 26
- **Total: ~131 unit tests across all packages**

**Architecture Decisions:**
- Bridge persists across daemon restarts (Stop() is a no-op for bridge)
- IP allocator uses sequential allocation with wrap-around on subnet exhaustion
- Container networking uses nsenter for in-namespace configuration (simpler than netlink)
- Veth host-side name: "veth-" + first 8 chars of container ID
- Container-side veth is always "eth0"
- Route table is in-memory only (stub for Session 3 reverse proxy)

**Next:**
- Session 10: HTTP reverse proxy with host-based routing

### Session 9 — 2026-02-06 ✅
**Task:** Phase 3 Session 2 — Integrate Networking into Container Lifecycle

**Completed:**
- Added `GetContainerPID(containerID string) (int, error)` to `runtime.Runtime` interface
  - Method already existed on LinuxRuntime, now in the interface contract
  - Updated mock runtime in tests
- Added `NetworkManager` field to `AppManager` struct:
  - Updated `NewAppManager` to accept `network.NetworkManager` (can be nil)
  - Network is optional — deploy succeeds even without networking
- Integrated networking into deploy pipeline (`internal/manager/deploy.go`):
  - `createAndStartManagedContainer`: after StartContainer, calls GetContainerPID → SetupContainerNetwork
  - Sets container IP and PID fields in store
  - Best-effort: network failure doesn't fail the deploy
  - `stopAndRemoveContainer`: calls TeardownContainerNetwork before stopping
- Updated Daemon (`internal/daemon/daemon.go`):
  - Added `network` field to Daemon struct
  - NewDaemon initializes LinuxNetworkManager (graceful degradation if fails)
  - Start() calls network.Start() to set up bridge before accepting connections
  - Shutdown calls network.Stop()
- Added `ListContainers()` to Store interface and BoltStore:
  - Returns all containers across all apps
  - Added `ListAllContainers(ctx)` to AppManager
  - Updated socket handler: `containers.list` supports both per-app and all-containers mode
- Updated `vessel ps` to show IP column:
  - Shows first container's IP with (+N) for additional instances
- Created `vessel network` CLI commands (`internal/cli/network.go`):
  - `vessel network status`: shows bridge, IP forwarding, allocated IPs, subnet info
  - `vessel network list`: tabular display of container IDs, apps, IPs, states
  - Both support `--json` output mode
- Wrote 12 new network integration tests (`internal/manager/network_test.go`):
  - mockNetworkManager with tracking of setup/teardown calls
  - TestDeploy_SetsUpNetworking: verifies container gets IP
  - TestDeploy_MultipleInstances_EachGetsIP: unique IPs per container
  - TestDeploy_NetworkFailure_BestEffort: deploy succeeds when network fails
  - TestDeploy_NoNetworkManager_Succeeds: nil network manager OK
  - TestRollingDeploy_TearsDownOldNetwork: teardown on rolling update
  - TestBlueGreenDeploy_TearsDownOldNetwork: teardown on blue-green swap
  - TestStopAndRemoveContainer_TearsDownNetwork: teardown on stop
  - TestDeploy_ContainerPID_StoredOnNetwork: PID stored correctly
  - TestDeploy_GetPIDFails_NoNetworkSetup: graceful PID failure
  - TestListAllContainers, TestListContainers_StoreMethod, TestListContainers_Empty

**Test Results:**
- `go build ./...` — clean
- `go test ./...` — all 6 packages pass
- `go vet ./...` — clean
- manager: 56 tests (was 44, +12 new)
- **Total: ~143 tests across all packages**

**Architecture Decisions:**
- Network setup is best-effort: deploy succeeds even if networking fails
- GetContainerPID added to Runtime interface (was implementation-only)
- NetworkManager is optional (nil-safe) throughout AppManager
- ListContainers added to Store for global container listing
- `vessel network` CLI uses daemon socket for container data

**Next:**
- Session 10: HTTP reverse proxy with host-based routing

### Session 10 — 2026-02-06 ✅
**Task:** Phase 3 Session 3 — HTTP Reverse Proxy with Host-Based Routing

**Completed:**
- Implemented `internal/network/proxy.go` (~183 lines):
  - `ReverseProxy` struct with thread-safe route table
  - `ServeHTTP`: host-based routing, hostname port stripping, random load balancing
  - `RegisterRoute`: adds container as backend with duplicate detection
  - `DeregisterRoute`: removes by hostname or from all routes (empty hostname)
  - `GetRoutes`: returns isolated copy of route table
  - `Start`/`Stop`: HTTP server lifecycle with graceful shutdown
  - Health check endpoint at `/__vessel/health`
  - Sets `X-Forwarded-Host`, `X-Forwarded-Proto`, `X-Real-IP` headers
  - Custom error handler returns 502 on backend errors
- Updated `internal/network/manager.go`:
  - Added `ReverseProxy` field to `LinuxNetworkManager`
  - `NewLinuxNetworkManagerWithProxy()` for custom proxy address
  - `Start()` now starts proxy in background goroutine
  - `Stop()` gracefully shuts down proxy with 10s timeout
  - `RegisterRoute`/`DeregisterRoute` delegate to proxy (replaces stub)
  - `GetRoutes()` delegates to proxy
  - `GetProxy()` for direct test access
- Integrated route registration into deploy pipeline (`internal/manager/deploy.go`):
  - `registerContainerRoutes()`: registers all configured domains for a container
  - Called after health check passes in `deployNewApp`, `deployRolling`, `deployBlueGreen`
  - Uses first port mapping's container port, defaults to 80
  - `stopAndRemoveContainer`: deregisters routes BEFORE network teardown
- Added `network.routes` handler to daemon socket (`internal/daemon/socket.go`):
  - Handler struct now accepts `network.NetworkManager`
  - `NewHandler` takes 5 args (mgr, rt, sm, net, logger) — net can be nil
  - `handleNetworkRoutes`: returns route table via socket protocol
- Updated `vessel network status` CLI (`internal/cli/network.go`):
  - Fetches routes from daemon via `network.routes` method
  - Displays hostname → IP:port (container ID) table
- Wrote comprehensive tests:
  - `proxy_test.go`: 15 tests (health endpoint, unknown host 502, backend routing, forwarded headers, hostname port stripping, load balancing, deduplication, deregister by hostname/all, empty hostname cleanup, route isolation, backend error 502, concurrent access, start/stop lifecycle, full integration lifecycle)
  - `network_test.go` updated: mockNetworkManager tracks register/deregister calls, +1 duplicate test
  - `manager/network_test.go`: 8 new route tests (domain routes, default port, no domains, multi-instance, rolling routes, blue-green routes, deregister on stop)

**Test Results:**
- `go build ./...` — clean
- `go test ./...` — all 6 packages pass
- `go vet ./...` — clean
- network: 35 tests (was 20, +15 new proxy tests)
- manager: 64 tests (was 56, +8 new route tests)
- config: 24, daemon: 10, runtime: 7, store: 26
- **Total: ~166 tests across all packages**

**Architecture Decisions:**
- Reverse proxy uses `net/http/httputil.NewSingleHostReverseProxy` (stdlib)
- Load balancing: random selection (simple, sufficient for single-node)
- Route registration uses config `Domains []string` field
- Container port determined from first `Ports[].Container` config, defaults to 80
- Proxy starts on `:80` by default (configurable via `NewLinuxNetworkManagerWithProxy`)
- Routes deregistered BEFORE network teardown in stop path
- NewHandler in socket.go now takes 5 args (added network.NetworkManager)

**Next:**
- Session 11: TLS and internal DNS (final Phase 3 session)

### Session 11 — 2026-02-06 ✅
**Task:** Phase 3 Session 4 (Final) — TLS Support & Internal DNS

**Completed:**
- Implemented TLS support for reverse proxy (`internal/network/proxy.go`):
  - `ProxyConfig` struct for TLS configuration (HTTPAddr, HTTPSAddr, TLSEnabled, ACMEEmail, CertDir)
  - `NewReverseProxyWithTLS()` constructor with autocert manager
  - ACME certificate management via `golang.org/x/crypto/acme/autocert`
  - `hostPolicy()` restricts ACME certs to registered route hostnames
  - `getCertificate()` checks custom certs first, falls back to ACME
  - `LoadCustomCert()` for importing custom TLS certificates
  - `StartTLS()` starts both HTTP and HTTPS servers
  - HTTP server handles ACME challenges (/.well-known/acme-challenge/) and 301 redirects to HTTPS
  - HTTPS server terminates TLS with MinVersion TLS 1.2
  - `X-Forwarded-Proto` correctly set to "https" for TLS connections
  - `IsTLSEnabled()`, `HasACME()`, `GetCustomCerts()` inspection methods

- Implemented internal DNS server (`internal/network/dns.go`, ~195 lines):
  - `DNSServer` with thread-safe record storage
  - `RegisterApp()`: maps <app-name>.vessel.internal to container IPs
  - `DeregisterApp()`: removes DNS records
  - `Resolve()`: direct lookup for testing
  - `handleDNS()`: processes DNS queries
    - A record queries for .vessel.internal resolved locally
    - NXDOMAIN for unknown .vessel.internal names
    - External domains forwarded to upstream DNS (default 8.8.8.8:53)
  - `Start()`/`Stop()`: UDP DNS server lifecycle
  - Uses `github.com/miekg/dns` library
  - Case-insensitive hostname matching
  - 60-second TTL for internal records

- Integrated DNS into NetworkManager (`internal/network/manager.go`):
  - Added `DNSServer` field to `LinuxNetworkManager`
  - DNS server starts on `10.88.0.1:53` (bridge gateway IP)
  - `RegisterAppDNS()`/`DeregisterAppDNS()` methods
  - `LoadCustomCert()`/`GetCustomCerts()`/`IsTLSEnabled()` delegations
  - DNS server shut down gracefully on `Stop()`

- Updated container init (`internal/runtime/init.go`):
  - `/etc/resolv.conf` now points to `10.88.0.1` (Vessel DNS) as primary nameserver
  - Falls back to `8.8.8.8` (Google DNS) as secondary
  - `search vessel.internal` allows short names (e.g., `curl myapp`)

- Integrated DNS registration into deploy pipeline (`internal/manager/deploy.go`):
  - `registerAppDNS()`: collects container IPs, registers with DNS server
  - `deregisterAppDNS()`: removes DNS records
  - DNS registered after successful deploy (deployNewApp, deployRolling, deployBlueGreen)
  - DNS deregistered on app removal (`RemoveApp`)

- Created `vessel cert` CLI commands (`internal/cli/certs.go`):
  - `vessel cert list`: shows TLS status, ACME config, loaded certificates
  - `vessel cert import <hostname> --cert cert.pem --key key.pem`: imports custom certificates
  - Both support `--json` output mode

- Added daemon socket handlers (`internal/daemon/socket.go`):
  - `cert.list`: returns TLS status and custom cert hostnames
  - `cert.import`: loads custom certificate into reverse proxy

- Wrote comprehensive test suite:
  - `dns_test.go`: 12 tests (constructor, register/deregister, case-insensitive, resolve, isolation, overwrite, DNS query for internal A record, NXDOMAIN, multiple IPs, upstream forwarding, start/stop, stop nil)
  - `proxy_test.go`: 13 new TLS tests (TLS constructor, no-ACME, custom cert load, invalid cert, getCertificate, no cert, host policy, forwarded proto HTTPS, HTTP→HTTPS redirect, health on TLS, stop both servers)
  - Full integration: mock upstream DNS server for forwarding tests

**Test Results:**
- `go build ./...` — clean
- `go test ./...` — all 7 packages pass
- `go vet ./...` — clean
- network: 60 tests (was 35, +12 DNS + 13 TLS)
- config: 24, daemon: 10, manager: 64, runtime: 7, store: 26
- **Total: ~191 tests across all packages**

**Dependencies Added:**
- `golang.org/x/crypto/acme/autocert` (autocert for ACME/Let's Encrypt)
- `github.com/miekg/dns` (DNS server for internal service discovery)

**Architecture Decisions:**
- TLS uses autocert for automatic Let's Encrypt certificate provisioning
- Custom certs checked first before ACME fallback (allows self-signed for dev)
- DNS server runs on bridge gateway IP (10.88.0.1:53) — containers resolve via bridge
- Container resolv.conf: primary=10.88.0.1 (Vessel), secondary=8.8.8.8 (Google), search=vessel.internal
- DNS records updated after deploy success (not during deploy — avoids partial state)
- App DNS deregistered on RemoveApp
- WebSocket proxying deferred to Phase 5 (not in scope for Phase 3)

**Phase 3 is COMPLETE. Next: Phase 4 — Health Monitoring & Reliability**

### Session 12 — 2026-02-09 ✅
**Task:** Phase 4 Session 1 — Continuous Health Monitor & Auto-Restart

**Completed:**
- Created shared health check functions (`internal/health/checker.go`, ~75 lines):
  - `ExecuteCheck()`: dispatches to HTTP, TCP, or command check
  - `ExecuteHTTPCheck()`: HTTP GET, expects 2xx response
  - `ExecuteTCPCheck()`: TCP connection test
  - `ExecuteCommandCheck()`: exec command in container via runtime
  - Updated `internal/manager/health.go` to delegate to shared implementations
- Implemented continuous health monitor (`internal/health/monitor.go`, ~345 lines):
  - `HealthMonitor` struct with periodic check loop
  - `RegisterContainer()`/`DeregisterContainer()` for container lifecycle
  - Configurable thresholds: unhealthy (3 consecutive fails), healthy (1 success)
  - Default 10s check interval
  - Status tracking per container (ContainerHealth with last check, failures, etc.)
  - Event emission on status change via subscriber channels
  - `Subscribe()`/`Unsubscribe()` for health event consumers
  - `GetStatus()`/`GetAllStatus()` for querying health state
  - Thread-safe: RWMutex for containers, separate lock for subscribers
  - Store operations and event emission happen outside lock
  - Triggers auto-restart when unhealthy threshold reached
  - Resets backoff when container recovers to healthy
- Implemented auto-restarter (`internal/health/restarter.go`, ~200 lines):
  - `AutoRestarter` with pending restart queue (buffered channel)
  - `AppRestarter` interface for manager integration (avoids circular deps)
  - Exponential backoff: 1s initial, 2x multiplier, 5m max
  - `RequestRestart()`: non-blocking queue submission
  - `handleRestart()`: enforces backoff, calls manager's RestartContainer
  - `ResetBackoff()`: clears state on recovery
  - `CalculateDelay()`: exported for testing
  - `SetManager()`: deferred wiring to break init-time circular dependency
  - Graceful shutdown: context cancellation exits backoff waits
  - Restart count incremented in store on success
- Implemented health result persistence (`internal/store/health.go`, ~115 lines):
  - Added `bucketHealthResults` bucket to BoltStore
  - `CreateHealthResult()`: keyed by "containerID:RFC3339Nano"
  - `GetHealthResults()`: reverse chronological, with limit
  - `GetLatestHealthResult()`: convenience wrapper
  - `PruneHealthResults()`: keeps N most recent per container
  - Added 4 methods to Store interface in db.go
- Added `RestartContainer()` to AppManager (`internal/manager/app.go`):
  - Deregisters from health monitor, tears down networking
  - Stops + removes old container
  - Creates + starts new container with same config
  - Sets up networking on new container
  - Re-registers with health monitor
  - Increments RestartCount from old container
  - Implements `health.AppRestarter` interface
- Integrated health monitor into deploy pipeline (`internal/manager/deploy.go`):
  - `registerHealthMonitor()`: registers containers after health check passes
  - Called in `deployNewApp`, `deployRolling`, `deployBlueGreen`
  - Deregisters in `stopAndRemoveContainer`
  - Added `SetHealthMonitor()`/`GetHealthMonitor()` to AppManager
- Wired health components into daemon (`internal/daemon/daemon.go`):
  - Creates AutoRestarter and HealthMonitor in NewDaemon
  - Wires restarter → manager, health monitor → manager
  - Starts restarter + monitor before reconciler
  - Stops monitor + restarter during shutdown
- Added socket handlers (`internal/daemon/socket.go`):
  - `health.status`: returns ContainerHealth for a specific container
  - `health.all`: returns all monitored container health states
  - NewHandler now takes 6 args (added HealthMonitor)
- Updated RemoveApp to deregister containers from health monitor
- Wrote comprehensive test suite (41 new tests):
  - `internal/health/health_test.go` (29 tests):
    - Checker: HTTP success/failure, TCP success/failure, command success/failure, dispatch
    - Monitor: register/deregister, getAll, unhealthy threshold, healthy threshold,
      event emission, subscription, deregister during check, stop cleanup, default config,
      restarter integration
    - Restarter: delay calculation, backoff reset, setManager, request restart,
      backoff delays, queue full, stop during backoff
    - Store: create/get, multiple results, prune, no results, prune noop
  - `internal/manager/health_test.go` (12 tests):
    - RestartContainer: success, app not found, container not found, create fails, with network
    - Deploy integration: registers with health monitor, deregisters on stop,
      re-registers on restart, AppManager implements AppRestarter
    - Rolling/blue-green: both register with health monitor
    - End-to-end: health monitor triggers restart of unhealthy container

**Test Results:**
- `go build ./...` — clean
- `go test ./...` — all 8 packages pass
- `go vet ./...` — clean
- health: 29 tests (new package)
- manager: 76 tests (was 64, +12 new)
- config: 24, daemon: 10, network: 60, runtime: 7, store: 26
- **Total: ~232 unit tests across all packages**

**Architecture Decisions:**
- Health checks shared between deploy-time and continuous monitoring (health/checker.go)
- manager/health.go delegates to health.ExecuteCheck (no duplication)
- AppRestarter interface in health package, implemented by manager.AppManager (no circular deps)
- AutoRestarter uses SetManager() for deferred wiring (breaks init-time cycle)
- Health monitor stores results on every check (best-effort, outside lock)
- Backoff: 1s → 2s → 4s → 8s → ... → 5m cap; reset on recovery
- Health monitor optional (nil-safe) throughout AppManager
- RemoveApp deregisters containers from health monitor
- Socket handler NewHandler takes 6 args (added HealthMonitor, can be nil)

**Next:**
- Phase 4 Session 2: Resource monitoring (cgroup stats), time-series metrics, `vessel health` CLI

### Session 13 — 2026-02-09 ✅
**Task:** Phase 4 Session 2 — Resource Monitoring & Time-Series Metrics

**Completed:**
- Implemented time-series metrics storage (`internal/store/metrics.go`, ~115 lines):
  - `CreateMetricPoint()`: keyed by "containerID:timestamp_unix_nano" for efficient range queries
  - `GetMetrics()`: supports time range, limit, and negative limit (last N)
  - `PruneMetrics()`: removes all metric points older than cutoff time
  - Added 3 new methods to Store interface, `bucketMetrics` bucket to BoltStore
- Implemented MetricsCollector (`internal/health/metrics.go`, ~170 lines):
  - `MetricsCollector` with periodic collection loop (default 15s interval)
  - `RegisterContainer()`/`DeregisterContainer()` for container lifecycle
  - `collectAll()`: reads cgroup stats via runtime.ContainerStats, stores as MetricPoint
  - `runPruner()`: hourly goroutine removes metrics beyond retention (default 24h)
  - `GetMetrics()`/`GetLatestMetrics()`: query methods for stored time-series
  - `Start()`/`Stop()`: lifecycle with WaitGroup-based graceful shutdown
- Integrated MetricsCollector into daemon (`internal/daemon/daemon.go`):
  - Added `metricsCollector` field to Daemon struct
  - Created in NewDaemon (15s interval, 24h retention)
  - Started in daemon Start(), stopped in shutdown()
  - Wired into AppManager via SetMetricsCollector()
- Added socket handlers (`internal/daemon/socket.go`):
  - `metrics.get`: returns metrics for a container within time range with limit
  - `metrics.latest`: returns most recent metric point for a container
  - NewHandler now takes 7 args (added MetricsCollector, can be nil)
- Integrated metrics into deploy pipeline (`internal/manager/deploy.go`):
  - `registerMetricsCollector()`: registers containers after deploy
  - Called in `deployNewApp`, `deployRolling`, `deployBlueGreen`
  - Deregistered in `stopAndRemoveContainer()` and `RemoveApp()`
  - `RestartContainer()` deregisters old, registers new with metrics
- Added `SetMetricsCollector`/`GetMetricsCollector` to AppManager
- Updated `vessel stats` (`internal/cli/stats.go`):
  - Added NET RX and NET TX columns
  - Tries `metrics.latest` first, falls back to `containers.stats` (live cgroup)
  - `humanBytes()` helper for formatting byte counts
- Implemented `vessel health` CLI command (`internal/cli/health.go`, ~160 lines):
  - `vessel health`: summary of all monitored containers (status, fails, last check, message)
  - `vessel health <app>`: detailed per-container health with container state
  - Both support `--json` output mode
- Wrote comprehensive test suite (25 new tests):
  - `internal/health/metrics_test.go` (18 tests):
    - MetricsCollector: defaults, custom intervals, register/deregister, multiple containers,
      collect stores metrics, stats error handling, start/stop, get latest, time range,
      limit, deregister stops collection
    - Store metrics: create/get, multiple containers, prune, prune multiple containers,
      empty results, prune noop, chronological order
  - `internal/manager/metrics_test.go` (7 tests):
    - Deploy registers with metrics collector
    - Deploy without metrics collector succeeds
    - Deregister metrics on stop/remove
    - Rolling deploy registers new/deregisters old
    - Blue-green deploy registers new/deregisters old
    - SetGetMetricsCollector

**Test Results:**
- `go build ./...` — clean
- `go test ./...` — all 8 packages pass
- `go vet ./...` — clean
- health: 47 tests (was 29, +18 new)
- manager: 83 tests (was 76, +7 new)
- config: 26, daemon: 10, network: 60, runtime: 7, store: 27
- **Total: ~260 unit tests across all packages**

**Architecture Decisions:**
- MetricsCollector in health/ package (alongside HealthMonitor — both monitor container state)
- Metrics stored in BoltDB with key "containerID:020d_timestamp_nano" (lexicographic = chronological)
- Collection interval 15s, retention 24h (both configurable)
- Pruning runs hourly in a separate goroutine
- `vessel stats` tries metrics.latest first (stored), falls back to live cgroup stats
- MetricsCollector optional (nil-safe) throughout AppManager — same pattern as HealthMonitor
- NewHandler in socket.go now takes 7 args (added MetricsCollector)
- Network Rx/Tx metrics are 0 for now (would need /proc/net/dev parsing inside container namespace)

**Next:**
- Phase 5: CLI Polish & Remote Deploy

### Session 14 — 2026-02-09 ✅
**Task:** Phase 4 Session 3 (Final) — Webhook Alerting & CLI Enhancements

**Completed:**
- Implemented webhook alerter (`internal/health/alerter.go`, ~120 lines):
  - `AlertConfig` struct: WebhookURL, Enabled, MinInterval, IncludeRecovers
  - `Alert` struct: Type, AppID, ContainerID, Message, Timestamp
  - `Alerter` with HTTP POST to webhook endpoint
  - Rate limiting per container (default 5m between alerts for same container)
  - Recovery alerts configurable (IncludeRecovers flag)
  - `HandleHealthEvent()`: converts HealthEvent to Alert and sends
  - JSON payload with Content-Type and User-Agent headers
- Integrated alerter with HealthMonitor (`internal/health/monitor.go`):
  - Added `alerter *Alerter` field to HealthMonitor struct
  - Updated `NewHealthMonitor()` to accept alerter parameter (can be nil)
  - `emitEvent()` now calls `alerter.HandleHealthEvent()` asynchronously
  - Updated all callers: daemon.go, health_test.go (11 calls), manager/health_test.go (7 calls)
- Added AlertingConfig to config types (`internal/config/types.go`):
  - `AlertingConfig` struct: Enabled, WebhookURL, MinInterval, IncludeRecovers
  - Added `Alerting AlertingConfig` field to `Config` struct
  - Supports `[alerting]` TOML section in vessel.toml
- Added health subcommands (`internal/cli/health.go`):
  - `vessel health check <container-id>`: runs immediate health check, shows result
  - `vessel health history <container-id>`: shows last 20 health check results
  - `vessel health watch`: continuous 2s refresh of all health statuses
  - `humanDuration()` helper for formatting durations
  - All subcommands support `--json` output mode
- Added socket handlers (`internal/daemon/socket.go`):
  - `health.check_now`: executes immediate health check using monitor's config
  - `health.history`: returns health results from store (with configurable limit)
  - Added `store.Store` field to Handler for health history queries
  - `NewHandler` now takes 8 args (added Store parameter)
- Wrote comprehensive test suite (15 new tests):
  - `TestAlerter_NewAlerter_Defaults`: verifies default 5m min interval
  - `TestAlerter_SendAlert_Disabled`: disabled alerter returns nil
  - `TestAlerter_SendAlert_EmptyURL`: empty URL returns nil
  - `TestAlerter_SendAlert_Success`: verifies payload, headers, Content-Type
  - `TestAlerter_SendAlert_WebhookError`: 500 response returns error
  - `TestAlerter_RateLimiting`: second alert for same container rate-limited
  - `TestAlerter_RecoveryAlerts_Disabled`: skips recovery when disabled
  - `TestAlerter_RecoveryAlerts_Enabled`: sends recovery when enabled
  - `TestAlerter_HandleHealthEvent_Unhealthy`: unhealthy event triggers alert
  - `TestAlerter_HandleHealthEvent_Recovered`: recovery event triggers alert
  - `TestAlerter_HandleHealthEvent_InitialHealthy_Ignored`: initial healthy ignored
  - `TestAlerter_HandleHealthEvent_UnknownStatus_Ignored`: unknown status ignored
  - `TestAlerter_MonitorIntegration`: end-to-end monitor→alerter→webhook
  - `TestHumanDuration`: duration formatting
  - All tests use httptest.Server for mock webhook endpoints

**Test Results:**
- `go build ./...` — clean
- `go test ./...` — all 8 packages pass
- `go vet ./...` — clean
- health: 62 tests (was 47, +15 new alerter tests)
- manager: 83 tests
- config: 26, daemon: 10, network: 60, runtime: 7, store: 27
- **Total: ~275 tests across all packages**

**Architecture Decisions:**
- Alerter is a standalone component, injected into HealthMonitor (nil-safe)
- Alerts sent asynchronously via goroutine in emitEvent (doesn't block monitor)
- Rate limiting uses per-container map with configurable MinInterval
- Recovery alerts only sent for unhealthy→healthy transitions (not initial healthy)
- health.check_now uses the container's registered health check config from monitor
- health.history reads directly from store (store reference added to Handler)
- NewHandler in socket.go now takes 8 args (added Store)
- AlertingConfig in vessel.toml via [alerting] section

**Phase 4 is COMPLETE. Next: Phase 5 — CLI Polish & Remote Deploy**

### Session 15 — 2026-02-09 ✅
**Task:** Phase 5 Session 1 — `vessel exec` (Run Commands Inside Containers)

**Completed:**
- Implemented full exec support with TTY and non-interactive modes
- Created `internal/runtime/exec.go` (~145 lines):
  - `ExecOptions` struct: ContainerID, Command, TTY, Interactive, Env, WorkDir, User, Stdin/Stdout/Stderr
  - `ExecResult` struct: ExitCode
  - `RunExec()`: standalone function using nsenter to enter PID/mount/UTS/IPC/net namespaces
  - `execWithTTY()`: pseudo-terminal allocation via creack/pty, raw terminal mode via x/term
  - `execWithoutTTY()`: standard stdin/stdout/stderr piping
  - `buildExecResult()`: exit code extraction from exec.ExitError
  - SIGWINCH handling for terminal resize propagation
- Updated `ExecInContainer` in `internal/runtime/container.go`:
  - Replaced "not yet implemented" stub with working delegation to Exec()
  - Returns error on non-zero exit code (for health check compatibility)
  - Health command checks now functional
- Implemented `vessel exec` CLI (`internal/cli/exec.go`, ~155 lines):
  - Flags: -i/--interactive, -t/--tty, -c/--container, -e/--env, -w/--workdir, -u/--user
  - Auto-detects TTY when stdin is a terminal and command is a shell
  - -i implies -t automatically
  - `resolveContainer()`: finds running container via daemon socket
  - Supports both app name and specific container ID (-c flag)
  - Short container ID matching (12-char prefix)
  - Exit code propagation (container exit code becomes vessel exit code)
- Added daemon socket handlers (`internal/daemon/socket.go`):
  - `containers.get`: look up a specific container by ID
  - `containers.find_by_app`: return running containers for an app name
- Wrote 17 new tests (`internal/runtime/exec_test.go`):
  - RunExec validation: invalid PID, negative PID, empty command, nil command
  - buildExecResult: nil error, exit code 42, exit code 1, exit code 0
  - ExecOptions: default values verification
  - execWithoutTTY: stdout capture, stderr capture, exit code propagation, stdin pipe, command not found
  - ExecResult zero value, context cancellation

**Test Results:**
- `go build ./...` — clean
- `go test ./...` — all 8 packages pass
- `go vet ./...` — clean
- runtime: 24 tests (was 7, +17 new exec tests)
- config: 26, daemon: 10, health: 62, manager: 83, network: 60, store: 27
- **Total: ~292 unit tests across all packages**

**Dependencies Added:**
- `github.com/creack/pty` v1.1.24 (PTY allocation for interactive exec)
- `golang.org/x/term` v0.40.0 (terminal raw mode for TTY support)

**Architecture Decisions:**
- `RunExec()` is a standalone function (not on LinuxRuntime) accepting PID directly
  - CLI gets PID from daemon, calls RunExec without needing full runtime instance
  - LinuxRuntime.Exec() wraps RunExec with PID lookup from container state
- Uses nsenter for namespace entry (simpler than raw setns syscalls)
  - Enters PID, mount, UTS, IPC, and network namespaces
  - Supports --wd flag for working directory inside container
- TTY mode: creack/pty allocates pseudo-terminal, x/term sets raw mode
  - SIGWINCH forwarded for terminal resize
  - Terminal restored on exit (deferred)
- Non-TTY mode: direct stdin/stdout/stderr piping with configurable writers
- Health checks (health/checker.go) now work via ExecInContainer delegation
- CLI auto-detects TTY for shell commands (sh, bash, zsh, ash)

**Next:**
- Phase 5 continued: `vessel doctor`, shell autocomplete, styled output

### Session 16 — 2026-02-09 ✅
**Task:** Phase 5 Session 2 — `vessel doctor` & Styled Terminal Output

**Completed:**
- Added `github.com/charmbracelet/lipgloss` v1.1.0 dependency
- Created centralized styles package (`internal/cli/styles/styles.go`, ~85 lines):
  - Status colors: Success (green), Error (red), Warning (orange), Info (blue), Muted (gray)
  - Semantic styles: AppName, ContainerID, Version, Header, TableHeader, Bold
  - Icon functions: SuccessIcon, ErrorIcon, WarningIcon, InfoIcon, CheckPassIcon, CheckFailIcon, CheckWarnIcon
  - StatusDot() for colored status indicators (running/stopped/failed/deploying/healthy/unhealthy)
  - NO_COLOR env var support via ColorEnabled() and Render() helper
- Implemented `vessel doctor` (`internal/cli/doctor.go`, ~300 lines):
  - 9 diagnostic checks: OS, Kernel, cgroups v2, OverlayFS, required commands (7), network, permissions, directories, daemon
  - CheckResult struct with JSON serialization support
  - Colored output with ✓/✗/! icons per check
  - Summary with pass/warn/fail counts
  - `--json` output mode for machine-readable diagnostics
  - `--verbose` shows additional details per check
- Updated `vessel ps` (`internal/cli/ps.go`):
  - Styled table headers (TableHeader style)
  - Colored app status (running=green, stopped=gray, failed=red, deploying=yellow)
  - Colored instance counts (all running=green, none=red, partial=yellow)
  - App names styled with AppName style
- Updated `vessel deploy` (`internal/cli/deploy.go`):
  - ✓/✗ icons for success/failure messages
  - Styled app names and version numbers
  - Colored deploy summary with styled header
- Updated `vessel health` (`internal/cli/health.go`):
  - Styled table headers across all subcommands (check, history, watch)
  - Colored health status (healthy=green, unhealthy=red, unknown=gray)
  - Colored container state (running=green, stopped=gray, failed=red)
  - Styled container IDs and app names
  - Helper functions: styledHealthStatus(), styledContainerState()
- Updated `vessel history` (`internal/cli/history.go`):
  - Styled table headers
  - Colored deploy status (active=green, failed=red, rolled_back=yellow, pending=gray)
  - Colored version numbers (Version style)
  - Error messages in red
- Updated `vessel stats` (`internal/cli/stats.go`):
  - Styled table headers
  - Styled container IDs
- Fixed flaky test (`internal/health/metrics_test.go`):
  - Added +1 tolerance for in-flight collection in DeregisterStopsCollection test
- Wrote comprehensive test suite (28 new tests):
  - `internal/cli/styles/styles_test.go` (17 tests):
    - ColorEnabled default and NO_COLOR modes
    - Render with/without color
    - All icon functions (Success, Error, Warning, Info, CheckPass, CheckFail, CheckWarn)
    - StatusDot for all states (running, stopped, failed, healthy, unknown)
    - All exported styles render without panic
  - `internal/cli/doctor_test.go` (11 tests):
    - checkOS on Linux
    - checkKernel version parsing
    - checkCgroupsV2, checkOverlayFS availability
    - checkRequiredCommands (7 commands verified)
    - checkUserPermissions, checkNetworkCapabilities
    - checkVesselDirectories, checkDaemonStatus
    - CheckResult struct fields and status values

**Test Results:**
- `go build ./...` — clean
- `go test ./...` — all 10 packages pass
- `go vet ./...` — clean
- cli: 11 tests (new package)
- styles: 17 tests (new package)
- config: 26, daemon: 10, health: 62, manager: 83, network: 60, runtime: 24, store: 27
- **Total: ~320 unit tests across all packages**

**Dependencies Added:**
- `github.com/charmbracelet/lipgloss` v1.1.0 (terminal styling)

**Architecture Decisions:**
- Styles centralized in `internal/cli/styles/` package (not scattered across CLI files)
- NO_COLOR env var respected per convention (https://no-color.org)
- Styled output bypassed for --json mode (JSON path returns before styled printing)
- Doctor checks return CheckResult structs (testable, JSON-serializable)
- Doctor uses daemon.Client.Ping() for daemon health check
- All styled CLI output uses the same color semantics (green=success, red=error, yellow=warning, gray=muted)

**Next:**
- Phase 5 continued: `vessel ssh` (remote deploy via SSH)

### Session 17 — 2026-02-09 ✅
**Task:** Phase 5 Session 3 — Progress Indicators & Shell Completion

**Completed:**
- Added `github.com/briandowns/spinner` v1.23.2 dependency for terminal spinners
- Created progress package (`internal/cli/progress/progress.go`, ~155 lines):
  - `Spinner` struct wrapping briandowns/spinner with Start/Stop/Success/Fail/UpdateMessage
  - `ProgressBar` struct with Add/Set/Finish/Percent, rate-limited redraws (50ms)
  - `ProgressWriter` wrapping io.Writer with progress tracking
  - Configurable writer for testability (defaults to os.Stderr)
- Updated `vessel deploy` (`internal/cli/deploy.go`):
  - Spinner during deploy operation (replaces static "Deploying..." text)
  - Success/Fail icons on completion
  - JSON output bypasses spinner entirely
- Updated `vessel stop` (`internal/cli/stop.go`):
  - Spinner during stop operation
- Updated `vessel rollback` (`internal/cli/rollback.go`):
  - Spinner during rollback operation
- Updated `vessel rm` (`internal/cli/rm.go`):
  - Spinner during remove operation
- Implemented shell completion (`internal/cli/completion.go`, ~55 lines):
  - `vessel completion [bash|zsh|fish|powershell]`
  - Full help text with installation instructions for each shell
  - Uses cobra's built-in GenBashCompletion/GenZshCompletion/GenFishCompletion/GenPowerShellCompletionWithDesc
- Implemented dynamic app name completion (`internal/cli/complete.go`, ~30 lines):
  - `completeAppNames()` queries daemon for running apps via `apps.list`
  - Returns app names as completion suggestions, NoFileComp directive
  - Graceful error handling when daemon is unavailable
- Added `ValidArgsFunction: completeAppNames` to 8 commands:
  - exec, stop, rm, logs, rollback, health, stats, history
- Registered `completionCmd` in root.go (now 18 subcommands)
- Fixed flag shorthand conflict: exec `--container` no longer uses `-c` shorthand
  (conflicted with root `--config -c` persistent flag, caused panic during completion generation)
- Wrote comprehensive test suite (28 new tests):
  - `internal/cli/progress/progress_test.go` (18 tests):
    - Spinner: constructor, start/stop, success output, fail output, update message
    - ProgressBar: constructor, percent empty/partial/full/overflow, add, set, finish, draw
    - ProgressWriter: constructor, write, tracks progress, finish
  - `internal/cli/completion_test.go` (10 tests):
    - Command: exists, valid args, bash/zsh/fish/powershell generation
    - Registration: completion in root, all 8 commands have ValidArgsFunction
    - completeAppNames: no args (daemon error), with existing args (returns empty)

**Test Results:**
- `go build ./...` — clean
- `go test ./...` — all 12 packages pass
- `go vet ./...` — clean
- progress: 18 tests (new package)
- cli: 21 tests (was 11, +10 new completion tests)
- config: 26, daemon: 10, health: 62, manager: 83, network: 60, runtime: 24, store: 27, styles: 17
- **Total: ~348 unit tests across all packages**

**Dependencies Added:**
- `github.com/briandowns/spinner` v1.23.2 (terminal spinners)

**Architecture Decisions:**
- Progress package is standalone in `internal/cli/progress/` (not mixed with styles)
- Spinner wraps briandowns/spinner for simple start/success/fail pattern
- ProgressBar and ProgressWriter support tracking known-size operations (image pulls)
- Completion uses cobra's built-in generators (no custom completion logic)
- Dynamic completion queries daemon — returns error directive if daemon unavailable
- Fixed `-c` shorthand conflict between root `--config` and exec `--container`
- All spinner output goes to stderr (keeps stdout clean for JSON/pipe)

**Next:**
- Phase 5 Session 4: `vessel ssh` (remote deploy via SSH)

### Session 18 — 2026-02-09 ✅
**Task:** Phase 5 Session 4 (Final) — SSH Remote Deploy & Remote Management

**Completed:**
- Created SSH client package (`internal/ssh/ssh.go`, ~175 lines):
  - `Config` struct: Host, User, Port, KeyFile
  - `NewClient()`: creates SSH client with defaults (port 22, $USER)
  - `Connect()`: establishes SSH tunnel via Unix socket forwarding (`-L local:remote`)
  - `Close()`: kills tunnel process, cleans up socket
  - `CopyFile()`: SCP local file to remote server
  - `RunCommand()`: execute command on remote via SSH
  - `RunCommandOutput()`: capture remote command output
  - `CheckVessel()`: verify Vessel is installed on remote
  - `CheckDaemon()`: verify daemon socket exists on remote
  - `ParseTarget()`: parse "user@host" strings
  - `buildSSHArgs()`: common SSH args with optional key file
  - SSH options: ExitOnForwardFailure, ServerAliveInterval, ConnectTimeout, accept-new
- Implemented `vessel ssh` CLI (`internal/cli/ssh.go`, ~395 lines):
  - `vessel ssh <user@host> [command] [args...]`
  - Named remote resolution from vessel.toml `[[remote]]` sections
  - Smart routing: interactive commands (exec, logs -f, health watch) use direct SSH
  - Non-interactive commands (ps, deploy, stop, rm, health, stats, history) use tunneled daemon socket
  - Config file sync: auto-SCP config to remote before deploy
  - Remote command implementations: runRemotePS, runRemoteDeploy, runRemoteStop, runRemoteRm,
    runRemoteHealth, runRemoteStats, runRemoteHistory, runRemoteLogs, runRemoteRollback
  - Spinner with progress feedback during connection
  - Signal handling (Ctrl+C gracefully closes tunnel)
  - All output styled with lipgloss
- Added `RemoteConfig` struct to config types (`internal/config/types.go`):
  - `[[remote]]` TOML array of tables support
  - Fields: Name, Host, User, Port, KeyFile
- Implemented `vessel remote` CLI (`internal/cli/remote.go`, ~190 lines):
  - `vessel remote list`: tabular display of configured remotes
  - `vessel remote add <name> <user@host>`: add remote with optional -p/-i flags
  - `vessel remote remove <name>`: remove remote from config
  - `writeRemotesToConfig()`: TOML-aware config file update (preserves non-remote sections)
  - Duplicate name detection
  - All subcommands support `--json` output mode
- Added SSH sentinel errors to `internal/errors.go`:
  - `ErrSSHConnectionFailed`, `ErrRemoteVesselNotFound`, `ErrRemoteNotFound`
- Registered `sshCmd` and `remoteCmd` in root.go (now 20 subcommands)
- Wrote comprehensive test suite (35 new tests):
  - `internal/ssh/ssh_test.go` (15 tests):
    - NewClient: defaults, custom config, zero port, empty user
    - ParseTarget: user@host (4 cases), host only, multiple @ signs
    - Client accessors: Host, User, Port, SocketPath (before connect)
    - Close: no tunnel (no panic)
    - buildSSHArgs: with/without key file
    - Connect: empty host error
  - `internal/cli/ssh_test.go` (16 tests):
    - SSHCommand: exists, registered in root, flags (port, identity)
    - RemoteCommand: registered in root, subcommands (list, add, remove)
    - needsDirectSSH: 10 cases (exec, logs -f, health watch, ps, deploy, etc.)
    - shouldSyncConfig: 5 cases (deploy --config, -c, plain deploy)
    - getConfigFromArgs: 6 cases (various flag positions, missing values)
    - replaceConfigPath: standard, short flag, no config flag
    - resolveSSHTarget: user@host, host only, named remote, named not found
  - `internal/config/remote_test.go` (4 tests):
    - RemoteConfig parse (2 remotes with all fields)
    - Empty remotes (config without [[remote]])
    - Remotes alongside apps
    - Port default (unset = 0, caller applies 22)

**Test Results:**
- `go build ./...` — clean
- `go test ./...` — all 13 packages pass
- `go vet ./...` — clean
- ssh: 15 tests (new package)
- cli: 37 tests (was 21, +16 new)
- config: 30 tests (was 26, +4 new)
- daemon: 10, health: 62, manager: 83, network: 60, runtime: 24, store: 27
- progress: 18, styles: 17
- **Total: ~383 unit tests across all packages**

**Architecture Decisions:**
- SSH uses `os/exec` with the system `ssh` binary (not a Go SSH library) — simpler, leverages user's SSH config/agent
- Unix socket forwarding via `ssh -L local:remote` for daemon RPC
- Interactive commands (exec, logs -f) run directly via SSH (needs TTY/streaming)
- Non-interactive commands use tunneled socket for structured daemon RPC responses
- Named remotes resolved from vessel.toml `[[remote]]` sections before ParseTarget fallback
- Config file sync via SCP before remote deploy
- Remote management persists to vessel.toml (TOML-aware, preserves other sections)
- No new Go dependencies (uses system ssh/scp binaries)
- SSH client in standalone `internal/ssh/` package (no dependency on cli or daemon)

**Phase 5 is COMPLETE. Next: Phase 6 — Web Dashboard**

### Session 19 — 2026-02-09 ✅
**Task:** Phase 6 Session 1 — REST API Layer for Web Dashboard

**Completed:**
- Added Chi router dependency (`github.com/go-chi/chi/v5` v5.2.5)
- Added CORS middleware dependency (`github.com/go-chi/cors` v1.2.2)
- Implemented REST API server (`internal/api/server.go`, ~223 lines):
  - `Server` struct with Chi router, HTTP server lifecycle
  - `Config` struct: Addr, APIKeys, CORSHosts
  - `NewServer()`: middleware stack (RequestID, RealIP, Recoverer, Timeout, CORS, auth)
  - `setupRoutes()`: 25+ route registrations across 8 resource groups
  - `Start()`/`Stop()`: graceful server lifecycle
  - `apiKeyAuth()`: middleware supporting X-API-Key header and Bearer token
  - `writeJSON()`/`writeError()`: consistent JSON response helpers
- Implemented API handlers (`internal/api/handlers.go`, ~370 lines):
  - `Handler` struct wrapping manager, runtime, store, secretManager, network, healthMonitor, metricsCollector
  - `NewHandler()`: 7 parameters (all optional except manager, runtime, store)
  - 25 endpoint handlers:
    - Ping: server health check
    - Apps: ListApps, GetApp (with containers), StopApp, RemoveApp, RestartApp, DeployApp, RollbackApp, GetDeployHistory
    - Containers: ListContainers (with ?app filter), GetContainer, GetContainerLogs, GetContainerStats
    - Health: GetAllHealth, GetHealthStatus, CheckHealthNow, GetHealthHistory
    - Metrics: GetMetrics (time range + limit), GetLatestMetrics
    - Secrets: ListSecrets, SetSecret (with key validation), DeleteSecret
    - Network: GetNetworkRoutes
    - Certs: ListCerts, ImportCert
    - System: GetSystemInfo (version, app/container counts)
- Added `APIConfig` to config types (`internal/config/types.go`):
  - `APIConfig` struct: Enabled, Addr, APIKeys, CORSHosts
  - Added `API APIConfig` field to `Config` struct
  - Supports `[api]` TOML section in vessel.toml
- Integrated API server into daemon (`internal/daemon/daemon.go`):
  - Added `apiServer` field to Daemon struct
  - `startAPIServer()`: reads vessel.toml, creates handler + server if `[api] enabled = true`
  - Starts in background goroutine (added to WaitGroup)
  - Graceful shutdown with 5s timeout in daemon's shutdown sequence
- Created `vessel api key-generate` CLI command (`internal/cli/api.go`, ~55 lines):
  - Generates 32-byte cryptographically random key
  - Base64 URL-safe encoding
  - Displays key and vessel.toml configuration example
  - Supports `--json` output mode
  - Registered `apiCmd` in root.go (now 21 subcommands)
- Updated stub files (routes.go, handlers_apps.go, handlers_deploys.go, handlers_logs.go, handlers_metrics.go, middleware.go, websocket.go) with proper comments pointing to implementation files
- Wrote comprehensive test suite (`internal/api/api_test.go`, 42 tests):
  - Ping: returns OK, no auth required with auth configured
  - Auth: blocks unauthenticated, accepts X-API-Key, accepts Bearer token, rejects invalid key, allows all when no keys configured
  - CORS: headers present on OPTIONS preflight
  - Apps: list empty, list with apps, get not found, get found (with containers), stop not found, remove not found
  - Containers: list empty, list with containers, filter by app, get not found, get found
  - Health: get all (nil monitor), get status (nil monitor), check now (nil monitor), get history
  - Metrics: get metrics (nil collector), get latest (nil collector)
  - Secrets: list (nil manager), set (nil manager), delete (nil manager), set invalid JSON, set empty key
  - Network: routes (nil manager)
  - Certs: list (nil network), import (nil network)
  - System: system info with running/stopped containers
  - Content: JSON content type, error response format
  - Server: default addr (:8080), custom addr, start/stop lifecycle
  - Deploy: invalid JSON
  - Rollback: invalid JSON
  - Logs: container logs with mock runtime
  - Stats: container stats with mock runtime
  - Config: TOML parsing of [api] section
  - Handler: nil components don't panic

**Test Results:**
- `go build ./...` — clean
- `go test ./...` — all 14 packages pass
- `go vet ./...` — clean
- api: 42 tests (new package)
- cli: 37 tests
- config: 30, daemon: 10, health: 62, manager: 83, network: 60, runtime: 24, store: 27
- progress: 18, styles: 17, ssh: 15
- **Total: ~425 unit tests across all packages**

**Dependencies Added:**
- `github.com/go-chi/chi/v5` v5.2.5 (HTTP router)
- `github.com/go-chi/cors` v1.2.2 (CORS middleware)

**Architecture Decisions:**
- Chi router chosen over stdlib ServeMux for middleware support and URL parameter extraction
- API server is optional — only starts if `[api] enabled = true` in vessel.toml
- API key auth supports both `X-API-Key` header and `Authorization: Bearer` token
- Ping endpoint (`/api/ping`) bypasses auth for health checking
- All optional components (secretManager, healthMonitor, metricsCollector, network) nil-safe
- API handlers wrap the same manager/runtime/store used by the daemon socket handlers
- Response format: `{"data": ...}` for success, `{"error": {"code": "...", "message": "..."}}` for errors
- GetApp returns both app and containers for rich app detail views
- ListContainers supports `?app=` query parameter for filtering
- CORS defaults to `*` if no CORSHosts configured
- API server shuts down before other daemon components (5s timeout)
- Chi router registered in root.go — now 21 subcommands total

**API Routes:**
```
GET    /api/ping                          # Health check (no auth)
GET    /api/apps                          # List all apps
GET    /api/apps/{name}                   # Get app detail + containers
DELETE /api/apps/{name}                   # Remove app
POST   /api/apps/{name}/stop              # Stop app
POST   /api/apps/{name}/restart           # Restart app
POST   /api/apps/{name}/deploy            # Deploy app
POST   /api/apps/{name}/rollback          # Rollback app
GET    /api/apps/{name}/history           # Deploy history
GET    /api/containers                    # List containers (?app= filter)
GET    /api/containers/{id}               # Get container
GET    /api/containers/{id}/logs          # Container logs (?tail=)
GET    /api/containers/{id}/stats         # Container stats
GET    /api/health                        # All health status
GET    /api/health/{id}                   # Container health
POST   /api/health/{id}/check             # Immediate health check
GET    /api/health/{id}/history           # Health check history
GET    /api/metrics/{id}                  # Container metrics (?start=&end=&limit=)
GET    /api/metrics/{id}/latest           # Latest metrics
GET    /api/secrets                       # List secret keys
POST   /api/secrets                       # Set secret
DELETE /api/secrets/{key}                 # Delete secret
GET    /api/network/routes                # Routing table
GET    /api/certs                         # TLS cert info
POST   /api/certs                         # Import cert
GET    /api/system                        # System info
```

**Next:**
- Phase 6 continued: WebSocket endpoints for log/metric streaming, React dashboard scaffold

### Session 20 — 2026-02-09 ✅
**Task:** Phase 6 Session 2 — WebSocket Streaming for Real-Time Data

**Completed:**
- Added `github.com/gorilla/websocket` v1.5.3 dependency
- Implemented WebSocket handlers (`internal/api/websocket.go`, ~340 lines):
  - `WSMessage` struct: unified message format with Type, Timestamp, Data
  - `StreamContainerLogs`: real-time log streaming via WebSocket with follow=true
    - Reads from runtime.ContainerLogs, streams line-by-line via bufio.Scanner
    - Client disconnect detection via ReadMessage goroutine
  - `StreamContainerMetrics`: per-container metric updates every 2 seconds
    - Sends initial metric immediately, then periodic via ticker
    - Error messages on stats failure
  - `StreamAllMetrics`: metrics for all running containers every 2 seconds
    - Queries store for containers, filters to running, collects stats
  - `StreamHealth`: health status updates via event subscription
    - Sends initial status snapshot, then streams HealthEvents
    - Uses HealthMonitor.Subscribe()/Unsubscribe() for event delivery
    - Graceful handling when health monitor is nil
  - `StreamEvents`: combined event stream (health + periodic state refresh)
    - Subscribes to health events when monitor available
    - Periodic state refresh every 5 seconds (apps + health)
    - Sets healthCh to nil on channel close (prevents busy loop)
  - Helper methods: sendMetric, sendAllMetrics, sendFullState
- Added WebSocket routes to `internal/api/server.go`:
  - `/api/ws/containers/{id}/logs` — log streaming
  - `/api/ws/containers/{id}/metrics` — per-container metrics
  - `/api/ws/metrics` — all-container metrics
  - `/api/ws/health` — health event streaming
  - `/api/ws/events` — combined event stream
- Updated auth middleware for WebSocket support:
  - Added `api_key` query parameter check (for browser WebSocket API)
  - WebSocket clients can authenticate via X-API-Key header, Bearer token, or query param
- Wrote comprehensive test suite (`internal/api/websocket_test.go`, 25 tests):
  - Log streaming: upgrade succeeds, streams log lines, error on bad container, follow enabled
  - Metrics: sends initial metric, periodic updates (2s), error on stats failure
  - All metrics: sends initial, skips stopped containers
  - Health: no monitor sends error, sends initial status, streams events
  - Events: sends initial state, periodic refresh (5s)
  - Auth: query param accepted/rejected, header accepted, no key rejected, no keys allows all
  - Client disconnect: logs handler cleanup, metrics handler cleanup
  - WSMessage: JSON serialization, all message types
  - Routes: all 5 WebSocket routes registered

**Test Results:**
- `go build ./...` — clean
- `go test ./...` — all 14 packages pass
- `go vet ./...` — clean
- api: 67 tests (was 42, +25 new WebSocket tests)
- cli: 37, config: 30, daemon: 10, health: 62, manager: 83, network: 60, runtime: 24, store: 27
- progress: 18, styles: 17, ssh: 15
- **Total: ~450 unit tests across all packages**

**Dependencies Added:**
- `github.com/gorilla/websocket` v1.5.3 (WebSocket protocol support)

**Architecture Decisions:**
- gorilla/websocket used for WebSocket upgrade and message handling
- Upgrader allows all origins (CheckOrigin returns true) — production should restrict
- Auth supports query param `api_key` for browser WebSocket API (can't set custom headers)
- WebSocket routes under `/api/ws/` prefix, inside auth middleware
- Client disconnect detected via ReadMessage goroutine (returns error on close)
- Context cancellation propagates from disconnect → stops streaming
- Metric streaming uses 2s ticker; event streaming uses 5s state refresh
- Health streaming uses HealthMonitor.Subscribe() for real-time events
- StreamEvents combines health subscription + periodic state refresh
- Nil-safe: health monitor, metrics collector handled gracefully
- WSMessage is the unified format: type field distinguishes log/metric/health/error/state

**WebSocket Endpoints:**
```
WS  /api/ws/containers/{id}/logs      # Real-time log streaming
WS  /api/ws/containers/{id}/metrics   # Per-container metrics (2s interval)
WS  /api/ws/metrics                   # All-container metrics (2s interval)
WS  /api/ws/health                    # Health event streaming
WS  /api/ws/events                    # Combined event stream (5s refresh)
```

**Next:**
- Phase 6 continued: React + TypeScript dashboard scaffold
