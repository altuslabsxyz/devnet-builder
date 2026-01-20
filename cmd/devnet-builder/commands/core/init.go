package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/altuslabsxyz/devnet-builder/cmd/devnet-builder/shared"
	"github.com/altuslabsxyz/devnet-builder/internal/application"
	"github.com/altuslabsxyz/devnet-builder/internal/application/dto"
	"github.com/altuslabsxyz/devnet-builder/internal/config"
	"github.com/altuslabsxyz/devnet-builder/internal/infrastructure/network"
	"github.com/altuslabsxyz/devnet-builder/internal/output"
	"github.com/altuslabsxyz/devnet-builder/internal/paths"
	"github.com/altuslabsxyz/devnet-builder/types"
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

// NewInitCmd creates the init command.
func NewInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize devnet configuration without starting nodes",
		Long: `Initialize creates devnet configuration and generates validators without starting nodes.

This allows you to:
1. Modify config files (config.toml, app.toml) before starting
2. Adjust genesis parameters
3. Start nodes separately using 'devnet-builder start'

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
  devnet-builder start`,
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
	cmd.Flags().StringVar(&initVersion, "network-version", "latest",
		"Network repository version for genesis export")
	cmd.Flags().StringVar(&initBlockchainNetwork, "blockchain", "stable",
		"Blockchain network module (stable, ault)")

	return cmd
}

func runInit(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	homeDir := shared.GetHomeDir()
	jsonMode := shared.GetJSONMode()

	// Build effective config from: default < config.toml < env < flag
	// Start with loaded config.toml values
	fileCfg := shared.GetLoadedFileConfig()
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
		em := types.ExecutionMode(initMode)
		fileCfg.ExecutionMode = &em
	}
	if cmd.Flags().Changed("no-cache") {
		fileCfg.NoCache = &initNoCache
	}
	if cmd.Flags().Changed("accounts") {
		fileCfg.Accounts = &initAccounts
	}

	// Apply environment variables (env overrides config.toml but not flags)
	if networkEnv := os.Getenv("DEVNET_NETWORK"); networkEnv != "" && !cmd.Flags().Changed("network") {
		fileCfg.Network = &networkEnv
	}
	if modeEnv := os.Getenv("DEVNET_MODE"); modeEnv != "" && !cmd.Flags().Changed("mode") {
		mode := types.ExecutionMode(modeEnv)
		fileCfg.ExecutionMode = &mode
	}

	// Run partial interactive setup for missing values
	setup := config.NewInteractiveSetup(homeDir)
	effectiveCfg, err := setup.RunPartial(fileCfg)
	if err != nil {
		// Check if it's a missing fields error for better messaging
		if mfErr, ok := err.(*config.MissingFieldsError); ok {
			return outputInitError(fmt.Errorf("missing required configuration: %v\nRun 'devnet-builder config init' to create a configuration file", mfErr.Fields), jsonMode)
		}
		return outputInitError(err, jsonMode)
	}

	// Extract values from effective config
	initNetwork = *effectiveCfg.Network
	initBlockchainNetwork = *effectiveCfg.BlockchainNetwork
	initValidators = *effectiveCfg.Validators
	initMode = string(*effectiveCfg.ExecutionMode)
	if effectiveCfg.NetworkVersion != nil {
		initVersion = *effectiveCfg.NetworkVersion
	}
	if effectiveCfg.NoCache != nil {
		initNoCache = *effectiveCfg.NoCache
	}
	if effectiveCfg.Accounts != nil {
		initAccounts = *effectiveCfg.Accounts
	}

	// Validate inputs
	if !types.NetworkSource(initNetwork).IsValid() {
		return outputInitError(fmt.Errorf("invalid network: %s (must be 'mainnet' or 'testnet')", initNetwork), jsonMode)
	}
	if initValidators < 1 || initValidators > 4 {
		return outputInitError(fmt.Errorf("invalid validators: %d (must be 1-4)", initValidators), jsonMode)
	}
	if !types.ExecutionMode(initMode).IsValid() {
		return outputInitError(fmt.Errorf("invalid mode: %s (must be 'docker' or 'local')", initMode), jsonMode)
	}
	// Validate blockchain network module exists
	if !network.Has(initBlockchainNetwork) {
		available := network.List()
		return outputInitError(fmt.Errorf("unknown blockchain network: %s (available: %v)", initBlockchainNetwork, available), jsonMode)
	}

	// Check if devnet already exists using DevnetService
	svc, err := application.GetService(homeDir)
	if err != nil {
		return outputInitError(fmt.Errorf("failed to initialize service: %w", err), jsonMode)
	}
	if svc.DevnetExists() {
		return outputInitError(fmt.Errorf("devnet already exists at %s\nUse 'devnet-builder destroy' to remove it first", homeDir), jsonMode)
	}

	// Run provision using DevnetService
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
		return outputInitError(err, jsonMode)
	}

	// Load devnet info for output
	devnetInfo, _ := svc.LoadDevnetInfo(ctx)

	// Output result
	if jsonMode {
		return outputInitJSON(result, devnetInfo, homeDir)
	}
	return outputInitText(result, devnetInfo, homeDir)
}

func outputInitText(result *dto.ProvisionOutput, devnetInfo *dto.DevnetInfo, homeDir string) error {
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
	devnetDir := paths.DevnetPath(homeDir)
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
	fmt.Println("  Run 'devnet-builder start' to start the nodes")
	fmt.Println()

	return nil
}

func outputInitJSON(result *dto.ProvisionOutput, devnetInfo *dto.DevnetInfo, homeDir string) error {
	devnetDir := paths.DevnetPath(homeDir)

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
		NextCommand: "devnet-builder start",
	}

	data, err := json.MarshalIndent(jsonResult, "", "  ")
	if err != nil {
		return err
	}

	fmt.Println(string(data))
	return nil
}

func outputInitError(err error, jsonMode bool) error {
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
