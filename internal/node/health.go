package node

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

const (
	// HealthCheckTimeout is the timeout for health check requests.
	HealthCheckTimeout = 5 * time.Second

	// HealthCheckRetries is the number of retries for health checks.
	HealthCheckRetries = 3

	// HealthCheckInterval is the interval between health check retries.
	HealthCheckInterval = 1 * time.Second
)

// NodeHealth represents the health status of a node.
type NodeHealth struct {
	Index       int        `json:"index"`
	Name        string     `json:"name"`
	Status      NodeStatus `json:"status"`
	BlockHeight int64      `json:"block_height"`
	PeerCount   int        `json:"peer_count"`
	CatchingUp  bool       `json:"catching_up"`
	Error       string     `json:"error,omitempty"`
}

// RPCStatusResponse represents the RPC /status response.
type RPCStatusResponse struct {
	Result struct {
		SyncInfo struct {
			LatestBlockHeight string `json:"latest_block_height"`
			CatchingUp        bool   `json:"catching_up"`
		} `json:"sync_info"`
		NodeInfo struct {
			Network string `json:"network"`
			Moniker string `json:"moniker"`
		} `json:"node_info"`
	} `json:"result"`
}

// RPCNetInfoResponse represents the RPC /net_info response.
type RPCNetInfoResponse struct {
	Result struct {
		NPeers string `json:"n_peers"`
		Peers  []struct {
			NodeInfo struct {
				Moniker string `json:"moniker"`
			} `json:"node_info"`
		} `json:"peers"`
	} `json:"result"`
}

// CheckHealth checks the health of a node via its RPC endpoint.
func CheckHealth(ctx context.Context, node *Node) (*NodeHealth, error) {
	health := &NodeHealth{
		Index:  node.Index,
		Name:   node.Name,
		Status: NodeStatusStopped,
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: HealthCheckTimeout,
	}

	// Check /status endpoint
	statusURL := fmt.Sprintf("%s/status", node.RPCURL())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, statusURL, nil)
	if err != nil {
		health.Error = fmt.Sprintf("failed to create request: %v", err)
		return health, nil
	}

	resp, err := client.Do(req)
	if err != nil {
		health.Error = fmt.Sprintf("failed to connect: %v", err)
		return health, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		health.Error = fmt.Sprintf("RPC returned status %d", resp.StatusCode)
		return health, nil
	}

	var statusResp RPCStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&statusResp); err != nil {
		health.Error = fmt.Sprintf("failed to parse response: %v", err)
		return health, nil
	}

	// Parse block height
	if height, err := strconv.ParseInt(statusResp.Result.SyncInfo.LatestBlockHeight, 10, 64); err == nil {
		health.BlockHeight = height
	}

	health.CatchingUp = statusResp.Result.SyncInfo.CatchingUp

	// Check /net_info for peer count
	netInfoURL := fmt.Sprintf("%s/net_info", node.RPCURL())
	netReq, err := http.NewRequestWithContext(ctx, http.MethodGet, netInfoURL, nil)
	if err == nil {
		netResp, err := client.Do(netReq)
		if err == nil {
			defer netResp.Body.Close()
			var netInfo RPCNetInfoResponse
			if err := json.NewDecoder(netResp.Body).Decode(&netInfo); err == nil {
				if peers, err := strconv.Atoi(netInfo.Result.NPeers); err == nil {
					health.PeerCount = peers
				}
			}
		}
	}

	// Determine status
	if health.CatchingUp {
		health.Status = NodeStatusSyncing
	} else {
		health.Status = NodeStatusRunning
	}

	return health, nil
}

// WaitForHealthy waits for a node to become healthy.
func WaitForHealthy(ctx context.Context, node *Node, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		health, _ := CheckHealth(ctx, node)
		if health.Status == NodeStatusRunning || health.Status == NodeStatusSyncing {
			return nil
		}

		if time.Now().After(deadline) {
			if health.Error != "" {
				return fmt.Errorf("timeout waiting for node %s to become healthy: %s", node.Name, health.Error)
			}
			return fmt.Errorf("timeout waiting for node %s to become healthy", node.Name)
		}

		time.Sleep(HealthCheckInterval)
	}
}

// CheckAllNodesHealth checks the health of all nodes.
func CheckAllNodesHealth(ctx context.Context, nodes []*Node) []*NodeHealth {
	results := make([]*NodeHealth, len(nodes))

	for i, node := range nodes {
		health, _ := CheckHealth(ctx, node)
		results[i] = health
	}

	return results
}

// WaitForAllNodesHealthy waits for all nodes to become healthy.
func WaitForAllNodesHealthy(ctx context.Context, nodes []*Node, timeout time.Duration) error {
	for _, node := range nodes {
		if err := WaitForHealthy(ctx, node, timeout); err != nil {
			return err
		}
	}
	return nil
}

// IsNodeResponding checks if a node's RPC endpoint is responding.
func IsNodeResponding(ctx context.Context, node *Node) bool {
	client := &http.Client{
		Timeout: HealthCheckTimeout,
	}

	statusURL := fmt.Sprintf("%s/status", node.RPCURL())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, statusURL, nil)
	if err != nil {
		return false
	}

	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// GetBlockHeight returns the current block height of a node.
func GetBlockHeight(ctx context.Context, node *Node) (int64, error) {
	health, err := CheckHealth(ctx, node)
	if err != nil {
		return 0, err
	}
	if health.Error != "" {
		return 0, fmt.Errorf(health.Error)
	}
	return health.BlockHeight, nil
}

// GetPeerCount returns the number of peers connected to a node.
func GetPeerCount(ctx context.Context, node *Node) (int, error) {
	health, err := CheckHealth(ctx, node)
	if err != nil {
		return 0, err
	}
	if health.Error != "" {
		return 0, fmt.Errorf(health.Error)
	}
	return health.PeerCount, nil
}
