// internal/client/grpc.go
package client

import (
	"context"
	"fmt"

	v1 "github.com/altuslabsxyz/devnet-builder/api/proto/gen/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// GRPCClient wraps the gRPC DevnetServiceClient, NodeServiceClient, UpgradeServiceClient, and TransactionServiceClient.
type GRPCClient struct {
	conn        *grpc.ClientConn
	devnet      v1.DevnetServiceClient
	node        v1.NodeServiceClient
	upgrade     v1.UpgradeServiceClient
	transaction v1.TransactionServiceClient
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
func (c *GRPCClient) CreateDevnet(ctx context.Context, name string, spec *v1.DevnetSpec, labels map[string]string) (*v1.Devnet, error) {
	req := &v1.CreateDevnetRequest{
		Name:   name,
		Spec:   spec,
		Labels: labels,
	}

	resp, err := c.devnet.CreateDevnet(ctx, req)
	if err != nil {
		return nil, wrapGRPCError(err)
	}

	return resp.Devnet, nil
}

// GetDevnet retrieves a devnet by name.
func (c *GRPCClient) GetDevnet(ctx context.Context, name string) (*v1.Devnet, error) {
	resp, err := c.devnet.GetDevnet(ctx, &v1.GetDevnetRequest{Name: name})
	if err != nil {
		return nil, wrapGRPCError(err)
	}
	return resp.Devnet, nil
}

// ListDevnets lists all devnets.
func (c *GRPCClient) ListDevnets(ctx context.Context) ([]*v1.Devnet, error) {
	resp, err := c.devnet.ListDevnets(ctx, &v1.ListDevnetsRequest{})
	if err != nil {
		return nil, wrapGRPCError(err)
	}
	return resp.Devnets, nil
}

// DeleteDevnet deletes a devnet.
func (c *GRPCClient) DeleteDevnet(ctx context.Context, name string) error {
	_, err := c.devnet.DeleteDevnet(ctx, &v1.DeleteDevnetRequest{Name: name})
	if err != nil {
		return wrapGRPCError(err)
	}
	return nil
}

// StartDevnet starts a stopped devnet.
func (c *GRPCClient) StartDevnet(ctx context.Context, name string) (*v1.Devnet, error) {
	resp, err := c.devnet.StartDevnet(ctx, &v1.StartDevnetRequest{Name: name})
	if err != nil {
		return nil, wrapGRPCError(err)
	}
	return resp.Devnet, nil
}

// StopDevnet stops a running devnet.
func (c *GRPCClient) StopDevnet(ctx context.Context, name string) (*v1.Devnet, error) {
	resp, err := c.devnet.StopDevnet(ctx, &v1.StopDevnetRequest{Name: name})
	if err != nil {
		return nil, wrapGRPCError(err)
	}
	return resp.Devnet, nil
}

// GetNode retrieves a node by devnet name and index.
func (c *GRPCClient) GetNode(ctx context.Context, devnetName string, index int) (*v1.Node, error) {
	resp, err := c.node.GetNode(ctx, &v1.GetNodeRequest{
		DevnetName: devnetName,
		Index:      int32(index),
	})
	if err != nil {
		return nil, wrapGRPCError(err)
	}
	return resp.Node, nil
}

// ListNodes lists all nodes in a devnet.
func (c *GRPCClient) ListNodes(ctx context.Context, devnetName string) ([]*v1.Node, error) {
	resp, err := c.node.ListNodes(ctx, &v1.ListNodesRequest{
		DevnetName: devnetName,
	})
	if err != nil {
		return nil, wrapGRPCError(err)
	}
	return resp.Nodes, nil
}

// StartNode starts a stopped node.
func (c *GRPCClient) StartNode(ctx context.Context, devnetName string, index int) (*v1.Node, error) {
	resp, err := c.node.StartNode(ctx, &v1.StartNodeRequest{
		DevnetName: devnetName,
		Index:      int32(index),
	})
	if err != nil {
		return nil, wrapGRPCError(err)
	}
	return resp.Node, nil
}

// StopNode stops a running node.
func (c *GRPCClient) StopNode(ctx context.Context, devnetName string, index int) (*v1.Node, error) {
	resp, err := c.node.StopNode(ctx, &v1.StopNodeRequest{
		DevnetName: devnetName,
		Index:      int32(index),
	})
	if err != nil {
		return nil, wrapGRPCError(err)
	}
	return resp.Node, nil
}

// RestartNode restarts a node.
func (c *GRPCClient) RestartNode(ctx context.Context, devnetName string, index int) (*v1.Node, error) {
	resp, err := c.node.RestartNode(ctx, &v1.RestartNodeRequest{
		DevnetName: devnetName,
		Index:      int32(index),
	})
	if err != nil {
		return nil, wrapGRPCError(err)
	}
	return resp.Node, nil
}

// CreateUpgrade creates a new upgrade.
func (c *GRPCClient) CreateUpgrade(ctx context.Context, name string, spec *v1.UpgradeSpec) (*v1.Upgrade, error) {
	req := &v1.CreateUpgradeRequest{
		Name: name,
		Spec: spec,
	}

	resp, err := c.upgrade.CreateUpgrade(ctx, req)
	if err != nil {
		return nil, wrapGRPCError(err)
	}

	return resp.Upgrade, nil
}

// GetUpgrade retrieves an upgrade by name.
func (c *GRPCClient) GetUpgrade(ctx context.Context, name string) (*v1.Upgrade, error) {
	resp, err := c.upgrade.GetUpgrade(ctx, &v1.GetUpgradeRequest{Name: name})
	if err != nil {
		return nil, wrapGRPCError(err)
	}
	return resp.Upgrade, nil
}

// ListUpgrades lists all upgrades for a devnet.
func (c *GRPCClient) ListUpgrades(ctx context.Context, devnetName string) ([]*v1.Upgrade, error) {
	resp, err := c.upgrade.ListUpgrades(ctx, &v1.ListUpgradesRequest{DevnetName: devnetName})
	if err != nil {
		return nil, wrapGRPCError(err)
	}
	return resp.Upgrades, nil
}

// DeleteUpgrade deletes an upgrade.
func (c *GRPCClient) DeleteUpgrade(ctx context.Context, name string) error {
	_, err := c.upgrade.DeleteUpgrade(ctx, &v1.DeleteUpgradeRequest{Name: name})
	if err != nil {
		return wrapGRPCError(err)
	}
	return nil
}

// CancelUpgrade cancels a running upgrade.
func (c *GRPCClient) CancelUpgrade(ctx context.Context, name string) (*v1.Upgrade, error) {
	resp, err := c.upgrade.CancelUpgrade(ctx, &v1.CancelUpgradeRequest{Name: name})
	if err != nil {
		return nil, wrapGRPCError(err)
	}
	return resp.Upgrade, nil
}

// RetryUpgrade retries a failed upgrade.
func (c *GRPCClient) RetryUpgrade(ctx context.Context, name string) (*v1.Upgrade, error) {
	resp, err := c.upgrade.RetryUpgrade(ctx, &v1.RetryUpgradeRequest{Name: name})
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
