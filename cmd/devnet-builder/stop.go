package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/stablelabs/stable-devnet/internal/devnet"
	"github.com/stablelabs/stable-devnet/internal/output"
)

var (
	stopTimeout time.Duration
)

func NewStopCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:        "stop",
		Short:      "Stop the running devnet (deprecated: use 'down' instead)",
		Deprecated: "use 'down' instead",
		Long: `Stop all running nodes in the devnet.

DEPRECATED: This command is deprecated. Use 'devnet-builder down' instead.

This command gracefully stops all validator nodes with a configurable timeout.
If nodes don't stop gracefully within the timeout, they will be forcefully terminated.

Examples:
  # Stop with default timeout (30s)
  devnet-builder down

  # Stop with custom timeout
  devnet-builder down --timeout 60s`,
		RunE: func(cmd *cobra.Command, args []string) error {
			PrintDeprecationWarning("stop", "down")
			return runStop(cmd, args)
		},
	}

	cmd.Flags().DurationVarP(&stopTimeout, "timeout", "t", 30*time.Second,
		"Graceful shutdown timeout")

	return cmd
}

func runStop(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	logger := output.DefaultLogger

	// Check if devnet exists
	if !devnet.DevnetExists(homeDir) {
		if jsonMode {
			return outputStopError(fmt.Errorf("no devnet found"))
		}
		return fmt.Errorf("no devnet found at %s", homeDir)
	}

	// Load devnet
	d, err := devnet.LoadDevnetWithNodes(homeDir, logger)
	if err != nil {
		if jsonMode {
			return outputStopError(err)
		}
		return fmt.Errorf("failed to load devnet: %w", err)
	}

	// Stop nodes
	if !jsonMode {
		output.Info("Stopping devnet nodes...")
	}

	if err := d.Stop(ctx, stopTimeout); err != nil {
		if jsonMode {
			return outputStopError(err)
		}
		return fmt.Errorf("failed to stop devnet: %w", err)
	}

	if jsonMode {
		return outputStopJSON()
	}

	output.Success("Devnet stopped successfully.")
	return nil
}

func outputStopJSON() error {
	result := map[string]interface{}{
		"status":  "success",
		"message": "Devnet stopped successfully",
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(data))
	return nil
}

func outputStopError(err error) error {
	result := map[string]interface{}{
		"error":   true,
		"code":    "DEVNET_NOT_RUNNING",
		"message": err.Error(),
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(data))
	return err
}
