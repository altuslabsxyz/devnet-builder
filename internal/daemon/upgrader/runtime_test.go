package upgrader

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/daemon/store"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/types"
)

func TestRuntime_GetCurrentHeight(t *testing.T) {
	// Create mock RPC server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/status" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"result": map[string]interface{}{
					"sync_info": map[string]interface{}{
						"latest_block_height": "12345",
					},
				},
			})
		}
	}))
	defer server.Close()

	// Extract port
	port := strings.Split(server.URL, ":")[2]
	var rpcPort int
	fmt.Sscanf(port, "%d", &rpcPort)

	// Set up store with a running node
	s := store.NewMemoryStore()
	devnet := &types.Devnet{
		Metadata: types.ResourceMeta{Name: "test-devnet"},
		Spec:     types.DevnetSpec{Validators: 1},
	}
	s.CreateDevnet(context.Background(), devnet)

	node := &types.Node{
		Metadata: types.ResourceMeta{Name: "test-node-0"},
		Spec: types.NodeSpec{
			DevnetRef: "test-devnet",
			Index:     0,
		},
		Status: types.NodeStatus{
			Phase: types.NodePhaseRunning,
		},
	}
	s.CreateNode(context.Background(), node)

	runtime := NewRuntime(s, Config{
		BaseRPC: rpcPort, // Use mock server port
		Timeout: 5 * time.Second,
	})

	height, err := runtime.GetCurrentHeight(context.Background(), "test-devnet")
	if err != nil {
		t.Fatalf("GetCurrentHeight failed: %v", err)
	}

	if height != 12345 {
		t.Errorf("Expected height 12345, got %d", height)
	}
}

func TestRuntime_GetValidatorCount(t *testing.T) {
	s := store.NewMemoryStore()
	devnet := &types.Devnet{
		Metadata: types.ResourceMeta{Name: "test-devnet"},
		Spec: types.DevnetSpec{
			Validators: 4,
			FullNodes:  2,
		},
	}
	s.CreateDevnet(context.Background(), devnet)

	runtime := NewRuntime(s, Config{})

	count, err := runtime.GetValidatorCount(context.Background(), "test-devnet")
	if err != nil {
		t.Fatalf("GetValidatorCount failed: %v", err)
	}

	if count != 4 {
		t.Errorf("Expected 4 validators, got %d", count)
	}
}

func TestRuntime_SwitchNodeBinary(t *testing.T) {
	s := store.NewMemoryStore()

	// Create devnet and node
	devnet := &types.Devnet{
		Metadata: types.ResourceMeta{Name: "test-devnet"},
		Spec:     types.DevnetSpec{Validators: 1},
	}
	s.CreateDevnet(context.Background(), devnet)

	node := &types.Node{
		Metadata: types.ResourceMeta{Name: "test-node-0"},
		Spec: types.NodeSpec{
			DevnetRef:  "test-devnet",
			Index:      0,
			BinaryPath: "/old/binary",
		},
		Status: types.NodeStatus{
			Phase: types.NodePhaseRunning,
		},
	}
	s.CreateNode(context.Background(), node)

	runtime := NewRuntime(s, Config{})

	newBinary := types.BinarySource{
		Type: "local",
		Path: "/new/binary/v2.0.0",
	}

	err := runtime.SwitchNodeBinary(context.Background(), "test-devnet", 0, newBinary)
	if err != nil {
		t.Fatalf("SwitchNodeBinary failed: %v", err)
	}

	// Verify node was updated
	updated, _ := s.GetNode(context.Background(), "", "test-devnet", 0)

	if updated.Spec.BinaryPath != "/new/binary/v2.0.0" {
		t.Errorf("Expected new binary path, got %s", updated.Spec.BinaryPath)
	}

	if updated.Status.Phase != types.NodePhasePending {
		t.Errorf("Expected Pending phase for restart, got %s", updated.Status.Phase)
	}

	if updated.Status.RestartCount != 1 {
		t.Errorf("Expected restart count 1, got %d", updated.Status.RestartCount)
	}
}

func TestRuntime_SubmitUpgradeProposal(t *testing.T) {
	s := store.NewMemoryStore()
	runtime := NewRuntime(s, Config{})

	proposalID, err := runtime.SubmitUpgradeProposal(context.Background(), "test-devnet", "v2-upgrade", 1000)
	if err != nil {
		t.Fatalf("SubmitUpgradeProposal failed: %v", err)
	}

	if proposalID != 1 {
		t.Errorf("Expected proposal ID 1, got %d", proposalID)
	}
}

func TestRuntime_GetProposalStatus(t *testing.T) {
	s := store.NewMemoryStore()
	devnet := &types.Devnet{
		Metadata: types.ResourceMeta{Name: "test-devnet"},
		Spec:     types.DevnetSpec{Validators: 3},
	}
	s.CreateDevnet(context.Background(), devnet)

	runtime := NewRuntime(s, Config{})

	votesReceived, votesRequired, passed, err := runtime.GetProposalStatus(context.Background(), "test-devnet", 1)
	if err != nil {
		t.Fatalf("GetProposalStatus failed: %v", err)
	}

	// Simulated: all validators voted
	if votesReceived != 3 {
		t.Errorf("Expected 3 votes received, got %d", votesReceived)
	}
	if votesRequired != 3 {
		t.Errorf("Expected 3 votes required, got %d", votesRequired)
	}
	if !passed {
		t.Error("Expected proposal to pass")
	}
}

func TestRuntime_VerifyNodeVersion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/abci_info" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"result": map[string]interface{}{
					"response": map[string]interface{}{
						"version": "2.0.0",
					},
				},
			})
		}
	}))
	defer server.Close()

	port := strings.Split(server.URL, ":")[2]
	var rpcPort int
	fmt.Sscanf(port, "%d", &rpcPort)

	s := store.NewMemoryStore()
	devnet := &types.Devnet{
		Metadata: types.ResourceMeta{Name: "test-devnet"},
		Spec:     types.DevnetSpec{Validators: 1},
	}
	s.CreateDevnet(context.Background(), devnet)

	node := &types.Node{
		Metadata: types.ResourceMeta{Name: "test-node-0"},
		Spec: types.NodeSpec{
			DevnetRef: "test-devnet",
			Index:     0,
		},
		Status: types.NodeStatus{
			Phase: types.NodePhaseRunning,
		},
	}
	s.CreateNode(context.Background(), node)

	runtime := NewRuntime(s, Config{BaseRPC: rpcPort})

	verified, err := runtime.VerifyNodeVersion(context.Background(), "test-devnet", 0, "2.0.0")
	if err != nil {
		t.Fatalf("VerifyNodeVersion failed: %v", err)
	}

	if !verified {
		t.Error("Expected version to be verified")
	}

	// Test version mismatch
	verified, err = runtime.VerifyNodeVersion(context.Background(), "test-devnet", 0, "1.0.0")
	if err != nil {
		t.Fatalf("VerifyNodeVersion failed: %v", err)
	}

	if verified {
		t.Error("Expected version mismatch")
	}
}
