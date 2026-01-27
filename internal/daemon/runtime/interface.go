// internal/daemon/runtime/interface.go
package runtime

import (
	"context"
	"io"
	"syscall"
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/daemon/types"
)

// NodeStatus represents the status of a running node
type NodeStatus struct {
	Running   bool
	PID       int
	StartedAt time.Time
	ExitCode  int       // last exit code if not running
	Restarts  int       // number of restarts
	LastError string    // last error message
	UpdatedAt time.Time
}

// StartOptions contains options for starting a node
type StartOptions struct {
	RestartPolicy RestartPolicy
	Env           map[string]string // additional environment variables
	PluginRuntime PluginRuntime     // network-specific runtime (overrides DockerConfig.PluginRuntime if set)
}

// RestartPolicy defines how to handle process restarts
type RestartPolicy struct {
	Policy         string        // "always", "on-failure", "never"
	MaxRestarts    int           // 0 = unlimited
	BackoffInitial time.Duration // default: 1s
	BackoffMax     time.Duration // default: 60s
	BackoffFactor  float64       // default: 2.0
	ResetAfter     time.Duration // reset restart count after healthy for this long
}

// DefaultRestartPolicy returns the default restart policy
func DefaultRestartPolicy() RestartPolicy {
	return RestartPolicy{
		Policy:         "on-failure",
		MaxRestarts:    5,
		BackoffInitial: 1 * time.Second,
		BackoffMax:     60 * time.Second,
		BackoffFactor:  2.0,
		ResetAfter:     5 * time.Minute,
	}
}

// LogOptions contains options for retrieving logs
type LogOptions struct {
	Follow bool      // tail -f behavior
	Lines  int       // last N lines (0 = all)
	Since  time.Time // logs after this time
}

// ExecResult contains the result of executing a command in a node
type ExecResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
}

// NodeRuntime manages node processes/containers
type NodeRuntime interface {
	// StartNode starts a node
	StartNode(ctx context.Context, node *types.Node, opts StartOptions) error

	// StopNode stops a node (graceful if true, force if false)
	StopNode(ctx context.Context, nodeID string, graceful bool) error

	// RestartNode restarts a node
	RestartNode(ctx context.Context, nodeID string) error

	// GetNodeStatus returns the current status of a node
	GetNodeStatus(ctx context.Context, nodeID string) (*NodeStatus, error)

	// GetLogs returns logs for a node
	GetLogs(ctx context.Context, nodeID string, opts LogOptions) (io.ReadCloser, error)

	// ExecInNode executes a command in a running node and returns the result
	ExecInNode(ctx context.Context, nodeID string, command []string, timeout time.Duration) (*ExecResult, error)

	// Cleanup cleans up all resources
	Cleanup(ctx context.Context) error
}

// PluginRuntime provides runtime commands for a specific network
type PluginRuntime interface {
	// StartCommand returns the command arguments to start the node
	StartCommand(node *types.Node) []string

	// StartEnv returns environment variables for the start command
	StartEnv(node *types.Node) map[string]string

	// StopSignal returns the signal to use for graceful shutdown
	StopSignal() syscall.Signal

	// GracePeriod returns how long to wait before SIGKILL
	GracePeriod() time.Duration

	// HealthEndpoint returns the health check endpoint
	HealthEndpoint(node *types.Node) string

	// ContainerHomePath returns the standardized home path inside containers.
	// This is where host node data directories should be mounted.
	ContainerHomePath() string
}
