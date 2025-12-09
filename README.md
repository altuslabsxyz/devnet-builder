# Stable Devnet

Local development network for Stable blockchain using mainnet/testnet state export.

## Prerequisites

- Go 1.21+ (for building devnet-builder)
- Docker (for docker mode) OR `stabled` binary (for local mode)
- curl, jq (for network operations)
- zstd or lz4 (for snapshot decompression)

## Installation

### Build from source

```bash
git clone https://github.com/stablelabs/stable-devnet.git
cd stable-devnet
make build

# Binary will be at ./build/devnet-builder
# Optionally move to PATH
sudo mv ./build/devnet-builder /usr/local/bin/
```

### Verify installation

```bash
devnet-builder version
devnet-builder --help
```

---

## Quick Start

### 1. Docker Mode (Recommended)

Start a 4-validator devnet using Docker containers:

```bash
# Start with default settings
devnet-builder start

# Check status
devnet-builder status

# Stop when done
devnet-builder stop
```

### 2. Local Binary Mode

Start using a local `stabled` binary:

```bash
# Ensure stabled is in PATH or specify location
devnet-builder start --mode local

# If stabled is not in PATH, build it first:
# git clone https://github.com/stablelabs/stable && cd stable && make install
```

---

## Usage Guide

### Starting a Devnet

```bash
# Default: 4 validators, mainnet data, docker mode
devnet-builder start

# Specify number of validators (1-4)
devnet-builder start --validators 2

# Use testnet data instead of mainnet
devnet-builder start --network testnet

# Use local binary mode instead of Docker
devnet-builder start --mode local

# Skip snapshot cache (force re-download)
devnet-builder start --no-cache

# Specify stable version for building
devnet-builder start --stable-version v1.2.3

# Full example with all options
devnet-builder start \
  --network mainnet \
  --validators 4 \
  --mode docker \
  --accounts 10
```

### Network Selection

| Network | Description | Snapshot Source |
|---------|-------------|-----------------|
| `mainnet` | Production mainnet state | stable-mainnet-data.s3.amazonaws.com |
| `testnet` | Testnet state | stable-testnet-data.s3.amazonaws.com |

```bash
# Use mainnet data (default)
devnet-builder start --network mainnet

# Use testnet data
devnet-builder start --network testnet
```

### Validator Count

Choose 1-4 validators based on your testing needs:

```bash
# Single validator (fastest, minimal resources)
devnet-builder start --validators 1

# Two validators (basic consensus testing)
devnet-builder start --validators 2

# Four validators (full devnet, default)
devnet-builder start --validators 4
```

### Execution Modes

#### Docker Mode (Default)

- Requires Docker to be installed and running
- Each node runs in an isolated container
- Easier cleanup and isolation

```bash
devnet-builder start --mode docker
```

#### Local Binary Mode

- Requires `stabled` binary in PATH
- Nodes run as background processes
- Useful when Docker is unavailable

```bash
# Ensure stabled is installed
which stabled

# Start in local mode
devnet-builder start --mode local
```

---

## Lifecycle Management

### Check Status

```bash
# Show devnet status and node health
devnet-builder status

# JSON output for scripting
devnet-builder status --json
```

Output shows:
- Chain ID, network, mode
- Node status (running/stopped/syncing)
- Block height, peer count, catching_up status

### Stop Devnet

```bash
# Graceful stop (default 30s timeout)
devnet-builder stop

# Custom timeout
devnet-builder stop --timeout 60s
```

### Restart Devnet

```bash
# Stop and start again
devnet-builder restart

# With custom timeout
devnet-builder restart --timeout 60s
```

### Reset Chain Data

```bash
# Soft reset: Clear chain data, keep genesis and config
devnet-builder reset

# Hard reset: Clear everything (requires re-provisioning)
devnet-builder reset --hard

# Skip confirmation prompt
devnet-builder reset --force
```

### Clean Up

```bash
# Remove devnet data (keeps snapshot cache)
devnet-builder clean

# Remove devnet data AND snapshot cache
devnet-builder clean --cache

# Skip confirmation prompt
devnet-builder clean --force
```

---

## Viewing Logs

```bash
# View logs from all nodes
devnet-builder logs

# View logs from specific node
devnet-builder logs node0
devnet-builder logs node1

# Follow logs in real-time (like tail -f)
devnet-builder logs -f
devnet-builder logs node0 -f

# Show last N lines
devnet-builder logs --tail 50

# Logs since specific duration
devnet-builder logs --since 5m
```

---

## Exporting Keys

Export validator and account keys for testing:

```bash
# Human-readable format
devnet-builder export-keys

# JSON format
devnet-builder export-keys --format json

# Environment variables (eval-able)
eval $(devnet-builder export-keys --format env)

# Export only validators
devnet-builder export-keys --type validators

# Export only accounts
devnet-builder export-keys --type accounts
```

---

## Backup and Restore

### Backup Devnet Data

```bash
# Backup the entire devnet directory
tar -czvf devnet-backup-$(date +%Y%m%d).tar.gz ~/.stable-devnet/devnet/

# Backup only chain data (smaller)
tar -czvf chaindata-backup-$(date +%Y%m%d).tar.gz \
  ~/.stable-devnet/devnet/node*/data/
```

### Restore from Backup

```bash
# Stop devnet first
devnet-builder stop

# Restore backup
tar -xzvf devnet-backup-20241209.tar.gz -C ~/

# Start devnet
devnet-builder start
```

### Backup Snapshot Cache

```bash
# Snapshot cache location
ls -la ~/.stable-devnet/snapshots/

# Backup snapshot cache (saves re-download time)
tar -czvf snapshot-cache.tar.gz ~/.stable-devnet/snapshots/
```

---

## Environment Variables

Configure defaults via environment variables:

```bash
# Set default home directory
export STABLE_DEVNET_HOME=~/.my-devnet

# Set default network
export STABLE_DEVNET_NETWORK=testnet

# Set default execution mode
export STABLE_DEVNET_MODE=local

# Set default stable version
export STABLE_VERSION=v1.2.3

# Disable colored output
export NO_COLOR=1
```

---

## Common Workflows

### Development Cycle

```bash
# 1. Start fresh devnet
devnet-builder start

# 2. Develop and test your changes
# ... make code changes ...

# 3. Reset chain data to test again
devnet-builder reset
devnet-builder start

# 4. Clean up when done
devnet-builder clean
```

### Testing Feature Branch

```bash
# Build with your feature branch
devnet-builder build --stable-version feat/my-feature

# Start devnet with that version
devnet-builder start --stable-version feat/my-feature

# Test your changes
# ...

# Compare with main
devnet-builder clean
devnet-builder start --stable-version main
```

### Integration Testing

```bash
# Start devnet
devnet-builder start

# Export keys for test scripts
eval $(devnet-builder export-keys --format env)

# Run integration tests
go test ./tests/integration/...

# Clean up
devnet-builder clean
```

### CI/CD Pipeline

```bash
# Start devnet in JSON mode for parsing
devnet-builder start --json > devnet-output.json

# Wait for health check
sleep 30
devnet-builder status --json | jq -e '.status == "running"'

# Run tests
./run-tests.sh

# Cleanup
devnet-builder clean --force
```

---

## Shell Completion

Enable tab completion for your shell:

### Bash

```bash
# Add to ~/.bashrc
source <(devnet-builder completion bash)

# Or install system-wide
devnet-builder completion bash > /etc/bash_completion.d/devnet-builder
```

### Zsh

```bash
# Add to ~/.zshrc
source <(devnet-builder completion zsh)
```

### Fish

```bash
# Add to ~/.config/fish/config.fish
devnet-builder completion fish | source
```

## Network Configuration

### RPC Endpoints

- Mainnet Cosmos RPC: `https://p40zma3acd216e70s-cosmos-rpc.stable.xyz`
- Testnet Cosmos RPC: `https://cosmos-rpc.testnet.stable.xyz`

### Snapshot Source

- Mainnet: `https://stable-mainnet-data.s3.amazonaws.com/snapshots/stable_pruned.tar.zst`

## Endpoints

| Service | Node 0 | Node 1 | Node 2 | Node 3 |
|---------|--------|--------|--------|--------|
| RPC | 26657 | 36657 | 46657 | 56657 |
| P2P | 26656 | 36656 | 46656 | 56656 |
| EVM JSON-RPC | 8545 | 18545 | 28545 | 38545 |
| EVM WebSocket | 8546 | 18546 | 28546 | 38546 |
| gRPC | 9090 | 19090 | 29090 | 39090 |

## How It Works

1. **Snapshot Download**: Downloads the latest mainnet pruned snapshot (~13GB)
2. **Genesis Download**: Fetches genesis from mainnet RPC endpoint
3. **State Export**: Uses `stabled export` to export current state from snapshot
4. **Devnet Build**: `devnet-builder` creates:
   - 4 new validators with fresh keys
   - 10 test accounts with tokens
   - Redistributes tokens from largest holder
   - Clears old staking state and creates new validators
   - Preserves total supply (no inflation/deflation)
5. **Node Start**: Starts 4 validator nodes with unique ports

## Directory Structure

```
stable-devnet/
├── cmd/devnet-builder/        # Go devnet builder tool
├── scripts/                   # Local devnet management scripts
│   ├── local-devnet.sh        # Main CLI script
│   ├── provision-and-sync.sh  # Snapshot/genesis helpers
│   └── manage-devnet.sh       # Lifecycle management helpers
├── devnet/                    # Generated devnet data (gitignored)
│   ├── accounts/              # Test account keyrings
│   ├── node0/                 # Validator 0 home directory
│   ├── node1/                 # Validator 1 home directory
│   ├── node2/                 # Validator 2 home directory
│   └── node3/                 # Validator 3 home directory
└── ~/.stable-devnet/          # Cached data (user home)
    ├── snapshots/             # Downloaded snapshots
    │   └── mainnet/
    └── genesis/               # Downloaded genesis files
        └── mainnet-genesis.json
```

## Building devnet-builder

```bash
# Build the devnet-builder tool
make build

# The binary will be at ./build/devnet-builder
```

## Troubleshooting

### Nodes not starting
Check the node logs:
```bash
cat devnet/node0/node.log
```

### Port conflicts
Make sure ports 26656-26657, 36656-36657, 46656-46657, 56656-56657, 8545, 18545, 28545, 38545, 9090, 19090, 29090, 39090 are available.

### Kill all stabled processes
```bash
pkill -f stabled
```

### Clean and restart
```bash
./scripts/local-devnet.sh clean
./scripts/local-devnet.sh start --local-binary /path/to/stabled
```

## License

Refer to the main Stable repository for license information.
