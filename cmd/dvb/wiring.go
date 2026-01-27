// cmd/dvb/wiring.go
// Package main provides wiring for CLI commands.
// This file contains the dependency injection layer that assembles real components
// for the provisioning system.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/altuslabsxyz/devnet-builder/internal/application/ports"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/builder"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/checker"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/controller"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/provisioner"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/runtime"
	"github.com/altuslabsxyz/devnet-builder/internal/plugin/cosmos"
	plugintypes "github.com/altuslabsxyz/devnet-builder/internal/plugin/types"
)

// =============================================================================
// Network Plugin Registry
// =============================================================================

// NetworkPlugin bundles all plugin interfaces for a specific network type.
// This ensures all required interfaces are provided together when configuring
// the provisioning system for a given network.
type NetworkPlugin struct {
	// Name is the network identifier (e.g., "stable", "cosmos")
	Name string

	// BinaryName is the chain binary name (e.g., "stabled", "gaiad")
	BinaryName string

	// DefaultRepo is the default git repository for the binary
	DefaultRepo string

	// Builder handles binary compilation
	Builder plugintypes.PluginBuilder

	// Genesis handles genesis operations
	Genesis plugintypes.PluginGenesis

	// Initializer handles node initialization
	Initializer plugintypes.PluginInitializer

	// Runtime is optional - if nil, ProcessRuntime uses default command
	Runtime runtime.PluginRuntime
}

// NetworkRegistry maps network names to their plugin configurations.
type NetworkRegistry struct {
	networks map[string]*NetworkPlugin
}

// NewNetworkRegistry creates a registry with all supported networks.
func NewNetworkRegistry() *NetworkRegistry {
	r := &NetworkRegistry{
		networks: make(map[string]*NetworkPlugin),
	}

	// Register supported networks
	r.registerStable()
	r.registerCosmos()

	return r
}

// Get returns the plugin for a network, or an error if not found.
func (r *NetworkRegistry) Get(name string) (*NetworkPlugin, error) {
	plugin, ok := r.networks[name]
	if !ok {
		names := make([]string, 0, len(r.networks))
		for n := range r.networks {
			names = append(names, n)
		}
		return nil, fmt.Errorf("unknown network %q (supported: %v)", name, names)
	}
	return plugin, nil
}

// registerStable registers the Stable network (stablelabs chain).
func (r *NetworkRegistry) registerStable() {
	r.networks["stable"] = &NetworkPlugin{
		Name:        "stable",
		BinaryName:  "stabled",
		DefaultRepo: "github.com/stablelabs/stable",
		Builder:     cosmos.NewCosmosBuilder("stabled", "github.com/stablelabs/stable"),
		Genesis: cosmos.NewCosmosGenesis("stabled").
			WithRPCEndpoint("mainnet", "https://rpc.stable.io").
			WithRPCEndpoint("testnet", "https://rpc-testnet.stable.io"),
		Initializer: cosmos.NewCosmosInitializer("stabled"),
	}
}

// registerCosmos registers the Cosmos Hub network.
func (r *NetworkRegistry) registerCosmos() {
	cosmosPlugin := &NetworkPlugin{
		Name:        "cosmos",
		BinaryName:  "gaiad",
		DefaultRepo: "github.com/cosmos/gaia",
		Builder:     cosmos.NewCosmosBuilder("gaiad", "github.com/cosmos/gaia"),
		Genesis:     cosmos.NewCosmosGenesis("gaiad"),
		Initializer: cosmos.NewCosmosInitializer("gaiad"),
	}
	r.networks["cosmos"] = cosmosPlugin
	r.networks["gaia"] = cosmosPlugin // alias
}

// =============================================================================
// Node Initializer Adapter
// =============================================================================

// nodeInitializerAdapter implements ports.NodeInitializer using a cosmos plugin.
// This adapter bridges the plugin interface to the port interface expected by
// the orchestrator.
type nodeInitializerAdapter struct {
	plugin     plugintypes.PluginInitializer
	binaryPath string
	logger     *slog.Logger
}

// newNodeInitializerAdapter creates an adapter implementing ports.NodeInitializer.
func newNodeInitializerAdapter(plugin plugintypes.PluginInitializer, binaryPath string, logger *slog.Logger) *nodeInitializerAdapter {
	return &nodeInitializerAdapter{
		plugin:     plugin,
		binaryPath: binaryPath,
		logger:     logger,
	}
}

// Initialize runs the chain init command for a node.
func (a *nodeInitializerAdapter) Initialize(ctx context.Context, nodeDir, moniker, chainID string) error {
	// Get init command args from plugin
	args := a.plugin.InitCommandArgs(nodeDir, moniker, chainID)

	// Prepend binary path
	cmd := exec.CommandContext(ctx, a.binaryPath, args...)

	a.logger.Info("initializing node",
		"binary", a.binaryPath,
		"args", args,
		"nodeDir", nodeDir,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("init failed: %w\noutput: %s", err, string(output))
	}

	return nil
}

// GetNodeID retrieves the node ID from an initialized node.
func (a *nodeInitializerAdapter) GetNodeID(ctx context.Context, nodeDir string) (string, error) {
	// Try to read from node_key.json first
	nodeKeyPath := filepath.Join(a.plugin.ConfigDir(nodeDir), "node_key.json")
	data, err := os.ReadFile(nodeKeyPath)
	if err == nil {
		// Parse node_key.json to extract ID
		// For now, use tendermint show-node-id command
	}
	_ = data // silence unused warning

	// Fallback: run tendermint show-node-id
	cmd := exec.CommandContext(ctx, a.binaryPath, "tendermint", "show-node-id", "--home", nodeDir)
	output, err := cmd.Output()
	if err != nil {
		// Try alternative: comet show-node-id
		cmd = exec.CommandContext(ctx, a.binaryPath, "comet", "show-node-id", "--home", nodeDir)
		output, err = cmd.Output()
		if err != nil {
			return "", fmt.Errorf("failed to get node ID: %w", err)
		}
	}

	return strings.TrimSpace(string(output)), nil
}

// CreateAccountKey creates a secp256k1 account key.
func (a *nodeInitializerAdapter) CreateAccountKey(ctx context.Context, keyringDir, keyName string) (*ports.AccountKeyInfo, error) {
	cmd := exec.CommandContext(ctx, a.binaryPath,
		"keys", "add", keyName,
		"--keyring-backend", "test",
		"--keyring-dir", keyringDir,
		"--output", "json",
	)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to create key: %w", err)
	}

	return parseKeyOutput(output, a.logger)
}

// CreateAccountKeyFromMnemonic creates/recovers an account key from a mnemonic.
func (a *nodeInitializerAdapter) CreateAccountKeyFromMnemonic(ctx context.Context, keyringDir, keyName, mnemonic string) (*ports.AccountKeyInfo, error) {
	cmd := exec.CommandContext(ctx, a.binaryPath,
		"keys", "add", keyName,
		"--recover",
		"--keyring-backend", "test",
		"--keyring-dir", keyringDir,
		"--output", "json",
	)

	// Provide mnemonic via stdin
	cmd.Stdin = strings.NewReader(mnemonic + "\n")

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to recover key: %w", err)
	}

	return parseKeyOutput(output, a.logger)
}

// GetAccountKey retrieves information about an existing account key.
func (a *nodeInitializerAdapter) GetAccountKey(ctx context.Context, keyringDir, keyName string) (*ports.AccountKeyInfo, error) {
	cmd := exec.CommandContext(ctx, a.binaryPath,
		"keys", "show", keyName,
		"--keyring-backend", "test",
		"--keyring-dir", keyringDir,
		"--output", "json",
	)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get key: %w", err)
	}

	return parseKeyOutput(output, a.logger)
}

// GetTestMnemonic returns a deterministic test mnemonic for the given validator index.
func (a *nodeInitializerAdapter) GetTestMnemonic(validatorIndex int) string {
	// Well-known test mnemonics (these are public test values)
	testMnemonics := []string{
		"abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about",
		"zoo zoo zoo zoo zoo zoo zoo zoo zoo zoo zoo wrong",
		"vessel ladder alter error federal sibling chat ability sun glass valve picture",
		"range sheriff try enroll deer over ten level bring display stamp recycle",
	}

	if validatorIndex < len(testMnemonics) {
		return testMnemonics[validatorIndex]
	}

	// For additional validators, return a deterministic but less memorable mnemonic
	return testMnemonics[validatorIndex%len(testMnemonics)]
}

// keyOutputV1 is the JSON format for cosmos-sdk <= v0.45.x
type keyOutputV1 struct {
	Name     string `json:"name"`
	Address  string `json:"address"`
	PubKey   string `json:"pubkey"`
	Mnemonic string `json:"mnemonic,omitempty"`
}

// keyOutputV2 is the JSON format for cosmos-sdk >= v0.46.x
type keyOutputV2 struct {
	Name     string `json:"name"`
	Address  string `json:"address"`
	PubKey   string `json:"pubkey"`
	Mnemonic string `json:"mnemonic,omitempty"`
	Type     string `json:"type,omitempty"`
}

// parseKeyOutput parses the JSON output from keys commands.
// The output format varies by cosmos-sdk version, so we handle both formats.
func parseKeyOutput(output []byte, logger *slog.Logger) (*ports.AccountKeyInfo, error) {
	logger.Debug("parsing key output", "output", string(output))

	// Trim any leading/trailing whitespace
	trimmed := strings.TrimSpace(string(output))
	if trimmed == "" {
		return nil, fmt.Errorf("empty key output")
	}

	// Try parsing as the newer v2 format first (more common)
	var v2 keyOutputV2
	if err := json.Unmarshal([]byte(trimmed), &v2); err == nil && v2.Address != "" {
		return &ports.AccountKeyInfo{
			Name:     v2.Name,
			Address:  v2.Address,
			PubKey:   v2.PubKey,
			Mnemonic: v2.Mnemonic,
		}, nil
	}

	// Fall back to v1 format
	var v1 keyOutputV1
	if err := json.Unmarshal([]byte(trimmed), &v1); err == nil && v1.Address != "" {
		return &ports.AccountKeyInfo{
			Name:     v1.Name,
			Address:  v1.Address,
			PubKey:   v1.PubKey,
			Mnemonic: v1.Mnemonic,
		}, nil
	}

	// If neither format works, try direct unmarshal into AccountKeyInfo
	var info ports.AccountKeyInfo
	if err := json.Unmarshal([]byte(trimmed), &info); err != nil {
		return nil, fmt.Errorf("failed to parse key output: %w", err)
	}

	if info.Address == "" {
		return nil, fmt.Errorf("unrecognized key output format: missing address field")
	}

	return &info, nil
}

// Ensure nodeInitializerAdapter implements ports.NodeInitializer
var _ ports.NodeInitializer = (*nodeInitializerAdapter)(nil)

// =============================================================================
// Component Factory
// =============================================================================

// ComponentFactory creates real component instances with proper dependency injection.
type ComponentFactory struct {
	registry *NetworkRegistry
	dataDir  string
	logger   *slog.Logger
}

// NewComponentFactory creates a new component factory.
func NewComponentFactory(dataDir string, logger *slog.Logger) *ComponentFactory {
	return &ComponentFactory{
		registry: NewNetworkRegistry(),
		dataDir:  dataDir,
		logger:   logger,
	}
}

// GetNetworkPlugin returns the plugin for a network.
func (f *ComponentFactory) GetNetworkPlugin(network string) (*NetworkPlugin, error) {
	return f.registry.Get(network)
}

// CreateBinaryBuilder creates a real binary builder.
// The builder uses the plugin system to handle network-specific build logic.
func (f *ComponentFactory) CreateBinaryBuilder() *builder.DefaultBuilder {
	return builder.NewDefaultBuilder(f.dataDir, f, f.logger)
}

// GetBuilder implements builder.PluginLoader interface.
func (f *ComponentFactory) GetBuilder(pluginName string) (plugintypes.PluginBuilder, error) {
	plugin, err := f.registry.Get(pluginName)
	if err != nil {
		return nil, err
	}
	return plugin.Builder, nil
}

// CreateGenesisForker creates a real genesis forker for a specific network.
func (f *ComponentFactory) CreateGenesisForker(network string) (*provisioner.GenesisForker, error) {
	plugin, err := f.registry.Get(network)
	if err != nil {
		return nil, err
	}

	return provisioner.NewGenesisForker(provisioner.GenesisForkerConfig{
		DataDir:       f.dataDir,
		PluginGenesis: plugin.Genesis,
		Logger:        f.logger,
	}), nil
}

// CreateNodeInitializer creates a real node initializer for a specific network.
// Returns an adapter that implements ports.NodeInitializer.
func (f *ComponentFactory) CreateNodeInitializer(network, binaryPath string) (ports.NodeInitializer, error) {
	plugin, err := f.registry.Get(network)
	if err != nil {
		return nil, err
	}

	return newNodeInitializerAdapter(plugin.Initializer, binaryPath, f.logger), nil
}

// CreateProcessRuntime creates a real process runtime for a specific network.
func (f *ComponentFactory) CreateProcessRuntime(network string) (*runtime.ProcessRuntime, error) {
	plugin, err := f.registry.Get(network)
	if err != nil {
		return nil, err
	}

	return runtime.NewProcessRuntime(runtime.ProcessRuntimeConfig{
		DataDir:       f.dataDir,
		PluginRuntime: plugin.Runtime, // May be nil, ProcessRuntime handles this
		Logger:        f.logger,
	}), nil
}

// CreateHealthChecker creates a real health checker.
func (f *ComponentFactory) CreateHealthChecker() controller.HealthChecker {
	return checker.NewRPCHealthChecker(checker.DefaultConfig())
}

// =============================================================================
// Orchestrator Factory
// =============================================================================

// OrchestratorOptions configures orchestrator creation.
type OrchestratorOptions struct {
	Network    string
	BinaryPath string // Optional: if provided, skips build
	DataDir    string
	Logger     *slog.Logger
}

// CreateOrchestrator creates a fully-wired ProvisioningOrchestrator.
// This is the main entry point for CLI commands that need to provision devnets.
func CreateOrchestrator(opts OrchestratorOptions) (*provisioner.ProvisioningOrchestrator, error) {
	factory := NewComponentFactory(opts.DataDir, opts.Logger)

	// Get network plugin
	plugin, err := factory.GetNetworkPlugin(opts.Network)
	if err != nil {
		return nil, err
	}

	// Create builder
	binaryBuilder := factory.CreateBinaryBuilder()

	// Create genesis forker
	genesisForker, err := factory.CreateGenesisForker(opts.Network)
	if err != nil {
		return nil, fmt.Errorf("failed to create genesis forker: %w", err)
	}

	// Determine binary path for node initializer
	// If not provided, we'll use the binary name - the orchestrator will update it after build
	binaryPath := opts.BinaryPath
	if binaryPath == "" {
		binaryPath = plugin.BinaryName // Will be resolved during build phase
	}

	// Create node initializer
	nodeInitializer, err := factory.CreateNodeInitializer(opts.Network, binaryPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create node initializer: %w", err)
	}

	// Create runtime
	nodeRuntime, err := factory.CreateProcessRuntime(opts.Network)
	if err != nil {
		return nil, fmt.Errorf("failed to create process runtime: %w", err)
	}

	// Create health checker
	healthChecker := factory.CreateHealthChecker()

	// Assemble orchestrator config
	config := provisioner.OrchestratorConfig{
		BinaryBuilder:   binaryBuilder,
		GenesisForker:   genesisForker,
		NodeInitializer: nodeInitializer,
		NodeRuntime:     nodeRuntime,
		HealthChecker:   healthChecker,
		DataDir:         opts.DataDir,
		Logger:          opts.Logger,
	}

	return provisioner.NewProvisioningOrchestrator(config), nil
}
