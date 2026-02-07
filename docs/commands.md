# Command Reference

Complete reference for devnet-builder CLI commands.

This documentation covers both the modern `dvb` CLI and the legacy `devnet-builder` CLI.

## Table of Contents

- [DVB CLI (Modern)](#dvb-cli-modern)
  - [Core Commands](#core-commands)
    - [apply](#apply)
    - [get](#get)
    - [delete](#delete)
    - [list](#list)
    - [status](#status)
  - [Lifecycle Commands](#lifecycle-commands)
    - [provision](#provision)
    - [start](#start)
    - [stop](#stop)
    - [destroy](#destroy)
  - [Node Management](#node-management)
    - [node list](#node-list)
    - [node get](#node-get)
    - [node health](#node-health)
    - [node ports](#node-ports)
    - [node start](#node-start)
    - [node stop](#node-stop)
    - [node restart](#node-restart)
    - [node exec](#node-exec)
    - [node init](#node-init)
  - [Utility Commands](#utility-commands)
    - [version](#version)
    - [daemon](#daemon)
    - [logs](#logs)
  - [DVB Global Flags](#dvb-global-flags)
- [Legacy devnet-builder CLI](#legacy-devnet-builder-cli)
  - [Main Commands](#main-commands)
    - [deploy](#deploy)
    - [init](#init)
    - [start (legacy)](#start-legacy)
    - [stop (legacy)](#stop-legacy)
    - [destroy (legacy)](#destroy-legacy)
  - [Monitoring Commands](#monitoring-commands)
    - [status](#status)
    - [logs (legacy)](#logs-legacy)
    - [node (legacy)](#node-legacy)
  - [Advanced Commands](#advanced-commands)
    - [upgrade](#upgrade)
    - [build (legacy)](#build-legacy)
    - [export-keys](#export-keys)
    - [reset](#reset)
  - [Utility Commands (Legacy)](#utility-commands-legacy)
    - [config](#config)
    - [cache](#cache)
    - [versions](#versions)
    - [exec](#exec)
    - [port-forward](#port-forward)
  - [Legacy Global Flags](#legacy-global-flags)
- [Environment Variables](#environment-variables)
- [Port Reference](#port-reference)

---

## DVB CLI (Modern)

The `dvb` CLI is the modern interface for managing devnets. It supports both daemon mode (connecting to `devnetd`) and standalone mode for local development.

### Core Commands

#### apply

Apply a devnet configuration from a YAML file. This is the primary way to create or update devnets.

```bash
dvb apply -f <file> [flags]
```

##### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `-f, --file` | string | | Path to YAML file or directory (required) |
| `--dry-run` | bool | false | Preview changes without applying |
| `--data-dir` | string | ~/.devnet-builder | Base data directory for standalone mode |

##### Examples

```bash
# Apply a devnet configuration
dvb apply -f devnet.yaml

# Preview changes without applying
dvb apply -f devnet.yaml --dry-run

# Apply all YAML files in a directory
dvb apply -f ./devnets/

# Apply in standalone mode with custom data directory
dvb apply -f devnet.yaml --standalone --data-dir /path/to/data
```

---

#### get

Display devnet resources. Similar to `kubectl get`.

```bash
dvb get [resource] [name] [flags]
```

##### Resource Types

| Resource | Aliases | Description |
|----------|---------|-------------|
| `devnets` | `devnet`, `dn` | List or get devnets |
| `nodes` | `node` | List nodes (use `dvb node list` instead) |

##### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `-n, --namespace` | string | | Namespace (empty = all namespaces for list) |
| `-o, --output` | string | | Output format: wide, yaml, json |
| `-l, --selector` | string | | Label selector (e.g., 'env=prod') |
| `--show-nodes` | bool | false | Show nodes when getting a devnet |
| `-A, --all-namespaces` | bool | false | List across all namespaces |

##### Examples

```bash
# List all devnets
dvb get devnets

# List devnets in a specific namespace
dvb get devnets -n production

# Get a specific devnet
dvb get devnet my-devnet

# Get devnet with node details
dvb get devnet my-devnet --show-nodes

# Output in wide format
dvb get devnets -o wide

# Output as YAML
dvb get devnet my-devnet -o yaml
```

##### Sample Output

```
NAMESPACE    NAME        PHASE     NODES  READY  HEIGHT
default      my-devnet   Running   4      4      12345
production   prod-net    Running   2      2      54321
```

---

#### delete

Delete devnet resources by name or from a YAML file.

```bash
dvb delete [resource] [name] [flags]
dvb delete -f <file> [flags]
```

##### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `-f, --file` | string | | Path to YAML file containing resources to delete |
| `-n, --namespace` | string | | Namespace (defaults to server default) |
| `--force` | bool | false | Skip confirmation prompt |
| `--dry-run` | bool | false | Preview what would be deleted |
| `--data-dir` | string | ~/.devnet-builder | Base data directory for standalone mode |

##### Examples

```bash
# Delete a devnet by name
dvb delete devnet my-devnet

# Delete a devnet in a specific namespace
dvb delete devnet my-devnet -n production

# Delete devnets defined in a YAML file
dvb delete -f devnet.yaml

# Delete without confirmation
dvb delete devnet my-devnet --force

# Preview what would be deleted
dvb delete -f devnet.yaml --dry-run
```

---

#### list

List all devnets. Alias for `dvb get devnets`.

```bash
dvb list [flags]
```

##### Aliases

`ls`

##### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `-n, --namespace` | string | | Filter by namespace (empty = all namespaces) |

##### Examples

```bash
# List all devnets
dvb list

# List devnets in a namespace
dvb list -n production

# Using alias
dvb ls
```

---

#### status

Show current devnet status. Use `--verbose/-v` for detailed output including conditions, events, and troubleshooting information (replaces the old `describe` command).

```bash
dvb status [devnet] [flags]
```

##### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `-v, --verbose` | bool | false | Show detailed output (conditions, events, troubleshooting) |
| `--events` | bool | false | Show recent events |
| `-n, --namespace` | string | | Namespace (defaults to context or server default) |

##### Examples

```bash
# Show status of current context
dvb status

# Show detailed status (like kubectl describe)
dvb status -v

# Show status with recent events
dvb status --events

# Show status of a specific devnet
dvb status my-devnet

# Show detailed status of a specific devnet
dvb status my-devnet -v
```

---

### Lifecycle Commands

#### provision

Provision a new devnet using the ProvisioningOrchestrator. This command works in standalone mode without requiring the daemon.

```bash
dvb provision [flags]
```

##### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `-i, --interactive` | bool | false | Use interactive wizard mode |
| `--name` | string | | Devnet name (required unless using -i) |
| `--network` | string | stable | Plugin/network name (e.g., stable, cosmos) |
| `--chain-id` | string | | Chain ID (default: `<name>-devnet`) |
| `--validators` | int | 1 | Number of validators |
| `--full-nodes` | int | 0 | Number of full nodes |
| `--binary-path` | string | | Path to chain binary (skips build if provided) |
| `--data-dir` | string | ~/.devnet-builder | Base data directory |
| `--mocks` | bool | false | Use mock implementations (for testing/demo) |

##### Examples

```bash
# Interactive wizard mode (recommended for first-time users)
dvb provision -i

# Provision a devnet with default settings
dvb provision --name my-devnet

# Provision with custom chain ID and 4 validators
dvb provision --name my-devnet --chain-id my-chain-1 --validators 4

# Provision using a pre-built binary
dvb provision --name my-devnet --binary-path /usr/local/bin/stabled

# Provision with 3 validators and 2 full nodes
dvb provision --name my-devnet --validators 3 --full-nodes 2

# Provision with custom data directory
dvb provision --name my-devnet --data-dir /path/to/devnets
```

---

#### start

Start a stopped devnet.

```bash
dvb start <devnet> [flags]
```

##### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `-n, --namespace` | string | | Namespace (defaults to server default) |

##### Examples

```bash
# Start a devnet
dvb start my-devnet

# Start in a specific namespace
dvb start my-devnet -n production
```

---

#### stop

Stop a running devnet.

```bash
dvb stop <devnet> [flags]
```

##### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `-n, --namespace` | string | | Namespace (defaults to server default) |

##### Examples

```bash
# Stop a devnet
dvb stop my-devnet

# Stop in a specific namespace
dvb stop my-devnet -n production
```

---

#### destroy

Destroy a devnet and remove all its data.

```bash
dvb destroy [devnet] [flags]
```

##### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `-n, --namespace` | string | | Namespace (defaults to server default) |
| `-f, --force` | bool | false | Skip confirmation prompt |
| `--data-dir` | string | ~/.devnet-builder | Base data directory |

##### Examples

```bash
# List available devnets to destroy
dvb destroy

# Destroy a devnet (with type-to-confirm prompt)
dvb destroy my-devnet

# Destroy without confirmation (use with caution!)
dvb destroy my-devnet --force
```

---

### Node Management

#### node list

List nodes in a devnet with their status.

```bash
dvb node list <devnet-name> [flags]
```

##### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `-n, --namespace` | string | | Namespace (defaults to server default) |
| `-w, --watch` | bool | false | Watch for changes (like kubectl -w) |
| `--interval` | int | 2 | Watch interval in seconds |
| `--wide` | bool | false | Wide output with additional details |

##### Examples

```bash
# List all nodes in a devnet
dvb node list my-devnet

# Watch node status in real-time
dvb node list my-devnet -w

# Watch with custom interval (5 seconds)
dvb node list my-devnet -w --interval 5

# Wide output with additional details
dvb node list my-devnet --wide
```

##### Sample Output

```
INDEX  HEALTH  ROLE        PHASE    CONTAINER     RESTARTS
0      ●       validator   Running  abc123def456  0
1      ●       validator   Running  def456abc789  0
2      ●       validator   Running  789abc123def  0
3      ●       full        Running  456def789abc  0
```

---

#### node get

Get details of a specific node.

```bash
dvb node get <devnet-name> <index> [flags]
```

##### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `-n, --namespace` | string | | Namespace (defaults to server default) |

##### Examples

```bash
# Get node 0 details
dvb node get my-devnet 0

# Get node in a specific namespace
dvb node get my-devnet 1 -n production
```

---

#### node health

Get health status of a specific node.

```bash
dvb node health <devnet-name> <index>
```

##### Health Status Values

| Status | Description |
|--------|-------------|
| Healthy | Node is running normally |
| Unhealthy | Node has health check failures |
| Stopped | Node is intentionally stopped |
| Transitioning | Node is changing state |
| Unknown | Health cannot be determined |

##### Examples

```bash
# Check health of node 0
dvb node health my-devnet 0

# Check health of node 1
dvb node health my-devnet 1
```

---

#### node ports

Show port mappings for a specific node.

```bash
dvb node ports <devnet-name> <index>
```

##### Examples

```bash
# Show ports for node 0 (host ports: 26656, 26657, 1317, 9090)
dvb node ports my-devnet 0

# Show ports for node 1 (host ports: 26756, 26757, 1417, 9190)
dvb node ports my-devnet 1
```

##### Sample Output

```
Ports for my-devnet/0:

SERVICE    CONTAINER  HOST   PROTOCOL
p2p        26656      26656  tcp
rpc        26657      26657  tcp
rest       1317       1317   tcp
grpc       9090       9090   tcp

RPC endpoint:  http://localhost:26657
REST endpoint: http://localhost:1317
gRPC endpoint: localhost:9090
```

---

#### node start

Start a specific node.

```bash
dvb node start <devnet-name> <index> [flags]
```

##### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `-n, --namespace` | string | | Namespace (defaults to server default) |

##### Examples

```bash
# Start node 1
dvb node start my-devnet 1
```

---

#### node stop

Stop a specific node.

```bash
dvb node stop <devnet-name> <index> [flags]
```

##### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `-n, --namespace` | string | | Namespace (defaults to server default) |

##### Examples

```bash
# Stop node 1
dvb node stop my-devnet 1
```

---

#### node restart

Restart a specific node.

```bash
dvb node restart <devnet-name> <index> [flags]
```

##### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `-n, --namespace` | string | | Namespace (defaults to server default) |

##### Examples

```bash
# Restart node 0
dvb node restart my-devnet 0
```

---

#### node exec

Execute a command inside a running node container.

```bash
dvb node exec <devnet-name> <index> -- <command> [args...]
```

##### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--timeout` | int | 30 | Command timeout in seconds |

##### Examples

```bash
# Check the chain binary version
dvb node exec my-devnet 0 -- stabled version

# List files in the home directory
dvb node exec my-devnet 0 -- ls -la /home/.stable

# Query the node status via RPC
dvb node exec my-devnet 0 -- curl -s localhost:26657/status

# Run a command with a longer timeout
dvb node exec my-devnet 0 --timeout 60 -- stabled query bank balances cosmos1...
```

---

#### node init

Initialize one or more node directories for a devnet.

```bash
dvb node init [flags]
```

##### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--chain-id` | string | | Chain ID for the devnet (required) |
| `--network` | string | stable | Network type (e.g., stable, cosmos) |
| `--data-dir` | string | ~/.devnet-builder/nodes | Base directory for node data |
| `--binary-path` | string | | Path to chain binary (uses network default if not specified) |
| `--num-nodes` | int | 1 | Number of nodes to initialize |
| `--moniker-prefix` | string | validator | Prefix for node monikers |

##### Examples

```bash
# Initialize a single node with default settings
dvb node init --chain-id my-devnet-1

# Initialize 4 validator nodes
dvb node init --chain-id my-devnet-1 --num-nodes 4

# Initialize with custom data directory
dvb node init --chain-id my-devnet-1 --data-dir /path/to/nodes

# Initialize using a specific binary
dvb node init --chain-id my-devnet-1 --binary-path /usr/local/bin/gaiad

# Initialize with custom moniker prefix
dvb node init --chain-id my-devnet-1 --num-nodes 3 --moniker-prefix node
```

---

### Utility Commands

#### version

Print version information.

```bash
dvb version [flags]
```

##### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--long` | bool | false | Show detailed version info including build dependencies |
| `--json` | bool | false | Output version info in JSON format |

##### Examples

```bash
# Show version
dvb version

# Show detailed version
dvb version --long

# Output as JSON
dvb version --json
```

---

#### daemon

Manage the devnetd daemon. All daemon-related functionality is grouped under this command.

```bash
dvb daemon <subcommand>
```

##### Subcommands

| Subcommand | Description |
|------------|-------------|
| `status` | Check daemon status and connectivity (version, latency, mode) |
| `logs` | View daemon logs |
| `whoami` | Show authenticated user information |
| `plugins` | Manage network plugins |

##### Examples

```bash
# Check daemon status with connectivity info
dvb daemon status
# Output:
# ● Daemon is running
#   Socket:  /var/run/devnetd.sock
#   Version: v1.2.3
#   Latency: 1.2ms
#   Mode:    local (trusted)

# View daemon logs
dvb daemon logs -f

# Show current user context
dvb daemon whoami

# List available plugins
dvb daemon plugins list
```

---

#### logs

View logs from devnet nodes.

```bash
dvb logs <devnet> [node] [flags]
```

##### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `-f, --follow` | bool | false | Follow log output (like tail -f) |
| `-n, --tail` | int | 0 | Number of lines to show from the end (0 = all) |
| `--data-dir` | string | ~/.devnet-builder | Base data directory |
| `-t, --timestamps` | bool | false | Show timestamps |

##### Examples

```bash
# Show all logs from a devnet
dvb logs my-devnet

# Show logs from a specific node
dvb logs my-devnet validator-0

# Follow logs in real-time
dvb logs my-devnet -f

# Show last 100 lines
dvb logs my-devnet --tail 100

# Show logs with timestamps
dvb logs my-devnet --timestamps
```

---

### DVB Global Flags

These flags work with all `dvb` commands.

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--standalone` | bool | false | Force standalone mode (don't connect to daemon) |

---

## Legacy devnet-builder CLI

The legacy `devnet-builder` CLI is still available for backward compatibility.

### Main Commands

#### deploy

Deploy a new local devnet with mainnet or testnet state.

```bash
devnet-builder deploy [flags]
```

##### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--validators` | int | 4 | Number of validator nodes |
| `--accounts` | int | 4 | Number of additional funded accounts |
| `-n, --network` | string | mainnet | Network source (mainnet, testnet) |
| `-m, --mode` | string | docker | Execution mode (docker, local) |
| `--image` | string | | Docker image tag (e.g., 1.1.3-mainnet) |
| `--network-version` | string | latest | Network version to use |
| `--no-cache` | bool | false | Skip snapshot cache, download fresh |
| `--no-interactive` | bool | false | Disable interactive mode |
| `--start-version` | string | | Version for devnet binary (non-interactive) |

##### Examples

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
devnet-builder deploy --network-version v1.1.3
```

---

#### init

Initialize devnet configuration without starting nodes.

```bash
devnet-builder init [flags]
```

##### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--validators` | int | 4 | Number of validator nodes (1-4) |
| `--accounts` | int | 4 | Number of additional funded accounts |
| `-n, --network` | string | mainnet | Network source (mainnet, testnet) |
| `-m, --mode` | string | docker | Execution mode (docker, local) |
| `--network-version` | string | latest | Network version to use |
| `--no-cache` | bool | false | Skip snapshot cache, download fresh |
| `--test-mnemonic` | bool | true | Use deterministic test mnemonics |

##### Examples

```bash
# Initialize with default settings
devnet-builder init

# Initialize, then customize config before starting
devnet-builder init --validators 2
# Edit ~/.devnet-builder/devnet/node0/config/config.toml
devnet-builder start
```

---

#### start (legacy)

Start nodes from existing configuration.

```bash
devnet-builder start [flags]
```

##### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--mode` | string | | Override execution mode |
| `--binary-ref` | string | | Binary reference for local mode |
| `--health-timeout` | duration | 5m | Timeout waiting for nodes to be healthy |
| `--network-version` | string | | Network repository version (overrides init version) |

---

#### stop (legacy)

Stop running nodes without removing data.

```bash
devnet-builder stop [flags]
```

##### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--timeout` | duration | 30s | Timeout for graceful shutdown |

---

#### destroy (legacy)

Remove all devnet data and optionally clear cache.

```bash
devnet-builder destroy [flags]
```

##### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--cache` | bool | false | Also remove snapshot cache |
| `--force` | bool | false | Skip confirmation prompt |

---

### Monitoring Commands

#### status

Show current devnet status including node health and endpoints.

```bash
devnet-builder status [flags]
```

##### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--json` | bool | false | Output in JSON format |

##### Sample Output

```
Devnet Status: running
Chain ID: <chain-id>
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

#### logs (legacy)

View node logs with filtering and follow options.

```bash
devnet-builder logs [node] [flags]
```

##### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `-f, --follow` | bool | false | Follow log output |
| `--tail` | int | 100 | Number of lines to show |
| `--since` | duration | | Show logs since duration (e.g., 5m) |

---

#### node (legacy)

Control individual nodes.

```bash
devnet-builder node <subcommand> <node> [flags]
```

##### Subcommands

| Subcommand | Description |
|------------|-------------|
| `start` | Start a specific node |
| `stop` | Stop a specific node |
| `logs` | View logs for a specific node |

---

### Advanced Commands

#### upgrade

Perform software upgrade via expedited governance proposal, or directly replace the binary.

```bash
devnet-builder upgrade [flags]
```

##### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `-n, --name` | string | | Upgrade handler name (required unless --skip-gov) |
| `-i, --image` | string | | Docker image for new version |
| `-b, --binary` | string | | Local binary path for new version |
| `-m, --mode` | string | | Override execution mode for upgrade |
| `--version` | string | | Target version (tag or branch/commit for building) |
| `--export-genesis` | bool | false | Export genesis before/after upgrade |
| `--genesis-dir` | string | | Directory for genesis exports |
| `--height-buffer` | int | 0 | Blocks to add after voting period ends |
| `--voting-period` | duration | 60s | Expedited voting period duration |
| `--skip-gov` | bool | false | Skip governance proposal and directly replace binary |
| `--no-interactive` | bool | false | Disable interactive mode |

---

#### build (legacy)

Build devnet configuration from an exported genesis file.

```bash
devnet-builder build [genesis-export.json] [flags]
```

##### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--validators` | int | 4 | Number of validators |
| `--accounts` | int | 10 | Number of funded accounts |
| `--validator-balance` | string | | Balance for each validator |
| `--account-balance` | string | | Balance for each account |
| `--validator-stake` | string | | Stake amount for each validator |
| `--output` | string | ./devnet | Output directory for devnet files |
| `--chain-id` | string | | Chain ID (defaults to from genesis) |

---

#### export-keys

Export validator and account private keys.

```bash
devnet-builder export-keys [flags]
```

##### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--type` | string | all | Key type: validators, accounts, or all |
| `--json` | bool | false | Output in JSON format |

---

#### reset

Reset chain data while preserving configuration.

```bash
devnet-builder reset [flags]
```

##### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--hard` | bool | false | Also reset configuration |
| `--force` | bool | false | Skip confirmation prompt |

---

### Utility Commands (Legacy)

#### config

Manage devnet-builder configuration.

```bash
devnet-builder config <subcommand> [flags]
```

##### Subcommands

| Subcommand | Description |
|------------|-------------|
| `init` | Create default config.toml |
| `show` | Display current configuration |
| `set` | Set a configuration value |

---

#### cache

Manage binary cache used for upgrades.

```bash
devnet-builder cache <subcommand> [flags]
```

##### Subcommands

| Subcommand | Description |
|------------|-------------|
| `list` | List cached binaries |
| `info` | Show cache size and location |
| `clean` | Remove cached binaries |

---

#### versions

Manage version information and cache.

```bash
devnet-builder versions [flags]
```

##### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--list` | bool | false | List available versions |
| `--refresh` | bool | false | Refresh version list from remote |
| `--clear-cache` | bool | false | Clear version cache |
| `--cache-info` | bool | false | Show cache status (age, expiry) |

---

#### exec

Execute a command in a running node container.

```bash
devnet-builder exec <node> -- <command> [flags]
```

##### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `-t, --tty` | bool | false | Allocate a pseudo-TTY |
| `-i, --interactive` | bool | false | Keep STDIN open |

---

#### port-forward

Forward local ports to a node.

```bash
devnet-builder port-forward <node> [flags]
```

##### Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--address` | string | 127.0.0.1 | Local address to bind |

---

### Legacy Global Flags

These flags work with all `devnet-builder` commands.

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--config` | string | auto-detect | Path to config.toml |
| `-H, --home` | string | ~/.devnet-builder | Base directory for devnet data |
| `--json` | bool | false | Output in JSON format |
| `--no-color` | bool | false | Disable colored output |
| `-v, --verbose` | bool | false | Enable verbose logging |

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

Default ports used by devnet nodes. Each node's ports are offset by `index * 100`:

| Service | Node 0 | Node 1 | Node 2 | Node 3 |
|---------|--------|--------|--------|--------|
| P2P | 26656 | 26756 | 26856 | 26956 |
| RPC | 26657 | 26757 | 26857 | 26957 |
| REST | 1317 | 1417 | 1517 | 1617 |
| gRPC | 9090 | 9190 | 9290 | 9390 |
| EVM RPC | 8545 | - | - | - |
| EVM WS | 8546 | - | - | - |

Note: EVM endpoints are only available on node0.

---

## See Also

- [Getting Started](getting-started.md) - Installation and first deployment
- [Configuration](configuration.md) - config.toml reference
- [Workflows](workflows.md) - Common debugging workflows
- [Troubleshooting](troubleshooting.md) - Common issues and solutions
