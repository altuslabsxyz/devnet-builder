package main

import (
	"fmt"

	"cosmossdk.io/log"
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	internalNetwork "github.com/b-harvest/devnet-builder/internal/network"
	pkgNetwork "github.com/b-harvest/devnet-builder/pkg/network"
)

// pluginAdapter adapts pkg/network.Module to internal/network.NetworkModule.
// This allows plugins to be registered with the internal network registry.
type pluginAdapter struct {
	module pkgNetwork.Module
}

// newPluginAdapter creates a new adapter for a plugin module.
func newPluginAdapter(module pkgNetwork.Module) *pluginAdapter {
	return &pluginAdapter{module: module}
}

// Verify interface compliance at compile time.
var _ internalNetwork.NetworkModule = (*pluginAdapter)(nil)

// ============================================
// NetworkIdentity
// ============================================

func (a *pluginAdapter) Name() string {
	return a.module.Name()
}

func (a *pluginAdapter) DisplayName() string {
	return a.module.DisplayName()
}

func (a *pluginAdapter) Version() string {
	return a.module.Version()
}

// ============================================
// BinaryProvider
// ============================================

func (a *pluginAdapter) BinaryName() string {
	return a.module.BinaryName()
}

func (a *pluginAdapter) BinarySource() internalNetwork.BinarySource {
	src := a.module.BinarySource()
	return internalNetwork.BinarySource{
		Type:      internalNetwork.BinarySourceType(src.Type),
		Owner:     src.Owner,
		Repo:      src.Repo,
		LocalPath: src.LocalPath,
	}
}

func (a *pluginAdapter) DefaultBinaryVersion() string {
	return a.module.DefaultBinaryVersion()
}

// ============================================
// ChainConfig
// ============================================

func (a *pluginAdapter) DefaultChainID() string {
	return a.module.DefaultChainID()
}

func (a *pluginAdapter) Bech32Prefix() string {
	return a.module.Bech32Prefix()
}

func (a *pluginAdapter) BaseDenom() string {
	return a.module.BaseDenom()
}

func (a *pluginAdapter) GenesisConfig() internalNetwork.GenesisConfig {
	cfg := a.module.GenesisConfig()
	return internalNetwork.GenesisConfig{
		ChainIDPattern:    cfg.ChainIDPattern,
		EVMChainID:        cfg.EVMChainID,
		BaseDenom:         cfg.BaseDenom,
		DenomExponent:     cfg.DenomExponent,
		DisplayDenom:      cfg.DisplayDenom,
		BondDenom:         cfg.BondDenom,
		MinSelfDelegation: cfg.MinSelfDelegation,
		UnbondingTime:     cfg.UnbondingTime,
		MaxValidators:     cfg.MaxValidators,
		MinDeposit:        cfg.MinDeposit,
		VotingPeriod:      cfg.VotingPeriod,
		MaxDepositPeriod:  cfg.MaxDepositPeriod,
		CommunityTax:      cfg.CommunityTax,
	}
}

// ============================================
// DockerConfig
// ============================================

func (a *pluginAdapter) DockerImage() string {
	return a.module.DockerImage()
}

func (a *pluginAdapter) DockerImageTag(version string) string {
	return a.module.DockerImageTag(version)
}

func (a *pluginAdapter) DockerHomeDir() string {
	return a.module.DockerHomeDir()
}

// ============================================
// CommandBuilder
// ============================================

func (a *pluginAdapter) InitCommand(homeDir, chainID, moniker string) []string {
	return a.module.InitCommand(homeDir, chainID, moniker)
}

func (a *pluginAdapter) StartCommand(homeDir string) []string {
	return a.module.StartCommand(homeDir)
}

func (a *pluginAdapter) ExportCommand(homeDir string) []string {
	return a.module.ExportCommand(homeDir)
}

// ============================================
// ProcessConfig
// ============================================

func (a *pluginAdapter) DefaultNodeHome() string {
	return a.module.DefaultNodeHome()
}

func (a *pluginAdapter) PIDFileName() string {
	return a.module.PIDFileName()
}

func (a *pluginAdapter) LogFileName() string {
	return a.module.LogFileName()
}

func (a *pluginAdapter) ProcessPattern() string {
	return a.module.ProcessPattern()
}

func (a *pluginAdapter) DefaultPorts() internalNetwork.PortConfig {
	ports := a.module.DefaultPorts()
	return internalNetwork.PortConfig{
		RPC:     ports.RPC,
		P2P:     ports.P2P,
		GRPC:    ports.GRPC,
		GRPCWeb: ports.GRPCWeb,
		API:     ports.API,
		EVMRPC:  ports.EVMRPC,
		EVMWS:   ports.EVMSocket, // pkg uses EVMSocket, internal uses EVMWS
	}
}

// ============================================
// GenesisModifier
// ============================================

func (a *pluginAdapter) ModifyGenesis(genesis []byte, opts internalNetwork.GenesisOptions) ([]byte, error) {
	pkgOpts := pkgNetwork.GenesisOptions{
		ChainID:       opts.ChainID,
		NumValidators: opts.NumValidators,
	}
	return a.module.ModifyGenesis(genesis, pkgOpts)
}

// ============================================
// DevnetGenerator
// ============================================

func (a *pluginAdapter) NewGenerator(config *internalNetwork.GeneratorConfig, logger log.Logger) (internalNetwork.Generator, error) {
	// Plugins use GenerateDevnet directly, not the Generator pattern
	// Return a plugin-based generator adapter
	return &pluginGeneratorAdapter{
		module: a.module,
		config: config,
		logger: logger,
	}, nil
}

func (a *pluginAdapter) DefaultGeneratorConfig() *internalNetwork.GeneratorConfig {
	cfg := a.module.DefaultGeneratorConfig()

	// Parse account balance from JSON string
	accountBalance, err := sdk.ParseCoinsNormalized(cfg.AccountBalance)
	if err != nil {
		accountBalance = sdk.NewCoins()
	}

	// Parse validator balance from JSON string
	validatorBalance, err := sdk.ParseCoinsNormalized(cfg.ValidatorBalance)
	if err != nil {
		validatorBalance = sdk.NewCoins()
	}

	// Parse validator stake from JSON string
	validatorStake, ok := math.NewIntFromString(cfg.ValidatorStake)
	if !ok {
		validatorStake = math.ZeroInt()
	}

	return &internalNetwork.GeneratorConfig{
		NumValidators:    cfg.NumValidators,
		NumAccounts:      cfg.NumAccounts,
		AccountBalance:   accountBalance,
		ValidatorBalance: validatorBalance,
		ValidatorStake:   validatorStake,
		OutputDir:        cfg.OutputDir,
		ChainID:          cfg.ChainID,
	}
}

// ============================================
// Validator
// ============================================

func (a *pluginAdapter) Validate() error {
	return a.module.Validate()
}

// ============================================
// SnapshotProvider
// ============================================

func (a *pluginAdapter) SnapshotURL(networkType string) string {
	return a.module.SnapshotURL(networkType)
}

func (a *pluginAdapter) RPCEndpoint(networkType string) string {
	return a.module.RPCEndpoint(networkType)
}

func (a *pluginAdapter) AvailableNetworks() []string {
	return a.module.AvailableNetworks()
}

// ============================================
// Generator Adapter
// ============================================

// pluginGeneratorAdapter adapts plugin GenerateDevnet to the Generator interface.
type pluginGeneratorAdapter struct {
	module     pkgNetwork.Module
	config     *internalNetwork.GeneratorConfig
	logger     log.Logger
	validators []internalNetwork.ValidatorInfo
	accounts   []internalNetwork.AccountInfo
}

// Build generates validators, modifies genesis, and saves to node directories.
func (g *pluginGeneratorAdapter) Build(genesisFile string) error {
	// Convert internal config to pkg config
	pkgConfig := pkgNetwork.GeneratorConfig{
		NumValidators:    g.config.NumValidators,
		NumAccounts:      g.config.NumAccounts,
		AccountBalance:   g.config.AccountBalance.String(),
		ValidatorBalance: g.config.ValidatorBalance.String(),
		ValidatorStake:   g.config.ValidatorStake.String(),
		OutputDir:        g.config.OutputDir,
		ChainID:          g.config.ChainID,
	}

	// Call the plugin's GenerateDevnet
	// Note: The plugin handles all file creation internally
	return g.module.GenerateDevnet(nil, pkgConfig, genesisFile)
}

// GetValidators returns the generated validators info.
func (g *pluginGeneratorAdapter) GetValidators() []internalNetwork.ValidatorInfo {
	// Plugin-based generation stores validators externally
	// For now, return placeholder data based on config
	validators := make([]internalNetwork.ValidatorInfo, g.config.NumValidators)
	for i := 0; i < g.config.NumValidators; i++ {
		validators[i] = internalNetwork.ValidatorInfo{
			Moniker: fmt.Sprintf("node%d", i),
			Tokens:  math.NewInt(100),
		}
	}
	return validators
}

// GetAccounts returns the generated accounts info.
func (g *pluginGeneratorAdapter) GetAccounts() []internalNetwork.AccountInfo {
	// Plugin-based generation stores accounts externally
	// For now, return placeholder data based on config
	accounts := make([]internalNetwork.AccountInfo, g.config.NumAccounts)
	for i := 0; i < g.config.NumAccounts; i++ {
		accounts[i] = internalNetwork.AccountInfo{
			Name: fmt.Sprintf("account%d", i),
		}
	}
	return accounts
}

