package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/b-harvest/devnet-builder/internal/application/dto"
	"github.com/b-harvest/devnet-builder/internal/output"
	"github.com/spf13/cobra"
)

var (
	upMode          string
	upBinaryRef     string
	upHealthTimeout time.Duration
	upStableVersion string
)

// UpJSONResult represents the JSON output for the up command.
type UpJSONResult struct {
	Status            string           `json:"status"`
	ChainID           string           `json:"chain_id,omitempty"`
	BlockchainNetwork string           `json:"blockchain_network,omitempty"`
	Mode              string           `json:"mode"`
	SuccessfulNodes   []int            `json:"successful_nodes"`
	FailedNodes       []FailedNodeJSON `json:"failed_nodes,omitempty"`
	Nodes             []NodeResult     `json:"nodes,omitempty"`
	Error             string           `json:"error,omitempty"`
}

func NewStartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start nodes from existing configuration",
		Long: `Start nodes from a previously initialized devnet configuration.

This command requires that 'devnet-builder init' or 'devnet-builder deploy'
has been run first. It allows you to restart nodes after stopping them.

Workflow:
  1. devnet-builder init    # Create config (or deploy)
  2. devnet-builder stop    # Stop nodes
  3. devnet-builder start   # Start nodes again

Examples:
  # Start nodes with default settings
  devnet-builder start

  # Start with local binary mode
  devnet-builder start --mode local

  # Start with specific binary from cache
  devnet-builder start --binary-ref v1.2.3

  # Start with custom health timeout
  devnet-builder start --health-timeout 10m`,
		RunE: runStart,
	}

	cmd.Flags().StringVarP(&upMode, "mode", "m", "",
		"Execution mode (docker, local). If not specified, uses init mode")
	cmd.Flags().StringVar(&upBinaryRef, "binary-ref", "",
		"Binary reference from cache (for local mode)")
	cmd.Flags().DurationVar(&upHealthTimeout, "health-timeout", 5*time.Minute,
		"Timeout for node health check")
	cmd.Flags().StringVar(&upStableVersion, "stable-version", "",
		"Stable repository version. If not specified, uses init version")

	return cmd
}

func runStart(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Apply config.toml values
	fileCfg := GetLoadedFileConfig()
	if fileCfg != nil {
		if !cmd.Flags().Changed("stable-version") && fileCfg.StableVersion != nil {
			upStableVersion = *fileCfg.StableVersion
		}
	}

	// Apply environment variables
	if version := os.Getenv("STABLE_VERSION"); version != "" && !cmd.Flags().Changed("stable-version") {
		upStableVersion = version
	}

	// Validate mode if specified
	if upMode != "" && upMode != "docker" && upMode != "local" {
		return outputUpErrorClean(fmt.Errorf("invalid mode: %s (must be 'docker' or 'local')", upMode))
	}

	// Initialize clean service
	svc, err := getCleanService()
	if err != nil {
		return outputUpErrorClean(fmt.Errorf("failed to initialize service: %w", err))
	}

	// Check if devnet exists
	if !svc.DevnetExists() {
		return outputUpErrorClean(fmt.Errorf("no devnet found at %s\nRun 'devnet-builder init' or 'devnet-builder deploy' first", homeDir))
	}

	// Load devnet info to check status
	devnetInfo, err := svc.LoadDevnetInfo(ctx)
	if err != nil {
		return outputUpErrorClean(fmt.Errorf("failed to load devnet: %w", err))
	}

	// Check if already running
	if devnetInfo.Status == "running" {
		return outputUpErrorClean(fmt.Errorf("devnet is already running\nUse 'devnet-builder stop' first"))
	}

	// Start nodes using CleanDevnetService
	if !jsonMode {
		output.Info("Starting devnet nodes...")
	}

	result, err := svc.Start(ctx, upHealthTimeout)
	if err != nil {
		return outputUpErrorClean(err)
	}

	// Reload devnet info for output
	devnetInfo, _ = svc.LoadDevnetInfo(ctx)

	// Output result
	if jsonMode {
		return outputUpJSONClean(result, devnetInfo)
	}
	return outputUpTextClean(result, devnetInfo)
}

func outputUpTextClean(result *dto.RunOutput, devnetInfo *dto.DevnetInfo) error {
	fmt.Println()
	output.Bold("Chain ID:     %s", devnetInfo.ChainID)
	output.Info("Network:      %s", devnetInfo.NetworkSource)
	output.Info("Blockchain:   %s", devnetInfo.BlockchainNetwork)
	output.Info("Mode:         %s", devnetInfo.ExecutionMode)
	output.Info("Validators:   %d", devnetInfo.NumValidators)
	fmt.Println()

	output.Bold("Endpoints:")
	for i := range devnetInfo.Nodes {
		n := &devnetInfo.Nodes[i]
		status := "running"
		for _, ns := range result.Nodes {
			if ns.Index == n.Index && !ns.IsRunning {
				status = "failed"
				break
			}
		}
		if status == "running" {
			fmt.Printf("  Node %d: %s (RPC) | %s (EVM)\n",
				n.Index, n.RPCURL, n.EVMURL)
		} else {
			fmt.Printf("  Node %d: [FAILED]\n", n.Index)
		}
	}
	fmt.Println()

	if !result.AllRunning {
		output.Warn("Some nodes failed to start")
		fmt.Println()
	}

	return nil
}

func outputUpJSONClean(result *dto.RunOutput, devnetInfo *dto.DevnetInfo) error {
	successfulNodes := make([]int, 0)
	for _, ns := range result.Nodes {
		if ns.IsRunning {
			successfulNodes = append(successfulNodes, ns.Index)
		}
	}

	jsonResult := UpJSONResult{
		Status:            "success",
		ChainID:           devnetInfo.ChainID,
		BlockchainNetwork: devnetInfo.BlockchainNetwork,
		Mode:              devnetInfo.ExecutionMode,
		SuccessfulNodes:   successfulNodes,
		Nodes:             make([]NodeResult, len(devnetInfo.Nodes)),
	}

	if !result.AllRunning {
		jsonResult.Status = "partial"
	}

	for i := range devnetInfo.Nodes {
		n := &devnetInfo.Nodes[i]
		status := "running"
		for _, ns := range result.Nodes {
			if ns.Index == n.Index && !ns.IsRunning {
				status = "failed"
				break
			}
		}
		jsonResult.Nodes[i] = NodeResult{
			Index:  n.Index,
			RPC:    n.RPCURL,
			EVMRPC: n.EVMURL,
			Status: status,
		}
	}

	data, err := json.MarshalIndent(jsonResult, "", "  ")
	if err != nil {
		return err
	}

	fmt.Println(string(data))
	return nil
}

func outputUpErrorClean(err error) error {
	if jsonMode {
		jsonResult := UpJSONResult{
			Status: "error",
			Error:  err.Error(),
		}
		data, _ := json.MarshalIndent(jsonResult, "", "  ")
		fmt.Println(string(data))
	}
	return err
}
