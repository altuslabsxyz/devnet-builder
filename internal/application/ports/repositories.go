package ports

import (
	"context"
	"time"
)

// ExecutionMode represents how the devnet nodes are executed.
type ExecutionMode string

const (
	ModeLocal  ExecutionMode = "local"
	ModeDocker ExecutionMode = "docker"
)

// DevnetState represents the current state of the devnet.
type DevnetState string

const (
	StateCreated     DevnetState = "created"
	StateProvisioned DevnetState = "provisioned"
	StateRunning     DevnetState = "running"
	StateStopped     DevnetState = "stopped"
	StateFailed      DevnetState = "failed"
)

// DevnetMetadata represents the devnet configuration and state.
type DevnetMetadata struct {
	HomeDir           string
	ChainID           string
	NetworkName       string        // e.g., "mainnet", "testnet"
	NetworkVersion    string        // stable version
	BlockchainNetwork string        // e.g., "stable", "ault"
	NumValidators     int
	NumAccounts       int
	ExecutionMode     ExecutionMode
	Status            DevnetState
	DockerImage       string
	CustomBinaryPath  string
	GenesisPath       string
	InitialVersion    string
	CurrentVersion    string
	BinaryName        string
	CreatedAt         time.Time
	LastProvisioned   *time.Time
	LastStarted       *time.Time
	LastStopped       *time.Time
}

// NodeMetadata represents a single node's configuration.
type NodeMetadata struct {
	Index       int
	Name        string
	HomeDir     string
	ChainID     string
	NodeID      string
	PID         *int
	ContainerID string
	Ports       PortConfig
}

// PortConfig holds port assignments for a node.
type PortConfig struct {
	RPC     int
	P2P     int
	GRPC    int
	API     int
	EVM     int
	EVMWS   int
	PProf   int
	Rosetta int
}

// DevnetRepository defines operations for persisting devnet state.
type DevnetRepository interface {
	// Save persists the devnet metadata to storage.
	Save(ctx context.Context, metadata *DevnetMetadata) error

	// Load retrieves devnet metadata from storage.
	Load(ctx context.Context, homeDir string) (*DevnetMetadata, error)

	// Delete removes all devnet data from storage.
	Delete(ctx context.Context, homeDir string) error

	// Exists checks if a devnet exists at the given path.
	Exists(homeDir string) bool
}

// NodeRepository defines operations for persisting node state.
type NodeRepository interface {
	// Save persists a node's metadata to storage.
	Save(ctx context.Context, node *NodeMetadata) error

	// Load retrieves a node's metadata from storage.
	Load(ctx context.Context, homeDir string, index int) (*NodeMetadata, error)

	// LoadAll retrieves all nodes for a devnet.
	LoadAll(ctx context.Context, homeDir string) ([]*NodeMetadata, error)

	// Delete removes a node's data from storage.
	Delete(ctx context.Context, homeDir string, index int) error
}

// BinaryCache defines operations for caching built binaries.
type BinaryCache interface {
	// Store saves a binary to the cache.
	Store(ctx context.Context, ref string, binaryPath string) (string, error)

	// Get retrieves a cached binary path by ref.
	Get(ref string) (string, bool)

	// Has checks if a binary is cached.
	Has(ref string) bool

	// List returns all cached binary refs.
	List() []string

	// Remove deletes a cached binary.
	Remove(ref string) error

	// SetActive sets the active binary version.
	SetActive(ref string) error

	// GetActive returns the currently active binary path.
	GetActive() (string, error)
}
