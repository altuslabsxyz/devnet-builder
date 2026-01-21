package manage

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/application"
	"github.com/altuslabsxyz/devnet-builder/internal/application/dto"
	"github.com/altuslabsxyz/devnet-builder/internal/output"
	"github.com/altuslabsxyz/devnet-builder/types"
	"github.com/altuslabsxyz/devnet-builder/types/ctxconfig"
	"github.com/spf13/cobra"
)

var (
	upMode          string
	upBinaryRef     string
	upHealthTimeout time.Duration
	upStableVersion string
)

// UpJSONResult represents the JSON output for the start command.
type UpJSONResult struct {
	Status            string                 `json:"status"`
	ChainID           string                 `json:"chain_id,omitempty"`
	BlockchainNetwork string                 `json:"blockchain_network,omitempty"`
	Mode              string                 `json:"mode"`
	SuccessfulNodes   []int                  `json:"successful_nodes"`
	FailedNodes       []types.FailedNodeJSON `json:"failed_nodes,omitempty"`
	Nodes             []types.NodeResult     `json:"nodes,omitempty"`
	Error             string                 `json:"error,omitempty"`
}

// NewStartCmd creates the start command.
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
	cmd.Flags().StringVar(&upStableVersion, "network-version", "",
		"Network repository version. If not specified, uses init version")

	return cmd
}

func runStart(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	cfg := ctxconfig.FromContext(ctx)
	homeDir := cfg.HomeDir()
	jsonMode := cfg.JSONMode()
	fileCfg := cfg.FileConfig()

	// Apply config.toml values
	if fileCfg != nil {
		if !cmd.Flags().Changed("network-version") && fileCfg.NetworkVersion != nil {
			upStableVersion = *fileCfg.NetworkVersion
		}
	}

	// Apply environment variables
	if version := os.Getenv("NETWORK_VERSION"); version != "" && !cmd.Flags().Changed("network-version") {
		upStableVersion = version
	}

	// Validate mode if specified
	if upMode != "" && !types.ExecutionMode(upMode).IsValid() {
		return outputStartErrorWithMode(fmt.Errorf("invalid mode: %s (must be 'docker' or 'local')", upMode), jsonMode)
	}

	// Initialize service
	svc, err := application.GetService(homeDir)
	if err != nil {
		return outputStartErrorWithMode(fmt.Errorf("failed to initialize service: %w", err), jsonMode)
	}

	// Check if devnet exists
	if !svc.DevnetExists() {
		return outputStartErrorWithMode(fmt.Errorf("no devnet found at %s\nRun 'devnet-builder init' or 'devnet-builder deploy' first", homeDir), jsonMode)
	}

	// Load devnet info to check status
	devnetInfo, err := svc.LoadDevnetInfo(ctx)
	if err != nil {
		return outputStartErrorWithMode(fmt.Errorf("failed to load devnet: %w", err), jsonMode)
	}

	// Check if already running
	if devnetInfo.Status == "running" {
		return outputStartErrorWithMode(fmt.Errorf("devnet is already running\nUse 'devnet-builder stop' first"), jsonMode)
	}

	// Start nodes using DevnetService
	if !jsonMode {
		output.Info("Starting devnet nodes...")
	}

	result, err := svc.Start(ctx, upHealthTimeout)
	if err != nil {
		return outputStartErrorWithMode(err, jsonMode)
	}

	// Reload devnet info for output
	devnetInfo, _ = svc.LoadDevnetInfo(ctx)

	// Output result
	if jsonMode {
		return outputStartJSON(result, devnetInfo)
	}
	return outputStartText(result, devnetInfo)
}

func outputStartText(result *dto.RunOutput, devnetInfo *dto.DevnetInfo) error {
	fmt.Println()
	output.Bold("Chain ID:     %s", devnetInfo.ChainID)
	output.Info("Network:      %s", devnetInfo.NetworkSource)
	output.Info("Blockchain:   %s", devnetInfo.BlockchainNetwork)
	output.Info("ExecutionMode:         %s", devnetInfo.ExecutionMode)
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

func outputStartJSON(result *dto.RunOutput, devnetInfo *dto.DevnetInfo) error {
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
		Mode:              string(devnetInfo.ExecutionMode),
		SuccessfulNodes:   successfulNodes,
		Nodes:             make([]types.NodeResult, len(devnetInfo.Nodes)),
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
		jsonResult.Nodes[i] = types.NodeResult{
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

func outputStartErrorWithMode(err error, jsonMode bool) error {
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
