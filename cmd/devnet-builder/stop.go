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

func NewStopCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop running nodes",
		Long: `Stop all running nodes in the devnet.

This command gracefully stops all validator nodes with a configurable timeout.
If nodes don't stop gracefully within the timeout, they will be forcefully terminated.

Use 'devnet-builder start' to restart the nodes later.

Examples:
  # Stop with default timeout (30s)
  devnet-builder stop

  # Stop with custom timeout
  devnet-builder stop --timeout 60s`,
		RunE: runStop,
	}

	cmd.Flags().DurationVarP(&downTimeout, "timeout", "t", 30*time.Second,
		"Graceful shutdown timeout")

	return cmd
}

func runStop(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	svc, err := getCleanService()
	if err != nil {
		return outputStopError(fmt.Errorf("failed to initialize service: %w", err))
	}

	// Check if devnet exists
	if !svc.DevnetExists() {
		if jsonMode {
			return outputStopError(fmt.Errorf("no devnet found"))
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
			return outputStopError(err)
		}
		return err
	}

	if jsonMode {
		return outputStopJSON()
	}

	output.Success("Devnet stopped successfully.")
	output.Info("Use 'devnet-builder start' to restart the nodes.")
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
