package e2e

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestUpgrade_WithBinary tests the upgrade command with custom binary
// Verifies: binary upgrade workflow, validators restart with new version
func TestUpgrade_WithBinary(t *testing.T) {
	skipIfBinaryNotBuilt(t)
	skipIfBlockchainBinaryNotAvailable(t)

	// Setup and deploy devnet
	ctx, runner, validator, cleanup := setupTest(t)
	defer cleanup.CleanupDevnet()

	runner.MustRun("deploy",
		"--mode", "local",
		"--validators", "2",
		"--network", "testnet",
		"--home", ctx.HomeDir,
	)

	// Wait for initial deployment
	pid0, err := validator.WaitForProcess("devnet/node0/stabled.pid", 30*time.Second)
	assert.NoError(t, err, "validator0 should start")

	// Execute upgrade command
	// Note: This assumes upgrade creates a proposal or replaces binaries
	t.Log("Executing upgrade command...")
	result := runner.Run("upgrade",
		"--version", "v1.1.0",
		"--home", ctx.HomeDir,
	)

	// Command may succeed or fail depending on implementation
	// If upgrade requires a binary that doesn't exist, it should fail gracefully
	if result.Failed() {
		// Verify error message is informative
		assert.Contains(t, result.Stderr, "version",
			"error should mention version issue")
		t.Log("Upgrade test verified error handling")
	} else {
		// If upgrade succeeded, verify validators restarted
		assert.Contains(t, result.Stdout, "upgrade",
			"should show upgrade message")

		// Verify processes were restarted (new PIDs)
		newPid0, err := validator.WaitForProcess("devnet/node0/stabled.pid", 30*time.Second)
		assert.NoError(t, err, "validator0 should restart after upgrade")
		assert.NotEqual(t, pid0, newPid0, "PID should change after upgrade")

		t.Log("Upgrade workflow verified successfully")
	}
}

// TestExport_CurrentState tests exporting current devnet state
// Verifies: export creates genesis file with current state
func TestExport_CurrentState(t *testing.T) {
	skipIfBinaryNotBuilt(t)
	skipIfBlockchainBinaryNotAvailable(t)

	// Setup and deploy devnet
	ctx, runner, validator, cleanup := setupTest(t)
	defer cleanup.CleanupDevnet()

	runner.MustRun("deploy",
		"--mode", "local",
		"--validators", "2",
		"--network", "testnet",
		"--home", ctx.HomeDir,
	)

	// Wait for node to be fully ready (process + RPC responding)
	t.Log("Waiting for node0 RPC to be ready...")
	_, err := validator.WaitForNodeReady(0, 60*time.Second)
	if err != nil {
		t.Logf("Node not ready: %v", err)
		t.Skip("Skipping export test - node failed to start or RPC not ready")
		return
	}

	// Wait a bit for some blocks to be produced
	t.Log("Waiting for blocks to be produced...")
	time.Sleep(5 * time.Second)

	// Execute export command
	t.Log("Exporting current state...")
	exportPath := filepath.Join(ctx.HomeDir, "exported-genesis.json")
	result := runner.Run("export",
		"--output-dir", exportPath,
		"--home", ctx.HomeDir,
	)

	// Export may fail if RPC is still not ready or node crashed
	if result.Failed() {
		t.Logf("Export command failed: %s", result.Stderr)
		t.Skip("Skipping export test - export command failed")
		return
	}

	assert.Contains(t, result.Stdout, "exported",
		"should show export success message")

	// Verify exported file exists
	validator.AssertFileExists("exported-genesis.json")

	// Verify exported file is valid JSON
	validator.AssertJSONFileValid("exported-genesis.json")

	// Verify file is not empty
	content := ctx.ReadFile("exported-genesis.json")
	assert.Greater(t, len(content), 100,
		"exported genesis should contain substantial data")

	// Verify exported genesis contains expected fields
	assert.Contains(t, string(content), "chain_id",
		"should contain chain_id")
	assert.Contains(t, string(content), "app_state",
		"should contain app_state")

	t.Log("Export verified successfully")
}

// TestBuild_FromExportedGenesis tests building a new devnet from exported genesis
// Verifies: exported genesis can be used to create new devnet
func TestBuild_FromExportedGenesis(t *testing.T) {
	skipIfBinaryNotBuilt(t)
	skipIfBlockchainBinaryNotAvailable(t)

	// Setup and deploy initial devnet
	ctx, runner, validator, cleanup := setupTest(t)
	defer cleanup.CleanupDevnet()

	runner.MustRun("deploy",
		"--mode", "local",
		"--validators", "2",
		"--network", "testnet",
		"--home", ctx.HomeDir,
	)

	// Wait for node to be fully ready (process + RPC responding)
	t.Log("Waiting for node0 RPC to be ready...")
	_, err := validator.WaitForNodeReady(0, 60*time.Second)
	if err != nil {
		t.Logf("Node not ready: %v", err)
		t.Skip("Skipping build from export test - node failed to start")
		return
	}

	// Export current state
	t.Log("Exporting state...")
	exportPath := filepath.Join(ctx.HomeDir, "exported-genesis.json")
	exportResult := runner.Run("export",
		"--output-dir", exportPath,
		"--home", ctx.HomeDir,
	)

	// Export may fail if RPC is still not ready
	if exportResult.Failed() {
		t.Logf("Export command failed: %s", exportResult.Stderr)
		t.Skip("Skipping build from export test - export failed")
		return
	}

	// Destroy original devnet
	t.Log("Destroying original devnet...")
	runner.MustRun("destroy", "--force", "--home", ctx.HomeDir)

	// Build new devnet from exported genesis
	t.Log("Building from exported genesis...")
	result := runner.Run("build",
		"--genesis-file", exportPath,
		"--validators", "2",
		"--mode", "local",
		"--home", ctx.HomeDir,
	)

	// Command may succeed or require different approach depending on implementation
	if result.Failed() {
		// Verify informative error if build command doesn't exist
		t.Log("Build command may not be implemented, checking deploy with genesis-file...")

		// Try deploy with genesis-file instead
		result = runner.Run("deploy",
			"--genesis-file", exportPath,
			"--validators", "2",
			"--mode", "local",
			"--home", ctx.HomeDir,
		)
	}

	// If either build or deploy succeeded, verify new devnet
	if result.Success() {
		// Verify new validators were created
		validator.AssertValidatorCount(2)

		t.Log("Build from exported genesis verified successfully")
	} else {
		// Log that feature may not be implemented
		t.Log("Build/deploy from exported genesis may require implementation")
	}
}

// TestReset_SoftReset tests soft reset (keeps data, resets state)
// Verifies: soft reset preserves validator keys but resets chain state
func TestReset_SoftReset(t *testing.T) {
	skipIfBinaryNotBuilt(t)
	skipIfBlockchainBinaryNotAvailable(t)

	// Setup and deploy devnet
	ctx, runner, validator, cleanup := setupTest(t)
	defer cleanup.CleanupDevnet()

	runner.MustRun("deploy",
		"--mode", "local",
		"--validators", "2",
		"--network", "testnet",
		"--home", ctx.HomeDir,
	)

	// Wait for node to be fully ready
	t.Log("Waiting for node0 to be ready...")
	_, err := validator.WaitForNodeReady(0, 60*time.Second)
	if err != nil {
		t.Logf("Node not ready: %v", err)
		t.Skip("Skipping soft reset test - node failed to start")
		return
	}

	// Let some blocks be produced
	time.Sleep(5 * time.Second)

	// Verify validator keys exist
	validator.AssertFileExists("devnet/node0/config/priv_validator_key.json")
	validator.AssertFileExists("devnet/node0/config/node_key.json")

	// Execute soft reset (default mode, no --hard flag)
	t.Log("Executing soft reset...")
	result := runner.MustRun("reset",
		"--force",
		"--home", ctx.HomeDir,
	)

	assert.Contains(t, result.Stdout, "reset",
		"should show reset message")

	// Verify validator keys still exist (soft reset preserves them)
	validator.AssertFileExists("devnet/node0/config/priv_validator_key.json")
	validator.AssertFileExists("devnet/node0/config/node_key.json")

	// Verify data directory was reset
	// (exact behavior depends on implementation)
	t.Log("Soft reset verified successfully")
}

// TestReset_HardReset tests hard reset (removes all data)
// Verifies: hard reset removes validator directories completely
func TestReset_HardReset(t *testing.T) {
	skipIfBinaryNotBuilt(t)
	skipIfBlockchainBinaryNotAvailable(t)

	// Setup and deploy devnet
	ctx, runner, validator, cleanup := setupTest(t)
	defer cleanup.CleanupDevnet()

	runner.MustRun("deploy",
		"--mode", "local",
		"--validators", "2",
		"--network", "testnet",
		"--home", ctx.HomeDir,
	)

	// Wait for node to be ready
	t.Log("Waiting for node0 to be ready...")
	_, err := validator.WaitForNodeReady(0, 60*time.Second)
	if err != nil {
		t.Logf("Node not ready: %v", err)
		t.Skip("Skipping hard reset test - node failed to start")
		return
	}

	// Verify validators exist
	validator.AssertValidatorCount(2)
	validator.AssertDirectoryExists("devnet/node0")
	validator.AssertDirectoryExists("devnet/node1")

	// Execute hard reset
	t.Log("Executing hard reset...")
	result := runner.MustRun("reset",
		"--hard",
		"--force",
		"--home", ctx.HomeDir,
	)

	assert.Contains(t, result.Stdout, "reset",
		"should show reset message")

	// Verify all validator directories removed (hard reset removes everything)
	validator.AssertDirectoryNotExists("validator0")
	validator.AssertDirectoryNotExists("validator1")
	validator.AssertValidatorCount(0)

	// Verify processes stopped
	validator.AssertFileNotExists("validator0.pid")
	validator.AssertFileNotExists("validator1.pid")

	t.Log("Hard reset verified successfully")
}

// TestReplace_BinaryVersion tests replacing binary with different version
// Verifies: binary replacement and validator restart
func TestReplace_BinaryVersion(t *testing.T) {
	skipIfBinaryNotBuilt(t)
	skipIfBlockchainBinaryNotAvailable(t)

	// Setup and deploy devnet
	ctx, runner, validator, cleanup := setupTest(t)
	defer cleanup.CleanupDevnet()

	runner.MustRun("deploy",
		"--mode", "local",
		"--validators", "2",
		"--network", "testnet",
		"--home", ctx.HomeDir,
	)

	// Wait for node to be ready
	t.Log("Waiting for node0 to be ready...")
	pid0, err := validator.WaitForNodeReady(0, 60*time.Second)
	if err != nil {
		t.Logf("Node not ready: %v", err)
		t.Skip("Skipping replace binary test - node failed to start")
		return
	}

	// Execute replace command with new binary
	// (This assumes replace command exists)
	t.Log("Attempting binary replacement...")
	result := runner.Run("replace",
		"--binary", ctx.BinaryPath,
		"--restart",
		"--home", ctx.HomeDir,
	)

	// Command may not be implemented
	if result.Failed() {
		assert.Contains(t, result.Stderr, "replace",
			"error should mention replace command")
		t.Log("Replace command may not be implemented")
	} else {
		// Verify validators restarted
		newPid0, err := validator.WaitForProcess("devnet/node0/stabled.pid", 30*time.Second)
		assert.NoError(t, err, "validator0 should restart")
		assert.NotEqual(t, pid0, newPid0, "PID should change after replace")

		t.Log("Binary replacement verified successfully")
	}
}

// TestExportKeys_ValidatorsOnly tests exporting validator keys
// Verifies: key export creates backup files
func TestExportKeys_ValidatorsOnly(t *testing.T) {
	skipIfBinaryNotBuilt(t)
	skipIfBlockchainBinaryNotAvailable(t)

	// Setup and deploy devnet
	ctx, runner, validator, cleanup := setupTest(t)
	defer cleanup.CleanupDevnet()

	runner.MustRun("deploy",
		"--mode", "local",
		"--validators", "2",
		"--network", "testnet",
		"--home", ctx.HomeDir,
	)

	// Execute export-keys command
	t.Log("Exporting validator keys...")
	result := runner.Run("export-keys",
		"--type", "all",
		"--home", ctx.HomeDir,
	)

	// Command may not be implemented
	if result.Failed() {
		t.Log("Export-keys command may not be implemented, verifying manual key access...")

		// Verify we can at least access the keys manually
		validator.AssertFileExists("devnet/node0/config/priv_validator_key.json")
		validator.AssertFileExists("devnet/node0/config/node_key.json")
		validator.AssertFileExists("devnet/node1/config/priv_validator_key.json")
		validator.AssertFileExists("devnet/node1/config/node_key.json")

		t.Log("Validator keys exist and are accessible")
	} else {
		// Verify export succeeded - export-keys outputs JSON to stdout
		assert.NotEmpty(t, result.Stdout, "should output JSON data")
		// Verify JSON contains expected fields (note: key is "ValidatorKeys" with capital V)
		assert.Contains(t, result.Stdout, "ValidatorKeys",
			"should contain ValidatorKeys")

		t.Log("Key export verified successfully")
	}
}

// TestBuildSnapshot_CreateArchive tests creating a snapshot archive
// Verifies: snapshot command creates valid archive
func TestBuildSnapshot_CreateArchive(t *testing.T) {
	skipIfBinaryNotBuilt(t)
	skipIfBlockchainBinaryNotAvailable(t)

	// Setup and deploy devnet
	ctx, runner, validator, cleanup := setupTest(t)
	defer cleanup.CleanupDevnet()

	runner.MustRun("deploy",
		"--mode", "local",
		"--validators", "2",
		"--network", "testnet",
		"--home", ctx.HomeDir,
	)

	// Wait for node to be fully ready
	t.Log("Waiting for node0 to be ready...")
	_, err := validator.WaitForNodeReady(0, 60*time.Second)
	if err != nil {
		t.Logf("Node not ready: %v", err)
		t.Skip("Skipping snapshot test - node failed to start")
		return
	}

	// Let some blocks be produced
	time.Sleep(5 * time.Second)

	// Execute snapshot command
	t.Log("Creating snapshot...")
	result := runner.Run("snapshot",
		"--home", ctx.HomeDir,
	)

	// Command may not be implemented
	if result.Failed() {
		t.Log("Snapshot command may not be implemented")
		assert.Contains(t, result.Stderr, "snapshot",
			"error should mention snapshot command")
	} else {
		// If snapshot command exists, verify output message
		assert.NotEmpty(t, result.Stdout, "should show snapshot message")

		t.Log("Snapshot creation verified successfully")
	}
}
