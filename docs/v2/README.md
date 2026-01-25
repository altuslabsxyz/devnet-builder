# Devnet Builder v2 Documentation

Devnet Builder v2 introduces a daemon-based architecture with persistent supervision, multi-chain coordination, and a comprehensive transaction API.

## Overview

Devnet Builder v2 consists of two components:

- **`devnetd`** - Persistent daemon that manages devnet lifecycle, monitors health, and orchestrates transactions
- **`dvb`** - Lightweight CLI client for controlling the daemon

This architecture enables:
- **Persistent supervision** - Automatic node restart, health monitoring, and crash recovery
- **Multi-chain coordination** - Single daemon manages multiple devnets across Cosmos SDK, EVM, and other platforms
- **Transaction orchestration** - Long-running governance workflows, proposal submission, and voting
- **API-first design** - Full gRPC API for programmatic control and integration
- **Streaming capabilities** - Real-time logs, events, and transaction confirmations

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                         dvb (client)                         │
│  Command-line interface for users and CI/CD pipelines       │
└───────────────────────────┬─────────────────────────────────┘
                            │ gRPC
┌───────────────────────────▼─────────────────────────────────┐
│                       devnetd (daemon)                       │
│                                                              │
│  ┌──────────────────────────────────────────────────────┐   │
│  │         gRPC Services (API Layer)                    │   │
│  │  Devnet │ Node │ Upgrade │ Transaction │ Export     │   │
│  └────────────────────────┬─────────────────────────────┘   │
│                           │                                  │
│  ┌────────────────────────▼─────────────────────────────┐   │
│  │      Controller Manager (Reconciliation Loop)        │   │
│  │                                                       │   │
│  │  DevnetController    - Provisions and manages devnets│   │
│  │  NodeController      - Manages individual nodes      │   │
│  │  HealthController    - Monitors and recovers nodes   │   │
│  │  UpgradeController   - Orchestrates chain upgrades   │   │
│  │  TxController        - Manages transaction lifecycle │   │
│  │  NetworkController   - Network isolation & routing   │   │
│  └────────────────────────┬─────────────────────────────┘   │
│                           │                                  │
│  ┌────────────────────────▼─────────────────────────────┐   │
│  │         State Store (BoltDB)                         │   │
│  │  Persists all resource state and event history      │   │
│  └────────────────────────┬─────────────────────────────┘   │
│                           │                                  │
│  ┌────────────────────────▼─────────────────────────────┐   │
│  │      Infrastructure Layer                            │   │
│  │  Docker │ Plugins │ RPC Clients │ Binary Cache      │   │
│  └──────────────────────────────────────────────────────┘   │
└──────────────────────────────────────────────────────────────┘
```

## Documentation Structure

### Getting Started
- **[Quickstart Guide](quickstart.md)** - Get up and running in 5 minutes
- **[Architecture Overview](architecture.md)** - Deep dive into system design and patterns

### Core Concepts
- **[Daemon Operations](daemon.md)** - Understanding devnetd lifecycle and configuration
- **[Client Usage](client.md)** - Complete dvb CLI reference and examples
- **[Transaction System](transactions.md)** - Building, signing, and broadcasting transactions
- **[Plugin Development](plugins.md)** - Creating network plugins for new chains

### API Reference
- **[gRPC API Reference](api-reference.md)** - Complete service and method documentation

### Design Documents
- **[Daemon Refactor Design](../plans/2026-01-23-daemon-refactor-design.md)** - Original architecture decisions
- **[TxController Design](../plans/2026-01-24-txcontroller-plugin-v2-design.md)** - Transaction system design

## Key Features

### Persistent Supervision

The daemon runs continuously in the background, providing:

```bash
# Start daemon
devnetd start

# Deploy a devnet (managed by daemon)
dvb deploy osmosisd --validators 4

# Daemon automatically:
# - Monitors node health
# - Restarts crashed nodes
# - Tracks block production
# - Manages upgrades
```

### Multi-Chain Support

Manage multiple devnets simultaneously:

```bash
# Deploy multiple chains
dvb deploy osmosisd --name osmosis-test
dvb deploy gaiad --name cosmos-hub
dvb deploy geth --name ethereum-local

# List all running devnets
dvb list

# Cross-chain operations
dvb tx submit osmosis-test --type ibc/transfer --to cosmos-hub
```

### Transaction Orchestration

Complete transaction lifecycle management:

```bash
# Submit governance proposal
dvb tx submit mydevnet \
  --type gov/proposal \
  --payload proposal.json \
  --signer validator:0

# Watch transaction progress
dvb tx watch <tx-id>

# Vote on proposal (all validators)
dvb gov vote mydevnet 1 yes --all-validators
```

### Health Monitoring

Automatic failure detection and recovery:

```bash
# View node health
dvb nodes health mydevnet

# Daemon automatically:
# - Detects crashed nodes
# - Restarts with exponential backoff
# - Alerts on persistent failures
# - Monitors chain halt conditions
```

### Chain Upgrades

Orchestrated upgrade workflows:

```bash
# Submit upgrade proposal
dvb upgrade create mydevnet \
  --upgrade-name v2.0 \
  --height 1000 \
  --binary /path/to/new-binary

# Daemon manages:
# - Proposal submission
# - Automated voting
# - Binary switching at upgrade height
# - Post-upgrade verification
```

## Resource Model

Devnet Builder v2 uses a Kubernetes-inspired resource model:

### Devnet Resource

Represents a complete blockchain network:

```yaml
apiVersion: v1
kind: Devnet
metadata:
  name: osmosis-test
  createdAt: "2026-01-25T10:00:00Z"
spec:
  networkType: cosmos
  plugin: osmosisd
  validators: 4
  fullNodes: 2
  binarySource:
    type: github
    url: osmosis-labs/osmosis
    version: v24.0.0
status:
  phase: Running
  nodes: 6
  readyNodes: 6
  currentHeight: 12450
  sdkVersion: "0.50.3"
```

### Node Resource

Represents an individual blockchain node:

```yaml
apiVersion: v1
kind: Node
metadata:
  name: osmosis-test-validator-0
spec:
  devnetRef: osmosis-test
  index: 0
  role: validator
  homeDir: /data/osmosis-test/validator-0
status:
  phase: Running
  containerID: abc123
  blockHeight: 12450
  peerCount: 5
  restartCount: 0
```

### Transaction Resource

Represents a transaction in its lifecycle:

```yaml
apiVersion: v1
kind: Transaction
metadata:
  name: tx-gov-proposal-1
spec:
  devnetRef: osmosis-test
  txType: gov/proposal
  signer: validator:0
  payload: {...}
  memo: "reward-tag:pool-alpha"
  gasLimit: 300000
status:
  phase: Confirmed
  txHash: "0x1234..."
  height: 12451
  gasUsed: 245000
```

## Controller Pattern

Devnet Builder v2 uses a controller-reconciler pattern inspired by Kubernetes:

```go
// Each controller continuously reconciles desired state
func (c *DevnetController) Reconcile(ctx context.Context, name string) error {
    // 1. Get current resource from store
    devnet, err := c.store.GetDevnet(ctx, name)

    // 2. Compare actual state vs desired state
    if devnet.Status.Phase != devnet.Spec.TargetPhase {
        // 3. Take action to reconcile
        return c.transitionPhase(ctx, devnet)
    }

    return nil
}
```

Controllers run independently and handle:
- **DevnetController** - Devnet provisioning and lifecycle
- **NodeController** - Individual node management
- **HealthController** - Monitoring and recovery
- **UpgradeController** - Chain upgrade orchestration
- **TxController** - Transaction lifecycle
- **NetworkController** - Network isolation

## State Management

All state is persisted in BoltDB (`~/.devnet-builder/devnetd.db`):

```
~/.devnet-builder/
├── devnetd.db         # BoltDB database
├── devnetd.sock       # Unix socket for gRPC
├── devnetd.pid        # Daemon process ID
├── config.toml        # Daemon configuration
├── plugins/           # Network plugin binaries
├── cache/             # Binary cache
└── devnets/           # Devnet data directories
    ├── osmosis-test/
    ├── cosmos-hub/
    └── ethereum-local/
```

## gRPC API

Complete programmatic access via gRPC:

```go
import "github.com/altuslabsxyz/devnet-builder/api/proto/v1"

// Connect to daemon
conn, _ := grpc.Dial("unix:///home/user/.devnet-builder/devnetd.sock")
client := v1.NewDevnetServiceClient(conn)

// Deploy devnet
resp, _ := client.Create(ctx, &v1.CreateDevnetRequest{
    Name:       "my-devnet",
    Plugin:     "osmosisd",
    Validators: 4,
})

// Stream logs
stream, _ := client.StreamLogs(ctx, &v1.StreamLogsRequest{
    Devnet: "my-devnet",
    Follow: true,
})
for {
    log, _ := stream.Recv()
    fmt.Println(log.Message)
}
```

## Migration from v1

V2 maintains compatibility with v1 workflows:

```bash
# V1 (one-shot execution)
devnet-builder deploy osmosisd --validators 4

# V2 (daemon-managed)
devnetd start              # Start daemon
dvb deploy osmosisd --validators 4  # Same command interface
```

The daemon provides additional benefits:
- Automatic node restart on crashes
- Health monitoring and alerting
- Persistent state across reboots
- Multi-devnet coordination
- Transaction orchestration

## Next Steps

1. **[Quickstart Guide](quickstart.md)** - Install and run your first devnet with v2
2. **[Architecture Overview](architecture.md)** - Understand the design decisions
3. **[Client Usage](client.md)** - Learn all dvb commands
4. **[Plugin Development](plugins.md)** - Create plugins for new chains

## Community and Support

- **GitHub Issues**: [github.com/altuslabsxyz/devnet-builder/issues](https://github.com/altuslabsxyz/devnet-builder/issues)
- **Documentation**: [docs/v2/](.)
- **Design Docs**: [docs/plans/](../plans/)

## Contributing

See [CONTRIBUTING.md](../../CONTRIBUTING.md) for development setup and guidelines.
