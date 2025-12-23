# Command Reference

Complete reference for all devnet-builder commands.

## Table of Contents

- [Main Commands](#main-commands)
  - [deploy](#deploy)
  - [init](#init)
  - [up](#up)
  - [down](#down)
  - [destroy](#destroy)
- [Monitoring Commands](#monitoring-commands)
  - [status](#status)
  - [logs](#logs)
  - [node](#node)
- [Advanced Commands](#advanced-commands)
  - [upgrade](#upgrade)
  - [build](#build)
  - [export-keys](#export-keys)
  - [reset](#reset)
  - [restart](#restart)
- [Utility Commands](#utility-commands)
  - [config](#config)
  - [cache](#cache)
  - [versions](#versions)
- [Global Flags](#global-flags)

---

## Main Commands

### deploy

Deploy a new local devnet with mainnet or testnet state. This is the primary command for getting started.

```bash
devnet-builder deploy [flags]
```

#### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--validators` | int | 4 | Number of validator nodes |
| `--accounts` | int | 0 | Number of additional funded accounts |
| `-n, --network` | string | mainnet | Network source (mainnet, testnet) |
| `-m, --mode` | string | docker | Execution mode (docker, local) |
| `--image` | string | | Docker image tag (e.g., 1.1.3-mainnet) |
| `--stable-version` | string | latest | Stable version to use |
| `--no-cache` | bool | false | Skip snapshot cache, download fresh |
| `--no-interactive` | bool | false | Disable interactive mode |
| `--export-version` | string | | Version for genesis export (non-interactive) |
| `--start-version` | string | | Version for node start (non-interactive) |

#### Examples

```bash
# Deploy with default settings (4 validators, mainnet, docker)
devnet-builder deploy

# Deploy single validator for quick testing
devnet-builder deploy --validators 1

# Deploy with funded test accounts
devnet-builder deploy --accounts 5

# Deploy with testnet state
devnet-builder deploy --network testnet

# Deploy using local binary mode
devnet-builder deploy --mode local

# Deploy specific version
devnet-builder deploy --stable-version v1.1.3
```

---

### init

Initialize devnet configuration without starting nodes. Useful for customizing config files before starting.

```bash
devnet-builder init [flags]
```

#### Flags

Same flags as `deploy`.

#### Examples

```bash
# Initialize with default settings
devnet-builder init

# Initialize, then customize config before starting
devnet-builder init --validators 2
# Edit ~/.stable-devnet/devnet/node0/config/config.toml
devnet-builder up
```

---

### up

Start nodes from existing configuration. Use after `init` or `down`.

```bash
devnet-builder up [flags]
```

#### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--mode` | string | | Override execution mode |
| `--binary-ref` | string | | Binary reference for local mode |
| `--health-timeout` | duration | 5m | Timeout waiting for nodes to be healthy |
| `--stable-version` | string | | Stable repository version (overrides init version) |

#### Examples

```bash
# Start all nodes
devnet-builder up

# Start with longer health timeout
devnet-builder up --health-timeout 10m

# Start in local mode (overrides original)
devnet-builder up --mode local

# Start with specific binary from cache
devnet-builder up --binary-ref v1.2.3
```

---

### down

Stop running nodes without removing data. Use `up` to restart.

```bash
devnet-builder down [flags]
```

#### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--timeout` | duration | 30s | Timeout for graceful shutdown |

#### Examples

```bash
# Stop all nodes gracefully
devnet-builder down

# Stop with longer timeout for busy nodes
devnet-builder down --timeout 60s
```

---

### destroy

Remove all devnet data and optionally clear cache.

```bash
devnet-builder destroy [flags]
```

#### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--cache` | bool | false | Also remove snapshot cache |
| `--force` | bool | false | Skip confirmation prompt |

#### Examples

```bash
# Remove devnet data (prompts for confirmation)
devnet-builder destroy

# Remove without confirmation
devnet-builder destroy --force

# Remove everything including snapshot cache
devnet-builder destroy --cache --force
```

---

## Monitoring Commands

### status

Show current devnet status including node health and endpoints.

```bash
devnet-builder status [flags]
```

#### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--json` | bool | false | Output in JSON format |

#### Examples

```bash
# Show human-readable status
devnet-builder status

# Get JSON output for scripts
devnet-builder status --json

# Check specific fields with jq
devnet-builder status --json | jq '.nodes[0].height'
```

#### Sample Output

```
Devnet Status: running
Chain ID: stable_988-1
Execution Mode: docker
Network Source: mainnet

Nodes:
  node0: healthy (height: 12345)
  node1: healthy (height: 12345)
  node2: healthy (height: 12343)
  node3: healthy (height: 12344)

Endpoints:
  RPC:     http://localhost:26657
  EVM:     http://localhost:8545
  gRPC:    localhost:9090
```

---

### logs

View node logs with filtering and follow options.

```bash
devnet-builder logs [node] [flags]
```

#### Arguments

| Argument | Description |
|----------|-------------|
| `node` | Optional node name (node0, node1, etc.) |

#### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `-f, --follow` | bool | false | Follow log output |
| `--tail` | int | 100 | Number of lines to show |
| `--since` | duration | | Show logs since duration (e.g., 5m) |

#### Examples

```bash
# View recent logs from all nodes
devnet-builder logs

# Follow logs in real-time
devnet-builder logs -f

# View logs from specific node
devnet-builder logs node0

# Show last 50 lines
devnet-builder logs --tail 50

# Show logs from last 5 minutes
devnet-builder logs --since 5m

# Follow specific node
devnet-builder logs node0 -f
```

---

### node

Control individual nodes.

```bash
devnet-builder node <subcommand> <node> [flags]
```

#### Subcommands

| Subcommand | Description |
|------------|-------------|
| `start` | Start a specific node |
| `stop` | Stop a specific node |
| `logs` | View logs for a specific node |

#### Examples

```bash
# Stop node 1
devnet-builder node stop 1

# Start node 1
devnet-builder node start 1

# View node 2 logs
devnet-builder node logs 2

# Follow node 0 logs
devnet-builder node logs 0 -f
```

---

## Advanced Commands

### upgrade

Perform software upgrade via expedited governance proposal.

```bash
devnet-builder upgrade [flags]
```

#### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `-n, --name` | string | | Upgrade handler name (required) |
| `-i, --image` | string | | Docker image for new version |
| `-b, --binary` | string | | Local binary path for new version |
| `-m, --mode` | string | | Override execution mode for upgrade |
| `--version` | string | | Target version (tag or branch/commit for building) |
| `--export-genesis` | bool | false | Export genesis before/after upgrade |
| `--genesis-dir` | string | | Directory for genesis exports |
| `--upgrade-height` | int | 0 | Explicit upgrade height (0 = auto-calculate) |
| `--height-buffer` | int | 0 | Blocks to add after voting period ends (0 = auto-calculate based on block time from recent 5 blocks) |
| `--voting-period` | duration | 60s | Expedited voting period duration |
| `--no-interactive` | bool | false | Disable interactive mode |

#### Examples

```bash
# Interactive mode (select version interactively)
devnet-builder upgrade

# Upgrade to new Docker image
devnet-builder upgrade --name v2 --image ghcr.io/stablelabs/stable:v2.0.0

# Upgrade with local binary
devnet-builder upgrade --name v2 --binary /path/to/stabled

# Upgrade and export genesis for debugging
devnet-builder upgrade --name v2 --image v2.0.0-mainnet --export-genesis

# Upgrade at specific height
devnet-builder upgrade --name v2 --image v2.0.0-mainnet --upgrade-height 15000

# Non-interactive with explicit version
devnet-builder upgrade --no-interactive --name v2 --version v2.0.0

# Custom voting period
devnet-builder upgrade --name v2 --image v2.0.0 --voting-period 120s
```

---

### build

Build devnet configuration from an exported genesis file.

```bash
devnet-builder build [genesis-export.json] [flags]
```

#### Arguments

| Argument | Description |
|----------|-------------|
| `genesis-export.json` | Path to exported genesis file |

#### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--validators` | int | 4 | Number of validators |
| `--accounts` | int | 10 | Number of funded accounts |
| `--validator-balance` | string | 1000000000000000000000astable,500000000000000000000agusdt | Balance for each validator |
| `--account-balance` | string | 1000000000000000000000astable,500000000000000000000agusdt | Balance for each account |
| `--validator-stake` | string | 100000000000000000000 | Stake amount for each validator (in astable) |
| `--output` | string | ./devnet | Output directory for devnet files |
| `--chain-id` | string | | Chain ID (defaults to from genesis) |

#### Examples

```bash
# Build from genesis file
devnet-builder build genesis-export.json

# Build with custom validator count
devnet-builder build genesis-export.json --validators 2

# Build with funded accounts
devnet-builder build genesis-export.json --validators 4 --accounts 5

# Build to custom output directory
devnet-builder build genesis-export.json --output /tmp/my-devnet
```

---

### export-keys

Export validator and account private keys.

```bash
devnet-builder export-keys [flags]
```

#### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--type` | string | all | Key type: validators, accounts, or all |
| `--json` | bool | false | Output in JSON format |

#### Examples

```bash
# Export all keys
devnet-builder export-keys

# Export only validator keys
devnet-builder export-keys --type validators

# Export only account keys
devnet-builder export-keys --type accounts

# Export as JSON for scripts
devnet-builder export-keys --json
```

---

### reset

Reset chain data while preserving configuration.

```bash
devnet-builder reset [flags]
```

#### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--hard` | bool | false | Also reset configuration |
| `--force` | bool | false | Skip confirmation prompt |

#### Examples

```bash
# Soft reset (keep config, reset data)
devnet-builder reset

# Hard reset (reset everything)
devnet-builder reset --hard

# Reset without confirmation
devnet-builder reset --force
```

---

### restart

Restart all running nodes.

```bash
devnet-builder restart [flags]
```

#### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--timeout` | duration | 30s | Timeout for graceful shutdown |

#### Examples

```bash
# Restart all nodes
devnet-builder restart

# Restart with longer timeout
devnet-builder restart --timeout 60s
```

---

## Utility Commands

### config

Manage devnet-builder configuration.

```bash
devnet-builder config <subcommand> [flags]
```

#### Subcommands

| Subcommand | Description |
|------------|-------------|
| `init` | Create default config.toml |
| `show` | Display current configuration |
| `set` | Set a configuration value |

#### Examples

```bash
# Create default config file
devnet-builder config init

# Show current config
devnet-builder config show

# Set default validator count
devnet-builder config set validators 2

# Set default execution mode
devnet-builder config set mode local
```

---

### cache

Manage binary cache used for upgrades.

```bash
devnet-builder cache <subcommand> [flags]
```

#### Subcommands

| Subcommand | Description |
|------------|-------------|
| `list` | List cached binaries |
| `info` | Show cache size and location |
| `clean` | Remove cached binaries |

#### Examples

```bash
# List cached binaries
devnet-builder cache list

# Show cache info
devnet-builder cache info

# Clear all cached binaries
devnet-builder cache clean
```

---

### versions

Manage version information and cache.

```bash
devnet-builder versions [flags]
```

#### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--list` | bool | false | List available versions |
| `--refresh` | bool | false | Refresh version list from remote |
| `--clear-cache` | bool | false | Clear version cache |
| `--cache-info` | bool | false | Show cache status (age, expiry) |

#### Examples

```bash
# Show current version
devnet-builder versions

# List available versions
devnet-builder versions --list

# Refresh version list
devnet-builder versions --refresh

# Clear version cache
devnet-builder versions --clear-cache

# Show cache status
devnet-builder versions --cache-info
```

---

## Global Flags

These flags work with all commands.

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--config` | string | auto-detect | Path to config.toml |
| `-H, --home` | string | ~/.stable-devnet | Base directory for devnet data |
| `--json` | bool | false | Output in JSON format |
| `--no-color` | bool | false | Disable colored output |
| `-v, --verbose` | bool | false | Enable verbose logging |

### Examples

```bash
# Use custom home directory
devnet-builder --home /tmp/my-devnet deploy

# Use custom config file
devnet-builder --config ./custom-config.toml deploy

# Enable verbose output for debugging
devnet-builder -v deploy

# Disable colors (useful for CI)
devnet-builder --no-color status
```

---

## Environment Variables

Configuration can also be set via environment variables with the `DEVNET_` prefix:

| Variable | Equivalent Flag |
|----------|-----------------|
| `DEVNET_HOME` | `--home` |
| `DEVNET_CONFIG` | `--config` |
| `DEVNET_VERBOSE` | `--verbose` |
| `DEVNET_NO_COLOR` | `--no-color` |
| `DEVNET_JSON` | `--json` |

```bash
# Example: Set home directory via environment
export DEVNET_HOME=/tmp/my-devnet
devnet-builder deploy
```

---

## Port Reference

Default ports used by devnet nodes:

| Service | Node 0 | Node 1 | Node 2 | Node 3 |
|---------|--------|--------|--------|--------|
| P2P | 26656 | 26666 | 26676 | 26686 |
| RPC | 26657 | 26667 | 26677 | 26687 |
| gRPC | 9090 | 9091 | 9092 | 9093 |
| EVM RPC | 8545 | - | - | - |
| EVM WS | 8546 | - | - | - |

Note: EVM endpoints are only available on node0.

---

## See Also

- [Getting Started](getting-started.md) - Installation and first deployment
- [Configuration](configuration.md) - config.toml reference
- [Workflows](workflows.md) - Common debugging workflows
- [Troubleshooting](troubleshooting.md) - Common issues and solutions
