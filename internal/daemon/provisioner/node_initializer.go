// internal/daemon/provisioner/node_initializer.go
package provisioner

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

	"github.com/altuslabsxyz/devnet-builder/internal/application/ports"
	"github.com/altuslabsxyz/devnet-builder/internal/plugin/types"
)

// NodeInitializerConfig configures the node initializer
type NodeInitializerConfig struct {
	DataDir    string
	BinaryPath string
	PluginInit types.PluginInitializer
	InfraInit  ports.NodeInitializer // optional: existing infrastructure
	Logger     *slog.Logger
}

// NodeInitializer handles node initialization using plugin interface for customization
type NodeInitializer struct {
	config NodeInitializerConfig
	logger *slog.Logger
}

// NewNodeInitializer creates a new node initializer
func NewNodeInitializer(config NodeInitializerConfig) *NodeInitializer {
	logger := config.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &NodeInitializer{
		config: config,
		logger: logger,
	}
}

// InitializeNode initializes a single node's home directory.
// It uses the plugin initializer for network-specific commands, or falls back to
// the infrastructure port if available.
func (n *NodeInitializer) InitializeNode(ctx context.Context, nodeConfig types.NodeInitConfig) error {
	// Validate config
	if err := nodeConfig.Validate(); err != nil {
		return fmt.Errorf("invalid node config: %w", err)
	}

	n.logger.Info("initializing node",
		"homeDir", nodeConfig.HomeDir,
		"moniker", nodeConfig.Moniker,
		"chainID", nodeConfig.ChainID,
		"validatorIndex", nodeConfig.ValidatorIndex,
	)

	// Use existing infrastructure if available
	if n.config.InfraInit != nil {
		return n.config.InfraInit.Initialize(ctx, nodeConfig.HomeDir, nodeConfig.Moniker, nodeConfig.ChainID)
	}

	// Fallback: direct initialization using plugin interface
	return n.initializeNodeDirect(ctx, nodeConfig)
}

// initializeNodeDirect initializes a node without infrastructure port
func (n *NodeInitializer) initializeNodeDirect(ctx context.Context, nodeConfig types.NodeInitConfig) error {
	// Determine binary path
	binaryPath := n.config.BinaryPath
	if binaryPath == "" && n.config.PluginInit != nil {
		binaryPath = n.config.PluginInit.BinaryName()
	}
	if binaryPath == "" {
		return fmt.Errorf("binary path not specified and no plugin initializer available")
	}

	// Get init command args from plugin
	var args []string
	if n.config.PluginInit != nil {
		args = n.config.PluginInit.InitCommandArgs(nodeConfig.HomeDir, nodeConfig.Moniker, nodeConfig.ChainID)
	} else {
		// Default Cosmos SDK style args
		args = []string{
			"init", nodeConfig.Moniker,
			"--chain-id", nodeConfig.ChainID,
			"--home", nodeConfig.HomeDir,
			"--overwrite",
		}
	}

	// Run the init command
	cmd := exec.CommandContext(ctx, binaryPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		n.logger.Error("node initialization failed",
			"binary", binaryPath,
			"args", args,
			"output", string(output),
			"error", err,
		)
		return fmt.Errorf("node init failed: %w", err)
	}

	// Fix permissions for Docker compatibility
	if err := n.fixConfigPermissions(nodeConfig.HomeDir); err != nil {
		n.logger.Warn("failed to fix config permissions", "error", err)
	}

	return nil
}

// fixConfigPermissions ensures config files are readable by Docker containers.
func (n *NodeInitializer) fixConfigPermissions(homeDir string) error {
	var configDir string
	if n.config.PluginInit != nil {
		configDir = n.config.PluginInit.ConfigDir(homeDir)
	} else {
		configDir = filepath.Join(homeDir, "config")
	}

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

// InitializeMultipleNodes initializes multiple nodes.
// This orchestrates initialization of multiple nodes in sequence.
func (n *NodeInitializer) InitializeMultipleNodes(ctx context.Context, configs []types.NodeInitConfig) error {
	if len(configs) == 0 {
		return fmt.Errorf("no node configs provided")
	}

	n.logger.Info("initializing multiple nodes", "count", len(configs))

	for i, config := range configs {
		n.logger.Debug("initializing node",
			"index", i,
			"moniker", config.Moniker,
		)

		if err := n.InitializeNode(ctx, config); err != nil {
			return fmt.Errorf("failed to initialize node %d (%s): %w", i, config.Moniker, err)
		}
	}

	n.logger.Info("all nodes initialized successfully", "count", len(configs))
	return nil
}

// GetNodeID retrieves the node ID from an initialized node's home directory.
// It reads the node_key.json file and derives the node ID from the public key.
func (n *NodeInitializer) GetNodeID(ctx context.Context, homeDir string) (string, error) {
	n.logger.Debug("getting node ID", "homeDir", homeDir)

	// Use existing infrastructure if available
	if n.config.InfraInit != nil {
		return n.config.InfraInit.GetNodeID(ctx, homeDir)
	}

	// Fallback: read node ID directly from file
	return n.getNodeIDFromFile(homeDir)
}

// getNodeIDFromFile reads the node ID from the node_key.json file.
// The node ID is the hex-encoded address of the public key derived from the private key.
func (n *NodeInitializer) getNodeIDFromFile(homeDir string) (string, error) {
	var configDir string
	if n.config.PluginInit != nil {
		configDir = n.config.PluginInit.ConfigDir(homeDir)
	} else {
		configDir = filepath.Join(homeDir, "config")
	}

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
	// Go's ed25519.PrivateKey is 64 bytes (seed + public key)
	privKey := ed25519.PrivateKey(privKeyBytes)
	pubKey := privKey.Public().(ed25519.PublicKey)

	// Node ID is the hex-encoded address (first 20 bytes of SHA256 of pubkey)
	// This matches CometBFT's address derivation
	hash := sha256.Sum256(pubKey)
	nodeID := strings.ToLower(fmt.Sprintf("%x", hash[:20]))

	return nodeID, nil
}

// BuildPersistentPeers builds the persistent_peers string from node IDs and base port.
// It generates peer addresses in the format: nodeID@127.0.0.1:port
// Each subsequent node uses basePort + (index * 10000) for port assignment.
func (n *NodeInitializer) BuildPersistentPeers(nodeIDs []string, basePort int) string {
	var peers []string
	for i, nodeID := range nodeIDs {
		port := basePort + (i * 10000)
		peer := fmt.Sprintf("%s@127.0.0.1:%d", nodeID, port)
		peers = append(peers, peer)
	}
	return strings.Join(peers, ",")
}

// BuildPersistentPeersWithExclusion builds persistent_peers excluding the specified node index.
// This is useful for generating peer lists where a node should not include itself.
func (n *NodeInitializer) BuildPersistentPeersWithExclusion(nodeIDs []string, basePort int, excludeIndex int) string {
	var peers []string
	for i, nodeID := range nodeIDs {
		if i == excludeIndex {
			continue
		}
		port := basePort + (i * 10000)
		peer := fmt.Sprintf("%s@127.0.0.1:%d", nodeID, port)
		peers = append(peers, peer)
	}
	return strings.Join(peers, ",")
}

// GetAllNodeIDs retrieves node IDs for all initialized nodes in the specified directories.
// This is a convenience method for getting all node IDs from a devnet setup.
func (n *NodeInitializer) GetAllNodeIDs(ctx context.Context, homeDirs []string) ([]string, error) {
	nodeIDs := make([]string, len(homeDirs))

	for i, homeDir := range homeDirs {
		nodeID, err := n.GetNodeID(ctx, homeDir)
		if err != nil {
			return nil, fmt.Errorf("failed to get node ID for %s: %w", homeDir, err)
		}
		nodeIDs[i] = nodeID
	}

	return nodeIDs, nil
}
