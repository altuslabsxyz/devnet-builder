package server

import (
	"context"
	"testing"

	v1 "github.com/altuslabsxyz/devnet-builder/api/proto/gen/v1"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/store"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestNodeService_GetNode(t *testing.T) {
	s := store.NewMemoryStore()
	svc := NewNodeService(s, nil, nil)

	// Create a node
	node := &types.Node{
		Metadata: types.ResourceMeta{Name: "test-0"},
		Spec:     types.NodeSpec{DevnetRef: "test-devnet", Index: 0, Role: "validator"},
		Status:   types.NodeStatus{Phase: types.NodePhaseRunning},
	}
	if err := s.CreateNode(context.Background(), node); err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	// Get it
	resp, err := svc.GetNode(context.Background(), &v1.GetNodeRequest{
		DevnetName: "test-devnet",
		Index:      0,
	})
	if err != nil {
		t.Fatalf("GetNode failed: %v", err)
	}

	if resp.Node.Metadata.DevnetName != "test-devnet" {
		t.Errorf("DevnetName = %q, want %q", resp.Node.Metadata.DevnetName, "test-devnet")
	}
	if resp.Node.Metadata.Index != 0 {
		t.Errorf("Index = %d, want 0", resp.Node.Metadata.Index)
	}
	if resp.Node.Spec.Role != "validator" {
		t.Errorf("Role = %q, want %q", resp.Node.Spec.Role, "validator")
	}
	if resp.Node.Status.Phase != types.NodePhaseRunning {
		t.Errorf("Phase = %q, want %q", resp.Node.Status.Phase, types.NodePhaseRunning)
	}
}

func TestNodeService_GetNode_NotFound(t *testing.T) {
	s := store.NewMemoryStore()
	svc := NewNodeService(s, nil, nil)

	_, err := svc.GetNode(context.Background(), &v1.GetNodeRequest{
		DevnetName: "nonexistent",
		Index:      0,
	})
	if err == nil {
		t.Fatal("expected error for nonexistent node")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %v", err)
	}
	if st.Code() != codes.NotFound {
		t.Errorf("expected NotFound, got %v", st.Code())
	}
}

func TestNodeService_GetNode_MissingDevnetName(t *testing.T) {
	s := store.NewMemoryStore()
	svc := NewNodeService(s, nil, nil)

	_, err := svc.GetNode(context.Background(), &v1.GetNodeRequest{
		DevnetName: "",
		Index:      0,
	})
	if err == nil {
		t.Fatal("expected error for missing devnet_name")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %v", err)
	}
	if st.Code() != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", st.Code())
	}
}

func TestNodeService_ListNodes(t *testing.T) {
	s := store.NewMemoryStore()
	svc := NewNodeService(s, nil, nil)

	// Create devnets first (required for ListNodes)
	devnetA := &types.Devnet{
		Metadata: types.ResourceMeta{Name: "devnet-a"},
		Spec:     types.DevnetSpec{Plugin: "stable", Validators: 3},
	}
	devnetB := &types.Devnet{
		Metadata: types.ResourceMeta{Name: "devnet-b"},
		Spec:     types.DevnetSpec{Plugin: "stable", Validators: 1},
	}
	if err := s.CreateDevnet(context.Background(), devnetA); err != nil {
		t.Fatalf("CreateDevnet: %v", err)
	}
	if err := s.CreateDevnet(context.Background(), devnetB); err != nil {
		t.Fatalf("CreateDevnet: %v", err)
	}

	// Create nodes for two devnets
	nodes := []*types.Node{
		{Spec: types.NodeSpec{DevnetRef: "devnet-a", Index: 0, Role: "validator"}},
		{Spec: types.NodeSpec{DevnetRef: "devnet-a", Index: 1, Role: "validator"}},
		{Spec: types.NodeSpec{DevnetRef: "devnet-a", Index: 2, Role: "fullnode"}},
		{Spec: types.NodeSpec{DevnetRef: "devnet-b", Index: 0, Role: "validator"}},
	}
	for _, n := range nodes {
		if err := s.CreateNode(context.Background(), n); err != nil {
			t.Fatalf("CreateNode: %v", err)
		}
	}

	// List devnet-a nodes
	resp, err := svc.ListNodes(context.Background(), &v1.ListNodesRequest{
		DevnetName: "devnet-a",
	})
	if err != nil {
		t.Fatalf("ListNodes failed: %v", err)
	}

	if len(resp.Nodes) != 3 {
		t.Errorf("got %d nodes, want 3", len(resp.Nodes))
	}

	// List devnet-b nodes
	resp, err = svc.ListNodes(context.Background(), &v1.ListNodesRequest{
		DevnetName: "devnet-b",
	})
	if err != nil {
		t.Fatalf("ListNodes failed: %v", err)
	}

	if len(resp.Nodes) != 1 {
		t.Errorf("got %d nodes, want 1", len(resp.Nodes))
	}
}

func TestNodeService_ListNodes_DevnetNotFound(t *testing.T) {
	s := store.NewMemoryStore()
	svc := NewNodeService(s, nil, nil)

	// List nodes for nonexistent devnet - should return NotFound
	_, err := svc.ListNodes(context.Background(), &v1.ListNodesRequest{
		DevnetName: "nonexistent",
	})
	if err == nil {
		t.Fatal("expected error for nonexistent devnet")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %v", err)
	}
	if st.Code() != codes.NotFound {
		t.Errorf("expected NotFound, got %v", st.Code())
	}
}

func TestNodeService_StartNode(t *testing.T) {
	s := store.NewMemoryStore()
	svc := NewNodeService(s, nil, nil)

	// Create a stopped node
	node := &types.Node{
		Metadata: types.ResourceMeta{Name: "test-0"},
		Spec:     types.NodeSpec{DevnetRef: "test-devnet", Index: 0, Role: "validator", Desired: types.NodePhaseStopped},
		Status:   types.NodeStatus{Phase: types.NodePhaseStopped},
	}
	if err := s.CreateNode(context.Background(), node); err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	// Start it
	resp, err := svc.StartNode(context.Background(), &v1.StartNodeRequest{
		DevnetName: "test-devnet",
		Index:      0,
	})
	if err != nil {
		t.Fatalf("StartNode failed: %v", err)
	}

	// Should transition to Pending
	if resp.Node.Status.Phase != types.NodePhasePending {
		t.Errorf("Phase = %q, want %q", resp.Node.Status.Phase, types.NodePhasePending)
	}
	if resp.Node.Spec.DesiredPhase != types.NodePhaseRunning {
		t.Errorf("DesiredPhase = %q, want %q", resp.Node.Spec.DesiredPhase, types.NodePhaseRunning)
	}
}

func TestNodeService_StartNode_AlreadyRunning(t *testing.T) {
	s := store.NewMemoryStore()
	svc := NewNodeService(s, nil, nil)

	// Create a running node
	node := &types.Node{
		Metadata: types.ResourceMeta{Name: "test-0"},
		Spec:     types.NodeSpec{DevnetRef: "test-devnet", Index: 0, Role: "validator"},
		Status:   types.NodeStatus{Phase: types.NodePhaseRunning},
	}
	if err := s.CreateNode(context.Background(), node); err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	// Try to start it
	_, err := svc.StartNode(context.Background(), &v1.StartNodeRequest{
		DevnetName: "test-devnet",
		Index:      0,
	})
	if err == nil {
		t.Fatal("expected error for already running node")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %v", err)
	}
	if st.Code() != codes.FailedPrecondition {
		t.Errorf("expected FailedPrecondition, got %v", st.Code())
	}
}

func TestNodeService_StopNode(t *testing.T) {
	s := store.NewMemoryStore()
	svc := NewNodeService(s, nil, nil)

	// Create a running node
	node := &types.Node{
		Metadata: types.ResourceMeta{Name: "test-0"},
		Spec:     types.NodeSpec{DevnetRef: "test-devnet", Index: 0, Role: "validator"},
		Status:   types.NodeStatus{Phase: types.NodePhaseRunning},
	}
	if err := s.CreateNode(context.Background(), node); err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	// Stop it
	resp, err := svc.StopNode(context.Background(), &v1.StopNodeRequest{
		DevnetName: "test-devnet",
		Index:      0,
	})
	if err != nil {
		t.Fatalf("StopNode failed: %v", err)
	}

	// Should transition to Stopping
	if resp.Node.Status.Phase != types.NodePhaseStopping {
		t.Errorf("Phase = %q, want %q", resp.Node.Status.Phase, types.NodePhaseStopping)
	}
	if resp.Node.Spec.DesiredPhase != types.NodePhaseStopped {
		t.Errorf("DesiredPhase = %q, want %q", resp.Node.Spec.DesiredPhase, types.NodePhaseStopped)
	}
}

func TestNodeService_StopNode_AlreadyStopped(t *testing.T) {
	s := store.NewMemoryStore()
	svc := NewNodeService(s, nil, nil)

	// Create a stopped node
	node := &types.Node{
		Metadata: types.ResourceMeta{Name: "test-0"},
		Spec:     types.NodeSpec{DevnetRef: "test-devnet", Index: 0, Role: "validator"},
		Status:   types.NodeStatus{Phase: types.NodePhaseStopped},
	}
	if err := s.CreateNode(context.Background(), node); err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	// Try to stop it
	_, err := svc.StopNode(context.Background(), &v1.StopNodeRequest{
		DevnetName: "test-devnet",
		Index:      0,
	})
	if err == nil {
		t.Fatal("expected error for already stopped node")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %v", err)
	}
	if st.Code() != codes.FailedPrecondition {
		t.Errorf("expected FailedPrecondition, got %v", st.Code())
	}
}

func TestNodeService_RestartNode(t *testing.T) {
	s := store.NewMemoryStore()
	svc := NewNodeService(s, nil, nil)

	// Create a running node
	node := &types.Node{
		Metadata: types.ResourceMeta{Name: "test-0"},
		Spec:     types.NodeSpec{DevnetRef: "test-devnet", Index: 0, Role: "validator"},
		Status:   types.NodeStatus{Phase: types.NodePhaseRunning, RestartCount: 1},
	}
	if err := s.CreateNode(context.Background(), node); err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	// Restart it
	resp, err := svc.RestartNode(context.Background(), &v1.RestartNodeRequest{
		DevnetName: "test-devnet",
		Index:      0,
	})
	if err != nil {
		t.Fatalf("RestartNode failed: %v", err)
	}

	// Should transition to Pending and increment restart count
	if resp.Node.Status.Phase != types.NodePhasePending {
		t.Errorf("Phase = %q, want %q", resp.Node.Status.Phase, types.NodePhasePending)
	}
	if resp.Node.Status.RestartCount != 2 {
		t.Errorf("RestartCount = %d, want 2", resp.Node.Status.RestartCount)
	}
}

func TestNodeService_GetNodeHealth(t *testing.T) {
	s := store.NewMemoryStore()
	svc := NewNodeService(s, nil, nil)

	tests := []struct {
		name           string
		phase          string
		expectedStatus string
	}{
		{"running node", types.NodePhaseRunning, "Healthy"},
		{"crashed node", types.NodePhaseCrashed, "Unhealthy"},
		{"stopped node", types.NodePhaseStopped, "Stopped"},
		{"pending node", types.NodePhasePending, "Transitioning"},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := &types.Node{
				Metadata: types.ResourceMeta{Name: "test-" + string(rune('0'+i))},
				Spec:     types.NodeSpec{DevnetRef: "health-test", Index: i, Role: "validator"},
				Status:   types.NodeStatus{Phase: tt.phase},
			}
			if err := s.CreateNode(context.Background(), node); err != nil {
				t.Fatalf("CreateNode: %v", err)
			}

			resp, err := svc.GetNodeHealth(context.Background(), &v1.GetNodeHealthRequest{
				DevnetName: "health-test",
				Index:      int32(i),
			})
			if err != nil {
				t.Fatalf("GetNodeHealth failed: %v", err)
			}

			if resp.Health.Status != tt.expectedStatus {
				t.Errorf("Health.Status = %q, want %q", resp.Health.Status, tt.expectedStatus)
			}
		})
	}
}

func TestNodeService_GetNodeHealth_NotFound(t *testing.T) {
	s := store.NewMemoryStore()
	svc := NewNodeService(s, nil, nil)

	_, err := svc.GetNodeHealth(context.Background(), &v1.GetNodeHealthRequest{
		DevnetName: "nonexistent",
		Index:      0,
	})
	if err == nil {
		t.Fatal("expected error for nonexistent node")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %v", err)
	}
	if st.Code() != codes.NotFound {
		t.Errorf("expected NotFound, got %v", st.Code())
	}
}

func TestNodeService_GetNodeHealth_MissingDevnetName(t *testing.T) {
	s := store.NewMemoryStore()
	svc := NewNodeService(s, nil, nil)

	_, err := svc.GetNodeHealth(context.Background(), &v1.GetNodeHealthRequest{
		DevnetName: "",
		Index:      0,
	})
	if err == nil {
		t.Fatal("expected error for missing devnet_name")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %v", err)
	}
	if st.Code() != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", st.Code())
	}
}

func TestNodeToProto(t *testing.T) {
	node := &types.Node{
		Metadata: types.ResourceMeta{
			Name:       "test-node",
			Generation: 5,
		},
		Spec: types.NodeSpec{
			DevnetRef:  "my-devnet",
			Index:      2,
			Role:       "fullnode",
			BinaryPath: "/usr/bin/node",
			HomeDir:    "/data/node",
			Desired:    types.NodePhaseRunning,
		},
		Status: types.NodeStatus{
			Phase:        types.NodePhaseRunning,
			PID:          12345,
			BlockHeight:  100000,
			PeerCount:    10,
			CatchingUp:   false,
			RestartCount: 3,
			Message:      "Running smoothly",
		},
	}

	pb := NodeToProto(node)

	if pb.Metadata.DevnetName != "my-devnet" {
		t.Errorf("Metadata.DevnetName = %q, want %q", pb.Metadata.DevnetName, "my-devnet")
	}
	if pb.Metadata.Index != 2 {
		t.Errorf("Metadata.Index = %d, want 2", pb.Metadata.Index)
	}
	if pb.Metadata.Generation != 5 {
		t.Errorf("Metadata.Generation = %d, want 5", pb.Metadata.Generation)
	}
	if pb.Spec.Role != "fullnode" {
		t.Errorf("Spec.Role = %q, want %q", pb.Spec.Role, "fullnode")
	}
	if pb.Spec.BinaryPath != "/usr/bin/node" {
		t.Errorf("Spec.BinaryPath = %q, want %q", pb.Spec.BinaryPath, "/usr/bin/node")
	}
	if pb.Spec.DesiredPhase != types.NodePhaseRunning {
		t.Errorf("Spec.DesiredPhase = %q, want %q", pb.Spec.DesiredPhase, types.NodePhaseRunning)
	}
	if pb.Status.Phase != types.NodePhaseRunning {
		t.Errorf("Status.Phase = %q, want %q", pb.Status.Phase, types.NodePhaseRunning)
	}
	// ContainerId is no longer populated (runtime tracks internally)
	if pb.Status.RestartCount != 3 {
		t.Errorf("Status.RestartCount = %d, want 3", pb.Status.RestartCount)
	}
}

func TestNodeToProto_Nil(t *testing.T) {
	if NodeToProto(nil) != nil {
		t.Error("NodeToProto(nil) should return nil")
	}
}

func TestNodeService_GetNodePorts(t *testing.T) {
	s := store.NewMemoryStore()
	svc := NewNodeService(s, nil, nil)

	tests := []struct {
		name        string
		nodeIndex   int
		expectedP2P int32
		expectedRPC int32
	}{
		{"node 0", 0, 26656, 26657},
		{"node 1", 1, 26756, 26757},
		{"node 5", 5, 27156, 27157},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a node with the specified index
			node := &types.Node{
				Metadata: types.ResourceMeta{Name: "test-" + string(rune('0'+tt.nodeIndex))},
				Spec:     types.NodeSpec{DevnetRef: "port-test", Index: tt.nodeIndex, Role: "validator"},
				Status:   types.NodeStatus{Phase: types.NodePhaseRunning},
			}
			if err := s.CreateNode(context.Background(), node); err != nil {
				t.Fatalf("CreateNode: %v", err)
			}

			resp, err := svc.GetNodePorts(context.Background(), &v1.GetNodePortsRequest{
				DevnetName: "port-test",
				Index:      int32(tt.nodeIndex),
			})
			if err != nil {
				t.Fatalf("GetNodePorts failed: %v", err)
			}

			if resp.DevnetName != "port-test" {
				t.Errorf("DevnetName = %q, want %q", resp.DevnetName, "port-test")
			}
			if resp.Index != int32(tt.nodeIndex) {
				t.Errorf("Index = %d, want %d", resp.Index, tt.nodeIndex)
			}
			if len(resp.Ports) != 4 {
				t.Fatalf("got %d ports, want 4", len(resp.Ports))
			}

			// Check P2P port
			var foundP2P, foundRPC bool
			for _, p := range resp.Ports {
				if p.Name == "p2p" {
					foundP2P = true
					if p.HostPort != tt.expectedP2P {
						t.Errorf("P2P HostPort = %d, want %d", p.HostPort, tt.expectedP2P)
					}
					if p.ContainerPort != 26656 {
						t.Errorf("P2P ContainerPort = %d, want 26656", p.ContainerPort)
					}
					if p.Protocol != "tcp" {
						t.Errorf("P2P Protocol = %q, want %q", p.Protocol, "tcp")
					}
				}
				if p.Name == "rpc" {
					foundRPC = true
					if p.HostPort != tt.expectedRPC {
						t.Errorf("RPC HostPort = %d, want %d", p.HostPort, tt.expectedRPC)
					}
				}
			}
			if !foundP2P {
				t.Error("P2P port not found in response")
			}
			if !foundRPC {
				t.Error("RPC port not found in response")
			}
		})
	}
}

func TestNodeService_GetNodePorts_NotFound(t *testing.T) {
	s := store.NewMemoryStore()
	svc := NewNodeService(s, nil, nil)

	_, err := svc.GetNodePorts(context.Background(), &v1.GetNodePortsRequest{
		DevnetName: "nonexistent",
		Index:      0,
	})
	if err == nil {
		t.Fatal("expected error for nonexistent node")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %v", err)
	}
	if st.Code() != codes.NotFound {
		t.Errorf("expected NotFound, got %v", st.Code())
	}
}

func TestNodeService_GetNodePorts_MissingDevnetName(t *testing.T) {
	s := store.NewMemoryStore()
	svc := NewNodeService(s, nil, nil)

	_, err := svc.GetNodePorts(context.Background(), &v1.GetNodePortsRequest{
		DevnetName: "",
		Index:      0,
	})
	if err == nil {
		t.Fatal("expected error for missing devnet_name")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %v", err)
	}
	if st.Code() != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", st.Code())
	}
}
