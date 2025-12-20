// Package devnet contains UseCases for devnet lifecycle management.
package devnet

import (
	"context"
	"fmt"
	"time"

	"github.com/b-harvest/devnet-builder/internal/application/dto"
	"github.com/b-harvest/devnet-builder/internal/application/ports"
)

// ProvisionUseCase handles devnet provisioning.
type ProvisionUseCase struct {
	devnetRepo    ports.DevnetRepository
	nodeRepo      ports.NodeRepository
	snapshotSvc   ports.SnapshotFetcher
	genesisSvc    ports.GenesisFetcher
	networkModule ports.NetworkModule
	logger        ports.Logger
}

// NewProvisionUseCase creates a new ProvisionUseCase.
func NewProvisionUseCase(
	devnetRepo ports.DevnetRepository,
	nodeRepo ports.NodeRepository,
	snapshotSvc ports.SnapshotFetcher,
	genesisSvc ports.GenesisFetcher,
	networkModule ports.NetworkModule,
	logger ports.Logger,
) *ProvisionUseCase {
	return &ProvisionUseCase{
		devnetRepo:    devnetRepo,
		nodeRepo:      nodeRepo,
		snapshotSvc:   snapshotSvc,
		genesisSvc:    genesisSvc,
		networkModule: networkModule,
		logger:        logger,
	}
}

// Execute provisions a new devnet.
func (uc *ProvisionUseCase) Execute(ctx context.Context, input dto.ProvisionInput) (*dto.ProvisionOutput, error) {
	uc.logger.Info("Provisioning devnet...")

	// Check if devnet already exists
	if uc.devnetRepo.Exists(input.HomeDir) {
		return nil, fmt.Errorf("devnet already exists at %s", input.HomeDir)
	}

	// Create metadata
	metadata := &ports.DevnetMetadata{
		HomeDir:         input.HomeDir,
		NetworkName:     input.NetworkName,
		NetworkVersion:  input.NetworkVersion,
		NumValidators:   input.NumValidators,
		NumAccounts:     input.NumAccounts,
		ExecutionMode:   input.ExecutionMode,
		Status:          ports.StateCreated,
		DockerImage:     input.DockerImage,
		CreatedAt:       time.Now(),
	}

	// Download snapshot if URL provided
	if input.SnapshotURL != "" {
		uc.logger.Info("Downloading snapshot...")
		if err := uc.downloadAndExtractSnapshot(ctx, input); err != nil {
			return nil, fmt.Errorf("failed to download snapshot: %w", err)
		}
	}

	// Export genesis
	uc.logger.Info("Exporting genesis...")
	genesis, err := uc.exportGenesis(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to export genesis: %w", err)
	}

	// Generate validators
	uc.logger.Info("Generating validators...")
	nodes, err := uc.generateValidators(ctx, input, metadata, genesis)
	if err != nil {
		return nil, fmt.Errorf("failed to generate validators: %w", err)
	}

	// Update metadata
	metadata.Status = ports.StateProvisioned
	now := time.Now()
	metadata.LastProvisioned = &now

	// Save metadata
	if err := uc.devnetRepo.Save(ctx, metadata); err != nil {
		return nil, fmt.Errorf("failed to save metadata: %w", err)
	}

	// Save nodes
	for _, node := range nodes {
		if err := uc.nodeRepo.Save(ctx, node); err != nil {
			uc.logger.Warn("Failed to save node %d: %v", node.Index, err)
		}
	}

	// Build output
	output := &dto.ProvisionOutput{
		HomeDir:       input.HomeDir,
		ChainID:       metadata.ChainID,
		GenesisPath:   metadata.GenesisPath,
		NumValidators: input.NumValidators,
		NumAccounts:   input.NumAccounts,
		Nodes:         make([]dto.NodeInfo, len(nodes)),
	}

	for i, node := range nodes {
		output.Nodes[i] = dto.NodeInfo{
			Index:   node.Index,
			Name:    node.Name,
			HomeDir: node.HomeDir,
			NodeID:  node.NodeID,
			Ports:   node.Ports,
		}
	}

	uc.logger.Success("Provision complete!")
	return output, nil
}

func (uc *ProvisionUseCase) downloadAndExtractSnapshot(ctx context.Context, input dto.ProvisionInput) error {
	// Download snapshot
	snapshotPath := fmt.Sprintf("%s/snapshot.tar.gz", input.HomeDir)
	if err := uc.snapshotSvc.Download(ctx, input.SnapshotURL, snapshotPath); err != nil {
		return err
	}

	// Extract snapshot
	return uc.snapshotSvc.Extract(ctx, snapshotPath, input.HomeDir)
}

func (uc *ProvisionUseCase) exportGenesis(ctx context.Context, input dto.ProvisionInput) ([]byte, error) {
	// Export genesis from chain or fetch from RPC
	return uc.genesisSvc.ExportFromChain(ctx, input.HomeDir)
}

func (uc *ProvisionUseCase) generateValidators(ctx context.Context, input dto.ProvisionInput, metadata *ports.DevnetMetadata, genesis []byte) ([]*ports.NodeMetadata, error) {
	nodes := make([]*ports.NodeMetadata, input.NumValidators)

	defaultPorts := uc.networkModule.DefaultPorts()

	for i := 0; i < input.NumValidators; i++ {
		nodeDir := fmt.Sprintf("%s/devnet/node%d", input.HomeDir, i)
		nodes[i] = &ports.NodeMetadata{
			Index:   i,
			Name:    fmt.Sprintf("node%d", i),
			HomeDir: nodeDir,
			ChainID: metadata.ChainID,
			Ports:   calculateNodePorts(defaultPorts, i),
		}
	}

	return nodes, nil
}

// calculateNodePorts calculates port assignments for a node.
func calculateNodePorts(basePorts ports.PortConfig, index int) ports.PortConfig {
	offset := index * 100
	return ports.PortConfig{
		RPC:     basePorts.RPC + offset,
		P2P:     basePorts.P2P + offset,
		GRPC:    basePorts.GRPC + offset,
		API:     basePorts.API + offset,
		EVM:     basePorts.EVM + offset,
		EVMWS:   basePorts.EVMWS + offset,
		PProf:   basePorts.PProf + offset,
		Rosetta: basePorts.Rosetta + offset,
	}
}
