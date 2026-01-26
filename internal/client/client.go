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
func (c *Client) CreateDevnet(ctx context.Context, namespace, name string, spec *v1.DevnetSpec, labels map[string]string) (*v1.Devnet, error) {
	return c.grpc.CreateDevnet(ctx, namespace, name, spec, labels)
}

// GetDevnet retrieves a devnet by name.
func (c *Client) GetDevnet(ctx context.Context, namespace, name string) (*v1.Devnet, error) {
	return c.grpc.GetDevnet(ctx, namespace, name)
}

// ListDevnets lists all devnets. Empty namespace returns all namespaces.
func (c *Client) ListDevnets(ctx context.Context, namespace string) ([]*v1.Devnet, error) {
	return c.grpc.ListDevnets(ctx, namespace)
}

// DeleteDevnet deletes a devnet.
func (c *Client) DeleteDevnet(ctx context.Context, namespace, name string) error {
	return c.grpc.DeleteDevnet(ctx, namespace, name)
}

// StartDevnet starts a stopped devnet.
func (c *Client) StartDevnet(ctx context.Context, namespace, name string) (*v1.Devnet, error) {
	return c.grpc.StartDevnet(ctx, namespace, name)
}

// StopDevnet stops a running devnet.
func (c *Client) StopDevnet(ctx context.Context, namespace, name string) (*v1.Devnet, error) {
	return c.grpc.StopDevnet(ctx, namespace, name)
}

// ApplyDevnet creates or updates a devnet (idempotent).
func (c *Client) ApplyDevnet(ctx context.Context, name string, spec *v1.DevnetSpec, labels, annotations map[string]string) (*v1.ApplyDevnetResponse, error) {
	return c.grpc.ApplyDevnet(ctx, name, spec, labels, annotations)
}

// UpdateDevnet updates an existing devnet.
func (c *Client) UpdateDevnet(ctx context.Context, name string, spec *v1.DevnetSpec, labels, annotations map[string]string) (*v1.Devnet, error) {
	return c.grpc.UpdateDevnet(ctx, name, spec, labels, annotations)
}

// GetNode retrieves a node by devnet name and index.
func (c *Client) GetNode(ctx context.Context, namespace, devnetName string, index int) (*v1.Node, error) {
	return c.grpc.GetNode(ctx, namespace, devnetName, index)
}

// ListNodes lists all nodes in a devnet.
func (c *Client) ListNodes(ctx context.Context, namespace, devnetName string) ([]*v1.Node, error) {
	return c.grpc.ListNodes(ctx, namespace, devnetName)
}

// StartNode starts a stopped node.
func (c *Client) StartNode(ctx context.Context, namespace, devnetName string, index int) (*v1.Node, error) {
	return c.grpc.StartNode(ctx, namespace, devnetName, index)
}

// StopNode stops a running node.
func (c *Client) StopNode(ctx context.Context, namespace, devnetName string, index int) (*v1.Node, error) {
	return c.grpc.StopNode(ctx, namespace, devnetName, index)
}

// RestartNode restarts a node.
func (c *Client) RestartNode(ctx context.Context, namespace, devnetName string, index int) (*v1.Node, error) {
	return c.grpc.RestartNode(ctx, namespace, devnetName, index)
}

// CreateUpgrade creates a new upgrade.
func (c *Client) CreateUpgrade(ctx context.Context, namespace, name string, spec *v1.UpgradeSpec) (*v1.Upgrade, error) {
	return c.grpc.CreateUpgrade(ctx, namespace, name, spec)
}

// GetUpgrade retrieves an upgrade by name.
func (c *Client) GetUpgrade(ctx context.Context, namespace, name string) (*v1.Upgrade, error) {
	return c.grpc.GetUpgrade(ctx, namespace, name)
}

// ListUpgrades lists all upgrades. Empty namespace returns all namespaces.
func (c *Client) ListUpgrades(ctx context.Context, namespace string) ([]*v1.Upgrade, error) {
	return c.grpc.ListUpgrades(ctx, namespace)
}

// DeleteUpgrade deletes an upgrade.
func (c *Client) DeleteUpgrade(ctx context.Context, namespace, name string) error {
	return c.grpc.DeleteUpgrade(ctx, namespace, name)
}

// CancelUpgrade cancels a running upgrade.
func (c *Client) CancelUpgrade(ctx context.Context, namespace, name string) (*v1.Upgrade, error) {
	return c.grpc.CancelUpgrade(ctx, namespace, name)
}

// RetryUpgrade retries a failed upgrade.
func (c *Client) RetryUpgrade(ctx context.Context, namespace, name string) (*v1.Upgrade, error) {
	return c.grpc.RetryUpgrade(ctx, namespace, name)
}

// SubmitTransaction submits a new transaction.
func (c *Client) SubmitTransaction(ctx context.Context, devnet, txType, signer string, payload []byte) (*v1.Transaction, error) {
	return c.grpc.SubmitTransaction(ctx, devnet, txType, signer, payload)
}

// GetTransaction retrieves a transaction by name.
func (c *Client) GetTransaction(ctx context.Context, name string) (*v1.Transaction, error) {
	return c.grpc.GetTransaction(ctx, name)
}

// ListTransactions lists transactions for a devnet.
func (c *Client) ListTransactions(ctx context.Context, devnet string, txType, phase string, limit int) ([]*v1.Transaction, error) {
	return c.grpc.ListTransactions(ctx, devnet, txType, phase, limit)
}

// CancelTransaction cancels a pending transaction.
func (c *Client) CancelTransaction(ctx context.Context, name string) (*v1.Transaction, error) {
	return c.grpc.CancelTransaction(ctx, name)
}

// SubmitGovVote submits a governance vote.
func (c *Client) SubmitGovVote(ctx context.Context, devnet string, proposalID uint64, voter, option string) (*v1.Transaction, error) {
	return c.grpc.SubmitGovVote(ctx, devnet, proposalID, voter, option)
}

// SubmitGovProposal submits a governance proposal.
func (c *Client) SubmitGovProposal(ctx context.Context, devnet, proposer, proposalType, title, description string, content []byte) (*v1.Transaction, error) {
	return c.grpc.SubmitGovProposal(ctx, devnet, proposer, proposalType, title, description, content)
}
