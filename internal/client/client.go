// internal/client/client.go
package client

import (
	"context"
	"fmt"

	v1 "github.com/altuslabsxyz/devnet-builder/api/proto/gen/v1"
)

// Client provides access to the devnetd daemon.
type Client struct {
	socketPath string // Unix socket path (for local connections)
	server     string // Remote server address (for remote connections)
	grpc       *GRPCClient
	isRemote   bool // true if connected to a remote server
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
		isRemote:   false,
	}, nil
}

// NewRemote creates a client connected to a remote devnetd server via TLS.
// Alias for NewRemoteClient.
func NewRemote(server, apiKey string) (*Client, error) {
	return NewRemoteClient(server, apiKey)
}

// NewRemoteClient creates a client connected to a remote devnetd server via TLS.
// The server should be in the format "host:port" (e.g., "devnetd.example.com:9000").
// The apiKey is used for authentication with the remote server.
func NewRemoteClient(server, apiKey string) (*Client, error) {
	if server == "" {
		return nil, fmt.Errorf("server address is required for remote connection")
	}

	grpcClient, err := NewRemoteGRPCClient(server, apiKey)
	if err != nil {
		return nil, err
	}

	return &Client{
		server:   server,
		grpc:     grpcClient,
		isRemote: true,
	}, nil
}

// IsRemote returns true if this client is connected to a remote server.
func (c *Client) IsRemote() bool {
	return c.isRemote
}

// Server returns the remote server address, or empty string for local connections.
func (c *Client) Server() string {
	return c.server
}

// SocketPath returns the Unix socket path, or empty string for remote connections.
func (c *Client) SocketPath() string {
	return c.socketPath
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
func (c *Client) ApplyDevnet(ctx context.Context, namespace, name string, spec *v1.DevnetSpec, labels, annotations map[string]string) (*v1.ApplyDevnetResponse, error) {
	return c.grpc.ApplyDevnet(ctx, namespace, name, spec, labels, annotations)
}

// UpdateDevnet updates an existing devnet.
func (c *Client) UpdateDevnet(ctx context.Context, namespace, name string, spec *v1.DevnetSpec, labels, annotations map[string]string) (*v1.Devnet, error) {
	return c.grpc.UpdateDevnet(ctx, namespace, name, spec, labels, annotations)
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

// ExecInNode executes a command inside a running node container.
func (c *Client) ExecInNode(ctx context.Context, devnetName string, index int, command []string, timeoutSeconds int) (*ExecResult, error) {
	return c.grpc.ExecInNode(ctx, devnetName, index, command, timeoutSeconds)
}

// GetNodePorts retrieves the port mappings for a node.
func (c *Client) GetNodePorts(ctx context.Context, devnetName string, index int) (*NodePorts, error) {
	return c.grpc.GetNodePorts(ctx, devnetName, index)
}

// GetNodeHealth retrieves the health status of a node.
func (c *Client) GetNodeHealth(ctx context.Context, devnetName string, index int) (*NodeHealth, error) {
	return c.grpc.GetNodeHealth(ctx, devnetName, index)
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

// StreamNodeLogs streams logs from a node, calling the callback for each log entry.
func (c *Client) StreamNodeLogs(ctx context.Context, devnetName string, index int, follow bool, since string, tail int, callback func(*LogEntry) error) error {
	return c.grpc.StreamNodeLogs(ctx, devnetName, index, follow, since, tail, callback)
}

// StreamProvisionLogs streams provisioner log entries for a devnet.
// The callback is called for each log entry received.
func (c *Client) StreamProvisionLogs(ctx context.Context, namespace, name string, callback func(*ProvisionLogEntry) error) error {
	return c.grpc.StreamProvisionLogs(ctx, namespace, name, callback)
}

// ListNetworks returns all registered network modules from the daemon.
func (c *Client) ListNetworks(ctx context.Context) ([]*v1.NetworkSummary, error) {
	return c.grpc.ListNetworks(ctx)
}

// GetNetworkInfo returns detailed information about a specific network module.
func (c *Client) GetNetworkInfo(ctx context.Context, name string) (*v1.NetworkInfo, error) {
	return c.grpc.GetNetworkInfo(ctx, name)
}

// ListBinaryVersions returns available binary versions for a network.
func (c *Client) ListBinaryVersions(ctx context.Context, networkName string, includePrerelease bool) (*v1.ListBinaryVersionsResponse, error) {
	return c.grpc.ListBinaryVersions(ctx, networkName, includePrerelease)
}

// Ping tests connectivity to the server.
func (c *Client) Ping(ctx context.Context) (*PingResponse, error) {
	return c.grpc.Ping(ctx)
}

// WhoAmI returns information about the authenticated user.
func (c *Client) WhoAmI(ctx context.Context) (*WhoAmIResponse, error) {
	return c.grpc.WhoAmI(ctx)
}
