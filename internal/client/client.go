// internal/client/client.go
package client

import (
	"context"
	"fmt"

	v1 "github.com/altuslabsxyz/devnet-builder/api/proto/gen/v1"
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

// GetNode retrieves a node by devnet name and index.
func (c *Client) GetNode(ctx context.Context, devnetName string, index int) (*v1.Node, error) {
	return c.grpc.GetNode(ctx, devnetName, index)
}

// ListNodes lists all nodes in a devnet.
func (c *Client) ListNodes(ctx context.Context, devnetName string) ([]*v1.Node, error) {
	return c.grpc.ListNodes(ctx, devnetName)
}

// StartNode starts a stopped node.
func (c *Client) StartNode(ctx context.Context, devnetName string, index int) (*v1.Node, error) {
	return c.grpc.StartNode(ctx, devnetName, index)
}

// StopNode stops a running node.
func (c *Client) StopNode(ctx context.Context, devnetName string, index int) (*v1.Node, error) {
	return c.grpc.StopNode(ctx, devnetName, index)
}

// RestartNode restarts a node.
func (c *Client) RestartNode(ctx context.Context, devnetName string, index int) (*v1.Node, error) {
	return c.grpc.RestartNode(ctx, devnetName, index)
}

// CreateUpgrade creates a new upgrade.
func (c *Client) CreateUpgrade(ctx context.Context, name string, spec *v1.UpgradeSpec) (*v1.Upgrade, error) {
	return c.grpc.CreateUpgrade(ctx, name, spec)
}

// GetUpgrade retrieves an upgrade by name.
func (c *Client) GetUpgrade(ctx context.Context, name string) (*v1.Upgrade, error) {
	return c.grpc.GetUpgrade(ctx, name)
}

// ListUpgrades lists all upgrades for a devnet.
func (c *Client) ListUpgrades(ctx context.Context, devnetName string) ([]*v1.Upgrade, error) {
	return c.grpc.ListUpgrades(ctx, devnetName)
}

// DeleteUpgrade deletes an upgrade.
func (c *Client) DeleteUpgrade(ctx context.Context, name string) error {
	return c.grpc.DeleteUpgrade(ctx, name)
}

// CancelUpgrade cancels a running upgrade.
func (c *Client) CancelUpgrade(ctx context.Context, name string) (*v1.Upgrade, error) {
	return c.grpc.CancelUpgrade(ctx, name)
}

// RetryUpgrade retries a failed upgrade.
func (c *Client) RetryUpgrade(ctx context.Context, name string) (*v1.Upgrade, error) {
	return c.grpc.RetryUpgrade(ctx, name)
}
