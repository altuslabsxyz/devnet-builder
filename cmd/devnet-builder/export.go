package main

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/b-harvest/devnet-builder/internal/output"
	"github.com/spf13/cobra"
)

var (
	exportOutputDir string
	exportForce     bool
)

func NewExportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export blockchain state at current height",
		Long: `Export the current blockchain state to a genesis file.

This command captures the blockchain state at the current height and saves it
to an export directory with comprehensive metadata. The export includes:
  - Genesis file with exported state
  - Metadata JSON with binary info, height, and configuration
  - SHA256 hash of the binary used for export

The export can be used for:
  - Debugging and testing
  - Chain migrations and forks
  - State snapshots for development
  - Automated exports during chain upgrades

Examples:
  # Export state to default directory (homeDir/exports)
  devnet-builder export

  # Export to custom directory
  devnet-builder export --output-dir /path/to/exports

  # Force overwrite existing export
  devnet-builder export --force`,
		RunE: runExport,
	}

	cmd.Flags().StringVarP(&exportOutputDir, "output-dir", "o", "", "Custom output directory (default: {homeDir}/exports)")
	cmd.Flags().BoolVarP(&exportForce, "force", "f", false, "Overwrite existing export directory")

	// Add subcommands
	cmd.AddCommand(
		NewExportListCmd(),
		NewExportInspectCmd(),
	)

	return cmd
}

func runExport(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Get clean service
	svc, err := getCleanService()
	if err != nil {
		return fmt.Errorf("failed to initialize service: %w", err)
	}

	// Check if devnet exists
	if !svc.DevnetExists() {
		return fmt.Errorf("no devnet found at %s\nRun 'devnet-builder deploy' first", homeDir)
	}

	// Execute export
	output.Info("Exporting blockchain state...")
	result, err := svc.Export(ctx, exportOutputDir, exportForce)
	if err != nil {
		return fmt.Errorf("export failed: %w", err)
	}

	// Display results
	output.Success("Export completed successfully!")
	fmt.Println()
	output.Bold("Export Details")
	fmt.Println("─────────────────────────────────────────────────────────")
	fmt.Printf("Export Path:   %s\n", result.ExportPath)
	fmt.Printf("Block Height:  %d\n", result.BlockHeight)
	fmt.Printf("Genesis File:  %s\n", filepath.Base(result.GenesisPath))
	fmt.Printf("Metadata File: %s\n", filepath.Base(result.MetadataPath))

	if result.WasRunning {
		fmt.Printf("Chain Status:  Running\n")
	} else {
		fmt.Printf("Chain Status:  Stopped\n")
	}

	if len(result.Warnings) > 0 {
		fmt.Println()
		output.Warn("Warnings:")
		for _, warning := range result.Warnings {
			fmt.Printf("  - %s\n", warning)
		}
	}

	return nil
}
