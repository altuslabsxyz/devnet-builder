package upgrade

import (
	"context"
	"fmt"
	"time"

	"github.com/b-harvest/devnet-builder/internal/application/dto"
	"github.com/b-harvest/devnet-builder/internal/application/ports"
)

// SwitchBinaryUseCase handles switching the chain binary during upgrade.
type SwitchBinaryUseCase struct {
	devnetRepo  ports.DevnetRepository
	nodeRepo    ports.NodeRepository
	executor    ports.ProcessExecutor
	binaryCache ports.BinaryCache
	logger      ports.Logger
}

// NewSwitchBinaryUseCase creates a new SwitchBinaryUseCase.
func NewSwitchBinaryUseCase(
	devnetRepo ports.DevnetRepository,
	nodeRepo ports.NodeRepository,
	executor ports.ProcessExecutor,
	binaryCache ports.BinaryCache,
	logger ports.Logger,
) *SwitchBinaryUseCase {
	return &SwitchBinaryUseCase{
		devnetRepo:  devnetRepo,
		nodeRepo:    nodeRepo,
		executor:    executor,
		binaryCache: binaryCache,
		logger:      logger,
	}
}

// Execute switches the binary and restarts nodes.
func (uc *SwitchBinaryUseCase) Execute(ctx context.Context, input dto.SwitchBinaryInput) (*dto.SwitchBinaryOutput, error) {
	uc.logger.Info("Switching binary...")

	// Load devnet
	metadata, err := uc.devnetRepo.Load(ctx, input.HomeDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load devnet: %w", err)
	}

	// Determine old binary
	oldBinary, _ := uc.binaryCache.GetActive()

	// Determine new binary path
	var newBinary string
	switch {
	case input.CachePath != "":
		// Use pre-cached binary
		newBinary = input.CachePath
		uc.logger.Debug("Using cached binary: %s", newBinary)
	case input.TargetBinary != "":
		// Use specified binary path
		newBinary = input.TargetBinary
		uc.logger.Debug("Using specified binary: %s", newBinary)
	case input.TargetImage != "":
		// Docker mode - update image in metadata
		metadata.DockerImage = input.TargetImage
		uc.logger.Debug("Using Docker image: %s", input.TargetImage)
	default:
		return nil, fmt.Errorf("no target binary specified")
	}

	// For local mode, activate the new binary
	if input.Mode == ports.ModeLocal && newBinary != "" {
		cacheRef := input.CacheRef
		if cacheRef == "" {
			cacheRef = input.CommitHash // Fallback for backward compatibility
		}
		if err := uc.binaryCache.SetActive(cacheRef); err != nil {
			return nil, fmt.Errorf("failed to activate binary: %w", err)
		}
	}

	// Load nodes
	nodes, err := uc.nodeRepo.LoadAll(ctx, input.HomeDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load nodes: %w", err)
	}

	// Restart nodes with new binary
	restarted := 0
	for _, node := range nodes {
		if node.PID != nil {
			// Stop existing process
			handle := &pidHandle{pid: *node.PID}
			if err := uc.executor.Stop(ctx, handle, 30*time.Second); err != nil {
				uc.logger.Warn("Failed to stop node %d: %v", node.Index, err)
			}
		}

		// Start with new binary
		cmd := uc.buildStartCommand(node, metadata, newBinary, input.UpgradeHeight)
		newHandle, err := uc.executor.Start(ctx, cmd)
		if err != nil {
			uc.logger.Error("Failed to restart node %d: %v", node.Index, err)
			continue
		}

		pid := newHandle.PID()
		node.PID = &pid
		if err := uc.nodeRepo.Save(ctx, node); err != nil {
			uc.logger.Warn("Failed to save node %d state: %v", node.Index, err)
		}
		restarted++
	}

	// Update metadata
	if err := uc.devnetRepo.Save(ctx, metadata); err != nil {
		uc.logger.Warn("Failed to update metadata: %v", err)
	}

	uc.logger.Success("Binary switched! %d nodes restarted", restarted)
	return &dto.SwitchBinaryOutput{
		OldBinary:      oldBinary,
		NewBinary:      newBinary,
		NodesRestarted: restarted,
	}, nil
}

func (uc *SwitchBinaryUseCase) buildStartCommand(node *ports.NodeMetadata, metadata *ports.DevnetMetadata, binaryPath string, upgradeHeight int64) ports.Command {
	binary := binaryPath
	if binary == "" {
		binary = "stabled" // Default binary name
	}

	args := []string{"start", "--home", node.HomeDir}

	// Add --unsafe-skip-upgrades if upgrade height is specified
	// This is needed because the new binary may not have the upgrade handler registered
	//
	// WARN: if this upgrade adds new module, or something need to migrate, IT MUST BE NOT SET.
	//
	//if upgradeHeight > 0 {
	//	args = append(args, "--unsafe-skip-upgrades", fmt.Sprintf("%d", upgradeHeight))
	//}

	return ports.Command{
		Binary:  binary,
		Args:    args,
		WorkDir: node.HomeDir,
		LogPath: fmt.Sprintf("%s/stabled.log", node.HomeDir),
		PIDPath: fmt.Sprintf("%s/stabled.pid", node.HomeDir),
	}
}

// pidHandle is a simple ProcessHandle implementation.
type pidHandle struct {
	pid int
}

func (h *pidHandle) PID() int        { return h.pid }
func (h *pidHandle) IsRunning() bool { return false }
func (h *pidHandle) Wait() error     { return nil }
func (h *pidHandle) Kill() error     { return nil }
