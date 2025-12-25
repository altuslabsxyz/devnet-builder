# Debugging Workflows

Step-by-step guides for common development and debugging scenarios.

## Table of Contents

- [Upgrade Testing Workflow](#upgrade-testing-workflow)
- [Key Export Workflow](#key-export-workflow)
- [State Reset Workflow](#state-reset-workflow)
- [Docker vs Local Mode](#docker-vs-local-mode)

---

## Upgrade Testing Workflow

Test software upgrades using expedited governance proposals.

### Overview

The upgrade workflow allows you to test chain upgrades in a controlled environment:
1. Deploy devnet with initial version
2. Submit upgrade proposal via governance
3. Validators vote YES automatically
4. Chain halts at upgrade height
5. Switch to new binary/image
6. Chain resumes with upgraded software

### Step-by-Step Guide

#### Step 1: Deploy Initial Version

```bash
# Deploy with specific initial version
devnet-builder deploy --image <docker-registry>/<network-image>:v1.0.0
```

#### Step 2: Verify Chain is Running

```bash
# Check status
devnet-builder status

# Verify block production
curl -s http://localhost:26657/status | jq '.result.sync_info.latest_block_height'
```

#### Step 3: Initiate Upgrade

```bash
# Start upgrade to new version
devnet-builder upgrade \
  --name v2-upgrade \
  --image <docker-registry>/<network-image>:v2.0.0
```

This will:
- Submit an expedited governance proposal
- All validators vote YES automatically
- Wait for upgrade height
- Restart nodes with new image

#### Step 4: Monitor Upgrade

```bash
# Watch logs during upgrade
devnet-builder logs -f

# Check upgrade progress
devnet-builder status
```

#### Step 5: Verify Post-Upgrade

```bash
# Check chain is producing blocks
curl -s http://localhost:26657/status | jq '.result.sync_info'

# Verify version
curl -s http://localhost:26657/abci_info | jq '.result.response.version'
```

### Export Genesis During Upgrade

For debugging upgrade issues, export genesis before and after:

```bash
devnet-builder upgrade \
  --name v2-upgrade \
  --image v2.0.0-mainnet \
  --export-genesis
```

Genesis files saved to:
- `~/.devnet-builder/genesis/pre-upgrade-v2-upgrade.json`
- `~/.devnet-builder/genesis/post-upgrade-v2-upgrade.json`

### Upgrade with Local Binary

For testing locally-built binaries:

```bash
# Build your binary first
cd /path/to/network
make build

# Upgrade using local binary
devnet-builder upgrade \
  --name v2-upgrade \
  --binary /path/to/network/build/<binary-name> \
  --mode local
```

---

## Key Export Workflow

Export validator and account keys for testing.

### Export All Keys

```bash
# Export all keys (validators + accounts)
devnet-builder export-keys
```

Output:
```
Validators:
  node0: <prefix>1abc...
    Private Key: 0x1234...
  node1: <prefix>1def...
    Private Key: 0x5678...

Accounts:
  account0: <prefix>1ghi...
    Private Key: 0xabcd...
```

### Export for Scripts

```bash
# JSON format for scripting
devnet-builder export-keys --json > keys.json

# Parse with jq
cat keys.json | jq '.validators[0].private_key'
```

### Export Specific Types

```bash
# Only validator keys
devnet-builder export-keys --type validators

# Only account keys
devnet-builder export-keys --type accounts
```

### Use Keys with Foundry

```bash
# Get first account private key
PRIVATE_KEY=$(devnet-builder export-keys --json | jq -r '.accounts[0].private_key')

# Use with cast
cast send \
  --rpc-url http://localhost:8545 \
  --private-key $PRIVATE_KEY \
  0xRecipientAddress \
  --value 1ether
```

### Use Keys with ethers.js

```javascript
const keys = require('./keys.json');
const { ethers } = require('ethers');

const provider = new ethers.JsonRpcProvider('http://localhost:8545');
const wallet = new ethers.Wallet(keys.accounts[0].private_key, provider);

// Now use wallet for transactions
```

---

## State Reset Workflow

Reset chain state while preserving configuration.

### Soft Reset (Keep Config)

Reset chain data but keep keys and configuration:

```bash
# Stop nodes
devnet-builder down

# Reset state
devnet-builder reset

# Restart
devnet-builder up
```

This preserves:
- Validator keys
- Account keys
- config.toml settings
- Genesis file

### Hard Reset (Full Reset)

Reset everything including configuration:

```bash
# Full reset
devnet-builder reset --hard --force

# Redeploy
devnet-builder deploy
```

### Reset and Redeploy Fresh

When you need a completely fresh start:

```bash
# Destroy everything
devnet-builder destroy --force

# Fresh deployment
devnet-builder deploy
```

### Partial Reset: Single Node

To reset a specific node:

```bash
# Stop the node
devnet-builder node stop node2

# Clear node data manually
rm -rf ~/.devnet-builder/devnet/node2/data/*

# Restart node
devnet-builder node start node2
```

Note: The node will sync from other validators.

---

## Docker vs Local Mode

Choose the right execution mode for your use case.

### Docker Mode (Recommended)

Best for:
- Quick setup
- Consistent environment
- No local Go installation needed
- Testing with official releases

```bash
# Deploy with Docker (default)
devnet-builder deploy --mode docker

# Use specific image
devnet-builder deploy --mode docker --image 1.1.3-mainnet
```

Advantages:
- Pre-built images, fast startup
- Isolated from host system
- Easy cleanup
- Reproducible across machines

### Local Mode (Advanced)

Best for:
- Testing local code changes
- Debugging with local tools
- Performance-critical testing
- Custom binary testing

```bash
# Deploy with local binaries
devnet-builder deploy --mode local

# Use specific binary
devnet-builder deploy --mode local --binary-ref /path/to/<binary-name>
```

Requirements:
- Go 1.23+ installed
- Docker (for building)
- Build dependencies

### Switching Modes

You can switch modes for an existing devnet:

```bash
# Started with Docker
devnet-builder deploy --mode docker

# Stop nodes
devnet-builder down

# Restart with local binary (for debugging)
devnet-builder up --mode local --binary-ref /path/to/debug-<binary-name>
```

### Mode Comparison

| Aspect | Docker Mode | Local Mode |
|--------|-------------|------------|
| Setup Time | Fast | Requires Go setup |
| Binary Source | Pre-built images | Built from source |
| Debugging | Container logs | Native tools (delve, etc.) |
| Performance | Good | Better (no container overhead) |
| Isolation | Full container | Shared host |
| Upgrades | Switch images | Build/cache binaries |

### Hybrid Workflow

Use both modes for different phases:

```bash
# Initial testing with Docker (fast)
devnet-builder deploy --mode docker
# Run tests...

# Deep debugging with local binary
devnet-builder destroy --force
devnet-builder deploy --mode local

# Final validation with Docker (production-like)
devnet-builder destroy --force
devnet-builder deploy --mode docker
```

---

## Common Debugging Commands

### Check Node Health

```bash
# Quick status
devnet-builder status

# Detailed node info
curl -s http://localhost:26657/status | jq

# Check peers
curl -s http://localhost:26657/net_info | jq '.result.n_peers'
```

### View Consensus State

```bash
# Consensus state
curl -s http://localhost:26657/consensus_state | jq

# Validators
curl -s http://localhost:26657/validators | jq '.result.validators[].address'
```

### Query EVM

```bash
# Block number
curl -s http://localhost:8545 \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
  | jq -r '.result'

# Chain ID
curl -s http://localhost:8545 \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"eth_chainId","params":[],"id":1}' \
  | jq -r '.result'
```

### Check Logs for Errors

```bash
# Recent errors
devnet-builder logs --tail 100 | grep -i error

# Follow logs for specific node
devnet-builder logs node0 -f
```

---

## See Also

- [Command Reference](commands.md) - Complete CLI documentation
- [Configuration](configuration.md) - config.toml options
- [Troubleshooting](troubleshooting.md) - Common issues and solutions
