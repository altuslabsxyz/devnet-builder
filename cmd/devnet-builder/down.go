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
	logger := output.DefaultLogger

	// Load devnet using consolidated helper
	loaded, err := loadDevnetOrFail(logger)
	if err != nil {
		if jsonMode {
			return outputDownError(err)
		}
		return err
	}
	d := loaded.Devnet

	// Stop nodes
	if !jsonMode {
		output.Info("Stopping devnet nodes...")
	}

	if err := d.Stop(ctx, downTimeout); err != nil {
		if jsonMode {
			return outputDownError(err)
		}
		return fmt.Errorf("failed to stop devnet: %w", err)
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
