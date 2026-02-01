// internal/plugin/cosmos/runtime.go
package cosmos

import (
	"fmt"
	"syscall"
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/daemon/runtime"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/types"
)

// CosmosRuntime implements runtime.PluginRuntime for Cosmos SDK chains.
// It provides network-specific start commands, environment variables,
// and runtime configuration for running Cosmos nodes in containers.
type CosmosRuntime struct {
	binaryName string
}

// NewCosmosRuntime creates a new Cosmos runtime plugin.
func NewCosmosRuntime(binaryName string) *CosmosRuntime {
	return &CosmosRuntime{
		binaryName: binaryName,
	}
}

// StartCommand returns the command arguments to start the node.
// For Cosmos SDK chains, this is typically: ["start", "--home", homeDir]
func (r *CosmosRuntime) StartCommand(node *types.Node) []string {
	// Inside docker container, we use a standardized home path
	// The actual host path is mounted to this container path
	homeDir := r.ContainerHomePath()

	// Determine bind address: use node's assigned IP (loopback subnet) or 0.0.0.0 (docker/legacy)
	bindAddr := "0.0.0.0"
	if node.Spec.Address != "" {
		bindAddr = node.Spec.Address
	}

	return []string{
		"start",
		"--home", homeDir,
		// Enable API for health checks
		"--api.enable=true",
		fmt.Sprintf("--api.address=tcp://%s:1317", bindAddr),
		// Enable gRPC
		"--grpc.enable=true",
		fmt.Sprintf("--grpc.address=%s:9090", bindAddr),
		// Logging
		"--log_format=json",
	}
}

// StartEnv returns environment variables for the start command.
// These are passed to the container when starting the node.
func (r *CosmosRuntime) StartEnv(node *types.Node) map[string]string {
	return map[string]string{
		// Tendermint/CometBFT variables
		"TMHOME": r.ContainerHomePath(),
		// Disable colored output for log parsing
		"NO_COLOR": "1",
	}
}

// StopSignal returns the signal to use for graceful shutdown.
// SIGTERM is standard for graceful process termination.
func (r *CosmosRuntime) StopSignal() syscall.Signal {
	return syscall.SIGTERM
}

// GracePeriod returns how long to wait before SIGKILL.
// Cosmos nodes need time to flush state and close connections.
func (r *CosmosRuntime) GracePeriod() time.Duration {
	return 30 * time.Second
}

// HealthEndpoint returns the health check endpoint for the node.
// For Cosmos SDK chains, we use the RPC status endpoint.
func (r *CosmosRuntime) HealthEndpoint(node *types.Node) string {
	// Use node's Address if set (loopback subnet mode), otherwise fall back to localhost with offset
	if node.Spec.Address != "" {
		// Loopback subnet mode: use node's unique IP with standard RPC port
		return fmt.Sprintf("http://%s:26657/status", node.Spec.Address)
	}

	// Legacy mode: Calculate RPC port based on node index
	// Each node gets a 100-port range offset
	rpcPort := 26657 + (node.Spec.Index * 100)
	return fmt.Sprintf("http://localhost:%d/status", rpcPort)
}

// ContainerHomePath returns the standardized home path inside containers.
// This is where the host home directory is mounted.
// Implements runtime.PluginRuntime interface.
func (r *CosmosRuntime) ContainerHomePath() string {
	// Use a consistent path inside containers for all Cosmos SDK chains
	return fmt.Sprintf("/root/.%s", r.binaryName)
}

// BinaryName returns the binary name for reference.
func (r *CosmosRuntime) BinaryName() string {
	return r.binaryName
}

// Ensure CosmosRuntime implements runtime.PluginRuntime
var _ runtime.PluginRuntime = (*CosmosRuntime)(nil)
