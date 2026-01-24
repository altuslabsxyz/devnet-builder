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

	// TODO: Implement label selector filtering
	// For now, return all devnets

	resp := &v1.ListDevnetsResponse{
		Devnets: make([]*v1.Devnet, 0, len(devnets)),
	}

	for _, d := range devnets {
		resp.Devnets = append(resp.Devnets, DevnetToProto(d))
	}

	return resp, nil
}

// DeleteDevnet deletes a devnet and all its nodes (cascade delete).
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

	err := s.store.DeleteDevnet(ctx, req.Name)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, status.Errorf(codes.NotFound, "devnet %q not found", req.Name)
		}
		s.logger.Error("failed to delete devnet", "name", req.Name, "error", err)
		return nil, status.Errorf(codes.Internal, "failed to delete devnet: %v", err)
	}

	// TODO: In Phase 3, enqueue for deprovisioning before actual deletion

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

	// TODO: In Phase 3, enqueue for actual container stopping

	return &v1.StopDevnetResponse{Devnet: DevnetToProto(devnet)}, nil
}
