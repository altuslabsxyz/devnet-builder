# Troubleshooting Guide

Solutions for common issues when using devnet-builder.

## Table of Contents

- [Data Directory Structure](#data-directory-structure)
- [Port Configuration](#port-configuration)
- [Common Issues](#common-issues)
  - [Docker Not Running](#docker-not-running)
  - [Port Conflicts](#port-conflicts)
  - [Insufficient Disk Space](#insufficient-disk-space)
  - [Binary/Mode Mismatch](#binarymode-mismatch)
  - [Chain Not Syncing](#chain-not-syncing)
  - [Upgrade Failure](#upgrade-failure)
  - [Permission Issues](#permission-issues)
  - [Node Unhealthy](#node-unhealthy)
  - [Snapshot Download Failed](#snapshot-download-failed)
  - [Genesis Export Failed](#genesis-export-failed)

---

## Data Directory Structure

Understanding the directory structure helps locate files for debugging:

```
~/.devnet-builder/
├── bin/                    # Local binaries (symlinks)
├── build/                  # Build artifacts
├── cache/                  # Binary cache for upgrades
│   └── binaries/          # Cached network binaries by commit
├── config.toml            # User configuration
├── devnet/                # Active devnet data
│   ├── metadata.json      # Devnet state and config
│   ├── node0/             # First validator
│   │   ├── config/        # genesis.json, config.toml, app.toml
│   │   ├── data/          # Chain data (blocks, state)
│   │   └── keyring-test/  # Validator key
│   ├── node1/             # Additional validators...
│   ├── node2/
│   ├── node3/
│   └── accounts/          # Funded test accounts
│       └── keyring-test/
├── genesis/               # Genesis exports
└── snapshots/             # Snapshot cache
    └── mainnet/           # Network-specific snapshots
```

### Important Files

| File | Location | Purpose |
|------|----------|---------|
| metadata.json | devnet/ | Current devnet state |
| genesis.json | node*/config/ | Chain genesis |
| config.toml | node*/config/ | Tendermint configuration |
| app.toml | node*/config/ | Application configuration |
| priv_validator_key.json | node*/config/ | Validator private key |

---

## Port Configuration

Default ports used by each node:

| Service | Node 0 | Node 1 | Node 2 | Node 3 |
|---------|--------|--------|--------|--------|
| P2P | 26656 | 26666 | 26676 | 26686 |
| RPC | 26657 | 26667 | 26677 | 26687 |
| Proxy App | 26658 | 26668 | 26678 | 26688 |
| gRPC | 9090 | 9091 | 9092 | 9093 |
| EVM RPC | 8545 | - | - | - |
| EVM WebSocket | 8546 | - | - | - |

Note: EVM endpoints are only available on node0.

---

## Common Issues

### Docker Not Running

**Symptoms:**
- Error: `Cannot connect to the Docker daemon`
- Error: `docker: command not found`

**Solutions:**

```bash
# Linux: Start Docker service
sudo systemctl start docker
sudo systemctl status docker

# macOS: Start Docker Desktop
open -a Docker

# Verify Docker is running
docker info
```

**Also check:**
```bash
# Ensure your user can run Docker
docker run hello-world

# If permission denied, add user to docker group
sudo usermod -aG docker $USER
# Then log out and back in
```

---

### Port Conflicts

**Symptoms:**
- Error: `bind: address already in use`
- Nodes fail to start
- Status shows nodes as unhealthy

**Solutions:**

```bash
# Find what's using a port
lsof -i :26657
lsof -i :8545

# Kill the process
kill -9 <PID>

# Or check for existing devnet
devnet-builder status

# Destroy existing devnet if needed
devnet-builder destroy --force
```

**Common culprits:**
- Previous devnet-builder instance
- Local network daemon process
- Other blockchain nodes

---

### Insufficient Disk Space

**Symptoms:**
- Snapshot download fails
- Node crashes with "no space left" error
- Genesis export fails

**Solutions:**

```bash
# Check disk space
df -h ~/.devnet-builder

# Clear snapshot cache
rm -rf ~/.devnet-builder/snapshots/*

# Clear binary cache
devnet-builder cache clean

# Full cleanup
devnet-builder destroy --cache --force
```

**Space requirements:**
- Mainnet snapshot: ~5-10GB
- Testnet snapshot: ~2-5GB
- Per devnet: ~1-2GB additional

---

### Binary/Mode Mismatch

**Symptoms:**
- Error: `binary not found`
- Error: `mode mismatch`
- Nodes start but immediately crash

**Cause:** Trying to use `--mode local` on a devnet initialized with Docker, or vice versa.

**Solutions:**

```bash
# Check current mode
devnet-builder status --json | jq '.mode'

# If mismatch, destroy and recreate
devnet-builder destroy --force
devnet-builder deploy --mode docker  # or --mode local

# Or restart with correct mode
devnet-builder stop
devnet-builder start --mode docker
```

---

### Chain Not Syncing

**Symptoms:**
- Block height not increasing
- Nodes show as unhealthy
- Peers not connecting

**Diagnosis:**

```bash
# Check node status
curl -s http://localhost:26657/status | jq '.result.sync_info'

# Check peers
curl -s http://localhost:26657/net_info | jq '.result.n_peers'

# Check consensus
curl -s http://localhost:26657/consensus_state | jq '.result.round_state.height_vote_set'
```

**Solutions:**

```bash
# View logs for errors
devnet-builder logs --tail 200 | grep -i "error\|panic"

# Restart nodes
devnet-builder restart

# If persistent, reset state
devnet-builder reset --force
devnet-builder start
```

---

### Upgrade Failure

**Symptoms:**
- Upgrade proposal not passing
- Chain halts but doesn't resume
- Binary compatibility errors

**Diagnosis:**

```bash
# Check upgrade status
devnet-builder status

# Check for panic in logs
devnet-builder logs --tail 100 | grep -i "panic\|upgrade"
```

**Solutions:**

```bash
# If chain halted, check binary is correct
devnet-builder logs node0 | grep "upgrade"

# Manually restart with correct binary
devnet-builder stop
devnet-builder start --image <correct-image>

# If completely stuck, export genesis and restart
devnet-builder destroy --force
devnet-builder deploy --image <new-image>
```

**Prevention:**
- Test upgrades in isolation first
- Use `--export-genesis` to capture state
- Verify binary compatibility before upgrading

---

### Permission Issues

**Symptoms:**
- Error: `permission denied`
- Cannot write to ~/.devnet-builder
- Docker socket permission errors

**Solutions:**

```bash
# Fix home directory permissions
chmod -R u+rw ~/.devnet-builder

# Fix Docker socket permissions
sudo chmod 666 /var/run/docker.sock
# Or add user to docker group
sudo usermod -aG docker $USER

# If using sudo, ensure HOME is correct
sudo -E devnet-builder deploy
```

---

### Node Unhealthy

**Symptoms:**
- `devnet-builder status` shows nodes as unhealthy
- Nodes appear running but not responding
- Intermittent connectivity

**Diagnosis:**

```bash
# Check container status (Docker mode)
docker ps -a | grep devnet-builder

# Check specific node logs
devnet-builder logs node0 --tail 100

# Check resource usage
docker stats devnet-builder-node0
```

**Solutions:**

```bash
# Restart unhealthy node
devnet-builder node stop node0
devnet-builder node start node0

# Restart all nodes
devnet-builder restart

# Check if it's a resource issue
docker stats --no-stream
```

---

### Snapshot Download Failed

**Symptoms:**
- Error during snapshot download
- Incomplete snapshot
- Network timeout

**Solutions:**

```bash
# Clear corrupted snapshot
rm -rf ~/.devnet-builder/snapshots/mainnet/*

# Retry with verbose logging
devnet-builder deploy -v

# Use no-cache to force fresh download
devnet-builder deploy --no-cache
```

**Check network:**
```bash
# Test connectivity
curl -I <snapshot-source>/snapshots/network_pruned.tar.zst

# Check available space
df -h ~/.devnet-builder
```

---

### Genesis Export Failed

**Symptoms:**
- Error exporting genesis
- Corrupted genesis file
- Timeout during export

**Solutions:**

```bash
# Ensure node is fully synced first
curl -s http://localhost:26657/status | jq '.result.sync_info.catching_up'
# Should return "false"

# Try manual export (Docker mode)
docker exec devnet-builder-node0 <binary-name> export --home /home/network/.<network-home>

# Check logs for specific error
devnet-builder logs node0 | grep -i "export\|genesis"
```

---

## Getting Help

### Collect Debug Information

When reporting issues, include:

```bash
# System info
uname -a
docker --version

# devnet-builder version
devnet-builder version

# Current status
devnet-builder status --json

# Recent logs
devnet-builder logs --tail 200 > devnet-logs.txt
```

### Log Locations

| Mode | Log Location |
|------|--------------|
| Docker | `docker logs devnet-builder-node0` |
| Local | `~/.devnet-builder/devnet/node0/node.log` |

### Useful Debug Commands

```bash
# Full status dump
devnet-builder status --json | jq

# Check all containers
docker ps -a | grep devnet

# Check Docker logs
docker logs devnet-builder-node0 --tail 100

# Check disk usage
du -sh ~/.devnet-builder/*

# Network diagnostics
curl -s http://localhost:26657/health
curl -s http://localhost:26657/net_info | jq '.result.n_peers'
```

---

## See Also

- [Getting Started](getting-started.md) - Initial setup guide
- [Command Reference](commands.md) - All CLI commands
- [Configuration](configuration.md) - config.toml options
- [Workflows](workflows.md) - Common debugging workflows
