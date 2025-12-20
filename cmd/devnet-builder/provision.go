package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/b-harvest/devnet-builder/internal/devnet"
	"github.com/b-harvest/devnet-builder/internal/output"
	"github.com/spf13/cobra"
)

var (
	provisionNetwork    string
	provisionValidators int
	provisionAccounts   int
	provisionMode       string
	provisionNoCache    bool
	provisionVersion    string
)

// ProvisionJSONResult represents the JSON output for the provision command.
type ProvisionJSONResult struct {
	Status         string            `json:"status"`
	ProvisionState string            `json:"provision_state"`
	ChainID        string            `json:"chain_id,omitempty"`
	Network        string            `json:"network"`
	Validators     int               `json:"validators"`
	ConfigPaths    map[string]string `json:"config_paths,omitempty"`
	NextCommand    string            `json:"next_command"`
	Error          string            `json:"error,omitempty"`
}

func NewProvisionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:        "provision",
		Short:      "Provision devnet configuration (deprecated: use 'init' instead)",
		Deprecated: "use 'init' instead",
		Long: `Provision creates devnet configuration and generates validators without starting nodes.

DEPRECATED: This command is deprecated. Use 'devnet-builder init' instead.

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
		RunE: func(cmd *cobra.Command, args []string) error {
			PrintDeprecationWarning("provision", "init")
			return runProvision(cmd, args)
		},
	}

	cmd.Flags().StringVarP(&provisionNetwork, "network", "n", "mainnet",
		"Network source (mainnet, testnet)")
	cmd.Flags().IntVar(&provisionValidators, "validators", 4,
		"Number of validators (1-4)")
	cmd.Flags().IntVar(&provisionAccounts, "accounts", 0,
		"Additional funded accounts")
	cmd.Flags().StringVarP(&provisionMode, "mode", "m", "docker",
		"Execution mode (docker, local)")
	cmd.Flags().BoolVar(&provisionNoCache, "no-cache", false,
		"Skip snapshot cache")
	cmd.Flags().StringVar(&provisionVersion, "stable-version", "latest",
		"Stable repository version for genesis export")

	return cmd
}

func runProvision(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	logger := output.DefaultLogger

	// Apply config.toml values (priority: default < config.toml < env < flag)
	fileCfg := GetLoadedFileConfig()
	if fileCfg != nil {
		if !cmd.Flags().Changed("network") && fileCfg.Network != nil {
			provisionNetwork = *fileCfg.Network
		}
		if !cmd.Flags().Changed("validators") && fileCfg.Validators != nil {
			provisionValidators = *fileCfg.Validators
		}
		if !cmd.Flags().Changed("mode") && fileCfg.Mode != nil {
			provisionMode = *fileCfg.Mode
		}
		if !cmd.Flags().Changed("stable-version") && fileCfg.StableVersion != nil {
			provisionVersion = *fileCfg.StableVersion
		}
		if !cmd.Flags().Changed("no-cache") && fileCfg.NoCache != nil {
			provisionNoCache = *fileCfg.NoCache
		}
		if !cmd.Flags().Changed("accounts") && fileCfg.Accounts != nil {
			provisionAccounts = *fileCfg.Accounts
		}
	}

	// Apply environment variables
	if network := os.Getenv("STABLE_DEVNET_NETWORK"); network != "" && !cmd.Flags().Changed("network") {
		provisionNetwork = network
	}
	if mode := os.Getenv("STABLE_DEVNET_MODE"); mode != "" && !cmd.Flags().Changed("mode") {
		provisionMode = mode
	}
	if version := os.Getenv("STABLE_VERSION"); version != "" && !cmd.Flags().Changed("stable-version") {
		provisionVersion = version
	}

	// Validate inputs
	if provisionNetwork != "mainnet" && provisionNetwork != "testnet" {
		return outputProvisionError(fmt.Errorf("invalid network: %s (must be 'mainnet' or 'testnet')", provisionNetwork))
	}
	if provisionValidators < 1 || provisionValidators > 4 {
		return outputProvisionError(fmt.Errorf("invalid validators: %d (must be 1-4)", provisionValidators))
	}
	if provisionMode != "docker" && provisionMode != "local" {
		return outputProvisionError(fmt.Errorf("invalid mode: %s (must be 'docker' or 'local')", provisionMode))
	}

	// Check if devnet already exists
	if devnet.DevnetExists(homeDir) {
		return outputProvisionError(fmt.Errorf("devnet already exists at %s\nUse 'devnet-builder clean' to remove it first", homeDir))
	}

	// Run provision
	opts := devnet.ProvisionOptions{
		HomeDir:       homeDir,
		Network:       provisionNetwork,
		NumValidators: provisionValidators,
		NumAccounts:   provisionAccounts,
		Mode:          devnet.ExecutionMode(provisionMode),
		StableVersion: provisionVersion,
		NoCache:       provisionNoCache,
		Logger:        logger,
	}

	result, err := devnet.Provision(ctx, opts)
	if err != nil {
		return outputProvisionError(err)
	}

	// Output result
	if jsonMode {
		return outputProvisionJSON(result)
	}
	return outputProvisionText(result)
}

func outputProvisionText(result *devnet.ProvisionResult) error {
	fmt.Println()
	output.Success("Provision complete!")
	fmt.Println()

	output.Bold("Chain ID:     %s", result.Metadata.ChainID)
	output.Info("Network:      %s", result.Metadata.NetworkSource)
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
	fmt.Println("  Run 'devnet-builder run' to start the nodes")
	fmt.Println()

	return nil
}

func outputProvisionJSON(result *devnet.ProvisionResult) error {
	devnetDir := filepath.Join(result.Metadata.HomeDir, "devnet")

	jsonResult := ProvisionJSONResult{
		Status:         "success",
		ProvisionState: string(result.Metadata.ProvisionState),
		ChainID:        result.Metadata.ChainID,
		Network:        result.Metadata.NetworkSource,
		Validators:     result.Metadata.NumValidators,
		ConfigPaths: map[string]string{
			"config.toml":  filepath.Join(devnetDir, "node0", "config", "config.toml"),
			"app.toml":     filepath.Join(devnetDir, "node0", "config", "app.toml"),
			"genesis.json": result.GenesisPath,
		},
		NextCommand: "devnet-builder run",
	}

	data, err := json.MarshalIndent(jsonResult, "", "  ")
	if err != nil {
		return err
	}

	fmt.Println(string(data))
	return nil
}

func outputProvisionError(err error) error {
	if jsonMode {
		jsonResult := ProvisionJSONResult{
			Status:         "error",
			ProvisionState: "failed",
			Error:          err.Error(),
		}

		data, _ := json.MarshalIndent(jsonResult, "", "  ")
		fmt.Println(string(data))
	}
	return err
}
