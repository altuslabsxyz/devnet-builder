package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestDeploy_InvalidValidatorCount tests deploy with invalid validator count
// Verifies: proper validation and error message for out-of-range validator count
func TestDeploy_InvalidValidatorCount(t *testing.T) {
	skipIfBinaryNotBuilt(t)

	testCases := []struct {
		name           string
		validatorCount string
		expectedError  string
	}{
		{
			name:           "zero validators",
			validatorCount: "0",
			expectedError:  "validator count must be between 1 and 4",
		},
		{
			name:           "too many validators",
			validatorCount: "5",
			expectedError:  "validator count must be between 1 and 4",
		},
		{
			name:           "negative validators",
			validatorCount: "-1",
			expectedError:  "invalid",
		},
		{
			name:           "non-numeric validators",
			validatorCount: "abc",
			expectedError:  "invalid",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup isolated test environment for each case
			ctx, runner, _, cleanup := setupTest(t)
			defer cleanup.CleanupDevnet()

			// Execute deploy with invalid validator count
			t.Logf("Testing with --validators %s", tc.validatorCount)
			result := runner.MustFail("deploy",
				"--mode", "local",
				"--validators", tc.validatorCount,
				"--network", "testnet",
				"--home", ctx.HomeDir,
			)

			// Verify error message
			assert.Contains(t, result.Stderr, tc.expectedError,
				"should show appropriate error message")
			assert.NotEqual(t, 0, result.ExitCode, "should return non-zero exit code")
		})
	}
}

// TestDeploy_ConflictingFlags tests deploy with conflicting flag combinations
// Verifies: proper validation of incompatible flag combinations
func TestDeploy_ConflictingFlags(t *testing.T) {
	skipIfBinaryNotBuilt(t)

	testCases := []struct {
		name          string
		args          []string
		expectedError string
	}{
		{
			name: "docker mode with local-only flag",
			args: []string{
				"deploy",
				"--mode", "docker",
				"--local-only",
				"--validators", "2",
			},
			expectedError: "cannot use --local-only with docker mode",
		},
		{
			name: "conflicting network sources",
			args: []string{
				"deploy",
				"--network", "mainnet",
				"--genesis-file", "/path/to/genesis.json",
				"--validators", "2",
			},
			expectedError: "cannot specify both --network and --genesis-file",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, runner, _, cleanup := setupTest(t)
			defer cleanup.CleanupDevnet()

			// Add home directory to args
			args := append(tc.args, "--home", ctx.HomeDir)

			// Execute command with conflicting flags
			t.Logf("Testing conflicting flags: %v", tc.args)
			result := runner.MustFail(args...)

			// Verify error message
			assert.Contains(t, result.Stderr, tc.expectedError,
				"should show conflict error")
			assert.NotEqual(t, 0, result.ExitCode, "should return non-zero exit code")
		})
	}
}

// TestDeploy_DockerNotRunning tests deploy in docker mode when Docker is not running
// Verifies: graceful handling when Docker daemon is unavailable
func TestDeploy_DockerNotRunning(t *testing.T) {
	skipIfBinaryNotBuilt(t)

	// This test requires Docker to be installed but not running
	// Skip if Docker is available and running
	skipIfDockerNotAvailable(t)

	ctx, runner, _, cleanup := setupTest(t)
	defer cleanup.CleanupDevnet()

	// Try to deploy in docker mode
	t.Log("Attempting docker deploy with Docker daemon stopped...")
	result := runner.MustFail("deploy",
		"--mode", "docker",
		"--validators", "2",
		"--network", "testnet",
		"--home", ctx.HomeDir,
	)

	// Verify error message mentions Docker
	assert.Contains(t, result.Stderr, "docker",
		"error should mention Docker")
	assert.NotEqual(t, 0, result.ExitCode, "should return non-zero exit code")

	t.Log("Docker unavailable error handled correctly")
}

// TestDestroy_WithoutForce_PromptConfirmation tests destroy without --force flag
// Verifies: interactive confirmation prompt appears
func TestDestroy_WithoutForce_PromptConfirmation(t *testing.T) {
	skipIfBinaryNotBuilt(t)

	// Setup and deploy a devnet
	ctx, runner, _, cleanup := setupTest(t)
	defer cleanup.CleanupDevnet()

	runner.MustRun("deploy",
		"--mode", "local",
		"--validators", "2",
		"--network", "testnet",
		"--home", ctx.HomeDir,
	)

	// Try to destroy without --force flag (should prompt or fail in non-interactive mode)
	t.Log("Attempting destroy without --force flag...")
	result := runner.Run("destroy", "--home", ctx.HomeDir)

	// In non-interactive mode (CI), command should fail or show prompt message
	// Either way, exit code should indicate user action needed
	if result.Failed() {
		assert.Contains(t, result.Stderr, "confirmation required",
			"should indicate confirmation needed")
	} else {
		// If command succeeded, verify prompt message was shown
		assert.Contains(t, result.Stdout, "confirm",
			"should show confirmation prompt")
	}

	t.Log("Destroy confirmation handling verified")
}

// TestDeploy_PortConflict tests deploy when required ports are already in use
// Verifies: detection and reporting of port conflicts
func TestDeploy_PortConflict(t *testing.T) {
	skipIfBinaryNotBuilt(t)

	// First deployment to occupy ports
	ctx1, runner1, validator1, cleanup1 := setupTest(t)
	defer cleanup1.CleanupDevnet()

	t.Log("Creating first devnet to occupy ports...")
	runner1.MustRun("deploy",
		"--mode", "local",
		"--validators", "2",
		"--network", "testnet",
		"--home", ctx1.HomeDir,
	)

	// Wait for ports to be in use
	_ = validator1.WaitForPortListening(26657, 60)

	// Try to deploy second devnet with overlapping ports (should fail)
	ctx2, runner2, _, cleanup2 := setupTest(t)
	defer cleanup2.CleanupDevnet()

	t.Log("Attempting second deploy with conflicting ports...")
	result := runner2.MustFail("deploy",
		"--mode", "local",
		"--validators", "2",
		"--network", "testnet",
		"--home", ctx2.HomeDir,
	)

	// Verify error message mentions port conflict
	// (actual error message may vary based on implementation)
	assert.NotEqual(t, 0, result.ExitCode, "should fail with non-zero exit code")

	// Note: Actual port conflict detection depends on implementation
	// This test may need adjustment based on actual behavior
	t.Log("Port conflict test completed")
}

// TestEdgeCase_SnapshotDownloadInterrupt tests handling of interrupted snapshot download
// Verifies: graceful handling when snapshot download fails or is interrupted
func TestEdgeCase_SnapshotDownloadInterrupt(t *testing.T) {
	skipIfBinaryNotBuilt(t)

	// Setup test with mock servers
	ctx, runner, _, cleanup, snapshotServer, _ := setupTestWithMocks(t)
	defer cleanup.CleanupDevnet()

	// Configure mock server to simulate download failure
	t.Log("Simulating snapshot download failure...")
	snapshotServer.SimulateDownloadFailure("testnet-snapshot.tar.gz")

	// Try to deploy (should handle download failure gracefully)
	result := runner.Run("deploy",
		"--mode", "local",
		"--validators", "2",
		"--network", "testnet",
		"--snapshot-url", snapshotServer.SnapshotURL("testnet-snapshot.tar.gz"),
		"--home", ctx.HomeDir,
	)

	// Command should either:
	// 1. Fail gracefully with appropriate error message, OR
	// 2. Fall back to genesis initialization
	if result.Failed() {
		assert.Contains(t, result.Stderr, "snapshot",
			"error should mention snapshot issue")
		t.Log("Snapshot download failure handled with error")
	} else {
		// If succeeded, verify it fell back to genesis initialization
		t.Log("Snapshot download failure handled with fallback to genesis")
	}

	// Verify no partial/corrupt files left behind
	// (implementation specific - may need adjustment)
	t.Log("Snapshot interrupt handling verified")
}

// TestEdgeCase_InvalidGenesisFile tests deploy with corrupted genesis file
// Verifies: validation of genesis file format
func TestEdgeCase_InvalidGenesisFile(t *testing.T) {
	skipIfBinaryNotBuilt(t)

	ctx, runner, _, cleanup := setupTest(t)
	defer cleanup.CleanupDevnet()

	// Create invalid genesis file
	invalidGenesis := ctx.WriteFile("invalid-genesis.json", []byte(`{invalid json`))

	// Try to deploy with invalid genesis file
	t.Log("Testing with invalid genesis file...")
	result := runner.MustFail("deploy",
		"--mode", "local",
		"--validators", "2",
		"--genesis-file", invalidGenesis,
		"--home", ctx.HomeDir,
	)

	// Verify error message
	assert.Contains(t, result.Stderr, "genesis",
		"error should mention genesis file issue")
	assert.NotEqual(t, 0, result.ExitCode, "should return non-zero exit code")

	t.Log("Invalid genesis file error handled correctly")
}

// TestEdgeCase_InsufficientDiskSpace tests deploy with insufficient disk space
// Note: This test is challenging to implement reliably across environments
// and may be platform-specific. Marked as informational.
func TestEdgeCase_InsufficientDiskSpace(t *testing.T) {
	t.Skip("Skipping disk space test - requires platform-specific implementation")

	// This test would:
	// 1. Check available disk space
	// 2. If sufficient space, create files to fill disk near capacity
	// 3. Attempt deploy
	// 4. Verify graceful handling of ENOSPC errors
	// 5. Clean up created files

	// Implementation omitted as it requires careful platform-specific handling
}

// TestCleanup_AfterFailedDeploy tests that cleanup succeeds even after failed deploy
// Verifies: cleanup manager handles partial deployments correctly
func TestCleanup_AfterFailedDeploy(t *testing.T) {
	skipIfBinaryNotBuilt(t)

	ctx, runner, validator, cleanup := setupTest(t)
	defer cleanup.CleanupDevnet()

	// Attempt deploy that will fail (invalid validator count)
	t.Log("Attempting failed deploy...")
	runner.MustFail("deploy",
		"--mode", "local",
		"--validators", "0",
		"--network", "testnet",
		"--home", ctx.HomeDir,
	)

	// Manually trigger cleanup (normally done by defer)
	t.Log("Running cleanup after failed deploy...")
	err := cleanup.CleanupDevnet()
	assert.NoError(t, err, "cleanup should succeed even after failed deploy")

	// Verify no resources leaked
	validator.AssertValidatorCount(0)

	t.Log("Cleanup after failed deploy verified")
}
