package nodeconfig

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/b-harvest/devnet-builder/internal/application/ports"
	"github.com/b-harvest/devnet-builder/internal/output"
	"github.com/b-harvest/devnet-builder/types"
)

// isGHCRImage returns true if the image is from GitHub Container Registry.
// GHCR images have stabled as entrypoint, so we don't need to prefix commands.
func isGHCRImage(image string) bool {
	return strings.HasPrefix(image, "ghcr.io/")
}

// NodeInitializer handles node initialization with stabled.
type NodeInitializer struct {
	mode        types.ExecutionMode
	dockerImage string
	binaryPath  string // Path to local stabled binary (used for local mode)
	logger      *output.Logger
}

// NewNodeInitializer creates a new NodeInitializer.
func NewNodeInitializer(mode types.ExecutionMode, dockerImage string, logger *output.Logger) *NodeInitializer {
	if logger == nil {
		logger = output.DefaultLogger
	}
	return &NodeInitializer{
		mode:        mode,
		dockerImage: dockerImage,
		logger:      logger,
	}
}

// NewNodeInitializerWithBinary creates a new NodeInitializer with a specific binary path.
// For local mode, this should be the managed binary at ~/.devnet-builder/bin/stabled.
func NewNodeInitializerWithBinary(mode types.ExecutionMode, dockerImage, binaryPath string, logger *output.Logger) *NodeInitializer {
	if logger == nil {
		logger = output.DefaultLogger
	}
	return &NodeInitializer{
		mode:        mode,
		dockerImage: dockerImage,
		binaryPath:  binaryPath,
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
	// Determine binary path - use managed binary if set, otherwise fallback to PATH lookup
	binaryPath := i.binaryPath
	if binaryPath == "" {
		binaryPath = "stabled" // Fallback for backward compatibility
	}

	// Use --overwrite to handle existing genesis.json files
	args := []string{"init", moniker, "--chain-id", chainID, "--home", nodeDir, "--overwrite"}
	cmd := exec.CommandContext(ctx, binaryPath, args...)
	cmdOutput, err := cmd.CombinedOutput()
	if err != nil {
		// Print detailed error for diagnosis
		i.logger.PrintCommandError(&output.CommandErrorInfo{
			Command:  binaryPath,
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

// Export runs `stabled export` to export the current state as genesis.
func (i *NodeInitializer) Export(ctx context.Context, nodeDir, destPath string) error {
	i.logger.Debug("Exporting genesis from %s", nodeDir)

	if i.mode == types.ExecutionModeDocker {
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
	// Determine binary path - use managed binary if set, otherwise fallback to PATH lookup
	binaryPath := i.binaryPath
	if binaryPath == "" {
		binaryPath = "stabled" // Fallback for backward compatibility
	}

	// Use %q for proper shell quoting to handle paths with spaces or special characters
	cmd := exec.CommandContext(ctx, "bash", "-c",
		fmt.Sprintf("%q export --home %q > %q", binaryPath, nodeDir, destPath),
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

// CreateAccountKey creates a new secp256k1 account key for transaction signing.
// Keys are stored in the specified keyringDir with the test backend.
// This is required for validators to sign governance transactions (proposals, votes).
func (i *NodeInitializer) CreateAccountKey(ctx context.Context, keyringDir, keyName string) (*ports.AccountKeyInfo, error) {
	i.logger.Debug("Creating account key %s in %s", keyName, keyringDir)

	// Ensure keyring directory exists
	if err := os.MkdirAll(keyringDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create keyring directory: %w", err)
	}

	// Determine binary path
	binaryPath := i.binaryPath
	if binaryPath == "" {
		binaryPath = "stabled"
	}

	// Delete existing key first to avoid interactive prompt (EOF error)
	// The prompt "override the existing name X [y/N]:" causes EOF when stdin is closed
	deleteArgs := []string{
		"keys", "delete", keyName,
		"--keyring-backend", "test",
		"--home", keyringDir,
		"-y", // Force delete without confirmation
	}
	// Ignore error - key may not exist
	_ = exec.CommandContext(ctx, binaryPath, deleteArgs...).Run()

	// Create the key using stabled keys add
	// Use --output json to get structured output
	args := []string{
		"keys", "add", keyName,
		"--keyring-backend", "test",
		"--home", keyringDir,
		"--output", "json",
	}

	cmd := exec.CommandContext(ctx, binaryPath, args...)
	cmdOutput, err := cmd.CombinedOutput()
	if err != nil {
		// Check if key already exists (shouldn't happen after delete, but just in case)
		if strings.Contains(string(cmdOutput), "already exists") || strings.Contains(string(cmdOutput), "EOF") {
			// Key exists, get its info
			return i.GetAccountKey(ctx, keyringDir, keyName)
		}
		i.logger.PrintCommandError(&output.CommandErrorInfo{
			Command:  binaryPath,
			Args:     args,
			WorkDir:  keyringDir,
			Stderr:   string(cmdOutput),
			ExitCode: getExitCode(err),
			Error:    err,
		})
		return nil, fmt.Errorf("failed to create account key: %w", err)
	}

	// Parse the JSON output
	var result ports.AccountKeyInfo
	if err := json.Unmarshal(cmdOutput, &result); err != nil {
		// Try to get key info if output parsing fails
		return i.GetAccountKey(ctx, keyringDir, keyName)
	}

	return &result, nil
}

// GetAccountKey retrieves information about an existing account key.
func (i *NodeInitializer) GetAccountKey(ctx context.Context, keyringDir, keyName string) (*ports.AccountKeyInfo, error) {
	binaryPath := i.binaryPath
	if binaryPath == "" {
		binaryPath = "stabled"
	}

	args := []string{
		"keys", "show", keyName,
		"--keyring-backend", "test",
		"--home", keyringDir,
		"--output", "json",
	}

	cmd := exec.CommandContext(ctx, binaryPath, args...)
	cmdOutput, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get account key: %w", err)
	}

	var result ports.AccountKeyInfo
	if err := json.Unmarshal(cmdOutput, &result); err != nil {
		return nil, fmt.Errorf("failed to parse key info: %w", err)
	}

	return &result, nil
}

// TestMnemonics contains well-known BIP39 mnemonics for deterministic testing.
// These are the same mnemonics used by popular development tools (Ganache, Hardhat, etc.)
// DO NOT use these for any real funds - they are publicly known test mnemonics.
// We need enough mnemonics to support validators + additional accounts without overlap.
var TestMnemonics = []string{
	// Validator 0 - Standard test mnemonic used by many tools (Ganache, Hardhat default)
	"abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about",
	// Validator 1 - Hardhat secondary
	"test test test test test test test test test test test junk",
	// Validator 2 - BIP-39 test vector
	"zoo zoo zoo zoo zoo zoo zoo zoo zoo zoo zoo wrong",
	// Validator 3 - Cosmos SDK test mnemonic
	"myth like bonus scare over problem client lizard pioneer submit female collect",
	// Account 0 - BIP-39 test vector (letter = abandon but with variety)
	"letter advice cage absurd amount doctor acoustic avoid letter advice cage above",
	// Account 1 - Foundry/Anvil default mnemonic
	"body quick review slot oblige virus address analyst much half royal canvas",
	// Account 2 - Standard test vector
	"void come effort suffer camp survey warrior heavy shoot primary clutch crush",
	// Account 3 - Another standard test vector
	"ozone drill grab fiber curtain grace pudding thank cruise elder eight picnic",
	// Account 4-7 - Additional test vectors for larger deployments
	"panda eyebrow bullet gorilla call smoke muffin taste mesh discover soft ostrich",
	"alley afraid soup fall idea toss can goat luck match mechanic coin",
	"all hour make first leader extend hole alien behind guard gospel lava",
	"cram scale desert dirt muffin front slow guard word lion great blast",
}

// GetTestMnemonic returns a deterministic test mnemonic for the given validator index.
// If the index exceeds available test mnemonics, it wraps around.
// Panics if TestMnemonics slice is empty (should never happen as it's initialized with defaults).
func (i *NodeInitializer) GetTestMnemonic(validatorIndex int) string {
	if len(TestMnemonics) == 0 {
		panic("TestMnemonics slice is unexpectedly empty - this is a programming error")
	}
	return TestMnemonics[validatorIndex%len(TestMnemonics)]
}

// CreateAccountKeyFromMnemonic creates/recovers an account key from a specific mnemonic.
// This is used for deterministic testing with well-known mnemonics.
func (i *NodeInitializer) CreateAccountKeyFromMnemonic(ctx context.Context, keyringDir, keyName, mnemonic string) (*ports.AccountKeyInfo, error) {
	i.logger.Debug("Recovering account key %s from mnemonic in %s", keyName, keyringDir)

	// Ensure keyring directory exists
	if err := os.MkdirAll(keyringDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create keyring directory: %w", err)
	}

	// Determine binary path
	binaryPath := i.binaryPath
	if binaryPath == "" {
		binaryPath = "stabled"
	}

	// Delete existing key first to avoid interactive prompt
	deleteArgs := []string{
		"keys", "delete", keyName,
		"--keyring-backend", "test",
		"--home", keyringDir,
		"-y",
	}
	_ = exec.CommandContext(ctx, binaryPath, deleteArgs...).Run()

	// Recover key from mnemonic using --recover flag
	// The mnemonic is passed via stdin
	args := []string{
		"keys", "add", keyName,
		"--keyring-backend", "test",
		"--home", keyringDir,
		"--recover",
		"--output", "json",
	}

	cmd := exec.CommandContext(ctx, binaryPath, args...)
	// Pass mnemonic via stdin
	cmd.Stdin = strings.NewReader(mnemonic + "\n")

	cmdOutput, err := cmd.CombinedOutput()
	if err != nil {
		i.logger.PrintCommandError(&output.CommandErrorInfo{
			Command:  binaryPath,
			Args:     args,
			WorkDir:  keyringDir,
			Stderr:   string(cmdOutput),
			ExitCode: getExitCode(err),
			Error:    err,
		})
		return nil, fmt.Errorf("failed to recover account key from mnemonic: %w", err)
	}

	// Parse the JSON output
	var result ports.AccountKeyInfo
	if err := json.Unmarshal(cmdOutput, &result); err != nil {
		// Try to get key info if output parsing fails
		keyInfo, getErr := i.GetAccountKey(ctx, keyringDir, keyName)
		if getErr != nil {
			return nil, fmt.Errorf("failed to parse key info and get key: %w", err)
		}
		keyInfo.Mnemonic = mnemonic
		return keyInfo, nil
	}

	// Set the mnemonic in the result (recover doesn't output mnemonic)
	result.Mnemonic = mnemonic
	return &result, nil
}
