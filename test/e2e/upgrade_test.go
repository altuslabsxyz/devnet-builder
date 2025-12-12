// Package e2e provides end-to-end tests for devnet-builder CLI.
package e2e

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const (
	// Upgrade-specific timeouts
	upgradeTimeout      = 15 * time.Minute
	upgradeVotingPeriod = "60s"
)

// UpgradeOutput represents JSON output from upgrade command
type UpgradeOutput struct {
	ProposalID    string `json:"proposal_id"`
	UpgradeHeight int64  `json:"upgrade_height"`
	Status        string `json:"status"`
	Duration      string `json:"duration"`
}

// =============================================================================
// P2: Upgrade Tests
// =============================================================================

// TestUS007_UpgradeDockerImage tests upgrading via Docker image swap
func TestUS007_UpgradeDockerImage(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E upgrade test in short mode")
	}

	// Skip if no upgrade image is available
	upgradeImage := os.Getenv("DEVNET_UPGRADE_IMAGE")
	if upgradeImage == "" {
		t.Skip("DEVNET_UPGRADE_IMAGE not set, skipping upgrade test")
	}

	config := DefaultTestConfig()
	runner := NewTestRunner(t, config)
	defer runner.Cleanup()

	// Deploy initial version
	ctx, cancel := context.WithTimeout(context.Background(), deployTimeout)
	defer cancel()

	_, _, err := runner.Run(ctx, "deploy", "--validators", "1", "--no-interactive")
	if err != nil {
		t.Fatalf("Deploy failed: %v", err)
	}

	if err := runner.WaitForHealthy(healthTimeout); err != nil {
		t.Fatalf("Node did not become healthy: %v", err)
	}

	// Get initial version/height
	initialHeight, err := runner.getBlockHeight(context.Background())
	if err != nil {
		t.Fatalf("Failed to get initial height: %v", err)
	}
	t.Logf("Initial height: %d", initialHeight)

	// Perform upgrade
	upgradeCtx, upgradeCancel := context.WithTimeout(context.Background(), upgradeTimeout)
	defer upgradeCancel()

	stdout, stderr, err := runner.Run(upgradeCtx,
		"upgrade",
		"--name", "test-upgrade",
		"--image", upgradeImage,
		"--voting-period", upgradeVotingPeriod,
		"--no-interactive",
	)
	if err != nil {
		t.Fatalf("Upgrade failed: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}

	// Wait for node to be healthy after upgrade
	if err := runner.WaitForHealthy(healthTimeout); err != nil {
		t.Fatalf("Node did not become healthy after upgrade: %v", err)
	}

	// Verify chain continues producing blocks
	if err := runner.WaitForBlocks(5, time.Minute); err != nil {
		t.Errorf("Chain stopped producing blocks after upgrade: %v", err)
	}

	// Verify height increased
	newHeight, err := runner.getBlockHeight(context.Background())
	if err != nil {
		t.Fatalf("Failed to get new height: %v", err)
	}
	t.Logf("New height: %d", newHeight)

	if newHeight <= initialHeight {
		t.Errorf("Height should have increased after upgrade: %d -> %d", initialHeight, newHeight)
	}
}

// TestUS008_UpgradeLocalBinary tests upgrading via local binary swap
func TestUS008_UpgradeLocalBinary(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E upgrade test in short mode")
	}

	// Skip if no upgrade binary is available
	upgradeBinary := os.Getenv("DEVNET_UPGRADE_BINARY")
	if upgradeBinary == "" {
		t.Skip("DEVNET_UPGRADE_BINARY not set, skipping binary upgrade test")
	}

	// Verify binary exists
	if _, err := os.Stat(upgradeBinary); os.IsNotExist(err) {
		t.Skipf("Upgrade binary not found at %s", upgradeBinary)
	}

	config := DefaultTestConfig()
	config.Mode = "local"
	runner := NewTestRunner(t, config)
	defer runner.Cleanup()

	// Deploy initial version in local mode
	ctx, cancel := context.WithTimeout(context.Background(), deployTimeout)
	defer cancel()

	_, _, err := runner.Run(ctx,
		"deploy",
		"--mode", "local",
		"--validators", "1",
		"--no-interactive",
	)
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

	// Perform upgrade with local binary
	upgradeCtx, upgradeCancel := context.WithTimeout(context.Background(), upgradeTimeout)
	defer upgradeCancel()

	stdout, stderr, err := runner.Run(upgradeCtx,
		"upgrade",
		"--name", "test-binary-upgrade",
		"--binary", upgradeBinary,
		"--voting-period", upgradeVotingPeriod,
		"--no-interactive",
	)
	if err != nil {
		t.Fatalf("Upgrade failed: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}

	// Wait for healthy
	if err := runner.WaitForHealthy(healthTimeout); err != nil {
		t.Fatalf("Node did not become healthy after upgrade: %v", err)
	}

	// Verify blocks continue
	if err := runner.WaitForBlocks(3, time.Minute); err != nil {
		t.Errorf("Chain stopped producing blocks after binary upgrade: %v", err)
	}

	// Verify height increased
	newHeight, err := runner.getBlockHeight(context.Background())
	if err != nil {
		t.Fatalf("Failed to get new height: %v", err)
	}

	if newHeight <= initialHeight {
		t.Errorf("Height should have increased: %d -> %d", initialHeight, newHeight)
	}
}

// TestUS009_UpWithCustomBinary tests starting with a cached binary reference
func TestUS009_UpWithCustomBinary(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	// This test requires a binary in the cache
	binaryRef := os.Getenv("DEVNET_BINARY_REF")
	if binaryRef == "" {
		t.Skip("DEVNET_BINARY_REF not set, skipping custom binary test")
	}

	config := DefaultTestConfig()
	config.Mode = "local"
	runner := NewTestRunner(t, config)
	defer runner.Cleanup()

	// Initialize devnet (don't start)
	ctx, cancel := context.WithTimeout(context.Background(), deployTimeout)
	defer cancel()

	_, _, err := runner.Run(ctx,
		"init",
		"--mode", "local",
		"--validators", "1",
		"--no-interactive",
	)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Start with custom binary reference
	upCtx, upCancel := context.WithTimeout(context.Background(), healthTimeout)
	defer upCancel()

	_, _, err = runner.Run(upCtx,
		"up",
		"--binary-ref", binaryRef,
	)
	if err != nil {
		t.Fatalf("Up with binary-ref failed: %v", err)
	}

	// Wait for healthy
	if err := runner.WaitForHealthy(healthTimeout); err != nil {
		t.Fatalf("Node did not become healthy: %v", err)
	}

	// Verify blocks are being produced
	if err := runner.WaitForBlocks(3, time.Minute); err != nil {
		t.Errorf("Blocks not being produced with custom binary: %v", err)
	}
}

// TestUpgrade_GenesisExport tests upgrade with genesis export before/after
func TestUpgrade_GenesisExport(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E upgrade test in short mode")
	}

	upgradeImage := os.Getenv("DEVNET_UPGRADE_IMAGE")
	if upgradeImage == "" {
		t.Skip("DEVNET_UPGRADE_IMAGE not set, skipping genesis export upgrade test")
	}

	config := DefaultTestConfig()
	runner := NewTestRunner(t, config)
	defer runner.Cleanup()

	// Create genesis export directory
	genesisDir := filepath.Join(config.HomeDir, "genesis-exports")
	if err := os.MkdirAll(genesisDir, 0755); err != nil {
		t.Fatalf("Failed to create genesis dir: %v", err)
	}

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

	// Upgrade with genesis export
	upgradeCtx, upgradeCancel := context.WithTimeout(context.Background(), upgradeTimeout)
	defer upgradeCancel()

	_, _, err = runner.Run(upgradeCtx,
		"upgrade",
		"--name", "genesis-export-upgrade",
		"--image", upgradeImage,
		"--export-genesis",
		"--genesis-dir", genesisDir,
		"--voting-period", upgradeVotingPeriod,
		"--no-interactive",
	)
	if err != nil {
		t.Fatalf("Upgrade with genesis export failed: %v", err)
	}

	// Verify genesis files were created
	preUpgradeGenesis := filepath.Join(genesisDir, "pre-upgrade-genesis.json")
	postUpgradeGenesis := filepath.Join(genesisDir, "post-upgrade-genesis.json")

	if _, err := os.Stat(preUpgradeGenesis); os.IsNotExist(err) {
		t.Error("Pre-upgrade genesis file not created")
	}
	if _, err := os.Stat(postUpgradeGenesis); os.IsNotExist(err) {
		t.Error("Post-upgrade genesis file not created")
	}

	// Verify genesis files are valid JSON
	for _, path := range []string{preUpgradeGenesis, postUpgradeGenesis} {
		if _, err := os.Stat(path); err == nil {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Errorf("Failed to read %s: %v", path, err)
				continue
			}
			var genesis map[string]interface{}
			if err := json.Unmarshal(data, &genesis); err != nil {
				t.Errorf("Invalid JSON in %s: %v", path, err)
			}
		}
	}
}

// TestUpgrade_MultiValidator tests upgrade with multiple validators
func TestUpgrade_MultiValidator(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E multi-validator upgrade test in short mode")
	}

	upgradeImage := os.Getenv("DEVNET_UPGRADE_IMAGE")
	if upgradeImage == "" {
		t.Skip("DEVNET_UPGRADE_IMAGE not set, skipping multi-validator upgrade test")
	}

	config := DefaultTestConfig()
	config.Validators = 4
	runner := NewTestRunner(t, config)
	defer runner.Cleanup()

	// Deploy 4 validators
	ctx, cancel := context.WithTimeout(context.Background(), deployTimeout)
	defer cancel()

	_, _, err := runner.Run(ctx, "deploy", "--validators", "4", "--no-interactive")
	if err != nil {
		t.Fatalf("Deploy failed: %v", err)
	}

	if err := runner.WaitForHealthy(healthTimeout); err != nil {
		t.Fatalf("Nodes did not become healthy: %v", err)
	}

	// Verify all 4 validators are running
	statusOut := runner.MustRun("status", "--json")
	var status StatusOutput
	if err := json.Unmarshal([]byte(statusOut), &status); err != nil {
		t.Fatalf("Failed to parse status: %v", err)
	}
	if len(status.Nodes) != 4 {
		t.Fatalf("Expected 4 nodes, got %d", len(status.Nodes))
	}

	// Perform upgrade
	upgradeCtx, upgradeCancel := context.WithTimeout(context.Background(), upgradeTimeout)
	defer upgradeCancel()

	_, _, err = runner.Run(upgradeCtx,
		"upgrade",
		"--name", "multi-validator-upgrade",
		"--image", upgradeImage,
		"--voting-period", upgradeVotingPeriod,
		"--no-interactive",
	)
	if err != nil {
		t.Fatalf("Multi-validator upgrade failed: %v", err)
	}

	// Wait for all nodes to be healthy after upgrade
	if err := runner.WaitForHealthy(healthTimeout); err != nil {
		t.Fatalf("Nodes did not become healthy after upgrade: %v", err)
	}

	// Verify all 4 nodes are still running
	statusOut = runner.MustRun("status", "--json")
	if err := json.Unmarshal([]byte(statusOut), &status); err != nil {
		t.Fatalf("Failed to parse status after upgrade: %v", err)
	}
	if len(status.Nodes) != 4 {
		t.Errorf("Expected 4 nodes after upgrade, got %d", len(status.Nodes))
	}

	// Verify consensus is working (blocks being produced)
	if err := runner.WaitForBlocks(5, time.Minute); err != nil {
		t.Errorf("Consensus broken after multi-validator upgrade: %v", err)
	}

	// Verify all nodes have similar heights (synced)
	var heights []int64
	for _, node := range status.Nodes {
		heights = append(heights, node.Height)
	}
	maxDiff := int64(0)
	for i := 1; i < len(heights); i++ {
		diff := heights[0] - heights[i]
		if diff < 0 {
			diff = -diff
		}
		if diff > maxDiff {
			maxDiff = diff
		}
	}
	if maxDiff > 5 {
		t.Errorf("Nodes are not synced, max height difference: %d", maxDiff)
	}
}

// TestUpgrade_Interruption tests upgrade recovery after interruption
func TestUpgrade_Interruption(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E upgrade interruption test in short mode")
	}

	upgradeImage := os.Getenv("DEVNET_UPGRADE_IMAGE")
	if upgradeImage == "" {
		t.Skip("DEVNET_UPGRADE_IMAGE not set, skipping interruption test")
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

	// Start upgrade with very short timeout (will timeout during voting)
	shortCtx, shortCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shortCancel()

	// This should timeout/fail during voting period
	runner.Run(shortCtx,
		"upgrade",
		"--name", "interrupted-upgrade",
		"--image", upgradeImage,
		"--voting-period", "300s", // Long voting period to ensure we timeout
		"--no-interactive",
	)

	// Node should still be running after interrupted upgrade
	if err := runner.WaitForHealthy(30 * time.Second); err != nil {
		// If nodes aren't healthy, try to restart
		runner.Run(context.Background(), "down")
		time.Sleep(2 * time.Second)
		runner.RunWithTimeout(healthTimeout, "up")

		if err := runner.WaitForHealthy(healthTimeout); err != nil {
			t.Errorf("Node did not recover after interrupted upgrade: %v", err)
		}
	}

	// Verify chain is still functional
	if err := runner.WaitForBlocks(3, time.Minute); err != nil {
		t.Errorf("Chain not functional after interrupted upgrade: %v", err)
	}
}
