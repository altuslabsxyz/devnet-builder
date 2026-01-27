# Getting Started with devnet-builder

This guide walks you through installing devnet-builder and deploying your first local blockchain development network.

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
git clone https://github.com/altuslabsxyz/devnet-builder.git
cd devnet-builder

# Build all binaries
make build

# This creates three binaries in ./build/:
#   devnet-builder  - Main interactive CLI
#   dvb             - Daemon-based kubectl-style CLI
#   devnetd         - Daemon server (for dvb)

# Verify installation
./build/devnet-builder version
./build/devnet-builder --help
```

### Install to GOPATH

```bash
# Install all binaries to $GOPATH/bin
make install
```

### Optional: Add to PATH

```bash
# Move to system path
sudo cp ./build/devnet-builder /usr/local/bin/
sudo cp ./build/dvb /usr/local/bin/
sudo cp ./build/devnetd /usr/local/bin/

# Or add build directory to PATH
export PATH="$PATH:$(pwd)/build"
```

---

## First Deployment

You can deploy devnets using either `devnet-builder` (standalone) or `dvb` (daemon-based). Choose the approach that fits your workflow.

### Option A: Using devnet-builder (Recommended for Beginners)

#### Step 1: Deploy the Devnet

```bash
# Deploy with default settings
# - 4 validators
# - Mainnet snapshot data
# - Docker execution mode
./build/devnet-builder deploy
```

This will:
1. Check prerequisites (Docker, curl, jq, zstd)
2. Download network snapshot (~5-10GB, cached for future use)
3. Export genesis state
4. Create 4 validator nodes with keys
5. Start all validators

#### Step 2: Verify It's Running

```bash
# Check status
./build/devnet-builder status
```

Example output:
```
Chain ID:     stable-devnet
Network:      mainnet
Blockchain:   stable
Mode:         docker
Validators:   4

Endpoints:
  Node 0: http://localhost:26657 (RPC) | http://localhost:8545 (EVM)
  Node 1: http://localhost:26757 (RPC) | http://localhost:8645 (EVM)
  ...
```

#### Step 3: View Logs

```bash
# View logs from all nodes (last 100 lines by default)
./build/devnet-builder logs

# Follow logs (like tail -f)
./build/devnet-builder logs -f

# View specific node (0-indexed)
./build/devnet-builder logs 0
./build/devnet-builder logs node0

# Show last 50 lines
./build/devnet-builder logs --tail 50

# Filter by log level
./build/devnet-builder logs --level error
```

#### Step 4: Stop the Devnet

```bash
# Stop nodes (preserves data)
./build/devnet-builder stop

# Restart later
./build/devnet-builder start

# Remove everything (requires confirmation)
./build/devnet-builder destroy

# Remove without confirmation (use with caution!)
./build/devnet-builder destroy --force
```

---

### Option B: Using dvb with Daemon (kubectl-style)

The `dvb` CLI provides a declarative, YAML-driven approach similar to kubectl.

#### Step 1: Start the Daemon

```bash
# Start devnetd (runs in foreground by default)
./build/devnetd

# Or with custom options
./build/devnetd --data-dir ~/.devnet-builder --log-level info
```

#### Step 2: Provision Using Interactive Wizard

```bash
# Interactive wizard mode (recommended for first-time users)
./build/dvb provision -i
```

The wizard will guide you through:
- Devnet name
- Network type (stable, cosmos, etc.)
- Number of validators
- Chain ID

#### Step 2 Alternative: Apply YAML Configuration

```bash
# Create a devnet configuration file
cat > my-devnet.yaml << EOF
apiVersion: devnet.lagos/v1
kind: Devnet
metadata:
  name: my-devnet
  namespace: default
spec:
  network: stable
  networkType: mainnet
  validators: 4
  mode: docker
EOF

# Apply the configuration
./build/dvb apply -f my-devnet.yaml

# Preview changes without applying
./build/dvb apply -f my-devnet.yaml --dry-run
```

#### Step 3: Check Status

```bash
# List all devnets
./build/dvb get devnets
./build/dvb list  # alias

# Get detailed info about a specific devnet
./build/dvb describe my-devnet

# Output in different formats
./build/dvb get devnets -o wide
./build/dvb get devnet my-devnet -o yaml
./build/dvb get devnet my-devnet -o json
```

#### Step 4: Manage Lifecycle

```bash
# Stop a devnet
./build/dvb stop my-devnet

# Start a stopped devnet
./build/dvb start my-devnet

# View logs
./build/dvb logs my-devnet
./build/dvb logs my-devnet validator-0 -f

# Destroy a devnet
./build/dvb destroy my-devnet
./build/dvb destroy my-devnet --force
```

#### Step 5: Check Daemon Status

```bash
# Check if daemon is running
./build/dvb daemon status
```

---

## Query the Chain

```bash
# Check block height via RPC
curl -s http://localhost:26657/status | jq '.result.sync_info.latest_block_height'

# Check via EVM JSON-RPC (if supported by network)
curl -s http://localhost:8545 \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
  | jq -r '.result'
```

---

## Deployment Options

### Validator Count

```bash
# Docker mode: 1-100 validators
devnet-builder deploy --validators 1   # Single validator (fastest)
devnet-builder deploy --validators 4   # Default (recommended)
devnet-builder deploy --validators 10  # Large scale testing

# Local mode: 1-4 validators only
devnet-builder deploy --mode local --validators 2
```

### Test Accounts

```bash
# Create 5 additional funded accounts (default: 4)
devnet-builder deploy --accounts 5

# Export account keys after deployment
devnet-builder export-keys
devnet-builder export-keys --json
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
# - Isolated network, automatic port management
# - Supports 1-100 validators
devnet-builder deploy --mode docker

# Local binary mode (advanced)
# - Uses local filesystem
# - Supports 1-4 validators
# - Requires chain binary
devnet-builder deploy --mode local
```

### Blockchain Network

```bash
# Select blockchain network module
devnet-builder deploy --blockchain stable  # Default
devnet-builder deploy --blockchain ault
```

### Fork Mode

```bash
# Fork live network state via snapshot export (default: enabled)
devnet-builder deploy --fork

# Disable forking (use RPC genesis instead)
devnet-builder deploy --fork=false
```

---

## Verification Checklist

After deployment, verify your devnet is working:

- [ ] `devnet-builder status` shows all nodes running
- [ ] `curl http://localhost:26657/status` returns JSON
- [ ] `curl http://localhost:8545` responds to EVM requests (if supported)
- [ ] Block height is increasing (check twice with 5 second gap)
- [ ] Logs show no errors: `devnet-builder logs --level error`

---

## Next Steps

- **[Command Reference](commands.md)** - Learn all available commands
- **[Configuration](configuration.md)** - Customize behavior with config.toml
- **[YAML Devnet Guide](yaml-devnet-guide.md)** - Using dvb with YAML configurations
- **[Workflows](workflows.md)** - Common debugging workflows
- **[Troubleshooting](troubleshooting.md)** - Fix common issues
