# Debugging Workflows

Step-by-step guides for common development and debugging scenarios.

## Table of Contents

- [Upgrade Testing Workflow](#upgrade-testing-workflow)
- [Quick Upgrade (Skip Governance)](#quick-upgrade-skip-governance)
- [Resuming Interrupted Upgrades](#resuming-interrupted-upgrades)
- [Key Export Workflow](#key-export-workflow)
- [State Reset Workflow](#state-reset-workflow)
- [Docker vs Local ExecutionMode](#docker-vs-local-mode)
- [dvb CLI Workflows (Daemon Mode)](#dvb-cli-workflows-daemon-mode)

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
# Start upgrade to new version (interactive binary selection)
devnet-builder upgrade \
  --name v2-upgrade \
  --version v2.0.0

# Or specify image directly
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

# Show detailed upgrade status
devnet-builder upgrade --show-status
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
  --version v2.0.0 \
  --with-export
```

Genesis files saved to:
- `~/.devnet-builder/exports/<export-name>/genesis-<height>-<commit>.json` (pre-upgrade)
- `~/.devnet-builder/exports/<export-name>/genesis-<height>-<commit>.json` (post-upgrade)

---

## Quick Upgrade (Skip Governance)

For rapid testing iterations, bypass the governance process entirely:

```bash
# Direct binary replacement without governance proposal
devnet-builder upgrade \
  --name v2-upgrade \
  --version v2.0.0 \
  --skip-gov
```

This will:
- Stop all nodes immediately
- Replace binary/image
- Restart nodes with new version

**Use cases:**
- Rapid iteration during development
- Testing binary compatibility
- Debugging upgrade migration code

**Note:** `--skip-gov` does not test the governance upgrade mechanism itself.

---

## Resuming Interrupted Upgrades

If an upgrade is interrupted (network issue, crash, etc.), you can resume:

### Check Upgrade Status

```bash
# View current upgrade state
devnet-builder upgrade --show-status
```

### Resume from Last Stage

```bash
# Continue from where it stopped
devnet-builder upgrade --resume
```

### Clear Stuck State

If upgrade state is corrupted:

```bash
# Clear upgrade state and start fresh
devnet-builder upgrade --clear-state

# Or force restart the upgrade
devnet-builder upgrade \
  --name v2-upgrade \
  --version v2.0.0 \
  --force-restart
```

### Upgrade Stages

The upgrade process goes through these stages:
1. **Pending** - Initial state
2. **Proposing** - Submitting governance proposal
3. **Voting** - Waiting for votes
4. **Waiting** - Waiting for upgrade height
5. **Halted** - Chain halted at upgrade height
6. **Switching** - Replacing binary/image
7. **Restarting** - Starting nodes with new version
8. **Verifying** - Confirming chain resumed
9. **Completed** - Upgrade successful

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
devnet-builder stop

# Reset state
devnet-builder reset

# Restart
devnet-builder start
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

## Docker vs Local ExecutionMode

Choose the right execution mode for your use case.

### Docker ExecutionMode (Recommended)

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

### Local ExecutionMode (Advanced)

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
devnet-builder stop

# Restart with local binary (for debugging)
devnet-builder start --mode local --binary-ref /path/to/debug-<binary-name>
```

### ExecutionMode Comparison

| Aspect | Docker ExecutionMode | Local ExecutionMode |
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

## dvb CLI Workflows (Daemon Mode)

The `dvb` CLI provides a kubectl-style interface for managing devnets through the daemon.

### Starting the Daemon

```bash
# Start the daemon
devnetd

# Or with custom socket
devnetd --socket /tmp/devnetd.sock
```

### Managing Devnets with dvb

```bash
# Apply a devnet configuration
dvb apply -f devnet.yaml

# List devnets
dvb get devnets

# Get detailed status
dvb describe devnet my-devnet

# Delete a devnet
dvb delete devnet my-devnet
```

### Managing Upgrades with dvb

```bash
# Create an upgrade
dvb upgrade create v2-upgrade \
  --devnet my-devnet \
  --upgrade-name v2-upgrade \
  --version v2.0.0

# List upgrades
dvb upgrade list

# Check upgrade status
dvb upgrade status v2-upgrade

# Cancel an upgrade
dvb upgrade cancel v2-upgrade

# Retry a failed upgrade
dvb upgrade retry v2-upgrade
```

### Namespace Support

```bash
# Work in a specific namespace
dvb apply -f devnet.yaml -n team-alpha

# List devnets in namespace
dvb get devnets -n team-alpha

# List all devnets across namespaces
dvb get devnets --all-namespaces
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
