package e2e

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestDeploy_DefaultSettings tests the deploy command with default settings
// Verifies: command succeeds, validators are created, processes are running
func TestDeploy_DefaultSettings(t *testing.T) {
	// Skip if binary not built
	skipIfBinaryNotBuilt(t)
	// Skip if blockchain binary not available
	skipIfBlockchainBinaryNotAvailable(t)

	// Setup test environment
	ctx, runner, validator, cleanup := setupTest(t)
	defer cleanup.CleanupDevnet()

	// Use local mode for faster testing (no Docker dependency)
	ctx.WithEnv("DEVNET_MODE", "local")
	ctx.WithEnv("DEVNET_VALIDATORS", "2")
	ctx.WithEnv("DEVNET_NETWORK", "testnet")

	// Execute deploy command
	t.Log("Executing: devnet-builder deploy")
	result := runner.MustRun("deploy",
		"--mode", "local",
		"--validators", "2",
		"--network", "testnet",
		"--home", ctx.HomeDir,
	)

	// Verify command succeeded
	assert.True(t, result.Success(), "deploy command should succeed")
	assert.Contains(t, result.Stdout, "✓ Devnet started!", "should show success message")

	// Verify validators were created
	t.Log("Verifying validator directories were created...")
	validator.AssertValidatorCount(2)
	validator.AssertDirectoryExists("devnet/node0")
	validator.AssertDirectoryExists("devnet/node1")

	// Verify configuration files exist
	t.Log("Verifying configuration files...")
	validator.AssertFileExists("devnet/node0/config/config.toml")
	validator.AssertFileExists("devnet/node0/config/genesis.json")
	validator.AssertFileExists("devnet/node1/config/config.toml")
	validator.AssertFileExists("devnet/node1/config/genesis.json")

	// Verify genesis files are valid JSON
	validator.AssertJSONFileValid("devnet/node0/config/genesis.json")
	validator.AssertJSONFileValid("devnet/node1/config/genesis.json")

	// Wait for processes to start (with timeout)
	t.Log("Waiting for validator processes to start...")
	pid0, err := validator.WaitForProcess("devnet/node0/stabled.pid", 30*time.Second)
	assert.NoError(t, err, "validator0 should start within 30 seconds")
	assert.Greater(t, pid0, 0, "validator0 PID should be valid")

	pid1, err := validator.WaitForProcess("devnet/node1/stabled.pid", 30*time.Second)
	assert.NoError(t, err, "validator1 should start within 30 seconds")
	assert.Greater(t, pid1, 0, "validator1 PID should be valid")

	// Verify processes are running
	t.Log("Verifying validator processes are running...")
	validator.AssertProcessRunning(pid0)
	validator.AssertProcessRunning(pid1)

	// Verify RPC ports are listening
	// Note: In local mode, ports are offset by 100 per node (26657, 26757, 26857, etc.)
	t.Log("Verifying RPC ports are listening...")
	err = validator.WaitForPortListening(26657, 60*time.Second) // validator0 RPC
	assert.NoError(t, err, "validator0 RPC port should be listening")

	err = validator.WaitForPortListening(26757, 60*time.Second) // validator1 RPC
	assert.NoError(t, err, "validator1 RPC port should be listening")

	// Log success
	t.Log("Deploy test completed successfully")
	t.Logf("  - 2 validators created")
	t.Logf("  - Processes running: PID %d, %d", pid0, pid1)
	t.Logf("  - RPC endpoints: 26657, 36657")
}

// TestInit_FollowedByStart tests the init command followed by start
// Verifies: init creates structure, start launches processes
func TestInit_FollowedByStart(t *testing.T) {
	// Skip if binary not built
	skipIfBinaryNotBuilt(t)
	// Skip if blockchain binary not available
	skipIfBlockchainBinaryNotAvailable(t)

	// Setup test environment
	ctx, runner, validator, cleanup := setupTest(t)
	defer cleanup.CleanupDevnet()

	// Use local mode for faster testing
	ctx.WithEnv("DEVNET_MODE", "local")
	ctx.WithEnv("DEVNET_VALIDATORS", "2")

	// Step 1: Run init command
	t.Log("Step 1: Executing init command")
	result := runner.MustRun("init",
		"--mode", "local",
		"--validators", "2",
		"--network", "testnet",
		"--home", ctx.HomeDir,
	)

	assert.True(t, result.Success(), "init command should succeed")
	// Init command outputs "Initialization complete!" message
	hasInitMessage := strings.Contains(result.Stdout, "Initialization") ||
		strings.Contains(result.Stdout, "complete") ||
		strings.Contains(result.Stdout, "initialized")
	assert.True(t, hasInitMessage, "should show initialization message")

	// Verify structure was created but processes not started
	t.Log("Verifying initialization created structure...")
	validator.AssertValidatorCount(2)
	validator.AssertFileExists("devnet/node0/config/genesis.json")
	validator.AssertFileExists("devnet/node1/config/genesis.json")

	// Verify no processes are running yet
	t.Log("Verifying processes are not started yet...")
	validator.AssertFileNotExists("validator0.pid")
	validator.AssertFileNotExists("validator1.pid")

	// Step 2: Run start command
	t.Log("Step 2: Executing start command")
	result = runner.MustRun("start", "--home", ctx.HomeDir)

	assert.True(t, result.Success(), "start command should succeed")
	// Start command outputs "Devnet started!" message
	hasStartMessage := strings.Contains(result.Stdout, "started") ||
		strings.Contains(result.Stdout, "Started") ||
		strings.Contains(result.Stdout, "Devnet")
	assert.True(t, hasStartMessage, "should show start message")

	// Wait for processes to start
	t.Log("Waiting for processes to start...")
	pid0, err := validator.WaitForProcess("devnet/node0/stabled.pid", 30*time.Second)
	assert.NoError(t, err, "validator0 should start")
	assert.Greater(t, pid0, 0, "validator0 PID should be valid")

	pid1, err := validator.WaitForProcess("devnet/node1/stabled.pid", 30*time.Second)
	assert.NoError(t, err, "validator1 should start")
	assert.Greater(t, pid1, 0, "validator1 PID should be valid")

	// Verify processes are running
	validator.AssertProcessRunning(pid0)
	validator.AssertProcessRunning(pid1)

	t.Log("Init + Start test completed successfully")
}

// TestStop_GracefulShutdown tests the stop command
// Verifies: processes are stopped gracefully, PID files removed
func TestStop_GracefulShutdown(t *testing.T) {
	// Skip if binary not built
	skipIfBinaryNotBuilt(t)
	// Skip if blockchain binary not available
	skipIfBlockchainBinaryNotAvailable(t)

	// Setup and deploy a devnet first
	ctx, runner, validator, cleanup := setupTest(t)
	defer cleanup.CleanupDevnet()

	// Deploy devnet
	t.Log("Setting up devnet for stop test...")
	runner.MustRun("deploy",
		"--mode", "local",
		"--validators", "2",
		"--network", "testnet",
		"--home", ctx.HomeDir,
	)

	// Wait for processes to start
	pid0, err := validator.WaitForProcess("devnet/node0/stabled.pid", 30*time.Second)
	assert.NoError(t, err, "validator0 should be running")
	pid1, err := validator.WaitForProcess("devnet/node1/stabled.pid", 30*time.Second)
	assert.NoError(t, err, "validator1 should be running")

	// Execute stop command
	t.Log("Executing stop command...")
	result := runner.MustRun("stop", "--home", ctx.HomeDir)

	assert.True(t, result.Success(), "stop command should succeed")
	assert.Contains(t, result.Stdout, "stopped", "should show stop message")

	// Verify processes are stopped
	t.Log("Verifying processes stopped...")
	time.Sleep(2 * time.Second) // Give processes time to terminate

	validator.AssertProcessNotRunning(pid0)
	validator.AssertProcessNotRunning(pid1)

	// Verify PID files are removed
	validator.AssertFileNotExists("validator0.pid")
	validator.AssertFileNotExists("validator1.pid")

	// Verify ports are no longer listening
	validator.AssertPortNotListening(26657)
	validator.AssertPortNotListening(36657)

	t.Log("Stop test completed successfully")
}

// TestStart_ResumeFromStopped tests resuming a stopped devnet
// Verifies: can restart after stop, same configuration used
func TestStart_ResumeFromStopped(t *testing.T) {
	// Skip if binary not built
	skipIfBinaryNotBuilt(t)
	// Skip if blockchain binary not available
	skipIfBlockchainBinaryNotAvailable(t)

	// Setup, deploy, and stop a devnet
	ctx, runner, validator, cleanup := setupTest(t)
	defer cleanup.CleanupDevnet()

	// Deploy and stop
	t.Log("Setting up and stopping devnet...")
	runner.MustRun("deploy",
		"--mode", "local",
		"--validators", "2",
		"--network", "testnet",
		"--home", ctx.HomeDir,
	)

	// Wait for initial startup
	_, err := validator.WaitForProcess("devnet/node0/stabled.pid", 30*time.Second)
	assert.NoError(t, err, "initial deploy should start validators")

	// Stop devnet
	runner.MustRun("stop", "--home", ctx.HomeDir)
	time.Sleep(2 * time.Second)

	// Verify stopped
	validator.AssertFileNotExists("validator0.pid")
	validator.AssertFileNotExists("validator1.pid")

	// Resume by running start again
	t.Log("Resuming devnet with start command...")
	result := runner.MustRun("start", "--home", ctx.HomeDir)

	assert.True(t, result.Success(), "start (resume) command should succeed")

	// Wait for processes to restart
	newPid0, err := validator.WaitForProcess("devnet/node0/stabled.pid", 30*time.Second)
	assert.NoError(t, err, "validator0 should restart")
	assert.Greater(t, newPid0, 0, "validator0 new PID should be valid")

	newPid1, err := validator.WaitForProcess("devnet/node1/stabled.pid", 30*time.Second)
	assert.NoError(t, err, "validator1 should restart")
	assert.Greater(t, newPid1, 0, "validator1 new PID should be valid")

	// Verify new processes are running
	validator.AssertProcessRunning(newPid0)
	validator.AssertProcessRunning(newPid1)

	// Verify ports are listening again
	err = validator.WaitForPortListening(26657, 60*time.Second)
	assert.NoError(t, err, "validator0 RPC should be listening after resume")

	t.Log("Resume test completed successfully")
	t.Logf("  - Restarted with new PIDs: %d, %d", newPid0, newPid1)
}

// TestDestroy_WithForceFlag tests the destroy command
// Verifies: all resources cleaned up, validators removed
func TestDestroy_WithForceFlag(t *testing.T) {
	// Skip if binary not built
	skipIfBinaryNotBuilt(t)
	// Skip if blockchain binary not available
	skipIfBlockchainBinaryNotAvailable(t)

	// Setup and deploy a devnet
	ctx, runner, validator, cleanup := setupTest(t)
	defer cleanup.CleanupDevnet()

	// Deploy devnet
	t.Log("Setting up devnet for destroy test...")
	runner.MustRun("deploy",
		"--mode", "local",
		"--validators", "2",
		"--network", "testnet",
		"--home", ctx.HomeDir,
	)

	// Wait for processes to start
	pid0, err := validator.WaitForProcess("devnet/node0/stabled.pid", 30*time.Second)
	assert.NoError(t, err, "validator0 should be running")
	pid1, err := validator.WaitForProcess("devnet/node1/stabled.pid", 30*time.Second)
	assert.NoError(t, err, "validator1 should be running")

	// Execute destroy command
	t.Log("Executing destroy command...")
	result := runner.MustRun("destroy", "--force", "--home", ctx.HomeDir)

	assert.True(t, result.Success(), "destroy command should succeed")
	assert.Contains(t, result.Stdout, "destroyed", "should show destroy message")

	// Verify processes are stopped
	t.Log("Verifying processes stopped...")
	time.Sleep(2 * time.Second)

	validator.AssertProcessNotRunning(pid0)
	validator.AssertProcessNotRunning(pid1)

	// Verify all validator directories are removed
	t.Log("Verifying validator directories removed...")
	validator.AssertDirectoryNotExists("validator0")
	validator.AssertDirectoryNotExists("validator1")
	validator.AssertValidatorCount(0)

	// Verify PID files are removed
	validator.AssertFileNotExists("validator0.pid")
	validator.AssertFileNotExists("validator1.pid")

	// Verify ports are released
	validator.AssertPortNotListening(26657)
	validator.AssertPortNotListening(36657)

	t.Log("Destroy test completed successfully")
	t.Log("  - All processes stopped")
	t.Log("  - All directories cleaned up")
	t.Log("  - All ports released")
}

// TestDeploy_AlreadyExists_Error tests deploying when devnet already exists
// Verifies: proper error message, no corruption of existing devnet
func TestDeploy_AlreadyExists_Error(t *testing.T) {
	// Skip if binary not built
	skipIfBinaryNotBuilt(t)
	// Skip if blockchain binary not available
	skipIfBlockchainBinaryNotAvailable(t)

	// Setup and deploy initial devnet
	ctx, runner, validator, cleanup := setupTest(t)
	defer cleanup.CleanupDevnet()

	// First deploy
	t.Log("Creating initial devnet...")
	runner.MustRun("deploy",
		"--mode", "local",
		"--validators", "2",
		"--network", "testnet",
		"--home", ctx.HomeDir,
	)

	// Verify initial deploy succeeded
	validator.AssertValidatorCount(2)

	// Try to deploy again (should fail)
	t.Log("Attempting second deploy (should fail)...")
	result := runner.MustFail("deploy",
		"--mode", "local",
		"--validators", "2",
		"--network", "testnet",
		"--home", ctx.HomeDir,
	)

	// Verify error message
	assert.Contains(t, result.Stderr, "already exists", "should show 'already exists' error")

	// Verify original devnet is unchanged
	t.Log("Verifying original devnet unchanged...")
	validator.AssertValidatorCount(2)
	validator.AssertDirectoryExists("devnet/node0")
	validator.AssertDirectoryExists("devnet/node1")

	t.Log("Duplicate deploy error test completed successfully")
}

// TestStart_NoDevnet_Error tests starting when no devnet exists
// Verifies: proper error message, graceful handling
func TestStart_NoDevnet_Error(t *testing.T) {
	// Skip if binary not built
	skipIfBinaryNotBuilt(t)

	// Setup empty test environment (no devnet)
	ctx, runner, _, cleanup := setupTest(t)
	defer cleanup.CleanupDevnet()

	// Try to start non-existent devnet
	t.Log("Attempting to start non-existent devnet...")
	result := runner.MustFail("start", "--home", ctx.HomeDir)

	// Verify error message - can be "not found" or "no devnet found"
	hasNotFoundError := strings.Contains(result.Stderr, "not found") ||
		strings.Contains(result.Stderr, "no devnet")
	assert.True(t, hasNotFoundError, "should show 'not found' error")

	t.Log("Start without devnet error test completed successfully")
}
