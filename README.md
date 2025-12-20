# Stable Devnet

> Start a local 4-validator devnet with mainnet state in 3 commands

## TL;DR

```bash
git clone https://github.com/b-harvest/devnet-builder.git && cd stable-devnet
make build
./build/devnet-builder deploy
```

After ~2 minutes, you'll have a running devnet at:
- **Cosmos RPC**: http://localhost:26657
- **EVM JSON-RPC**: http://localhost:8545
- **Chain ID**: stable_988-1 (mainnet) or stabletestnet_2201-1 (testnet)

---

## Table of Contents

- [TL;DR](#tldr)
- [Prerequisites](#prerequisites)
- [Quick Start](#quick-start)
- [Basic Commands](#basic-commands)
- [Documentation](#documentation)
- [Test Accounts](#test-accounts)
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
git clone https://github.com/b-harvest/devnet-builder.git
cd stable-devnet
make build

# Binary will be at ./build/devnet-builder
```

### Deploy Your First Devnet

```bash
# Deploy with default settings (4 validators, mainnet data, docker mode)
./build/devnet-builder deploy

# Check status
./build/devnet-builder status

# View logs
./build/devnet-builder logs -f

# Stop when done
./build/devnet-builder down
```

### Common Variations

```bash
# Single validator (fastest startup)
devnet-builder deploy --validators 1

# With test accounts
devnet-builder deploy --accounts 5

# Use testnet data instead of mainnet
devnet-builder deploy --network testnet

# Local binary mode (advanced)
devnet-builder deploy --mode local
```

---

## Basic Commands

| Command | Description |
|---------|-------------|
| `devnet-builder deploy` | Deploy a new devnet |
| `devnet-builder status` | Show devnet status |
| `devnet-builder logs` | View node logs |
| `devnet-builder down` | Stop running nodes |
| `devnet-builder up` | Restart stopped nodes |
| `devnet-builder destroy` | Remove all devnet data |

For complete command reference, see [docs/commands.md](docs/commands.md).

---

## Documentation

For detailed documentation, see the [docs/](docs/) directory:

- **[Getting Started](docs/getting-started.md)** - Detailed installation and first deployment guide
- **[Command Reference](docs/commands.md)** - Complete CLI documentation with all flags and examples
- **[Configuration](docs/configuration.md)** - config.toml options and customization
- **[Workflows](docs/workflows.md)** - Common debugging and testing workflows
- **[Troubleshooting](docs/troubleshooting.md)** - Common issues and solutions

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

### EVM Configuration

| Parameter | Value |
|-----------|-------|
| Network Name | Stable Devnet |
| Chain ID | stable_988-1 (mainnet fork) |
| EVM Chain ID | 988 (mainnet) / 2201 (testnet) |
| RPC URL | http://localhost:8545 |
| WebSocket | ws://localhost:8546 |

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

For more troubleshooting help, see [docs/troubleshooting.md](docs/troubleshooting.md).

---

## License

This project is licensed under the [MIT License](LICENSE).
