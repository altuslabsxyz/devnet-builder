package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/b-harvest/devnet-builder/internal/output"
	"github.com/spf13/cobra"
)

var exportListFormat string

func NewExportListCmd() *cobra.Command {
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
		RunE: runExportList,
	}

	cmd.Flags().StringVar(&exportListFormat, "format", "table", "Output format (table, json)")

	return cmd
}

func runExportList(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Get clean service
	svc, err := getCleanService()
	if err != nil {
		return fmt.Errorf("failed to initialize service: %w", err)
	}

	// Check if devnet exists
	if !svc.DevnetExists() {
		return fmt.Errorf("no devnet found at %s", homeDir)
	}

	// Get export use case
	exportUC := svc.Container().ExportUseCase()

	// List exports
	result, err := exportUC.List(ctx, homeDir)
	if err != nil {
		return fmt.Errorf("failed to list exports: %w", err)
	}

	// Handle JSON output
	if exportListFormat == "json" {
		return outputExportListJSON(result)
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
		// Format timestamp
		timestamp := exp.Timestamp.Format("2006-01-02 15:04:05")

		// Truncate directory name if too long
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

func outputExportListJSON(result interface{}) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

// formatBytes already exists in cache.go, removed to avoid redeclaration
