package nodeconfig

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cometbft/cometbft/crypto/ed25519"
	"github.com/stablelabs/stable-devnet/internal/output"
)

// isGHCRImage returns true if the image is from GitHub Container Registry.
// GHCR images have stabled as entrypoint, so we don't need to prefix commands.
func isGHCRImage(image string) bool {
	return strings.HasPrefix(image, "ghcr.io/")
}

// ExecutionMode defines how nodes are executed.
type ExecutionMode string

const (
	ModeDocker ExecutionMode = "docker"
	ModeLocal  ExecutionMode = "local"
)

// NodeInitializer handles node initialization with stabled.
type NodeInitializer struct {
	mode        ExecutionMode
	dockerImage string
	logger      *output.Logger
}

// NewNodeInitializer creates a new NodeInitializer.
func NewNodeInitializer(mode ExecutionMode, dockerImage string, logger *output.Logger) *NodeInitializer {
	if logger == nil {
		logger = output.DefaultLogger
	}
	return &NodeInitializer{
		mode:        mode,
		dockerImage: dockerImage,
		logger:      logger,
	}
}

// Initialize runs `stabled init` for a node.
// Note: Always uses local stabled binary for init because Docker images
// may have issues with init command requiring pre-existing config files.
func (i *NodeInitializer) Initialize(ctx context.Context, nodeDir, moniker, chainID string) error {
	i.logger.Debug("Initializing node %s at %s", moniker, nodeDir)

	// Always use local init - Docker GHCR images have issues with init command
	// that expects client.toml to already exist
	return i.initLocal(ctx, nodeDir, moniker, chainID)
}

func (i *NodeInitializer) initDocker(ctx context.Context, nodeDir, moniker, chainID string) error {
	args := []string{
		"run", "--rm",
		"-v", fmt.Sprintf("%s:/root/.stabled", nodeDir),
		i.dockerImage,
	}
	// GHCR images have stabled as entrypoint, others need explicit command
	if !isGHCRImage(i.dockerImage) {
		args = append(args, "stabled")
	}
	args = append(args, "init", moniker,
		"--chain-id", chainID,
		"--home", "/root/.stabled",
	)

	cmd := exec.CommandContext(ctx, "docker", args...)
	cmdOutput, err := cmd.CombinedOutput()
	if err != nil {
		// Print detailed error for diagnosis
		i.logger.PrintCommandError(&output.CommandErrorInfo{
			Command:  "docker",
			Args:     args,
			WorkDir:  nodeDir,
			Stderr:   string(cmdOutput),
			ExitCode: getExitCode(err),
			Error:    err,
		})
		return fmt.Errorf("docker init failed: %w", err)
	}
	return nil
}

func (i *NodeInitializer) initLocal(ctx context.Context, nodeDir, moniker, chainID string) error {
	// Use --overwrite to handle existing genesis.json files
	args := []string{"init", moniker, "--chain-id", chainID, "--home", nodeDir, "--overwrite"}
	cmd := exec.CommandContext(ctx, "stabled", args...)
	cmdOutput, err := cmd.CombinedOutput()
	if err != nil {
		// Print detailed error for diagnosis
		i.logger.PrintCommandError(&output.CommandErrorInfo{
			Command:  "stabled",
			Args:     args,
			WorkDir:  nodeDir,
			Stderr:   string(cmdOutput),
			ExitCode: getExitCode(err),
			Error:    err,
		})
		return fmt.Errorf("stabled init failed: %w", err)
	}

	// Fix permissions for Docker compatibility
	// stabled init creates some files with 0600, but Docker needs 0644 to read them
	if err := fixConfigPermissions(nodeDir); err != nil {
		i.logger.Debug("Warning: failed to fix config permissions: %v", err)
	}

	return nil
}

// fixConfigPermissions ensures config files are readable by Docker containers.
// stabled init creates client.toml and other files with 0600 permissions,
// but Docker containers running as different users need read access.
func fixConfigPermissions(nodeDir string) error {
	configDir := filepath.Join(nodeDir, "config")

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

// GetNodeID retrieves the node ID from the node_key.json file.
// This reads the node key directly without requiring stabled binary or docker.
func (i *NodeInitializer) GetNodeID(ctx context.Context, nodeDir string) (string, error) {
	i.logger.Debug("Getting node ID for %s", nodeDir)

	// Read node_key.json directly
	nodeKeyPath := filepath.Join(nodeDir, "config", "node_key.json")
	return readNodeIDFromFile(nodeKeyPath)
}

// readNodeIDFromFile reads the node ID from a node_key.json file.
// The node ID is the hex-encoded address of the public key derived from the private key.
func readNodeIDFromFile(nodeKeyPath string) (string, error) {
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

	// Create ed25519 private key and get public key address
	privKey := ed25519.PrivKey(privKeyBytes)
	pubKey := privKey.PubKey()

	// Node ID is the hex-encoded address (first 20 bytes of SHA256 of pubkey)
	// Note: CometBFT's Address type returns uppercase hex, but stabled uses lowercase
	nodeID := strings.ToLower(fmt.Sprintf("%x", pubKey.Address()))

	return nodeID, nil
}

// Export runs `stabled export` to export the current state as genesis.
func (i *NodeInitializer) Export(ctx context.Context, nodeDir, destPath string) error {
	i.logger.Debug("Exporting genesis from %s", nodeDir)

	if i.mode == ModeDocker {
		return i.exportDocker(ctx, nodeDir, destPath)
	}
	return i.exportLocal(ctx, nodeDir, destPath)
}

func (i *NodeInitializer) exportDocker(ctx context.Context, nodeDir, destPath string) error {
	args := []string{
		"run", "--rm",
		"-v", fmt.Sprintf("%s:/root/.stabled", nodeDir),
		"-v", fmt.Sprintf("%s:/output", destPath),
	}
	// GHCR images have stabled as entrypoint, need to override it for bash
	if isGHCRImage(i.dockerImage) {
		args = append(args, "--entrypoint", "bash", i.dockerImage, "-c",
			"stabled export --home /root/.stabled > /output/genesis.json")
	} else {
		args = append(args, i.dockerImage, "bash", "-c",
			"stabled export --home /root/.stabled > /output/genesis.json")
	}

	cmd := exec.CommandContext(ctx, "docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker export failed: %s: %w", string(output), err)
	}
	return nil
}

func (i *NodeInitializer) exportLocal(ctx context.Context, nodeDir, destPath string) error {
	cmd := exec.CommandContext(ctx, "bash", "-c",
		fmt.Sprintf("stabled export --home %s > %s", nodeDir, destPath),
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("stabled export failed: %s: %w", string(output), err)
	}
	return nil
}

// BuildPersistentPeers builds the persistent_peers string from node IDs and ports.
func BuildPersistentPeers(nodeIDs []string, baseP2PPort int) string {
	var peers []string
	for i, nodeID := range nodeIDs {
		port := baseP2PPort + (i * 10000)
		peer := fmt.Sprintf("%s@127.0.0.1:%d", nodeID, port)
		peers = append(peers, peer)
	}
	return strings.Join(peers, ",")
}

// BuildPersistentPeersWithExclusion builds persistent_peers excluding the specified node index.
func BuildPersistentPeersWithExclusion(nodeIDs []string, baseP2PPort int, excludeIndex int) string {
	var peers []string
	for i, nodeID := range nodeIDs {
		if i == excludeIndex {
			continue
		}
		port := baseP2PPort + (i * 10000)
		peer := fmt.Sprintf("%s@127.0.0.1:%d", nodeID, port)
		peers = append(peers, peer)
	}
	return strings.Join(peers, ",")
}

// getExitCode extracts the exit code from an error.
func getExitCode(err error) int {
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode()
	}
	return -1
}
