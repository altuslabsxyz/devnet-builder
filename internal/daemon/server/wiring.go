// internal/daemon/server/wiring.go
// Package server provides wiring for the daemon server.
// This file contains dependency injection for the provisioning system,
// adapting the pattern from cmd/dvb/wiring.go for daemon context.
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"sync"

	"github.com/altuslabsxyz/devnet-builder/internal/application/ports"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/builder"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/provisioner"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/runtime"
	"github.com/altuslabsxyz/devnet-builder/internal/plugin/cosmos"
	plugintypes "github.com/altuslabsxyz/devnet-builder/internal/plugin/types"
)

// =============================================================================
// Network Plugin Registry
// =============================================================================

// NetworkPlugin bundles all plugin interfaces for a specific network type.
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

	// Runtime provides container/process runtime configuration
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

// GetPluginRuntime returns the PluginRuntime for a network.
func (r *NetworkRegistry) GetPluginRuntime(name string) (runtime.PluginRuntime, error) {
	plugin, err := r.Get(name)
	if err != nil {
		return nil, err
	}
	return plugin.Runtime, nil
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
		Runtime:     cosmos.NewCosmosRuntime("stabled"),
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
		Runtime:     cosmos.NewCosmosRuntime("gaiad"),
	}
	r.networks["cosmos"] = cosmosPlugin
	r.networks["gaia"] = cosmosPlugin // alias
}

// =============================================================================
// Node Initializer Adapter
// =============================================================================

// nodeInitializerAdapter implements ports.NodeInitializer and provisioner.BinaryPathUpdater.
// This adapter bridges the plugin interface to the port interface expected by
// the orchestrator. It supports deferred binary path injection since the
// daemon creates the orchestrator before the binary is built.
type nodeInitializerAdapter struct {
	plugin     plugintypes.PluginInitializer
	binaryName string // Binary name (e.g., "stabled", "gaiad")
	binaryPath string // Full path to binary (set after build via SetBinaryPath)
	logger     *slog.Logger
	mu         sync.RWMutex // Protects binaryPath
}

// Ensure nodeInitializerAdapter implements ports.NodeInitializer
var _ ports.NodeInitializer = (*nodeInitializerAdapter)(nil)

// Ensure nodeInitializerAdapter implements provisioner.BinaryPathUpdater
var _ provisioner.BinaryPathUpdater = (*nodeInitializerAdapter)(nil)

// newNodeInitializerAdapter creates an adapter implementing ports.NodeInitializer.
// The binaryPath is not set at construction time; call SetBinaryPath after build.
func newNodeInitializerAdapter(plugin plugintypes.PluginInitializer, binaryName string, logger *slog.Logger) *nodeInitializerAdapter {
	return &nodeInitializerAdapter{
		plugin:     plugin,
		binaryName: binaryName,
		logger:     logger,
	}
}

// SetBinaryPath sets the binary path for use in subsequent operations.
// This implements provisioner.BinaryPathUpdater and is called by the orchestrator
// after the build phase completes.
func (a *nodeInitializerAdapter) SetBinaryPath(path string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.binaryPath = path
	a.logger.Debug("binary path set", "binaryPath", path)
}

// getBinaryPath returns the current binary path (thread-safe).
func (a *nodeInitializerAdapter) getBinaryPath() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.binaryPath
}

// Initialize runs the chain init command for a node.
// The binaryPath must have been set via SetBinaryPath before calling this method.
func (a *nodeInitializerAdapter) Initialize(ctx context.Context, nodeDir, moniker, chainID string) error {
	binaryPath := a.getBinaryPath()
	if binaryPath == "" {
		return fmt.Errorf("binary path not set; SetBinaryPath must be called before Initialize")
	}

	// Get init command args from plugin
	args := a.plugin.InitCommandArgs(nodeDir, moniker, chainID)

	// Run the init command
	cmd := exec.CommandContext(ctx, binaryPath, args...)

	a.logger.Info("initializing node",
		"binary", binaryPath,
		"args", args,
		"nodeDir", nodeDir,
		"moniker", moniker,
		"chainID", chainID,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("init failed: %w\noutput: %s", err, string(output))
	}

	a.logger.Debug("node initialization completed",
		"nodeDir", nodeDir,
		"output", string(output),
	)

	return nil
}

// GetNodeID retrieves the node ID from an initialized node.
func (a *nodeInitializerAdapter) GetNodeID(ctx context.Context, nodeDir string) (string, error) {
	binaryPath := a.getBinaryPath()
	if binaryPath == "" {
		return "", fmt.Errorf("binary path not set; SetBinaryPath must be called before GetNodeID")
	}

	// Try tendermint show-node-id first
	cmd := exec.CommandContext(ctx, binaryPath, "tendermint", "show-node-id", "--home", nodeDir)
	output, err := cmd.Output()
	if err != nil {
		// Try alternative: comet show-node-id (newer SDK versions)
		cmd = exec.CommandContext(ctx, binaryPath, "comet", "show-node-id", "--home", nodeDir)
		output, err = cmd.Output()
		if err != nil {
			return "", fmt.Errorf("failed to get node ID: %w", err)
		}
	}

	return strings.TrimSpace(string(output)), nil
}

// CreateAccountKey creates a secp256k1 account key.
func (a *nodeInitializerAdapter) CreateAccountKey(ctx context.Context, keyringDir, keyName string) (*ports.AccountKeyInfo, error) {
	binaryPath := a.getBinaryPath()
	if binaryPath == "" {
		return nil, fmt.Errorf("binary path not set; SetBinaryPath must be called before CreateAccountKey")
	}

	cmd := exec.CommandContext(ctx, binaryPath,
		"keys", "add", keyName,
		"--keyring-backend", "test",
		"--keyring-dir", keyringDir,
		"--output", "json",
	)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to create key: %w", err)
	}

	return a.parseKeyOutput(output)
}

// CreateAccountKeyFromMnemonic creates/recovers an account key from a mnemonic.
func (a *nodeInitializerAdapter) CreateAccountKeyFromMnemonic(ctx context.Context, keyringDir, keyName, mnemonic string) (*ports.AccountKeyInfo, error) {
	binaryPath := a.getBinaryPath()
	if binaryPath == "" {
		return nil, fmt.Errorf("binary path not set; SetBinaryPath must be called before CreateAccountKeyFromMnemonic")
	}

	cmd := exec.CommandContext(ctx, binaryPath,
		"keys", "add", keyName,
		"--keyring-backend", "test",
		"--keyring-dir", keyringDir,
		"--recover",
		"--output", "json",
	)

	// Provide mnemonic via stdin
	cmd.Stdin = strings.NewReader(mnemonic + "\n")

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to recover key from mnemonic: %w", err)
	}

	return a.parseKeyOutput(output)
}

// GetAccountKey retrieves information about an existing account key.
func (a *nodeInitializerAdapter) GetAccountKey(ctx context.Context, keyringDir, keyName string) (*ports.AccountKeyInfo, error) {
	binaryPath := a.getBinaryPath()
	if binaryPath == "" {
		return nil, fmt.Errorf("binary path not set; SetBinaryPath must be called before GetAccountKey")
	}

	cmd := exec.CommandContext(ctx, binaryPath,
		"keys", "show", keyName,
		"--keyring-backend", "test",
		"--keyring-dir", keyringDir,
		"--output", "json",
	)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get key: %w", err)
	}

	return a.parseKeyOutput(output)
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
func (a *nodeInitializerAdapter) parseKeyOutput(output []byte) (*ports.AccountKeyInfo, error) {
	a.logger.Debug("parsing key output", "output", string(output))

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
	// (in case the JSON tags already match)
	var direct ports.AccountKeyInfo
	if err := json.Unmarshal([]byte(trimmed), &direct); err == nil && direct.Address != "" {
		return &direct, nil
	}

	// Return error with context for debugging
	return nil, fmt.Errorf("failed to parse key output: unrecognized format (output: %.100s...)", trimmed)
}

// GetTestMnemonic returns a deterministic test mnemonic for the given validator index.
func (a *nodeInitializerAdapter) GetTestMnemonic(validatorIndex int) string {
	testMnemonics := []string{
		"abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about",
		"zoo zoo zoo zoo zoo zoo zoo zoo zoo zoo zoo wrong",
		"vessel ladder alter error federal sibling chat ability sun glass valve picture",
		"range sheriff try enroll deer over ten level bring display stamp recycle",
	}
	if validatorIndex < len(testMnemonics) {
		return testMnemonics[validatorIndex]
	}
	return testMnemonics[validatorIndex%len(testMnemonics)]
}

// =============================================================================
// Orchestrator Factory
// =============================================================================

// OrchestratorFactory creates orchestrators for the daemon.
type OrchestratorFactory struct {
	registry *NetworkRegistry
	dataDir  string
	logger   *slog.Logger
}

// NewOrchestratorFactory creates a new orchestrator factory.
func NewOrchestratorFactory(dataDir string, logger *slog.Logger) *OrchestratorFactory {
	return &OrchestratorFactory{
		registry: NewNetworkRegistry(),
		dataDir:  dataDir,
		logger:   logger,
	}
}

// GetBuilder implements builder.PluginLoader interface.
func (f *OrchestratorFactory) GetBuilder(pluginName string) (plugintypes.PluginBuilder, error) {
	plugin, err := f.registry.Get(pluginName)
	if err != nil {
		return nil, err
	}
	return plugin.Builder, nil
}

// GetPluginRuntime returns the PluginRuntime for a network.
func (f *OrchestratorFactory) GetPluginRuntime(pluginName string) (runtime.PluginRuntime, error) {
	return f.registry.GetPluginRuntime(pluginName)
}

// CreateOrchestrator creates an Orchestrator for the given network.
// In daemon mode, the orchestrator is configured to skip the start phase
// (SkipStart=true in ProvisionOptions), so NodeRuntime is not needed.
// Returns provisioner.Orchestrator interface for testability.
func (f *OrchestratorFactory) CreateOrchestrator(network string) (provisioner.Orchestrator, error) {
	plugin, err := f.registry.Get(network)
	if err != nil {
		return nil, err
	}

	// Create binary builder
	binaryBuilder := builder.NewDefaultBuilder(f.dataDir, f, f.logger)

	// Create genesis forker
	genesisForker := provisioner.NewGenesisForker(provisioner.GenesisForkerConfig{
		DataDir:       f.dataDir,
		PluginGenesis: plugin.Genesis,
		Logger:        f.logger,
	})

	// Create node initializer adapter
	// Note: The adapter is a placeholder since actual init uses the built binary path
	nodeInitializer := newNodeInitializerAdapter(plugin.Initializer, plugin.BinaryName, f.logger)

	// Assemble orchestrator config
	// NodeRuntime and HealthChecker are nil since daemon uses SkipStart=true
	// and NodeController handles starting/health checking
	config := provisioner.OrchestratorConfig{
		BinaryBuilder:   binaryBuilder,
		GenesisForker:   genesisForker,
		NodeInitializer: nodeInitializer,
		// NodeRuntime: nil - not needed, daemon skips start phase
		// HealthChecker: nil - not needed, NodeController handles health
		DataDir: f.dataDir,
		Logger:  f.logger,
	}

	return provisioner.NewProvisioningOrchestrator(config), nil
}
