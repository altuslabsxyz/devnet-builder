package devnet

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/b-harvest/devnet-builder/internal/application/dto"
	"github.com/b-harvest/devnet-builder/internal/application/ports"
	domainExport "github.com/b-harvest/devnet-builder/internal/domain/export"
	infraExport "github.com/b-harvest/devnet-builder/internal/infrastructure/export"
)

// ExportUseCase handles blockchain state export operations.
type ExportUseCase struct {
	devnetRepo     ports.DevnetRepository
	nodeRepo       ports.NodeRepository
	exportRepo     ports.ExportRepository
	hashCalc       *infraExport.HashCalculator
	heightResolver *infraExport.HeightResolver
	executor       *infraExport.ExportExecutor
	logger         ports.Logger
}

// NewExportUseCase creates a new ExportUseCase.
func NewExportUseCase(
	devnetRepo ports.DevnetRepository,
	nodeRepo ports.NodeRepository,
	exportRepo ports.ExportRepository,
	logger ports.Logger,
) *ExportUseCase {
	return &ExportUseCase{
		devnetRepo:     devnetRepo,
		nodeRepo:       nodeRepo,
		exportRepo:     exportRepo,
		hashCalc:       infraExport.NewHashCalculator(),
		heightResolver: infraExport.NewHeightResolver(),
		executor:       infraExport.NewExportExecutor(),
		logger:         logger,
	}
}

// Execute performs a blockchain state export at the current height.
func (uc *ExportUseCase) Execute(ctx context.Context, input dto.ExportInput) (*dto.ExportOutput, error) {
	// Step 1: Load devnet metadata
	uc.logger.Info("Loading devnet configuration...")
	devnet, err := uc.devnetRepo.Load(ctx, input.HomeDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load devnet: %w", err)
	}

	// Step 2: Check if devnet is running and find an active node
	nodes, err := uc.nodeRepo.LoadAll(ctx, input.HomeDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load nodes: %w", err)
	}

	wasRunning := false
	var rpcURL string
	var activeNode *ports.NodeMetadata
	for _, node := range nodes {
		if node.PID != nil && *node.PID > 0 {
			wasRunning = true
			rpcURL = fmt.Sprintf("http://localhost:%d", node.Ports.RPC)
			activeNode = node
			break
		}
	}

	// Step 3: Get current block height
	uc.logger.Info("Querying current block height...")
	var blockHeight int64
	if wasRunning && rpcURL != "" && activeNode != nil {
		blockHeight, err = uc.heightResolver.GetCurrentHeight(ctx, rpcURL)
		if err != nil {
			return nil, fmt.Errorf("failed to get block height: %w", err)
		}
		uc.logger.Info("Current block height: %d", blockHeight)
	} else {
		return nil, fmt.Errorf("devnet is not running; cannot determine block height")
	}

	// Step 4: Determine binary path and calculate hash
	binaryPath := devnet.CustomBinaryPath
	if binaryPath == "" && devnet.ExecutionMode == ports.ModeLocal {
		// Use symlinked binary from cache
		cachePath := filepath.Join(os.Getenv("HOME"), ".stable-devnet", "cache", "binaries", devnet.BinaryName)
		if _, err := os.Stat(cachePath); err == nil {
			binaryPath = cachePath
		}
	}

	if binaryPath == "" {
		return nil, fmt.Errorf("cannot determine binary path for export")
	}

	uc.logger.Info("Calculating binary hash...")
	binaryHash, err := uc.hashCalc.CalculateHash(binaryPath)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate binary hash: %w", err)
	}

	// Step 5: Get binary version
	binaryVersion, err := uc.executor.GetBinaryVersion(ctx, binaryPath)
	if err != nil {
		uc.logger.Debug("Failed to get binary version: %v", err)
		binaryVersion = devnet.CurrentVersion
	}

	// Step 6: Create export entities
	timestamp := time.Now()

	binaryInfo, err := domainExport.NewBinaryInfo(
		binaryPath,
		"", // Docker image (empty for local mode)
		binaryHash,
		binaryVersion,
		domainExport.ExecutionModeLocal,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create binary info: %w", err)
	}

	metadata, err := domainExport.NewExportMetadata(
		timestamp,
		blockHeight,
		devnet.NetworkName,
		blockHeight, // Fork height same as export height
		binaryPath,
		binaryHash,
		binaryVersion,
		devnet.DockerImage,
		devnet.ChainID,
		devnet.NumValidators,
		devnet.NumAccounts,
		string(devnet.ExecutionMode),
		devnet.HomeDir,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create metadata: %w", err)
	}

	// Step 7: Determine output directory
	outputDir := input.OutputDir
	if outputDir == "" {
		outputDir = filepath.Join(input.HomeDir, "exports")
	}

	// Generate directory and file names using utility functions
	exportDirName := domainExport.GenerateDirectoryName(devnet.NetworkName, binaryInfo.GetIdentifier(), blockHeight, timestamp)
	exportPath := filepath.Join(outputDir, exportDirName)

	// Check if export directory already exists
	if _, err := os.Stat(exportPath); err == nil && !input.Force {
		return nil, fmt.Errorf("export directory already exists: %s (use --force to overwrite)", exportPath)
	}

	// Step 8: Create export directory
	uc.logger.Info("Creating export directory: %s", exportPath)
	if err := os.MkdirAll(exportPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create export directory: %w", err)
	}

	// Step 9: Execute export command
	genesisFileName := domainExport.GenerateGenesisFileName(blockHeight, binaryInfo.GetIdentifier())
	genesisPath := filepath.Join(exportPath, genesisFileName)

	uc.logger.Info("Exporting state at height %d (this may take a few minutes)...", blockHeight)
	// Use the active node's home directory (contains config/genesis.json and data/)
	_, err = uc.executor.ExportAtHeight(ctx, binaryPath, activeNode.HomeDir, blockHeight, genesisPath)
	if err != nil {
		// Clean up failed export
		os.RemoveAll(exportPath)
		return nil, fmt.Errorf("export failed: %w", err)
	}

	// Step 10: Create final export entity with correct paths
	export, err := domainExport.NewExport(
		exportPath,
		timestamp,
		blockHeight,
		devnet.NetworkName,
		binaryInfo,
		metadata,
		genesisPath,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create export: %w", err)
	}

	// Step 11: Save export metadata
	uc.logger.Info("Saving export metadata...")
	if err := uc.exportRepo.Save(ctx, export); err != nil {
		return nil, fmt.Errorf("failed to save export metadata: %w", err)
	}

	// Step 12: Build output
	output := &dto.ExportOutput{
		ExportPath:   exportPath,
		BlockHeight:  blockHeight,
		GenesisPath:  genesisPath,
		MetadataPath: export.GetMetadataPath(),
		WasRunning:   wasRunning,
		Warnings:     []string{},
	}

	uc.logger.Success("Export completed successfully!")
	uc.logger.Info("  Export directory: %s", exportPath)
	uc.logger.Info("  Block height: %d", blockHeight)
	uc.logger.Info("  Genesis file: %s", genesisPath)
	uc.logger.Info("  Metadata file: %s", output.MetadataPath)

	return output, nil
}

// List returns all exports for a devnet.
func (uc *ExportUseCase) List(ctx context.Context, homeDir string) (*dto.ExportListOutput, error) {
	// Load all exports
	exportsInterface, err := uc.exportRepo.ListForDevnet(ctx, homeDir)
	if err != nil {
		return nil, fmt.Errorf("failed to list exports: %w", err)
	}

	exports, ok := exportsInterface.([]*domainExport.Export)
	if !ok {
		return nil, fmt.Errorf("invalid export list type")
	}

	// Build summaries
	summaries := make([]*dto.ExportSummary, 0, len(exports))
	var totalSize int64

	for _, exp := range exports {
		// Calculate directory size
		size, err := calculateDirectorySize(exp.DirectoryPath)
		if err != nil {
			uc.logger.Debug("Failed to calculate size for %s: %v", exp.DirectoryPath, err)
			size = 0
		}
		totalSize += size

		summaries = append(summaries, &dto.ExportSummary{
			DirectoryName: filepath.Base(exp.DirectoryPath),
			DirectoryPath: exp.DirectoryPath,
			BlockHeight:   exp.BlockHeight,
			Timestamp:     exp.ExportTimestamp,
			BinaryVersion: exp.BinaryInfo.Version,
			NetworkSource: exp.NetworkSource,
			SizeBytes:     size,
		})
	}

	return &dto.ExportListOutput{
		Exports:    summaries,
		TotalCount: len(summaries),
		TotalSize:  totalSize,
	}, nil
}

// Inspect returns detailed information about a specific export.
func (uc *ExportUseCase) Inspect(ctx context.Context, exportPath string) (*dto.ExportInspectOutput, error) {
	// Validate export
	resultInterface, err := uc.exportRepo.Validate(ctx, exportPath)
	if err != nil && err != domainExport.ErrExportIncomplete {
		return nil, fmt.Errorf("failed to validate export: %w", err)
	}

	result, ok := resultInterface.(*infraExport.ValidationResult)
	if !ok {
		return nil, fmt.Errorf("invalid validation result type")
	}

	// Calculate directory size
	size, _ := calculateDirectorySize(exportPath)

	output := &dto.ExportInspectOutput{
		Metadata:        result.Export.Metadata,
		GenesisChecksum: "", // TODO: Calculate SHA256 of genesis file
		IsComplete:      result.IsComplete,
		MissingFiles:    result.MissingFiles,
		SizeBytes:       size,
	}

	return output, nil
}

// calculateDirectorySize returns the total size of a directory in bytes.
func calculateDirectorySize(path string) (int64, error) {
	var size int64

	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})

	return size, err
}
