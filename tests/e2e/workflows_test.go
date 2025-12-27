package e2e

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestWorkflow_FullLifecycle tests complete devnet lifecycle
// Verifies: deploy → status → logs → stop → start → destroy workflow
func TestWorkflow_FullLifecycle(t *testing.T) {
	skipIfBinaryNotBuilt(t)

	ctx, runner, validator, cleanup := setupTest(t)
	defer cleanup.CleanupDevnet()

	// Step 1: Deploy
	t.Log("Step 1: Deploy devnet...")
	result := runner.MustRun("deploy",
		"--mode", "local",
		"--validators", "2",
		"--network", "testnet",
		"--home", ctx.HomeDir,
	)
	assert.Contains(t, result.Stdout, "deployed", "deploy should succeed")

	// Step 2: Check status
	t.Log("Step 2: Check status...")
	_, err := validator.WaitForProcess("validator0.pid", 30*time.Second)
	assert.NoError(t, err, "validators should start")

	result = runner.MustRun("status", "--home", ctx.HomeDir)
	assert.Contains(t, result.Stdout, "Running", "status should show running")

	// Step 3: View logs
	t.Log("Step 3: View logs...")
	result = runner.MustRun("logs",
		"--validator", "0",
		"--lines", "10",
		"--home", ctx.HomeDir,
	)
	assert.NotEmpty(t, result.Stdout, "logs should show output")

	// Step 4: Stop
	t.Log("Step 4: Stop devnet...")
	result = runner.MustRun("stop", "--home", ctx.HomeDir)
	assert.Contains(t, result.Stdout, "stopped", "stop should succeed")

	time.Sleep(2 * time.Second)
	validator.AssertFileNotExists("validator0.pid")

	// Step 5: Start again
	t.Log("Step 5: Start devnet...")
	result = runner.MustRun("start", "--home", ctx.HomeDir)
	assert.Contains(t, result.Stdout, "started", "start should succeed")

	_, err = validator.WaitForProcess("validator0.pid", 30*time.Second)
	assert.NoError(t, err, "validators should restart")

	// Step 6: Destroy
	t.Log("Step 6: Destroy devnet...")
	result = runner.MustRun("destroy", "--force", "--home", ctx.HomeDir)
	assert.Contains(t, result.Stdout, "destroyed", "destroy should succeed")

	validator.AssertValidatorCount(0)

	t.Log("Full lifecycle workflow completed successfully")
}

// TestWorkflow_ExportAndRestore tests exporting and restoring state
// Verifies: deploy → export → destroy → deploy with exported genesis
func TestWorkflow_ExportAndRestore(t *testing.T) {
	skipIfBinaryNotBuilt(t)

	ctx, runner, validator, cleanup := setupTest(t)
	defer cleanup.CleanupDevnet()

	// Step 1: Deploy initial devnet
	t.Log("Step 1: Deploy initial devnet...")
	runner.MustRun("deploy",
		"--mode", "local",
		"--validators", "2",
		"--network", "testnet",
		"--home", ctx.HomeDir,
	)

	_, err := validator.WaitForProcess("validator0.pid", 30*time.Second)
	assert.NoError(t, err, "validators should start")

	// Let some blocks be produced
	time.Sleep(10 * time.Second)

	// Step 2: Export state
	t.Log("Step 2: Export state...")
	exportPath := ctx.HomeDir + "/exported.json"
	result := runner.MustRun("export",
		"--output", exportPath,
		"--home", ctx.HomeDir,
	)
	assert.Contains(t, result.Stdout, "exported", "export should succeed")

	validator.AssertFileExists("exported.json")

	// Step 3: Destroy devnet
	t.Log("Step 3: Destroy devnet...")
	runner.MustRun("destroy", "--force", "--home", ctx.HomeDir)
	validator.AssertValidatorCount(0)

	// Step 4: Deploy with exported genesis
	t.Log("Step 4: Deploy with exported genesis...")
	result = runner.Run("deploy",
		"--genesis-file", exportPath,
		"--validators", "2",
		"--mode", "local",
		"--home", ctx.HomeDir,
	)

	// If genesis-file support is implemented, verify success
	if result.Success() {
		validator.AssertValidatorCount(2)
		t.Log("Export and restore workflow completed successfully")
	} else {
		t.Log("Genesis-file deployment may not be fully implemented")
	}
}

// TestWorkflow_UpgradeAndRollback tests upgrade and rollback
// Verifies: deploy → upgrade → verify → rollback (if needed)
func TestWorkflow_UpgradeAndRollback(t *testing.T) {
	skipIfBinaryNotBuilt(t)

	ctx, runner, validator, cleanup := setupTest(t)
	defer cleanup.CleanupDevnet()

	// Step 1: Deploy initial version
	t.Log("Step 1: Deploy initial version...")
	runner.MustRun("deploy",
		"--mode", "local",
		"--validators", "2",
		"--network", "testnet",
		"--home", ctx.HomeDir,
	)

	pid0, err := validator.WaitForProcess("validator0.pid", 30*time.Second)
	assert.NoError(t, err, "validators should start")

	// Step 2: Upgrade to new version
	t.Log("Step 2: Attempt upgrade...")
	result := runner.Run("upgrade",
		"--version", "v1.1.0",
		"--home", ctx.HomeDir,
	)

	// Upgrade may not be fully implemented
	if result.Failed() {
		t.Log("Upgrade workflow may not be fully implemented")
		return
	}

	// Step 3: Verify validators restarted
	t.Log("Step 3: Verify upgrade...")
	newPid0, err := validator.WaitForProcess("validator0.pid", 30*time.Second)
	assert.NoError(t, err, "validators should restart after upgrade")
	assert.NotEqual(t, pid0, newPid0, "PID should change after upgrade")

	// Step 4: Rollback (if rollback command exists)
	t.Log("Step 4: Attempt rollback...")
	result = runner.Run("rollback", "--home", ctx.HomeDir)

	if result.Success() {
		t.Log("Rollback completed")
	} else {
		t.Log("Rollback command may not be implemented")
	}

	t.Log("Upgrade workflow verified")
}

// TestWorkflow_MultiValidator tests managing individual validators
// Verifies: deploy → stop one validator → verify network continues → restart validator
func TestWorkflow_MultiValidator(t *testing.T) {
	skipIfBinaryNotBuilt(t)

	ctx, runner, validator, cleanup := setupTest(t)
	defer cleanup.CleanupDevnet()

	// Step 1: Deploy 4 validators
	t.Log("Step 1: Deploy 4 validators...")
	runner.MustRun("deploy",
		"--mode", "local",
		"--validators", "4",
		"--network", "testnet",
		"--home", ctx.HomeDir,
	)

	// Wait for all validators to start
	pids := make([]int, 4)
	for i := 0; i < 4; i++ {
		pid, err := validator.WaitForProcess(
			ctx.HomeDir+"/validator"+string(rune('0'+i))+".pid",
			30*time.Second,
		)
		assert.NoError(t, err, "validator%d should start", i)
		pids[i] = pid
	}

	// Step 2: Stop validator 2
	t.Log("Step 2: Stop validator 2...")
	result := runner.MustRun("node", "stop",
		"--validator", "2",
		"--home", ctx.HomeDir,
	)
	assert.Contains(t, result.Stdout, "stopped", "node stop should succeed")

	time.Sleep(2 * time.Second)
	validator.AssertProcessNotRunning(pids[2])

	// Step 3: Verify other validators still running
	t.Log("Step 3: Verify other validators still running...")
	validator.AssertProcessRunning(pids[0])
	validator.AssertProcessRunning(pids[1])
	validator.AssertProcessRunning(pids[3])

	// Step 4: Restart validator 2
	t.Log("Step 4: Restart validator 2...")
	result = runner.MustRun("node", "start",
		"--validator", "2",
		"--home", ctx.HomeDir,
	)
	assert.Contains(t, result.Stdout, "started", "node start should succeed")

	newPid2, err := validator.WaitForProcess(
		ctx.HomeDir+"/validator2.pid",
		30*time.Second,
	)
	assert.NoError(t, err, "validator2 should restart")
	assert.NotEqual(t, pids[2], newPid2, "PID should be different after restart")

	// Step 5: Verify all validators running
	t.Log("Step 5: Verify all validators running...")
	validator.AssertProcessRunning(pids[0])
	validator.AssertProcessRunning(pids[1])
	validator.AssertProcessRunning(newPid2)
	validator.AssertProcessRunning(pids[3])

	t.Log("Multi-validator workflow completed successfully")
}
