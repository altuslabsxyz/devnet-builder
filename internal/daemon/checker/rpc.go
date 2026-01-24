// Package checker provides health checking implementations for nodes.
package checker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/daemon/types"
)

// RPCHealthChecker implements HealthChecker using node RPC endpoints.
// It supports CometBFT/Tendermint-based chains (Cosmos SDK).
type RPCHealthChecker struct {
	client  *http.Client
	baseRPC int
	logger  *slog.Logger
}

// Config configures the RPCHealthChecker.
type Config struct {
	// Timeout for HTTP requests.
	Timeout time.Duration

	// BaseRPC is the base RPC port (default: 26657).
	// Each node's RPC port is calculated as BaseRPC + node.Spec.Index.
	BaseRPC int

	// Logger for checker operations.
	Logger *slog.Logger
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		Timeout: 5 * time.Second,
		BaseRPC: 26657,
	}
}

// NewRPCHealthChecker creates a new RPC-based health checker.
func NewRPCHealthChecker(cfg Config) *RPCHealthChecker {
	if cfg.Timeout == 0 {
		cfg.Timeout = 5 * time.Second
	}
	if cfg.BaseRPC == 0 {
		cfg.BaseRPC = 26657
	}

	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &RPCHealthChecker{
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
		baseRPC: cfg.BaseRPC,
		logger:  logger,
	}
}

// CheckHealth performs a health check on a node by querying its RPC endpoint.
func (c *RPCHealthChecker) CheckHealth(ctx context.Context, node *types.Node) (*types.HealthCheckResult, error) {
	result := &types.HealthCheckResult{
		NodeKey:   fmt.Sprintf("%s/%d", node.Spec.DevnetRef, node.Spec.Index),
		CheckedAt: time.Now(),
	}

	// Calculate RPC port for this node
	rpcPort := c.baseRPC + node.Spec.Index
	statusURL := fmt.Sprintf("http://127.0.0.1:%d/status", rpcPort)

	c.logger.Debug("checking node health",
		"node", result.NodeKey,
		"url", statusURL)

	// Make request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, statusURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		result.Healthy = false
		result.Error = fmt.Sprintf("RPC request failed: %v", err)
		return result, nil // Return result, not error - let caller handle
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		result.Healthy = false
		result.Error = fmt.Sprintf("RPC returned status %d", resp.StatusCode)
		return result, nil
	}

	// Parse CometBFT status response
	var statusResp CometBFTStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&statusResp); err != nil {
		result.Healthy = false
		result.Error = fmt.Sprintf("failed to parse status response: %v", err)
		return result, nil
	}

	// Extract health information
	result.Healthy = true
	result.BlockHeight = statusResp.Result.SyncInfo.LatestBlockHeight
	result.CatchingUp = statusResp.Result.SyncInfo.CatchingUp

	// Get peer count from net_info
	peerCount, err := c.getPeerCount(ctx, rpcPort)
	if err != nil {
		c.logger.Debug("failed to get peer count", "node", result.NodeKey, "error", err)
	} else {
		result.PeerCount = peerCount
	}

	c.logger.Debug("node health check complete",
		"node", result.NodeKey,
		"healthy", result.Healthy,
		"height", result.BlockHeight,
		"catchingUp", result.CatchingUp,
		"peers", result.PeerCount)

	return result, nil
}

// getPeerCount fetches the peer count from the node's net_info endpoint.
func (c *RPCHealthChecker) getPeerCount(ctx context.Context, rpcPort int) (int, error) {
	netInfoURL := fmt.Sprintf("http://127.0.0.1:%d/net_info", rpcPort)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, netInfoURL, nil)
	if err != nil {
		return 0, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("net_info returned status %d", resp.StatusCode)
	}

	var netInfoResp CometBFTNetInfoResponse
	if err := json.NewDecoder(resp.Body).Decode(&netInfoResp); err != nil {
		return 0, err
	}

	return netInfoResp.Result.NPeers, nil
}

// CometBFT RPC response types

// CometBFTStatusResponse is the response from /status endpoint.
type CometBFTStatusResponse struct {
	Result struct {
		NodeInfo struct {
			ID      string `json:"id"`
			Network string `json:"network"`
			Moniker string `json:"moniker"`
		} `json:"node_info"`
		SyncInfo struct {
			LatestBlockHash   string `json:"latest_block_hash"`
			LatestBlockHeight int64  `json:"latest_block_height,string"`
			LatestBlockTime   string `json:"latest_block_time"`
			CatchingUp        bool   `json:"catching_up"`
		} `json:"sync_info"`
		ValidatorInfo struct {
			Address     string `json:"address"`
			VotingPower string `json:"voting_power"`
		} `json:"validator_info"`
	} `json:"result"`
}

// CometBFTNetInfoResponse is the response from /net_info endpoint.
type CometBFTNetInfoResponse struct {
	Result struct {
		Listening bool   `json:"listening"`
		NPeers    int    `json:"n_peers,string"`
		Peers     []Peer `json:"peers"`
	} `json:"result"`
}

// Peer represents a connected peer.
type Peer struct {
	NodeInfo struct {
		ID      string `json:"id"`
		Moniker string `json:"moniker"`
	} `json:"node_info"`
	IsOutbound bool `json:"is_outbound"`
}
