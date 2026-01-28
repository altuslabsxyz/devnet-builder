// internal/daemon/nodeinit/service.go
package nodeinit

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/altuslabsxyz/devnet-builder/internal/infrastructure/network"
)

// =============================================================================
// Node Init Options
// =============================================================================

// NodeInitOptions contains options for initializing a node.
type NodeInitOptions struct {
	// HomeDir is the node's home directory where data will be stored.
	HomeDir string

	// Moniker is the node's human-readable name.
	// If empty, uses module.DefaultMoniker(NodeIndex).
	Moniker string

	// ChainID is the chain identifier.
	// If empty, uses module.DefaultChainID().
	ChainID string

	// NodeIndex is the index of this node (0, 1, 2, ...).
	// Used for generating default monikers and ports.
	NodeIndex int

	// BinaryPath is the path to the chain binary.
	BinaryPath string

	// IsValidator indicates if this node will be a validator.
	IsValidator bool
}

// Validate validates the NodeInitOptions.
func (o *NodeInitOptions) Validate() error {
	if o.HomeDir == "" {
		return fmt.Errorf("home directory is required")
	}
	if o.BinaryPath == "" {
		return fmt.Errorf("binary path is required")
	}
	return nil
}

// =============================================================================
// Node Init Service
// =============================================================================

// NodeInitService handles node initialization using NetworkModule for configuration.
// This service is GENERIC - it works with ANY network via the NetworkModule interface.
// The NetworkModule provides all chain-specific configuration (init command, config paths,
// default monikers), while this service provides the actual execution behavior.
type NodeInitService struct {
	dataDir string
	logger  *slog.Logger
}

// NodeInitServiceConfig configures the NodeInitService.
type NodeInitServiceConfig struct {
	// DataDir is the base directory for node data.
	DataDir string

	// Logger for logging progress.
	Logger *slog.Logger
}

// NewNodeInitService creates a new NodeInitService.
func NewNodeInitService(config NodeInitServiceConfig) *NodeInitService {
	logger := config.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &NodeInitService{
		dataDir: config.DataDir,
		logger:  logger,
	}
}

// Initialize initializes a node using configuration from the NetworkModule.
// The module provides:
//   - InitCommand(homeDir, chainID, moniker) - command arguments for initialization
//   - DefaultMoniker(index) - default moniker for node at index
//   - DefaultChainID() - default chain ID for devnets
//   - ConfigDir(homeDir) - path to config directory
//
// Returns an error if initialization fails.
func (s *NodeInitService) Initialize(ctx context.Context, module network.NetworkModule, opts NodeInitOptions) error {
	// Validate options
	if err := opts.Validate(); err != nil {
		return fmt.Errorf("invalid node options: %w", err)
	}

	// Determine moniker (use module default if not specified)
	moniker := opts.Moniker
	if moniker == "" {
		moniker = module.DefaultMoniker(opts.NodeIndex)
		s.logger.Debug("using default moniker", "moniker", moniker, "index", opts.NodeIndex)
	}

	// Determine chain ID (use module default if not specified)
	chainID := opts.ChainID
	if chainID == "" {
		chainID = module.DefaultChainID()
		s.logger.Debug("using default chain ID", "chainID", chainID)
	}

	s.logger.Info("initializing node",
		"network", module.Name(),
		"homeDir", opts.HomeDir,
		"moniker", moniker,
		"chainID", chainID,
		"nodeIndex", opts.NodeIndex,
	)

	// Create home directory
	if err := os.MkdirAll(opts.HomeDir, 0755); err != nil {
		return fmt.Errorf("failed to create home directory: %w", err)
	}

	// Get init command from module
	args := module.InitCommand(opts.HomeDir, chainID, moniker)

	s.logger.Debug("running init command",
		"binary", opts.BinaryPath,
		"args", args,
	)

	// Run init command
	cmd := exec.CommandContext(ctx, opts.BinaryPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		s.logger.Error("node initialization failed",
			"output", string(output),
			"error", err,
		)
		return fmt.Errorf("node init failed: %w: %s", err, string(output))
	}

	// Fix permissions for Docker compatibility
	if err := s.fixConfigPermissions(module, opts.HomeDir); err != nil {
		s.logger.Warn("failed to fix config permissions", "error", err)
	}

	s.logger.Info("node initialization completed",
		"homeDir", opts.HomeDir,
		"moniker", moniker,
	)

	return nil
}

// InitializeMultiple initializes multiple nodes sequentially.
func (s *NodeInitService) InitializeMultiple(ctx context.Context, module network.NetworkModule, optsList []NodeInitOptions) error {
	if len(optsList) == 0 {
		return fmt.Errorf("no node options provided")
	}

	s.logger.Info("initializing multiple nodes",
		"network", module.Name(),
		"count", len(optsList),
	)

	for i, opts := range optsList {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("context cancelled during initialization: %w", err)
		}

		if err := s.Initialize(ctx, module, opts); err != nil {
			return fmt.Errorf("failed to initialize node %d: %w", i, err)
		}
	}

	s.logger.Info("all nodes initialized successfully", "count", len(optsList))
	return nil
}

// =============================================================================
// Node ID Methods
// =============================================================================

// GetNodeID retrieves the node ID from an initialized node's home directory.
// The node ID is derived from the node key's public key.
func (s *NodeInitService) GetNodeID(ctx context.Context, module network.NetworkModule, homeDir string) (string, error) {
	s.logger.Debug("getting node ID", "homeDir", homeDir)

	// Get config directory from module
	configDir := module.ConfigDir(homeDir)

	// Read node_key.json
	nodeKeyPath := filepath.Join(configDir, "node_key.json")
	data, err := os.ReadFile(nodeKeyPath)
	if err != nil {
		return "", fmt.Errorf("failed to read node_key.json: %w", err)
	}

	var nodeKey struct {
		PrivKey struct {
			Type  string `json:"type"`
			Value string `json:"value"`
		} `json:"priv_key"`
	}

	if err := json.Unmarshal(data, &nodeKey); err != nil {
		return "", fmt.Errorf("failed to parse node_key.json: %w", err)
	}

	// Decode base64 private key
	privKeyBytes, err := base64.StdEncoding.DecodeString(nodeKey.PrivKey.Value)
	if err != nil {
		return "", fmt.Errorf("failed to decode private key: %w", err)
	}

	// Create ed25519 private key and derive public key
	privKey := ed25519.PrivateKey(privKeyBytes)
	pubKey := privKey.Public().(ed25519.PublicKey)

	// Node ID is the hex-encoded address (first 20 bytes of SHA256 of pubkey)
	// This matches CometBFT's address derivation
	hash := sha256.Sum256(pubKey)
	nodeID := strings.ToLower(fmt.Sprintf("%x", hash[:20]))

	return nodeID, nil
}

// GetAllNodeIDs retrieves node IDs for all nodes in the specified directories.
func (s *NodeInitService) GetAllNodeIDs(ctx context.Context, module network.NetworkModule, homeDirs []string) ([]string, error) {
	nodeIDs := make([]string, len(homeDirs))

	for i, homeDir := range homeDirs {
		nodeID, err := s.GetNodeID(ctx, module, homeDir)
		if err != nil {
			return nil, fmt.Errorf("failed to get node ID for %s: %w", homeDir, err)
		}
		nodeIDs[i] = nodeID
	}

	return nodeIDs, nil
}

// =============================================================================
// Peer Configuration Methods
// =============================================================================

// BuildPersistentPeers builds the persistent_peers string from node IDs and ports.
// Each peer address is in format: nodeID@host:port
func (s *NodeInitService) BuildPersistentPeers(module network.NetworkModule, nodeIDs []string, basePort int, host string) string {
	if host == "" {
		host = "127.0.0.1"
	}

	defaultPorts := module.DefaultPorts()
	p2pPort := defaultPorts.P2P

	var peers []string
	for i, nodeID := range nodeIDs {
		// Calculate port: base + (index * 10000) + p2p offset
		port := basePort + (i * 10000) + p2pPort
		peer := fmt.Sprintf("%s@%s:%d", nodeID, host, port)
		peers = append(peers, peer)
	}

	return strings.Join(peers, ",")
}

// BuildPersistentPeersWithExclusion builds persistent_peers excluding the specified node index.
// This is useful for generating peer lists where a node should not include itself.
func (s *NodeInitService) BuildPersistentPeersWithExclusion(module network.NetworkModule, nodeIDs []string, basePort int, host string, excludeIndex int) string {
	if host == "" {
		host = "127.0.0.1"
	}

	defaultPorts := module.DefaultPorts()
	p2pPort := defaultPorts.P2P

	var peers []string
	for i, nodeID := range nodeIDs {
		if i == excludeIndex {
			continue
		}
		port := basePort + (i * 10000) + p2pPort
		peer := fmt.Sprintf("%s@%s:%d", nodeID, host, port)
		peers = append(peers, peer)
	}

	return strings.Join(peers, ",")
}

// =============================================================================
// Configuration Methods
// =============================================================================

// WriteGenesis writes genesis JSON to the node's config directory.
func (s *NodeInitService) WriteGenesis(module network.NetworkModule, homeDir string, genesis []byte) error {
	configDir := module.ConfigDir(homeDir)
	genesisPath := filepath.Join(configDir, "genesis.json")

	s.logger.Debug("writing genesis", "path", genesisPath)

	if err := os.WriteFile(genesisPath, genesis, 0644); err != nil {
		return fmt.Errorf("failed to write genesis: %w", err)
	}

	return nil
}

// ApplyConfigOverrides applies configuration overrides from the module.
func (s *NodeInitService) ApplyConfigOverrides(ctx context.Context, module network.NetworkModule, homeDir string, nodeIndex int, configOpts network.NodeConfigOptions) error {
	s.logger.Debug("applying config overrides",
		"homeDir", homeDir,
		"nodeIndex", nodeIndex,
	)

	// Get config overrides from module
	configToml, appToml, err := module.GetConfigOverrides(nodeIndex, configOpts)
	if err != nil {
		return fmt.Errorf("failed to get config overrides: %w", err)
	}

	configDir := module.ConfigDir(homeDir)

	// Apply config.toml overrides
	if len(configToml) > 0 {
		configPath := filepath.Join(configDir, "config.toml")
		if err := s.mergeTomlConfig(configPath, configToml); err != nil {
			return fmt.Errorf("failed to apply config.toml overrides: %w", err)
		}
	}

	// Apply app.toml overrides
	if len(appToml) > 0 {
		appPath := filepath.Join(configDir, "app.toml")
		if err := s.mergeTomlConfig(appPath, appToml); err != nil {
			return fmt.Errorf("failed to apply app.toml overrides: %w", err)
		}
	}

	return nil
}

// =============================================================================
// Internal Methods
// =============================================================================

// fixConfigPermissions ensures config files are readable by Docker containers.
func (s *NodeInitService) fixConfigPermissions(module network.NetworkModule, homeDir string) error {
	configDir := module.ConfigDir(homeDir)

	// Files that need to be readable by Docker containers
	files := []string{
		"client.toml",
		"config.toml",
		"app.toml",
		"genesis.json",
		"node_key.json",
		"priv_validator_key.json",
	}

	for _, file := range files {
		path := filepath.Join(configDir, file)
		if _, err := os.Stat(path); err == nil {
			// Make file readable (0644)
			if err := os.Chmod(path, 0644); err != nil {
				return fmt.Errorf("failed to chmod %s: %w", file, err)
			}
		}
	}

	return nil
}

// mergeTomlConfig merges override TOML into an existing config file.
// This is a simple append-based approach - for full merge, consider using a TOML library.
func (s *NodeInitService) mergeTomlConfig(configPath string, overrides []byte) error {
	// Read existing config
	existing, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read existing config: %w", err)
	}

	// For now, we'll append the overrides with a clear separator.
	// A proper implementation would parse and merge TOML.
	merged := append(existing, []byte("\n# --- Module Config Overrides ---\n")...)
	merged = append(merged, overrides...)

	// Write back
	if err := os.WriteFile(configPath, merged, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// =============================================================================
// Additional Node Operations
// =============================================================================

// CopyValidatorKey copies the priv_validator_key.json from source to destination node.
// This is useful for setting up multi-validator devnets.
func (s *NodeInitService) CopyValidatorKey(module network.NetworkModule, srcHomeDir, dstHomeDir string) error {
	srcConfigDir := module.ConfigDir(srcHomeDir)
	dstConfigDir := module.ConfigDir(dstHomeDir)

	srcPath := filepath.Join(srcConfigDir, "priv_validator_key.json")
	dstPath := filepath.Join(dstConfigDir, "priv_validator_key.json")

	data, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("failed to read source validator key: %w", err)
	}

	if err := os.WriteFile(dstPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write destination validator key: %w", err)
	}

	return nil
}

// ResetNodeState resets the node's data directory while preserving configuration.
func (s *NodeInitService) ResetNodeState(module network.NetworkModule, homeDir string) error {
	dataDir := module.DataDir(homeDir)

	s.logger.Debug("resetting node state", "dataDir", dataDir)

	// Remove data directory contents but not the directory itself
	entries, err := os.ReadDir(dataDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Nothing to reset
		}
		return fmt.Errorf("failed to read data directory: %w", err)
	}

	for _, entry := range entries {
		path := filepath.Join(dataDir, entry.Name())
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("failed to remove %s: %w", entry.Name(), err)
		}
	}

	return nil
}
