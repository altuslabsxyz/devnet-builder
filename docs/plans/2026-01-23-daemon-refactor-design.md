# Devnet Builder Daemon Refactor Design

**Date:** 2026-01-23
**Status:** Draft
**Author:** Claude (with user direction)

## Executive Summary

Refactor devnet-builder from a one-shot CLI tool into a persistent daemon (`devnetd`) with a thin client (`dvb`). This enables:

1. **Monitoring & lifecycle management** - Full node supervision, auto-restart, health monitoring
2. **API exposure** - gRPC API for programmatic control
3. **Multi-chain coordination** - Single daemon manages multiple devnets
4. **Transaction orchestration** - Long-running governance workflows
5. **Multi-platform support** - Cosmos SDK, EVM, Tempo/Commonware

## Priority Order (User-Defined)

1. **A - Monitoring & lifecycle management** (foundation)
2. **B - API exposure** (enables everything else)
3. **E - Multi-chain coordination** (key differentiator)
4. **C - Transaction orchestration** (builds on API + multi-chain)
5. **D - Resource efficiency** (natural byproduct)

## Architecture Decision Records

### ADR-1: Controller-Reconciler Pattern

**Decision:** Use Kubernetes-style controller-reconciler pattern.

**Rationale:**
- Battle-tested at scale (Kubernetes, Terraform, Crossplane)
- Natural fit for supervision and self-healing
- Clean separation of concerns per controller
- Easy to extend with new controllers

**Alternatives Considered:**
- Event-driven Actor Model (Erlang-style) - excellent isolation but less common in Go
- Service Layer with FSM - simpler but less elegant for complex state

### ADR-2: Separate Binaries

**Decision:** Two binaries: `devnetd` (daemon) + `dvb` (client)

**Rationale:**
- Clear mental model for developers unfamiliar with infra tooling
- No naming conflicts (start/stop nodes vs start/stop daemon)
- Follows established patterns (dockerd/docker, kubelet/kubectl)
- Easy to explain: "devnetd runs, dvb controls"

### ADR-3: gRPC-Only API

**Decision:** gRPC API without REST gateway.

**Rationale:**
- Existing HashiCorp go-plugin infrastructure uses gRPC
- Bidirectional streaming for logs/events
- Strong typing with protobuf
- Efficient binary protocol for multi-chain coordination

### ADR-4: Single Daemon, Multiple Devnets

**Decision:** One `devnetd` process manages all chains.

**Rationale:**
- Unified API for all chains
- Shared connection pools (Docker, gRPC)
- Easier cross-chain workflows
- Single point of configuration

### ADR-5: BoltDB State Store

**Decision:** Use BoltDB for persistent storage.

**Rationale:**
- No external dependencies
- ACID transactions
- Fast key-value lookups
- Embedded Go database

---

## Core Architecture

```
┌──────────────────────────────────────────────────────────────────┐
│                           devnetd                                 │
│                                                                   │
│  ┌────────────────────────────────────────────────────────────┐  │
│  │                      gRPC Server                            │  │
│  │   DevnetService │ NodeService │ UpgradeService │ TxService  │  │
│  └─────────────────────────┬──────────────────────────────────┘  │
│                            │                                      │
│  ┌─────────────────────────▼──────────────────────────────────┐  │
│  │                  Controller Manager                         │  │
│  │                                                             │  │
│  │   Informers ──► Work Queue ──► Controllers ──► Reconcile   │  │
│  │                                                             │  │
│  │   ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐     │  │
│  │   │  Devnet  │ │   Node   │ │  Health  │ │ Upgrade  │     │  │
│  │   │   Ctrl   │ │   Ctrl   │ │   Ctrl   │ │   Ctrl   │     │  │
│  │   └──────────┘ └──────────┘ └──────────┘ └──────────┘     │  │
│  │   ┌──────────┐ ┌──────────┐ ┌──────────┐                  │  │
│  │   │ Network  │ │    Tx    │ │  Plugin  │                  │  │
│  │   │   Ctrl   │ │   Ctrl   │ │   Ctrl   │                  │  │
│  │   └──────────┘ └──────────┘ └──────────┘                  │  │
│  └─────────────────────────────────────────────────────────────┘  │
│                            │                                      │
│  ┌─────────────────────────▼──────────────────────────────────┐  │
│  │                   State Store (BoltDB)                      │  │
│  │   Resources: Devnets │ Nodes │ Upgrades │ Transactions     │  │
│  └─────────────────────────────────────────────────────────────┘  │
│                            │                                      │
│  ┌─────────────────────────▼──────────────────────────────────┐  │
│  │                   Infrastructure Layer                      │  │
│  │   Docker │ Plugins (gRPC) │ Binary Cache │ RPC Clients     │  │
│  └─────────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────────┘
```

---

## Resource Model

### Devnet

```go
type Devnet struct {
    Metadata ResourceMeta   `json:"metadata"`
    Spec     DevnetSpec     `json:"spec"`
    Status   DevnetStatus   `json:"status"`
}

type DevnetSpec struct {
    Name        string       `json:"name"`
    NetworkType string       `json:"networkType"` // "cosmos", "evm", "tempo"
    Plugin      string       `json:"plugin"`
    Validators  int          `json:"validators"`
    FullNodes   int          `json:"fullNodes"`
    SnapshotURL string       `json:"snapshotUrl,omitempty"`
    GenesisPath string       `json:"genesisPath,omitempty"`
    BinarySource BinarySource `json:"binarySource"`
    Mode        string       `json:"mode"` // "docker", "local"
    Ports       PortConfig   `json:"ports"`
    Resources   ResourceLimits `json:"resources,omitempty"`
}

type DevnetStatus struct {
    Phase             string    `json:"phase"` // Pending, Provisioning, Running, Degraded, Stopped
    Nodes             int       `json:"nodes"`
    ReadyNodes        int       `json:"readyNodes"`
    CurrentHeight     int64     `json:"currentHeight"`
    SDKVersion        string    `json:"sdkVersion"`
    SDKVersionHistory []SDKVersionChange `json:"sdkVersionHistory"`
    LastHealthCheck   time.Time `json:"lastHealthCheck"`
    Conditions        []Condition `json:"conditions"`
}
```

### Node

```go
type Node struct {
    Metadata ResourceMeta `json:"metadata"`
    Spec     NodeSpec     `json:"spec"`
    Status   NodeStatus   `json:"status"`
}

type NodeSpec struct {
    DevnetRef   string `json:"devnetRef"`
    Index       int    `json:"index"`
    Role        string `json:"role"` // "validator", "fullnode"
    BinaryPath  string `json:"binaryPath"`
    HomeDir     string `json:"homeDir"`
}

type NodeStatus struct {
    Phase        string `json:"phase"` // Starting, Running, Stopped, Crashed
    ContainerID  string `json:"containerId,omitempty"`
    PID          int    `json:"pid,omitempty"`
    BlockHeight  int64  `json:"blockHeight"`
    PeerCount    int    `json:"peerCount"`
    CatchingUp   bool   `json:"catchingUp"`
    RestartCount int    `json:"restartCount"`
}
```

### Upgrade

```go
type Upgrade struct {
    Metadata ResourceMeta  `json:"metadata"`
    Spec     UpgradeSpec   `json:"spec"`
    Status   UpgradeStatus `json:"status"`
}

type UpgradeSpec struct {
    DevnetRef     string       `json:"devnetRef"`
    UpgradeName   string       `json:"upgradeName"`
    TargetHeight  int64        `json:"targetHeight"`
    NewBinary     BinarySource `json:"newBinary"`
    WithExport    bool         `json:"withExport"`
    AutoVote      bool         `json:"autoVote"`
}

type UpgradeStatus struct {
    Phase          string `json:"phase"` // Pending, Proposed, Voting, Waiting, Switching, Verifying, Completed, Failed
    ProposalID     uint64 `json:"proposalId"`
    VotesReceived  int    `json:"votesReceived"`
    CurrentHeight  int64  `json:"currentHeight"`
    PreExportPath  string `json:"preExportPath,omitempty"`
    PostExportPath string `json:"postExportPath,omitempty"`
}
```

### Transaction

```go
type Transaction struct {
    Metadata ResourceMeta      `json:"metadata"`
    Spec     TransactionSpec   `json:"spec"`
    Status   TransactionStatus `json:"status"`
}

type TransactionSpec struct {
    DevnetRef  string          `json:"devnetRef"`
    TxType     string          `json:"txType"`     // "gov/vote", "staking/delegate", etc.
    Signer     string          `json:"signer"`
    Payload    json.RawMessage `json:"payload"`
    SDKVersion string          `json:"sdkVersion"` // Auto-detected or specified
}

type TransactionStatus struct {
    Phase   string `json:"phase"` // Pending, Submitted, Confirmed, Failed
    TxHash  string `json:"txHash"`
    Height  int64  `json:"height"`
    GasUsed int64  `json:"gasUsed"`
    Error   string `json:"error,omitempty"`
}
```

---

## Controllers

### DevnetController

Orchestrates the entire devnet lifecycle.

**Reconciliation States:**
- `""` (new) → Start provisioning
- `Provisioning` → Check if complete
- `Running` → Ensure correct node count
- `Degraded` → Attempt recovery
- `Stopped` → Check if should restart

### NodeController

Manages individual node lifecycle.

**Responsibilities:**
- Start/stop nodes
- Auto-restart crashed nodes (up to max retries)
- Update node status (block height, peer count)

### HealthController

Monitors health and triggers recovery.

**Failure Scenarios Handled:**
- Node crash recovery
- Health degradation (stuck chains)
- Resource exhaustion warnings
- Network partition detection

### UpgradeController

Orchestrates chain upgrades with SDK version awareness.

**Phases:**
1. `Pending` → Pre-export if requested
2. `Proposing` → Submit upgrade proposal
3. `Voting` → Auto-vote if enabled
4. `Waiting` → Wait for upgrade height
5. `Switching` → Stop nodes, switch binary, restart
6. `Verifying` → Confirm chain producing blocks, update SDK version
7. `Completed` → Post-export if requested

### TxController

Handles generic transaction execution with dynamic SDK version awareness.

**Key Feature:** TxBuilder is refreshed when SDK version changes after upgrade.

---

## gRPC API

### Services

- `DevnetService` - Create, Get, List, Delete, Start, Stop, Watch, StreamLogs
- `NodeService` - Get, List, Start, Stop, Restart, GetHealth, StreamLogs
- `UpgradeService` - Create, Get, List, Cancel, Watch
- `TransactionService` - Submit, Get, List, SubmitBatch, Watch
- `ExportService` - Create, Get, List, Delete
- `PluginService` - List, Get, Install, Uninstall
- `DaemonService` - GetStatus, Shutdown, GetConfig

### Streaming

- `DevnetService.Watch` - Stream devnet events
- `DevnetService.StreamLogs` - Stream node logs
- `UpgradeService.Watch` - Watch upgrade progress
- `TransactionService.Watch` - Watch tx confirmation

---

## Plugin Interface v2

### Multi-Platform Support

Platforms:
- Cosmos SDK (v0.47, v0.50, v0.53)
- EVM (Geth, Besu, Reth)
- Tempo/Commonware

### SDK Version-Aware Tx Building

```go
type NetworkPluginV2 interface {
    GetInfo() (*PluginInfo, error)
    GetSupportedSDKVersions() (*SDKVersionRange, error)
    CreateTxBuilder(req *CreateTxBuilderRequest) (TxBuilder, error)
    GetSupportedTxTypes() (*SupportedTxTypes, error)
    // ... existing methods
}
```

**SDK Version Lifecycle:**
- Detected from binary on devnet creation
- Re-detected after upgrade completes
- TxBuilder refreshed when version changes
- History tracked in DevnetStatus

---

## dvb Client

### Dual-Mode Operation

```
dvb command
    │
    ├─► Daemon running? ──► Yes ──► gRPC call to devnetd
    │                              (gets supervision, multi-chain, streaming)
    │
    └─► No ──► Standalone execution
               (identical to current devnet-builder)
```

### Command Structure

```bash
# Devnet lifecycle
dvb deploy [flags]
dvb status [devnet]
dvb start [devnet]
dvb stop [devnet]
dvb destroy [devnet]
dvb list

# Node operations
dvb nodes list [devnet]
dvb nodes logs [devnet] [node]
dvb nodes restart [devnet] [node]
dvb nodes health [devnet]

# Upgrades
dvb upgrade create [flags]
dvb upgrade status [name]
dvb upgrade list [devnet]
dvb upgrade cancel [name]

# Transactions
dvb tx submit [flags]
dvb tx status [id]
dvb tx list [devnet]

# Governance shortcuts
dvb gov propose [devnet] [flags]
dvb gov vote [devnet] [proposal] [option]
dvb gov list [devnet]

# Daemon management
dvb daemon status
dvb daemon logs
dvb daemon config

# Standalone mode (force)
dvb --standalone deploy [flags]
```

---

## State Store

### Directory Structure

```
~/.devnet-builder/
├── devnetd.db           # BoltDB database
├── devnetd.sock         # Unix socket for gRPC
├── devnetd.pid          # PID file
├── config.toml          # Daemon configuration
├── plugins/             # Plugin binaries
├── cache/               # Binary cache (existing)
└── devnets/             # Devnet data directories (existing)
```

### BoltDB Buckets

- `devnets` - Devnet resources
- `nodes` - Node resources
- `upgrades` - Upgrade resources
- `transactions` - Transaction resources
- `exports` - Export resources
- `events` - Event log
- `meta` - Schema version, daemon metadata

---

## Migration Path

### Phase 1: Foundation (Weeks 1-2)

- Define resource model (protobuf + Go types)
- Implement BoltDB state store
- Create dvb binary skeleton with dual-mode detection
- No breaking changes to existing devnet-builder

### Phase 2: Core Daemon (Weeks 3-6)

- Implement controller manager and work queue
- Implement DevnetController, NodeController, HealthController
- Implement gRPC server with basic operations
- dvb client connects to daemon
- Reuse existing infrastructure via DI container

### Phase 3: Full Features (Weeks 7-9)

- UpgradeController with SDK version awareness
- TxController with plugin-based tx building
- Plugin v2 interface with multi-platform support
- Multi-devnet management
- Full streaming support

### Phase 4: Cleanup (Week 10)

- Deprecate standalone mode (optional)
- Remove redundant code paths
- Documentation and examples
- Performance optimization

### Backwards Compatibility

- dvb works standalone when daemon not running
- Identical command structure to existing devnet-builder
- Existing devnets migrate automatically on first daemon start
- devnet-builder binary kept for compatibility (or aliased to `dvb --standalone`)

---

## Directory Structure (Post-Refactor)

```
columbus/
├── cmd/
│   ├── devnet-builder/    # Existing (kept for compatibility)
│   ├── devnetd/           # NEW: daemon entry point
│   └── dvb/               # NEW: client entry point
├── api/
│   └── proto/v1/          # NEW: protobuf definitions
├── internal/
│   ├── application/       # Existing (reused by controllers)
│   ├── domain/            # Existing
│   ├── infrastructure/    # Existing (reused)
│   ├── daemon/            # NEW
│   │   ├── server/        # gRPC server
│   │   ├── controller/    # Controllers
│   │   ├── store/         # State store
│   │   └── types/         # Resource types
│   └── client/            # NEW: dvb client lib
└── pkg/
    ├── network/           # Existing
    └── plugin/v2/         # NEW: plugin v2 interface
```

---

## Open Questions

1. **Tempo/Commonware plugin details** - Need more information on Tempo's specific primitives and transaction types
2. **IBC relayer integration** - Should the daemon manage IBC relayers for multi-chain scenarios?
3. **Remote daemon** - Should dvb support connecting to remote daemons (not just local Unix socket)?
4. **Authentication** - Should the gRPC API have authentication for production use?

---

## Next Steps

1. Review and approve this design
2. Create implementation plan with detailed tasks
3. Set up git worktree for isolated development
4. Begin Phase 1 implementation
