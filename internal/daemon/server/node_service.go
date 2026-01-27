package server

import (
	"context"
	"log/slog"

	v1 "github.com/altuslabsxyz/devnet-builder/api/proto/gen/v1"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/controller"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/store"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// NodeService implements the gRPC NodeServiceServer.
type NodeService struct {
	v1.UnimplementedNodeServiceServer
	store   store.Store
	manager *controller.Manager
	logger  *slog.Logger
}

// NewNodeService creates a new NodeService.
func NewNodeService(s store.Store, m *controller.Manager) *NodeService {
	return &NodeService{
		store:   s,
		manager: m,
		logger:  slog.Default(),
	}
}

// SetLogger sets the logger for the service.
func (s *NodeService) SetLogger(logger *slog.Logger) {
	s.logger = logger
}

// GetNode retrieves a node by devnet name and index.
func (s *NodeService) GetNode(ctx context.Context, req *v1.GetNodeRequest) (*v1.GetNodeResponse, error) {
	if req.DevnetName == "" {
		return nil, status.Error(codes.InvalidArgument, "devnet_name is required")
	}

	// Use namespace from request, empty string uses default namespace
	namespace := req.GetNamespace()

	node, err := s.store.GetNode(ctx, namespace, req.DevnetName, int(req.Index))
	if err != nil {
		if store.IsNotFound(err) {
			return nil, status.Errorf(codes.NotFound, "node %s/%d not found", req.DevnetName, req.Index)
		}
		s.logger.Error("failed to get node", "devnet", req.DevnetName, "index", req.Index, "error", err)
		return nil, status.Errorf(codes.Internal, "failed to get node: %v", err)
	}

	return &v1.GetNodeResponse{Node: NodeToProto(node)}, nil
}

// ListNodes lists all nodes for a devnet.
func (s *NodeService) ListNodes(ctx context.Context, req *v1.ListNodesRequest) (*v1.ListNodesResponse, error) {
	if req.DevnetName == "" {
		return nil, status.Error(codes.InvalidArgument, "devnet_name is required")
	}

	// Use namespace from request, empty string uses default namespace
	namespace := req.GetNamespace()

	// Verify the devnet exists first
	_, err := s.store.GetDevnet(ctx, namespace, req.DevnetName)
	if err != nil {
		if store.IsNotFound(err) {
			return nil, status.Errorf(codes.NotFound, "devnet %q not found", req.DevnetName)
		}
		s.logger.Error("failed to get devnet", "name", req.DevnetName, "error", err)
		return nil, status.Errorf(codes.Internal, "failed to get devnet: %v", err)
	}

	nodes, err := s.store.ListNodes(ctx, namespace, req.DevnetName)
	if err != nil {
		s.logger.Error("failed to list nodes", "devnet", req.DevnetName, "error", err)
		return nil, status.Errorf(codes.Internal, "failed to list nodes: %v", err)
	}

	resp := &v1.ListNodesResponse{
		Nodes: make([]*v1.Node, 0, len(nodes)),
	}
	for _, n := range nodes {
		resp.Nodes = append(resp.Nodes, NodeToProto(n))
	}

	return resp, nil
}

// StartNode starts a stopped node.
func (s *NodeService) StartNode(ctx context.Context, req *v1.StartNodeRequest) (*v1.StartNodeResponse, error) {
	if req.DevnetName == "" {
		return nil, status.Error(codes.InvalidArgument, "devnet_name is required")
	}

	// Use namespace from request, default if empty
	namespace := req.GetNamespace()
	if namespace == "" {
		namespace = types.DefaultNamespace
	}

	s.logger.Info("starting node", "namespace", namespace, "devnet", req.DevnetName, "index", req.Index)

	node, err := s.store.GetNode(ctx, namespace, req.DevnetName, int(req.Index))
	if err != nil {
		if store.IsNotFound(err) {
			return nil, status.Errorf(codes.NotFound, "node %s/%d not found", req.DevnetName, req.Index)
		}
		return nil, status.Errorf(codes.Internal, "failed to get node: %v", err)
	}

	// Check if already running
	if node.Status.Phase == types.NodePhaseRunning || node.Status.Phase == types.NodePhaseStarting {
		return nil, status.Errorf(codes.FailedPrecondition, "node is already %s", node.Status.Phase)
	}

	// Set desired state to Running and transition to Pending for reconciliation
	node.Spec.Desired = types.NodePhaseRunning
	node.Status.Phase = types.NodePhasePending
	node.Status.Message = "Starting node"

	if err := s.store.UpdateNode(ctx, node); err != nil {
		s.logger.Error("failed to update node", "devnet", req.DevnetName, "index", req.Index, "error", err)
		return nil, status.Errorf(codes.Internal, "failed to update node: %v", err)
	}

	// Enqueue for reconciliation with namespace-aware key
	if s.manager != nil {
		s.manager.Enqueue("nodes", controller.NodeKeyWithNamespace(namespace, req.DevnetName, int(req.Index)))
	}

	return &v1.StartNodeResponse{Node: NodeToProto(node)}, nil
}

// StopNode stops a running node.
func (s *NodeService) StopNode(ctx context.Context, req *v1.StopNodeRequest) (*v1.StopNodeResponse, error) {
	if req.DevnetName == "" {
		return nil, status.Error(codes.InvalidArgument, "devnet_name is required")
	}

	// Use namespace from request, default if empty
	namespace := req.GetNamespace()
	if namespace == "" {
		namespace = types.DefaultNamespace
	}

	s.logger.Info("stopping node", "namespace", namespace, "devnet", req.DevnetName, "index", req.Index)

	node, err := s.store.GetNode(ctx, namespace, req.DevnetName, int(req.Index))
	if err != nil {
		if store.IsNotFound(err) {
			return nil, status.Errorf(codes.NotFound, "node %s/%d not found", req.DevnetName, req.Index)
		}
		return nil, status.Errorf(codes.Internal, "failed to get node: %v", err)
	}

	// Check if already stopped
	if node.Status.Phase == types.NodePhaseStopped || node.Status.Phase == types.NodePhaseStopping {
		return nil, status.Errorf(codes.FailedPrecondition, "node is already %s", node.Status.Phase)
	}

	// Set desired state to Stopped
	node.Spec.Desired = types.NodePhaseStopped
	node.Status.Phase = types.NodePhaseStopping
	node.Status.Message = "Stopping node"

	if err := s.store.UpdateNode(ctx, node); err != nil {
		s.logger.Error("failed to update node", "devnet", req.DevnetName, "index", req.Index, "error", err)
		return nil, status.Errorf(codes.Internal, "failed to update node: %v", err)
	}

	// Enqueue for reconciliation with namespace-aware key
	if s.manager != nil {
		s.manager.Enqueue("nodes", controller.NodeKeyWithNamespace(namespace, req.DevnetName, int(req.Index)))
	}

	return &v1.StopNodeResponse{Node: NodeToProto(node)}, nil
}

// RestartNode restarts a node.
func (s *NodeService) RestartNode(ctx context.Context, req *v1.RestartNodeRequest) (*v1.RestartNodeResponse, error) {
	if req.DevnetName == "" {
		return nil, status.Error(codes.InvalidArgument, "devnet_name is required")
	}

	// Use namespace from request, default if empty
	namespace := req.GetNamespace()
	if namespace == "" {
		namespace = types.DefaultNamespace
	}

	s.logger.Info("restarting node", "namespace", namespace, "devnet", req.DevnetName, "index", req.Index)

	node, err := s.store.GetNode(ctx, namespace, req.DevnetName, int(req.Index))
	if err != nil {
		if store.IsNotFound(err) {
			return nil, status.Errorf(codes.NotFound, "node %s/%d not found", req.DevnetName, req.Index)
		}
		return nil, status.Errorf(codes.Internal, "failed to get node: %v", err)
	}

	// Set desired state to Running and transition to Pending for restart
	node.Spec.Desired = types.NodePhaseRunning
	node.Status.Phase = types.NodePhasePending
	node.Status.Message = "Restarting node"
	node.Status.RestartCount++

	if err := s.store.UpdateNode(ctx, node); err != nil {
		s.logger.Error("failed to update node", "devnet", req.DevnetName, "index", req.Index, "error", err)
		return nil, status.Errorf(codes.Internal, "failed to update node: %v", err)
	}

	// Enqueue for reconciliation with namespace-aware key
	if s.manager != nil {
		s.manager.Enqueue("nodes", controller.NodeKeyWithNamespace(namespace, req.DevnetName, int(req.Index)))
	}

	return &v1.RestartNodeResponse{Node: NodeToProto(node)}, nil
}

// GetNodeHealth retrieves the health status of a node.
func (s *NodeService) GetNodeHealth(ctx context.Context, req *v1.GetNodeHealthRequest) (*v1.GetNodeHealthResponse, error) {
	if req.DevnetName == "" {
		return nil, status.Error(codes.InvalidArgument, "devnet_name is required")
	}

	// Use namespace from request, empty string uses default namespace
	namespace := req.GetNamespace()

	node, err := s.store.GetNode(ctx, namespace, req.DevnetName, int(req.Index))
	if err != nil {
		if store.IsNotFound(err) {
			return nil, status.Errorf(codes.NotFound, "node %s/%d not found", req.DevnetName, req.Index)
		}
		s.logger.Error("failed to get node", "devnet", req.DevnetName, "index", req.Index, "error", err)
		return nil, status.Errorf(codes.Internal, "failed to get node: %v", err)
	}

	// Derive health from phase
	healthStatus := "Unknown"
	healthMessage := ""

	switch node.Status.Phase {
	case types.NodePhaseRunning:
		healthStatus = "Healthy"
		healthMessage = "Node is running normally"
	case types.NodePhaseCrashed:
		healthStatus = "Unhealthy"
		healthMessage = node.Status.Message
	case types.NodePhaseStopped:
		healthStatus = "Stopped"
		healthMessage = "Node is stopped"
	default:
		healthStatus = "Transitioning"
		healthMessage = "Node is in " + node.Status.Phase + " phase"
	}

	return &v1.GetNodeHealthResponse{
		Health: &v1.NodeHealth{
			Status:  healthStatus,
			Message: healthMessage,
		},
	}, nil
}

// NodeToProto converts a domain Node to a proto Node.
func NodeToProto(n *types.Node) *v1.Node {
	if n == nil {
		return nil
	}

	return &v1.Node{
		Metadata: &v1.NodeMetadata{
			Id:         n.Metadata.Name,
			Namespace:  n.Metadata.Namespace,
			DevnetName: n.Spec.DevnetRef,
			Index:      int32(n.Spec.Index),
			Generation: n.Metadata.Generation,
			CreatedAt:  timestamppb.New(n.Metadata.CreatedAt),
			UpdatedAt:  timestamppb.New(n.Metadata.UpdatedAt),
		},
		Spec: &v1.NodeSpec{
			Role:         n.Spec.Role,
			BinaryPath:   n.Spec.BinaryPath,
			HomeDir:      n.Spec.HomeDir,
			DesiredPhase: n.Spec.Desired,
		},
		Status: &v1.NodeStatus{
			Phase:        n.Status.Phase,
			Pid:          int32(n.Status.PID),
			BlockHeight:  n.Status.BlockHeight,
			PeerCount:    int32(n.Status.PeerCount),
			CatchingUp:   n.Status.CatchingUp,
			RestartCount: int32(n.Status.RestartCount),
			Message:      n.Status.Message,
		},
	}
}

// NodeFromProto converts a proto Node to a domain Node.
func NodeFromProto(pb *v1.Node) *types.Node {
	if pb == nil {
		return nil
	}

	n := &types.Node{}

	if pb.Metadata != nil {
		n.Metadata.Name = pb.Metadata.Id
		n.Metadata.Namespace = pb.Metadata.Namespace
		n.Metadata.Generation = pb.Metadata.Generation
		if pb.Metadata.CreatedAt != nil {
			n.Metadata.CreatedAt = pb.Metadata.CreatedAt.AsTime()
		}
		if pb.Metadata.UpdatedAt != nil {
			n.Metadata.UpdatedAt = pb.Metadata.UpdatedAt.AsTime()
		}
		n.Spec.DevnetRef = pb.Metadata.DevnetName
		n.Spec.Index = int(pb.Metadata.Index)
	}

	if pb.Spec != nil {
		n.Spec.Role = pb.Spec.Role
		n.Spec.BinaryPath = pb.Spec.BinaryPath
		n.Spec.HomeDir = pb.Spec.HomeDir
		n.Spec.Desired = pb.Spec.DesiredPhase
	}

	if pb.Status != nil {
		n.Status.Phase = pb.Status.Phase
		n.Status.PID = int(pb.Status.Pid)
		n.Status.BlockHeight = pb.Status.BlockHeight
		n.Status.PeerCount = int(pb.Status.PeerCount)
		n.Status.CatchingUp = pb.Status.CatchingUp
		n.Status.RestartCount = int(pb.Status.RestartCount)
		n.Status.Message = pb.Status.Message
	}

	return n
}
