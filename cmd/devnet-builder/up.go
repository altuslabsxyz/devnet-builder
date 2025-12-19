package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/stablelabs/stable-devnet/internal/builder"
	"github.com/stablelabs/stable-devnet/internal/devnet"
	"github.com/stablelabs/stable-devnet/internal/output"
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

func NewUpCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "up",
		Short: "Start nodes from existing configuration",
		Long: `Start nodes from a previously initialized devnet configuration.

This command requires that 'devnet-builder init' or 'devnet-builder deploy'
has been run first. It allows you to restart nodes after stopping them.

Workflow:
  1. devnet-builder init    # Create config (or deploy)
  2. devnet-builder down    # Stop nodes
  3. devnet-builder up      # Start nodes again

Examples:
  # Start nodes with default settings
  devnet-builder up

  # Start with local binary mode
  devnet-builder up --mode local

  # Start with specific binary from cache
  devnet-builder up --binary-ref v1.2.3

  # Start with custom health timeout
  devnet-builder up --health-timeout 10m`,
		RunE: runUp,
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

func runUp(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	logger := output.DefaultLogger

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
		return outputUpError(fmt.Errorf("invalid mode: %s (must be 'docker' or 'local')", upMode))
	}

	// Load metadata using consolidated helper
	metadata, err := loadMetadataOrFail(logger)
	if err != nil {
		return outputUpError(err)
	}

	// Check if already running
	if metadata.IsRunning() {
		return outputUpError(fmt.Errorf("devnet is already running\nUse 'devnet-builder down' first"))
	}

	// Build custom binary if binary-ref is specified
	var customBinaryPath string
	var isCustomRef bool
	if upBinaryRef != "" {
		networkModule, _ := metadata.GetNetworkModule()
		b := builder.NewBuilder(homeDir, logger, networkModule)
		logger.Info("Building binary from source (ref: %s)...", upBinaryRef)
		buildResult, err := b.Build(ctx, builder.BuildOptions{
			Ref:     upBinaryRef,
			Network: metadata.NetworkSource,
		})
		if err != nil {
			return outputUpError(fmt.Errorf("failed to build from source: %w", err))
		}
		customBinaryPath = buildResult.BinaryPath
		isCustomRef = true
		logger.Success("Binary built: %s (commit: %s)", buildResult.BinaryPath, buildResult.CommitHash)
	}

	// Prepare run options
	opts := devnet.RunOptions{
		HomeDir:          homeDir,
		Mode:             devnet.ExecutionMode(upMode),
		StableVersion:    upStableVersion,
		BinaryRef:        upBinaryRef,
		HealthTimeout:    upHealthTimeout,
		Logger:           logger,
		IsCustomRef:      isCustomRef,
		CustomBinaryPath: customBinaryPath,
	}

	result, err := devnet.Run(ctx, opts)
	if err != nil {
		if result != nil {
			if jsonMode {
				return outputUpJSONWithError(result, err)
			}
			outputUpTextPartial(result)
		}
		return err
	}

	// Output result
	if jsonMode {
		return outputUpJSON(result)
	}
	return outputUpText(result)
}

func outputUpText(result *devnet.RunResult) error {
	fmt.Println()
	output.Bold("Chain ID:     %s", result.Devnet.Metadata.ChainID)
	output.Info("Network:      %s", result.Devnet.Metadata.NetworkSource)
	output.Info("Blockchain:   %s", result.Devnet.Metadata.BlockchainNetwork)
	output.Info("Mode:         %s", result.Devnet.Metadata.ExecutionMode)
	output.Info("Validators:   %d", result.Devnet.Metadata.NumValidators)
	fmt.Println()

	output.Bold("Endpoints:")
	for _, n := range result.Devnet.Nodes {
		status := "running"
		for _, fn := range result.FailedNodes {
			if fn.Index == n.Index {
				status = "failed"
				break
			}
		}
		if status == "running" {
			fmt.Printf("  Node %d: %s (RPC) | %s (EVM)\n",
				n.Index, n.RPCURL(), n.EVMRPCURL())
		} else {
			fmt.Printf("  Node %d: [FAILED]\n", n.Index)
		}
	}
	fmt.Println()

	if len(result.FailedNodes) > 0 {
		output.Warn("Some nodes failed to start:")
		for _, fn := range result.FailedNodes {
			fmt.Printf("  Node %d: %s\n", fn.Index, fn.Error)
			if fn.LogPath != "" {
				fmt.Printf("    Log: %s\n", fn.LogPath)
			}
		}
		fmt.Println()
	}

	return nil
}

func outputUpTextPartial(result *devnet.RunResult) {
	if len(result.SuccessfulNodes) > 0 {
		output.Info("Successfully started nodes: %v", result.SuccessfulNodes)
	}
	if len(result.FailedNodes) > 0 {
		output.Warn("Failed nodes:")
		for _, fn := range result.FailedNodes {
			fmt.Printf("  Node %d: %s\n", fn.Index, fn.Error)
		}
	}
}

func outputUpJSON(result *devnet.RunResult) error {
	jsonResult := UpJSONResult{
		Status:            "success",
		ChainID:           result.Devnet.Metadata.ChainID,
		BlockchainNetwork: result.Devnet.Metadata.BlockchainNetwork,
		Mode:              string(result.Devnet.Metadata.ExecutionMode),
		SuccessfulNodes:   result.SuccessfulNodes,
		Nodes:             make([]NodeResult, len(result.Devnet.Nodes)),
	}

	if !result.AllHealthy {
		jsonResult.Status = "partial"
	}

	for i, n := range result.Devnet.Nodes {
		status := "running"
		for _, fn := range result.FailedNodes {
			if fn.Index == n.Index {
				status = "failed"
				break
			}
		}
		jsonResult.Nodes[i] = NodeResult{
			Index:  n.Index,
			RPC:    n.RPCURL(),
			EVMRPC: n.EVMRPCURL(),
			Status: status,
		}
	}

	if len(result.FailedNodes) > 0 {
		jsonResult.FailedNodes = make([]FailedNodeJSON, len(result.FailedNodes))
		for i, fn := range result.FailedNodes {
			jsonResult.FailedNodes[i] = FailedNodeJSON{
				Index:   fn.Index,
				Error:   fn.Error,
				LogPath: fn.LogPath,
				LogTail: fn.LogTail,
			}
		}
	}

	data, err := json.MarshalIndent(jsonResult, "", "  ")
	if err != nil {
		return err
	}

	fmt.Println(string(data))
	return nil
}

func outputUpJSONWithError(result *devnet.RunResult, err error) error {
	jsonResult := UpJSONResult{
		Status:          "error",
		SuccessfulNodes: result.SuccessfulNodes,
		Error:           err.Error(),
	}

	if result.Devnet != nil && result.Devnet.Metadata != nil {
		jsonResult.ChainID = result.Devnet.Metadata.ChainID
		jsonResult.BlockchainNetwork = result.Devnet.Metadata.BlockchainNetwork
		jsonResult.Mode = string(result.Devnet.Metadata.ExecutionMode)
	}

	if len(result.FailedNodes) > 0 {
		jsonResult.FailedNodes = make([]FailedNodeJSON, len(result.FailedNodes))
		for i, fn := range result.FailedNodes {
			jsonResult.FailedNodes[i] = FailedNodeJSON{
				Index:   fn.Index,
				Error:   fn.Error,
				LogPath: fn.LogPath,
				LogTail: fn.LogTail,
			}
		}
	}

	data, _ := json.MarshalIndent(jsonResult, "", "  ")
	fmt.Println(string(data))
	return err
}

func outputUpError(err error) error {
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
