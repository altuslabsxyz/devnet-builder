// internal/daemon/provisioner/node_initializer_test.go
package provisioner

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/altuslabsxyz/devnet-builder/internal/application/ports"
	"github.com/altuslabsxyz/devnet-builder/internal/plugin/types"
)

// mockPluginInitializer implements types.PluginInitializer for testing
type mockPluginInitializer struct {
	binaryName     string
	defaultChainID string
}

func (m *mockPluginInitializer) BinaryName() string {
	return m.binaryName
}

func (m *mockPluginInitializer) DefaultChainID() string {
	return m.defaultChainID
}

func (m *mockPluginInitializer) DefaultMoniker(index int) string {
	return fmt.Sprintf("validator-%d", index)
}

func (m *mockPluginInitializer) InitCommandArgs(homeDir, moniker, chainID string) []string {
	return []string{"init", moniker, "--chain-id", chainID, "--home", homeDir, "--overwrite"}
}

func (m *mockPluginInitializer) ConfigDir(homeDir string) string {
	return filepath.Join(homeDir, "config")
}

func (m *mockPluginInitializer) DataDir(homeDir string) string {
	return filepath.Join(homeDir, "data")
}

func (m *mockPluginInitializer) KeyringDir(homeDir string) string {
	return homeDir
}

// mockInfraInitializer implements ports.NodeInitializer for testing
type mockInfraInitializer struct {
	initError     error
	nodeIDError   error
	nodeIDValue   string
	initCalls     []mockInitCall
	nodeIDCalls   []string
	accountKeys   map[string]*ports.AccountKeyInfo
	testMnemonics []string
}

type mockInitCall struct {
	NodeDir string
	Moniker string
	ChainID string
}

func (m *mockInfraInitializer) Initialize(ctx context.Context, nodeDir, moniker, chainID string) error {
	m.initCalls = append(m.initCalls, mockInitCall{
		NodeDir: nodeDir,
		Moniker: moniker,
		ChainID: chainID,
	})
	if m.initError != nil {
		return m.initError
	}
	// Create the config directory structure like the real initializer would
	configDir := filepath.Join(nodeDir, "config")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}
	return nil
}

func (m *mockInfraInitializer) GetNodeID(ctx context.Context, nodeDir string) (string, error) {
	m.nodeIDCalls = append(m.nodeIDCalls, nodeDir)
	if m.nodeIDError != nil {
		return "", m.nodeIDError
	}
	return m.nodeIDValue, nil
}

func (m *mockInfraInitializer) CreateAccountKey(ctx context.Context, keyringDir, keyName string) (*ports.AccountKeyInfo, error) {
	if m.accountKeys != nil {
		if key, ok := m.accountKeys[keyName]; ok {
			return key, nil
		}
	}
	return &ports.AccountKeyInfo{Name: keyName, Address: "test-address"}, nil
}

func (m *mockInfraInitializer) CreateAccountKeyFromMnemonic(ctx context.Context, keyringDir, keyName, mnemonic string) (*ports.AccountKeyInfo, error) {
	return &ports.AccountKeyInfo{Name: keyName, Address: "test-address", Mnemonic: mnemonic}, nil
}

func (m *mockInfraInitializer) GetAccountKey(ctx context.Context, keyringDir, keyName string) (*ports.AccountKeyInfo, error) {
	if m.accountKeys != nil {
		if key, ok := m.accountKeys[keyName]; ok {
			return key, nil
		}
	}
	return &ports.AccountKeyInfo{Name: keyName, Address: "test-address"}, nil
}

func (m *mockInfraInitializer) GetTestMnemonic(validatorIndex int) string {
	if m.testMnemonics != nil && validatorIndex < len(m.testMnemonics) {
		return m.testMnemonics[validatorIndex]
	}
	return "test mnemonic for validator"
}

func TestNodeInitializerConfig(t *testing.T) {
	tempDir := t.TempDir()

	config := NodeInitializerConfig{
		DataDir:    tempDir,
		PluginInit: &mockPluginInitializer{binaryName: "testd"},
	}

	initializer := NewNodeInitializer(config)

	if initializer == nil {
		t.Fatal("NewNodeInitializer returned nil")
	}

	if initializer.logger == nil {
		t.Error("Expected default logger to be set")
	}
}

func TestNodeInitializerInitializeNode_WithInfraInit(t *testing.T) {
	tempDir := t.TempDir()

	mockInfra := &mockInfraInitializer{
		nodeIDValue: "test-node-id",
	}

	config := NodeInitializerConfig{
		DataDir:   tempDir,
		InfraInit: mockInfra,
	}

	initializer := NewNodeInitializer(config)

	nodeConfig := types.NodeInitConfig{
		HomeDir:        filepath.Join(tempDir, "node0"),
		Moniker:        "validator-0",
		ChainID:        "test-chain",
		ValidatorIndex: 0,
	}

	ctx := context.Background()
	err := initializer.InitializeNode(ctx, nodeConfig)
	if err != nil {
		t.Fatalf("InitializeNode failed: %v", err)
	}

	// Verify infrastructure was called
	if len(mockInfra.initCalls) != 1 {
		t.Errorf("Expected 1 init call, got %d", len(mockInfra.initCalls))
	}

	call := mockInfra.initCalls[0]
	if call.NodeDir != nodeConfig.HomeDir {
		t.Errorf("Expected nodeDir %s, got %s", nodeConfig.HomeDir, call.NodeDir)
	}
	if call.Moniker != "validator-0" {
		t.Errorf("Expected moniker 'validator-0', got '%s'", call.Moniker)
	}
	if call.ChainID != "test-chain" {
		t.Errorf("Expected chainID 'test-chain', got '%s'", call.ChainID)
	}
}

func TestNodeInitializerInitializeNode_InvalidConfig(t *testing.T) {
	tempDir := t.TempDir()

	config := NodeInitializerConfig{
		DataDir:   tempDir,
		InfraInit: &mockInfraInitializer{},
	}

	initializer := NewNodeInitializer(config)

	// Test with empty HomeDir
	nodeConfig := types.NodeInitConfig{
		HomeDir:        "", // Invalid: empty
		Moniker:        "validator-0",
		ChainID:        "test-chain",
		ValidatorIndex: 0,
	}

	ctx := context.Background()
	err := initializer.InitializeNode(ctx, nodeConfig)
	if err == nil {
		t.Fatal("Expected error for invalid config")
	}
	if !strings.Contains(err.Error(), "HomeDir") {
		t.Errorf("Expected error about HomeDir, got: %v", err)
	}
}

func TestNodeInitializerInitializeMultipleNodes(t *testing.T) {
	tempDir := t.TempDir()

	mockInfra := &mockInfraInitializer{}

	config := NodeInitializerConfig{
		DataDir:   tempDir,
		InfraInit: mockInfra,
	}

	initializer := NewNodeInitializer(config)

	configs := []types.NodeInitConfig{
		{HomeDir: filepath.Join(tempDir, "node0"), Moniker: "validator-0", ChainID: "test-chain", ValidatorIndex: 0},
		{HomeDir: filepath.Join(tempDir, "node1"), Moniker: "validator-1", ChainID: "test-chain", ValidatorIndex: 1},
		{HomeDir: filepath.Join(tempDir, "node2"), Moniker: "validator-2", ChainID: "test-chain", ValidatorIndex: 2},
	}

	ctx := context.Background()
	err := initializer.InitializeMultipleNodes(ctx, configs)
	if err != nil {
		t.Fatalf("InitializeMultipleNodes failed: %v", err)
	}

	// Verify all nodes were initialized
	if len(mockInfra.initCalls) != 3 {
		t.Errorf("Expected 3 init calls, got %d", len(mockInfra.initCalls))
	}

	for i, call := range mockInfra.initCalls {
		expectedMoniker := fmt.Sprintf("validator-%d", i)
		if call.Moniker != expectedMoniker {
			t.Errorf("Node %d: expected moniker '%s', got '%s'", i, expectedMoniker, call.Moniker)
		}
	}
}

func TestNodeInitializerInitializeMultipleNodes_Empty(t *testing.T) {
	tempDir := t.TempDir()

	config := NodeInitializerConfig{
		DataDir:   tempDir,
		InfraInit: &mockInfraInitializer{},
	}

	initializer := NewNodeInitializer(config)

	ctx := context.Background()
	err := initializer.InitializeMultipleNodes(ctx, []types.NodeInitConfig{})
	if err == nil {
		t.Fatal("Expected error for empty configs")
	}
}

func TestNodeInitializerGetNodeID_WithInfraInit(t *testing.T) {
	tempDir := t.TempDir()

	expectedNodeID := "abc123def456789"
	mockInfra := &mockInfraInitializer{
		nodeIDValue: expectedNodeID,
	}

	config := NodeInitializerConfig{
		DataDir:   tempDir,
		InfraInit: mockInfra,
	}

	initializer := NewNodeInitializer(config)

	ctx := context.Background()
	nodeID, err := initializer.GetNodeID(ctx, filepath.Join(tempDir, "node0"))
	if err != nil {
		t.Fatalf("GetNodeID failed: %v", err)
	}

	if nodeID != expectedNodeID {
		t.Errorf("Expected node ID '%s', got '%s'", expectedNodeID, nodeID)
	}
}

func TestNodeInitializerGetNodeID_FromFile(t *testing.T) {
	tempDir := t.TempDir()

	// Create a mock node_key.json file
	configDir := filepath.Join(tempDir, "node0", "config")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	// Create a test ed25519 key (64 bytes: 32-byte seed + 32-byte public key)
	// Using a known test key for reproducibility
	testPrivKey := make([]byte, 64)
	for i := 0; i < 64; i++ {
		testPrivKey[i] = byte(i)
	}
	testPrivKeyB64 := base64.StdEncoding.EncodeToString(testPrivKey)

	nodeKey := map[string]interface{}{
		"priv_key": map[string]interface{}{
			"type":  "tendermint/PrivKeyEd25519",
			"value": testPrivKeyB64,
		},
	}
	nodeKeyBytes, _ := json.Marshal(nodeKey)
	nodeKeyPath := filepath.Join(configDir, "node_key.json")
	if err := os.WriteFile(nodeKeyPath, nodeKeyBytes, 0644); err != nil {
		t.Fatalf("Failed to write node_key.json: %v", err)
	}

	config := NodeInitializerConfig{
		DataDir:    tempDir,
		PluginInit: &mockPluginInitializer{},
		// No InfraInit - should use file-based fallback
	}

	initializer := NewNodeInitializer(config)

	ctx := context.Background()
	nodeID, err := initializer.GetNodeID(ctx, filepath.Join(tempDir, "node0"))
	if err != nil {
		t.Fatalf("GetNodeID failed: %v", err)
	}

	// Node ID should be a 40-character hex string (20 bytes)
	if len(nodeID) != 40 {
		t.Errorf("Expected 40-character node ID, got %d characters: %s", len(nodeID), nodeID)
	}

	// Verify it's valid hex
	for _, c := range nodeID {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("Invalid hex character in node ID: %c", c)
		}
	}
}

func TestNodeInitializerBuildPersistentPeers(t *testing.T) {
	tempDir := t.TempDir()

	config := NodeInitializerConfig{
		DataDir: tempDir,
	}

	initializer := NewNodeInitializer(config)

	nodeIDs := []string{
		"abc123",
		"def456",
		"ghi789",
	}
	basePort := 26656

	peers := initializer.BuildPersistentPeers(nodeIDs, basePort)

	// Expected: "abc123@127.0.0.1:26656,def456@127.0.0.1:36656,ghi789@127.0.0.1:46656"
	expectedParts := []string{
		"abc123@127.0.0.1:26656",
		"def456@127.0.0.1:36656",
		"ghi789@127.0.0.1:46656",
	}

	for _, expected := range expectedParts {
		if !strings.Contains(peers, expected) {
			t.Errorf("Expected peers to contain '%s', got: %s", expected, peers)
		}
	}

	// Verify the format
	peerList := strings.Split(peers, ",")
	if len(peerList) != 3 {
		t.Errorf("Expected 3 peers, got %d", len(peerList))
	}
}

func TestNodeInitializerBuildPersistentPeers_Empty(t *testing.T) {
	tempDir := t.TempDir()

	config := NodeInitializerConfig{
		DataDir: tempDir,
	}

	initializer := NewNodeInitializer(config)

	peers := initializer.BuildPersistentPeers([]string{}, 26656)

	if peers != "" {
		t.Errorf("Expected empty peers string, got: %s", peers)
	}
}

func TestNodeInitializerBuildPersistentPeers_SingleNode(t *testing.T) {
	tempDir := t.TempDir()

	config := NodeInitializerConfig{
		DataDir: tempDir,
	}

	initializer := NewNodeInitializer(config)

	nodeIDs := []string{"abc123"}
	basePort := 26656

	peers := initializer.BuildPersistentPeers(nodeIDs, basePort)

	expected := "abc123@127.0.0.1:26656"
	if peers != expected {
		t.Errorf("Expected '%s', got '%s'", expected, peers)
	}
}

func TestNodeInitializerBuildPersistentPeersWithExclusion(t *testing.T) {
	tempDir := t.TempDir()

	config := NodeInitializerConfig{
		DataDir: tempDir,
	}

	initializer := NewNodeInitializer(config)

	nodeIDs := []string{
		"abc123",
		"def456",
		"ghi789",
	}
	basePort := 26656

	// Exclude node 1 (def456)
	peers := initializer.BuildPersistentPeersWithExclusion(nodeIDs, basePort, 1)

	// Should only contain nodes 0 and 2
	if strings.Contains(peers, "def456") {
		t.Errorf("Expected peers to NOT contain 'def456', got: %s", peers)
	}

	if !strings.Contains(peers, "abc123@127.0.0.1:26656") {
		t.Errorf("Expected peers to contain 'abc123@127.0.0.1:26656', got: %s", peers)
	}

	if !strings.Contains(peers, "ghi789@127.0.0.1:46656") {
		t.Errorf("Expected peers to contain 'ghi789@127.0.0.1:46656', got: %s", peers)
	}
}

func TestNodeInitializerGetAllNodeIDs(t *testing.T) {
	tempDir := t.TempDir()

	// Create mock node directories with node_key.json files
	for i := 0; i < 3; i++ {
		configDir := filepath.Join(tempDir, fmt.Sprintf("node%d", i), "config")
		if err := os.MkdirAll(configDir, 0755); err != nil {
			t.Fatalf("Failed to create config dir: %v", err)
		}

		// Create a unique test key for each node
		testPrivKey := make([]byte, 64)
		for j := 0; j < 64; j++ {
			testPrivKey[j] = byte(i*64 + j)
		}
		testPrivKeyB64 := base64.StdEncoding.EncodeToString(testPrivKey)

		nodeKey := map[string]interface{}{
			"priv_key": map[string]interface{}{
				"type":  "tendermint/PrivKeyEd25519",
				"value": testPrivKeyB64,
			},
		}
		nodeKeyBytes, _ := json.Marshal(nodeKey)
		if err := os.WriteFile(filepath.Join(configDir, "node_key.json"), nodeKeyBytes, 0644); err != nil {
			t.Fatalf("Failed to write node_key.json: %v", err)
		}
	}

	config := NodeInitializerConfig{
		DataDir:    tempDir,
		PluginInit: &mockPluginInitializer{},
	}

	initializer := NewNodeInitializer(config)

	homeDirs := []string{
		filepath.Join(tempDir, "node0"),
		filepath.Join(tempDir, "node1"),
		filepath.Join(tempDir, "node2"),
	}

	ctx := context.Background()
	nodeIDs, err := initializer.GetAllNodeIDs(ctx, homeDirs)
	if err != nil {
		t.Fatalf("GetAllNodeIDs failed: %v", err)
	}

	if len(nodeIDs) != 3 {
		t.Errorf("Expected 3 node IDs, got %d", len(nodeIDs))
	}

	// Verify each node ID is unique
	seen := make(map[string]bool)
	for _, id := range nodeIDs {
		if seen[id] {
			t.Errorf("Duplicate node ID found: %s", id)
		}
		seen[id] = true
	}
}

func TestNodeInitializerGetNodeID_FileNotFound(t *testing.T) {
	tempDir := t.TempDir()

	config := NodeInitializerConfig{
		DataDir:    tempDir,
		PluginInit: &mockPluginInitializer{},
	}

	initializer := NewNodeInitializer(config)

	ctx := context.Background()
	_, err := initializer.GetNodeID(ctx, filepath.Join(tempDir, "nonexistent"))
	if err == nil {
		t.Fatal("Expected error for non-existent node directory")
	}
}

func TestNodeInitializerWithNilPluginInit(t *testing.T) {
	tempDir := t.TempDir()

	mockInfra := &mockInfraInitializer{}

	config := NodeInitializerConfig{
		DataDir:    tempDir,
		PluginInit: nil, // No plugin initializer
		InfraInit:  mockInfra,
	}

	initializer := NewNodeInitializer(config)

	nodeConfig := types.NodeInitConfig{
		HomeDir:        filepath.Join(tempDir, "node0"),
		Moniker:        "validator-0",
		ChainID:        "test-chain",
		ValidatorIndex: 0,
	}

	ctx := context.Background()
	err := initializer.InitializeNode(ctx, nodeConfig)
	if err != nil {
		t.Fatalf("InitializeNode failed: %v", err)
	}

	// Should still work with infrastructure fallback
	if len(mockInfra.initCalls) != 1 {
		t.Errorf("Expected 1 init call, got %d", len(mockInfra.initCalls))
	}
}

func TestNodeInitializerWithoutInfraOrBinary(t *testing.T) {
	tempDir := t.TempDir()

	// No InfraInit and no BinaryPath
	config := NodeInitializerConfig{
		DataDir:    tempDir,
		PluginInit: nil, // No plugin either
	}

	initializer := NewNodeInitializer(config)

	nodeConfig := types.NodeInitConfig{
		HomeDir:        filepath.Join(tempDir, "node0"),
		Moniker:        "validator-0",
		ChainID:        "test-chain",
		ValidatorIndex: 0,
	}

	ctx := context.Background()
	err := initializer.InitializeNode(ctx, nodeConfig)
	if err == nil {
		t.Fatal("Expected error when no infrastructure or binary available")
	}
}
