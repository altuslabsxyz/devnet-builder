package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/stablelabs/stable-devnet/internal/devnet"
)

var (
	exportFormat string
	exportType   string
)

func NewExportKeysCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export-keys",
		Short: "Export validator and account keys",
		Long: `Export validator and account keys in various formats.

This command exports key information from the devnet nodes.
Useful for scripting and integration testing.

Examples:
  # Human-readable format
  devnet-builder export-keys

  # JSON format
  devnet-builder export-keys --format json

  # Environment variables (eval-able)
  eval $(devnet-builder export-keys --format env)

  # Export only validators
  devnet-builder export-keys --type validators`,
		RunE: runExportKeys,
	}

	cmd.Flags().StringVar(&exportFormat, "format", "text",
		"Output format (text, json, env)")
	cmd.Flags().StringVar(&exportType, "type", "all",
		"Key type to export (validators, accounts, all)")

	return cmd
}

func runExportKeys(cmd *cobra.Command, args []string) error {
	// Check if devnet exists
	if !devnet.DevnetExists(homeDir) {
		if jsonMode {
			return outputExportKeysError(fmt.Errorf("no devnet found"))
		}
		return fmt.Errorf("no devnet found at %s", homeDir)
	}

	// Validate inputs
	if exportFormat != "text" && exportFormat != "json" && exportFormat != "env" {
		return fmt.Errorf("invalid format: %s (must be 'text', 'json', or 'env')", exportFormat)
	}
	if exportType != "all" && exportType != "validators" && exportType != "accounts" {
		return fmt.Errorf("invalid type: %s (must be 'validators', 'accounts', or 'all')", exportType)
	}

	// Export keys
	export, err := devnet.ExportKeys(homeDir, exportType)
	if err != nil {
		if jsonMode || exportFormat == "json" {
			return outputExportKeysError(err)
		}
		return fmt.Errorf("failed to export keys: %w", err)
	}

	// Format output
	switch exportFormat {
	case "json":
		return outputExportKeysJSON(export)
	case "env":
		fmt.Print(devnet.FormatKeysEnv(export))
	default:
		fmt.Print(devnet.FormatKeysText(export))
	}

	return nil
}

func outputExportKeysJSON(export *devnet.KeyExport) error {
	data, err := json.MarshalIndent(export, "", "  ")
	if err != nil {
		return err
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
