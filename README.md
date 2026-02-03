# Vessel

**A single-binary container orchestrator for the rest of us.**

<!-- Badges -->
![Build Status](https://img.shields.io/badge/build-passing-brightgreen)
![License](https://img.shields.io/badge/license-MIT-blue)
![Go Version](https://img.shields.io/badge/go-1.22+-00ADD8)

---

## Why not Kubernetes?

Kubernetes is fantastic for massive-scale, multi-team, multi-cloud deployments. But for most of us running a few services on a handful of servers, it's overkill. Vessel gives you zero-downtime deploys, automatic TLS, health monitoring, and rollbacks in a single binary with zero dependencies. No YAML manifests, no etcd cluster, no PhD required.

## Features

- **Single binary** — Download and run. No Docker daemon, no Kubernetes cluster, no external dependencies.
- **Zero-downtime deploys** — Rolling and blue-green deployment strategies built-in.
- **Automatic TLS** — Let's Encrypt certificates provisioned automatically.
- **Health monitoring** — HTTP, TCP, and command health checks with auto-restart.
- **One-click rollbacks** — Every deploy is versioned. Roll back instantly.
- **Web dashboard** — Beautiful real-time dashboard embedded in the binary.
- **Raw Linux primitives** — Built on namespaces, cgroups v2, and OverlayFS. OCI-compatible images.

## Quickstart

### Install

```bash
# One-line install (coming soon)
curl -fsSL https://get.vessel.dev | sh

# Or download from releases
# https://github.com/vessel/vessel/releases
```

### Create a `vessel.toml`

```toml
[[app]]
name = "myapi"
image = "myapp:latest"
instances = 2
domains = ["api.example.com"]

[app.resources]
cpu = 100      # 100% = 1 core
memory = "256MB"

[app.health_check]
type = "http"
path = "/health"
port = 8080
interval = "30s"
timeout = "5s"

[app.deploy]
strategy = "rolling"
```

### Deploy

```bash
# Check system prerequisites
vessel doctor

# Deploy your app
vessel deploy

# Check status
vessel ps

# View logs
vessel logs myapi -f

# Rollback if needed
vessel rollback myapi
```

## Commands

| Command | Description |
|---------|-------------|
| `vessel deploy` | Deploy applications from vessel.toml |
| `vessel ps` | List running applications |
| `vessel logs` | View application logs |
| `vessel stop` | Stop an application |
| `vessel rm` | Remove an application |
| `vessel rollback` | Rollback to a previous deploy |
| `vessel history` | Show deploy history |
| `vessel health` | Show health status |
| `vessel stats` | Show resource usage |
| `vessel secret` | Manage secrets |
| `vessel exec` | Execute command in container |
| `vessel init` | Create a new vessel.toml |
| `vessel doctor` | Check system prerequisites |
| `vessel daemon` | Start the Vessel daemon |
| `vessel fmt` | Format configuration file |

## Requirements

- Linux with kernel 4.18+ (for cgroups v2)
- Root privileges (for namespaces and cgroups)

## Project Status

Vessel is under active development. See [PROGRESS.md](PROGRESS.md) for current status.

## License

MIT License. See [LICENSE](LICENSE) for details.

---

*Kubernetes is for Google. Vessel is for the rest of us.*
