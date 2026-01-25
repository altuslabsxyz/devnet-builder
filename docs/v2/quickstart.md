# Quickstart Guide

Get started with Devnet Builder v2 in under 5 minutes.

## Prerequisites

- **Docker** (v20+) or **Podman** for containerized devnets
- **Go** (v1.21+) for building from source
- **Git** for cloning the repository

## Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/altuslabsxyz/devnet-builder.git
cd devnet-builder

# Build both binaries
make build

# Install to $GOPATH/bin
make install

# Verify installation
devnetd version
dvb version
```

### From Release (Coming Soon)

```bash
# Download latest release
curl -L https://github.com/altuslabsxyz/devnet-builder/releases/latest/download/devnet-builder-linux-amd64.tar.gz | tar xz

# Move binaries to PATH
sudo mv devnetd dvb /usr/local/bin/

# Verify
devnetd version
dvb version
```

## Start the Daemon

The daemon runs in the background and manages all devnets:

```bash
# Start daemon (runs in foreground for this quickstart)
devnetd start

# In another terminal, verify daemon is running
dvb daemon status
```

**Output:**
```
Daemon Status: Running
Version: v2.0.0
Uptime: 2s
gRPC Socket: /home/user/.devnet-builder/devnetd.sock
Active Devnets: 0
```

### Running as System Service (Optional)

For production use, run devnetd as a systemd service:

```bash
# Create systemd service
sudo tee /etc/systemd/system/devnetd.service <<EOF
[Unit]
Description=Devnet Builder Daemon
After=docker.service
Requires=docker.service

[Service]
Type=simple
User=$USER
ExecStart=$(which devnetd) start
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
EOF

# Enable and start
sudo systemctl enable devnetd
sudo systemctl start devnetd

# Check status
sudo systemctl status devnetd
```

## Deploy Your First Devnet

Deploy a 4-validator Osmosis testnet:

```bash
dvb deploy osmosisd \
  --validators 4 \
  --name osmosis-local

# Deployment output:
# ✓ Creating devnet osmosis-local
# ✓ Downloading binary osmosisd v24.0.0
# ✓ Generating genesis and configs
# ✓ Starting validator-0
# ✓ Starting validator-1
# ✓ Starting validator-2
# ✓ Starting validator-3
# ✓ Devnet is producing blocks (height: 5)
#
# Devnet: osmosis-local
# RPC:    http://localhost:26657
# REST:   http://localhost:1317
# gRPC:   localhost:9090
```

### Check Devnet Status

```bash
dvb status osmosis-local
```

**Output:**
```
Devnet: osmosis-local
Status: Running
Nodes:  4/4 ready
Height: 145
SDK:    v0.50.3

Nodes:
  validator-0  Running  Height: 145  Peers: 3  Restarts: 0
  validator-1  Running  Height: 145  Peers: 3  Restarts: 0
  validator-2  Running  Height: 145  Peers: 3  Restarts: 0
  validator-3  Running  Height: 145  Peers: 3  Restarts: 0
```

### View Node Logs

```bash
# Stream logs from validator-0
dvb nodes logs osmosis-local validator:0 --follow

# View last 100 lines
dvb nodes logs osmosis-local validator:0 --tail 100
```

## Submit Your First Transaction

### Bank Send

```bash
# Send tokens between validators
dvb tx submit osmosis-local \
  --type bank/send \
  --signer validator:0 \
  --payload '{
    "to_address": "osmo1...",
    "amount": "1000000uosmo"
  }'

# Output:
# ✓ Transaction submitted: tx-12345
# ✓ Confirmed at height 150
#   TxHash: 0xABCD...
#   GasUsed: 85000
```

### Governance Proposal

```bash
# Submit a text proposal
dvb tx submit osmosis-local \
  --type gov/proposal \
  --signer validator:0 \
  --payload '{
    "title": "Enable IBC",
    "description": "Proposal to enable IBC module",
    "deposit": "10000000uosmo"
  }'

# Vote on proposal (all validators automatically)
dvb gov vote osmosis-local 1 yes --all-validators
```

### Watch Transaction

```bash
# Watch transaction progress in real-time
dvb tx watch tx-12345

# Output updates:
# Phase: Pending
# Phase: Building
# Phase: Signing
# Phase: Submitted (TxHash: 0xABCD...)
# Phase: Confirmed (Height: 150)
```

## Test Health Monitoring

The daemon automatically monitors and recovers crashed nodes:

```bash
# Simulate node crash
docker stop osmosis-local-validator-0

# Watch daemon recover (in logs)
dvb daemon logs --follow

# Daemon automatically:
# 1. Detects crash within 10 seconds
# 2. Attempts restart with backoff
# 3. Verifies node rejoins network
# 4. Updates devnet status

# Verify recovery
dvb nodes health osmosis-local
```

**Output after recovery:**
```
Node Health Check:
  validator-0  Running  Height: 155  Peers: 3  Restarts: 1 ✓ Recovered
  validator-1  Running  Height: 155  Peers: 3  Restarts: 0
  validator-2  Running  Height: 155  Peers: 3  Restarts: 0
  validator-3  Running  Height: 155  Peers: 3  Restarts: 0

Overall: Healthy
```

## Perform Chain Upgrade

Test chain upgrades with automatic orchestration:

```bash
# Create upgrade proposal
dvb upgrade create osmosis-local \
  --upgrade-name v25 \
  --height 500 \
  --binary /path/to/osmosisd-v25 \
  --auto-vote

# Daemon orchestrates:
# 1. Submits upgrade proposal
# 2. Votes yes (all validators)
# 3. Waits for upgrade height
# 4. Stops all nodes
# 5. Switches binaries
# 6. Restarts nodes
# 7. Verifies block production

# Watch upgrade progress
dvb upgrade status v25
```

**Output:**
```
Upgrade: v25
Status: Completed
Devnet: osmosis-local

Timeline:
  ✓ Proposed at height 250
  ✓ Voting completed (4/4 yes)
  ✓ Upgrade height 500 reached
  ✓ Nodes stopped
  ✓ Binaries switched
  ✓ Nodes restarted
  ✓ Chain producing blocks (height: 505)

New SDK Version: v0.50.4
```

## Multi-Chain Management

Deploy and manage multiple chains simultaneously:

```bash
# Deploy Osmosis
dvb deploy osmosisd --name osmosis --validators 4

# Deploy Cosmos Hub
dvb deploy gaiad --name hub --validators 4

# Deploy Ethereum (EVM)
dvb deploy geth --name eth-local --validators 1

# List all devnets
dvb list
```

**Output:**
```
NAME         TYPE     STATUS   NODES  HEIGHT  SDK VERSION
osmosis      cosmos   Running  4/4    1245    v0.50.3
hub          cosmos   Running  4/4    856     v0.47.10
eth-local    evm      Running  1/1    3421    -
```

### Cross-Chain Operations

```bash
# Setup IBC connection (future feature)
dvb ibc connect osmosis hub --channel transfer

# Transfer tokens via IBC
dvb tx submit osmosis \
  --type ibc/transfer \
  --signer validator:0 \
  --payload '{
    "receiver": "cosmos1...",
    "amount": "1000000uosmo",
    "channel": "channel-0"
  }'
```

## Stop and Cleanup

### Stop a Devnet

```bash
# Stop nodes but keep state
dvb stop osmosis-local

# Restart later
dvb start osmosis-local
```

### Destroy a Devnet

```bash
# Completely remove devnet (destructive)
dvb destroy osmosis-local --confirm

# Output:
# ⚠  This will permanently delete all data for osmosis-local
# ✓ Stopped all nodes
# ✓ Removed containers
# ✓ Deleted data directory
# ✓ Removed from state store
```

### Stop the Daemon

```bash
# Graceful shutdown (stops all devnets)
dvb daemon shutdown

# Or send SIGTERM
kill $(cat ~/.devnet-builder/devnetd.pid)
```

## Configuration

### Daemon Configuration

Edit `~/.devnet-builder/config.toml`:

```toml
[daemon]
# Directory for state and data
data_dir = "/home/user/.devnet-builder"

# gRPC socket path
socket_path = "/home/user/.devnet-builder/devnetd.sock"

# Log level (debug, info, warn, error)
log_level = "info"

[controller]
# Reconciliation interval
reconcile_interval = "5s"

# Health check interval
health_check_interval = "10s"

# Max node restart attempts
max_restart_attempts = 3

[network]
# Docker network mode (bridge, host)
docker_network = "bridge"

# Port range for services
port_range_start = 26650
port_range_end = 26750

[cache]
# Binary cache directory
cache_dir = "/home/user/.devnet-builder/cache"

# Cache retention (days)
retention_days = 30
```

### Per-Devnet Configuration

Override defaults per devnet:

```bash
dvb deploy osmosisd \
  --name my-devnet \
  --validators 4 \
  --config '{
    "homeBaseDir": "/data/my-custom-path",
    "chainId": "my-chain-1",
    "ports": {
      "rpcPortStart": 30000,
      "restPortStart": 31000
    }
  }'
```

## Troubleshooting

### Daemon Won't Start

```bash
# Check if socket file exists
ls -la ~/.devnet-builder/devnetd.sock

# Remove stale socket
rm ~/.devnet-builder/devnetd.sock

# Check logs
journalctl -u devnetd -f  # if using systemd
devnetd start --log-level debug  # run in foreground
```

### Node Keeps Crashing

```bash
# View crash logs
dvb nodes logs osmosis-local validator:0 --tail 500

# Check node status
dvb nodes health osmosis-local

# Manual restart
dvb nodes restart osmosis-local validator:0

# Check resource limits
docker stats osmosis-local-validator-0
```

### Transaction Fails

```bash
# Get transaction details
dvb tx status tx-12345

# View full error
dvb tx get tx-12345 --output json | jq .status.error

# Common issues:
# - Insufficient gas: increase --gas-limit
# - Invalid payload: check JSON structure
# - Signer not found: verify --signer address
```

### Database Corruption

```bash
# Backup database
cp ~/.devnet-builder/devnetd.db ~/.devnet-builder/devnetd.db.backup

# Reset database (destructive)
dvb daemon shutdown
rm ~/.devnet-builder/devnetd.db
devnetd start

# Re-deploy devnets
dvb deploy osmosisd --name osmosis --validators 4
```

## Next Steps

Now that you have a devnet running:

1. **[Architecture Guide](architecture.md)** - Understand how the system works
2. **[Client Reference](client.md)** - Learn all dvb commands
3. **[Transaction Guide](transactions.md)** - Deep dive into transaction building
4. **[Plugin Development](plugins.md)** - Add support for new chains

## Common Workflows

### Development Testing

```bash
# Deploy test environment
dvb deploy osmosisd --name test --validators 1

# Run your application tests
npm test  # or cargo test, go test, etc.

# Clean up
dvb destroy test --confirm
```

### Governance Testing

```bash
# Deploy with auto-vote enabled
dvb deploy osmosisd --name gov-test --validators 4

# Submit proposal
PROPOSAL_ID=$(dvb tx submit gov-test \
  --type gov/proposal \
  --signer validator:0 \
  --payload @proposal.json \
  --output json | jq -r .proposalId)

# Vote and wait for result
dvb gov vote gov-test $PROPOSAL_ID yes --all-validators
dvb gov status gov-test $PROPOSAL_ID --wait
```

### Upgrade Testing

```bash
# Deploy with old version
dvb deploy osmosisd \
  --name upgrade-test \
  --validators 4 \
  --version v24.0.0

# Test upgrade
dvb upgrade create upgrade-test \
  --upgrade-name v25 \
  --height 100 \
  --binary /path/to/osmosisd-v25 \
  --auto-vote \
  --with-export

# Verify state after upgrade
dvb nodes health upgrade-test
dvb tx submit upgrade-test --type bank/send ...
```

## Resources

- **Main Documentation**: [README.md](README.md)
- **API Reference**: [api-reference.md](api-reference.md)
- **Design Documents**: [../plans/](../plans/)
- **GitHub Issues**: [github.com/altuslabsxyz/devnet-builder/issues](https://github.com/altuslabsxyz/devnet-builder/issues)
