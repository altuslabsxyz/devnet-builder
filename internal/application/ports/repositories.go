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
	NetworkName       string // e.g., "mainnet", "testnet"
	NetworkVersion    string // stable version
	BlockchainNetwork string // e.g., "stable", "ault"
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
	DockerConfig      *DockerConfigMetadata // Docker-specific configuration (nil if not Docker mode)
}

// DockerConfigMetadata contains Docker-specific metadata for devnet
type DockerConfigMetadata struct {
	NetworkID      string // Docker network ID
	NetworkName    string // Docker network name
	Subnet         string // Subnet CIDR
	PortRangeStart int    // Start of allocated port range
	PortRangeEnd   int    // End of allocated port range
	Image          string // Docker image used
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
// This is the canonical type for the application layer.
// Infrastructure adapters must convert to this type.
type PortConfig struct {
	RPC     int // Tendermint RPC (default: 26657)
	P2P     int // P2P networking (default: 26656)
	GRPC    int // gRPC server (default: 9090)
	GRPCWeb int // gRPC-Web (default: 9091)
	API     int // REST API (default: 1317)
	EVM     int // EVM JSON-RPC (default: 8545)
	EVMWS   int // EVM WebSocket (default: 8546)
	PProf   int // pprof debugging (default: 6060)
	Rosetta int // Rosetta API (default: 8080)
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

// CachedBinaryInfo contains detailed information about a cached binary.
type CachedBinaryInfo struct {
	Ref        string
	CommitHash string
	Path       string
	Size       int64
	BuildTime  time.Time
	Network    string
}

// CacheStats contains cache statistics.
type CacheStats struct {
	TotalEntries int
	TotalSize    int64
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

	// ListDetailed returns detailed information about all cached binaries.
	ListDetailed() []CachedBinaryInfo

	// Stats returns cache statistics.
	Stats() CacheStats

	// Remove deletes a cached binary.
	Remove(ref string) error

	// Clean removes all cached binaries.
	Clean() error

	// SetActive sets the active binary version.
	SetActive(ref string) error

	// GetActive returns the currently active binary path.
	GetActive() (string, error)

	// CacheDir returns the cache directory path.
	CacheDir() string

	// SymlinkPath returns the symlink path.
	SymlinkPath() string

	// SymlinkInfo returns information about the current symlink.
	SymlinkInfo() (*SymlinkInfo, error)
}

// SymlinkInfo contains information about the active binary symlink.
type SymlinkInfo struct {
	Path       string
	Target     string
	CommitHash string
	Exists     bool
	IsRegular  bool // true if it's a regular file instead of symlink
}

// NodeConfigOptions contains options for node configuration.
// This is passed to ConfigureNode to customize config.toml and app.toml.
type NodeConfigOptions struct {
	// ChainID is the chain identifier
	ChainID string

	// Ports is the port configuration for this node
	Ports PortConfig

	// PersistentPeers is the persistent peers string (node_id@host:port,...)
	PersistentPeers string

	// NumValidators is the total number of validators
	NumValidators int

	// IsValidator indicates if this node is a validator
	IsValidator bool

	// Moniker is the node's moniker/name
	Moniker string
}

// ExportRepository manages export persistence and querying
type ExportRepository interface {
	// Save saves export metadata to disk
	Save(ctx context.Context, exp interface{}) error

	// Load loads export from directory
	Load(ctx context.Context, exportPath string) (interface{}, error)

	// ListForDevnet lists all exports for a devnet
	ListForDevnet(ctx context.Context, devnetHomeDir string) (interface{}, error)

	// Delete removes an export directory
	Delete(ctx context.Context, exportPath string) error

	// Validate checks export completeness
	Validate(ctx context.Context, exportPath string) (interface{}, error)
}
