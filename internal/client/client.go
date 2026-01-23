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
