# Architecture Overview

This document provides a deep dive into Devnet Builder v2's architecture, design patterns, and implementation details.

## Table of Contents

- [Architecture Philosophy](#architecture-philosophy)
- [System Components](#system-components)
- [Controller-Reconciler Pattern](#controller-reconciler-pattern)
- [Resource Model](#resource-model)
- [State Management](#state-management)
- [gRPC API Layer](#grpc-api-layer)
- [Plugin System](#plugin-system)
- [Infrastructure Layer](#infrastructure-layer)
- [Concurrency Model](#concurrency-model)
- [Failure Handling](#failure-handling)

## Architecture Philosophy

### Design Principles

1. **Declarative over Imperative** - Users declare desired state; controllers reconcile to achieve it
2. **Single Responsibility** - Each controller manages one concern
3. **Event-Driven Reconciliation** - Controllers react to state changes, not time-based polling
4. **Fail-Safe Defaults** - System defaults to safe states on errors
5. **Observable** - All state changes generate events; full audit trail

### Inspiration

The architecture draws from:

- **Kubernetes** - Controller pattern, resource model, reconciliation loops
- **Terraform** - Declarative infrastructure, state management
- **Docker** - Client-daemon model, container lifecycle
- **systemd** - Process supervision, dependency management

## System Components

```
┌────────────────────────────────────────────────────────────────┐
│                                                                 │
│                         User / CI/CD                           │
│                              │                                  │
└──────────────────────────────┼─────────────────────────────────┘
                               │
                    ┌──────────▼──────────┐
                    │   dvb (Client)      │
                    │  - CLI Interface    │
                    │  - gRPC Client      │
                    └──────────┬──────────┘
                               │ Unix Socket / TCP
┌──────────────────────────────▼─────────────────────────────────┐
│                     devnetd (Daemon)                            │
│                                                                 │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │              gRPC Server (API Layer)                     │  │
│  │                                                          │  │
│  │  DevnetService    │ NodeService    │ UpgradeService    │  │
│  │  TransactionSvc   │ ExportService  │ PluginService     │  │
│  │  DaemonService                                          │  │
│  └────────────────────────┬─────────────────────────────────┘  │
│                           │                                    │
│  ┌────────────────────────▼─────────────────────────────────┐  │
│  │           Controller Manager (Orchestration)             │  │
│  │                                                          │  │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐     │  │
│  │  │   Informer  │  │ Work Queue  │  │ Controllers │     │  │
│  │  │             │  │             │  │             │     │  │
│  │  │  - Watch    │──│  - Keyed    │──│ - Devnet    │     │  │
│  │  │  - List     │  │  - Buffered │  │ - Node      │     │  │
│  │  │  - Events   │  │  - Dedupe   │  │ - Health    │     │  │
│  │  └─────────────┘  └─────────────┘  │ - Upgrade   │     │  │
│  │                                     │ - Tx        │     │  │
│  │                                     │ - Network   │     │  │
│  │                                     └─────────────┘     │  │
│  └────────────────────────┬─────────────────────────────────┘  │
│                           │                                    │
│  ┌────────────────────────▼─────────────────────────────────┐  │
│  │         State Store (BoltDB) - ACID Persistence          │  │
│  │                                                          │  │
│  │  Buckets:                                               │  │
│  │  - devnets/       - nodes/         - upgrades/         │  │
│  │  - transactions/  - exports/       - events/           │  │
│  │  - meta/          - locks/                             │  │
│  └────────────────────────┬─────────────────────────────────┘  │
│                           │                                    │
│  ┌────────────────────────▼─────────────────────────────────┐  │
│  │          Runtime Layer (Infrastructure Access)           │  │
│  │                                                          │  │
│  │  Container Runtime:       │  Network Plugins:           │  │
│  │  - Docker Engine          │  - Cosmos SDK (v0.47/50/53) │  │
│  │  - Container Lifecycle    │  - EVM (Geth/Besu/Reth)     │  │
│  │  - Volume Management      │  - Tempo/Commonware         │  │
│  │                                                          │  │
│  │  Supporting Services:                                   │  │
│  │  - Binary Cache   - RPC Clients   - Key Management     │  │
│  └──────────────────────────────────────────────────────────┘  │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

## Controller-Reconciler Pattern

### Core Concept

Controllers continuously drive actual state toward desired state through reconciliation:

```go
type Controller interface {
    // Reconcile a single resource by name
    Reconcile(ctx context.Context, name string) error
}

// Generic reconciliation loop
func (m *Manager) Run(ctx context.Context) error {
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case key := <-m.workQueue:
            // Get controller for resource type
            controller := m.getController(key.Type)

            // Reconcile with retries
            err := controller.Reconcile(ctx, key.Name)
            if err != nil {
                m.workQueue.AddAfter(key, backoff)
            }
        }
    }
}
```

### Reconciliation Phases

Each controller implements a state machine:

```go
func (c *DevnetController) Reconcile(ctx context.Context, name string) error {
    devnet, err := c.store.GetDevnet(ctx, name)
    if err != nil {
        return err
    }

    switch devnet.Status.Phase {
    case "":
        return c.transitionToPending(ctx, devnet)
    case types.DevnetPhasePending:
        return c.transitionToProvisioning(ctx, devnet)
    case types.DevnetPhaseProvisioning:
        return c.waitForProvisioning(ctx, devnet)
    case types.DevnetPhaseRunning:
        return c.ensureRunning(ctx, devnet)
    case types.DevnetPhaseDegraded:
        return c.attemptRecovery(ctx, devnet)
    case types.DevnetPhaseStopped:
        return c.handleStopped(ctx, devnet)
    }

    return nil
}
```

### Work Queue

The work queue provides:

- **Deduplication** - Multiple events for same resource → single reconciliation
- **Rate Limiting** - Exponential backoff for failing reconciliations
- **Ordering** - FIFO within resource type
- **Backpressure** - Bounded queue prevents resource exhaustion

```go
type WorkQueue struct {
    queue chan QueueKey
    processing sync.Map  // name → inProgress
    rateLimiter RateLimiter
}

func (q *WorkQueue) Add(key QueueKey) {
    // Skip if already processing
    if _, exists := q.processing.LoadOrStore(key.Name, true); exists {
        return
    }

    // Apply rate limiting
    delay := q.rateLimiter.When(key)
    time.AfterFunc(delay, func() {
        q.queue <- key
    })
}
```

## Resource Model

### Resource Structure

All resources follow the same pattern:

```go
type Resource struct {
    Metadata ResourceMeta  `json:"metadata"`
    Spec     interface{}    `json:"spec"`     // Desired state
    Status   interface{}    `json:"status"`   // Actual state
}

type ResourceMeta struct {
    Name      string    `json:"name"`
    Namespace string    `json:"namespace,omitempty"`
    Labels    map[string]string `json:"labels,omitempty"`
    CreatedAt time.Time `json:"createdAt"`
    UpdatedAt time.Time `json:"updatedAt"`
}
```

### Resource Types

#### Devnet

Represents a complete blockchain network:

```go
type DevnetSpec struct {
    Name         string        `json:"name"`
    NetworkType  string        `json:"networkType"`  // cosmos, evm, tempo
    Plugin       string        `json:"plugin"`
    Validators   int           `json:"validators"`
    FullNodes    int           `json:"fullNodes"`
    BinarySource BinarySource  `json:"binarySource"`
    Genesis      GenesisConfig `json:"genesis,omitempty"`
    Mode         string        `json:"mode"`  // docker, local
}

type DevnetStatus struct {
    Phase             string      `json:"phase"`
    Nodes             int         `json:"nodes"`
    ReadyNodes        int         `json:"readyNodes"`
    CurrentHeight     int64       `json:"currentHeight"`
    SDKVersion        string      `json:"sdkVersion"`
    SDKVersionHistory []SDKChange `json:"sdkVersionHistory"`
    Conditions        []Condition `json:"conditions"`
}
```

#### Node

Represents an individual blockchain node:

```go
type NodeSpec struct {
    DevnetRef  string `json:"devnetRef"`
    Index      int    `json:"index"`
    Role       string `json:"role"`  // validator, fullnode, seed
    BinaryPath string `json:"binaryPath"`
    HomeDir    string `json:"homeDir"`
    Ports      PortConfig `json:"ports"`
}

type NodeStatus struct {
    Phase        string    `json:"phase"`
    ContainerID  string    `json:"containerId,omitempty"`
    PID          int       `json:"pid,omitempty"`
    BlockHeight  int64     `json:"blockHeight"`
    PeerCount    int       `json:"peerCount"`
    CatchingUp   bool      `json:"catchingUp"`
    RestartCount int       `json:"restartCount"`
    LastRestart  time.Time `json:"lastRestart,omitempty"`
    HealthChecks []HealthCheck `json:"healthChecks"`
}
```

#### Transaction

Represents transaction lifecycle:

```go
type TransactionSpec struct {
    DevnetRef string          `json:"devnetRef"`
    TxType    string          `json:"txType"`
    Signer    string          `json:"signer"`
    Payload   json.RawMessage `json:"payload"`
    GasLimit  uint64          `json:"gasLimit,omitempty"`
    Memo      string          `json:"memo,omitempty"`
}

type TransactionStatus struct {
    Phase   string    `json:"phase"`  // Pending, Building, Signing, Submitted, Confirmed, Failed
    TxHash  string    `json:"txHash,omitempty"`
    Height  int64     `json:"height,omitempty"`
    GasUsed int64     `json:"gasUsed,omitempty"`
    Error   string    `json:"error,omitempty"`
    Events  []TxEvent `json:"events,omitempty"`
}
```

### Resource Relationships

```
Devnet
  ├─► Node (validator-0)
  ├─► Node (validator-1)
  ├─► Node (validator-2)
  ├─► Node (fullnode-0)
  │
  ├─► Transaction (tx-1)
  ├─► Transaction (tx-2)
  │
  └─► Upgrade (v2.0)
```

Controllers maintain referential integrity:
- Deleting Devnet cascades to Nodes, Transactions, Upgrades
- Node failures update Devnet status
- Upgrade completion updates Devnet SDK version

## State Management

### BoltDB Storage

BoltDB provides ACID transactions with embedded storage:

```go
type Store interface {
    // Devnets
    CreateDevnet(ctx context.Context, devnet *types.Devnet) error
    GetDevnet(ctx context.Context, name string) (*types.Devnet, error)
    UpdateDevnet(ctx context.Context, devnet *types.Devnet) error
    ListDevnets(ctx context.Context) ([]*types.Devnet, error)
    DeleteDevnet(ctx context.Context, name string) error

    // Similar for Nodes, Transactions, Upgrades...
}
```

### Bucket Organization

```
devnetd.db
├── devnets/
│   ├── osmosis-test    → {Devnet JSON}
│   └── hub-testnet     → {Devnet JSON}
├── nodes/
│   ├── osmosis-test/validator-0  → {Node JSON}
│   ├── osmosis-test/validator-1  → {Node JSON}
│   └── hub-testnet/validator-0   → {Node JSON}
├── transactions/
│   ├── tx-12345  → {Transaction JSON}
│   └── tx-12346  → {Transaction JSON}
├── upgrades/
│   └── osmosis-test/v2.0  → {Upgrade JSON}
├── events/
│   ├── 2026-01-25/001  → {Event JSON}
│   └── 2026-01-25/002  → {Event JSON}
└── meta/
    ├── schema_version  → "2"
    └── daemon_id       → "uuid"
```

### Transactions and Consistency

All state mutations use BoltDB transactions:

```go
func (s *BoltStore) UpdateDevnetWithNodes(
    ctx context.Context,
    devnet *types.Devnet,
    nodes []*types.Node,
) error {
    return s.db.Update(func(tx *bolt.Tx) error {
        // Update devnet
        if err := putJSON(tx, "devnets", devnet.Metadata.Name, devnet); err != nil {
            return err
        }

        // Update all nodes
        for _, node := range nodes {
            key := fmt.Sprintf("%s/%s", node.Spec.DevnetRef, node.Metadata.Name)
            if err := putJSON(tx, "nodes", key, node); err != nil {
                return err
            }
        }

        return nil
    })
}
```

## gRPC API Layer

### Service Architecture

Each service maps to a resource controller:

```protobuf
service DevnetService {
  rpc Create(CreateDevnetRequest) returns (CreateDevnetResponse);
  rpc Get(GetDevnetRequest) returns (GetDevnetResponse);
  rpc List(ListDevnetsRequest) returns (ListDevnetsResponse);
  rpc Delete(DeleteDevnetRequest) returns (DeleteDevnetResponse);
  rpc Start(StartDevnetRequest) returns (StartDevnetResponse);
  rpc Stop(StopDevnetRequest) returns (StopDevnetResponse);

  // Streaming
  rpc Watch(WatchDevnetRequest) returns (stream DevnetEvent);
  rpc StreamLogs(StreamLogsRequest) returns (stream LogEntry);
}
```

### Request Flow

```
Client Request
     │
     ▼
gRPC Service Handler
     │
     ├─► Validate Request
     ├─► Create/Update Resource in Store
     └─► Enqueue Work Item
              │
              ▼
         Work Queue
              │
              ▼
      Controller Reconciles
              │
              ▼
      State Store Updated
              │
              ▼
     Event Watchers Notified
```

### Streaming Implementation

Watch streams provide real-time updates:

```go
func (s *DevnetService) Watch(
    req *pb.WatchDevnetRequest,
    stream pb.DevnetService_WatchServer,
) error {
    // Create event channel
    events := make(chan *types.DevnetEvent, 100)

    // Register with event system
    s.events.Subscribe(events, func(e *types.Event) bool {
        return e.Type == "devnet" && e.ResourceName == req.Name
    })
    defer s.events.Unsubscribe(events)

    // Stream events
    for {
        select {
        case <-stream.Context().Done():
            return stream.Context().Err()
        case event := <-events:
            if err := stream.Send(event.ToProto()); err != nil {
                return err
            }
        }
    }
}
```

## Plugin System

### Plugin Interface

Network plugins implement chain-specific logic:

```go
type NetworkPlugin interface {
    // Metadata
    GetInfo() (*PluginInfo, error)
    GetSupportedSDKVersions() (*SDKVersionRange, error)

    // Lifecycle
    Initialize(config *Config) error
    GenerateGenesis(req *GenesisRequest) (*GenesisResult, error)
    StartNode(req *StartNodeRequest) (*StartNodeResult, error)
    StopNode(req *StopNodeRequest) error

    // Queries
    GetNodeStatus(req *NodeStatusRequest) (*NodeStatus, error)
    GetBlockHeight(rpcURL string) (int64, error)

    // Transactions
    CreateTxBuilder(req *CreateTxBuilderRequest) (TxBuilder, error)
    GetSupportedTxTypes() (*SupportedTxTypes, error)

    // Cleanup
    Close() error
}
```

### Plugin Discovery

Plugins are discovered via:

```
~/.devnet-builder/plugins/
├── cosmos-v047-plugin
├── cosmos-v050-plugin
├── cosmos-v053-plugin
├── evm-geth-plugin
└── tempo-plugin
```

Each plugin is a gRPC server using HashiCorp go-plugin:

```go
// Plugin implementation
type CosmosV050Plugin struct {
    config *Config
    txBuilders map[string]TxBuilder
}

// Plugin server
func main() {
    plugin.Serve(&plugin.ServeConfig{
        HandshakeConfig: handshake,
        Plugins: map[string]plugin.Plugin{
            "network": &NetworkPluginImpl{},
        },
        GRPCServer: plugin.DefaultGRPCServer,
    })
}
```

## Infrastructure Layer

### Container Runtime

Docker integration for node management:

```go
type ContainerRuntime interface {
    CreateContainer(ctx context.Context, config *ContainerConfig) (string, error)
    StartContainer(ctx context.Context, id string) error
    StopContainer(ctx context.Context, id string, timeout time.Duration) error
    RemoveContainer(ctx context.Context, id string) error
    GetContainerLogs(ctx context.Context, id string, follow bool) (io.ReadCloser, error)
    GetContainerStats(ctx context.Context, id string) (*Stats, error)
}

type DockerRuntime struct {
    client *docker.Client
    network string
}
```

### Binary Cache

Efficient binary management:

```go
type BinaryCache interface {
    Get(ctx context.Context, req *BinaryRequest) (string, error)
    Has(ctx context.Context, req *BinaryRequest) bool
    Evict(ctx context.Context, olderThan time.Duration) error
}

// Cache structure
~/.devnet-builder/cache/
├── osmosisd-v24.0.0-linux-amd64
├── osmosisd-v25.0.0-linux-amd64
└── gaiad-v14.1.0-linux-amd64
```

## Concurrency Model

### Thread Safety

All controllers use:

```go
type TxController struct {
    store   store.Store
    runtime TxRuntime
    logger  *slog.Logger

    // Thread-safe cache
    cacheMu         sync.RWMutex
    unsignedTxCache map[string]*network.UnsignedTx
}

func (c *TxController) getFromCache(name string) (*network.UnsignedTx, bool) {
    c.cacheMu.RLock()
    defer c.cacheMu.RUnlock()
    tx, ok := c.unsignedTxCache[name]
    return tx, ok
}

func (c *TxController) putInCache(name string, tx *network.UnsignedTx) {
    c.cacheMu.Lock()
    defer c.cacheMu.Unlock()
    c.unsignedTxCache[name] = tx
}
```

### Context Propagation

All operations respect context cancellation:

```go
func (c *DevnetController) Reconcile(ctx context.Context, name string) error {
    // Check context at entry
    select {
    case <-ctx.Done():
        return ctx.Err()
    default:
    }

    // Pass context to all operations
    devnet, err := c.store.GetDevnet(ctx, name)
    if err != nil {
        return err
    }

    // Check context before expensive operations
    select {
    case <-ctx.Done():
        return ctx.Err()
    default:
    }

    return c.provision(ctx, devnet)
}
```

## Failure Handling

### Node Crash Recovery

HealthController implements exponential backoff:

```go
func (c *HealthController) handleCrashedNode(ctx context.Context, node *types.Node) error {
    if node.Status.RestartCount >= c.maxRestarts {
        return c.markNodeFailed(ctx, node)
    }

    // Exponential backoff: 5s, 10s, 20s, 40s...
    backoff := 5 * time.Second * time.Duration(math.Pow(2, float64(node.Status.RestartCount)))

    c.logger.Info("scheduling node restart",
        "node", node.Metadata.Name,
        "attempt", node.Status.RestartCount+1,
        "backoff", backoff)

    time.AfterFunc(backoff, func() {
        c.workQueue.Add(WorkKey{Type: "node", Name: node.Metadata.Name})
    })

    return nil
}
```

### Transaction Retry

TxController handles transient failures:

```go
func (c *TxController) handleBroadcastError(ctx context.Context, tx *types.Transaction, err error) error {
    // Classify error
    if isRetryable(err) {
        tx.Status.Phase = types.TxPhaseRetrying
        tx.Status.RetryCount++

        if tx.Status.RetryCount > c.maxRetries {
            return c.failTransaction(ctx, tx, "max retries exceeded")
        }

        return c.store.UpdateTransaction(ctx, tx)
    }

    // Non-retryable error
    return c.failTransaction(ctx, tx, err.Error())
}
```

### Graceful Shutdown

Daemon handles SIGTERM gracefully:

```go
func (d *Daemon) Shutdown(ctx context.Context) error {
    d.logger.Info("beginning graceful shutdown")

    // 1. Stop accepting new requests
    d.grpcServer.GracefulStop()

    // 2. Stop controller manager
    d.controllerManager.Stop()

    // 3. Drain work queue
    d.workQueue.Drain(ctx)

    // 4. Close store
    d.store.Close()

    d.logger.Info("shutdown complete")
    return nil
}
```

## Next Steps

- **[Daemon Operations](daemon.md)** - Learn daemon configuration and management
- **[Client Guide](client.md)** - Master the dvb CLI
- **[Transaction System](transactions.md)** - Deep dive into transaction building
- **[Plugin Development](plugins.md)** - Create custom network plugins
