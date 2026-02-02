// internal/daemon/server/wiring.go
// Package server provides wiring for the daemon server.
// This file contains dependency injection for the provisioning system,
// using NetworkModule from the global registry (populated by loaded plugins).
package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/application/ports"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/builder"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/provisioner"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/runtime"
	daemontypes "github.com/altuslabsxyz/devnet-builder/internal/daemon/types"
	"github.com/altuslabsxyz/devnet-builder/internal/infrastructure/genesis"
	"github.com/altuslabsxyz/devnet-builder/internal/infrastructure/network"
	"github.com/altuslabsxyz/devnet-builder/internal/infrastructure/snapshot"
	"github.com/altuslabsxyz/devnet-builder/internal/infrastructure/stateexport"
	plugintypes "github.com/altuslabsxyz/devnet-builder/internal/plugin/types"
	sdknetwork "github.com/altuslabsxyz/devnet-builder/pkg/network"
)

// =============================================================================
// NetworkModule Adapters
// =============================================================================
// These adapters bridge NetworkModule (new interface) to the old plugin types
// (PluginBuilder, PluginGenesis, PluginInitializer). This allows gradual migration.

// moduleBuilderAdapter adapts NetworkModule to plugintypes.PluginBuilder.
type moduleBuilderAdapter struct {
	module network.NetworkModule
}

func (a *moduleBuilderAdapter) BinaryName() string {
	return a.module.BinaryName()
}

func (a *moduleBuilderAdapter) DefaultGitRepo() string {
	src := a.module.BinarySource()
	if src.IsLocal() {
		return src.LocalPath
	}
	return fmt.Sprintf("github.com/%s/%s", src.Owner, src.Repo)
}

func (a *moduleBuilderAdapter) DefaultBuildFlags() map[string]string {
	// Get build config from plugin - this is the authoritative source for build flags
	cfg, err := a.module.GetBuildConfig("")
	if err != nil || cfg == nil {
		return nil
	}
	// Convert BuildConfig to map format for compatibility
	flags := make(map[string]string)
	if len(cfg.Tags) > 0 {
		flags["tags"] = strings.Join(cfg.Tags, ",")
	}
	if len(cfg.LDFlags) > 0 {
		flags["ldflags"] = strings.Join(cfg.LDFlags, " ")
	}
	return flags
}

func (a *moduleBuilderAdapter) BuildBinary(ctx context.Context, opts plugintypes.BuildOptions) error {
	binaryName := a.module.BinaryName()
	outputPath := filepath.Join(opts.OutputDir, binaryName)

	// Get build config from plugin - plugins define their own build configuration
	buildCfg, err := a.module.GetBuildConfig("")
	if err != nil {
		if opts.Logger != nil {
			opts.Logger.Warn("failed to get build config from plugin, using defaults", "error", err)
		}
	}

	// Check if Makefile exists - most Cosmos SDK chains use make install
	makefilePath := filepath.Join(opts.SourceDir, "Makefile")
	if _, err := os.Stat(makefilePath); err == nil {
		return a.buildWithMake(ctx, opts, buildCfg, outputPath)
	}

	// Fallback to direct go build
	return a.buildWithGo(ctx, opts, buildCfg, outputPath)
}

func (a *moduleBuilderAdapter) buildWithMake(ctx context.Context, opts plugintypes.BuildOptions, buildCfg *sdknetwork.BuildConfig, outputPath string) error {
	// Pass VERSION and COMMIT as make variables - most Cosmos SDK Makefiles use these
	makeArgs := []string{"install"}
	if opts.GitRef != "" {
		makeArgs = append(makeArgs, fmt.Sprintf("VERSION=%s", opts.GitRef))
	}
	if opts.GitCommit != "" {
		makeArgs = append(makeArgs, fmt.Sprintf("COMMIT=%s", opts.GitCommit))
	}

	// Start with base environment
	env := append(os.Environ(),
		"GO111MODULE=on",
		fmt.Sprintf("GOBIN=%s", opts.OutputDir),
		fmt.Sprintf("VERSION=%s", opts.GitRef),
		fmt.Sprintf("COMMIT=%s", opts.GitCommit),
	)

	// Apply environment from plugin's build config
	if buildCfg != nil {
		for k, v := range buildCfg.Env {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	// Ensure CGO_ENABLED is set (default to 1 if not specified by plugin)
	hasCGO := false
	for _, e := range env {
		if strings.HasPrefix(e, "CGO_ENABLED=") {
			hasCGO = true
			break
		}
	}
	if !hasCGO {
		env = append(env, "CGO_ENABLED=1")
	}

	cmd := exec.CommandContext(ctx, "make", makeArgs...)
	cmd.Dir = opts.SourceDir
	cmd.Env = env

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if opts.Logger != nil {
		opts.Logger.Info("running make install", "dir", opts.SourceDir, "version", opts.GitRef)
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("make install failed: %w\nstdout: %s\nstderr: %s",
			err, stdout.String(), stderr.String())
	}

	// Verify binary was created
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		return fmt.Errorf("binary not found at %s after make install", outputPath)
	}

	return nil
}

func (a *moduleBuilderAdapter) buildWithGo(ctx context.Context, opts plugintypes.BuildOptions, buildCfg *sdknetwork.BuildConfig, outputPath string) error {
	binaryName := a.module.BinaryName()

	// Build ldflags from plugin config + version injection
	var ldflagParts []string

	// Add plugin-provided ldflags first
	if buildCfg != nil && len(buildCfg.LDFlags) > 0 {
		ldflagParts = append(ldflagParts, buildCfg.LDFlags...)
	}

	// Add standard Cosmos SDK version injection flags
	// These use the standard cosmos-sdk/version package paths
	ldflagParts = append(ldflagParts,
		"-w", "-s",
		fmt.Sprintf("-X github.com/cosmos/cosmos-sdk/version.Name=%s", binaryName),
		fmt.Sprintf("-X github.com/cosmos/cosmos-sdk/version.AppName=%s", binaryName),
		fmt.Sprintf("-X github.com/cosmos/cosmos-sdk/version.Version=%s", opts.GitRef),
		fmt.Sprintf("-X github.com/cosmos/cosmos-sdk/version.Commit=%s", opts.GitCommit),
	)

	ldflags := strings.Join(ldflagParts, " ")
	args := []string{"build", "-o", outputPath, "-ldflags", ldflags}

	// Add build tags from plugin config
	if buildCfg != nil && len(buildCfg.Tags) > 0 {
		args = append(args, "-tags", strings.Join(buildCfg.Tags, ","))
	} else if tags, ok := opts.Flags["tags"]; ok && tags != "" {
		// Fallback to opts.Flags for backward compatibility
		args = append(args, "-tags", tags)
	}

	// Add main package path
	args = append(args, "./cmd/"+binaryName)

	// Build environment
	env := append(os.Environ(), "GO111MODULE=on")

	// Apply environment from plugin's build config
	if buildCfg != nil {
		for k, v := range buildCfg.Env {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	// Ensure CGO_ENABLED is set (default to 1 if not specified by plugin)
	hasCGO := false
	for _, e := range env {
		if strings.HasPrefix(e, "CGO_ENABLED=") {
			hasCGO = true
			break
		}
	}
	if !hasCGO {
		env = append(env, "CGO_ENABLED=1")
	}

	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = opts.SourceDir
	cmd.Env = env

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if opts.Logger != nil {
		opts.Logger.Info("building binary", "binary", binaryName, "output", outputPath, "ldflags", ldflags)
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go build failed: %w\nstdout: %s\nstderr: %s",
			err, stdout.String(), stderr.String())
	}

	return nil
}

func (a *moduleBuilderAdapter) ValidateBinary(ctx context.Context, binaryPath string) error {
	// Run binary with --version or version flag to validate it works
	cmd := exec.CommandContext(ctx, binaryPath, "version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Try --version as alternative
		cmd = exec.CommandContext(ctx, binaryPath, "--version")
		output, err = cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("binary validation failed: %w\noutput: %s", err, string(output))
		}
	}
	return nil
}

// Ensure interface compliance
var _ plugintypes.PluginBuilder = (*moduleBuilderAdapter)(nil)

// moduleGenesisAdapter adapts NetworkModule to plugintypes.PluginGenesis.
type moduleGenesisAdapter struct {
	module network.NetworkModule
}

func (a *moduleGenesisAdapter) GetRPCEndpoint(networkType string) string {
	return a.module.RPCEndpoint(networkType)
}

func (a *moduleGenesisAdapter) GetSnapshotURL(networkType string) string {
	return a.module.SnapshotURL(networkType)
}

func (a *moduleGenesisAdapter) BinaryName() string {
	return a.module.BinaryName()
}

func (a *moduleGenesisAdapter) ValidateGenesis(genesis []byte) error {
	// Basic validation: check if genesis is valid JSON with required fields
	var gen struct {
		ChainID     string          `json:"chain_id"`
		GenesisTime string          `json:"genesis_time"`
		AppState    json.RawMessage `json:"app_state"`
	}
	if err := json.Unmarshal(genesis, &gen); err != nil {
		return fmt.Errorf("invalid genesis JSON: %w", err)
	}
	if gen.ChainID == "" {
		return fmt.Errorf("genesis missing chain_id")
	}
	return nil
}

func (a *moduleGenesisAdapter) PatchGenesis(genesis []byte, opts plugintypes.GenesisPatchOptions) ([]byte, error) {
	// Apply patches via the module's ModifyGenesis
	modifyOpts := network.GenesisOptions{
		ChainID: opts.ChainID,
	}
	return a.module.ModifyGenesis(genesis, modifyOpts)
}

func (a *moduleGenesisAdapter) PatchGenesisFile(inputPath, outputPath string, opts plugintypes.GenesisPatchOptions) (int64, error) {
	// Check if module supports file-based modification
	fileModifier, ok := a.module.(network.FileBasedGenesisModifier)
	if !ok {
		return 0, fmt.Errorf("plugin does not support file-based genesis modification")
	}

	// Use file-based modification to bypass gRPC message size limits
	modifyOpts := network.GenesisOptions{
		ChainID: opts.ChainID,
	}
	return fileModifier.ModifyGenesisFile(inputPath, outputPath, modifyOpts)
}

func (a *moduleGenesisAdapter) ExportCommandArgs(homeDir string) []string {
	return a.module.ExportCommand(homeDir)
}

// Ensure interface compliance
var _ plugintypes.PluginGenesis = (*moduleGenesisAdapter)(nil)
var _ plugintypes.FileBasedPluginGenesis = (*moduleGenesisAdapter)(nil)

// moduleInitializerAdapter adapts NetworkModule to plugintypes.PluginInitializer.
type moduleInitializerAdapter struct {
	module network.NetworkModule
}

func (a *moduleInitializerAdapter) BinaryName() string {
	return a.module.BinaryName()
}

func (a *moduleInitializerAdapter) DefaultChainID() string {
	return a.module.DefaultChainID()
}

func (a *moduleInitializerAdapter) DefaultMoniker(index int) string {
	return a.module.DefaultMoniker(index)
}

func (a *moduleInitializerAdapter) InitCommandArgs(homeDir, moniker, chainID string) []string {
	cmd := a.module.InitCommand(homeDir, chainID, moniker)
	// The InitCommand returns full args
	if len(cmd) > 0 {
		return cmd
	}
	return nil
}

func (a *moduleInitializerAdapter) ConfigDir(homeDir string) string {
	return a.module.ConfigDir(homeDir)
}

func (a *moduleInitializerAdapter) DataDir(homeDir string) string {
	return a.module.DataDir(homeDir)
}

func (a *moduleInitializerAdapter) KeyringDir(homeDir string) string {
	return a.module.KeyringDir(homeDir, "test")
}

// Ensure interface compliance
var _ plugintypes.PluginInitializer = (*moduleInitializerAdapter)(nil)

// moduleRuntimeAdapter adapts NetworkModule to runtime.PluginRuntime.
type moduleRuntimeAdapter struct {
	module network.NetworkModule
}

func (a *moduleRuntimeAdapter) StartCommand(node *daemontypes.Node) []string {
	// Get home directory from node spec
	homeDir := node.Spec.HomeDir
	if homeDir == "" {
		homeDir = a.module.DefaultNodeHome()
	}

	// For daemon-managed nodes, we pass empty networkMode to get the base start command.
	// The networkMode parameter ("mainnet"/"testnet") is used by plugins to select
	// pre-configured chain-ids for public networks. However, devnets have custom
	// chain-ids (e.g., "mydevnet-1") that are defined at provisioning time and stored
	// in NodeSpec.ChainID. We append this chain-id separately below.
	args := a.module.StartCommand(homeDir, "")

	// Append chain-id from node spec. This is required for Cosmos SDK nodes to start.
	// The ChainID is copied from DevnetSpec during node creation (see provisioner).
	if node.Spec.ChainID != "" {
		args = append(args, "--chain-id", node.Spec.ChainID)
	}

	return args
}

func (a *moduleRuntimeAdapter) StartEnv(node *daemontypes.Node) map[string]string {
	// Return empty env map - can be extended per-network if needed
	return nil
}

func (a *moduleRuntimeAdapter) StopSignal() syscall.Signal {
	// SIGTERM is standard for graceful shutdown
	return syscall.SIGTERM
}

func (a *moduleRuntimeAdapter) GracePeriod() time.Duration {
	// 10 seconds is reasonable for Cosmos SDK nodes
	return 10 * time.Second
}

func (a *moduleRuntimeAdapter) HealthEndpoint(node *daemontypes.Node) string {
	// Standard Cosmos SDK health endpoint
	ports := a.module.DefaultPorts()
	return fmt.Sprintf("http://localhost:%d/health", ports.RPC)
}

func (a *moduleRuntimeAdapter) ContainerHomePath() string {
	return a.module.DockerHomeDir()
}

// Ensure interface compliance
var _ runtime.PluginRuntime = (*moduleRuntimeAdapter)(nil)

// =============================================================================
// Node Initializer Adapter (ports.NodeInitializer)
// =============================================================================

// nodeInitializerAdapter implements ports.NodeInitializer and provisioner.BinaryPathUpdater.
// This adapter bridges the NetworkModule interface to the port interface expected by
// the orchestrator. It supports deferred binary path injection since the
// daemon creates the orchestrator before the binary is built.
type nodeInitializerAdapter struct {
	module     network.NetworkModule
	binaryPath string // Full path to binary (set after build via SetBinaryPath)
	logger     *slog.Logger
	mu         sync.RWMutex // Protects binaryPath
}

// Ensure nodeInitializerAdapter implements ports.NodeInitializer
var _ ports.NodeInitializer = (*nodeInitializerAdapter)(nil)

// Ensure nodeInitializerAdapter implements provisioner.BinaryPathUpdater
var _ provisioner.BinaryPathUpdater = (*nodeInitializerAdapter)(nil)

// ensureOverwriteFlag ensures the --overwrite flag is present in init command args.
// This is a defensive measure to handle cases where plugins don't include the flag,
// which causes init to fail when genesis.json already exists (e.g., from a previous
// provisioning attempt or leftover state).
func ensureOverwriteFlag(args []string) []string {
	for _, arg := range args {
		// -o is the standard Cosmos SDK short form for --overwrite in init commands
		if arg == "--overwrite" || arg == "-o" {
			return args // Already has the flag
		}
	}
	return append(args, "--overwrite")
}

// newNodeInitializerAdapter creates an adapter implementing ports.NodeInitializer.
// The binaryPath is not set at construction time; call SetBinaryPath after build.
func newNodeInitializerAdapter(module network.NetworkModule, logger *slog.Logger) *nodeInitializerAdapter {
	return &nodeInitializerAdapter{
		module: module,
		logger: logger,
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

	// Get init command args from module
	args := a.module.InitCommand(nodeDir, chainID, moniker)

	// Ensure --overwrite flag is present to handle existing genesis.json files
	// This is critical for re-provisioning devnets or when leftover state exists
	args = ensureOverwriteFlag(args)

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
// It uses the global network registry to obtain NetworkModules from loaded plugins.
type OrchestratorFactory struct {
	dataDir string
	logger  *slog.Logger
}

// NewOrchestratorFactory creates a new orchestrator factory.
func NewOrchestratorFactory(dataDir string, logger *slog.Logger) *OrchestratorFactory {
	return &OrchestratorFactory{
		dataDir: dataDir,
		logger:  logger,
	}
}

// GetBuilder implements builder.PluginLoader interface.
func (f *OrchestratorFactory) GetBuilder(pluginName string) (plugintypes.PluginBuilder, error) {
	module, err := network.Get(pluginName)
	if err != nil {
		return nil, err
	}
	return &moduleBuilderAdapter{module: module}, nil
}

// GetPluginRuntime returns the PluginRuntime for a network.
func (f *OrchestratorFactory) GetPluginRuntime(pluginName string) (runtime.PluginRuntime, error) {
	module, err := network.Get(pluginName)
	if err != nil {
		return nil, err
	}
	return &moduleRuntimeAdapter{module: module}, nil
}

// CreateOrchestrator creates an Orchestrator for the given network.
// In daemon mode, the orchestrator is configured to skip the start phase
// (SkipStart=true in ProvisionOptions), so NodeRuntime is not needed.
// Returns provisioner.Orchestrator interface for testability.
func (f *OrchestratorFactory) CreateOrchestrator(networkName string) (provisioner.Orchestrator, error) {
	module, err := network.Get(networkName)
	if err != nil {
		return nil, err
	}

	// Create adapters for the old plugin interfaces
	genesisAdapter := &moduleGenesisAdapter{module: module}

	// Create binary builder
	binaryBuilder := builder.NewDefaultBuilder(f.dataDir, f, f.logger)

	// Create infrastructure services for snapshot-based genesis forking
	snapshotFetcher := snapshot.NewFetcherAdapter(f.dataDir, nil)
	stateExportSvc := stateexport.NewAdapter(f.dataDir, nil)
	genesisFetcher := genesis.NewFetcherAdapter(f.dataDir, "", "", false, nil)

	// Create genesis forker with infrastructure for all fork modes
	genesisForker := provisioner.NewGenesisForker(provisioner.GenesisForkerConfig{
		DataDir:            f.dataDir,
		PluginGenesis:      genesisAdapter,
		GenesisFetcher:     genesisFetcher,
		SnapshotFetcher:    snapshotFetcher,
		StateExportService: stateExportSvc,
		Logger:             f.logger,
	})

	// Create node initializer adapter
	nodeInitializer := newNodeInitializerAdapter(module, f.logger)

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

// ListAvailableNetworks returns the names of all registered networks.
func (f *OrchestratorFactory) ListAvailableNetworks() []string {
	return network.List()
}

// GetNetworkDefaults returns default URLs for a network/plugin.
// Implements provisioner.OrchestratorFactory interface.
func (f *OrchestratorFactory) GetNetworkDefaults(pluginName, networkType string) (*provisioner.NetworkDefaults, error) {
	module, err := network.Get(pluginName)
	if err != nil {
		return nil, err
	}

	return &provisioner.NetworkDefaults{
		RPCURL:            module.RPCEndpoint(networkType),
		SnapshotURL:       module.SnapshotURL(networkType),
		AvailableNetworks: module.AvailableNetworks(),
	}, nil
}
