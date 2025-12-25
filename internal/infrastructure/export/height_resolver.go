package export

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// HeightResolver resolves the current blockchain height from RPC endpoints.
type HeightResolver struct {
	httpClient *http.Client
	timeout    time.Duration
}

// NewHeightResolver creates a new HeightResolver with default timeout.
func NewHeightResolver() *HeightResolver {
	return &HeightResolver{
		httpClient: &http.Client{},
		timeout:    10 * time.Second,
	}
}

// WithTimeout sets a custom timeout for RPC queries.
func (r *HeightResolver) WithTimeout(timeout time.Duration) *HeightResolver {
	r.timeout = timeout
	r.httpClient.Timeout = timeout
	return r
}

// GetCurrentHeight queries the RPC endpoint to get the current block height.
// rpcURL should be in the format "http://localhost:26657"
func (r *HeightResolver) GetCurrentHeight(ctx context.Context, rpcURL string) (int64, error) {
	if rpcURL == "" {
		return 0, fmt.Errorf("RPC URL cannot be empty")
	}

	// Create request context with timeout
	reqCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	// Query /status endpoint
	statusURL := fmt.Sprintf("%s/status", rpcURL)
	req, err := http.NewRequestWithContext(reqCtx, "GET", statusURL, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to query RPC: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("RPC returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var status struct {
		Result struct {
			SyncInfo struct {
				LatestBlockHeight string `json:"latest_block_height"`
			} `json:"sync_info"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return 0, fmt.Errorf("failed to parse RPC response: %w", err)
	}

	// Parse height string to int64
	var height int64
	if _, err := fmt.Sscanf(status.Result.SyncInfo.LatestBlockHeight, "%d", &height); err != nil {
		return 0, fmt.Errorf("failed to parse block height '%s': %w",
			status.Result.SyncInfo.LatestBlockHeight, err)
	}

	if height <= 0 {
		return 0, fmt.Errorf("invalid block height: %d", height)
	}

	return height, nil
}

// WaitForHeight waits until the blockchain reaches or exceeds the target height.
// It polls the RPC endpoint at the specified interval.
func (r *HeightResolver) WaitForHeight(ctx context.Context, rpcURL string, targetHeight int64, pollInterval time.Duration) error {
	if pollInterval == 0 {
		pollInterval = 2 * time.Second
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			currentHeight, err := r.GetCurrentHeight(ctx, rpcURL)
			if err != nil {
				return fmt.Errorf("failed to get current height: %w", err)
			}

			if currentHeight >= targetHeight {
				return nil
			}
		}
	}
}
