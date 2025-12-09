package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/stablelabs/stable-devnet/internal/devnet"
	"github.com/stablelabs/stable-devnet/internal/output"
)

var (
	startNetwork       string
	startValidators    int
	startMode          string
	startStableVersion string
	startNoCache       bool
	startAccounts      int
)

// StartResult represents the JSON output for the start command.
type StartResult struct {
	Status     string       `json:"status"`
	ChainID    string       `json:"chain_id"`
	Network    string       `json:"network"`
	Mode       string       `json:"mode"`
	Validators int          `json:"validators"`
	Nodes      []NodeResult `json:"nodes"`
}

// NodeResult represents a node in the JSON output.
type NodeResult struct {
	Index  int    `json:"index"`
	RPC    string `json:"rpc"`
	EVMRPC string `json:"evm_rpc"`
	Status string `json:"status"`
}

func NewStartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start a local devnet",
		Long: `Start a local devnet with configurable validators and network source.

This command will:
1. Check prerequisites (Docker/local binary, curl, jq, zstd/lz4)
2. Download or use cached snapshot from the specified network
3. Export genesis state from snapshot
4. Generate devnet configuration for all validators
5. Start all validator nodes

Examples:
  # Start with default settings (4 validators, mainnet, docker mode)
  devnet-builder start

  # Start with testnet data
  devnet-builder start --network testnet

  # Start with 2 validators
  devnet-builder start --validators 2

  # Start with local binary mode
  devnet-builder start --mode local

  # Start with specific stable version
  devnet-builder start --stable-version v1.2.3`,
		RunE: runStart,
	}

	// Command flags
	cmd.Flags().StringVarP(&startNetwork, "network", "n", "mainnet",
		"Network source (mainnet, testnet)")
	cmd.Flags().IntVar(&startValidators, "validators", 4,
		"Number of validators (1-4)")
	cmd.Flags().StringVarP(&startMode, "mode", "m", "docker",
		"Execution mode (docker, local)")
	cmd.Flags().StringVar(&startStableVersion, "stable-version", "latest",
		"Stable repository version")
	cmd.Flags().BoolVar(&startNoCache, "no-cache", false,
		"Skip snapshot cache")
	cmd.Flags().IntVar(&startAccounts, "accounts", 0,
		"Additional funded accounts")

	return cmd
}

func runStart(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	logger := output.DefaultLogger

	// Apply environment variable defaults
	if network := os.Getenv("STABLE_DEVNET_NETWORK"); network != "" && !cmd.Flags().Changed("network") {
		startNetwork = network
	}
	if mode := os.Getenv("STABLE_DEVNET_MODE"); mode != "" && !cmd.Flags().Changed("mode") {
		startMode = mode
	}
	if version := os.Getenv("STABLE_VERSION"); version != "" && !cmd.Flags().Changed("stable-version") {
		startStableVersion = version
	}

	// Validate inputs
	if startNetwork != "mainnet" && startNetwork != "testnet" {
		return fmt.Errorf("invalid network: %s (must be 'mainnet' or 'testnet')", startNetwork)
	}
	if startValidators < 1 || startValidators > 4 {
		return fmt.Errorf("invalid validators: %d (must be 1-4)", startValidators)
	}
	if startMode != "docker" && startMode != "local" {
		return fmt.Errorf("invalid mode: %s (must be 'docker' or 'local')", startMode)
	}

	// Check if devnet already exists
	if devnet.DevnetExists(homeDir) {
		return fmt.Errorf("devnet already exists at %s\nUse 'devnet-builder clean' to remove it first", homeDir)
	}

	// Start devnet
	opts := devnet.StartOptions{
		HomeDir:       homeDir,
		Network:       startNetwork,
		NumValidators: startValidators,
		NumAccounts:   startAccounts,
		Mode:          devnet.ExecutionMode(startMode),
		StableVersion: startStableVersion,
		NoCache:       startNoCache,
		Logger:        logger,
	}

	d, err := devnet.Start(ctx, opts)
	if err != nil {
		if jsonMode {
			return outputStartError(err)
		}
		return err
	}

	// Output result
	if jsonMode {
		return outputStartJSON(d)
	}
	return outputStartText(d)
}

func outputStartText(d *devnet.Devnet) error {
	fmt.Println()
	output.Bold("Chain ID:     %s", d.Metadata.ChainID)
	output.Info("Network:      %s", d.Metadata.NetworkSource)
	output.Info("Mode:         %s", d.Metadata.ExecutionMode)
	output.Info("Validators:   %d", d.Metadata.NumValidators)
	fmt.Println()
	output.Bold("Endpoints:")

	for _, n := range d.Nodes {
		fmt.Printf("  Node %d: %s (RPC) | %s (EVM)\n",
			n.Index, n.RPCURL(), n.EVMRPCURL())
	}

	return nil
}

func outputStartJSON(d *devnet.Devnet) error {
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

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}

	fmt.Println(string(data))
	return nil
}

func outputStartError(err error) error {
	result := map[string]interface{}{
		"error":   true,
		"code":    getErrorCode(err),
		"message": err.Error(),
	}

	data, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(data))
	return err
}

func getErrorCode(err error) string {
	errStr := err.Error()
	switch {
	case contains(errStr, "prerequisite"):
		return "PREREQUISITE_MISSING"
	case contains(errStr, "already exists"):
		return "DEVNET_ALREADY_RUNNING"
	case contains(errStr, "snapshot"):
		return "SNAPSHOT_DOWNLOAD_FAILED"
	case contains(errStr, "start"):
		return "NODE_START_FAILED"
	case contains(errStr, "port"):
		return "PORT_CONFLICT"
	default:
		return "GENERAL_ERROR"
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
