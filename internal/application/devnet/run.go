package devnet

import (
	"context"
	"fmt"
	"time"

	"github.com/b-harvest/devnet-builder/internal/application/dto"
	"github.com/b-harvest/devnet-builder/internal/application/ports"
)

// RunUseCase handles starting devnet nodes.
type RunUseCase struct {
	devnetRepo    ports.DevnetRepository
	nodeRepo      ports.NodeRepository
	executor      ports.ProcessExecutor
	healthChecker ports.HealthChecker
	networkModule ports.NetworkModule
	logger        ports.Logger
}

// NewRunUseCase creates a new RunUseCase.
func NewRunUseCase(
	devnetRepo ports.DevnetRepository,
	nodeRepo ports.NodeRepository,
	executor ports.ProcessExecutor,
	healthChecker ports.HealthChecker,
	networkModule ports.NetworkModule,
	logger ports.Logger,
) *RunUseCase {
	return &RunUseCase{
		devnetRepo:    devnetRepo,
		nodeRepo:      nodeRepo,
		executor:      executor,
		healthChecker: healthChecker,
		networkModule: networkModule,
		logger:        logger,
	}
}

// Execute starts the devnet nodes.
func (uc *RunUseCase) Execute(ctx context.Context, input dto.RunInput) (*dto.RunOutput, error) {
	uc.logger.Info("Starting devnet nodes...")

	// Load devnet metadata
	metadata, err := uc.devnetRepo.Load(ctx, input.HomeDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load devnet: %w", err)
	}

	// Check state
	if metadata.Status == ports.StateRunning {
		return nil, fmt.Errorf("devnet is already running")
	}

	// Load nodes
	nodes, err := uc.nodeRepo.LoadAll(ctx, input.HomeDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load nodes: %w", err)
	}

	// Start each node
	statuses := make([]dto.NodeStatus, len(nodes))
	allRunning := true

	for i, node := range nodes {
		uc.logger.Debug("Starting node %d...", node.Index)

		cmd := uc.buildStartCommand(node, metadata)
		handle, err := uc.executor.Start(ctx, cmd)
		if err != nil {
			uc.logger.Error("Failed to start node %d: %v", node.Index, err)
			allRunning = false
			statuses[i] = dto.NodeStatus{
				Index:     node.Index,
				Name:      node.Name,
				IsRunning: false,
			}
			continue
		}

		pid := handle.PID()
		node.PID = &pid
		if err := uc.nodeRepo.Save(ctx, node); err != nil {
			uc.logger.Warn("Failed to save node %d state: %v", node.Index, err)
		}

		statuses[i] = dto.NodeStatus{
			Index:     node.Index,
			Name:      node.Name,
			IsRunning: true,
			PID:       &pid,
		}
	}

	// Wait for health if requested
	if input.WaitForSync && allRunning {
		uc.logger.Info("Waiting for nodes to sync...")
		if err := uc.waitForHealth(ctx, nodes, input.Timeout); err != nil {
			uc.logger.Warn("Health check failed: %v", err)
		}
	}

	// Update metadata
	metadata.Status = ports.StateRunning
	now := time.Now()
	metadata.LastStarted = &now
	if err := uc.devnetRepo.Save(ctx, metadata); err != nil {
		uc.logger.Warn("Failed to update metadata: %v", err)
	}

	uc.logger.Success("Devnet started!")
	return &dto.RunOutput{
		Nodes:      statuses,
		AllRunning: allRunning,
	}, nil
}

func (uc *RunUseCase) buildStartCommand(node *ports.NodeMetadata, metadata *ports.DevnetMetadata) ports.Command {
	args := uc.networkModule.StartCommand(node.HomeDir)
	return ports.Command{
		Binary:  uc.networkModule.BinaryName(),
		Args:    args,
		WorkDir: node.HomeDir,
		LogPath: fmt.Sprintf("%s/%s", node.HomeDir, uc.networkModule.LogFileName()),
		PIDPath: fmt.Sprintf("%s/%s", node.HomeDir, uc.networkModule.PIDFileName()),
	}
}

func (uc *RunUseCase) waitForHealth(ctx context.Context, nodes []*ports.NodeMetadata, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		statuses, err := uc.healthChecker.CheckAllNodes(ctx, nodes)
		if err != nil {
			return err
		}

		allHealthy := true
		for _, status := range statuses {
			if !status.IsRunning || status.CatchingUp {
				allHealthy = false
				break
			}
		}

		if allHealthy {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return fmt.Errorf("timeout waiting for nodes to become healthy")
}

// StopUseCase handles stopping devnet nodes.
type StopUseCase struct {
	devnetRepo ports.DevnetRepository
	nodeRepo   ports.NodeRepository
	executor   ports.ProcessExecutor
	logger     ports.Logger
}

// NewStopUseCase creates a new StopUseCase.
func NewStopUseCase(
	devnetRepo ports.DevnetRepository,
	nodeRepo ports.NodeRepository,
	executor ports.ProcessExecutor,
	logger ports.Logger,
) *StopUseCase {
	return &StopUseCase{
		devnetRepo: devnetRepo,
		nodeRepo:   nodeRepo,
		executor:   executor,
		logger:     logger,
	}
}

// Execute stops the devnet nodes.
func (uc *StopUseCase) Execute(ctx context.Context, input dto.StopInput) (*dto.StopOutput, error) {
	uc.logger.Info("Stopping devnet nodes...")

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

	// Stop each node
	stoppedCount := 0
	var warnings []string

	for _, node := range nodes {
		if node.PID == nil {
			continue
		}

		uc.logger.Debug("Stopping node %d (PID: %d)...", node.Index, *node.PID)

		// Create a handle for the process
		handle := &pidHandle{pid: *node.PID}
		if err := uc.executor.Stop(ctx, handle, input.Timeout); err != nil {
			if input.Force {
				if err := uc.executor.Kill(handle); err != nil {
					warnings = append(warnings, fmt.Sprintf("failed to kill node %d: %v", node.Index, err))
				}
			} else {
				warnings = append(warnings, fmt.Sprintf("failed to stop node %d: %v", node.Index, err))
				continue
			}
		}

		node.PID = nil
		if err := uc.nodeRepo.Save(ctx, node); err != nil {
			uc.logger.Warn("Failed to save node %d state: %v", node.Index, err)
		}
		stoppedCount++
	}

	// Update metadata
	metadata.Status = ports.StateStopped
	now := time.Now()
	metadata.LastStopped = &now
	if err := uc.devnetRepo.Save(ctx, metadata); err != nil {
		uc.logger.Warn("Failed to update metadata: %v", err)
	}

	uc.logger.Success("Devnet stopped!")
	return &dto.StopOutput{
		StoppedNodes: stoppedCount,
		Warnings:     warnings,
	}, nil
}

// pidHandle is a simple ProcessHandle implementation for stopping by PID.
type pidHandle struct {
	pid int
}

func (h *pidHandle) PID() int        { return h.pid }
func (h *pidHandle) IsRunning() bool { return false }
func (h *pidHandle) Wait() error     { return nil }
func (h *pidHandle) Kill() error     { return nil }
