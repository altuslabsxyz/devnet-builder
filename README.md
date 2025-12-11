# Stable Devnet

> Start a local 4-validator devnet with mainnet state in 3 commands

## TL;DR

```bash
git clone https://github.com/stablelabs/stable-devnet.git && cd stable-devnet
make build
./build/devnet-builder start
```

After ~2 minutes, you'll have a running devnet at:
- **Cosmos RPC**: http://localhost:26657
- **EVM JSON-RPC**: http://localhost:8545
- **Chain ID**: 988

---

## Table of Contents

- [TL;DR](#tldr)
- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [EVM Development](#evm-development)
- [Test Accounts](#test-accounts)
- [CLI Reference](#cli-reference)
- [Version Compatibility](#version-compatibility)
- [CI/CD Integration](#cicd-integration)
- [Architecture](#architecture)
- [Troubleshooting](#troubleshooting)
- [License](#license)

---

## Prerequisites

- Go 1.21+ (for building devnet-builder)
- Docker (for docker mode) OR `stabled` binary (for local mode)
- curl, jq (for network operations)
- zstd (for snapshot decompression)

---

## Installation

```bash
git clone https://github.com/stablelabs/stable-devnet.git
cd stable-devnet
make build

# Binary will be at ./build/devnet-builder
# Optionally move to PATH
sudo mv ./build/devnet-builder /usr/local/bin/
```

Verify installation:

```bash
devnet-builder version
devnet-builder --help
```

---

## Quick Start

### Docker Mode (Recommended)

```bash
# Start with default settings (4 validators, mainnet data)
devnet-builder start

# Check status
devnet-builder status

# Stop when done
devnet-builder stop
```

### Local Binary Mode

```bash
# Ensure stabled is in PATH
devnet-builder start --mode local
```

### Common Options

```bash
# Single validator (fastest startup)
devnet-builder start --validators 1

# With test accounts
devnet-builder start --accounts 5

# Use testnet data instead of mainnet
devnet-builder start --network testnet

# Specify stable version (branch, tag, or commit)
devnet-builder start --stable-version v1.2.3
devnet-builder start --stable-version feat/my-feature
```

---

## EVM Development

### Network Configuration

| Parameter | Value |
|-----------|-------|
| Network Name | Stable Devnet |
| Chain ID | 988 |
| Currency Symbol | USDT |
| RPC URL | http://localhost:8545 |
| WebSocket | ws://localhost:8546 |

### MetaMask Setup

1. Open MetaMask > Settings > Networks > Add Network
2. Enter the configuration above
3. Import test account using private key from `devnet-builder export-keys`

### Send Test Transaction (cast)

```bash
# Export test account private key
eval $(devnet-builder export-keys --format env)

# Send transaction using Foundry's cast
cast send --rpc-url http://localhost:8545 \
  --private-key $ACCOUNT_0_PRIVATE_KEY \
  0x0000000000000000000000000000000000000000 \
  --value 1ether
```

### Deploy Contract (forge)

```bash
# Deploy a simple contract
forge create --rpc-url http://localhost:8545 \
  --private-key $ACCOUNT_0_PRIVATE_KEY \
  src/MyContract.sol:MyContract
```

---

## Test Accounts

Devnet creates pre-funded test accounts for development.

### Export All Keys

```bash
# View all test accounts
devnet-builder export-keys

# JSON format
devnet-builder export-keys --format json

# Set as environment variables
eval $(devnet-builder export-keys --format env)

# Export only accounts (not validators)
devnet-builder export-keys --type accounts
```

### Account Details

When starting with `--accounts N`, devnet-builder creates N test accounts:

| Account | Cosmos Address | EVM Address | Balance |
|---------|----------------|-------------|---------|
| account0 | stable1... | 0x... | 10,000 USDT |
| account1 | stable1... | 0x... | 10,000 USDT |
| ... | ... | ... | ... |

### Mnemonic Recovery

Each account is derived from a unique mnemonic stored in:
`~/.stable-devnet/devnet/accounts/account{i}.json`

```bash
# View account mnemonic
cat ~/.stable-devnet/devnet/accounts/account0.json | jq -r '.mnemonic'
```

### HD Derivation Path

- Cosmos: `m/44'/118'/0'/0/0`
- Ethereum: `m/44'/60'/0'/0/0`

---

## CLI Reference

### Lifecycle Commands

| Command | Description |
|---------|-------------|
| `devnet-builder start` | Start the devnet |
| `devnet-builder stop` | Stop all nodes |
| `devnet-builder restart` | Restart the devnet |
| `devnet-builder status` | Show devnet status |
| `devnet-builder logs [node]` | View node logs |

### Data Management

| Command | Description |
|---------|-------------|
| `devnet-builder reset` | Reset chain data (keep config) |
| `devnet-builder reset --hard` | Full reset (re-provision required) |
| `devnet-builder clean` | Remove devnet data |
| `devnet-builder clean --cache` | Remove data AND snapshot cache |

### Key Export

| Command | Description |
|---------|-------------|
| `devnet-builder export-keys` | Human-readable format |
| `devnet-builder export-keys --format json` | JSON format |
| `devnet-builder export-keys --format env` | Environment variables |

### Start Options

| Flag | Description | Default |
|------|-------------|---------|
| `--validators N` | Number of validators (1-4) | 4 |
| `--accounts N` | Number of test accounts | 0 |
| `--network` | mainnet or testnet | mainnet |
| `--mode` | docker or local | docker |
| `--stable-version` | Branch/tag/commit to build | latest |
| `--no-cache` | Force re-download snapshot | false |
| `--no-interactive` | Skip prompts (for CI) | false |

### Interactive Version Selection

When `--stable-version` is not specified, devnet-builder offers interactive selection:

```bash
devnet-builder start
# Prompts: Select stable version
# Options: main, latest tag, or custom branch/commit
```

---

## Version Compatibility

| Component | Minimum | Recommended | Notes |
|-----------|---------|-------------|-------|
| Go | 1.21 | 1.23 | Required for building |
| Docker | 20.10 | 24.0 | For docker mode |
| zstd | 1.4 | 1.5 | For snapshot decompression |
| OS | Ubuntu 20.04, macOS 12 | Ubuntu 22.04, macOS 14 | Linux/macOS only |

### Windows Support

Windows is supported via WSL2:

```bash
# Install WSL2
wsl --install

# Inside WSL2
sudo apt update && sudo apt install -y golang docker.io zstd
```

---

## CI/CD Integration

### GitHub Actions Example

```yaml
name: Integration Tests

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.23'

      - name: Build devnet-builder
        run: make build

      - name: Start devnet
        run: |
          ./build/devnet-builder start --validators 1 --no-interactive
          sleep 30

      - name: Health check
        run: |
          curl -s http://localhost:26657/status | jq -e '.result.sync_info.catching_up == false'

      - name: Run integration tests
        run: go test ./tests/integration/...

      - name: Cleanup
        if: always()
        run: ./build/devnet-builder clean --force
```

### Non-Interactive Mode

For CI environments, use `--no-interactive` to skip prompts:

```bash
devnet-builder start --no-interactive
devnet-builder clean --force
```

### JSON Output

Machine-parseable status for scripts:

```bash
# Get status as JSON
devnet-builder status --json

# Check if running
devnet-builder status --json | jq -e '.status == "running"'

# Get chain ID
devnet-builder status --json | jq -r '.chain_id'
```

---

## Architecture

```
                    ┌─────────────────────────────────────────┐
                    │           devnet-builder CLI            │
                    │  (provision, build, start, stop, ...)   │
                    └─────────────────┬───────────────────────┘
                                      │
            ┌─────────────────────────┼─────────────────────────┐
            │                         │                         │
            ▼                         ▼                         ▼
    ┌───────────────┐       ┌───────────────┐       ┌───────────────┐
    │  Mainnet RPC  │       │   S3 Bucket   │       │    Docker/    │
    │  (genesis)    │       │  (snapshot)   │       │    Local      │
    └───────┬───────┘       └───────┬───────┘       └───────┬───────┘
            │                       │                       │
            └───────────────────────┼───────────────────────┘
                                    │
                    ┌───────────────┴───────────────┐
                    │      Devnet Directory         │
                    │   ~/.stable-devnet/devnet/    │
                    ├───────────────────────────────┤
                    │  node0/  node1/  node2/  ...  │
                    │  accounts/  metadata.json     │
                    └───────────────┬───────────────┘
                                    │
         ┌──────────────────────────┼──────────────────────────┐
         │                          │                          │
         ▼                          ▼                          ▼
    ┌─────────┐               ┌─────────┐               ┌─────────┐
    │  Node0  │◄─────────────►│  Node1  │◄─────────────►│  Node2  │
    │  :26657 │   P2P Mesh    │  :36657 │   P2P Mesh    │  :46657 │
    │  :8545  │               │  :18545 │               │  :28545 │
    └─────────┘               └─────────┘               └─────────┘
```

### Data Flow

1. **Provision**: Download genesis from mainnet RPC, snapshot from S3
2. **Build**: Export state, create validators, generate test accounts
3. **Start**: Launch validator nodes with P2P networking
4. **Run**: Validators produce blocks, EVM and Cosmos APIs available

### Endpoints

| Service | Node 0 | Node 1 | Node 2 | Node 3 |
|---------|--------|--------|--------|--------|
| Cosmos RPC | 26657 | 36657 | 46657 | 56657 |
| P2P | 26656 | 36656 | 46656 | 56656 |
| EVM JSON-RPC | 8545 | 18545 | 28545 | 38545 |
| EVM WebSocket | 8546 | 18546 | 28546 | 38546 |
| gRPC | 9090 | 19090 | 29090 | 39090 |

---

## Troubleshooting

### Port Already in Use

```bash
# Find process using port
lsof -i :26657

# Kill specific process
kill -9 <PID>

# Or kill all stabled processes
pkill -f stabled
```

### Docker Container Issues

```bash
# List running containers
docker ps -a | grep stable

# Remove stuck containers
docker rm -f $(docker ps -aq --filter name=stable)

# Check Docker daemon
systemctl status docker
```

### Snapshot Download Fails

```bash
# Clear snapshot cache and retry
devnet-builder clean --cache
devnet-builder start

# Manual download (for debugging)
curl -I https://stable-mainnet-data.s3.amazonaws.com/snapshots/stable_pruned.tar.zst
```

### Node Won't Start

```bash
# Check node logs
devnet-builder logs node0

# Common issues:
# - "address already in use" -> ports conflict
# - "validator set is nil" -> genesis issue
# - "panic: runtime error" -> version mismatch
```

### Chain Not Producing Blocks

```bash
# Check if nodes are connected
curl -s http://localhost:26657/net_info | jq '.result.n_peers'

# Check consensus state
curl -s http://localhost:26657/consensus_state | jq '.result.round_state.height'
```

### Reset to Clean State

```bash
# Soft reset (keep genesis)
devnet-builder reset

# Hard reset (re-provision)
devnet-builder reset --hard

# Full clean (remove everything)
devnet-builder clean --cache --force
```

---

## Environment Variables

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

## Directory Structure

```
~/.stable-devnet/
├── devnet/
│   ├── node0/                 # Validator 0 home directory
│   ├── node1/                 # Validator 1 home directory
│   ├── node2/                 # Validator 2 home directory
│   ├── node3/                 # Validator 3 home directory
│   ├── accounts/              # Test account keyrings
│   └── metadata.json          # Devnet configuration
├── snapshots/                 # Downloaded snapshots (cached)
└── genesis/                   # Downloaded genesis files
```

---

## License

Refer to the main Stable repository for license information.
