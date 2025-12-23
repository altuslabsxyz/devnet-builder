package devnet

import (
	"context"
	"fmt"
	"time"

	"github.com/b-harvest/devnet-builder/internal/application/dto"
	"github.com/b-harvest/devnet-builder/internal/application/ports"
)

// ResetUseCase handles resetting devnet data.
type ResetUseCase struct {
	devnetRepo ports.DevnetRepository
	nodeRepo   ports.NodeRepository
	stopUC     *StopUseCase
	logger     ports.Logger
}

// NewResetUseCase creates a new ResetUseCase.
func NewResetUseCase(
	devnetRepo ports.DevnetRepository,
	nodeRepo ports.NodeRepository,
	stopUC *StopUseCase,
	logger ports.Logger,
) *ResetUseCase {
	return &ResetUseCase{
		devnetRepo: devnetRepo,
		nodeRepo:   nodeRepo,
		stopUC:     stopUC,
		logger:     logger,
	}
}

// Execute resets the devnet data.
func (uc *ResetUseCase) Execute(ctx context.Context, input dto.ResetInput) (*dto.ResetOutput, error) {
	// Load devnet metadata
	metadata, err := uc.devnetRepo.Load(ctx, input.HomeDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load devnet: %w", err)
	}

	// Stop nodes if running
	if metadata.Status == ports.StateRunning {
		uc.logger.Info("Stopping running nodes...")
		_, err := uc.stopUC.Execute(ctx, dto.StopInput{
			HomeDir: input.HomeDir,
			Timeout: 5 * time.Second, // Short timeout for reset
			Force:   true,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to stop nodes: %w", err)
		}
	}

	if input.HardReset {
		return uc.hardReset(ctx, input)
	}
	return uc.softReset(ctx, input, metadata)
}

// softReset clears chain data but preserves genesis and configuration.
func (uc *ResetUseCase) softReset(ctx context.Context, input dto.ResetInput, metadata *ports.DevnetMetadata) (*dto.ResetOutput, error) {
	uc.logger.Info("Performing soft reset (preserving genesis and config)...")

	// Load nodes
	nodes, err := uc.nodeRepo.LoadAll(ctx, input.HomeDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load nodes: %w", err)
	}

	removed := make([]string, 0)
	for _, node := range nodes {
		dataDir := fmt.Sprintf("%s/data", node.HomeDir)
		removed = append(removed, dataDir)
		uc.logger.Debug("Clearing data for node %d", node.Index)
	}

	// Update metadata status
	metadata.Status = ports.StateProvisioned
	if err := uc.devnetRepo.Save(ctx, metadata); err != nil {
		return nil, fmt.Errorf("failed to update metadata: %w", err)
	}

	uc.logger.Success("Soft reset complete!")
	return &dto.ResetOutput{
		Type:    "soft",
		Removed: removed,
	}, nil
}

// hardReset removes all devnet data including genesis.
func (uc *ResetUseCase) hardReset(ctx context.Context, input dto.ResetInput) (*dto.ResetOutput, error) {
	uc.logger.Info("Performing hard reset (removing all data)...")

	devnetDir := fmt.Sprintf("%s/devnet", input.HomeDir)

	// Delete all devnet data
	if err := uc.devnetRepo.Delete(ctx, input.HomeDir); err != nil {
		return nil, fmt.Errorf("failed to delete devnet data: %w", err)
	}

	uc.logger.Success("Hard reset complete!")
	return &dto.ResetOutput{
		Type:    "hard",
		Removed: []string{devnetDir},
	}, nil
}

// DestroyUseCase handles complete devnet destruction.
type DestroyUseCase struct {
	devnetRepo ports.DevnetRepository
	stopUC     *StopUseCase
	logger     ports.Logger
}

// NewDestroyUseCase creates a new DestroyUseCase.
func NewDestroyUseCase(
	devnetRepo ports.DevnetRepository,
	stopUC *StopUseCase,
	logger ports.Logger,
) *DestroyUseCase {
	return &DestroyUseCase{
		devnetRepo: devnetRepo,
		stopUC:     stopUC,
		logger:     logger,
	}
}

// Execute destroys the devnet completely.
func (uc *DestroyUseCase) Execute(ctx context.Context, input dto.DestroyInput) (*dto.DestroyOutput, error) {
	uc.logger.Info("Destroying devnet...")

	// Check if devnet exists
	if !uc.devnetRepo.Exists(input.HomeDir) {
		return nil, fmt.Errorf("no devnet found at %s", input.HomeDir)
	}

	// Stop nodes if running (use short timeout for destroy - we're deleting anyway)
	metadata, err := uc.devnetRepo.Load(ctx, input.HomeDir)
	stoppedNodes := 0
	if err == nil && metadata.Status == ports.StateRunning {
		uc.logger.Info("Stopping running nodes...")
		result, err := uc.stopUC.Execute(ctx, dto.StopInput{
			HomeDir: input.HomeDir,
			Timeout: 3 * time.Second, // Short timeout - force kill quickly
			Force:   true,
		})
		if err != nil {
			uc.logger.Warn("Failed to stop nodes: %v", err)
		} else {
			stoppedNodes = result.StoppedNodes
		}
	}

	// Delete all data
	if err := uc.devnetRepo.Delete(ctx, input.HomeDir); err != nil {
		return nil, fmt.Errorf("failed to delete devnet: %w", err)
	}

	uc.logger.Success("Devnet destroyed!")
	return &dto.DestroyOutput{
		RemovedDir:   fmt.Sprintf("%s/devnet", input.HomeDir),
		NodesStopped: stoppedNodes,
	}, nil
}
