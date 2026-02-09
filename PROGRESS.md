# PROGRESS.md — Vessel Development Progress

> **Updated after every Claude Code session.**
> This file tracks what has been completed, what's in progress, and what's next.

---

## Current Phase: PHASE 4 — Health Monitoring & Reliability

### Status: IN PROGRESS (Sessions 1-2 COMPLETE — Health Monitor, Auto-Restart, Resource Metrics)

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

## Phase 4: Health Monitoring & Reliability (Week 10) — IN PROGRESS
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
### Remaining
- [ ] Webhook alerting

## Phase 5: CLI Polish & Remote Deploy (Week 11) — NOT STARTED
- [ ] Styled terminal output (lipgloss)
- [ ] Progress indicators for image pulls and deploys
- [x] `vessel init` — interactive setup (moved to Phase 2 Week 7)
- [ ] `vessel exec` — run command inside container
- [ ] `vessel ssh` — remote deploy via SSH
- [ ] Shell autocomplete (bash, zsh, fish)
- [ ] `vessel doctor` — system prerequisites check
- [x] `vessel fmt` — config validation and formatting (moved to Phase 2 Week 7)

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
- Phase 4 Session 3: Webhook alerting (final remaining Phase 4 item)
