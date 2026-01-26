package server

import (
	"context"
	"errors"
	"log/slog"
	"time"

	v1 "github.com/altuslabsxyz/devnet-builder/api/proto/gen/v1"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/controller"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/store"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// DevnetService implements the gRPC DevnetServiceServer.
type DevnetService struct {
	v1.UnimplementedDevnetServiceServer
	store   store.Store
	manager *controller.Manager
	logger  *slog.Logger
}

// NewDevnetService creates a new DevnetService.
func NewDevnetService(s store.Store, m *controller.Manager) *DevnetService {
	return &DevnetService{
		store:   s,
		manager: m,
		logger:  slog.Default(),
	}
}

// SetLogger sets the logger for the service.
func (s *DevnetService) SetLogger(logger *slog.Logger) {
	s.logger = logger
}

// CreateDevnet creates a new devnet.
func (s *DevnetService) CreateDevnet(ctx context.Context, req *v1.CreateDevnetRequest) (*v1.CreateDevnetResponse, error) {
	// Validate request
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	s.logger.Info("creating devnet", "name", req.Name)

	// Convert to domain type
	devnet := CreateRequestToDevnet(req)

	// Store it
	err := s.store.CreateDevnet(ctx, devnet)
	if err != nil {
		if errors.Is(err, store.ErrAlreadyExists) {
			return nil, status.Errorf(codes.AlreadyExists, "devnet %q already exists", req.Name)
		}
		s.logger.Error("failed to create devnet", "name", req.Name, "error", err)
		return nil, status.Errorf(codes.Internal, "failed to create devnet: %v", err)
	}

	// Enqueue for reconciliation
	if s.manager != nil {
		s.manager.Enqueue("devnets", req.Name)
	}

	return &v1.CreateDevnetResponse{Devnet: DevnetToProto(devnet)}, nil
}

// GetDevnet retrieves a devnet by name.
func (s *DevnetService) GetDevnet(ctx context.Context, req *v1.GetDevnetRequest) (*v1.GetDevnetResponse, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	devnet, err := s.store.GetDevnet(ctx, req.Name)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, status.Errorf(codes.NotFound, "devnet %q not found", req.Name)
		}
		s.logger.Error("failed to get devnet", "name", req.Name, "error", err)
		return nil, status.Errorf(codes.Internal, "failed to get devnet: %v", err)
	}

	return &v1.GetDevnetResponse{Devnet: DevnetToProto(devnet)}, nil
}

// ListDevnets lists all devnets.
func (s *DevnetService) ListDevnets(ctx context.Context, req *v1.ListDevnetsRequest) (*v1.ListDevnetsResponse, error) {
	devnets, err := s.store.ListDevnets(ctx)
	if err != nil {
		s.logger.Error("failed to list devnets", "error", err)
		return nil, status.Errorf(codes.Internal, "failed to list devnets: %v", err)
	}

	// Filter by label selector if provided
	// Format: "key1=value1,key2=value2" (all must match)
	labelFilter := parseLabelSelector(req.GetLabelSelector())

	resp := &v1.ListDevnetsResponse{
		Devnets: make([]*v1.Devnet, 0, len(devnets)),
	}

	for _, d := range devnets {
		if matchesLabels(d.Metadata.Labels, labelFilter) {
			resp.Devnets = append(resp.Devnets, DevnetToProto(d))
		}
	}

	return resp, nil
}

// DeleteDevnet deletes a devnet and all its nodes and upgrades (cascade delete).
func (s *DevnetService) DeleteDevnet(ctx context.Context, req *v1.DeleteDevnetRequest) (*v1.DeleteDevnetResponse, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	s.logger.Info("deleting devnet", "name", req.Name)

	// Cascade delete: remove all nodes belonging to this devnet first
	if err := s.store.DeleteNodesByDevnet(ctx, req.Name); err != nil {
		s.logger.Warn("failed to delete nodes during cascade delete", "devnet", req.Name, "error", err)
		// Continue with devnet deletion even if node deletion fails
	}

	// Cascade delete: remove all upgrades belonging to this devnet
	if err := s.store.DeleteUpgradesByDevnet(ctx, req.Name); err != nil {
		s.logger.Warn("failed to delete upgrades during cascade delete", "devnet", req.Name, "error", err)
		// Continue with devnet deletion even if upgrade deletion fails
	}

	err := s.store.DeleteDevnet(ctx, req.Name)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, status.Errorf(codes.NotFound, "devnet %q not found", req.Name)
		}
		s.logger.Error("failed to delete devnet", "name", req.Name, "error", err)
		return nil, status.Errorf(codes.Internal, "failed to delete devnet: %v", err)
	}

	// Enqueue for deprovisioning (controller will handle cleanup)
	if s.manager != nil {
		s.manager.Enqueue("devnets", req.Name)
	}

	return &v1.DeleteDevnetResponse{Deleted: true}, nil
}

// StartDevnet starts a stopped devnet.
func (s *DevnetService) StartDevnet(ctx context.Context, req *v1.StartDevnetRequest) (*v1.StartDevnetResponse, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	s.logger.Info("starting devnet", "name", req.Name)

	devnet, err := s.store.GetDevnet(ctx, req.Name)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, status.Errorf(codes.NotFound, "devnet %q not found", req.Name)
		}
		return nil, status.Errorf(codes.Internal, "failed to get devnet: %v", err)
	}

	// Transition to Pending to trigger reconciliation
	devnet.Status.Phase = types.PhasePending
	devnet.Status.Message = "Starting devnet"
	devnet.Metadata.UpdatedAt = time.Now()

	err = s.store.UpdateDevnet(ctx, devnet)
	if err != nil {
		s.logger.Error("failed to update devnet", "name", req.Name, "error", err)
		return nil, status.Errorf(codes.Internal, "failed to update devnet: %v", err)
	}

	// Enqueue for reconciliation
	if s.manager != nil {
		s.manager.Enqueue("devnets", req.Name)
	}

	return &v1.StartDevnetResponse{Devnet: DevnetToProto(devnet)}, nil
}

// StopDevnet stops a running devnet.
func (s *DevnetService) StopDevnet(ctx context.Context, req *v1.StopDevnetRequest) (*v1.StopDevnetResponse, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	s.logger.Info("stopping devnet", "name", req.Name)

	devnet, err := s.store.GetDevnet(ctx, req.Name)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, status.Errorf(codes.NotFound, "devnet %q not found", req.Name)
		}
		return nil, status.Errorf(codes.Internal, "failed to get devnet: %v", err)
	}

	// Transition to Stopped
	devnet.Status.Phase = types.PhaseStopped
	devnet.Status.Message = "Devnet stopped"
	devnet.Status.ReadyNodes = 0
	devnet.Metadata.UpdatedAt = time.Now()

	err = s.store.UpdateDevnet(ctx, devnet)
	if err != nil {
		s.logger.Error("failed to update devnet", "name", req.Name, "error", err)
		return nil, status.Errorf(codes.Internal, "failed to update devnet: %v", err)
	}

	// Enqueue for container stopping (controller will handle the actual stop)
	if s.manager != nil {
		s.manager.Enqueue("devnets", req.Name)
	}

	return &v1.StopDevnetResponse{Devnet: DevnetToProto(devnet)}, nil
}

// ApplyDevnet creates or updates a devnet (idempotent).
func (s *DevnetService) ApplyDevnet(ctx context.Context, req *v1.ApplyDevnetRequest) (*v1.ApplyDevnetResponse, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	s.logger.Info("applying devnet", "name", req.Name)

	// Check if devnet exists
	existing, err := s.store.GetDevnet(ctx, req.Name)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		s.logger.Error("failed to get devnet", "name", req.Name, "error", err)
		return nil, status.Errorf(codes.Internal, "failed to get devnet: %v", err)
	}

	if existing == nil {
		// Create new devnet
		devnet := ApplyRequestToDevnet(req)
		err = s.store.CreateDevnet(ctx, devnet)
		if err != nil {
			s.logger.Error("failed to create devnet", "name", req.Name, "error", err)
			return nil, status.Errorf(codes.Internal, "failed to create devnet: %v", err)
		}

		if s.manager != nil {
			s.manager.Enqueue("devnets", req.Name)
		}

		return &v1.ApplyDevnetResponse{
			Devnet: DevnetToProto(devnet),
			Action: "created",
		}, nil
	}

	// Check if spec changed
	if specsEqual(existing.Spec, req.Spec) && labelsEqual(existing.Metadata.Labels, req.Labels) {
		return &v1.ApplyDevnetResponse{
			Devnet: DevnetToProto(existing),
			Action: "unchanged",
		}, nil
	}

	// Update existing devnet
	existing.Spec = specFromProto(req.Spec)
	if req.Labels != nil {
		existing.Metadata.Labels = req.Labels
	}
	if req.Annotations != nil {
		existing.Metadata.Annotations = req.Annotations
	}
	existing.Metadata.UpdatedAt = time.Now()
	existing.Metadata.Generation++

	err = s.store.UpdateDevnet(ctx, existing)
	if err != nil {
		s.logger.Error("failed to update devnet", "name", req.Name, "error", err)
		return nil, status.Errorf(codes.Internal, "failed to update devnet: %v", err)
	}

	if s.manager != nil {
		s.manager.Enqueue("devnets", req.Name)
	}

	return &v1.ApplyDevnetResponse{
		Devnet: DevnetToProto(existing),
		Action: "configured",
	}, nil
}

// UpdateDevnet updates an existing devnet.
func (s *DevnetService) UpdateDevnet(ctx context.Context, req *v1.UpdateDevnetRequest) (*v1.UpdateDevnetResponse, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	s.logger.Info("updating devnet", "name", req.Name)

	existing, err := s.store.GetDevnet(ctx, req.Name)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, status.Errorf(codes.NotFound, "devnet %q not found", req.Name)
		}
		return nil, status.Errorf(codes.Internal, "failed to get devnet: %v", err)
	}

	existing.Spec = specFromProto(req.Spec)
	if req.Labels != nil {
		existing.Metadata.Labels = req.Labels
	}
	if req.Annotations != nil {
		existing.Metadata.Annotations = req.Annotations
	}
	existing.Metadata.UpdatedAt = time.Now()
	existing.Metadata.Generation++

	err = s.store.UpdateDevnet(ctx, existing)
	if err != nil {
		s.logger.Error("failed to update devnet", "name", req.Name, "error", err)
		return nil, status.Errorf(codes.Internal, "failed to update devnet: %v", err)
	}

	if s.manager != nil {
		s.manager.Enqueue("devnets", req.Name)
	}

	return &v1.UpdateDevnetResponse{Devnet: DevnetToProto(existing)}, nil
}

// parseLabelSelector parses a comma-separated label selector string into a map.
// Format: "key1=value1,key2=value2"
// Returns an empty map if the selector is empty.
func parseLabelSelector(selector string) map[string]string {
	result := make(map[string]string)
	if selector == "" {
		return result
	}

	pairs := splitTrimmed(selector, ",")
	for _, pair := range pairs {
		if pair == "" {
			continue
		}
		kv := splitTrimmed(pair, "=")
		if len(kv) == 2 && kv[0] != "" {
			result[kv[0]] = kv[1]
		}
	}
	return result
}

// matchesLabels returns true if the resource labels match all the filter labels.
// An empty filter matches everything.
func matchesLabels(resourceLabels, filter map[string]string) bool {
	for k, v := range filter {
		if resourceLabels == nil || resourceLabels[k] != v {
			return false
		}
	}
	return true
}

// splitTrimmed splits a string and trims whitespace from each part.
func splitTrimmed(s, sep string) []string {
	parts := make([]string, 0)
	for _, part := range splitString(s, sep) {
		trimmed := trimSpace(part)
		parts = append(parts, trimmed)
	}
	return parts
}

// splitString splits a string by separator without using strings package.
func splitString(s, sep string) []string {
	var result []string
	for {
		i := indexOf(s, sep)
		if i < 0 {
			result = append(result, s)
			break
		}
		result = append(result, s[:i])
		s = s[i+len(sep):]
	}
	return result
}

// indexOf returns the index of sep in s, or -1 if not found.
func indexOf(s, sep string) int {
	for i := 0; i <= len(s)-len(sep); i++ {
		if s[i:i+len(sep)] == sep {
			return i
		}
	}
	return -1
}

// trimSpace removes leading and trailing whitespace.
func trimSpace(s string) string {
	start := 0
	for start < len(s) && isSpace(s[start]) {
		start++
	}
	end := len(s)
	for end > start && isSpace(s[end-1]) {
		end--
	}
	return s[start:end]
}

// isSpace returns true if c is a whitespace character.
func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}
