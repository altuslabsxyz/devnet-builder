package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/b-harvest/devnet-builder/internal/application/dto"
	"github.com/b-harvest/devnet-builder/internal/config"
	"github.com/b-harvest/devnet-builder/internal/network"
	"github.com/b-harvest/devnet-builder/internal/output"
	"github.com/spf13/cobra"
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

	// Build effective config from: default < config.toml < env < flag
	// Start with loaded config.toml values
	fileCfg := GetLoadedFileConfig()
	if fileCfg == nil {
		fileCfg = &config.FileConfig{}
	}

	// Apply flag values (flags override config.toml)
	if cmd.Flags().Changed("network") {
		fileCfg.Network = &initNetwork
	}
	if cmd.Flags().Changed("blockchain") {
		fileCfg.BlockchainNetwork = &initBlockchainNetwork
	}
	if cmd.Flags().Changed("validators") {
		fileCfg.Validators = &initValidators
	}
	if cmd.Flags().Changed("mode") {
		fileCfg.Mode = &initMode
	}
	if cmd.Flags().Changed("stable-version") {
		fileCfg.StableVersion = &initVersion
	}
	if cmd.Flags().Changed("no-cache") {
		fileCfg.NoCache = &initNoCache
	}
	if cmd.Flags().Changed("accounts") {
		fileCfg.Accounts = &initAccounts
	}

	// Apply environment variables (env overrides config.toml but not flags)
	if networkEnv := os.Getenv("STABLE_DEVNET_NETWORK"); networkEnv != "" && !cmd.Flags().Changed("network") {
		fileCfg.Network = &networkEnv
	}
	if modeEnv := os.Getenv("STABLE_DEVNET_MODE"); modeEnv != "" && !cmd.Flags().Changed("mode") {
		fileCfg.Mode = &modeEnv
	}
	if versionEnv := os.Getenv("STABLE_VERSION"); versionEnv != "" && !cmd.Flags().Changed("stable-version") {
		fileCfg.StableVersion = &versionEnv
	}

	// Run partial interactive setup for missing values
	setup := config.NewInteractiveSetup(homeDir)
	effectiveCfg, err := setup.RunPartial(fileCfg)
	if err != nil {
		// Check if it's a missing fields error for better messaging
		if mfErr, ok := err.(*config.MissingFieldsError); ok {
			return outputInitErrorClean(fmt.Errorf("missing required configuration: %v\nRun 'devnet-builder config init' to create a configuration file", mfErr.Fields))
		}
		return outputInitErrorClean(err)
	}

	// Extract values from effective config
	initNetwork = *effectiveCfg.Network
	initBlockchainNetwork = *effectiveCfg.BlockchainNetwork
	initValidators = *effectiveCfg.Validators
	initMode = *effectiveCfg.Mode
	if effectiveCfg.StableVersion != nil {
		initVersion = *effectiveCfg.StableVersion
	}
	if effectiveCfg.NoCache != nil {
		initNoCache = *effectiveCfg.NoCache
	}
	if effectiveCfg.Accounts != nil {
		initAccounts = *effectiveCfg.Accounts
	}

	// Validate inputs
	if initNetwork != "mainnet" && initNetwork != "testnet" {
		return outputInitErrorClean(fmt.Errorf("invalid network: %s (must be 'mainnet' or 'testnet')", initNetwork))
	}
	if initValidators < 1 || initValidators > 4 {
		return outputInitErrorClean(fmt.Errorf("invalid validators: %d (must be 1-4)", initValidators))
	}
	if initMode != "docker" && initMode != "local" {
		return outputInitErrorClean(fmt.Errorf("invalid mode: %s (must be 'docker' or 'local')", initMode))
	}
	// Validate blockchain network module exists
	if !network.Has(initBlockchainNetwork) {
		available := network.List()
		return outputInitErrorClean(fmt.Errorf("unknown blockchain network: %s (available: %v)", initBlockchainNetwork, available))
	}

	// Check if devnet already exists using CleanDevnetService
	svc, err := getCleanService()
	if err != nil {
		return outputInitErrorClean(fmt.Errorf("failed to initialize service: %w", err))
	}
	if svc.DevnetExists() {
		return outputInitErrorClean(fmt.Errorf("devnet already exists at %s\nUse 'devnet-builder destroy' to remove it first", homeDir))
	}

	// Run provision using CleanDevnetService
	provisionInput := dto.ProvisionInput{
		HomeDir:           homeDir,
		Network:           initNetwork,
		BlockchainNetwork: initBlockchainNetwork,
		NumValidators:     initValidators,
		NumAccounts:       initAccounts,
		Mode:              initMode,
		StableVersion:     initVersion,
		NoCache:           initNoCache,
	}

	result, err := svc.Provision(ctx, provisionInput)
	if err != nil {
		return outputInitErrorClean(err)
	}

	// Load devnet info for output
	devnetInfo, _ := svc.LoadDevnetInfo(ctx)

	// Output result
	if jsonMode {
		return outputInitJSONClean(result, devnetInfo)
	}
	return outputInitTextClean(result, devnetInfo)
}

func outputInitTextClean(result *dto.ProvisionOutput, devnetInfo *dto.DevnetInfo) error {
	fmt.Println()
	output.Success("Initialization complete!")
	fmt.Println()

	output.Bold("Chain ID:     %s", devnetInfo.ChainID)
	output.Info("Network:      %s", devnetInfo.NetworkSource)
	output.Info("Blockchain:   %s", devnetInfo.BlockchainNetwork)
	output.Info("Validators:   %d", devnetInfo.NumValidators)
	fmt.Println()

	// Print config file paths
	output.Bold("Configuration files:")
	devnetDir := filepath.Join(homeDir, "devnet")
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

func outputInitJSONClean(result *dto.ProvisionOutput, devnetInfo *dto.DevnetInfo) error {
	devnetDir := filepath.Join(homeDir, "devnet")

	jsonResult := InitJSONResult{
		Status:            "success",
		ProvisionState:    "provisioned",
		ChainID:           devnetInfo.ChainID,
		Network:           devnetInfo.NetworkSource,
		BlockchainNetwork: devnetInfo.BlockchainNetwork,
		Validators:        devnetInfo.NumValidators,
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

func outputInitErrorClean(err error) error {
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
