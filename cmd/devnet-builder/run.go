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
	runMode          string
	runBinaryRef     string
	runHealthTimeout time.Duration
	runStableVersion string
)

// RunJSONResult represents the JSON output for the run command.
type RunJSONResult struct {
	Status          string           `json:"status"`
	ChainID         string           `json:"chain_id,omitempty"`
	Mode            string           `json:"mode"`
	SuccessfulNodes []int            `json:"successful_nodes"`
	FailedNodes     []FailedNodeJSON `json:"failed_nodes,omitempty"`
	Nodes           []NodeResult     `json:"nodes,omitempty"`
	Error           string           `json:"error,omitempty"`
}

// FailedNodeJSON represents a failed node in JSON output.
type FailedNodeJSON struct {
	Index   int      `json:"index"`
	Error   string   `json:"error"`
	LogPath string   `json:"log_path"`
	LogTail []string `json:"log_tail,omitempty"`
}

func NewRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:        "run",
		Short:      "Start nodes from a provisioned devnet (deprecated: use 'up' instead)",
		Deprecated: "use 'up' instead",
		Long: `Start nodes from a previously provisioned devnet configuration.

DEPRECATED: This command is deprecated. Use 'devnet-builder up' instead.

This command requires that 'devnet-builder init' has been run first.
It allows you to modify config files between init and up.

Workflow:
  1. devnet-builder init    # Create config
  2. # Edit config files...
  3. devnet-builder up      # Start nodes

Examples:
  # Start nodes with default settings
  devnet-builder up

  # Start with local binary mode
  devnet-builder up --mode local

  # Start with specific binary from cache
  devnet-builder up --binary-ref v1.2.3`,
		RunE: func(cmd *cobra.Command, args []string) error {
			PrintDeprecationWarning("run", "up")
			return runRun(cmd, args)
		},
	}

	cmd.Flags().StringVarP(&runMode, "mode", "m", "",
		"Execution mode (docker, local). If not specified, uses provision mode")
	cmd.Flags().StringVar(&runBinaryRef, "binary-ref", "",
		"Binary reference from cache (for local mode)")
	cmd.Flags().DurationVar(&runHealthTimeout, "health-timeout", 5*time.Minute,
		"Timeout for node health check")
	cmd.Flags().StringVar(&runStableVersion, "stable-version", "",
		"Stable repository version. If not specified, uses provision version")

	return cmd
}

func runRun(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	logger := output.DefaultLogger

	// Apply config.toml values
	// NOTE: Mode is NOT applied from config.toml for the run command.
	// The run command uses the execution mode from metadata (set during provision/start),
	// and only the CLI --mode flag can override it.
	fileCfg := GetLoadedFileConfig()
	if fileCfg != nil {
		// Mode intentionally not applied from config.toml - use metadata instead
		if !cmd.Flags().Changed("stable-version") && fileCfg.StableVersion != nil {
			runStableVersion = *fileCfg.StableVersion
		}
	}

	// Apply environment variables
	// NOTE: STABLE_DEVNET_MODE is NOT applied for the run command.
	// Only the CLI --mode flag can override the metadata's execution mode.
	if version := os.Getenv("STABLE_VERSION"); version != "" && !cmd.Flags().Changed("stable-version") {
		runStableVersion = version
	}

	// Validate mode if specified
	if runMode != "" && runMode != "docker" && runMode != "local" {
		return outputRunError(fmt.Errorf("invalid mode: %s (must be 'docker' or 'local')", runMode))
	}

	// Check if devnet exists and is provisioned
	if !devnet.DevnetExists(homeDir) {
		return outputRunError(fmt.Errorf("devnet not found at %s\nRun 'devnet-builder provision' first", homeDir))
	}

	// Load metadata to check provision state
	metadata, err := devnet.LoadDevnetMetadata(homeDir)
	if err != nil {
		return outputRunError(fmt.Errorf("failed to load devnet metadata: %w", err))
	}

	// Check if already running
	if metadata.IsRunning() {
		return outputRunError(fmt.Errorf("devnet is already running\nUse 'devnet-builder stop' first"))
	}

	// Build custom binary if binary-ref is specified
	var customBinaryPath string
	var isCustomRef bool
	if runBinaryRef != "" {
		b := builder.NewBuilder(homeDir, logger)
		logger.Info("Building binary from source (ref: %s)...", runBinaryRef)
		buildResult, err := b.Build(ctx, builder.BuildOptions{
			Ref:     runBinaryRef,
			Network: metadata.NetworkSource,
		})
		if err != nil {
			return outputRunError(fmt.Errorf("failed to build from source: %w", err))
		}
		customBinaryPath = buildResult.BinaryPath
		isCustomRef = true
		logger.Success("Binary built: %s (commit: %s)", buildResult.BinaryPath, buildResult.CommitHash)
	}

	// Prepare run options
	opts := devnet.RunOptions{
		HomeDir:          homeDir,
		Mode:             devnet.ExecutionMode(runMode),
		StableVersion:    runStableVersion,
		BinaryRef:        runBinaryRef,
		HealthTimeout:    runHealthTimeout,
		Logger:           logger,
		IsCustomRef:      isCustomRef,
		CustomBinaryPath: customBinaryPath,
	}

	result, err := devnet.Run(ctx, opts)
	if err != nil {
		// Still output partial results if available
		if result != nil {
			if jsonMode {
				return outputRunJSONWithError(result, err)
			}
			outputRunTextPartial(result)
		}
		return err
	}

	// Output result
	if jsonMode {
		return outputRunJSON(result)
	}
	return outputRunText(result)
}

func outputRunText(result *devnet.RunResult) error {
	fmt.Println()
	output.Bold("Chain ID:     %s", result.Devnet.Metadata.ChainID)
	output.Info("Network:      %s", result.Devnet.Metadata.NetworkSource)
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

func outputRunTextPartial(result *devnet.RunResult) {
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

func outputRunJSON(result *devnet.RunResult) error {
	jsonResult := RunJSONResult{
		Status:          "success",
		ChainID:         result.Devnet.Metadata.ChainID,
		Mode:            string(result.Devnet.Metadata.ExecutionMode),
		SuccessfulNodes: result.SuccessfulNodes,
		Nodes:           make([]NodeResult, len(result.Devnet.Nodes)),
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

func outputRunJSONWithError(result *devnet.RunResult, err error) error {
	jsonResult := RunJSONResult{
		Status:          "error",
		SuccessfulNodes: result.SuccessfulNodes,
		Error:           err.Error(),
	}

	if result.Devnet != nil && result.Devnet.Metadata != nil {
		jsonResult.ChainID = result.Devnet.Metadata.ChainID
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

func outputRunError(err error) error {
	if jsonMode {
		jsonResult := RunJSONResult{
			Status: "error",
			Error:  err.Error(),
		}
		data, _ := json.MarshalIndent(jsonResult, "", "  ")
		fmt.Println(string(data))
	}
	return err
}
