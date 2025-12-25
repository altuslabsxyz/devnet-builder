package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/b-harvest/devnet-builder/internal/output"
	"github.com/spf13/cobra"
)

func NewExportInspectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "inspect <export-path>",
		Short: "Inspect an export and display detailed information",
		Long: `Inspect a specific export directory and display comprehensive details.

Shows:
  - Full metadata contents
  - Genesis file checksum (SHA256)
  - Completeness status
  - Missing files (if incomplete)
  - Total export size

Examples:
  # Inspect an export
  devnet-builder export inspect ~/.stable-devnet/devnet-xyz/exports/mainnet-abc12345-1000000-20240115120000

  # Inspect with relative path
  devnet-builder export inspect exports/mainnet-abc12345-1000000-20240115120000`,
		Args: cobra.ExactArgs(1),
		RunE: runExportInspect,
	}

	return cmd
}

func runExportInspect(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	exportPath := args[0]

	// Get clean service
	svc, err := getCleanService()
	if err != nil {
		return fmt.Errorf("failed to initialize service: %w", err)
	}

	// Get export use case
	exportUC := svc.Container().ExportUseCase()

	// Inspect export
	result, err := exportUC.Inspect(ctx, exportPath)
	if err != nil {
		return fmt.Errorf("failed to inspect export: %w", err)
	}

	// Calculate genesis checksum - construct path from metadata
	genesisPath := filepath.Join(exportPath, fmt.Sprintf("genesis-%d-%s.json",
		result.Metadata.BlockHeight,
		result.Metadata.BinaryHashPrefix))

	genesisChecksum, err := calculateFileChecksum(genesisPath)
	if err != nil {
		output.Warning("Failed to calculate genesis checksum: %v", err)
		genesisChecksum = "N/A"
	}

	// Display results
	output.Bold("Export Inspection")
	fmt.Println("─────────────────────────────────────────────────────────")
	fmt.Println()

	// Status
	if result.IsComplete {
		output.Success("Status: Complete ✓")
	} else {
		output.Error("Status: Incomplete ✗")
		if len(result.MissingFiles) > 0 {
			fmt.Println("\nMissing files:")
			for _, file := range result.MissingFiles {
				fmt.Printf("  - %s\n", file)
			}
		}
	}

	fmt.Println()
	fmt.Printf("Export Path:      %s\n", exportPath)
	fmt.Printf("Total Size:       %s\n", formatBytes(result.SizeBytes))
	fmt.Printf("Genesis Checksum: %s\n", genesisChecksum)

	// Metadata section
	fmt.Println()
	output.Bold("Metadata")
	fmt.Println("─────────────────────────────────────────────────────────")

	// Pretty print metadata as JSON
	metadataJSON, err := json.MarshalIndent(result.Metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}
	fmt.Println(string(metadataJSON))

	return nil
}

func calculateFileChecksum(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hasher.Sum(nil)), nil
}
