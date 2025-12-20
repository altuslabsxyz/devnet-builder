package provision

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/b-harvest/devnet-builder/internal/output"
)

const (
	// DefaultPollInterval is the default interval for polling sync status.
	DefaultPollInterval = 2 * time.Second

	// DefaultStartupWait is the time to wait for node to start responding.
	DefaultStartupWait = 30 * time.Second
)

// SyncMonitor monitors the sync status of a node.
type SyncMonitor struct {
	rpcEndpoint  string
	pollInterval time.Duration
	logger       *output.Logger
}

// StatusResponse represents the CometBFT status response.
type StatusResponse struct {
	Result struct {
		SyncInfo struct {
			LatestBlockHeight   string `json:"latest_block_height"`
			LatestBlockTime     string `json:"latest_block_time"`
			CatchingUp          bool   `json:"catching_up"`
			EarliestBlockHeight string `json:"earliest_block_height"`
		} `json:"sync_info"`
		NodeInfo struct {
			Network string `json:"network"`
		} `json:"node_info"`
	} `json:"result"`
}

// NewSyncMonitor creates a new SyncMonitor.
func NewSyncMonitor(rpcEndpoint string, logger *output.Logger) *SyncMonitor {
	if logger == nil {
		logger = output.DefaultLogger
	}
	return &SyncMonitor{
		rpcEndpoint:  rpcEndpoint,
		pollInterval: DefaultPollInterval,
		logger:       logger,
	}
}

// WaitForSync waits until the node is fully synced.
// Returns the final synced height.
func (m *SyncMonitor) WaitForSync(ctx context.Context) (int64, error) {
	m.logger.Debug("Waiting for node to start...")

	// First, wait for the node to start responding
	if err := m.waitForNodeReady(ctx); err != nil {
		return 0, fmt.Errorf("node failed to start: %w", err)
	}

	m.logger.Debug("Node is responding, waiting for sync to complete...")

	ticker := time.NewTicker(m.pollInterval)
	defer ticker.Stop()

	var lastHeight int64
	var lastProgressLog time.Time

	for {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-ticker.C:
			status, err := m.getStatus(ctx)
			if err != nil {
				m.logger.Debug("Status check failed: %v", err)
				continue
			}

			height := parseHeight(status.Result.SyncInfo.LatestBlockHeight)

			// Log progress periodically
			if time.Since(lastProgressLog) > 10*time.Second || height != lastHeight {
				if status.Result.SyncInfo.CatchingUp {
					m.logger.Debug("Syncing... height=%d (catching up)", height)
				} else {
					m.logger.Debug("Syncing... height=%d", height)
				}
				lastProgressLog = time.Now()
				lastHeight = height
			}

			// Check if sync is complete
			if !status.Result.SyncInfo.CatchingUp && height > 0 {
				m.logger.Debug("Sync complete at height %d", height)
				return height, nil
			}
		}
	}
}

// waitForNodeReady waits for the node to start responding to RPC requests.
func (m *SyncMonitor) waitForNodeReady(ctx context.Context) error {
	deadline := time.Now().Add(DefaultStartupWait)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if time.Now().After(deadline) {
				return fmt.Errorf("timeout waiting for node to start")
			}

			_, err := m.getStatus(ctx)
			if err == nil {
				return nil
			}
		}
	}
}

// getStatus fetches the status from the RPC endpoint.
func (m *SyncMonitor) getStatus(ctx context.Context) (*StatusResponse, error) {
	url := fmt.Sprintf("%s/status", m.rpcEndpoint)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status request failed: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var status StatusResponse
	if err := json.Unmarshal(body, &status); err != nil {
		return nil, err
	}

	return &status, nil
}

// GetCurrentHeight returns the current block height.
func (m *SyncMonitor) GetCurrentHeight(ctx context.Context) (int64, error) {
	status, err := m.getStatus(ctx)
	if err != nil {
		return 0, err
	}
	return parseHeight(status.Result.SyncInfo.LatestBlockHeight), nil
}

// IsSynced returns true if the node is fully synced.
func (m *SyncMonitor) IsSynced(ctx context.Context) (bool, error) {
	status, err := m.getStatus(ctx)
	if err != nil {
		return false, err
	}
	return !status.Result.SyncInfo.CatchingUp, nil
}

// parseHeight converts a string height to int64.
func parseHeight(s string) int64 {
	var height int64
	fmt.Sscanf(s, "%d", &height)
	return height
}

// WaitForHeight waits until the node reaches the specified height.
func (m *SyncMonitor) WaitForHeight(ctx context.Context, targetHeight int64) error {
	ticker := time.NewTicker(m.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			height, err := m.GetCurrentHeight(ctx)
			if err != nil {
				continue
			}

			if height >= targetHeight {
				return nil
			}

			m.logger.Debug("Waiting for height %d (current: %d)", targetHeight, height)
		}
	}
}
