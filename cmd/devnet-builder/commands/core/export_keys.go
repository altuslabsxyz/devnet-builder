package core

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/altuslabsxyz/devnet-builder/cmd/devnet-builder/shared"
	"github.com/altuslabsxyz/devnet-builder/internal/application"
	"github.com/altuslabsxyz/devnet-builder/internal/application/dto"
	"github.com/spf13/cobra"
)

var (
	exportKeysType string
)

// NewExportKeysCmd creates the export-keys command.
func NewExportKeysCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export-keys",
		Short: "Export validator and account keys as JSON",
		Long: `Export validator and account keys in JSON format.

This command exports key information from the devnet nodes.
Useful for scripting and integration testing.

Examples:
  # Export all keys (validators and accounts)
  devnet-builder export-keys

  # Export only validators
  devnet-builder export-keys --type validators

  # Export only accounts
  devnet-builder export-keys --type accounts`,
		RunE: runExportKeys,
	}

	cmd.Flags().StringVar(&exportKeysType, "type", "all",
		"Key type to export (validators, accounts, all)")

	return cmd
}

func runExportKeys(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	homeDir := shared.GetHomeDir()

	svc, err := application.GetService(homeDir)
	if err != nil {
		return outputExportKeysError(fmt.Errorf("failed to initialize service: %w", err))
	}

	// Validate inputs first
	if exportKeysType != "all" && exportKeysType != "validators" && exportKeysType != "accounts" {
		return outputExportKeysError(fmt.Errorf("invalid type: %s (must be 'validators', 'accounts', or 'all')", exportKeysType))
	}

	// Check if devnet exists
	if !svc.DevnetExists() {
		return outputExportKeysError(fmt.Errorf("no devnet found at %s", homeDir))
	}

	// Export keys using service
	export, err := svc.ExportKeys(ctx, exportKeysType)
	if err != nil {
		return outputExportKeysError(err)
	}

	// Output as JSON only
	return outputExportKeysJSON(export)
}

func outputExportKeysJSON(export *dto.ExportKeysOutput) error {
	data, err := json.MarshalIndent(export, "", "  ")
	if err != nil {
		return outputExportKeysError(err)
	}
	fmt.Println(string(data))
	return nil
}

func outputExportKeysError(err error) error {
	result := map[string]interface{}{
		"error":   true,
		"code":    "EXPORT_FAILED",
		"message": err.Error(),
	}
	data, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(data))
	return err
}
