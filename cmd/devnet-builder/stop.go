package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/b-harvest/devnet-builder/internal/output"
	"github.com/spf13/cobra"
)

var (
	downTimeout time.Duration
)

func NewDownCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "down",
		Short: "Stop running nodes",
		Long: `Stop all running nodes in the devnet.

This command gracefully stops all validator nodes with a configurable timeout.
If nodes don't stop gracefully within the timeout, they will be forcefully terminated.

Use 'devnet-builder up' to restart the nodes later.

Examples:
  # Stop with default timeout (30s)
  devnet-builder down

  # Stop with custom timeout
  devnet-builder down --timeout 60s`,
		RunE: runDown,
	}

	cmd.Flags().DurationVarP(&downTimeout, "timeout", "t", 30*time.Second,
		"Graceful shutdown timeout")

	return cmd
}

func runDown(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	svc, err := getCleanService()
	if err != nil {
		return outputDownError(fmt.Errorf("failed to initialize service: %w", err))
	}

	// Check if devnet exists
	if !svc.DevnetExists() {
		if jsonMode {
			return outputDownError(fmt.Errorf("no devnet found"))
		}
		return fmt.Errorf("no devnet found at %s", homeDir)
	}

	// Stop nodes
	if !jsonMode {
		output.Info("Stopping devnet nodes...")
	}

	_, err = svc.Stop(ctx, downTimeout)
	if err != nil {
		if jsonMode {
			return outputDownError(err)
		}
		return err
	}

	if jsonMode {
		return outputDownJSON()
	}

	output.Success("Devnet stopped successfully.")
	output.Info("Use 'devnet-builder up' to restart the nodes.")
	return nil
}

func outputDownJSON() error {
	result := map[string]interface{}{
		"status":  "success",
		"message": "Devnet stopped successfully",
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(data))
	return nil
}

func outputDownError(err error) error {
	result := map[string]interface{}{
		"error":   true,
		"code":    "DEVNET_NOT_RUNNING",
		"message": err.Error(),
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(data))
	return err
}
