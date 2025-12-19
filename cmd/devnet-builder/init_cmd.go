package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/stablelabs/stable-devnet/internal/devnet"
	"github.com/stablelabs/stable-devnet/internal/network"
	"github.com/stablelabs/stable-devnet/internal/output"
)

var (
	initNetwork           string
	initBlockchainNetwork string // Network module selection (stable, ault, etc.)
	initValidators        int
	initAccounts          int
	initMode              string
	initNoCache           bool
	initVersion           string
)

// InitJSONResult represents the JSON output for the init command.
type InitJSONResult struct {
	Status            string            `json:"status"`
	ProvisionState    string            `json:"provision_state"`
	ChainID           string            `json:"chain_id,omitempty"`
	Network           string            `json:"network"`
	BlockchainNetwork string            `json:"blockchain_network"`
	Validators        int               `json:"validators"`
	ConfigPaths       map[string]string `json:"config_paths,omitempty"`
	NextCommand       string            `json:"next_command"`
	Error             string            `json:"error,omitempty"`
}

func NewInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize devnet configuration without starting nodes",
		Long: `Initialize creates devnet configuration and generates validators without starting nodes.

This allows you to:
1. Modify config files (config.toml, app.toml) before starting
2. Adjust genesis parameters
3. Start nodes separately using 'devnet-builder up'

The init command performs:
- Download snapshot from network
- Export genesis state
- Generate validator keys
- Configure node directories

Examples:
  # Initialize with default settings
  devnet-builder init

  # Initialize with testnet data
  devnet-builder init --network testnet

  # Initialize with 2 validators
  devnet-builder init --validators 2

  # After initializing, modify config then run:
  devnet-builder up`,
		RunE: runInit,
	}

	cmd.Flags().StringVarP(&initNetwork, "network", "n", "mainnet",
		"Network source (mainnet, testnet)")
	cmd.Flags().IntVar(&initValidators, "validators", 4,
		"Number of validators (1-4)")
	cmd.Flags().IntVar(&initAccounts, "accounts", 0,
		"Additional funded accounts")
	cmd.Flags().StringVarP(&initMode, "mode", "m", "docker",
		"Execution mode (docker, local)")
	cmd.Flags().BoolVar(&initNoCache, "no-cache", false,
		"Skip snapshot cache")
	cmd.Flags().StringVar(&initVersion, "stable-version", "latest",
		"Stable repository version for genesis export")
	cmd.Flags().StringVar(&initBlockchainNetwork, "blockchain", "stable",
		"Blockchain network module (stable, ault)")

	return cmd
}

func runInit(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	logger := output.DefaultLogger

	// Apply config.toml values (priority: default < config.toml < env < flag)
	fileCfg := GetLoadedFileConfig()
	if fileCfg != nil {
		if !cmd.Flags().Changed("network") && fileCfg.Network != nil {
			initNetwork = *fileCfg.Network
		}
		if !cmd.Flags().Changed("blockchain") && fileCfg.BlockchainNetwork != nil {
			initBlockchainNetwork = *fileCfg.BlockchainNetwork
		}
		if !cmd.Flags().Changed("validators") && fileCfg.Validators != nil {
			initValidators = *fileCfg.Validators
		}
		if !cmd.Flags().Changed("mode") && fileCfg.Mode != nil {
			initMode = *fileCfg.Mode
		}
		if !cmd.Flags().Changed("stable-version") && fileCfg.StableVersion != nil {
			initVersion = *fileCfg.StableVersion
		}
		if !cmd.Flags().Changed("no-cache") && fileCfg.NoCache != nil {
			initNoCache = *fileCfg.NoCache
		}
		if !cmd.Flags().Changed("accounts") && fileCfg.Accounts != nil {
			initAccounts = *fileCfg.Accounts
		}
	}

	// Apply environment variables
	if network := os.Getenv("STABLE_DEVNET_NETWORK"); network != "" && !cmd.Flags().Changed("network") {
		initNetwork = network
	}
	if mode := os.Getenv("STABLE_DEVNET_MODE"); mode != "" && !cmd.Flags().Changed("mode") {
		initMode = mode
	}
	if version := os.Getenv("STABLE_VERSION"); version != "" && !cmd.Flags().Changed("stable-version") {
		initVersion = version
	}

	// Validate inputs
	if initNetwork != "mainnet" && initNetwork != "testnet" {
		return outputInitError(fmt.Errorf("invalid network: %s (must be 'mainnet' or 'testnet')", initNetwork))
	}
	if initValidators < 1 || initValidators > 4 {
		return outputInitError(fmt.Errorf("invalid validators: %d (must be 1-4)", initValidators))
	}
	if initMode != "docker" && initMode != "local" {
		return outputInitError(fmt.Errorf("invalid mode: %s (must be 'docker' or 'local')", initMode))
	}
	// Validate blockchain network module exists
	if !network.Has(initBlockchainNetwork) {
		available := network.List()
		return outputInitError(fmt.Errorf("unknown blockchain network: %s (available: %v)", initBlockchainNetwork, available))
	}

	// Check if devnet already exists
	if devnet.DevnetExists(homeDir) {
		return outputInitError(fmt.Errorf("devnet already exists at %s\nUse 'devnet-builder destroy' to remove it first", homeDir))
	}

	// Run provision
	opts := devnet.ProvisionOptions{
		HomeDir:           homeDir,
		Network:           initNetwork,
		BlockchainNetwork: initBlockchainNetwork,
		NumValidators:     initValidators,
		NumAccounts:       initAccounts,
		Mode:              devnet.ExecutionMode(initMode),
		StableVersion:     initVersion,
		NoCache:           initNoCache,
		Logger:            logger,
	}

	result, err := devnet.Provision(ctx, opts)
	if err != nil {
		return outputInitError(err)
	}

	// Output result
	if jsonMode {
		return outputInitJSON(result)
	}
	return outputInitText(result)
}

func outputInitText(result *devnet.ProvisionResult) error {
	fmt.Println()
	output.Success("Initialization complete!")
	fmt.Println()

	output.Bold("Chain ID:     %s", result.Metadata.ChainID)
	output.Info("Network:      %s", result.Metadata.NetworkSource)
	output.Info("Blockchain:   %s", result.Metadata.BlockchainNetwork)
	output.Info("Validators:   %d", result.Metadata.NumValidators)
	fmt.Println()

	// Print config file paths
	output.Bold("Configuration files:")
	devnetDir := filepath.Join(result.Metadata.HomeDir, "devnet")
	fmt.Printf("  config.toml:   %s/node0/config/config.toml\n", devnetDir)
	fmt.Printf("  app.toml:      %s/node0/config/app.toml\n", devnetDir)
	fmt.Printf("  genesis.json:  %s\n", result.GenesisPath)
	fmt.Println()

	// Print modifiable parameters guide
	output.Bold("Modifiable parameters:")
	fmt.Println("  config.toml:")
	fmt.Println("    - consensus.timeout_commit     Block time (default: 1s)")
	fmt.Println("    - p2p.persistent_peers         Peer connections")
	fmt.Println("    - p2p.max_num_inbound_peers    Max inbound peers")
	fmt.Println()
	fmt.Println("  app.toml:")
	fmt.Println("    - api.enable                   REST API (default: true for node0)")
	fmt.Println("    - grpc.enable                  gRPC (default: true)")
	fmt.Println("    - json-rpc.enable              EVM JSON-RPC (default: true for node0)")
	fmt.Println("    - minimum-gas-prices           Gas price floor")
	fmt.Println()

	output.Bold("Next step:")
	fmt.Println("  Run 'devnet-builder up' to start the nodes")
	fmt.Println()

	return nil
}

func outputInitJSON(result *devnet.ProvisionResult) error {
	devnetDir := filepath.Join(result.Metadata.HomeDir, "devnet")

	jsonResult := InitJSONResult{
		Status:            "success",
		ProvisionState:    string(result.Metadata.ProvisionState),
		ChainID:           result.Metadata.ChainID,
		Network:           result.Metadata.NetworkSource,
		BlockchainNetwork: result.Metadata.BlockchainNetwork,
		Validators:        result.Metadata.NumValidators,
		ConfigPaths: map[string]string{
			"config.toml":  filepath.Join(devnetDir, "node0", "config", "config.toml"),
			"app.toml":     filepath.Join(devnetDir, "node0", "config", "app.toml"),
			"genesis.json": result.GenesisPath,
		},
		NextCommand: "devnet-builder up",
	}

	data, err := json.MarshalIndent(jsonResult, "", "  ")
	if err != nil {
		return err
	}

	fmt.Println(string(data))
	return nil
}

func outputInitError(err error) error {
	if jsonMode {
		jsonResult := InitJSONResult{
			Status:         "error",
			ProvisionState: "failed",
			Error:          err.Error(),
		}

		data, _ := json.MarshalIndent(jsonResult, "", "  ")
		fmt.Println(string(data))
	}
	return err
}
