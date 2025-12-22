package devnet

import (
	"context"
	"fmt"

	"github.com/b-harvest/devnet-builder/internal/application/dto"
	"github.com/b-harvest/devnet-builder/internal/application/ports"
)

// HealthUseCase handles health checking of devnet nodes.
type HealthUseCase struct {
	devnetRepo    ports.DevnetRepository
	nodeRepo      ports.NodeRepository
	healthChecker ports.HealthChecker
	logger        ports.Logger
}

// NewHealthUseCase creates a new HealthUseCase.
func NewHealthUseCase(
	devnetRepo ports.DevnetRepository,
	nodeRepo ports.NodeRepository,
	healthChecker ports.HealthChecker,
	logger ports.Logger,
) *HealthUseCase {
	return &HealthUseCase{
		devnetRepo:    devnetRepo,
		nodeRepo:      nodeRepo,
		healthChecker: healthChecker,
		logger:        logger,
	}
}

// Execute checks the health of all devnet nodes.
func (uc *HealthUseCase) Execute(ctx context.Context, input dto.HealthInput) (*dto.HealthOutput, error) {
	// Load devnet metadata
	metadata, err := uc.devnetRepo.Load(ctx, input.HomeDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load devnet: %w", err)
	}

	// Load nodes
	nodes, err := uc.nodeRepo.LoadAll(ctx, input.HomeDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load nodes: %w", err)
	}

	// Check health
	statuses, err := uc.healthChecker.CheckAllNodes(ctx, nodes)
	if err != nil {
		return nil, fmt.Errorf("failed to check health: %w", err)
	}

	// Build output
	output := &dto.HealthOutput{
		AllHealthy:   true,
		Nodes:        make([]dto.NodeHealthStatus, len(statuses)),
		BlockHeights: make(map[int]int64),
	}

	for i, status := range statuses {
		output.Nodes[i] = dto.NodeHealthStatus{
			Index:       status.NodeIndex,
			Name:        status.NodeName,
			Status:      status.Status,
			IsRunning:   status.IsRunning,
			BlockHeight: status.BlockHeight,
			CatchingUp:  status.CatchingUp,
			AppVersion:  status.AppVersion,
		}
		if status.Error != nil {
			output.Nodes[i].Error = status.Error.Error()
		}

		output.BlockHeights[status.NodeIndex] = status.BlockHeight

		if !status.IsRunning || status.Status == ports.NodeStatusError {
			output.AllHealthy = false
		}
	}

	// Log results if verbose
	if input.Verbose {
		uc.logger.Info("Devnet status: %s", metadata.Status)
		for _, node := range output.Nodes {
			uc.logger.Info("  %s: %s (height: %d)", node.Name, node.Status, node.BlockHeight)
		}
	}

	return output, nil
}

// GetStatus returns a quick status check without full health details.
func (uc *HealthUseCase) GetStatus(ctx context.Context, homeDir string) (ports.DevnetState, error) {
	metadata, err := uc.devnetRepo.Load(ctx, homeDir)
	if err != nil {
		return "", err
	}
	return metadata.Status, nil
}

// IsRunning checks if the devnet is currently running.
func (uc *HealthUseCase) IsRunning(ctx context.Context, homeDir string) bool {
	status, err := uc.GetStatus(ctx, homeDir)
	if err != nil {
		return false
	}
	return status == ports.StateRunning
}
