// Package rpc provides RPC client implementations.
package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/b-harvest/devnet-builder/internal/application/ports"
)

const (
	// DefaultTimeout is the default HTTP timeout.
	DefaultTimeout = 10 * time.Second

	// DefaultBlockPollInterval is the default interval for polling blocks.
	DefaultBlockPollInterval = 2 * time.Second

	// DefaultWaitTimeout is the default timeout for waiting operations.
	DefaultWaitTimeout = 10 * time.Minute
)

// CosmosRPCClient implements RPCClient for Cosmos chains.
type CosmosRPCClient struct {
	baseURL      string
	client       *http.Client
	pollInterval time.Duration
	waitTimeout  time.Duration
}

// NewCosmosRPCClient creates a new CosmosRPCClient.
func NewCosmosRPCClient(host string, port int) *CosmosRPCClient {
	return &CosmosRPCClient{
		baseURL: fmt.Sprintf("http://%s:%d", host, port),
		client: &http.Client{
			Timeout: DefaultTimeout,
		},
		pollInterval: DefaultBlockPollInterval,
		waitTimeout:  DefaultWaitTimeout,
	}
}

// NewCosmosRPCClientWithURL creates a new CosmosRPCClient with a full URL.
func NewCosmosRPCClientWithURL(url string) *CosmosRPCClient {
	return &CosmosRPCClient{
		baseURL: url,
		client: &http.Client{
			Timeout: DefaultTimeout,
		},
		pollInterval: DefaultBlockPollInterval,
		waitTimeout:  DefaultWaitTimeout,
	}
}

// WithPollInterval sets the block poll interval.
func (c *CosmosRPCClient) WithPollInterval(interval time.Duration) *CosmosRPCClient {
	c.pollInterval = interval
	return c
}

// WithWaitTimeout sets the wait timeout.
func (c *CosmosRPCClient) WithWaitTimeout(timeout time.Duration) *CosmosRPCClient {
	c.waitTimeout = timeout
	return c
}

// statusResponse represents the RPC status response.
type statusResponse struct {
	Result struct {
		SyncInfo struct {
			LatestBlockHeight string `json:"latest_block_height"`
			LatestBlockTime   string `json:"latest_block_time"`
			CatchingUp        bool   `json:"catching_up"`
		} `json:"sync_info"`
		NodeInfo struct {
			Network string `json:"network"`
		} `json:"node_info"`
	} `json:"result"`
}

// GetBlockHeight returns the current block height.
func (c *CosmosRPCClient) GetBlockHeight(ctx context.Context) (int64, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/status", nil)
	if err != nil {
		return 0, &RPCError{Operation: "status", Message: err.Error()}
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return 0, &RPCError{Operation: "status", Message: err.Error()}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, &RPCError{Operation: "status", Message: fmt.Sprintf("HTTP %d", resp.StatusCode)}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, &RPCError{Operation: "status", Message: err.Error()}
	}

	var status statusResponse
	if err := json.Unmarshal(body, &status); err != nil {
		return 0, &RPCError{Operation: "status", Message: "failed to parse response"}
	}

	height, err := strconv.ParseInt(status.Result.SyncInfo.LatestBlockHeight, 10, 64)
	if err != nil {
		return 0, &RPCError{Operation: "status", Message: "failed to parse height"}
	}

	return height, nil
}

// GetBlockTime estimates the average block time from recent blocks.
func (c *CosmosRPCClient) GetBlockTime(ctx context.Context, sampleSize int) (time.Duration, error) {
	if sampleSize < 2 {
		sampleSize = 10
	}

	currentHeight, err := c.GetBlockHeight(ctx)
	if err != nil {
		return 0, err
	}

	startHeight := currentHeight - int64(sampleSize)
	if startHeight < 1 {
		startHeight = 1
	}

	startTime, err := c.getBlockTimestamp(ctx, startHeight)
	if err != nil {
		return 2 * time.Second, nil // Default fallback
	}

	endTime, err := c.getBlockTimestamp(ctx, currentHeight)
	if err != nil {
		return 2 * time.Second, nil
	}

	blockCount := currentHeight - startHeight
	if blockCount <= 0 {
		return 2 * time.Second, nil
	}

	totalDuration := endTime.Sub(startTime)
	avgBlockTime := totalDuration / time.Duration(blockCount)

	// Sanity check
	if avgBlockTime < 100*time.Millisecond || avgBlockTime > 30*time.Second {
		return 2 * time.Second, nil
	}

	return avgBlockTime, nil
}

func (c *CosmosRPCClient) getBlockTimestamp(ctx context.Context, height int64) (time.Time, error) {
	url := fmt.Sprintf("%s/block?height=%d", c.baseURL, height)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return time.Time{}, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return time.Time{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return time.Time{}, fmt.Errorf("block request returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return time.Time{}, err
	}

	var blockResp struct {
		Result struct {
			Block struct {
				Header struct {
					Time string `json:"time"`
				} `json:"header"`
			} `json:"block"`
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &blockResp); err != nil {
		return time.Time{}, err
	}

	blockTime, err := time.Parse(time.RFC3339Nano, blockResp.Result.Block.Header.Time)
	if err != nil {
		return time.Time{}, err
	}

	return blockTime, nil
}

// IsChainRunning checks if the chain is responding.
func (c *CosmosRPCClient) IsChainRunning(ctx context.Context) bool {
	_, err := c.GetBlockHeight(ctx)
	return err == nil
}

// WaitForBlock waits until the chain reaches the specified height.
func (c *CosmosRPCClient) WaitForBlock(ctx context.Context, height int64) error {
	ticker := time.NewTicker(c.pollInterval)
	defer ticker.Stop()

	timeout := time.NewTimer(c.waitTimeout)
	defer timeout.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout.C:
			return &RPCError{
				Operation: "wait_for_block",
				Message:   fmt.Sprintf("timeout waiting for height %d", height),
			}
		case <-ticker.C:
			currentHeight, err := c.GetBlockHeight(ctx)
			if err != nil {
				// Chain might be halted for upgrade, continue waiting
				continue
			}

			if currentHeight >= height {
				return nil
			}
		}
	}
}

// GetProposal retrieves a governance proposal by ID.
func (c *CosmosRPCClient) GetProposal(ctx context.Context, id uint64) (*ports.Proposal, error) {
	url := fmt.Sprintf("%s/cosmos/gov/v1/proposals/%d", c.restURL(), id)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, &RPCError{Operation: "get_proposal", Message: err.Error()}
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, &RPCError{Operation: "get_proposal", Message: err.Error()}
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, &NotFoundError{Resource: fmt.Sprintf("proposal %d", id)}
	}

	if resp.StatusCode != http.StatusOK {
		return nil, &RPCError{Operation: "get_proposal", Message: fmt.Sprintf("HTTP %d", resp.StatusCode)}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &RPCError{Operation: "get_proposal", Message: err.Error()}
	}

	var result struct {
		Proposal struct {
			ID           string `json:"id"`
			Status       string `json:"status"`
			SubmitTime   string `json:"submit_time"`
			DepositEnd   string `json:"deposit_end_time"`
			VotingEnd    string `json:"voting_end_time"`
			TotalDeposit []struct {
				Denom  string `json:"denom"`
				Amount string `json:"amount"`
			} `json:"total_deposit"`
			FinalTallyResult struct {
				YesCount        string `json:"yes_count"`
				NoCount         string `json:"no_count"`
				AbstainCount    string `json:"abstain_count"`
				NoWithVetoCount string `json:"no_with_veto_count"`
			} `json:"final_tally_result"`
		} `json:"proposal"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, &RPCError{Operation: "get_proposal", Message: "failed to parse proposal"}
	}

	p := &ports.Proposal{
		ID:              id,
		Status:          ports.ProposalStatus(result.Proposal.Status),
		FinalTallyYes:   result.Proposal.FinalTallyResult.YesCount,
		FinalTallyNo:    result.Proposal.FinalTallyResult.NoCount,
		FinalTallyAbstain: result.Proposal.FinalTallyResult.AbstainCount,
	}

	if submitTime, err := time.Parse(time.RFC3339, result.Proposal.SubmitTime); err == nil {
		p.SubmitTime = submitTime
	}
	if depositEnd, err := time.Parse(time.RFC3339, result.Proposal.DepositEnd); err == nil {
		p.DepositEndTime = depositEnd
	}
	if votingEnd, err := time.Parse(time.RFC3339, result.Proposal.VotingEnd); err == nil {
		p.VotingEndTime = votingEnd
	}

	if len(result.Proposal.TotalDeposit) > 0 {
		p.TotalDeposit = result.Proposal.TotalDeposit[0].Amount
	}

	return p, nil
}

// GetUpgradePlan retrieves the current upgrade plan.
func (c *CosmosRPCClient) GetUpgradePlan(ctx context.Context) (*ports.UpgradePlan, error) {
	url := fmt.Sprintf("%s/cosmos/upgrade/v1beta1/current_plan", c.restURL())
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, &RPCError{Operation: "get_upgrade_plan", Message: err.Error()}
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, &RPCError{Operation: "get_upgrade_plan", Message: err.Error()}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, &RPCError{Operation: "get_upgrade_plan", Message: fmt.Sprintf("HTTP %d", resp.StatusCode)}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &RPCError{Operation: "get_upgrade_plan", Message: err.Error()}
	}

	var result struct {
		Plan *struct {
			Name   string `json:"name"`
			Height string `json:"height"`
			Info   string `json:"info"`
			Time   string `json:"time"`
		} `json:"plan"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, &RPCError{Operation: "get_upgrade_plan", Message: "failed to parse plan"}
	}

	if result.Plan == nil {
		return nil, nil // No upgrade scheduled
	}

	height, _ := strconv.ParseInt(result.Plan.Height, 10, 64)

	plan := &ports.UpgradePlan{
		Name:   result.Plan.Name,
		Height: height,
		Info:   result.Plan.Info,
	}

	if result.Plan.Time != "" {
		if t, err := time.Parse(time.RFC3339, result.Plan.Time); err == nil {
			plan.Time = t
		}
	}

	return plan, nil
}

// restURL returns the REST API base URL (same as RPC port for simplicity).
func (c *CosmosRPCClient) restURL() string {
	return c.baseURL
}

// Ensure CosmosRPCClient implements RPCClient.
var _ ports.RPCClient = (*CosmosRPCClient)(nil)
