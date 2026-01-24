package checker

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/daemon/types"
)

func TestRPCHealthChecker_CheckHealth_Success(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/status" {
			resp := CometBFTStatusResponse{}
			resp.Result.SyncInfo.LatestBlockHeight = 12345
			resp.Result.SyncInfo.CatchingUp = false
			resp.Result.NodeInfo.Moniker = "test-node"
			json.NewEncoder(w).Encode(resp)
		} else if r.URL.Path == "/net_info" {
			w.Write([]byte(`{"result":{"listening":true,"n_peers":"3","peers":[]}}`))
		}
	}))
	defer server.Close()

	// Extract port from mock server
	port := strings.Split(server.URL, ":")[2]
	var rpcPort int
	_, _ = fmt.Sscanf(port, "%d", &rpcPort)

	checker := NewRPCHealthChecker(Config{
		Timeout: 5 * time.Second,
		BaseRPC: rpcPort, // Use mock server port directly
	})

	node := &types.Node{
		Spec: types.NodeSpec{
			DevnetRef: "test",
			Index:     0, // Index 0 so port matches
		},
		Status: types.NodeStatus{
			Phase: types.NodePhaseRunning,
		},
	}

	result, err := checker.CheckHealth(context.Background(), node)
	if err != nil {
		t.Fatalf("CheckHealth failed: %v", err)
	}

	if !result.Healthy {
		t.Error("Expected node to be healthy")
	}
	if result.BlockHeight != 12345 {
		t.Errorf("Expected block height 12345, got %d", result.BlockHeight)
	}
	if result.CatchingUp {
		t.Error("Expected node not catching up")
	}
	if result.PeerCount != 3 {
		t.Errorf("Expected 3 peers, got %d", result.PeerCount)
	}
}

func TestRPCHealthChecker_CheckHealth_ConnectionFailed(t *testing.T) {
	// Use a port that nothing is listening on
	checker := NewRPCHealthChecker(Config{
		Timeout: 100 * time.Millisecond,
		BaseRPC: 59999, // Unlikely to be in use
	})

	node := &types.Node{
		Spec: types.NodeSpec{
			DevnetRef: "test",
			Index:     0,
		},
		Status: types.NodeStatus{
			Phase: types.NodePhaseRunning,
		},
	}

	result, err := checker.CheckHealth(context.Background(), node)
	if err != nil {
		t.Fatalf("CheckHealth should not return error, got: %v", err)
	}

	if result.Healthy {
		t.Error("Expected node to be unhealthy when connection fails")
	}
	if result.Error == "" {
		t.Error("Expected error message when connection fails")
	}
}

func TestRPCHealthChecker_CheckHealth_CatchingUp(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/status" {
			resp := CometBFTStatusResponse{}
			resp.Result.SyncInfo.LatestBlockHeight = 100
			resp.Result.SyncInfo.CatchingUp = true
			json.NewEncoder(w).Encode(resp)
		} else if r.URL.Path == "/net_info" {
			w.Write([]byte(`{"result":{"listening":true,"n_peers":"1","peers":[]}}`))
		}
	}))
	defer server.Close()

	port := strings.Split(server.URL, ":")[2]
	var rpcPort int
	_, _ = fmt.Sscanf(port, "%d", &rpcPort)

	checker := NewRPCHealthChecker(Config{
		Timeout: 5 * time.Second,
		BaseRPC: rpcPort,
	})

	node := &types.Node{
		Spec: types.NodeSpec{
			DevnetRef: "test",
			Index:     0,
		},
	}

	result, _ := checker.CheckHealth(context.Background(), node)

	if !result.Healthy {
		t.Error("Node should be healthy even when catching up")
	}
	if !result.CatchingUp {
		t.Error("Expected CatchingUp to be true")
	}
	if result.BlockHeight != 100 {
		t.Errorf("Expected height 100, got %d", result.BlockHeight)
	}
}

func TestRPCHealthChecker_CheckHealth_BadStatusCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	port := strings.Split(server.URL, ":")[2]
	var rpcPort int
	_, _ = fmt.Sscanf(port, "%d", &rpcPort)

	checker := NewRPCHealthChecker(Config{
		Timeout: 5 * time.Second,
		BaseRPC: rpcPort,
	})

	node := &types.Node{
		Spec: types.NodeSpec{
			DevnetRef: "test",
			Index:     0,
		},
	}

	result, _ := checker.CheckHealth(context.Background(), node)

	if result.Healthy {
		t.Error("Expected unhealthy result for 500 status")
	}
	if !strings.Contains(result.Error, "500") {
		t.Errorf("Expected error to mention status code, got: %s", result.Error)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Timeout != 5*time.Second {
		t.Errorf("Expected timeout 5s, got %v", cfg.Timeout)
	}
	if cfg.BaseRPC != 26657 {
		t.Errorf("Expected base RPC 26657, got %d", cfg.BaseRPC)
	}
}
