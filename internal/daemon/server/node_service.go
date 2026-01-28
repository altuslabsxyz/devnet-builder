package server

import (
	"bufio"
	"context"
	"io"
	"log/slog"
	"strings"
	"time"

	v1 "github.com/altuslabsxyz/devnet-builder/api/proto/gen/v1"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/controller"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/runtime"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/server/ante"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/store"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// NodeService implements the gRPC NodeServiceServer.
type NodeService struct {
	v1.UnimplementedNodeServiceServer
	store   store.Store
	manager *controller.Manager
	runtime runtime.NodeRuntime
	logger  *slog.Logger
	ante    *ante.AnteHandler
}

// NewNodeService creates a new NodeService.
func NewNodeService(s store.Store, m *controller.Manager, r runtime.NodeRuntime) *NodeService {
	return &NodeService{
		store:   s,
		manager: m,
		runtime: r,
		logger:  slog.Default(),
	}
}

// NewNodeServiceWithAnte creates a new NodeService with ante handler.
func NewNodeServiceWithAnte(s store.Store, m *controller.Manager, r runtime.NodeRuntime, anteHandler *ante.AnteHandler) *NodeService {
	return &NodeService{
		store:   s,
		manager: m,
		runtime: r,
		logger:  slog.Default(),
		ante:    anteHandler,
	}
}

// SetLogger sets the logger for the service.
func (s *NodeService) SetLogger(logger *slog.Logger) {
	s.logger = logger
}

// GetNode retrieves a node by devnet name and index.
func (s *NodeService) GetNode(ctx context.Context, req *v1.GetNodeRequest) (*v1.GetNodeResponse, error) {
	if s.ante != nil {
		if err := s.ante.ValidateGetNode(ctx, req); err != nil {
			return nil, ante.ToGRPCError(err)
		}
	} else {
		if req.DevnetName == "" {
			return nil, status.Error(codes.InvalidArgument, "devnet_name is required")
		}
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
	if s.ante != nil {
		if err := s.ante.ValidateStartNode(ctx, req); err != nil {
			return nil, ante.ToGRPCError(err)
		}
	} else {
		if req.DevnetName == "" {
			return nil, status.Error(codes.InvalidArgument, "devnet_name is required")
		}
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
	if s.ante != nil {
		if err := s.ante.ValidateStopNode(ctx, req); err != nil {
			return nil, ante.ToGRPCError(err)
		}
	} else {
		if req.DevnetName == "" {
			return nil, status.Error(codes.InvalidArgument, "devnet_name is required")
		}
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
	if s.ante != nil {
		if err := s.ante.ValidateRestartNode(ctx, req); err != nil {
			return nil, ante.ToGRPCError(err)
		}
	} else {
		if req.DevnetName == "" {
			return nil, status.Error(codes.InvalidArgument, "devnet_name is required")
		}
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
	if s.ante != nil {
		if err := s.ante.ValidateGetNodeHealth(ctx, req); err != nil {
			return nil, ante.ToGRPCError(err)
		}
	} else {
		if req.DevnetName == "" {
			return nil, status.Error(codes.InvalidArgument, "devnet_name is required")
		}
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

// ExecInNode executes a command inside a running node container.
func (s *NodeService) ExecInNode(ctx context.Context, req *v1.ExecInNodeRequest) (*v1.ExecInNodeResponse, error) {
	if req.DevnetName == "" {
		return nil, status.Error(codes.InvalidArgument, "devnet_name is required")
	}
	if len(req.Command) == 0 {
		return nil, status.Error(codes.InvalidArgument, "command is required")
	}

	s.logger.Info("executing command in node",
		"devnet", req.DevnetName,
		"index", req.Index,
		"command", req.Command)

	// Verify node exists and get its details
	node, err := s.store.GetNode(ctx, req.Namespace, req.DevnetName, int(req.Index))
	if err != nil {
		if store.IsNotFound(err) {
			return nil, status.Errorf(codes.NotFound, "node %s/%d not found", req.DevnetName, req.Index)
		}
		s.logger.Error("failed to get node", "devnet", req.DevnetName, "index", req.Index, "error", err)
		return nil, status.Errorf(codes.Internal, "failed to get node: %v", err)
	}

	// Check if node is running
	if node.Status.Phase != types.NodePhaseRunning {
		return nil, status.Errorf(codes.FailedPrecondition, "node is not running (current phase: %s)", node.Status.Phase)
	}

	// Check if runtime is available
	if s.runtime == nil {
		return nil, status.Error(codes.Unavailable, "exec not available: no runtime configured")
	}

	// Determine timeout (default to 30 seconds if not specified)
	timeout := 30 * time.Second
	if req.TimeoutSeconds > 0 {
		timeout = time.Duration(req.TimeoutSeconds) * time.Second
	}

	// Get the node ID for runtime lookup
	nodeID := node.Metadata.Name
	if nodeID == "" {
		nodeID = controller.NodeKey(req.DevnetName, int(req.Index))
	}

	// Execute the command
	result, err := s.runtime.ExecInNode(ctx, nodeID, req.Command, timeout)
	if err != nil {
		s.logger.Error("exec failed", "nodeID", nodeID, "error", err)
		return nil, status.Errorf(codes.Internal, "exec failed: %v", err)
	}

	return &v1.ExecInNodeResponse{
		ExitCode: int32(result.ExitCode),
		Stdout:   result.Stdout,
		Stderr:   result.Stderr,
	}, nil
}

// Default Cosmos SDK ports for calculating exposed ports
const (
	defaultP2PPort  = 26656
	defaultRPCPort  = 26657
	defaultRESTPort = 1317
	defaultGRPCPort = 9090
)

// GetNodePorts returns the port mappings for a node.
func (s *NodeService) GetNodePorts(ctx context.Context, req *v1.GetNodePortsRequest) (*v1.GetNodePortsResponse, error) {
	if req.DevnetName == "" {
		return nil, status.Error(codes.InvalidArgument, "devnet_name is required")
	}

	// Verify node exists (GetNodePortsRequest doesn't have namespace, use default)
	node, err := s.store.GetNode(ctx, "", req.DevnetName, int(req.Index))
	if err != nil {
		if store.IsNotFound(err) {
			return nil, status.Errorf(codes.NotFound, "node %s/%d not found", req.DevnetName, req.Index)
		}
		s.logger.Error("failed to get node", "devnet", req.DevnetName, "index", req.Index, "error", err)
		return nil, status.Errorf(codes.Internal, "failed to get node: %v", err)
	}

	// Calculate port offset: each node gets 100 ports apart
	offset := int32(node.Spec.Index * 100)

	ports := []*v1.PortMapping{
		{
			Name:          "p2p",
			ContainerPort: defaultP2PPort,
			HostPort:      defaultP2PPort + offset,
			Protocol:      "tcp",
		},
		{
			Name:          "rpc",
			ContainerPort: defaultRPCPort,
			HostPort:      defaultRPCPort + offset,
			Protocol:      "tcp",
		},
		{
			Name:          "rest",
			ContainerPort: defaultRESTPort,
			HostPort:      defaultRESTPort + offset,
			Protocol:      "tcp",
		},
		{
			Name:          "grpc",
			ContainerPort: defaultGRPCPort,
			HostPort:      defaultGRPCPort + offset,
			Protocol:      "tcp",
		},
	}

	return &v1.GetNodePortsResponse{
		DevnetName: req.DevnetName,
		Index:      req.Index,
		Ports:      ports,
	}, nil
}

// StreamNodeLogs streams logs from a node to the client.
func (s *NodeService) StreamNodeLogs(req *v1.StreamNodeLogsRequest, stream grpc.ServerStreamingServer[v1.StreamNodeLogsResponse]) error {
	if req.DevnetName == "" {
		return status.Error(codes.InvalidArgument, "devnet_name is required")
	}

	ctx := stream.Context()

	// Verify node exists
	node, err := s.store.GetNode(ctx, req.Namespace, req.DevnetName, int(req.Index))
	if err != nil {
		if store.IsNotFound(err) {
			return status.Errorf(codes.NotFound, "node %s/%d not found", req.DevnetName, req.Index)
		}
		s.logger.Error("failed to get node", "devnet", req.DevnetName, "index", req.Index, "error", err)
		return status.Errorf(codes.Internal, "failed to get node: %v", err)
	}

	// Check if runtime is available
	if s.runtime == nil {
		return status.Error(codes.Unavailable, "log streaming not available: no runtime configured")
	}

	// Parse since timestamp if provided
	var since time.Time
	if req.Since != "" {
		var err error
		since, err = time.Parse(time.RFC3339, req.Since)
		if err != nil {
			return status.Errorf(codes.InvalidArgument, "invalid since timestamp: %v", err)
		}
	}

	// Build log options
	logOpts := runtime.LogOptions{
		Follow: req.Follow,
		Lines:  int(req.Tail),
		Since:  since,
	}

	s.logger.Info("streaming logs",
		"devnet", req.DevnetName,
		"index", req.Index,
		"follow", req.Follow,
		"tail", req.Tail)

	// Get logs from runtime
	nodeID := controller.NodeKey(req.DevnetName, int(req.Index))
	// Try to get logs using the node name from store
	if node.Metadata.Name != "" {
		nodeID = node.Metadata.Name
	}

	reader, err := s.runtime.GetLogs(ctx, nodeID, logOpts)
	if err != nil {
		s.logger.Error("failed to get logs", "nodeID", nodeID, "error", err)
		return status.Errorf(codes.Internal, "failed to get logs: %v", err)
	}
	defer reader.Close()

	// Stream logs to client
	scanner := bufio.NewScanner(reader)
	// Increase buffer size for long log lines
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		line := scanner.Text()
		// Docker logs include a header (8 bytes) for stream type
		// The first byte indicates: 0=stdin, 1=stdout, 2=stderr
		streamType := "stdout"
		message := line

		// If line is long enough and starts with stream header, parse it
		if len(line) >= 8 {
			switch line[0] {
			case 1:
				streamType = "stdout"
				message = line[8:]
			case 2:
				streamType = "stderr"
				message = line[8:]
			default:
				// No header, use as-is
				message = line
			}
		}

		// Skip empty messages
		message = strings.TrimSpace(message)
		if message == "" {
			continue
		}

		resp := &v1.StreamNodeLogsResponse{
			Timestamp: timestamppb.Now(),
			Stream:    streamType,
			Message:   message,
		}

		if err := stream.Send(resp); err != nil {
			s.logger.Debug("client disconnected", "error", err)
			return nil
		}
	}

	if err := scanner.Err(); err != nil && err != io.EOF {
		s.logger.Error("error reading logs", "error", err)
		return status.Errorf(codes.Internal, "error reading logs: %v", err)
	}

	return nil
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
