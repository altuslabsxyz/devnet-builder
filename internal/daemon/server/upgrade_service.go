// internal/daemon/server/upgrade_service.go
package server

import (
	"context"
	"errors"
	"log/slog"

	v1 "github.com/altuslabsxyz/devnet-builder/api/proto/gen/v1"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/controller"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/store"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// UpgradeService implements the gRPC UpgradeServiceServer.
type UpgradeService struct {
	v1.UnimplementedUpgradeServiceServer
	store   store.Store
	manager *controller.Manager
	logger  *slog.Logger
}

// NewUpgradeService creates a new UpgradeService.
func NewUpgradeService(s store.Store, m *controller.Manager) *UpgradeService {
	return &UpgradeService{
		store:   s,
		manager: m,
		logger:  slog.Default(),
	}
}

// SetLogger sets the logger for the service.
func (s *UpgradeService) SetLogger(logger *slog.Logger) {
	s.logger = logger
}

// CreateUpgrade creates a new upgrade.
func (s *UpgradeService) CreateUpgrade(ctx context.Context, req *v1.CreateUpgradeRequest) (*v1.CreateUpgradeResponse, error) {
	// Validate request
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	if req.Spec == nil {
		return nil, status.Error(codes.InvalidArgument, "spec is required")
	}
	if req.Spec.DevnetRef == "" {
		return nil, status.Error(codes.InvalidArgument, "spec.devnet_ref is required")
	}
	if req.Spec.UpgradeName == "" {
		return nil, status.Error(codes.InvalidArgument, "spec.upgrade_name is required")
	}

	s.logger.Info("creating upgrade",
		"name", req.Name,
		"devnet", req.Spec.DevnetRef,
		"upgradeName", req.Spec.UpgradeName)

	// Verify devnet exists
	_, err := s.store.GetDevnet(ctx, req.Spec.DevnetRef)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, status.Errorf(codes.NotFound, "devnet %q not found", req.Spec.DevnetRef)
		}
		s.logger.Error("failed to get devnet", "devnet", req.Spec.DevnetRef, "error", err)
		return nil, status.Errorf(codes.Internal, "failed to verify devnet: %v", err)
	}

	// Convert to domain type
	upgrade := CreateUpgradeRequestToUpgrade(req)

	// Store it
	err = s.store.CreateUpgrade(ctx, upgrade)
	if err != nil {
		if errors.Is(err, store.ErrAlreadyExists) {
			return nil, status.Errorf(codes.AlreadyExists, "upgrade %q already exists", req.Name)
		}
		s.logger.Error("failed to create upgrade", "name", req.Name, "error", err)
		return nil, status.Errorf(codes.Internal, "failed to create upgrade: %v", err)
	}

	// Enqueue for reconciliation
	if s.manager != nil {
		s.manager.Enqueue("upgrades", req.Name)
	}

	return &v1.CreateUpgradeResponse{Upgrade: UpgradeToProto(upgrade)}, nil
}

// GetUpgrade retrieves an upgrade by name.
func (s *UpgradeService) GetUpgrade(ctx context.Context, req *v1.GetUpgradeRequest) (*v1.GetUpgradeResponse, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	upgrade, err := s.store.GetUpgrade(ctx, req.Name)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, status.Errorf(codes.NotFound, "upgrade %q not found", req.Name)
		}
		s.logger.Error("failed to get upgrade", "name", req.Name, "error", err)
		return nil, status.Errorf(codes.Internal, "failed to get upgrade: %v", err)
	}

	return &v1.GetUpgradeResponse{Upgrade: UpgradeToProto(upgrade)}, nil
}

// ListUpgrades lists upgrades, optionally filtered by devnet.
func (s *UpgradeService) ListUpgrades(ctx context.Context, req *v1.ListUpgradesRequest) (*v1.ListUpgradesResponse, error) {
	upgrades, err := s.store.ListUpgrades(ctx, req.DevnetName)
	if err != nil {
		s.logger.Error("failed to list upgrades", "error", err)
		return nil, status.Errorf(codes.Internal, "failed to list upgrades: %v", err)
	}

	resp := &v1.ListUpgradesResponse{
		Upgrades: make([]*v1.Upgrade, 0, len(upgrades)),
	}

	for _, u := range upgrades {
		resp.Upgrades = append(resp.Upgrades, UpgradeToProto(u))
	}

	return resp, nil
}

// DeleteUpgrade deletes an upgrade.
func (s *UpgradeService) DeleteUpgrade(ctx context.Context, req *v1.DeleteUpgradeRequest) (*v1.DeleteUpgradeResponse, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	s.logger.Info("deleting upgrade", "name", req.Name)

	// Check if upgrade exists and is in a terminal state
	upgrade, err := s.store.GetUpgrade(ctx, req.Name)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, status.Errorf(codes.NotFound, "upgrade %q not found", req.Name)
		}
		s.logger.Error("failed to get upgrade", "name", req.Name, "error", err)
		return nil, status.Errorf(codes.Internal, "failed to get upgrade: %v", err)
	}

	// Don't allow deleting upgrades that are in progress
	switch upgrade.Status.Phase {
	case types.UpgradePhasePending, types.UpgradePhaseCompleted, types.UpgradePhaseFailed:
		// These are safe to delete
	default:
		return nil, status.Errorf(codes.FailedPrecondition,
			"cannot delete upgrade in phase %q - cancel it first", upgrade.Status.Phase)
	}

	err = s.store.DeleteUpgrade(ctx, req.Name)
	if err != nil {
		s.logger.Error("failed to delete upgrade", "name", req.Name, "error", err)
		return nil, status.Errorf(codes.Internal, "failed to delete upgrade: %v", err)
	}

	return &v1.DeleteUpgradeResponse{Deleted: true}, nil
}

// CancelUpgrade cancels an in-progress upgrade.
func (s *UpgradeService) CancelUpgrade(ctx context.Context, req *v1.CancelUpgradeRequest) (*v1.CancelUpgradeResponse, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	s.logger.Info("cancelling upgrade", "name", req.Name)

	upgrade, err := s.store.GetUpgrade(ctx, req.Name)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, status.Errorf(codes.NotFound, "upgrade %q not found", req.Name)
		}
		s.logger.Error("failed to get upgrade", "name", req.Name, "error", err)
		return nil, status.Errorf(codes.Internal, "failed to get upgrade: %v", err)
	}

	// Check if upgrade can be cancelled
	switch upgrade.Status.Phase {
	case types.UpgradePhaseCompleted:
		return nil, status.Error(codes.FailedPrecondition, "cannot cancel a completed upgrade")
	case types.UpgradePhaseFailed:
		return nil, status.Error(codes.FailedPrecondition, "upgrade already failed")
	case types.UpgradePhaseSwitching, types.UpgradePhaseVerifying:
		return nil, status.Errorf(codes.FailedPrecondition,
			"cannot cancel upgrade in phase %q - binary switch in progress", upgrade.Status.Phase)
	}

	// Transition to Failed (cancelled)
	upgrade.Status.Phase = types.UpgradePhaseFailed
	upgrade.Status.Message = "Upgrade cancelled by user"
	upgrade.Status.Error = "cancelled"

	err = s.store.UpdateUpgrade(ctx, upgrade)
	if err != nil {
		s.logger.Error("failed to update upgrade", "name", req.Name, "error", err)
		return nil, status.Errorf(codes.Internal, "failed to cancel upgrade: %v", err)
	}

	return &v1.CancelUpgradeResponse{Upgrade: UpgradeToProto(upgrade)}, nil
}

// RetryUpgrade retries a failed upgrade.
func (s *UpgradeService) RetryUpgrade(ctx context.Context, req *v1.RetryUpgradeRequest) (*v1.RetryUpgradeResponse, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	s.logger.Info("retrying upgrade", "name", req.Name)

	upgrade, err := s.store.GetUpgrade(ctx, req.Name)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, status.Errorf(codes.NotFound, "upgrade %q not found", req.Name)
		}
		s.logger.Error("failed to get upgrade", "name", req.Name, "error", err)
		return nil, status.Errorf(codes.Internal, "failed to get upgrade: %v", err)
	}

	// Only Failed upgrades can be retried
	if upgrade.Status.Phase != types.UpgradePhaseFailed {
		return nil, status.Errorf(codes.FailedPrecondition,
			"can only retry failed upgrades, current phase: %q", upgrade.Status.Phase)
	}

	// Reset to Pending to restart the upgrade process
	upgrade.Status.Phase = types.UpgradePhasePending
	upgrade.Status.Message = "Retrying upgrade"
	upgrade.Status.Error = ""

	err = s.store.UpdateUpgrade(ctx, upgrade)
	if err != nil {
		s.logger.Error("failed to update upgrade", "name", req.Name, "error", err)
		return nil, status.Errorf(codes.Internal, "failed to retry upgrade: %v", err)
	}

	// Enqueue for reconciliation
	if s.manager != nil {
		s.manager.Enqueue("upgrades", req.Name)
	}

	return &v1.RetryUpgradeResponse{Upgrade: UpgradeToProto(upgrade)}, nil
}
