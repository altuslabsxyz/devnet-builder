package upgrade

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/b-harvest/devnet-builder/internal/devnet"
	"github.com/b-harvest/devnet-builder/internal/output"
)

// GenesisExportOptions contains options for genesis export.
type GenesisExportOptions struct {
	HomeDir     string
	ExportDir   string
	Metadata    *devnet.DevnetMetadata
	Logger      *output.Logger
	UpgradeName string
	PreUpgrade  bool // true for pre-upgrade, false for post-upgrade
}

// genesisExportResult is the internal result of a genesis export operation.
type genesisExportResult struct {
	Path      string
	Height    int64
	Timestamp time.Time
	Type      string // "pre-upgrade" or "post-upgrade"
}

// ExportGenesis exports the current genesis state.
func ExportGenesis(ctx context.Context, opts *GenesisExportOptions) (*GenesisSnapshot, error) {
	logger := opts.Logger
	if logger == nil {
		logger = output.DefaultLogger
	}

	// Determine export type
	exportType := "post-upgrade"
	if opts.PreUpgrade {
		exportType = "pre-upgrade"
	}

	// Create export directory if it doesn't exist
	exportDir := opts.ExportDir
	if exportDir == "" {
		exportDir = filepath.Join(opts.HomeDir, "devnet", "genesis-snapshots")
	}
	if err := os.MkdirAll(exportDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create export directory: %w", err)
	}

	// Get current height
	rpc := NewRPCClient("localhost", DefaultRPCPort)
	height, err := rpc.GetCurrentHeight(ctx)
	if err != nil {
		logger.Warn("Could not get current height for genesis export: %v", err)
		height = 0
	}

	// Generate filename
	timestamp := time.Now()
	filename := fmt.Sprintf("%s_%s_%d.json", opts.UpgradeName, exportType, timestamp.Unix())
	exportPath := filepath.Join(exportDir, filename)

	logger.Debug("Exporting genesis to %s", exportPath)

	// Export genesis using stabled export
	if err := exportGenesisFile(ctx, opts, exportPath, logger); err != nil {
		return nil, err
	}

	// Get file size
	info, err := os.Stat(exportPath)
	var fileSize int64
	if err == nil {
		fileSize = info.Size()
	}

	// Determine snapshot type
	snapshotType := SnapshotPostUpgrade
	if opts.PreUpgrade {
		snapshotType = SnapshotPreUpgrade
	}

	return &GenesisSnapshot{
		Type:       snapshotType,
		FilePath:   exportPath,
		Height:     height,
		ChainID:    opts.Metadata.ChainID,
		ExportedAt: timestamp,
		SizeBytes:  fileSize,
	}, nil
}

// exportGenesisFile executes the genesis export command.
func exportGenesisFile(ctx context.Context, opts *GenesisExportOptions, outputPath string, logger *output.Logger) error {
	// Determine node home directory (use first node)
	nodeDir := filepath.Join(opts.HomeDir, "devnet", "node0")

	var cmd *exec.Cmd

	if opts.Metadata.ExecutionMode == devnet.ModeDocker {
		// Use docker exec to run export
		containerName := "stable-devnet-node0"
		cmd = exec.CommandContext(ctx, "docker", "exec", containerName,
			"stabled", "export",
			"--home", "/root/.stabled",
		)
	} else {
		// Use local binary
		binary := "stabled"
		cmd = exec.CommandContext(ctx, binary, "export",
			"--home", nodeDir,
		)
	}

	// Create output file
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	cmd.Stdout = outFile
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		// Clean up failed export
		os.Remove(outputPath)
		return fmt.Errorf("genesis export failed: %w", err)
	}

	// Verify file was created and has content
	info, err := os.Stat(outputPath)
	if err != nil || info.Size() == 0 {
		os.Remove(outputPath)
		return fmt.Errorf("genesis export produced empty or missing file")
	}

	logger.Debug("Genesis exported: %s (%d bytes)", outputPath, info.Size())
	return nil
}
