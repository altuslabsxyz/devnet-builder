package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/b-harvest/devnet-builder/internal/application/dto"
	"github.com/b-harvest/devnet-builder/internal/output"
	"github.com/spf13/cobra"
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
	svc, err := getCleanService()
	if err != nil {
		return outputRestartError(fmt.Errorf("failed to initialize service: %w", err))
	}

	// Check if devnet exists
	if !svc.DevnetExists() {
		if jsonMode {
			return outputRestartError(fmt.Errorf("no devnet found"))
		}
		return fmt.Errorf("no devnet found at %s", homeDir)
	}

	// Restart
	if !jsonMode {
		output.Info("Restarting devnet...")
	}

	result, err := svc.Restart(ctx, restartTimeout)
	if err != nil {
		if jsonMode {
			return outputRestartError(err)
		}
		return err
	}

	if jsonMode {
		return outputRestartJSONResult(result)
	}

	if result.AllRunning {
		output.Success("Devnet restarted successfully.")
	} else {
		output.Warn("Devnet restarted with some issues (stopped: %d, started: %d)",
			result.StoppedNodes, result.StartedNodes)
	}

	// Show endpoints
	info, err := svc.LoadDevnetInfo(ctx)
	if err == nil && info != nil {
		fmt.Println()
		output.Bold("Endpoints:")
		for _, n := range info.Nodes {
			fmt.Printf("  Node %d: %s (RPC) | %s (EVM)\n",
				n.Index, n.RPCURL, n.EVMURL)
		}
	}

	return nil
}

func outputRestartJSONResult(result *dto.RestartOutput) error {
	status := "success"
	if !result.AllRunning {
		status = "partial"
	}

	jsonResult := map[string]interface{}{
		"status":        status,
		"stopped_nodes": result.StoppedNodes,
		"started_nodes": result.StartedNodes,
		"all_running":   result.AllRunning,
	}

	data, _ := json.MarshalIndent(jsonResult, "", "  ")
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
