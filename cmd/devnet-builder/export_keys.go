package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/b-harvest/devnet-builder/internal/application/dto"
	"github.com/spf13/cobra"
)

var (
	exportType string
)

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

	cmd.Flags().StringVar(&exportType, "type", "all",
		"Key type to export (validators, accounts, all)")

	return cmd
}

func runExportKeys(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	svc, err := getCleanService()
	if err != nil {
		return outputExportKeysError(fmt.Errorf("failed to initialize service: %w", err))
	}

	// Validate inputs first
	if exportType != "all" && exportType != "validators" && exportType != "accounts" {
		return outputExportKeysError(fmt.Errorf("invalid type: %s (must be 'validators', 'accounts', or 'all')", exportType))
	}

	// Check if devnet exists
	if !svc.DevnetExists() {
		return outputExportKeysError(fmt.Errorf("no devnet found at %s", homeDir))
	}

	// Export keys using service
	export, err := svc.ExportKeys(ctx, exportType)
	if err != nil {
		return outputExportKeysError(err)
	}

	// Output as JSON only
	return outputExportKeysJSONClean(export)
}

func outputExportKeysJSONClean(export *dto.ExportKeysOutput) error {
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
