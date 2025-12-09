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
	restartTimeout time.Duration
)

func NewRestartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "restart",
		Short: "Restart the devnet",
		Long: `Restart all nodes in the devnet.

This command stops all running nodes and starts them again.
It preserves all configuration and data.

Examples:
  # Restart with default timeout
  devnet-builder restart

  # Restart with custom timeout
  devnet-builder restart --timeout 60s`,
		RunE: runRestart,
	}

	cmd.Flags().DurationVarP(&restartTimeout, "timeout", "t", 30*time.Second,
		"Graceful shutdown timeout")

	return cmd
}

func runRestart(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	logger := output.DefaultLogger

	// Check if devnet exists
	if !devnet.DevnetExists(homeDir) {
		if jsonMode {
			return outputRestartError(fmt.Errorf("no devnet found"))
		}
		return fmt.Errorf("no devnet found at %s", homeDir)
	}

	// Load devnet
	d, err := devnet.LoadDevnetWithNodes(homeDir, logger)
	if err != nil {
		if jsonMode {
			return outputRestartError(err)
		}
		return fmt.Errorf("failed to load devnet: %w", err)
	}

	// Stop nodes
	if !jsonMode {
		output.Info("Stopping devnet nodes...")
	}

	if err := d.Stop(ctx, restartTimeout); err != nil {
		logger.Warn("Stop encountered issues: %v", err)
	}

	// Wait a moment for cleanup
	time.Sleep(2 * time.Second)

	// Start nodes
	if !jsonMode {
		output.Info("Starting devnet nodes...")
	}

	if err := d.StartNodes(ctx, d.Metadata.GenesisPath); err != nil {
		if jsonMode {
			return outputRestartError(err)
		}
		return fmt.Errorf("failed to start nodes: %w", err)
	}

	// Update metadata
	d.Metadata.SetRunning()
	if err := d.Metadata.Save(); err != nil {
		logger.Warn("Failed to update metadata: %v", err)
	}

	if jsonMode {
		return outputRestartJSON(d)
	}

	output.Success("Devnet restarted successfully.")
	fmt.Println()
	output.Bold("Endpoints:")
	for _, n := range d.Nodes {
		fmt.Printf("  Node %d: %s (RPC) | %s (EVM)\n",
			n.Index, n.RPCURL(), n.EVMRPCURL())
	}

	return nil
}

func outputRestartJSON(d *devnet.Devnet) error {
	result := StartResult{
		Status:     "success",
		ChainID:    d.Metadata.ChainID,
		Network:    d.Metadata.NetworkSource,
		Mode:       string(d.Metadata.ExecutionMode),
		Validators: d.Metadata.NumValidators,
		Nodes:      make([]NodeResult, len(d.Nodes)),
	}

	for i, n := range d.Nodes {
		result.Nodes[i] = NodeResult{
			Index:  n.Index,
			RPC:    n.RPCURL(),
			EVMRPC: n.EVMRPCURL(),
			Status: string(n.Status),
		}
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(data))
	return nil
}

func outputRestartError(err error) error {
	result := map[string]interface{}{
		"error":   true,
		"code":    "RESTART_FAILED",
		"message": err.Error(),
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(data))
	return err
}
