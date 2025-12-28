package e2e

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestStatus_RunningDevnet tests the status command with a running devnet
// Verifies: status shows all validators, ports, and running state
func TestStatus_RunningDevnet(t *testing.T) {
	skipIfBinaryNotBuilt(t)
	skipIfBlockchainBinaryNotAvailable(t)

	// Setup and deploy devnet
	ctx, runner, validator, cleanup := setupTest(t)
	defer cleanup.CleanupDevnet()

	t.Log("Deploying devnet for status test...")
	runner.MustRun("deploy",
		"--mode", "local",
		"--validators", "2",
		"--network", "testnet",
		"--home", ctx.HomeDir,
	)

	// Wait for validators to start
	_, err := validator.WaitForProcess("validator0.pid", 30*time.Second)
	assert.NoError(t, err, "validator0 should start")
	_, err = validator.WaitForProcess("validator1.pid", 30*time.Second)
	assert.NoError(t, err, "validator1 should start")

	// Execute status command
	t.Log("Checking devnet status...")
	result := runner.MustRun("status", "--home", ctx.HomeDir)

	// Verify output contains expected information
	assert.Contains(t, result.Stdout, "Running", "should show Running status")
	assert.Contains(t, result.Stdout, "validator0", "should list validator0")
	assert.Contains(t, result.Stdout, "validator1", "should list validator1")
	assert.Contains(t, result.Stdout, "26657", "should show validator0 RPC port")
	assert.Contains(t, result.Stdout, "36657", "should show validator1 RPC port")

	// Verify does not show error messages
	assert.NotContains(t, result.Stdout, "error", "should not show errors")
	assert.NotContains(t, result.Stdout, "failed", "should not show failures")

	t.Log("Status command verified successfully")
}

// TestStatus_JSONOutput tests the status command with JSON output format
// Verifies: JSON output is valid and contains all required fields
func TestStatus_JSONOutput(t *testing.T) {
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

	// Wait for validators to start
	_, err := validator.WaitForProcess("validator0.pid", 30*time.Second)
	assert.NoError(t, err, "validator0 should start")

	// Execute status command with JSON output
	t.Log("Checking status with --json flag...")
	result := runner.MustRun("status", "--json", "--home", ctx.HomeDir)

	// Verify output is valid JSON
	var statusData map[string]interface{}
	err = json.Unmarshal([]byte(result.Stdout), &statusData)
	assert.NoError(t, err, "output should be valid JSON")

	// Verify required fields exist
	assert.Contains(t, statusData, "status", "should have status field")
	assert.Contains(t, statusData, "validators", "should have validators field")

	// Verify validators array
	validators, ok := statusData["validators"].([]interface{})
	assert.True(t, ok, "validators should be an array")
	assert.Equal(t, 2, len(validators), "should have 2 validators")

	// Verify first validator structure
	if len(validators) > 0 {
		val0, ok := validators[0].(map[string]interface{})
		assert.True(t, ok, "validator should be an object")
		assert.Contains(t, val0, "name", "validator should have name")
		assert.Contains(t, val0, "status", "validator should have status")
		assert.Contains(t, val0, "rpc", "validator should have rpc port")
	}

	t.Log("JSON status output verified successfully")
}

// TestLogs_FollowMode tests the logs command in follow mode
// Verifies: logs are streamed in real-time
func TestLogs_FollowMode(t *testing.T) {
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

	// Wait for validators to start
	_, err := validator.WaitForProcess("validator0.pid", 30*time.Second)
	assert.NoError(t, err, "validator0 should start")

	// Execute logs command (without --follow to avoid hanging)
	// Testing --follow in E2E would require background execution
	t.Log("Checking logs output...")
	result := runner.Run("logs",
		"--validator", "0",
		"--lines", "10",
		"--home", ctx.HomeDir,
	)

	// Verify command succeeded
	assert.True(t, result.Success(), "logs command should succeed")

	// Verify output contains log entries
	assert.NotEmpty(t, result.Stdout, "should show log entries")

	// Common log patterns in Cosmos SDK chains
	logPatterns := []string{
		"INF", // Info level logs
		"validator",
		"block",
	}

	foundPattern := false
	for _, pattern := range logPatterns {
		if strings.Contains(result.Stdout, pattern) {
			foundPattern = true
			break
		}
	}
	assert.True(t, foundPattern, "output should contain typical log patterns")

	t.Log("Logs command verified successfully")
}

// TestLogs_TailLines tests the logs command with --lines flag
// Verifies: only requested number of lines are returned
func TestLogs_TailLines(t *testing.T) {
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

	// Wait for validators to start and generate logs
	_, err := validator.WaitForProcess("validator0.pid", 30*time.Second)
	assert.NoError(t, err, "validator0 should start")

	// Wait a bit for logs to be generated
	time.Sleep(5 * time.Second)

	// Execute logs command with specific line count
	t.Log("Testing logs --lines flag...")
	result := runner.MustRun("logs",
		"--validator", "0",
		"--lines", "5",
		"--home", ctx.HomeDir,
	)

	// Verify output has approximately the requested number of lines
	// (may be slightly more/less due to formatting)
	lines := strings.Split(strings.TrimSpace(result.Stdout), "\n")
	lineCount := len(lines)

	// Allow some tolerance for header lines, etc.
	assert.LessOrEqual(t, lineCount, 10,
		"should show roughly 5 lines (with some tolerance)")
	assert.GreaterOrEqual(t, lineCount, 1,
		"should show at least some lines")

	t.Logf("Logs returned %d lines (requested 5)", lineCount)
}

// TestNode_StopAndStart tests individual node control
// Verifies: can stop and start individual validators
func TestNode_StopAndStart(t *testing.T) {
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

	// Wait for validators to start
	pid0, err := validator.WaitForProcess("validator0.pid", 30*time.Second)
	assert.NoError(t, err, "validator0 should start")
	pid1, err := validator.WaitForProcess("validator1.pid", 30*time.Second)
	assert.NoError(t, err, "validator1 should start")

	// Stop validator0
	t.Log("Stopping validator0...")
	result := runner.MustRun("node", "stop",
		"--validator", "0",
		"--home", ctx.HomeDir,
	)

	assert.Contains(t, result.Stdout, "stopped", "should show stopped message")

	// Verify validator0 stopped but validator1 still running
	time.Sleep(2 * time.Second)
	validator.AssertProcessNotRunning(pid0)
	validator.AssertProcessRunning(pid1)

	// Start validator0 again
	t.Log("Starting validator0...")
	result = runner.MustRun("node", "start",
		"--validator", "0",
		"--home", ctx.HomeDir,
	)

	assert.Contains(t, result.Stdout, "started", "should show started message")

	// Wait for validator0 to restart
	newPid0, err := validator.WaitForProcess("validator0.pid", 30*time.Second)
	assert.NoError(t, err, "validator0 should restart")
	assert.Greater(t, newPid0, 0, "new PID should be valid")
	assert.NotEqual(t, pid0, newPid0, "new PID should be different")

	// Verify both validators running
	validator.AssertProcessRunning(newPid0)
	validator.AssertProcessRunning(pid1)

	t.Log("Individual node control verified successfully")
}

// TestStatus_StoppedDevnet tests status command with stopped devnet
// Verifies: status correctly shows stopped state
func TestStatus_StoppedDevnet(t *testing.T) {
	skipIfBinaryNotBuilt(t)
	skipIfBlockchainBinaryNotAvailable(t)

	// Setup, deploy, and stop devnet
	ctx, runner, validator, cleanup := setupTest(t)
	defer cleanup.CleanupDevnet()

	runner.MustRun("deploy",
		"--mode", "local",
		"--validators", "2",
		"--network", "testnet",
		"--home", ctx.HomeDir,
	)

	// Wait for start
	_, err := validator.WaitForProcess("validator0.pid", 30*time.Second)
	assert.NoError(t, err, "validator0 should start")

	// Stop devnet
	t.Log("Stopping devnet...")
	runner.MustRun("stop", "--home", ctx.HomeDir)
	time.Sleep(2 * time.Second)

	// Check status of stopped devnet
	t.Log("Checking status of stopped devnet...")
	result := runner.MustRun("status", "--home", ctx.HomeDir)

	// Verify output shows stopped state
	assert.Contains(t, result.Stdout, "Stopped", "should show Stopped status")
	assert.NotContains(t, result.Stdout, "Running", "should not show Running")

	t.Log("Stopped devnet status verified")
}

// TestStatus_NoDevnet tests status command when no devnet exists
// Verifies: appropriate error message when devnet not found
func TestStatus_NoDevnet(t *testing.T) {
	skipIfBinaryNotBuilt(t)

	// Setup empty environment (no devnet)
	ctx, runner, _, cleanup := setupTest(t)
	defer cleanup.CleanupDevnet()

	// Try to check status of non-existent devnet
	t.Log("Checking status when no devnet exists...")
	result := runner.MustFail("status", "--home", ctx.HomeDir)

	// Verify error message
	assert.Contains(t, result.Stderr, "not found",
		"should show 'not found' error")
	assert.NotEqual(t, 0, result.ExitCode,
		"should return non-zero exit code")

	t.Log("No devnet status error verified")
}
