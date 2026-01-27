# devnet-builder

> Build and manage local blockchain development networks with production state

## TL;DR

```bash
git clone https://github.com/altuslabsxyz/devnet-builder.git && cd devnet-builder
make build
./build/devnet-builder deploy
```

After ~2 minutes, you'll have a running local blockchain network with:
- **Cosmos RPC**: http://localhost:26657
- **EVM JSON-RPC**: http://localhost:8545 (if supported by network)
- **Multiple validators**: Production-like consensus environment

---

## Table of Contents

- [TL;DR](#tldr)
- [Prerequisites](#prerequisites)
- [Quick Start](#quick-start)
- [Architecture](#architecture)
- [Basic Commands](#basic-commands)
- [Documentation](#documentation)
- [Test Accounts](#test-accounts)
- [Plugin System](#plugin-system)
- [Troubleshooting](#troubleshooting)
- [License](#license)

---

## Prerequisites

- **Docker** (for docker mode - recommended)
- **curl** (for network operations)
- **jq** (for JSON processing)
- **zstd** or **lz4** (for snapshot decompression)

### Verify Prerequisites

```bash
docker --version
curl --version
jq --version
zstd --version || lz4 --version
```

---

## Quick Start

### Build from Source

```bash
git clone https://github.com/altuslabsxyz/devnet-builder.git
cd devnet-builder
make build

# This builds three binaries:
#   ./build/devnet-builder  - Main CLI (interactive, recommended)
#   ./build/dvb             - Daemon-based CLI (kubectl-style)
#   ./build/devnetd         - Daemon server (for dvb)
```

### Deploy Your First Devnet

**Option 1: Using devnet-builder (recommended for most users)**

```bash
# Deploy with default settings (4 validators, mainnet data, docker mode)
./build/devnet-builder deploy

# Check status
./build/devnet-builder status

# View logs
./build/devnet-builder logs -f

# Stop when done
./build/devnet-builder stop
```

**Option 2: Using dvb with daemon (kubectl-style workflow)**

```bash
# Start the daemon
./build/devnetd &

# Provision using interactive wizard
./build/dvb provision -i

# Or apply from YAML configuration
./build/dvb apply -f devnet.yaml

# List devnets
./build/dvb get devnets

# View detailed status
./build/dvb describe my-devnet
```

### Common Variations

```bash
# Single validator (fastest startup)
devnet-builder deploy --validators 1

# With 5 funded test accounts
devnet-builder deploy --accounts 5

# Use testnet data instead of mainnet
devnet-builder deploy --network testnet

# Local binary mode (requires binary, 1-4 validators max)
devnet-builder deploy --mode local --validators 2

# Docker mode with many validators (1-100)
devnet-builder deploy --mode docker --validators 10
```

---

## Architecture

The project provides three binaries for different use cases:

| Binary | Purpose | Use Case |
|--------|---------|----------|
| `devnet-builder` | Full-featured interactive CLI | Most users, interactive workflows |
| `dvb` | Daemon-based kubectl-style CLI | Automation, YAML-driven workflows |
| `devnetd` | Daemon server | Required for `dvb` commands |

**devnet-builder** is a standalone CLI that handles everything internally - ideal for interactive use and simple deployments.

**dvb + devnetd** follow a client-server architecture similar to kubectl/kube-apiserver. The daemon manages devnet lifecycle and state, while dvb provides a declarative interface with YAML support.

---

## Basic Commands

### devnet-builder (Standalone CLI)

| Command | Description |
|---------|-------------|
| `devnet-builder deploy` | Deploy a new devnet (provision + start) |
| `devnet-builder status` | Show devnet status |
| `devnet-builder logs [node]` | View node logs |
| `devnet-builder stop` | Stop running nodes |
| `devnet-builder start` | Restart stopped nodes |
| `devnet-builder destroy` | Remove all devnet data |
| `devnet-builder export` | Export blockchain state |
| `devnet-builder export-keys` | Export validator/account keys |
| `devnet-builder upgrade` | Upgrade chain version |
| `devnet-builder networks` | List available network plugins |

### dvb (Daemon CLI)

| Command | Description |
|---------|-------------|
| `dvb apply -f <file>` | Apply devnet configuration from YAML |
| `dvb get devnets` | List all devnets |
| `dvb get devnet <name>` | Get specific devnet details |
| `dvb describe <devnet>` | Show detailed devnet info with events |
| `dvb provision -i` | Interactive provisioning wizard |
| `dvb start <devnet>` | Start a stopped devnet |
| `dvb stop <devnet>` | Stop a running devnet |
| `dvb destroy <devnet>` | Remove a devnet |
| `dvb logs <devnet> [node]` | View logs from devnet nodes |
| `dvb daemon status` | Check if daemon is running |

For complete command reference, see [docs/commands.md](docs/commands.md).

---

## Documentation

For detailed documentation, see the [docs/](docs/) directory:

- **[Getting Started](docs/getting-started.md)** - Detailed installation and first deployment guide
- **[Command Reference](docs/commands.md)** - Complete CLI documentation with all flags and examples
- **[Configuration](docs/configuration.md)** - config.toml options and customization
- **[YAML Devnet Guide](docs/yaml-devnet-guide.md)** - Using dvb with YAML configurations
- **[Plugin System](docs/plugins.md)** - Build custom network plugins for V1 and V2
- **[Workflows](docs/workflows.md)** - Common debugging and testing workflows
- **[Troubleshooting](docs/troubleshooting.md)** - Common issues and solutions

### V2 Architecture (daemon-based)

- **[V2 Overview](docs/v2/README.md)** - V2 architecture and features
- **[V2 Plugins](docs/v2/plugins.md)** - Advanced V2 plugin development
- **[API Reference](docs/v2/api-reference.md)** - gRPC API documentation

---

## Test Accounts

Devnet creates pre-funded test accounts for development.

### Export Keys

```bash
# View all test accounts
devnet-builder export-keys

# JSON format for scripts
devnet-builder export-keys --json

# Export only accounts (not validators)
devnet-builder export-keys --type accounts
```

### State Export

Export blockchain state at any height for testing upgrades, snapshots, or state analysis:

```bash
# Export current state
devnet-builder export

# List all exports
devnet-builder export list

# Inspect export details
devnet-builder export inspect <export-path>

# Custom output directory
devnet-builder export --output-dir /path/to/exports

# Force overwrite existing export
devnet-builder export --force
```

### Network Configuration

Configure your tools to connect to the local devnet:

| Parameter | Value |
|-----------|-------|
| Network Name | Local Devnet |
| RPC URL | http://localhost:26657 |
| EVM JSON-RPC | http://localhost:8545 |
| WebSocket | ws://localhost:8546 |

---

## Plugin System

devnet-builder supports multiple blockchain networks through a plugin architecture. Create custom plugins for any Cosmos SDK-based chain.

```bash
# List available networks
devnet-builder networks

# Deploy with specific blockchain network
devnet-builder deploy --blockchain stable
devnet-builder deploy --blockchain ault

# Build plugins separately
make plugins          # Build public plugins
make plugins-private  # Build private plugins
make plugin-osmosis   # Build specific plugin
```

---

## Troubleshooting

### Quick Fixes

**Docker not running:**
```bash
sudo systemctl start docker
# or on macOS
open -a Docker
```

**Port already in use:**
```bash
lsof -i :26657
```

**Previous devnet exists:**
```bash
devnet-builder destroy --force
devnet-builder deploy
```

**Daemon not running (for dvb):**
```bash
# Check daemon status
dvb daemon status

# Start daemon
devnetd
```

For more troubleshooting help, see [docs/troubleshooting.md](docs/troubleshooting.md).

---

## License

This project is licensed under the [MIT License](LICENSE).
