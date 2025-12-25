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

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/b-harvest/devnet-builder/internal/application/ports"
	pb "github.com/b-harvest/devnet-builder/pkg/network/plugin"
)

// NetworkPluginModule defines the interface for plugin-based governance parameter queries.
// This is a minimal interface that only includes the method we need from the full network.Module.
type NetworkPluginModule interface {
	GetGovernanceParams(rpcEndpoint, networkType string) (*pb.GovernanceParamsResponse, error)
}

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
	baseURL       string // RPC URL (port 26657)
	restBaseURL   string // REST API URL (port 1317)
	client        *http.Client
	pollInterval  time.Duration
	waitTimeout   time.Duration
	pluginModule  NetworkPluginModule // Optional: for plugin-based parameter queries
	networkType   string              // Optional: network type for plugin queries
}

// NewCosmosRPCClient creates a new CosmosRPCClient.
// It uses the standard REST API port (1317) for REST endpoints.
func NewCosmosRPCClient(host string, port int) *CosmosRPCClient {
	return &CosmosRPCClient{
		baseURL:     fmt.Sprintf("http://%s:%d", host, port),
		restBaseURL: fmt.Sprintf("http://%s:%d", host, 1317), // Default REST port
		client: &http.Client{
			Timeout: DefaultTimeout,
		},
		pollInterval: DefaultBlockPollInterval,
		waitTimeout:  DefaultWaitTimeout,
	}
}

// NewCosmosRPCClientWithURL creates a new CosmosRPCClient with a full URL.
// It derives the REST URL from the RPC URL by using port 1317.
func NewCosmosRPCClientWithURL(url string) *CosmosRPCClient {
	// Extract host from URL and construct REST URL with port 1317
	restURL := "http://localhost:1317" // Default fallback
	if len(url) > 7 { // http://
		// Try to parse and replace port
		// Format: http://host:port
		hostStart := 7
		if url[:8] == "https://" {
			hostStart = 8
		}
		hostEnd := len(url)
		for i := hostStart; i < len(url); i++ {
			if url[i] == ':' {
				hostEnd = i
				break
			}
		}
		if hostEnd > hostStart {
			restURL = fmt.Sprintf("http://%s:1317", url[hostStart:hostEnd])
		}
	}
	return &CosmosRPCClient{
		baseURL:     url,
		restBaseURL: restURL,
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

// WithPlugin sets the network plugin module for plugin-based parameter queries.
// This enables delegation of governance parameter queries to network-specific plugins.
func (c *CosmosRPCClient) WithPlugin(pluginModule NetworkPluginModule, networkType string) *CosmosRPCClient {
	c.pluginModule = pluginModule
	c.networkType = networkType
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
	return c.restBaseURL
}

// GetAppVersion returns the application version from /abci_info.
// This is set by the application during InitChain and can be updated via upgrades.
func (c *CosmosRPCClient) GetAppVersion(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/abci_info", nil)
	if err != nil {
		return "", &RPCError{Operation: "abci_info", Message: err.Error()}
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return "", &RPCError{Operation: "abci_info", Message: err.Error()}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", &RPCError{Operation: "abci_info", Message: fmt.Sprintf("HTTP %d", resp.StatusCode)}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", &RPCError{Operation: "abci_info", Message: err.Error()}
	}

	var abciInfo struct {
		Result struct {
			Response struct {
				Version string `json:"version"`
			} `json:"response"`
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &abciInfo); err != nil {
		return "", &RPCError{Operation: "abci_info", Message: "failed to parse response"}
	}

	return abciInfo.Result.Response.Version, nil
}

// GetGovParams retrieves governance parameters from the chain.
func (c *CosmosRPCClient) GetGovParams(ctx context.Context) (*ports.GovParams, error) {
	// Phase 1: Try plugin-based query first (if plugin is configured)
	if c.pluginModule != nil {
		pluginParams, err := c.tryPluginGovernanceParams(ctx)
		if err == nil {
			// Plugin query succeeded
			return pluginParams, nil
		}

		// Check if error is Unimplemented (plugin doesn't support GetGovernanceParams)
		if s, ok := status.FromError(err); ok && s.Code() == codes.Unimplemented {
			// Plugin doesn't implement GetGovernanceParams, fall back to REST
			// This is expected for plugins that haven't been updated yet
		} else {
			// Plugin error (network issue, validation error, etc.)
			// Return the error - don't silently fall back to REST for real errors
			return nil, &RPCError{Operation: "gov_params_plugin", Message: err.Error()}
		}
	}

	// Phase 2: Fall back to direct REST API query
	// This path is used when:
	// - No plugin is configured
	// - Plugin returns Unimplemented (backward compatibility)
	return c.queryGovernanceParamsViaREST(ctx)
}

// tryPluginGovernanceParams attempts to query governance parameters via plugin.
func (c *CosmosRPCClient) tryPluginGovernanceParams(ctx context.Context) (*ports.GovParams, error) {
	resp, err := c.pluginModule.GetGovernanceParams(c.restBaseURL, c.networkType)
	if err != nil {
		return nil, err
	}

	// Check for error in response
	if resp.Error != "" {
		return nil, fmt.Errorf("plugin error: %s", resp.Error)
	}

	// Convert nanoseconds to time.Duration
	return &ports.GovParams{
		VotingPeriod:          time.Duration(resp.VotingPeriodNs),
		ExpeditedVotingPeriod: time.Duration(resp.ExpeditedVotingPeriodNs),
		MinDeposit:            resp.MinDeposit,
		ExpeditedMinDeposit:   resp.ExpeditedMinDeposit,
	}, nil
}

// queryGovernanceParamsViaREST queries governance parameters directly from Cosmos SDK REST API.
// This is the fallback method when plugin-based queries are unavailable.
func (c *CosmosRPCClient) queryGovernanceParamsViaREST(ctx context.Context) (*ports.GovParams, error) {
	// Query voting params
	votingURL := c.restBaseURL + "/cosmos/gov/v1/params/voting"
	req, err := http.NewRequestWithContext(ctx, "GET", votingURL, nil)
	if err != nil {
		return nil, &RPCError{Operation: "gov_params", Message: err.Error()}
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, &RPCError{Operation: "gov_params", Message: err.Error()}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, &RPCError{Operation: "gov_params", Message: fmt.Sprintf("HTTP %d", resp.StatusCode)}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &RPCError{Operation: "gov_params", Message: err.Error()}
	}

	var votingParams struct {
		VotingParams struct {
			VotingPeriod          string `json:"voting_period"`
			ExpeditedVotingPeriod string `json:"expedited_voting_period"`
		} `json:"voting_params"`
	}

	if err := json.Unmarshal(body, &votingParams); err != nil {
		return nil, &RPCError{Operation: "gov_params", Message: "failed to parse voting params"}
	}

	// Query deposit params
	depositURL := c.restBaseURL + "/cosmos/gov/v1/params/deposit"
	req, err = http.NewRequestWithContext(ctx, "GET", depositURL, nil)
	if err != nil {
		return nil, &RPCError{Operation: "gov_params", Message: err.Error()}
	}

	resp, err = c.client.Do(req)
	if err != nil {
		return nil, &RPCError{Operation: "gov_params", Message: err.Error()}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, &RPCError{Operation: "gov_params", Message: fmt.Sprintf("HTTP %d", resp.StatusCode)}
	}

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, &RPCError{Operation: "gov_params", Message: err.Error()}
	}

	var depositParams struct {
		DepositParams struct {
			MinDeposit          []struct {
				Denom  string `json:"denom"`
				Amount string `json:"amount"`
			} `json:"min_deposit"`
			ExpeditedMinDeposit []struct {
				Denom  string `json:"denom"`
				Amount string `json:"amount"`
			} `json:"expedited_min_deposit"`
		} `json:"deposit_params"`
	}

	if err := json.Unmarshal(body, &depositParams); err != nil {
		return nil, &RPCError{Operation: "gov_params", Message: "failed to parse deposit params"}
	}

	// Parse durations (format: "172800s" -> 172800 seconds)
	votingPeriod, err := parseDuration(votingParams.VotingParams.VotingPeriod)
	if err != nil {
		return nil, &RPCError{Operation: "gov_params", Message: fmt.Sprintf("failed to parse voting_period: %v", err)}
	}

	expeditedVotingPeriod, err := parseDuration(votingParams.VotingParams.ExpeditedVotingPeriod)
	if err != nil {
		return nil, &RPCError{Operation: "gov_params", Message: fmt.Sprintf("failed to parse expedited_voting_period: %v", err)}
	}

	// Extract min deposit amount (first denom)
	minDeposit := "0"
	if len(depositParams.DepositParams.MinDeposit) > 0 {
		minDeposit = depositParams.DepositParams.MinDeposit[0].Amount
	}

	expeditedMinDeposit := "0"
	if len(depositParams.DepositParams.ExpeditedMinDeposit) > 0 {
		expeditedMinDeposit = depositParams.DepositParams.ExpeditedMinDeposit[0].Amount
	}

	return &ports.GovParams{
		VotingPeriod:          votingPeriod,
		ExpeditedVotingPeriod: expeditedVotingPeriod,
		MinDeposit:            minDeposit,
		ExpeditedMinDeposit:   expeditedMinDeposit,
	}, nil
}

// parseDuration parses a duration string like "172800s" to time.Duration.
func parseDuration(s string) (time.Duration, error) {
	// Remove trailing 's' and parse as seconds
	if len(s) == 0 {
		return 0, fmt.Errorf("empty duration string")
	}
	if s[len(s)-1] != 's' {
		return 0, fmt.Errorf("duration must end with 's': %s", s)
	}

	seconds, err := strconv.ParseInt(s[:len(s)-1], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse duration: %v", err)
	}

	return time.Duration(seconds) * time.Second, nil
}

// Ensure CosmosRPCClient implements RPCClient.
var _ ports.RPCClient = (*CosmosRPCClient)(nil)
