// internal/client/grpc.go
package client

import (
	"context"
	"fmt"
	"io"
	"time"

	v1 "github.com/altuslabsxyz/devnet-builder/api/proto/gen/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// GRPCClient wraps the gRPC DevnetServiceClient, NodeServiceClient, UpgradeServiceClient, TransactionServiceClient, and NetworkServiceClient.
type GRPCClient struct {
	conn        *grpc.ClientConn
	devnet      v1.DevnetServiceClient
	node        v1.NodeServiceClient
	upgrade     v1.UpgradeServiceClient
	transaction v1.TransactionServiceClient
	network     v1.NetworkServiceClient
}

// NewGRPCClient creates a new gRPC client connected to the daemon.
func NewGRPCClient(socketPath string) (*GRPCClient, error) {
	// Connect via Unix socket
	target := "unix://" + socketPath
	conn, err := grpc.NewClient(target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to daemon: %w", err)
	}

	return &GRPCClient{
		conn:        conn,
		devnet:      v1.NewDevnetServiceClient(conn),
		node:        v1.NewNodeServiceClient(conn),
		upgrade:     v1.NewUpgradeServiceClient(conn),
		transaction: v1.NewTransactionServiceClient(conn),
		network:     v1.NewNetworkServiceClient(conn),
	}, nil
}

// Close closes the gRPC connection.
func (c *GRPCClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// CreateDevnet creates a new devnet.
func (c *GRPCClient) CreateDevnet(ctx context.Context, namespace, name string, spec *v1.DevnetSpec, labels map[string]string) (*v1.Devnet, error) {
	req := &v1.CreateDevnetRequest{
		Namespace: namespace,
		Name:      name,
		Spec:      spec,
		Labels:    labels,
	}

	resp, err := c.devnet.CreateDevnet(ctx, req)
	if err != nil {
		return nil, wrapGRPCError(err)
	}

	return resp.Devnet, nil
}

// GetDevnet retrieves a devnet by name.
func (c *GRPCClient) GetDevnet(ctx context.Context, namespace, name string) (*v1.Devnet, error) {
	resp, err := c.devnet.GetDevnet(ctx, &v1.GetDevnetRequest{
		Namespace: namespace,
		Name:      name,
	})
	if err != nil {
		return nil, wrapGRPCError(err)
	}
	return resp.Devnet, nil
}

// ListDevnets lists all devnets. Empty namespace returns all namespaces.
func (c *GRPCClient) ListDevnets(ctx context.Context, namespace string) ([]*v1.Devnet, error) {
	resp, err := c.devnet.ListDevnets(ctx, &v1.ListDevnetsRequest{
		Namespace: namespace,
	})
	if err != nil {
		return nil, wrapGRPCError(err)
	}
	return resp.Devnets, nil
}

// DeleteDevnet deletes a devnet.
func (c *GRPCClient) DeleteDevnet(ctx context.Context, namespace, name string) error {
	_, err := c.devnet.DeleteDevnet(ctx, &v1.DeleteDevnetRequest{
		Namespace: namespace,
		Name:      name,
	})
	if err != nil {
		return wrapGRPCError(err)
	}
	return nil
}

// StartDevnet starts a stopped devnet.
func (c *GRPCClient) StartDevnet(ctx context.Context, namespace, name string) (*v1.Devnet, error) {
	resp, err := c.devnet.StartDevnet(ctx, &v1.StartDevnetRequest{
		Namespace: namespace,
		Name:      name,
	})
	if err != nil {
		return nil, wrapGRPCError(err)
	}
	return resp.Devnet, nil
}

// StopDevnet stops a running devnet.
func (c *GRPCClient) StopDevnet(ctx context.Context, namespace, name string) (*v1.Devnet, error) {
	resp, err := c.devnet.StopDevnet(ctx, &v1.StopDevnetRequest{
		Namespace: namespace,
		Name:      name,
	})
	if err != nil {
		return nil, wrapGRPCError(err)
	}
	return resp.Devnet, nil
}

// ApplyDevnet creates or updates a devnet.
func (c *GRPCClient) ApplyDevnet(ctx context.Context, namespace, name string, spec *v1.DevnetSpec, labels, annotations map[string]string) (*v1.ApplyDevnetResponse, error) {
	resp, err := c.devnet.ApplyDevnet(ctx, &v1.ApplyDevnetRequest{
		Namespace:   namespace,
		Name:        name,
		Spec:        spec,
		Labels:      labels,
		Annotations: annotations,
	})
	if err != nil {
		return nil, fmt.Errorf("apply devnet: %w", err)
	}
	return resp, nil
}

// UpdateDevnet updates an existing devnet.
func (c *GRPCClient) UpdateDevnet(ctx context.Context, namespace, name string, spec *v1.DevnetSpec, labels, annotations map[string]string) (*v1.Devnet, error) {
	resp, err := c.devnet.UpdateDevnet(ctx, &v1.UpdateDevnetRequest{
		Namespace:   namespace,
		Name:        name,
		Spec:        spec,
		Labels:      labels,
		Annotations: annotations,
	})
	if err != nil {
		return nil, fmt.Errorf("update devnet: %w", err)
	}
	return resp.Devnet, nil
}

// GetNode retrieves a node by devnet name and index.
func (c *GRPCClient) GetNode(ctx context.Context, namespace, devnetName string, index int) (*v1.Node, error) {
	resp, err := c.node.GetNode(ctx, &v1.GetNodeRequest{
		Namespace:  namespace,
		DevnetName: devnetName,
		Index:      int32(index),
	})
	if err != nil {
		return nil, wrapGRPCError(err)
	}
	return resp.Node, nil
}

// ListNodes lists all nodes in a devnet.
func (c *GRPCClient) ListNodes(ctx context.Context, namespace, devnetName string) ([]*v1.Node, error) {
	resp, err := c.node.ListNodes(ctx, &v1.ListNodesRequest{
		Namespace:  namespace,
		DevnetName: devnetName,
	})
	if err != nil {
		return nil, wrapGRPCError(err)
	}
	return resp.Nodes, nil
}

// StartNode starts a stopped node.
func (c *GRPCClient) StartNode(ctx context.Context, namespace, devnetName string, index int) (*v1.Node, error) {
	resp, err := c.node.StartNode(ctx, &v1.StartNodeRequest{
		Namespace:  namespace,
		DevnetName: devnetName,
		Index:      int32(index),
	})
	if err != nil {
		return nil, wrapGRPCError(err)
	}
	return resp.Node, nil
}

// StopNode stops a running node.
func (c *GRPCClient) StopNode(ctx context.Context, namespace, devnetName string, index int) (*v1.Node, error) {
	resp, err := c.node.StopNode(ctx, &v1.StopNodeRequest{
		Namespace:  namespace,
		DevnetName: devnetName,
		Index:      int32(index),
	})
	if err != nil {
		return nil, wrapGRPCError(err)
	}
	return resp.Node, nil
}

// RestartNode restarts a node.
func (c *GRPCClient) RestartNode(ctx context.Context, namespace, devnetName string, index int) (*v1.Node, error) {
	resp, err := c.node.RestartNode(ctx, &v1.RestartNodeRequest{
		Namespace:  namespace,
		DevnetName: devnetName,
		Index:      int32(index),
	})
	if err != nil {
		return nil, wrapGRPCError(err)
	}
	return resp.Node, nil
}

// ExecResult contains the result of executing a command in a node.
type ExecResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
}

// ExecInNode executes a command inside a running node container.
func (c *GRPCClient) ExecInNode(ctx context.Context, devnetName string, index int, command []string, timeoutSeconds int) (*ExecResult, error) {
	resp, err := c.node.ExecInNode(ctx, &v1.ExecInNodeRequest{
		DevnetName:     devnetName,
		Index:          int32(index),
		Command:        command,
		TimeoutSeconds: int32(timeoutSeconds),
	})
	if err != nil {
		return nil, wrapGRPCError(err)
	}
	return &ExecResult{
		ExitCode: int(resp.ExitCode),
		Stdout:   resp.Stdout,
		Stderr:   resp.Stderr,
	}, nil
}

// NodeHealth contains the health status of a node.
type NodeHealth struct {
	Status              string    // "Healthy", "Unhealthy", "Stopped", "Transitioning", "Unknown"
	Message             string    // Human-readable health message
	LastCheck           time.Time // Last health check timestamp
	ConsecutiveFailures int       // Number of consecutive health check failures
}

// GetNodeHealth retrieves the health status of a node.
func (c *GRPCClient) GetNodeHealth(ctx context.Context, devnetName string, index int) (*NodeHealth, error) {
	resp, err := c.node.GetNodeHealth(ctx, &v1.GetNodeHealthRequest{
		DevnetName: devnetName,
		Index:      int32(index),
	})
	if err != nil {
		return nil, wrapGRPCError(err)
	}

	health := &NodeHealth{
		Status:  resp.Health.Status,
		Message: resp.Health.Message,
	}
	if resp.Health.LastCheck != nil {
		health.LastCheck = resp.Health.LastCheck.AsTime()
	}
	health.ConsecutiveFailures = int(resp.Health.ConsecutiveFailures)

	return health, nil
}

// PortInfo describes a port mapping for a node.
type PortInfo struct {
	Name          string // Service name: "p2p", "rpc", "rest", "grpc"
	ContainerPort int    // Port inside container
	HostPort      int    // Port on host
	Protocol      string // "tcp" or "udp"
}

// NodePorts contains port mappings for a node.
type NodePorts struct {
	DevnetName string
	Index      int
	Ports      []PortInfo
}

// GetNodePorts retrieves the port mappings for a node.
func (c *GRPCClient) GetNodePorts(ctx context.Context, devnetName string, index int) (*NodePorts, error) {
	resp, err := c.node.GetNodePorts(ctx, &v1.GetNodePortsRequest{
		DevnetName: devnetName,
		Index:      int32(index),
	})
	if err != nil {
		return nil, wrapGRPCError(err)
	}

	ports := make([]PortInfo, len(resp.Ports))
	for i, p := range resp.Ports {
		ports[i] = PortInfo{
			Name:          p.Name,
			ContainerPort: int(p.ContainerPort),
			HostPort:      int(p.HostPort),
			Protocol:      p.Protocol,
		}
	}

	return &NodePorts{
		DevnetName: resp.DevnetName,
		Index:      int(resp.Index),
		Ports:      ports,
	}, nil
}

// CreateUpgrade creates a new upgrade.
func (c *GRPCClient) CreateUpgrade(ctx context.Context, namespace, name string, spec *v1.UpgradeSpec) (*v1.Upgrade, error) {
	req := &v1.CreateUpgradeRequest{
		Namespace: namespace,
		Name:      name,
		Spec:      spec,
	}

	resp, err := c.upgrade.CreateUpgrade(ctx, req)
	if err != nil {
		return nil, wrapGRPCError(err)
	}

	return resp.Upgrade, nil
}

// GetUpgrade retrieves an upgrade by name.
func (c *GRPCClient) GetUpgrade(ctx context.Context, namespace, name string) (*v1.Upgrade, error) {
	resp, err := c.upgrade.GetUpgrade(ctx, &v1.GetUpgradeRequest{
		Namespace: namespace,
		Name:      name,
	})
	if err != nil {
		return nil, wrapGRPCError(err)
	}
	return resp.Upgrade, nil
}

// ListUpgrades lists all upgrades. Empty namespace returns all namespaces.
func (c *GRPCClient) ListUpgrades(ctx context.Context, namespace string) ([]*v1.Upgrade, error) {
	resp, err := c.upgrade.ListUpgrades(ctx, &v1.ListUpgradesRequest{
		Namespace: namespace,
	})
	if err != nil {
		return nil, wrapGRPCError(err)
	}
	return resp.Upgrades, nil
}

// DeleteUpgrade deletes an upgrade.
func (c *GRPCClient) DeleteUpgrade(ctx context.Context, namespace, name string) error {
	_, err := c.upgrade.DeleteUpgrade(ctx, &v1.DeleteUpgradeRequest{
		Namespace: namespace,
		Name:      name,
	})
	if err != nil {
		return wrapGRPCError(err)
	}
	return nil
}

// CancelUpgrade cancels a running upgrade.
func (c *GRPCClient) CancelUpgrade(ctx context.Context, namespace, name string) (*v1.Upgrade, error) {
	resp, err := c.upgrade.CancelUpgrade(ctx, &v1.CancelUpgradeRequest{
		Namespace: namespace,
		Name:      name,
	})
	if err != nil {
		return nil, wrapGRPCError(err)
	}
	return resp.Upgrade, nil
}

// RetryUpgrade retries a failed upgrade.
func (c *GRPCClient) RetryUpgrade(ctx context.Context, namespace, name string) (*v1.Upgrade, error) {
	resp, err := c.upgrade.RetryUpgrade(ctx, &v1.RetryUpgradeRequest{
		Namespace: namespace,
		Name:      name,
	})
	if err != nil {
		return nil, wrapGRPCError(err)
	}
	return resp.Upgrade, nil
}

// SubmitTransaction submits a new transaction.
func (c *GRPCClient) SubmitTransaction(ctx context.Context, devnet, txType, signer string, payload []byte) (*v1.Transaction, error) {
	resp, err := c.transaction.SubmitTransaction(ctx, &v1.SubmitTransactionRequest{
		Devnet:  devnet,
		TxType:  txType,
		Signer:  signer,
		Payload: payload,
	})
	if err != nil {
		return nil, wrapGRPCError(err)
	}
	return resp.Transaction, nil
}

// GetTransaction retrieves a transaction by name.
func (c *GRPCClient) GetTransaction(ctx context.Context, name string) (*v1.Transaction, error) {
	resp, err := c.transaction.GetTransaction(ctx, &v1.GetTransactionRequest{Name: name})
	if err != nil {
		return nil, wrapGRPCError(err)
	}
	return resp.Transaction, nil
}

// ListTransactions lists transactions for a devnet.
func (c *GRPCClient) ListTransactions(ctx context.Context, devnet string, txType, phase string, limit int) ([]*v1.Transaction, error) {
	resp, err := c.transaction.ListTransactions(ctx, &v1.ListTransactionsRequest{
		Devnet: devnet,
		TxType: txType,
		Phase:  phase,
		Limit:  int32(limit),
	})
	if err != nil {
		return nil, wrapGRPCError(err)
	}
	return resp.Transactions, nil
}

// CancelTransaction cancels a pending transaction.
func (c *GRPCClient) CancelTransaction(ctx context.Context, name string) (*v1.Transaction, error) {
	resp, err := c.transaction.CancelTransaction(ctx, &v1.CancelTransactionRequest{Name: name})
	if err != nil {
		return nil, wrapGRPCError(err)
	}
	return resp.Transaction, nil
}

// SubmitGovVote submits a governance vote.
func (c *GRPCClient) SubmitGovVote(ctx context.Context, devnet string, proposalID uint64, voter, option string) (*v1.Transaction, error) {
	resp, err := c.transaction.SubmitGovVote(ctx, &v1.SubmitGovVoteRequest{
		Devnet:     devnet,
		ProposalId: proposalID,
		Voter:      voter,
		VoteOption: option,
	})
	if err != nil {
		return nil, wrapGRPCError(err)
	}
	return resp.Transaction, nil
}

// SubmitGovProposal submits a governance proposal.
func (c *GRPCClient) SubmitGovProposal(ctx context.Context, devnet, proposer, proposalType, title, description string, content []byte) (*v1.Transaction, error) {
	resp, err := c.transaction.SubmitGovProposal(ctx, &v1.SubmitGovProposalRequest{
		Devnet:       devnet,
		Proposer:     proposer,
		ProposalType: proposalType,
		Title:        title,
		Description:  description,
		Content:      content,
	})
	if err != nil {
		return nil, wrapGRPCError(err)
	}
	return resp.Transaction, nil
}

// ListNetworks returns all registered network modules.
func (c *GRPCClient) ListNetworks(ctx context.Context) ([]*v1.NetworkSummary, error) {
	resp, err := c.network.ListNetworks(ctx, &v1.ListNetworksRequest{})
	if err != nil {
		return nil, wrapGRPCError(err)
	}
	return resp.Networks, nil
}

// GetNetworkInfo returns detailed information about a specific network module.
func (c *GRPCClient) GetNetworkInfo(ctx context.Context, name string) (*v1.NetworkInfo, error) {
	resp, err := c.network.GetNetworkInfo(ctx, &v1.GetNetworkInfoRequest{
		Name: name,
	})
	if err != nil {
		return nil, wrapGRPCError(err)
	}
	return resp.Network, nil
}

// LogEntry represents a single log line from a node.
type LogEntry struct {
	Timestamp time.Time
	Stream    string // "stdout" or "stderr"
	Message   string
}

// StreamNodeLogs streams logs from a node, calling the callback for each log entry.
// The callback should return an error to stop streaming.
func (c *GRPCClient) StreamNodeLogs(ctx context.Context, devnetName string, index int, follow bool, since string, tail int, callback func(*LogEntry) error) error {
	req := &v1.StreamNodeLogsRequest{
		DevnetName: devnetName,
		Index:      int32(index),
		Follow:     follow,
		Since:      since,
		Tail:       int32(tail),
	}

	stream, err := c.node.StreamNodeLogs(ctx, req)
	if err != nil {
		return wrapGRPCError(err)
	}

	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			// Check if context was cancelled (normal for Ctrl+C)
			if ctx.Err() != nil {
				return nil
			}
			return wrapGRPCError(err)
		}

		entry := &LogEntry{
			Stream:  resp.Stream,
			Message: resp.Message,
		}
		if resp.Timestamp != nil {
			entry.Timestamp = resp.Timestamp.AsTime()
		}

		if err := callback(entry); err != nil {
			return err
		}
	}
}

// wrapGRPCError converts gRPC errors to user-friendly messages.
func wrapGRPCError(err error) error {
	st, ok := status.FromError(err)
	if !ok {
		return err
	}

	switch st.Code() {
	case codes.NotFound:
		return fmt.Errorf("not found: %s", st.Message())
	case codes.AlreadyExists:
		return fmt.Errorf("already exists: %s", st.Message())
	case codes.InvalidArgument:
		return fmt.Errorf("invalid argument: %s", st.Message())
	case codes.Unavailable:
		return fmt.Errorf("daemon unavailable: %s", st.Message())
	default:
		return fmt.Errorf("%s: %s", st.Code(), st.Message())
	}
}
