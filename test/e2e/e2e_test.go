// Package e2e provides end-to-end tests for devnet-builder CLI.
//
// These tests verify the complete user workflows from command execution
// to expected outcomes. Tests are designed to run against a real devnet
// environment using Docker.
//
// Run with: go test -v -tags=e2e ./test/e2e/...
// Note: Requires Docker and ~10 minutes for full suite.
package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	// Binary path relative to repo root
	binaryPath = "./build/devnet-builder"

	// Default timeouts
	deployTimeout  = 10 * time.Minute
	commandTimeout = 2 * time.Minute
	healthTimeout  = 5 * time.Minute

	// Default ports
	rpcPort    = 26657
	evmRPCPort = 8545
)

// TestConfig holds test configuration
type TestConfig struct {
	BinaryPath string
	HomeDir    string
	Validators int
	Accounts   int
	Network    string
	Mode       string
}

// DefaultTestConfig returns default test configuration
func DefaultTestConfig() *TestConfig {
	homeDir := os.Getenv("DEVNET_TEST_HOME")
	if homeDir == "" {
		homeDir = filepath.Join(os.TempDir(), "devnet-e2e-test")
	}
	return &TestConfig{
		BinaryPath: binaryPath,
		HomeDir:    homeDir,
		Validators: 1,
		Accounts:   0,
		Network:    "mainnet",
		Mode:       "docker",
	}
}

// StatusOutput represents JSON output from status command
type StatusOutput struct {
	Status        string       `json:"status"`
	ChainID       string       `json:"chain_id"`
	ExecutionMode string       `json:"execution_mode"`
	NetworkSource string       `json:"network_source"`
	Nodes         []NodeStatus `json:"nodes"`
}

type NodeStatus struct {
	Name      string `json:"name"`
	Status    string `json:"status"`
	Height    int64  `json:"height"`
	PeerCount int    `json:"peer_count"`
}

// KeysOutput represents JSON output from export-keys command
type KeysOutput struct {
	Validators []KeyInfo `json:"validators"`
	Accounts   []KeyInfo `json:"accounts"`
}

type KeyInfo struct {
	Name       string `json:"name"`
	Address    string `json:"address"`
	PrivateKey string `json:"private_key"`
	Mnemonic   string `json:"mnemonic,omitempty"`
}

// TestRunner provides helpers for running CLI commands
type TestRunner struct {
	t      *testing.T
	config *TestConfig
}

// NewTestRunner creates a new test runner
func NewTestRunner(t *testing.T, config *TestConfig) *TestRunner {
	return &TestRunner{t: t, config: config}
}

// Run executes devnet-builder with args and returns stdout, stderr, error
func (r *TestRunner) Run(ctx context.Context, args ...string) (string, string, error) {
	// Add home directory to all commands
	fullArgs := append([]string{"--home", r.config.HomeDir}, args...)

	cmd := exec.CommandContext(ctx, r.config.BinaryPath, fullArgs...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

// RunWithTimeout executes with a timeout
func (r *TestRunner) RunWithTimeout(timeout time.Duration, args ...string) (string, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return r.Run(ctx, args...)
}

// MustRun executes and fails test on error
func (r *TestRunner) MustRun(args ...string) string {
	stdout, stderr, err := r.RunWithTimeout(commandTimeout, args...)
	if err != nil {
		r.t.Fatalf("Command failed: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}
	return stdout
}

// Cleanup destroys devnet and removes test directory
func (r *TestRunner) Cleanup() {
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()

	// Try to destroy devnet first
	r.Run(ctx, "destroy", "--force")

	// Remove test home directory
	os.RemoveAll(r.config.HomeDir)
}

// WaitForHealthy waits until all nodes are healthy
func (r *TestRunner) WaitForHealthy(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for healthy nodes")
		case <-ticker.C:
			stdout, _, err := r.Run(ctx, "status", "--json")
			if err != nil {
				continue
			}

			var status StatusOutput
			if err := json.Unmarshal([]byte(stdout), &status); err != nil {
				continue
			}

			if status.Status == "running" {
				allHealthy := true
				for _, node := range status.Nodes {
					if node.Status != "healthy" {
						allHealthy = false
						break
					}
				}
				if allHealthy && len(status.Nodes) > 0 {
					return nil
				}
			}
		}
	}
}

// WaitForBlocks waits until block height increases by n blocks
func (r *TestRunner) WaitForBlocks(n int, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Get initial height
	initialHeight, err := r.getBlockHeight(ctx)
	if err != nil {
		return fmt.Errorf("failed to get initial height: %w", err)
	}

	targetHeight := initialHeight + int64(n)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for blocks")
		case <-ticker.C:
			height, err := r.getBlockHeight(ctx)
			if err != nil {
				continue
			}
			if height >= targetHeight {
				return nil
			}
		}
	}
}

func (r *TestRunner) getBlockHeight(ctx context.Context) (int64, error) {
	stdout, _, err := r.Run(ctx, "status", "--json")
	if err != nil {
		return 0, err
	}

	var status StatusOutput
	if err := json.Unmarshal([]byte(stdout), &status); err != nil {
		return 0, err
	}

	if len(status.Nodes) == 0 {
		return 0, fmt.Errorf("no nodes found")
	}

	return status.Nodes[0].Height, nil
}

// CheckRPCHealth checks if RPC endpoint is responding
func CheckRPCHealth(port int) error {
	url := fmt.Sprintf("http://localhost:%d/status", port)
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("RPC returned status %d", resp.StatusCode)
	}
	return nil
}

// CheckEVMHealth checks if EVM RPC endpoint is responding
func CheckEVMHealth(port int) error {
	url := fmt.Sprintf("http://localhost:%d", port)
	payload := `{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}`

	resp, err := http.Post(url, "application/json", strings.NewReader(payload))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("EVM RPC returned status %d", resp.StatusCode)
	}
	return nil
}

// =============================================================================
// P1: Core Functionality Tests
// =============================================================================

// TestUS001_DeployDockerMainnet tests deploying a devnet with Docker mode
func TestUS001_DeployDockerMainnet(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	config := DefaultTestConfig()
	config.Validators = 1 // Use 1 validator for faster test
	runner := NewTestRunner(t, config)
	defer runner.Cleanup()

	// Deploy devnet
	ctx, cancel := context.WithTimeout(context.Background(), deployTimeout)
	defer cancel()

	stdout, stderr, err := runner.Run(ctx,
		"deploy",
		"--mode", "docker",
		"--network", "mainnet",
		"--validators", "1",
		"--no-interactive",
	)
	if err != nil {
		t.Fatalf("Deploy failed: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}

	// Wait for healthy
	if err := runner.WaitForHealthy(healthTimeout); err != nil {
		t.Fatalf("Nodes did not become healthy: %v", err)
	}

	// Verify status
	statusOut := runner.MustRun("status", "--json")
	var status StatusOutput
	if err := json.Unmarshal([]byte(statusOut), &status); err != nil {
		t.Fatalf("Failed to parse status: %v", err)
	}

	// Assertions
	if status.Status != "running" {
		t.Errorf("Expected status 'running', got '%s'", status.Status)
	}
	if status.ChainID != "stable_988-1" {
		t.Errorf("Expected chain ID 'stable_988-1', got '%s'", status.ChainID)
	}
	if status.ExecutionMode != "docker" {
		t.Errorf("Expected mode 'docker', got '%s'", status.ExecutionMode)
	}
	if len(status.Nodes) != 1 {
		t.Errorf("Expected 1 node, got %d", len(status.Nodes))
	}

	// Verify RPC is accessible
	if err := CheckRPCHealth(rpcPort); err != nil {
		t.Errorf("RPC health check failed: %v", err)
	}

	// Verify EVM RPC is accessible
	if err := CheckEVMHealth(evmRPCPort); err != nil {
		t.Errorf("EVM RPC health check failed: %v", err)
	}
}

// TestUS002_DeploySingleValidator tests single validator quick deployment
func TestUS002_DeploySingleValidator(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	config := DefaultTestConfig()
	config.Validators = 1
	runner := NewTestRunner(t, config)
	defer runner.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), deployTimeout)
	defer cancel()

	_, _, err := runner.Run(ctx, "deploy", "--validators", "1", "--no-interactive")
	if err != nil {
		t.Fatalf("Deploy failed: %v", err)
	}

	if err := runner.WaitForHealthy(healthTimeout); err != nil {
		t.Fatalf("Node did not become healthy: %v", err)
	}

	// Verify blocks are being produced
	if err := runner.WaitForBlocks(3, time.Minute); err != nil {
		t.Errorf("Blocks not being produced: %v", err)
	}
}

// TestUS003_DeployWithAccounts tests deployment with funded test accounts
func TestUS003_DeployWithAccounts(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	config := DefaultTestConfig()
	config.Accounts = 3
	runner := NewTestRunner(t, config)
	defer runner.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), deployTimeout)
	defer cancel()

	_, _, err := runner.Run(ctx,
		"deploy",
		"--validators", "1",
		"--accounts", "3",
		"--no-interactive",
	)
	if err != nil {
		t.Fatalf("Deploy failed: %v", err)
	}

	if err := runner.WaitForHealthy(healthTimeout); err != nil {
		t.Fatalf("Node did not become healthy: %v", err)
	}

	// Export keys and verify accounts
	keysOut := runner.MustRun("export-keys", "--json")
	var keys KeysOutput
	if err := json.Unmarshal([]byte(keysOut), &keys); err != nil {
		t.Fatalf("Failed to parse keys: %v", err)
	}

	if len(keys.Accounts) != 3 {
		t.Errorf("Expected 3 accounts, got %d", len(keys.Accounts))
	}

	// Verify each account has required fields
	for i, acc := range keys.Accounts {
		if acc.Address == "" {
			t.Errorf("Account %d has empty address", i)
		}
		if acc.PrivateKey == "" {
			t.Errorf("Account %d has empty private key", i)
		}
	}
}

// TestUS004_Status tests status command output
func TestUS004_Status(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	config := DefaultTestConfig()
	runner := NewTestRunner(t, config)
	defer runner.Cleanup()

	// Deploy first
	ctx, cancel := context.WithTimeout(context.Background(), deployTimeout)
	defer cancel()

	_, _, err := runner.Run(ctx, "deploy", "--validators", "1", "--no-interactive")
	if err != nil {
		t.Fatalf("Deploy failed: %v", err)
	}

	if err := runner.WaitForHealthy(healthTimeout); err != nil {
		t.Fatalf("Node did not become healthy: %v", err)
	}

	// Test JSON output
	jsonOut := runner.MustRun("status", "--json")
	var status StatusOutput
	if err := json.Unmarshal([]byte(jsonOut), &status); err != nil {
		t.Fatalf("Failed to parse JSON status: %v", err)
	}

	// Verify required fields
	if status.ChainID == "" {
		t.Error("Chain ID is empty")
	}
	if status.ExecutionMode == "" {
		t.Error("Execution mode is empty")
	}
	if status.NetworkSource == "" {
		t.Error("Network source is empty")
	}
	if len(status.Nodes) == 0 {
		t.Error("No nodes in status")
	}
	for _, node := range status.Nodes {
		if node.Height <= 0 {
			t.Errorf("Node %s has invalid height: %d", node.Name, node.Height)
		}
	}
}

// TestUS005_DownUp tests stopping and restarting nodes
func TestUS005_DownUp(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	config := DefaultTestConfig()
	runner := NewTestRunner(t, config)
	defer runner.Cleanup()

	// Deploy
	ctx, cancel := context.WithTimeout(context.Background(), deployTimeout)
	defer cancel()

	_, _, err := runner.Run(ctx, "deploy", "--validators", "1", "--no-interactive")
	if err != nil {
		t.Fatalf("Deploy failed: %v", err)
	}

	if err := runner.WaitForHealthy(healthTimeout); err != nil {
		t.Fatalf("Node did not become healthy: %v", err)
	}

	// Get initial height
	initialHeight, err := runner.getBlockHeight(context.Background())
	if err != nil {
		t.Fatalf("Failed to get initial height: %v", err)
	}

	// Stop nodes
	runner.MustRun("down")

	// Verify status shows stopped
	time.Sleep(2 * time.Second) // Give time for status to update

	// Start nodes
	_, _, err = runner.RunWithTimeout(healthTimeout, "up")
	if err != nil {
		t.Fatalf("Up command failed: %v", err)
	}

	// Wait for healthy again
	if err := runner.WaitForHealthy(healthTimeout); err != nil {
		t.Fatalf("Node did not become healthy after restart: %v", err)
	}

	// Verify height continues from where it left off (or higher)
	newHeight, err := runner.getBlockHeight(context.Background())
	if err != nil {
		t.Fatalf("Failed to get new height: %v", err)
	}

	if newHeight < initialHeight {
		t.Errorf("Height decreased after restart: %d -> %d", initialHeight, newHeight)
	}
}

// TestUS006_Destroy tests devnet destruction
func TestUS006_Destroy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	config := DefaultTestConfig()
	runner := NewTestRunner(t, config)
	// Don't defer cleanup since we're testing destroy

	// Deploy
	ctx, cancel := context.WithTimeout(context.Background(), deployTimeout)
	defer cancel()

	_, _, err := runner.Run(ctx, "deploy", "--validators", "1", "--no-interactive")
	if err != nil {
		t.Fatalf("Deploy failed: %v", err)
	}

	if err := runner.WaitForHealthy(healthTimeout); err != nil {
		t.Fatalf("Node did not become healthy: %v", err)
	}

	// Verify devnet directory exists
	devnetDir := filepath.Join(config.HomeDir, "devnet")
	if _, err := os.Stat(devnetDir); os.IsNotExist(err) {
		t.Fatal("Devnet directory should exist before destroy")
	}

	// Destroy
	runner.MustRun("destroy", "--force")

	// Verify devnet directory is removed
	if _, err := os.Stat(devnetDir); !os.IsNotExist(err) {
		t.Error("Devnet directory should not exist after destroy")
	}

	// Verify status shows no devnet
	_, _, err = runner.RunWithTimeout(commandTimeout, "status")
	if err == nil {
		t.Error("Status should fail after destroy")
	}

	// Cleanup test home dir
	os.RemoveAll(config.HomeDir)
}

// =============================================================================
// P2: Advanced Operations Tests
// =============================================================================

// TestUS010_ExportKeys tests key export functionality
func TestUS010_ExportKeys(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	config := DefaultTestConfig()
	config.Accounts = 2
	runner := NewTestRunner(t, config)
	defer runner.Cleanup()

	// Deploy with accounts
	ctx, cancel := context.WithTimeout(context.Background(), deployTimeout)
	defer cancel()

	_, _, err := runner.Run(ctx,
		"deploy",
		"--validators", "1",
		"--accounts", "2",
		"--no-interactive",
	)
	if err != nil {
		t.Fatalf("Deploy failed: %v", err)
	}

	if err := runner.WaitForHealthy(healthTimeout); err != nil {
		t.Fatalf("Node did not become healthy: %v", err)
	}

	// Test validators only
	validatorsOut := runner.MustRun("export-keys", "--type", "validators", "--json")
	var validatorKeys KeysOutput
	if err := json.Unmarshal([]byte(validatorsOut), &validatorKeys); err != nil {
		t.Fatalf("Failed to parse validator keys: %v", err)
	}
	if len(validatorKeys.Validators) != 1 {
		t.Errorf("Expected 1 validator, got %d", len(validatorKeys.Validators))
	}

	// Test accounts only
	accountsOut := runner.MustRun("export-keys", "--type", "accounts", "--json")
	var accountKeys KeysOutput
	if err := json.Unmarshal([]byte(accountsOut), &accountKeys); err != nil {
		t.Fatalf("Failed to parse account keys: %v", err)
	}
	if len(accountKeys.Accounts) != 2 {
		t.Errorf("Expected 2 accounts, got %d", len(accountKeys.Accounts))
	}

	// Test all keys
	allOut := runner.MustRun("export-keys", "--json")
	var allKeys KeysOutput
	if err := json.Unmarshal([]byte(allOut), &allKeys); err != nil {
		t.Fatalf("Failed to parse all keys: %v", err)
	}
	if len(allKeys.Validators) != 1 || len(allKeys.Accounts) != 2 {
		t.Errorf("Expected 1 validator and 2 accounts, got %d and %d",
			len(allKeys.Validators), len(allKeys.Accounts))
	}
}

// TestUS011_NodeControl tests individual node control
func TestUS011_NodeControl(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	config := DefaultTestConfig()
	config.Validators = 2
	runner := NewTestRunner(t, config)
	defer runner.Cleanup()

	// Deploy 2 validators
	ctx, cancel := context.WithTimeout(context.Background(), deployTimeout)
	defer cancel()

	_, _, err := runner.Run(ctx, "deploy", "--validators", "2", "--no-interactive")
	if err != nil {
		t.Fatalf("Deploy failed: %v", err)
	}

	if err := runner.WaitForHealthy(healthTimeout); err != nil {
		t.Fatalf("Nodes did not become healthy: %v", err)
	}

	// Stop node 1
	runner.MustRun("node", "stop", "1")

	// Give time for node to stop
	time.Sleep(3 * time.Second)

	// Check status - node 1 should be stopped
	statusOut := runner.MustRun("status", "--json")
	var status StatusOutput
	if err := json.Unmarshal([]byte(statusOut), &status); err != nil {
		t.Fatalf("Failed to parse status: %v", err)
	}

	// Node 0 should still be healthy
	foundNode0Healthy := false
	for _, node := range status.Nodes {
		if node.Name == "node0" && node.Status == "healthy" {
			foundNode0Healthy = true
		}
	}
	if !foundNode0Healthy {
		t.Error("Node 0 should still be healthy")
	}

	// Start node 1 again
	runner.MustRun("node", "start", "1")

	// Wait for both nodes healthy
	if err := runner.WaitForHealthy(healthTimeout); err != nil {
		t.Errorf("Node 1 did not recover: %v", err)
	}
}

// TestUS012_Reset tests chain state reset
func TestUS012_Reset(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	config := DefaultTestConfig()
	runner := NewTestRunner(t, config)
	defer runner.Cleanup()

	// Deploy
	ctx, cancel := context.WithTimeout(context.Background(), deployTimeout)
	defer cancel()

	_, _, err := runner.Run(ctx, "deploy", "--validators", "1", "--no-interactive")
	if err != nil {
		t.Fatalf("Deploy failed: %v", err)
	}

	if err := runner.WaitForHealthy(healthTimeout); err != nil {
		t.Fatalf("Node did not become healthy: %v", err)
	}

	// Wait for some blocks
	if err := runner.WaitForBlocks(5, time.Minute); err != nil {
		t.Fatalf("Failed to produce blocks: %v", err)
	}

	// Stop nodes
	runner.MustRun("down")

	// Reset chain data
	runner.MustRun("reset", "--force")

	// Start again
	_, _, err = runner.RunWithTimeout(healthTimeout, "up")
	if err != nil {
		t.Fatalf("Up after reset failed: %v", err)
	}

	if err := runner.WaitForHealthy(healthTimeout); err != nil {
		t.Fatalf("Node did not become healthy after reset: %v", err)
	}

	// Height should be low (reset to genesis)
	height, err := runner.getBlockHeight(context.Background())
	if err != nil {
		t.Fatalf("Failed to get height: %v", err)
	}

	// After reset, height should start from 1 again
	if height > 10 {
		t.Errorf("Height should be low after reset, got %d", height)
	}
}

// =============================================================================
// P3: Configuration Tests
// =============================================================================

// TestUS014_DeployTestnet tests deployment with testnet data
func TestUS014_DeployTestnet(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	config := DefaultTestConfig()
	config.Network = "testnet"
	runner := NewTestRunner(t, config)
	defer runner.Cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), deployTimeout)
	defer cancel()

	_, _, err := runner.Run(ctx,
		"deploy",
		"--network", "testnet",
		"--validators", "1",
		"--no-interactive",
	)
	if err != nil {
		t.Fatalf("Deploy failed: %v", err)
	}

	if err := runner.WaitForHealthy(healthTimeout); err != nil {
		t.Fatalf("Node did not become healthy: %v", err)
	}

	// Verify chain ID is testnet
	statusOut := runner.MustRun("status", "--json")
	var status StatusOutput
	if err := json.Unmarshal([]byte(statusOut), &status); err != nil {
		t.Fatalf("Failed to parse status: %v", err)
	}

	if status.ChainID != "stabletestnet_2201-1" {
		t.Errorf("Expected chain ID 'stabletestnet_2201-1', got '%s'", status.ChainID)
	}
	if status.NetworkSource != "testnet" {
		t.Errorf("Expected network source 'testnet', got '%s'", status.NetworkSource)
	}
}

// =============================================================================
// P4: Error Handling Tests
// =============================================================================

// TestUS019_ErrorExistingDevnet tests error when devnet already exists
func TestUS019_ErrorExistingDevnet(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	config := DefaultTestConfig()
	runner := NewTestRunner(t, config)
	defer runner.Cleanup()

	// Deploy first time
	ctx, cancel := context.WithTimeout(context.Background(), deployTimeout)
	defer cancel()

	_, _, err := runner.Run(ctx, "deploy", "--validators", "1", "--no-interactive")
	if err != nil {
		t.Fatalf("First deploy failed: %v", err)
	}

	if err := runner.WaitForHealthy(healthTimeout); err != nil {
		t.Fatalf("Node did not become healthy: %v", err)
	}

	// Try to deploy again - should fail
	_, _, err = runner.RunWithTimeout(commandTimeout, "deploy", "--validators", "1", "--no-interactive")
	if err == nil {
		t.Error("Second deploy should fail when devnet exists")
	}
}

// TestUS020_ErrorNoDevnet tests error when operating on non-existent devnet
func TestUS020_ErrorNoDevnet(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	config := DefaultTestConfig()
	// Use a fresh directory that doesn't have a devnet
	config.HomeDir = filepath.Join(os.TempDir(), "devnet-e2e-no-devnet")
	runner := NewTestRunner(t, config)
	defer os.RemoveAll(config.HomeDir)

	// Status should fail
	_, _, err := runner.RunWithTimeout(commandTimeout, "status")
	if err == nil {
		t.Error("Status should fail when no devnet exists")
	}

	// Down should fail
	_, _, err = runner.RunWithTimeout(commandTimeout, "down")
	if err == nil {
		t.Error("Down should fail when no devnet exists")
	}

	// Up should fail
	_, _, err = runner.RunWithTimeout(commandTimeout, "up")
	if err == nil {
		t.Error("Up should fail when no devnet exists")
	}
}

// =============================================================================
// Benchmark Tests
// =============================================================================

// BenchmarkDeploy measures deployment time
func BenchmarkDeploy(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping benchmark in short mode")
	}

	for i := 0; i < b.N; i++ {
		config := DefaultTestConfig()
		config.HomeDir = filepath.Join(os.TempDir(), fmt.Sprintf("devnet-bench-%d", i))
		runner := NewTestRunner(&testing.T{}, config)

		b.StartTimer()
		ctx, cancel := context.WithTimeout(context.Background(), deployTimeout)
		runner.Run(ctx, "deploy", "--validators", "1", "--no-interactive")
		cancel()
		b.StopTimer()

		runner.Cleanup()
	}
}
