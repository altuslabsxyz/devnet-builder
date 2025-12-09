package upgrade

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// RPCClient handles CometBFT RPC requests.
type RPCClient struct {
	baseURL string
	client  *http.Client
}

// NewRPCClient creates a new RPC client.
func NewRPCClient(host string, port int) *RPCClient {
	return &RPCClient{
		baseURL: fmt.Sprintf("http://%s:%d", host, port),
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// StatusResponse represents the RPC status response.
type StatusResponse struct {
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

// GetCurrentHeight returns the current block height from RPC.
func (c *RPCClient) GetCurrentHeight(ctx context.Context) (int64, error) {
	status, err := c.GetStatus(ctx)
	if err != nil {
		return 0, err
	}

	height, err := strconv.ParseInt(status.Result.SyncInfo.LatestBlockHeight, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse height: %w", err)
	}

	return height, nil
}

// GetStatus returns the full status response.
func (c *RPCClient) GetStatus(ctx context.Context) (*StatusResponse, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/status", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("status request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status request returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var status StatusResponse
	if err := json.Unmarshal(body, &status); err != nil {
		return nil, fmt.Errorf("failed to parse status: %w", err)
	}

	return &status, nil
}

// GetBlockTime estimates the average block time from recent blocks.
func (c *RPCClient) GetBlockTime(ctx context.Context, sampleSize int) (time.Duration, error) {
	if sampleSize < 2 {
		sampleSize = 2
	}

	// Get current height
	currentHeight, err := c.GetCurrentHeight(ctx)
	if err != nil {
		return 0, err
	}

	// Get timestamps of recent blocks
	startHeight := currentHeight - int64(sampleSize)
	if startHeight < 1 {
		startHeight = 1
	}

	startTime, err := c.getBlockTime(ctx, startHeight)
	if err != nil {
		// If we can't get historical blocks, use default
		return 2 * time.Second, nil
	}

	endTime, err := c.getBlockTime(ctx, currentHeight)
	if err != nil {
		return 2 * time.Second, nil
	}

	// Calculate average block time
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

func (c *RPCClient) getBlockTime(ctx context.Context, height int64) (time.Time, error) {
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
		return time.Time{}, fmt.Errorf("failed to parse block time: %w", err)
	}

	return blockTime, nil
}

// IsChainRunning checks if the chain is producing blocks.
func (c *RPCClient) IsChainRunning(ctx context.Context) bool {
	_, err := c.GetCurrentHeight(ctx)
	return err == nil
}

// WaitForBlock waits until the chain reaches the specified height.
func (c *RPCClient) WaitForBlock(ctx context.Context, targetHeight int64, callback func(current, target int64)) error {
	ticker := time.NewTicker(BlockPollInterval)
	defer ticker.Stop()

	timeout := time.NewTimer(UpgradeTimeout)
	defer timeout.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout.C:
			return ErrUpgradeTimeout
		case <-ticker.C:
			height, err := c.GetCurrentHeight(ctx)
			if err != nil {
				// Chain might be halted for upgrade
				continue
			}

			if callback != nil {
				callback(height, targetHeight)
			}

			if height >= targetHeight {
				return nil
			}
		}
	}
}
