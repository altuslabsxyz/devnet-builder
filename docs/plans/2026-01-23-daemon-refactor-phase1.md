# Daemon Refactor Phase 1: Foundation

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Establish the foundation for daemon architecture: resource model, state store, and binary skeletons.

**Architecture:** Controller-Reconciler pattern with BoltDB persistence. Two binaries: `devnetd` (daemon) and `dvb` (client). gRPC API for communication.

**Tech Stack:** Go 1.24, Protocol Buffers, BoltDB (bbolt), gRPC, Cobra CLI

**Design Document:** `docs/plans/2026-01-23-daemon-refactor-design.md`

---

## Task 1: Create Directory Structure

**Files:**
- Create: `api/proto/v1/` (directory)
- Create: `internal/daemon/types/` (directory)
- Create: `internal/daemon/store/` (directory)
- Create: `internal/daemon/server/` (directory)
- Create: `internal/daemon/controller/` (directory)
- Create: `internal/client/` (directory)
- Create: `cmd/devnetd/` (directory)
- Create: `cmd/dvb/` (directory)

**Step 1: Create directories**

```bash
mkdir -p api/proto/v1
mkdir -p internal/daemon/types
mkdir -p internal/daemon/store
mkdir -p internal/daemon/server
mkdir -p internal/daemon/controller
mkdir -p internal/client
mkdir -p cmd/devnetd
mkdir -p cmd/dvb
```

**Step 2: Commit**

```bash
git add -A
git commit -m "chore: create directory structure for daemon refactor"
```

---

## Task 2: Define Resource Types (Go)

**Files:**
- Create: `internal/daemon/types/meta.go`
- Create: `internal/daemon/types/devnet.go`
- Create: `internal/daemon/types/node.go`
- Create: `internal/daemon/types/upgrade.go`
- Create: `internal/daemon/types/transaction.go`
- Test: `internal/daemon/types/types_test.go`

**Step 1: Write test for resource metadata**

```go
// internal/daemon/types/types_test.go
package types

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResourceMeta_JSON(t *testing.T) {
	meta := ResourceMeta{
		Name:       "test-devnet",
		Generation: 1,
		CreatedAt:  time.Date(2026, 1, 23, 10, 0, 0, 0, time.UTC),
		UpdatedAt:  time.Date(2026, 1, 23, 11, 0, 0, 0, time.UTC),
		Labels:     map[string]string{"env": "test"},
	}

	data, err := json.Marshal(meta)
	require.NoError(t, err)

	var decoded ResourceMeta
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, meta.Name, decoded.Name)
	assert.Equal(t, meta.Generation, decoded.Generation)
	assert.Equal(t, meta.Labels["env"], decoded.Labels["env"])
}

func TestDevnet_DefaultPhase(t *testing.T) {
	devnet := &Devnet{
		Metadata: ResourceMeta{Name: "test"},
		Spec:     DevnetSpec{Plugin: "stable", Validators: 4},
	}

	assert.Empty(t, devnet.Status.Phase)
}

func TestDevnetStatus_SDKVersionHistory(t *testing.T) {
	status := DevnetStatus{
		Phase:      PhaseRunning,
		SDKVersion: "0.53.4",
		SDKVersionHistory: []SDKVersionChange{
			{
				FromVersion: "0.50.9",
				ToVersion:   "0.53.4",
				Height:      15000,
				UpgradeRef:  "v2-upgrade",
			},
		},
	}

	assert.Len(t, status.SDKVersionHistory, 1)
	assert.Equal(t, "0.50.9", status.SDKVersionHistory[0].FromVersion)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/daemon/types/... -v`
Expected: FAIL (package doesn't exist)

**Step 3: Write meta.go**

```go
// internal/daemon/types/meta.go
package types

import "time"

// ResourceMeta contains metadata common to all resources.
type ResourceMeta struct {
	// Name is the unique identifier for this resource.
	Name string `json:"name"`

	// Generation is incremented each time the resource is updated.
	// Used for optimistic concurrency control.
	Generation int64 `json:"generation"`

	// CreatedAt is when the resource was created.
	CreatedAt time.Time `json:"createdAt"`

	// UpdatedAt is when the resource was last modified.
	UpdatedAt time.Time `json:"updatedAt"`

	// Labels are key-value pairs for organizing resources.
	Labels map[string]string `json:"labels,omitempty"`

	// Annotations are key-value pairs for storing arbitrary metadata.
	Annotations map[string]string `json:"annotations,omitempty"`
}

// Condition represents a condition of a resource.
type Condition struct {
	Type               string    `json:"type"`
	Status             string    `json:"status"` // "True", "False", "Unknown"
	LastTransitionTime time.Time `json:"lastTransitionTime"`
	Reason             string    `json:"reason,omitempty"`
	Message            string    `json:"message,omitempty"`
}
```

**Step 4: Write devnet.go**

```go
// internal/daemon/types/devnet.go
package types

import "time"

// Phase constants for Devnet.
const (
	PhasePending      = "Pending"
	PhaseProvisioning = "Provisioning"
	PhaseRunning      = "Running"
	PhaseDegraded     = "Degraded"
	PhaseStopped      = "Stopped"
)

// Devnet represents a blockchain development network.
type Devnet struct {
	Metadata ResourceMeta  `json:"metadata"`
	Spec     DevnetSpec    `json:"spec"`
	Status   DevnetStatus  `json:"status"`
}

// DevnetSpec defines the desired state of a Devnet.
type DevnetSpec struct {
	// Plugin is the network plugin name (e.g., "stable", "osmosis", "geth").
	Plugin string `json:"plugin"`

	// NetworkType is the blockchain platform ("cosmos", "evm", "tempo").
	NetworkType string `json:"networkType,omitempty"`

	// Validators is the number of validator nodes.
	Validators int `json:"validators"`

	// FullNodes is the number of non-validator full nodes.
	FullNodes int `json:"fullNodes,omitempty"`

	// Mode is the execution mode ("docker" or "local").
	Mode string `json:"mode"`

	// BinarySource specifies where to get the node binary.
	BinarySource BinarySource `json:"binarySource,omitempty"`

	// SnapshotURL is an optional URL to download chain state from.
	SnapshotURL string `json:"snapshotUrl,omitempty"`

	// GenesisPath is an optional path to a custom genesis file.
	GenesisPath string `json:"genesisPath,omitempty"`

	// Ports configures port allocation for nodes.
	Ports PortConfig `json:"ports,omitempty"`

	// Resources configures resource limits for Docker mode.
	Resources ResourceLimits `json:"resources,omitempty"`

	// Options are plugin-specific configuration options.
	Options map[string]string `json:"options,omitempty"`
}

// DevnetStatus defines the observed state of a Devnet.
type DevnetStatus struct {
	// Phase is the current lifecycle phase.
	Phase string `json:"phase"`

	// Nodes is the total number of nodes.
	Nodes int `json:"nodes"`

	// ReadyNodes is the number of nodes that are healthy.
	ReadyNodes int `json:"readyNodes"`

	// CurrentHeight is the latest block height.
	CurrentHeight int64 `json:"currentHeight"`

	// SDKVersion is the detected Cosmos SDK version (for cosmos networks).
	SDKVersion string `json:"sdkVersion,omitempty"`

	// SDKVersionHistory tracks SDK version changes from upgrades.
	SDKVersionHistory []SDKVersionChange `json:"sdkVersionHistory,omitempty"`

	// LastHealthCheck is when health was last checked.
	LastHealthCheck time.Time `json:"lastHealthCheck"`

	// Conditions represent the current conditions of the devnet.
	Conditions []Condition `json:"conditions,omitempty"`

	// Message provides additional status information.
	Message string `json:"message,omitempty"`
}

// SDKVersionChange records an SDK version change from an upgrade.
type SDKVersionChange struct {
	FromVersion string    `json:"fromVersion"`
	ToVersion   string    `json:"toVersion"`
	Height      int64     `json:"height"`
	Timestamp   time.Time `json:"timestamp"`
	UpgradeRef  string    `json:"upgradeRef"`
}

// BinarySource specifies where to obtain a node binary.
type BinarySource struct {
	// Type is the source type: "cache", "local", "github", "url".
	Type string `json:"type"`

	// Path is used for "local" type.
	Path string `json:"path,omitempty"`

	// Version is used for "cache" and "github" types.
	Version string `json:"version,omitempty"`

	// URL is used for "url" type.
	URL string `json:"url,omitempty"`

	// Owner is the GitHub owner for "github" type.
	Owner string `json:"owner,omitempty"`

	// Repo is the GitHub repo for "github" type.
	Repo string `json:"repo,omitempty"`
}

// PortConfig configures port allocation.
type PortConfig struct {
	// BaseRPC is the starting RPC port (default: 26657).
	BaseRPC int `json:"baseRpc,omitempty"`

	// BaseP2P is the starting P2P port (default: 26656).
	BaseP2P int `json:"baseP2p,omitempty"`

	// BaseGRPC is the starting gRPC port (default: 9090).
	BaseGRPC int `json:"baseGrpc,omitempty"`

	// BaseAPI is the starting REST API port (default: 1317).
	BaseAPI int `json:"baseApi,omitempty"`
}

// ResourceLimits configures container resource limits.
type ResourceLimits struct {
	// Memory limit (e.g., "2g").
	Memory string `json:"memory,omitempty"`

	// CPUs limit (e.g., "2.0").
	CPUs string `json:"cpus,omitempty"`
}
```

**Step 5: Write node.go**

```go
// internal/daemon/types/node.go
package types

// Node phase constants.
const (
	NodePhaseStopped  = "Stopped"
	NodePhaseStarting = "Starting"
	NodePhaseRunning  = "Running"
	NodePhaseCrashed  = "Crashed"
)

// Node represents a single blockchain node within a Devnet.
type Node struct {
	Metadata ResourceMeta `json:"metadata"`
	Spec     NodeSpec     `json:"spec"`
	Status   NodeStatus   `json:"status"`
}

// NodeSpec defines the desired state of a Node.
type NodeSpec struct {
	// DevnetRef is the name of the parent Devnet.
	DevnetRef string `json:"devnetRef"`

	// Index is the node's index within the devnet (0-based).
	Index int `json:"index"`

	// Role is "validator" or "fullnode".
	Role string `json:"role"`

	// BinaryPath is the path to the node binary.
	BinaryPath string `json:"binaryPath"`

	// HomeDir is the node's data directory.
	HomeDir string `json:"homeDir"`

	// Desired is the desired state: "Running" or "Stopped".
	Desired string `json:"desired"`
}

// NodeStatus defines the observed state of a Node.
type NodeStatus struct {
	// Phase is the current phase.
	Phase string `json:"phase"`

	// ContainerID is the Docker container ID (docker mode).
	ContainerID string `json:"containerId,omitempty"`

	// PID is the process ID (local mode).
	PID int `json:"pid,omitempty"`

	// BlockHeight is the node's current block height.
	BlockHeight int64 `json:"blockHeight"`

	// PeerCount is the number of connected peers.
	PeerCount int `json:"peerCount"`

	// CatchingUp indicates if the node is syncing.
	CatchingUp bool `json:"catchingUp"`

	// RestartCount is how many times the node has been restarted.
	RestartCount int `json:"restartCount"`

	// ValidatorAddress is the validator's address (if validator).
	ValidatorAddress string `json:"validatorAddress,omitempty"`

	// ValidatorPubKey is the validator's public key (if validator).
	ValidatorPubKey string `json:"validatorPubKey,omitempty"`

	// Message provides additional status information.
	Message string `json:"message,omitempty"`
}
```

**Step 6: Write upgrade.go**

```go
// internal/daemon/types/upgrade.go
package types

// Upgrade phase constants.
const (
	UpgradePhasePending   = "Pending"
	UpgradePhaseProposing = "Proposing"
	UpgradePhaseVoting    = "Voting"
	UpgradePhaseWaiting   = "Waiting"
	UpgradePhaseSwitching = "Switching"
	UpgradePhaseVerifying = "Verifying"
	UpgradePhaseCompleted = "Completed"
	UpgradePhaseFailed    = "Failed"
)

// Upgrade represents a chain upgrade operation.
type Upgrade struct {
	Metadata ResourceMeta  `json:"metadata"`
	Spec     UpgradeSpec   `json:"spec"`
	Status   UpgradeStatus `json:"status"`
}

// UpgradeSpec defines the desired upgrade configuration.
type UpgradeSpec struct {
	// DevnetRef is the name of the target Devnet.
	DevnetRef string `json:"devnetRef"`

	// UpgradeName is the on-chain upgrade name.
	UpgradeName string `json:"upgradeName"`

	// TargetHeight is the block height for the upgrade.
	// 0 means auto-calculate (current + default offset).
	TargetHeight int64 `json:"targetHeight"`

	// NewBinary specifies the upgraded binary source.
	NewBinary BinarySource `json:"newBinary"`

	// WithExport enables state export before/after upgrade.
	WithExport bool `json:"withExport"`

	// AutoVote automatically votes yes with all validators.
	AutoVote bool `json:"autoVote"`
}

// UpgradeStatus defines the observed state of an Upgrade.
type UpgradeStatus struct {
	// Phase is the current upgrade phase.
	Phase string `json:"phase"`

	// ProposalID is the governance proposal ID.
	ProposalID uint64 `json:"proposalId,omitempty"`

	// VotesReceived is the number of validator votes.
	VotesReceived int `json:"votesReceived"`

	// VotesRequired is the number of votes needed.
	VotesRequired int `json:"votesRequired"`

	// CurrentHeight is the chain's current height.
	CurrentHeight int64 `json:"currentHeight"`

	// PreExportPath is the path to pre-upgrade state export.
	PreExportPath string `json:"preExportPath,omitempty"`

	// PostExportPath is the path to post-upgrade state export.
	PostExportPath string `json:"postExportPath,omitempty"`

	// Message provides additional status information.
	Message string `json:"message,omitempty"`

	// Error contains error details if phase is Failed.
	Error string `json:"error,omitempty"`
}
```

**Step 7: Write transaction.go**

```go
// internal/daemon/types/transaction.go
package types

import "encoding/json"

// Transaction phase constants.
const (
	TxPhasePending   = "Pending"
	TxPhaseSubmitted = "Submitted"
	TxPhaseConfirmed = "Confirmed"
	TxPhaseFailed    = "Failed"
)

// Transaction represents a blockchain transaction operation.
type Transaction struct {
	Metadata ResourceMeta      `json:"metadata"`
	Spec     TransactionSpec   `json:"spec"`
	Status   TransactionStatus `json:"status"`
}

// TransactionSpec defines the desired transaction.
type TransactionSpec struct {
	// DevnetRef is the name of the target Devnet.
	DevnetRef string `json:"devnetRef"`

	// TxType is the transaction type (e.g., "gov/vote", "staking/delegate").
	TxType string `json:"txType"`

	// Signer identifies who signs the tx (e.g., "validator:0", "account:alice").
	Signer string `json:"signer"`

	// Payload is the transaction-specific data (JSON).
	Payload json.RawMessage `json:"payload"`

	// SDKVersion overrides auto-detected SDK version.
	SDKVersion string `json:"sdkVersion,omitempty"`
}

// TransactionStatus defines the observed state of a Transaction.
type TransactionStatus struct {
	// Phase is the current phase.
	Phase string `json:"phase"`

	// TxHash is the transaction hash.
	TxHash string `json:"txHash,omitempty"`

	// Height is the block height where tx was included.
	Height int64 `json:"height,omitempty"`

	// GasUsed is the gas consumed by the transaction.
	GasUsed int64 `json:"gasUsed,omitempty"`

	// Error contains error details if phase is Failed.
	Error string `json:"error,omitempty"`
}
```

**Step 8: Run tests**

Run: `go test ./internal/daemon/types/... -v`
Expected: PASS

**Step 9: Commit**

```bash
git add internal/daemon/types/
git commit -m "feat(daemon): add resource type definitions

Define core resource types for daemon:
- ResourceMeta: common metadata for all resources
- Devnet: blockchain network with SDK version tracking
- Node: individual blockchain node
- Upgrade: chain upgrade with phase tracking
- Transaction: generic tx operation"
```

---

## Task 3: State Store Interface

**Files:**
- Create: `internal/daemon/store/interface.go`
- Create: `internal/daemon/store/errors.go`
- Test: `internal/daemon/store/interface_test.go`

**Step 1: Write test for store interface compliance**

```go
// internal/daemon/store/interface_test.go
package store

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestStoreInterface ensures the interface is well-defined.
func TestStoreInterface(t *testing.T) {
	// This is a compile-time check that BoltStore implements Store.
	// We'll implement BoltStore in the next task.
	var _ Store = (*mockStore)(nil)
}

// mockStore is a minimal mock for interface testing.
type mockStore struct{}

func (m *mockStore) CreateDevnet(ctx context.Context, devnet *Devnet) error { return nil }
func (m *mockStore) GetDevnet(ctx context.Context, name string) (*Devnet, error) { return nil, nil }
func (m *mockStore) UpdateDevnet(ctx context.Context, devnet *Devnet) error { return nil }
func (m *mockStore) DeleteDevnet(ctx context.Context, name string) error { return nil }
func (m *mockStore) ListDevnets(ctx context.Context) ([]*Devnet, error) { return nil, nil }

func (m *mockStore) CreateNode(ctx context.Context, node *Node) error { return nil }
func (m *mockStore) GetNode(ctx context.Context, devnetName string, index int) (*Node, error) { return nil, nil }
func (m *mockStore) UpdateNode(ctx context.Context, node *Node) error { return nil }
func (m *mockStore) DeleteNode(ctx context.Context, devnetName string, index int) error { return nil }
func (m *mockStore) ListNodes(ctx context.Context, devnetName string) ([]*Node, error) { return nil, nil }

func (m *mockStore) CreateUpgrade(ctx context.Context, upgrade *Upgrade) error { return nil }
func (m *mockStore) GetUpgrade(ctx context.Context, name string) (*Upgrade, error) { return nil, nil }
func (m *mockStore) UpdateUpgrade(ctx context.Context, upgrade *Upgrade) error { return nil }
func (m *mockStore) ListUpgrades(ctx context.Context, devnetName string) ([]*Upgrade, error) { return nil, nil }

func (m *mockStore) CreateTransaction(ctx context.Context, tx *Transaction) error { return nil }
func (m *mockStore) GetTransaction(ctx context.Context, id string) (*Transaction, error) { return nil, nil }
func (m *mockStore) UpdateTransaction(ctx context.Context, tx *Transaction) error { return nil }
func (m *mockStore) ListTransactions(ctx context.Context, devnetName string, opts ListTxOptions) ([]*Transaction, error) { return nil, nil }

func (m *mockStore) Watch(ctx context.Context, resourceType string, handler WatchHandler) error { return nil }
func (m *mockStore) Close() error { return nil }

func TestNotFoundError(t *testing.T) {
	err := &NotFoundError{Resource: "devnet", Name: "test"}
	assert.Contains(t, err.Error(), "devnet")
	assert.Contains(t, err.Error(), "test")
	assert.True(t, IsNotFound(err))
}

func TestConflictError(t *testing.T) {
	err := &ConflictError{Resource: "devnet", Name: "test", Message: "generation mismatch"}
	assert.Contains(t, err.Error(), "conflict")
	assert.True(t, IsConflict(err))
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/daemon/store/... -v`
Expected: FAIL (package doesn't exist)

**Step 3: Write errors.go**

```go
// internal/daemon/store/errors.go
package store

import "fmt"

// NotFoundError is returned when a resource is not found.
type NotFoundError struct {
	Resource string
	Name     string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("%s %q not found", e.Resource, e.Name)
}

// IsNotFound returns true if err is a NotFoundError.
func IsNotFound(err error) bool {
	_, ok := err.(*NotFoundError)
	return ok
}

// ConflictError is returned on optimistic concurrency conflicts.
type ConflictError struct {
	Resource string
	Name     string
	Message  string
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("conflict updating %s %q: %s", e.Resource, e.Name, e.Message)
}

// IsConflict returns true if err is a ConflictError.
func IsConflict(err error) bool {
	_, ok := err.(*ConflictError)
	return ok
}

// AlreadyExistsError is returned when creating a resource that already exists.
type AlreadyExistsError struct {
	Resource string
	Name     string
}

func (e *AlreadyExistsError) Error() string {
	return fmt.Sprintf("%s %q already exists", e.Resource, e.Name)
}

// IsAlreadyExists returns true if err is an AlreadyExistsError.
func IsAlreadyExists(err error) bool {
	_, ok := err.(*AlreadyExistsError)
	return ok
}
```

**Step 4: Write interface.go**

```go
// internal/daemon/store/interface.go
package store

import (
	"context"

	"github.com/altuslabsxyz/devnet-builder/internal/daemon/types"
)

// Re-export types for convenience.
type (
	Devnet      = types.Devnet
	Node        = types.Node
	Upgrade     = types.Upgrade
	Transaction = types.Transaction
)

// WatchHandler is called when a resource changes.
type WatchHandler func(eventType string, resource interface{})

// ListTxOptions configures transaction listing.
type ListTxOptions struct {
	// TxType filters by transaction type.
	TxType string
	// Phase filters by phase.
	Phase string
	// Limit is the maximum number of results.
	Limit int
}

// Store defines the interface for resource persistence.
type Store interface {
	// Devnet operations
	CreateDevnet(ctx context.Context, devnet *Devnet) error
	GetDevnet(ctx context.Context, name string) (*Devnet, error)
	UpdateDevnet(ctx context.Context, devnet *Devnet) error
	DeleteDevnet(ctx context.Context, name string) error
	ListDevnets(ctx context.Context) ([]*Devnet, error)

	// Node operations
	CreateNode(ctx context.Context, node *Node) error
	GetNode(ctx context.Context, devnetName string, index int) (*Node, error)
	UpdateNode(ctx context.Context, node *Node) error
	DeleteNode(ctx context.Context, devnetName string, index int) error
	ListNodes(ctx context.Context, devnetName string) ([]*Node, error)

	// Upgrade operations
	CreateUpgrade(ctx context.Context, upgrade *Upgrade) error
	GetUpgrade(ctx context.Context, name string) (*Upgrade, error)
	UpdateUpgrade(ctx context.Context, upgrade *Upgrade) error
	ListUpgrades(ctx context.Context, devnetName string) ([]*Upgrade, error)

	// Transaction operations
	CreateTransaction(ctx context.Context, tx *Transaction) error
	GetTransaction(ctx context.Context, id string) (*Transaction, error)
	UpdateTransaction(ctx context.Context, tx *Transaction) error
	ListTransactions(ctx context.Context, devnetName string, opts ListTxOptions) ([]*Transaction, error)

	// Watch watches for resource changes.
	Watch(ctx context.Context, resourceType string, handler WatchHandler) error

	// Close closes the store.
	Close() error
}
```

**Step 5: Run tests**

Run: `go test ./internal/daemon/store/... -v`
Expected: PASS

**Step 6: Commit**

```bash
git add internal/daemon/store/
git commit -m "feat(daemon): add state store interface

Define Store interface for resource persistence:
- CRUD operations for Devnet, Node, Upgrade, Transaction
- Watch for real-time change notifications
- Error types: NotFoundError, ConflictError, AlreadyExistsError"
```

---

## Task 4: BoltDB Store Implementation

**Files:**
- Create: `internal/daemon/store/bolt.go`
- Create: `internal/daemon/store/bolt_devnet.go`
- Test: `internal/daemon/store/bolt_test.go`

**Step 1: Write test for BoltDB store**

```go
// internal/daemon/store/bolt_test.go
package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/daemon/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBoltStore_DevnetCRUD(t *testing.T) {
	// Setup temp database
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	store, err := NewBoltStore(dbPath)
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()

	// Create
	devnet := &types.Devnet{
		Metadata: types.ResourceMeta{Name: "test-devnet"},
		Spec: types.DevnetSpec{
			Plugin:     "stable",
			Validators: 4,
			Mode:       "docker",
		},
	}

	err = store.CreateDevnet(ctx, devnet)
	require.NoError(t, err)
	assert.Equal(t, int64(1), devnet.Metadata.Generation)
	assert.False(t, devnet.Metadata.CreatedAt.IsZero())

	// Get
	got, err := store.GetDevnet(ctx, "test-devnet")
	require.NoError(t, err)
	assert.Equal(t, "test-devnet", got.Metadata.Name)
	assert.Equal(t, "stable", got.Spec.Plugin)

	// Update
	got.Spec.Validators = 8
	err = store.UpdateDevnet(ctx, got)
	require.NoError(t, err)
	assert.Equal(t, int64(2), got.Metadata.Generation)

	// Verify update
	updated, err := store.GetDevnet(ctx, "test-devnet")
	require.NoError(t, err)
	assert.Equal(t, 8, updated.Spec.Validators)

	// List
	list, err := store.ListDevnets(ctx)
	require.NoError(t, err)
	assert.Len(t, list, 1)

	// Delete
	err = store.DeleteDevnet(ctx, "test-devnet")
	require.NoError(t, err)

	// Verify delete
	_, err = store.GetDevnet(ctx, "test-devnet")
	assert.True(t, IsNotFound(err))
}

func TestBoltStore_ConflictDetection(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	store, err := NewBoltStore(dbPath)
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()

	// Create devnet
	devnet := &types.Devnet{
		Metadata: types.ResourceMeta{Name: "conflict-test"},
		Spec:     types.DevnetSpec{Plugin: "stable", Validators: 4},
	}
	err = store.CreateDevnet(ctx, devnet)
	require.NoError(t, err)

	// Get two copies
	copy1, _ := store.GetDevnet(ctx, "conflict-test")
	copy2, _ := store.GetDevnet(ctx, "conflict-test")

	// Update first copy
	copy1.Spec.Validators = 8
	err = store.UpdateDevnet(ctx, copy1)
	require.NoError(t, err)

	// Try to update second copy (should conflict)
	copy2.Spec.Validators = 16
	err = store.UpdateDevnet(ctx, copy2)
	assert.True(t, IsConflict(err))
}

func TestBoltStore_AlreadyExists(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	store, err := NewBoltStore(dbPath)
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()

	devnet := &types.Devnet{
		Metadata: types.ResourceMeta{Name: "duplicate"},
		Spec:     types.DevnetSpec{Plugin: "stable"},
	}

	err = store.CreateDevnet(ctx, devnet)
	require.NoError(t, err)

	// Try to create again
	err = store.CreateDevnet(ctx, devnet)
	assert.True(t, IsAlreadyExists(err))
}

func TestBoltStore_Watch(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	store, err := NewBoltStore(dbPath)
	require.NoError(t, err)
	defer store.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	events := make(chan string, 10)

	// Start watching in goroutine
	go func() {
		store.Watch(ctx, "devnets", func(eventType string, resource interface{}) {
			events <- eventType
		})
	}()

	// Give watcher time to start
	time.Sleep(100 * time.Millisecond)

	// Create devnet
	devnet := &types.Devnet{
		Metadata: types.ResourceMeta{Name: "watch-test"},
		Spec:     types.DevnetSpec{Plugin: "stable"},
	}
	err = store.CreateDevnet(ctx, devnet)
	require.NoError(t, err)

	// Wait for event
	select {
	case event := <-events:
		assert.Equal(t, "ADDED", event)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for watch event")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/daemon/store/... -v -run TestBoltStore`
Expected: FAIL (NewBoltStore not defined)

**Step 3: Add bbolt dependency**

```bash
go get go.etcd.io/bbolt@v1.4.0-alpha.1
```

**Step 4: Write bolt.go**

```go
// internal/daemon/store/bolt.go
package store

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	bolt "go.etcd.io/bbolt"
)

// Bucket names
var (
	bucketDevnets      = []byte("devnets")
	bucketNodes        = []byte("nodes")
	bucketUpgrades     = []byte("upgrades")
	bucketTransactions = []byte("transactions")
	bucketMeta         = []byte("meta")
)

// BoltStore implements Store using BoltDB.
type BoltStore struct {
	db       *bolt.DB
	watchers map[string][]WatchHandler
	mu       sync.RWMutex
}

// NewBoltStore creates a new BoltDB-backed store.
func NewBoltStore(path string) (*BoltStore, error) {
	db, err := bolt.Open(path, 0600, &bolt.Options{
		Timeout: 1 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Initialize buckets
	err = db.Update(func(tx *bolt.Tx) error {
		buckets := [][]byte{
			bucketDevnets,
			bucketNodes,
			bucketUpgrades,
			bucketTransactions,
			bucketMeta,
		}
		for _, bucket := range buckets {
			if _, err := tx.CreateBucketIfNotExists(bucket); err != nil {
				return fmt.Errorf("failed to create bucket %s: %w", bucket, err)
			}
		}
		return nil
	})
	if err != nil {
		db.Close()
		return nil, err
	}

	return &BoltStore{
		db:       db,
		watchers: make(map[string][]WatchHandler),
	}, nil
}

// Close closes the database.
func (s *BoltStore) Close() error {
	return s.db.Close()
}

// notify sends events to registered watchers.
func (s *BoltStore) notify(resourceType, eventType string, resource interface{}) {
	s.mu.RLock()
	handlers := s.watchers[resourceType]
	s.mu.RUnlock()

	for _, h := range handlers {
		go h(eventType, resource)
	}
}

// Watch registers a handler for resource changes.
func (s *BoltStore) Watch(ctx context.Context, resourceType string, handler WatchHandler) error {
	s.mu.Lock()
	s.watchers[resourceType] = append(s.watchers[resourceType], handler)
	s.mu.Unlock()

	// Send initial list as ADDED events
	switch resourceType {
	case "devnets":
		devnets, err := s.ListDevnets(ctx)
		if err != nil {
			return err
		}
		for _, d := range devnets {
			handler("ADDED", d)
		}
	}

	// Block until context is cancelled
	<-ctx.Done()
	return ctx.Err()
}

// encode marshals a value to JSON.
func encode(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

// decode unmarshals JSON to a value.
func decode(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}
```

**Step 5: Write bolt_devnet.go**

```go
// internal/daemon/store/bolt_devnet.go
package store

import (
	"context"
	"fmt"
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/daemon/types"
	bolt "go.etcd.io/bbolt"
)

// CreateDevnet creates a new devnet.
func (s *BoltStore) CreateDevnet(ctx context.Context, devnet *Devnet) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketDevnets)

		// Check if already exists
		if b.Get([]byte(devnet.Metadata.Name)) != nil {
			return &AlreadyExistsError{Resource: "devnet", Name: devnet.Metadata.Name}
		}

		// Set metadata
		now := time.Now()
		devnet.Metadata.Generation = 1
		devnet.Metadata.CreatedAt = now
		devnet.Metadata.UpdatedAt = now

		data, err := encode(devnet)
		if err != nil {
			return fmt.Errorf("failed to encode devnet: %w", err)
		}

		if err := b.Put([]byte(devnet.Metadata.Name), data); err != nil {
			return fmt.Errorf("failed to store devnet: %w", err)
		}

		s.notify("devnets", "ADDED", devnet)
		return nil
	})
}

// GetDevnet retrieves a devnet by name.
func (s *BoltStore) GetDevnet(ctx context.Context, name string) (*Devnet, error) {
	var devnet Devnet

	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketDevnets)
		data := b.Get([]byte(name))
		if data == nil {
			return &NotFoundError{Resource: "devnet", Name: name}
		}
		return decode(data, &devnet)
	})
	if err != nil {
		return nil, err
	}

	return &devnet, nil
}

// UpdateDevnet updates an existing devnet.
func (s *BoltStore) UpdateDevnet(ctx context.Context, devnet *Devnet) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketDevnets)

		// Get existing for conflict detection
		existing := b.Get([]byte(devnet.Metadata.Name))
		if existing == nil {
			return &NotFoundError{Resource: "devnet", Name: devnet.Metadata.Name}
		}

		var old types.Devnet
		if err := decode(existing, &old); err != nil {
			return fmt.Errorf("failed to decode existing devnet: %w", err)
		}

		// Optimistic concurrency check
		if old.Metadata.Generation != devnet.Metadata.Generation {
			return &ConflictError{
				Resource: "devnet",
				Name:     devnet.Metadata.Name,
				Message:  fmt.Sprintf("generation mismatch: expected %d, got %d", old.Metadata.Generation, devnet.Metadata.Generation),
			}
		}

		// Update metadata
		devnet.Metadata.Generation++
		devnet.Metadata.UpdatedAt = time.Now()

		data, err := encode(devnet)
		if err != nil {
			return fmt.Errorf("failed to encode devnet: %w", err)
		}

		if err := b.Put([]byte(devnet.Metadata.Name), data); err != nil {
			return fmt.Errorf("failed to store devnet: %w", err)
		}

		s.notify("devnets", "MODIFIED", devnet)
		return nil
	})
}

// DeleteDevnet deletes a devnet.
func (s *BoltStore) DeleteDevnet(ctx context.Context, name string) error {
	var devnet *Devnet

	err := s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketDevnets)

		data := b.Get([]byte(name))
		if data == nil {
			return &NotFoundError{Resource: "devnet", Name: name}
		}

		devnet = &Devnet{}
		if err := decode(data, devnet); err != nil {
			return err
		}

		return b.Delete([]byte(name))
	})
	if err != nil {
		return err
	}

	s.notify("devnets", "DELETED", devnet)
	return nil
}

// ListDevnets returns all devnets.
func (s *BoltStore) ListDevnets(ctx context.Context) ([]*Devnet, error) {
	var devnets []*Devnet

	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketDevnets)
		return b.ForEach(func(k, v []byte) error {
			var devnet Devnet
			if err := decode(v, &devnet); err != nil {
				return err
			}
			devnets = append(devnets, &devnet)
			return nil
		})
	})
	if err != nil {
		return nil, err
	}

	return devnets, nil
}

// Node operations (stub implementations for now)

func (s *BoltStore) CreateNode(ctx context.Context, node *Node) error {
	return fmt.Errorf("not implemented")
}

func (s *BoltStore) GetNode(ctx context.Context, devnetName string, index int) (*Node, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *BoltStore) UpdateNode(ctx context.Context, node *Node) error {
	return fmt.Errorf("not implemented")
}

func (s *BoltStore) DeleteNode(ctx context.Context, devnetName string, index int) error {
	return fmt.Errorf("not implemented")
}

func (s *BoltStore) ListNodes(ctx context.Context, devnetName string) ([]*Node, error) {
	return nil, fmt.Errorf("not implemented")
}

// Upgrade operations (stub implementations for now)

func (s *BoltStore) CreateUpgrade(ctx context.Context, upgrade *Upgrade) error {
	return fmt.Errorf("not implemented")
}

func (s *BoltStore) GetUpgrade(ctx context.Context, name string) (*Upgrade, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *BoltStore) UpdateUpgrade(ctx context.Context, upgrade *Upgrade) error {
	return fmt.Errorf("not implemented")
}

func (s *BoltStore) ListUpgrades(ctx context.Context, devnetName string) ([]*Upgrade, error) {
	return nil, fmt.Errorf("not implemented")
}

// Transaction operations (stub implementations for now)

func (s *BoltStore) CreateTransaction(ctx context.Context, tx *Transaction) error {
	return fmt.Errorf("not implemented")
}

func (s *BoltStore) GetTransaction(ctx context.Context, id string) (*Transaction, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *BoltStore) UpdateTransaction(ctx context.Context, tx *Transaction) error {
	return fmt.Errorf("not implemented")
}

func (s *BoltStore) ListTransactions(ctx context.Context, devnetName string, opts ListTxOptions) ([]*Transaction, error) {
	return nil, fmt.Errorf("not implemented")
}
```

**Step 6: Run tests**

Run: `go test ./internal/daemon/store/... -v`
Expected: PASS

**Step 7: Commit**

```bash
git add internal/daemon/store/
git commit -m "feat(daemon): implement BoltDB state store

Implement Store interface with BoltDB:
- Full Devnet CRUD with optimistic concurrency
- Watch support for real-time notifications
- Conflict detection via generation field
- Stub implementations for Node, Upgrade, Transaction"
```

---

## Task 5: dvb Client Skeleton with Daemon Detection

**Files:**
- Create: `cmd/dvb/main.go`
- Create: `internal/client/client.go`
- Create: `internal/client/detect.go`

**Step 1: Write client detection logic**

```go
// internal/client/detect.go
package client

import (
	"net"
	"os"
	"path/filepath"
	"time"
)

// DefaultSocketPath returns the default daemon socket path.
func DefaultSocketPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".devnet-builder", "devnetd.sock")
}

// IsDaemonRunning checks if the daemon is accessible.
func IsDaemonRunning() bool {
	return IsDaemonRunningAt(DefaultSocketPath())
}

// IsDaemonRunningAt checks if the daemon is accessible at the given socket path.
func IsDaemonRunningAt(socketPath string) bool {
	conn, err := net.DialTimeout("unix", socketPath, 500*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
```

**Step 2: Write client struct**

```go
// internal/client/client.go
package client

import (
	"fmt"
)

// Client provides access to the devnetd daemon.
type Client struct {
	socketPath string
	// gRPC clients will be added later
}

// New creates a new client connected to the daemon.
func New() (*Client, error) {
	return NewWithSocket(DefaultSocketPath())
}

// NewWithSocket creates a client with a specific socket path.
func NewWithSocket(socketPath string) (*Client, error) {
	if !IsDaemonRunningAt(socketPath) {
		return nil, fmt.Errorf("daemon not running at %s", socketPath)
	}

	return &Client{
		socketPath: socketPath,
	}, nil
}

// Close closes the client connection.
func (c *Client) Close() error {
	// Will close gRPC connection when implemented
	return nil
}
```

**Step 3: Write dvb main.go**

```go
// cmd/dvb/main.go
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/altuslabsxyz/devnet-builder/internal/client"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	standalone bool
	daemonClient *client.Client
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "dvb",
		Short: "Devnet Builder CLI",
		Long:  `dvb is a CLI for managing blockchain development networks.`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Skip daemon connection for daemon subcommand
			if cmd.Name() == "daemon" || cmd.Parent() != nil && cmd.Parent().Name() == "daemon" {
				return nil
			}

			// Skip if standalone mode
			if standalone {
				return nil
			}

			// Try to connect to daemon
			c, err := client.New()
			if err == nil {
				daemonClient = c
				return nil
			}

			// Daemon not running - fall back to standalone
			return nil
		},
		PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
			if daemonClient != nil {
				return daemonClient.Close()
			}
			return nil
		},
	}

	// Global flags
	rootCmd.PersistentFlags().BoolVar(&standalone, "standalone", false, "Force standalone mode (don't connect to daemon)")

	// Add commands
	rootCmd.AddCommand(
		newVersionCmd(),
		newDaemonCmd(),
		newStatusCmd(),
	)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("dvb version 0.1.0")
			if daemonClient != nil {
				fmt.Println("Mode: daemon")
			} else {
				fmt.Println("Mode: standalone")
			}
		},
	}
}

func newDaemonCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "daemon",
		Short: "Manage the devnetd daemon",
	}

	cmd.AddCommand(
		&cobra.Command{
			Use:   "status",
			Short: "Check daemon status",
			Run: func(cmd *cobra.Command, args []string) {
				if client.IsDaemonRunning() {
					color.Green("● Daemon is running")
					fmt.Printf("  Socket: %s\n", client.DefaultSocketPath())
				} else {
					color.Yellow("○ Daemon is not running")
					fmt.Println("  Start with: devnetd")
				}
			},
		},
	)

	return cmd
}

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status [devnet]",
		Short: "Show devnet status",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if daemonClient != nil {
				return statusViaDaemon(cmd.Context(), args)
			}
			return statusStandalone(cmd.Context(), args)
		},
	}
}

func statusViaDaemon(ctx context.Context, args []string) error {
	fmt.Println("Status via daemon (not implemented yet)")
	return nil
}

func statusStandalone(ctx context.Context, args []string) error {
	fmt.Println("Status in standalone mode (not implemented yet)")
	return nil
}
```

**Step 4: Build and test**

```bash
go build -o dvb ./cmd/dvb
./dvb version
./dvb daemon status
```

**Step 5: Commit**

```bash
git add cmd/dvb/ internal/client/
git commit -m "feat(dvb): add dvb CLI skeleton with daemon detection

Create dvb binary with:
- Automatic daemon detection via Unix socket
- Fallback to standalone mode if daemon not running
- --standalone flag to force standalone mode
- daemon status subcommand
- version command showing current mode"
```

---

## Task 6: devnetd Skeleton

**Files:**
- Create: `cmd/devnetd/main.go`
- Create: `internal/daemon/server/server.go`

**Step 1: Write server skeleton**

```go
// internal/daemon/server/server.go
package server

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/altuslabsxyz/devnet-builder/internal/daemon/store"
)

// Config holds server configuration.
type Config struct {
	// SocketPath is the Unix socket path.
	SocketPath string
	// DataDir is the data directory.
	DataDir string
	// Foreground runs in foreground (don't daemonize).
	Foreground bool
}

// DefaultConfig returns default configuration.
func DefaultConfig() *Config {
	home, _ := os.UserHomeDir()
	dataDir := filepath.Join(home, ".devnet-builder")
	return &Config{
		SocketPath: filepath.Join(dataDir, "devnetd.sock"),
		DataDir:    dataDir,
		Foreground: false,
	}
}

// Server is the devnetd daemon server.
type Server struct {
	config   *Config
	store    store.Store
	listener net.Listener
}

// New creates a new server.
func New(config *Config) (*Server, error) {
	// Ensure data directory exists
	if err := os.MkdirAll(config.DataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	// Open state store
	dbPath := filepath.Join(config.DataDir, "devnetd.db")
	st, err := store.NewBoltStore(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open state store: %w", err)
	}

	return &Server{
		config: config,
		store:  st,
	}, nil
}

// Run starts the server and blocks until shutdown.
func (s *Server) Run(ctx context.Context) error {
	// Remove stale socket
	os.Remove(s.config.SocketPath)

	// Create listener
	listener, err := net.Listen("unix", s.config.SocketPath)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}
	s.listener = listener

	// Write PID file
	pidPath := filepath.Join(s.config.DataDir, "devnetd.pid")
	if err := os.WriteFile(pidPath, []byte(fmt.Sprintf("%d", os.Getpid())), 0644); err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}
	defer os.Remove(pidPath)

	fmt.Printf("devnetd started\n")
	fmt.Printf("  Socket: %s\n", s.config.SocketPath)
	fmt.Printf("  Data: %s\n", s.config.DataDir)
	fmt.Printf("  PID: %d\n", os.Getpid())

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Wait for context cancellation or signal
	select {
	case <-ctx.Done():
	case sig := <-sigCh:
		fmt.Printf("\nReceived %s, shutting down...\n", sig)
	}

	return s.Shutdown()
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown() error {
	if s.listener != nil {
		s.listener.Close()
	}
	if s.store != nil {
		s.store.Close()
	}
	os.Remove(s.config.SocketPath)
	fmt.Println("devnetd stopped")
	return nil
}
```

**Step 2: Write devnetd main.go**

```go
// cmd/devnetd/main.go
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/altuslabsxyz/devnet-builder/internal/daemon/server"
	"github.com/spf13/cobra"
)

func main() {
	var config server.Config

	rootCmd := &cobra.Command{
		Use:   "devnetd",
		Short: "Devnet Builder Daemon",
		Long:  `devnetd is the daemon that manages blockchain development networks.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			srv, err := server.New(&config)
			if err != nil {
				return err
			}
			return srv.Run(context.Background())
		},
	}

	defaults := server.DefaultConfig()
	rootCmd.Flags().StringVar(&config.SocketPath, "socket", defaults.SocketPath, "Unix socket path")
	rootCmd.Flags().StringVar(&config.DataDir, "data-dir", defaults.DataDir, "Data directory")
	rootCmd.Flags().BoolVar(&config.Foreground, "foreground", true, "Run in foreground")

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
```

**Step 3: Build and test**

```bash
# Build devnetd
go build -o devnetd ./cmd/devnetd

# Start daemon in background
./devnetd &

# Check status with dvb
./dvb daemon status

# Stop daemon
kill $(cat ~/.devnet-builder/devnetd.pid)
```

**Step 4: Commit**

```bash
git add cmd/devnetd/ internal/daemon/server/
git commit -m "feat(devnetd): add devnetd daemon skeleton

Create devnetd binary with:
- Unix socket listener
- BoltDB state store initialization
- PID file management
- Graceful shutdown on SIGINT/SIGTERM
- Configurable socket path and data directory"
```

---

## Task 7: Copy Design Document to Worktree

**Step 1: Copy design document**

```bash
cp docs/plans/2026-01-23-daemon-refactor-design.md docs/plans/
git add docs/plans/2026-01-23-daemon-refactor-design.md
```

**Step 2: Commit**

```bash
git commit -m "docs: add daemon refactor design document"
```

---

## Summary

Phase 1 establishes the foundation:

| Component | Files | Purpose |
|-----------|-------|---------|
| Directory structure | `api/proto/v1/`, `internal/daemon/`, etc. | Organize new code |
| Resource types | `internal/daemon/types/*.go` | Define Devnet, Node, Upgrade, Transaction |
| Store interface | `internal/daemon/store/interface.go` | Abstract persistence |
| BoltDB store | `internal/daemon/store/bolt*.go` | Persistent state storage |
| dvb client | `cmd/dvb/`, `internal/client/` | CLI with daemon detection |
| devnetd daemon | `cmd/devnetd/`, `internal/daemon/server/` | Daemon process skeleton |

**Next Phase:** Phase 2 will add the Controller Manager, gRPC server, and basic controllers.
