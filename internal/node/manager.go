package node

import (
	"context"
	"time"
)

// NodeManager defines the interface for node lifecycle management.
// Both DockerManager and LocalManager implement this interface.
type NodeManager interface {
	// Start starts a node with the given genesis path.
	Start(ctx context.Context, node *Node, genesisPath string) error

	// Stop stops a running node with the given timeout.
	Stop(ctx context.Context, node *Node, timeout time.Duration) error

	// IsRunning checks if the node is currently running.
	IsRunning(ctx context.Context, node *Node) bool
}
