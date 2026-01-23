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
	Metadata ResourceMeta `json:"metadata"`
	Spec     DevnetSpec   `json:"spec"`
	Status   DevnetStatus `json:"status"`
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
