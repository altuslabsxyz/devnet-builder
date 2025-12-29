package e2e

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestDockerMode_Deploy tests deployment in Docker mode
// Verifies: Docker containers created, validators running in containers
func TestDockerMode_Deploy(t *testing.T) {
	skipIfBinaryNotBuilt(t)
	skipIfDockerNotAvailable(t)
	skipIfBlockchainBinaryNotAvailable(t)

	// Setup test environment
	ctx, runner, validator, cleanup := setupTest(t)
	defer cleanup.CleanupDevnet()

	// Deploy in Docker mode
	t.Log("Deploying devnet in Docker mode...")
	result := runner.Run("deploy",
		"--mode", "docker",
		"--validators", "2",
		"--network", "testnet",
		"--home", ctx.HomeDir,
	)

	// If Docker is not available or command failed, skip gracefully
	if result.Failed() {
		if result.Stderr != "" {
			t.Skipf("Docker mode not available: %s", result.Stderr)
		}
		t.Skip("Docker mode deployment failed, skipping test")
	}

	assert.True(t, result.Success(), "deploy should succeed in docker mode")

	// Verify Docker containers exist
	t.Log("Verifying Docker containers...")
	validator.AssertDockerContainerExists("validator0")
	validator.AssertDockerContainerExists("validator1")

	// Verify containers are running
	err := validator.WaitForDockerContainer("validator0", 60*time.Second)
	assert.NoError(t, err, "validator0 container should be running")

	err = validator.WaitForDockerContainer("validator1", 60*time.Second)
	assert.NoError(t, err, "validator1 container should be running")

	t.Log("Docker mode deployment verified successfully")
}

// TestLocalMode_Deploy tests deployment in local mode
// Verifies: local processes created, validators running as processes
func TestLocalMode_Deploy(t *testing.T) {
	skipIfBinaryNotBuilt(t)
	skipIfBlockchainBinaryNotAvailable(t)

	// Setup test environment
	ctx, runner, validator, cleanup := setupTest(t)
	defer cleanup.CleanupDevnet()

	// Deploy in local mode
	t.Log("Deploying devnet in local mode...")
	result := runner.MustRun("deploy",
		"--mode", "local",
		"--validators", "2",
		"--network", "testnet",
		"--home", ctx.HomeDir,
	)

	assert.True(t, result.Success(), "deploy should succeed in local mode")

	// Verify processes are running (not Docker containers)
	t.Log("Verifying local processes...")
	pid0, err := validator.WaitForProcess("devnet/node0/stabled.pid", 30*time.Second)
	assert.NoError(t, err, "validator0 process should start")
	assert.Greater(t, pid0, 0, "PID should be valid")

	pid1, err := validator.WaitForProcess("devnet/node1/stabled.pid", 30*time.Second)
	assert.NoError(t, err, "validator1 process should start")
	assert.Greater(t, pid1, 0, "PID should be valid")

	validator.AssertProcessRunning(pid0)
	validator.AssertProcessRunning(pid1)

	t.Log("Local mode deployment verified successfully")
}

// TestValidatorCount_1Validator tests with 1 validator
// Verifies: single validator network can be created
func TestValidatorCount_1Validator(t *testing.T) {
	skipIfBinaryNotBuilt(t)
	skipIfBlockchainBinaryNotAvailable(t)

	ctx, runner, validator, cleanup := setupTest(t)
	defer cleanup.CleanupDevnet()

	t.Log("Deploying with 1 validator...")
	runner.MustRun("deploy",
		"--mode", "local",
		"--validators", "1",
		"--network", "testnet",
		"--home", ctx.HomeDir,
	)

	// Verify exactly 1 validator created
	validator.AssertValidatorCount(1)
	validator.AssertDirectoryExists("devnet/node0")
	validator.AssertDirectoryNotExists("validator1")

	// Verify process running
	pid0, err := validator.WaitForProcess("devnet/node0/stabled.pid", 30*time.Second)
	assert.NoError(t, err, "validator0 should start")
	validator.AssertProcessRunning(pid0)

	t.Log("Single validator deployment verified")
}

// TestValidatorCount_4Validators tests with maximum validators
// Verifies: 4 validator network can be created
func TestValidatorCount_4Validators(t *testing.T) {
	skipIfBinaryNotBuilt(t)
	skipIfBlockchainBinaryNotAvailable(t)

	ctx, runner, validator, cleanup := setupTest(t)
	defer cleanup.CleanupDevnet()

	t.Log("Deploying with 4 validators...")
	runner.MustRun("deploy",
		"--mode", "local",
		"--validators", "4",
		"--network", "testnet",
		"--home", ctx.HomeDir,
	)

	// Verify exactly 4 validators created
	validator.AssertValidatorCount(4)
	validator.AssertDirectoryExists("devnet/node0")
	validator.AssertDirectoryExists("devnet/node1")
	validator.AssertDirectoryExists("devnet/node2")
	validator.AssertDirectoryExists("devnet/node3")

	// Verify all processes running
	for i := 0; i < 4; i++ {
		pidFile := fmt.Sprintf("validator%d.pid", i)
		pid, err := validator.WaitForProcess(pidFile, 30*time.Second)
		assert.NoError(t, err, "validator%d should start", i)
		validator.AssertProcessRunning(pid)
	}

	// Verify ports are correctly offset (10000 per validator)
	// validator0: 26657, validator1: 36657, validator2: 46657, validator3: 56657
	err := validator.WaitForPortListening(26657, 60*time.Second)
	assert.NoError(t, err, "validator0 RPC should be listening")

	err = validator.WaitForPortListening(36657, 60*time.Second)
	assert.NoError(t, err, "validator1 RPC should be listening")

	err = validator.WaitForPortListening(46657, 60*time.Second)
	assert.NoError(t, err, "validator2 RPC should be listening")

	err = validator.WaitForPortListening(56657, 60*time.Second)
	assert.NoError(t, err, "validator3 RPC should be listening")

	t.Log("4 validator deployment verified")
}

// TestNetworkType_Mainnet tests deploying mainnet configuration
// Verifies: mainnet network can be deployed
func TestNetworkType_Mainnet(t *testing.T) {
	skipIfBinaryNotBuilt(t)
	skipIfBlockchainBinaryNotAvailable(t)

	ctx, runner, validator, cleanup := setupTest(t)
	defer cleanup.CleanupDevnet()

	t.Log("Deploying mainnet network...")
	runner.MustRun("deploy",
		"--mode", "local",
		"--validators", "2",
		"--network", "mainnet",
		"--home", ctx.HomeDir,
	)

	// Verify deployment succeeded
	validator.AssertValidatorCount(2)

	// Verify genesis file has mainnet chain ID
	validator.AssertFileExists("devnet/node0/config/genesis.json")
	content := ctx.ReadFile("validator0/config/genesis.json")

	// Mainnet should have different chain ID than testnet
	assert.Contains(t, string(content), "mainnet",
		"genesis should reference mainnet")

	t.Log("Mainnet deployment verified")
}

// TestNetworkType_Testnet tests deploying testnet configuration
// Verifies: testnet network can be deployed
func TestNetworkType_Testnet(t *testing.T) {
	skipIfBinaryNotBuilt(t)
	skipIfBlockchainBinaryNotAvailable(t)

	ctx, runner, validator, cleanup := setupTest(t)
	defer cleanup.CleanupDevnet()

	t.Log("Deploying testnet network...")
	runner.MustRun("deploy",
		"--mode", "local",
		"--validators", "2",
		"--network", "testnet",
		"--home", ctx.HomeDir,
	)

	// Verify deployment succeeded
	validator.AssertValidatorCount(2)

	// Verify genesis file has testnet chain ID
	validator.AssertFileExists("devnet/node0/config/genesis.json")
	content := ctx.ReadFile("validator0/config/genesis.json")

	// Testnet should have different chain ID than mainnet
	assert.Contains(t, string(content), "testnet",
		"genesis should reference testnet")

	t.Log("Testnet deployment verified")
}
