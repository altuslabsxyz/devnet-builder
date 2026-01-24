// Package upgrader provides upgrade runtime implementations.
package upgrader

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/daemon/store"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/types"
)

// Runtime implements UpgradeRuntime for Cosmos SDK chains.
// It uses RPC calls for chain queries and store operations for node management.
type Runtime struct {
	store   store.Store
	baseRPC int
	client  *http.Client
	logger  *slog.Logger
}

// Config configures the upgrade runtime.
type Config struct {
	// BaseRPC is the base RPC port (default: 26657).
	BaseRPC int

	// Timeout for HTTP requests.
	Timeout time.Duration

	// Logger for runtime operations.
	Logger *slog.Logger
}

// NewRuntime creates a new upgrade runtime.
func NewRuntime(s store.Store, cfg Config) *Runtime {
	if cfg.BaseRPC == 0 {
		cfg.BaseRPC = 26657
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 10 * time.Second
	}

	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &Runtime{
		store:   s,
		baseRPC: cfg.BaseRPC,
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
		logger: logger,
	}
}

// SubmitUpgradeProposal submits a governance proposal for the upgrade.
// Note: In a real implementation, this would use the tx broadcast endpoint.
// For now, we simulate proposal creation.
func (r *Runtime) SubmitUpgradeProposal(ctx context.Context, devnetName string, upgradeName string, targetHeight int64) (uint64, error) {
	r.logger.Info("submitting upgrade proposal",
		"devnet", devnetName,
		"upgradeName", upgradeName,
		"targetHeight", targetHeight)

	// In a real implementation, we would:
	// 1. Build the upgrade proposal tx
	// 2. Sign it with a validator key
	// 3. Broadcast via /broadcast_tx_commit
	//
	// For now, we simulate with proposal ID 1
	// Real implementation would parse the proposal ID from the tx result

	return 1, nil
}

// GetProposalStatus returns the current voting status of a proposal.
func (r *Runtime) GetProposalStatus(ctx context.Context, devnetName string, proposalID uint64) (votesReceived, votesRequired int, passed bool, err error) {
	r.logger.Debug("getting proposal status",
		"devnet", devnetName,
		"proposalID", proposalID)

	// Query ABCI for proposal status
	// In a real implementation, this would query:
	// /abci_query?path="custom/gov/proposal/{proposalID}"
	//
	// For devnet, we can simulate auto-pass since we control all validators

	validatorCount, err := r.GetValidatorCount(ctx, devnetName)
	if err != nil {
		return 0, 0, false, err
	}

	// Simulate all validators voted
	return validatorCount, validatorCount, true, nil
}

// VoteOnProposal casts a vote on the proposal from a validator.
func (r *Runtime) VoteOnProposal(ctx context.Context, devnetName string, proposalID uint64, validatorIndex int, voteYes bool) error {
	r.logger.Info("voting on proposal",
		"devnet", devnetName,
		"proposalID", proposalID,
		"validator", validatorIndex,
		"vote", voteYes)

	// In a real implementation, we would:
	// 1. Build the vote tx
	// 2. Sign it with the validator's key
	// 3. Broadcast via /broadcast_tx_commit
	//
	// For devnet with controlled validators, we can simulate this

	return nil
}

// GetCurrentHeight returns the chain's current block height.
func (r *Runtime) GetCurrentHeight(ctx context.Context, devnetName string) (int64, error) {
	// Get any running node for this devnet
	nodes, err := r.store.ListNodes(ctx, devnetName)
	if err != nil {
		return 0, fmt.Errorf("failed to list nodes: %w", err)
	}

	for _, node := range nodes {
		if node.Status.Phase == types.NodePhaseRunning {
			rpcPort := r.baseRPC + node.Spec.Index
			height, err := r.getNodeHeight(ctx, rpcPort)
			if err != nil {
				r.logger.Debug("failed to get height from node",
					"nodeIndex", node.Spec.Index,
					"error", err)
				continue
			}
			return height, nil
		}
	}

	return 0, fmt.Errorf("no running nodes found for devnet %s", devnetName)
}

// getNodeHeight queries a node's current block height.
func (r *Runtime) getNodeHeight(ctx context.Context, rpcPort int) (int64, error) {
	url := fmt.Sprintf("http://127.0.0.1:%d/status", rpcPort)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("status endpoint returned %d", resp.StatusCode)
	}

	var statusResp struct {
		Result struct {
			SyncInfo struct {
				LatestBlockHeight int64 `json:"latest_block_height,string"`
			} `json:"sync_info"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&statusResp); err != nil {
		return 0, err
	}

	return statusResp.Result.SyncInfo.LatestBlockHeight, nil
}

// SwitchNodeBinary replaces the binary on a node and restarts it.
func (r *Runtime) SwitchNodeBinary(ctx context.Context, devnetName string, nodeIndex int, newBinary types.BinarySource) error {
	r.logger.Info("switching node binary",
		"devnet", devnetName,
		"nodeIndex", nodeIndex,
		"newBinary", newBinary)

	// Get the node
	node, err := r.store.GetNode(ctx, devnetName, nodeIndex)
	if err != nil {
		return fmt.Errorf("failed to get node: %w", err)
	}

	// Update binary path
	// For Docker mode, this would be the image reference
	// For local mode, this would be the binary path
	switch newBinary.Type {
	case "local":
		node.Spec.BinaryPath = newBinary.Path
	case "cache", "github":
		// These would be resolved to a local path by the cache layer
		node.Spec.BinaryPath = newBinary.Path
	case "url":
		// For Docker, this might be the image URL
		node.Spec.BinaryPath = newBinary.URL
	default:
		// Try to use whatever value was provided
		if newBinary.Path != "" {
			node.Spec.BinaryPath = newBinary.Path
		} else if newBinary.URL != "" {
			node.Spec.BinaryPath = newBinary.URL
		}
	}

	// Trigger restart by setting phase to Pending
	// The NodeController will handle the actual restart
	node.Status.Phase = types.NodePhasePending
	node.Status.Message = "Restarting with new binary for upgrade"
	node.Status.RestartCount++
	node.Metadata.UpdatedAt = time.Now()

	if err := r.store.UpdateNode(ctx, node); err != nil {
		return fmt.Errorf("failed to update node: %w", err)
	}

	return nil
}

// VerifyNodeVersion checks that a node is running the expected version.
func (r *Runtime) VerifyNodeVersion(ctx context.Context, devnetName string, nodeIndex int, expectedVersion string) (bool, error) {
	r.logger.Debug("verifying node version",
		"devnet", devnetName,
		"nodeIndex", nodeIndex,
		"expectedVersion", expectedVersion)

	// Check if node is running
	node, err := r.store.GetNode(ctx, devnetName, nodeIndex)
	if err != nil {
		return false, fmt.Errorf("failed to get node: %w", err)
	}

	if node.Status.Phase != types.NodePhaseRunning {
		return false, nil // Node not running yet
	}

	// Query the node's application version
	rpcPort := r.baseRPC + nodeIndex
	version, err := r.getNodeVersion(ctx, rpcPort)
	if err != nil {
		return false, err
	}

	// Version comparison - could be more sophisticated
	return version == expectedVersion || expectedVersion == "", nil
}

// getNodeVersion queries a node's application version.
func (r *Runtime) getNodeVersion(ctx context.Context, rpcPort int) (string, error) {
	url := fmt.Sprintf("http://127.0.0.1:%d/abci_info", rpcPort)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("abci_info endpoint returned %d", resp.StatusCode)
	}

	var abciResp struct {
		Result struct {
			Response struct {
				Version string `json:"version"`
			} `json:"response"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&abciResp); err != nil {
		return "", err
	}

	return abciResp.Result.Response.Version, nil
}

// ExportState exports the chain state to a file.
func (r *Runtime) ExportState(ctx context.Context, devnetName string, outputPath string) error {
	r.logger.Info("exporting chain state",
		"devnet", devnetName,
		"outputPath", outputPath)

	// In a real implementation, this would:
	// 1. Stop the nodes gracefully
	// 2. Run the export command on one node
	// 3. Copy the export to the output path
	// 4. Restart the nodes
	//
	// For now, we simulate success
	// The actual implementation would use the CLI tool or Docker exec

	return nil
}

// GetValidatorCount returns the number of validators in the devnet.
func (r *Runtime) GetValidatorCount(ctx context.Context, devnetName string) (int, error) {
	// Get devnet spec
	devnet, err := r.store.GetDevnet(ctx, devnetName)
	if err != nil {
		return 0, fmt.Errorf("failed to get devnet: %w", err)
	}

	return devnet.Spec.Validators, nil
}
