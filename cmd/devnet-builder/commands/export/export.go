// Package export provides export-related commands for devnet-builder.
package export

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/altuslabsxyz/devnet-builder/internal/application"
	domainExport "github.com/altuslabsxyz/devnet-builder/internal/domain/export"
	"github.com/altuslabsxyz/devnet-builder/internal/output"
	"github.com/altuslabsxyz/devnet-builder/types/ctxconfig"
	"github.com/spf13/cobra"
)

var (
	exportOutputDir  string
	exportForce      bool
	exportListFormat string
)

// NewExportCmd creates the export command group with all subcommands.
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

	cmd.AddCommand(
		NewListCmd(),
		NewInspectCmd(),
	)

	return cmd
}

func runExport(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	cfg := ctxconfig.FromContext(ctx)
	homeDir := cfg.HomeDir()

	svc, err := application.GetService(homeDir)
	if err != nil {
		return fmt.Errorf("failed to initialize service: %w", err)
	}

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

// NewListCmd creates the export list command.
func NewListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all exports for the current devnet",
		Long: `List all state exports in the devnet's export directory.

Displays a table with export details including:
  - Directory name
  - Block height
  - Export timestamp
  - Binary version
  - Network source
  - Total size

Examples:
  # List all exports
  devnet-builder export list

  # List exports in JSON format
  devnet-builder export list --format json`,
		RunE: runList,
	}

	cmd.Flags().StringVar(&exportListFormat, "format", "table", "Output format (table, json)")

	return cmd
}

func runList(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	cfg := ctxconfig.FromContext(ctx)
	homeDir := cfg.HomeDir()
	jsonMode := cfg.JSONMode()

	svc, err := application.GetService(homeDir)
	if err != nil {
		return fmt.Errorf("failed to initialize service: %w", err)
	}

	if !svc.DevnetExists() {
		return fmt.Errorf("no devnet found at %s", homeDir)
	}

	exportUC := svc.Container().ExportUseCase(ctx)

	result, err := exportUC.List(ctx, homeDir)
	if err != nil {
		return fmt.Errorf("failed to list exports: %w", err)
	}

	// Handle JSON output
	if exportListFormat == "json" || jsonMode {
		return outputListJSON(result)
	}

	// Handle table output
	if result.TotalCount == 0 {
		output.Info("No exports found")
		return nil
	}

	output.Bold(fmt.Sprintf("Exports (%d total, %s)", result.TotalCount, formatBytes(result.TotalSize)))
	fmt.Println("─────────────────────────────────────────────────────────────────────────────────────────")
	fmt.Printf("%-35s  %-10s  %-20s  %-12s  %s\n", "DIRECTORY", "HEIGHT", "TIMESTAMP", "VERSION", "NETWORK")
	fmt.Println("─────────────────────────────────────────────────────────────────────────────────────────")

	for _, exp := range result.Exports {
		timestamp := exp.Timestamp.Format("2006-01-02 15:04:05")

		dirName := exp.DirectoryName
		if len(dirName) > 35 {
			dirName = dirName[:32] + "..."
		}

		fmt.Printf("%-35s  %-10d  %-20s  %-12s  %s\n",
			dirName,
			exp.BlockHeight,
			timestamp,
			exp.BinaryVersion,
			exp.NetworkSource,
		)
	}

	fmt.Println()
	output.Info("Total exports: %d (%s)", result.TotalCount, formatBytes(result.TotalSize))

	return nil
}

func outputListJSON(result interface{}) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

// NewInspectCmd creates the export inspect command.
func NewInspectCmd() *cobra.Command {
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
		RunE: runInspect,
	}

	return cmd
}

func runInspect(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	exportPath := args[0]

	cfg := ctxconfig.FromContext(ctx)
	homeDir := cfg.HomeDir()
	svc, err := application.GetService(homeDir)
	if err != nil {
		return fmt.Errorf("failed to initialize service: %w", err)
	}

	exportUC := svc.Container().ExportUseCase(ctx)

	result, err := exportUC.Inspect(ctx, exportPath)
	if err != nil {
		return fmt.Errorf("failed to inspect export: %w", err)
	}

	// Calculate genesis checksum
	metadata, ok := result.Metadata.(*domainExport.ExportMetadata)
	if !ok {
		return fmt.Errorf("invalid metadata type")
	}

	genesisPath := filepath.Join(exportPath, fmt.Sprintf("genesis-%d-%s.json",
		metadata.BlockHeight,
		metadata.BinaryHashPrefix))

	genesisChecksum, err := calculateFileChecksum(genesisPath)
	if err != nil {
		output.Warn("Failed to calculate genesis checksum: %v", err)
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

// formatBytes formats bytes as human-readable string.
func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
