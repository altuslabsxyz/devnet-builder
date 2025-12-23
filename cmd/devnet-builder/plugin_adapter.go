package main

import (
	"fmt"

	"cosmossdk.io/log"
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	internalNetwork "github.com/b-harvest/devnet-builder/internal/infrastructure/network"
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
		BuildTags: src.BuildTags,
	}
}

func (a *pluginAdapter) DefaultBinaryVersion() string {
	return a.module.DefaultBinaryVersion()
}

// ============================================
// ChainConfig
// ============================================

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
	// Convert validators from internal to pkg types
	validators := make([]pkgNetwork.ValidatorInfo, len(opts.Validators))
	for i, v := range opts.Validators {
		validators[i] = pkgNetwork.ValidatorInfo{
			Moniker:         v.Moniker,
			ConsPubKey:      v.ConsPubKey,
			OperatorAddress: v.OperatorAddress,
			SelfDelegation:  v.SelfDelegation,
		}
	}

	pkgOpts := pkgNetwork.GenesisOptions{
		ChainID:       opts.ChainID,
		NumValidators: opts.NumValidators,
		Validators:    validators,
	}
	return a.module.ModifyGenesis(genesis, pkgOpts)
}

// ModifyGenesisFile implements FileBasedGenesisModifier for large genesis files.
// This bypasses gRPC message size limits by using file paths instead of raw bytes.
func (a *pluginAdapter) ModifyGenesisFile(inputPath, outputPath string, opts internalNetwork.GenesisOptions) (int64, error) {
	// Check if underlying module supports file-based modification
	fileModifier, ok := a.module.(pkgNetwork.FileBasedGenesisModifier)
	if !ok {
		return 0, fmt.Errorf("plugin does not support file-based genesis modification")
	}

	// Convert validators from internal to pkg types
	validators := make([]pkgNetwork.ValidatorInfo, len(opts.Validators))
	for i, v := range opts.Validators {
		validators[i] = pkgNetwork.ValidatorInfo{
			Moniker:         v.Moniker,
			ConsPubKey:      v.ConsPubKey,
			OperatorAddress: v.OperatorAddress,
			SelfDelegation:  v.SelfDelegation,
		}
	}

	pkgOpts := pkgNetwork.GenesisOptions{
		ChainID:       opts.ChainID,
		NumValidators: opts.NumValidators,
		Validators:    validators,
	}

	return fileModifier.ModifyGenesisFile(inputPath, outputPath, pkgOpts)
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
// NodeConfigurator
// ============================================

func (a *pluginAdapter) GetConfigOverrides(nodeIndex int, opts internalNetwork.NodeConfigOptions) ([]byte, []byte, error) {
	// Convert internal options to pkg options
	pkgOpts := pkgNetwork.NodeConfigOptions{
		ChainID:         opts.ChainID,
		PersistentPeers: opts.PersistentPeers,
		NumValidators:   opts.NumValidators,
		IsValidator:     opts.IsValidator,
		Moniker:         opts.Moniker,
		Ports: pkgNetwork.PortConfig{
			RPC:       opts.Ports.RPC,
			P2P:       opts.Ports.P2P,
			GRPC:      opts.Ports.GRPC,
			GRPCWeb:   opts.Ports.GRPCWeb,
			API:       opts.Ports.API,
			EVMRPC:    opts.Ports.EVMRPC,
			EVMSocket: opts.Ports.EVMWS,
		},
	}
	return a.module.GetConfigOverrides(nodeIndex, pkgOpts)
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

// ============================================
// StateExporter Adapter (Optional Interface)
// ============================================

// AsStateExporter returns a StateExporter if the underlying module implements it.
// Returns nil if the module does not support state export.
func (a *pluginAdapter) AsStateExporter() internalNetwork.StateExporter {
	exporter, ok := a.module.(pkgNetwork.StateExporter)
	if !ok {
		return nil
	}
	return &pluginStateExporterAdapter{module: exporter}
}

// pluginStateExporterAdapter adapts pkg/network.StateExporter to internal/network.StateExporter.
type pluginStateExporterAdapter struct {
	module pkgNetwork.StateExporter
}

// ExportCommandWithOptions returns arguments for exporting genesis/state with options.
func (a *pluginStateExporterAdapter) ExportCommandWithOptions(homeDir string, opts internalNetwork.ExportOptions) []string {
	pkgOpts := pkgNetwork.ExportOptions{
		ForZeroHeight: opts.ForZeroHeight,
		JailWhitelist: opts.JailWhitelist,
		ModulesToSkip: opts.ModulesToSkip,
		Height:        opts.Height,
		OutputPath:    opts.OutputPath,
	}
	return a.module.ExportCommandWithOptions(homeDir, pkgOpts)
}

// ValidateExportedGenesis validates the exported genesis for this network.
func (a *pluginStateExporterAdapter) ValidateExportedGenesis(genesis []byte) error {
	return a.module.ValidateExportedGenesis(genesis)
}

// RequiredModules returns the list of modules that must be present.
func (a *pluginStateExporterAdapter) RequiredModules() []string {
	return a.module.RequiredModules()
}

// SnapshotFormat returns the expected snapshot archive format.
func (a *pluginStateExporterAdapter) SnapshotFormat(networkType string) internalNetwork.SnapshotFormat {
	format := a.module.SnapshotFormat(networkType)
	return internalNetwork.SnapshotFormat(format)
}

