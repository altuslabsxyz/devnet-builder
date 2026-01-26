// internal/daemon/types/health.go
package types

import "time"

// Condition types for devnet status
const (
	ConditionTypeReady           = "Ready"
	ConditionTypeHealthy         = "Healthy"
	ConditionTypeProgressing     = "Progressing"
	ConditionTypePluginAvailable = "PluginAvailable"
	ConditionTypeNodesCreated    = "NodesCreated"
	ConditionTypeNodesRunning    = "NodesRunning"
	ConditionTypeDegraded        = "Degraded"
)

// Condition status values
const (
	ConditionTrue    = "True"
	ConditionFalse   = "False"
	ConditionUnknown = "Unknown"
)

// Event types
const (
	EventTypeNormal  = "Normal"
	EventTypeWarning = "Warning"
)

// Condition reasons - CamelCase identifiers
const (
	// Progressing reasons
	ReasonProvisioning    = "Provisioning"
	ReasonCreatingNodes   = "CreatingNodes"
	ReasonStartingNodes   = "StartingNodes"
	ReasonWaitingForNodes = "WaitingForNodes"

	// Ready reasons
	ReasonAllNodesReady = "AllNodesReady"
	ReasonNodesNotReady = "NodesNotReady"

	// Degraded reasons
	ReasonNodesCrashed      = "NodesCrashed"
	ReasonHealthCheckFailed = "HealthCheckFailed"

	// Plugin reasons
	ReasonPluginFound    = "PluginFound"
	ReasonPluginNotFound = "PluginNotFound"

	// Error reasons
	ReasonImageNotFound       = "ImageNotFound"
	ReasonCredentialsNotFound = "CredentialsNotFound"
	ReasonModeNotSupported    = "ModeNotSupported"
	ReasonBinaryNotFound      = "BinaryNotFound"
	ReasonContainerFailed     = "ContainerFailed"
	ReasonNetworkError        = "NetworkError"
)

// RestartPolicy defines how crashed nodes should be restarted.
type RestartPolicy struct {
	// Enabled controls whether auto-restart is enabled.
	Enabled bool `json:"enabled"`

	// MaxRestarts is the maximum number of restarts before giving up.
	// 0 means unlimited.
	MaxRestarts int `json:"maxRestarts,omitempty"`

	// BackoffInitial is the initial backoff duration after a crash.
	BackoffInitial time.Duration `json:"backoffInitial,omitempty"`

	// BackoffMax is the maximum backoff duration.
	BackoffMax time.Duration `json:"backoffMax,omitempty"`

	// BackoffMultiplier multiplies the backoff after each restart.
	BackoffMultiplier float64 `json:"backoffMultiplier,omitempty"`
}

// DefaultRestartPolicy returns sensible defaults for restart policy.
func DefaultRestartPolicy() RestartPolicy {
	return RestartPolicy{
		Enabled:           true,
		MaxRestarts:       5,
		BackoffInitial:    5 * time.Second,
		BackoffMax:        5 * time.Minute,
		BackoffMultiplier: 2.0,
	}
}

// HealthCheckResult contains the result of a node health check.
type HealthCheckResult struct {
	// NodeKey is the node identifier (devnetName/index).
	NodeKey string `json:"nodeKey"`

	// Healthy indicates if the node is healthy.
	Healthy bool `json:"healthy"`

	// BlockHeight is the current block height.
	BlockHeight int64 `json:"blockHeight"`

	// PeerCount is the number of connected peers.
	PeerCount int `json:"peerCount"`

	// CatchingUp indicates if the node is syncing.
	CatchingUp bool `json:"catchingUp"`

	// Error is set if the health check failed.
	Error string `json:"error,omitempty"`

	// CheckedAt is when the check was performed.
	CheckedAt time.Time `json:"checkedAt"`
}

// HealthState tracks health state for a node over time.
type HealthState struct {
	// LastBlockHeight is the block height from last check.
	LastBlockHeight int64 `json:"lastBlockHeight"`

	// LastBlockTime is when we last saw a new block.
	LastBlockTime time.Time `json:"lastBlockTime"`

	// LastHealthyTime is when the node was last healthy.
	LastHealthyTime time.Time `json:"lastHealthyTime"`

	// ConsecutiveFailures counts consecutive health check failures.
	ConsecutiveFailures int `json:"consecutiveFailures"`

	// NextRestartTime is when the next restart attempt is allowed.
	NextRestartTime time.Time `json:"nextRestartTime,omitempty"`
}

// NodeHealthStatus combines current status with health state.
type NodeHealthStatus struct {
	// Current phase of the node.
	Phase string `json:"phase"`

	// HealthState tracks health over time.
	HealthState HealthState `json:"healthState"`

	// RestartCount from NodeStatus.
	RestartCount int `json:"restartCount"`
}

// StuckChainThreshold is the duration after which a chain is considered stuck
// if no new blocks are produced.
const StuckChainThreshold = 2 * time.Minute
