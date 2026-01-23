// internal/client/client.go
package client

import (
	"context"
	"fmt"

	v1 "github.com/altuslabsxyz/devnet-builder/api/proto/v1"
)

// Client provides access to the devnetd daemon.
type Client struct {
	socketPath string
	grpc       *GRPCClient
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

	grpcClient, err := NewGRPCClient(socketPath)
	if err != nil {
		return nil, err
	}

	return &Client{
		socketPath: socketPath,
		grpc:       grpcClient,
	}, nil
}

// Close closes the client connection.
func (c *Client) Close() error {
	if c.grpc != nil {
		return c.grpc.Close()
	}
	return nil
}

// CreateDevnet creates a new devnet.
func (c *Client) CreateDevnet(ctx context.Context, name string, spec *v1.DevnetSpec, labels map[string]string) (*v1.Devnet, error) {
	return c.grpc.CreateDevnet(ctx, name, spec, labels)
}

// GetDevnet retrieves a devnet by name.
func (c *Client) GetDevnet(ctx context.Context, name string) (*v1.Devnet, error) {
	return c.grpc.GetDevnet(ctx, name)
}

// ListDevnets lists all devnets.
func (c *Client) ListDevnets(ctx context.Context) ([]*v1.Devnet, error) {
	return c.grpc.ListDevnets(ctx)
}

// DeleteDevnet deletes a devnet.
func (c *Client) DeleteDevnet(ctx context.Context, name string) error {
	return c.grpc.DeleteDevnet(ctx, name)
}

// StartDevnet starts a stopped devnet.
func (c *Client) StartDevnet(ctx context.Context, name string) (*v1.Devnet, error) {
	return c.grpc.StartDevnet(ctx, name)
}

// StopDevnet stops a running devnet.
func (c *Client) StopDevnet(ctx context.Context, name string) (*v1.Devnet, error) {
	return c.grpc.StopDevnet(ctx, name)
}
