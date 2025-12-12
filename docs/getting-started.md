# Getting Started with devnet-builder

This guide walks you through installing devnet-builder and deploying your first local Stable devnet.

## Prerequisites

### Required Tools

| Tool | Version | Purpose | Installation |
|------|---------|---------|--------------|
| Docker | 20.10+ | Run validator nodes | [docker.com](https://docs.docker.com/get-docker/) |
| curl | any | Download snapshots | Usually pre-installed |
| jq | 1.6+ | JSON processing | `apt install jq` / `brew install jq` |
| zstd or lz4 | any | Decompress snapshots | `apt install zstd` / `brew install zstd` |

### Verify Prerequisites

```bash
# Check Docker
docker --version
docker info  # Ensure daemon is running

# Check other tools
curl --version
jq --version
zstd --version || lz4 --version
```

### Operating System Notes

**Linux (Ubuntu/Debian):**
```bash
sudo apt update
sudo apt install -y docker.io curl jq zstd
sudo usermod -aG docker $USER  # Allow Docker without sudo
```

**macOS:**
```bash
# Install Docker Desktop from docker.com
brew install curl jq zstd
```

**Windows (WSL2):**
```bash
# Install WSL2 first, then inside WSL:
sudo apt update
sudo apt install -y docker.io curl jq zstd
```

---

## Installation

### Build from Source

```bash
# Clone the repository
git clone https://github.com/stablelabs/stable-devnet.git
cd stable-devnet

# Build the binary
make build

# Verify installation
./build/devnet-builder version
./build/devnet-builder --help
```

### Optional: Add to PATH

```bash
# Move to system path
sudo cp ./build/devnet-builder /usr/local/bin/

# Or add build directory to PATH
export PATH="$PATH:$(pwd)/build"
```

---

## First Deployment

### Step 1: Deploy the Devnet

```bash
# Deploy with default settings
# - 4 validators
# - Mainnet snapshot data
# - Docker execution mode
./build/devnet-builder deploy
```

This will:
1. Check prerequisites (Docker, curl, jq, zstd)
2. Download mainnet snapshot (~5-10GB, cached for future use)
3. Export genesis state
4. Create 4 validator nodes with keys
5. Start all validators

### Step 2: Verify It's Running

```bash
# Check status
./build/devnet-builder status
```

Expected output:
```
Devnet Status: running
Chain ID: stable_988-1
Execution Mode: docker
Network Source: mainnet

Nodes:
  node0: healthy (height: 12345)
  node1: healthy (height: 12345)
  node2: healthy (height: 12345)
  node3: healthy (height: 12345)

Endpoints:
  RPC:     http://localhost:26657
  EVM:     http://localhost:8545
  gRPC:    localhost:9090
```

### Step 3: Query the Chain

```bash
# Check block height via RPC
curl -s http://localhost:26657/status | jq '.result.sync_info.latest_block_height'

# Check via EVM JSON-RPC
curl -s http://localhost:8545 \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
  | jq -r '.result'
```

### Step 4: View Logs

```bash
# View logs from all nodes
./build/devnet-builder logs

# Follow logs (like tail -f)
./build/devnet-builder logs -f

# View specific node
./build/devnet-builder logs node0
```

### Step 5: Stop the Devnet

```bash
# Stop nodes (preserves data)
./build/devnet-builder down

# Restart later
./build/devnet-builder up

# Remove everything
./build/devnet-builder destroy --force
```

---

## Deployment Options

### Validator Count

```bash
# Single validator (fastest startup, good for basic testing)
devnet-builder deploy --validators 1

# Two validators
devnet-builder deploy --validators 2

# Default: 4 validators (full consensus testing)
devnet-builder deploy --validators 4
```

### Test Accounts

```bash
# Create 5 additional funded accounts
devnet-builder deploy --accounts 5

# Export account keys
devnet-builder export-keys --type accounts
```

### Network Source

```bash
# Use mainnet snapshot (default)
devnet-builder deploy --network mainnet

# Use testnet snapshot
devnet-builder deploy --network testnet
```

### Execution Mode

```bash
# Docker mode (recommended, default)
devnet-builder deploy --mode docker

# Local binary mode (advanced)
devnet-builder deploy --mode local
```

---

## Verification Checklist

After deployment, verify your devnet is working:

- [ ] `devnet-builder status` shows all nodes healthy
- [ ] `curl http://localhost:26657/status` returns JSON
- [ ] `curl http://localhost:8545` responds to EVM requests
- [ ] Block height is increasing (check twice with 5 second gap)
- [ ] Logs show no errors: `devnet-builder logs --tail 20`

---

## Next Steps

- **[Command Reference](commands.md)** - Learn all available commands
- **[Configuration](configuration.md)** - Customize behavior with config.toml
- **[Workflows](workflows.md)** - Common debugging workflows
- **[Troubleshooting](troubleshooting.md)** - Fix common issues
