package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/application/ports"
	"github.com/altuslabsxyz/devnet-builder/internal/application/upgrade"
	"github.com/altuslabsxyz/devnet-builder/internal/infrastructure/persistence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestResumableUpgrade_StatePersistence tests that upgrade state persists across
// simulated process restarts (new state manager instances).
func TestResumableUpgrade_StatePersistence(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	homeDir := t.TempDir()

	// Create first state manager (simulates first process)
	stateManager1 := persistence.NewFileUpgradeStateManager(homeDir)

	// Create initial state
	// Note: NewUpgradeState(upgradeName, mode, skipGov) - first param is upgradeName
	state := ports.NewUpgradeState("test-upgrade", "local", false)
	state.TargetVersion = "v2.0.0"
	state.ProposalID = 42

	// Save state
	err := stateManager1.SaveState(ctx, state)
	require.NoError(t, err)

	// Verify state file exists (path is homeDir/.upgrade-state.json)
	statePath := filepath.Join(homeDir, ".upgrade-state.json")
	_, err = os.Stat(statePath)
	require.NoError(t, err, "State file should exist")

	// Create second state manager (simulates new process after restart)
	stateManager2 := persistence.NewFileUpgradeStateManager(homeDir)

	// Load state from second manager
	loadedState, err := stateManager2.LoadState(ctx)
	require.NoError(t, err)
	require.NotNil(t, loadedState)

	// Verify state was preserved
	assert.Equal(t, "v2.0.0", loadedState.TargetVersion)
	assert.Equal(t, "test-upgrade", loadedState.UpgradeName)
	assert.Equal(t, uint64(42), loadedState.ProposalID)
	assert.Equal(t, ports.ResumableStageInitialized, loadedState.Stage)
	assert.Equal(t, "local", loadedState.Mode)
}

// TestResumableUpgrade_StateTransitions tests the full state machine workflow.
func TestResumableUpgrade_StateTransitions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	homeDir := t.TempDir()

	stateManager := persistence.NewFileUpgradeStateManager(homeDir)
	transitioner := upgrade.NewStateTransitioner()

	// Create state for governance path
	state := ports.NewUpgradeState("v2.0.0", "local", false)
	state.UpgradeName = "governance-upgrade"

	// Simulate full governance upgrade path
	transitions := []struct {
		toStage ports.ResumableStage
		reason  string
	}{
		{ports.ResumableStageProposalSubmitted, "proposal 42 submitted"},
		{ports.ResumableStageVoting, "voting started"},
		{ports.ResumableStageWaitingForHeight, "vote passed, waiting for height 100"},
		{ports.ResumableStageChainHalted, "chain halted at upgrade height"},
		{ports.ResumableStageSwitchingBinary, "switching binary"},
		{ports.ResumableStageVerifyingResume, "verifying chain resumes"},
		{ports.ResumableStageCompleted, "upgrade completed successfully"},
	}

	for _, tr := range transitions {
		t.Run(tr.toStage.String(), func(t *testing.T) {
			err := transitioner.TransitionTo(state, tr.toStage, tr.reason)
			require.NoError(t, err)
			assert.Equal(t, tr.toStage, state.Stage)

			// Save and reload to verify persistence
			err = stateManager.SaveState(ctx, state)
			require.NoError(t, err)

			reloaded, err := stateManager.LoadState(ctx)
			require.NoError(t, err)
			assert.Equal(t, tr.toStage, reloaded.Stage)
		})
	}

	// Verify final state
	finalState, err := stateManager.LoadState(ctx)
	require.NoError(t, err)
	assert.True(t, finalState.Stage.IsTerminal())
	assert.Equal(t, ports.ResumableStageCompleted, finalState.Stage)
	assert.Len(t, finalState.StageHistory, 8) // Initial + 7 transitions
}

// TestResumableUpgrade_SkipGovPath tests the skip-governance path.
func TestResumableUpgrade_SkipGovPath(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	homeDir := t.TempDir()

	stateManager := persistence.NewFileUpgradeStateManager(homeDir)
	transitioner := upgrade.NewStateTransitioner()

	// Create state for skip-gov path
	state := ports.NewUpgradeState("v2.0.0", "local", true) // skipGov=true
	state.UpgradeName = "skip-gov-upgrade"

	// Skip-gov path: Initialized → SwitchingBinary → VerifyingResume → Completed
	transitions := []struct {
		toStage ports.ResumableStage
		reason  string
	}{
		{ports.ResumableStageSwitchingBinary, "switching binary (skip-gov)"},
		{ports.ResumableStageVerifyingResume, "verifying chain resumes"},
		{ports.ResumableStageCompleted, "upgrade completed successfully"},
	}

	for _, tr := range transitions {
		err := transitioner.TransitionTo(state, tr.toStage, tr.reason)
		require.NoError(t, err, "Transition to %s should succeed", tr.toStage)

		// Persist after each transition
		err = stateManager.SaveState(ctx, state)
		require.NoError(t, err)
	}

	// Verify final state
	finalState, err := stateManager.LoadState(ctx)
	require.NoError(t, err)
	assert.Equal(t, ports.ResumableStageCompleted, finalState.Stage)
	assert.True(t, finalState.SkipGovernance)
}

// TestResumableUpgrade_InterruptionAndResume tests that state survives interruption
// and can be resumed from the correct stage.
func TestResumableUpgrade_InterruptionAndResume(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	homeDir := t.TempDir()

	// First "process" - simulate partial upgrade
	{
		stateManager := persistence.NewFileUpgradeStateManager(homeDir)
		transitioner := upgrade.NewStateTransitioner()

		state := ports.NewUpgradeState("v2.0.0", "local", false)
		state.UpgradeName = "interrupted-upgrade"
		state.ProposalID = 123
		state.UpgradeHeight = 500

		// Progress to WaitingForHeight stage
		_ = transitioner.TransitionTo(state, ports.ResumableStageProposalSubmitted, "submitted")
		_ = transitioner.TransitionTo(state, ports.ResumableStageVoting, "voting")
		err := transitioner.TransitionTo(state, ports.ResumableStageWaitingForHeight, "waiting")
		require.NoError(t, err)

		// Record some validator votes
		now := time.Now()
		state.ValidatorVotes = []ports.ValidatorVoteState{
			{Address: "validator0", Voted: true, TxHash: "tx-hash-0", Timestamp: &now},
			{Address: "validator1", Voted: true, TxHash: "tx-hash-1", Timestamp: &now},
		}

		// Save state and "crash"
		err = stateManager.SaveState(ctx, state)
		require.NoError(t, err)
	}

	// Second "process" - resume from where we left off
	{
		stateManager := persistence.NewFileUpgradeStateManager(homeDir)

		// Load persisted state
		resumedState, err := stateManager.LoadState(ctx)
		require.NoError(t, err)
		require.NotNil(t, resumedState)

		// Verify state was correctly resumed
		assert.Equal(t, ports.ResumableStageWaitingForHeight, resumedState.Stage)
		assert.Equal(t, "interrupted-upgrade", resumedState.UpgradeName)
		assert.Equal(t, uint64(123), resumedState.ProposalID)
		assert.Equal(t, int64(500), resumedState.UpgradeHeight)

		// Verify validator votes were preserved
		assert.Len(t, resumedState.ValidatorVotes, 2)
		assert.True(t, resumedState.ValidatorVotes[0].Voted)
		assert.Equal(t, "tx-hash-0", resumedState.ValidatorVotes[0].TxHash)

		// Continue from resumed state
		transitioner := upgrade.NewStateTransitioner()
		err = transitioner.TransitionTo(resumedState, ports.ResumableStageChainHalted, "chain halted")
		require.NoError(t, err)
		err = transitioner.TransitionTo(resumedState, ports.ResumableStageSwitchingBinary, "switching")
		require.NoError(t, err)
		err = transitioner.TransitionTo(resumedState, ports.ResumableStageVerifyingResume, "verifying")
		require.NoError(t, err)
		err = transitioner.TransitionTo(resumedState, ports.ResumableStageCompleted, "completed")
		require.NoError(t, err)

		// Verify upgrade completed
		assert.Equal(t, ports.ResumableStageCompleted, resumedState.Stage)
	}
}

// TestResumableUpgrade_CorruptedStateRecovery tests handling of corrupted state files.
func TestResumableUpgrade_CorruptedStateRecovery(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	homeDir := t.TempDir()

	// Write corrupted state file directly to homeDir
	statePath := filepath.Join(homeDir, ".upgrade-state.json")
	err := os.WriteFile(statePath, []byte("not valid json{"), 0644)
	require.NoError(t, err)

	// Try to load - should get corruption error
	stateManager := persistence.NewFileUpgradeStateManager(homeDir)
	_, err = stateManager.LoadState(ctx)
	require.Error(t, err)

	// Should be a StateCorruptionError
	var corruptionErr *ports.StateCorruptionError
	assert.ErrorAs(t, err, &corruptionErr)
}

// TestResumableUpgrade_ChecksumValidation tests that checksum tampering is detected.
func TestResumableUpgrade_ChecksumValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	homeDir := t.TempDir()

	stateManager := persistence.NewFileUpgradeStateManager(homeDir)

	// Create and save valid state
	state := ports.NewUpgradeState("v2.0.0", "local", false)
	state.UpgradeName = "checksum-test"
	err := stateManager.SaveState(ctx, state)
	require.NoError(t, err)

	// Tamper with the state file (path is homeDir/.upgrade-state.json)
	statePath := filepath.Join(homeDir, ".upgrade-state.json")

	// Replace checksum with invalid value
	tamperedData := []byte(`{"data":{"version":"1.0","upgrade_name":"tampered","stage":"Initialized"},"checksum":"invalid-checksum"}`)
	err = os.WriteFile(statePath, tamperedData, 0644)
	require.NoError(t, err)

	// Try to load - should fail checksum validation
	_, err = stateManager.LoadState(ctx)
	require.Error(t, err)

	var corruptionErr *ports.StateCorruptionError
	assert.ErrorAs(t, err, &corruptionErr)
	assert.Contains(t, corruptionErr.Error(), "checksum")
}

// TestResumableUpgrade_ConcurrentAccessPrevention tests that file locking prevents
// concurrent upgrades.
func TestResumableUpgrade_ConcurrentAccessPrevention(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	homeDir := t.TempDir()

	stateManager1 := persistence.NewFileUpgradeStateManager(homeDir)
	stateManager2 := persistence.NewFileUpgradeStateManager(homeDir)

	// First process acquires lock
	err := stateManager1.AcquireLock(ctx)
	require.NoError(t, err)
	defer stateManager1.ReleaseLock(ctx)

	// Second process should fail to acquire lock
	err = stateManager2.AcquireLock(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "upgrade is in progress")
}

// TestResumableUpgrade_NodeSwitchTracking tests that node binary switches are tracked.
func TestResumableUpgrade_NodeSwitchTracking(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	homeDir := t.TempDir()

	stateManager := persistence.NewFileUpgradeStateManager(homeDir)
	transitioner := upgrade.NewStateTransitioner()

	state := ports.NewUpgradeState("v2.0.0", "local", false)
	state.UpgradeName = "node-switch-test"

	// Progress to SwitchingBinary stage
	_ = transitioner.TransitionTo(state, ports.ResumableStageProposalSubmitted, "submitted")
	_ = transitioner.TransitionTo(state, ports.ResumableStageVoting, "voting")
	_ = transitioner.TransitionTo(state, ports.ResumableStageWaitingForHeight, "waiting")
	_ = transitioner.TransitionTo(state, ports.ResumableStageChainHalted, "halted")
	err := transitioner.TransitionTo(state, ports.ResumableStageSwitchingBinary, "switching")
	require.NoError(t, err)

	// Record node switches
	now := time.Now()
	state.NodeSwitches = []ports.NodeSwitchState{
		{NodeName: "node0", Switched: true, Stopped: true, Started: true, OldBinary: "/old/bin", NewBinary: "/new/bin", Timestamp: &now},
		{NodeName: "node1", Switched: true, Stopped: true, Started: false, OldBinary: "/old/bin", NewBinary: "/new/bin", Timestamp: &now},
	}

	err = stateManager.SaveState(ctx, state)
	require.NoError(t, err)

	// Reload and verify
	reloaded, err := stateManager.LoadState(ctx)
	require.NoError(t, err)

	assert.Len(t, reloaded.NodeSwitches, 2)
	assert.Equal(t, "node0", reloaded.NodeSwitches[0].NodeName)
	assert.Equal(t, "node1", reloaded.NodeSwitches[1].NodeName)
	assert.True(t, reloaded.NodeSwitches[0].Started)
	assert.False(t, reloaded.NodeSwitches[1].Started)
}

// TestResumableUpgrade_FailureRecovery tests transitioning to failed state and metadata preservation.
func TestResumableUpgrade_FailureRecovery(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	homeDir := t.TempDir()

	stateManager := persistence.NewFileUpgradeStateManager(homeDir)
	transitioner := upgrade.NewStateTransitioner()

	state := ports.NewUpgradeState("v2.0.0", "local", false)
	state.UpgradeName = "failure-test"
	state.ProposalID = 99

	// Progress partway through upgrade
	_ = transitioner.TransitionTo(state, ports.ResumableStageProposalSubmitted, "submitted")
	_ = transitioner.TransitionTo(state, ports.ResumableStageVoting, "voting")

	// Simulate failure
	err := transitioner.TransitionTo(state, ports.ResumableStageFailed, "RPC connection lost")
	require.NoError(t, err)

	err = stateManager.SaveState(ctx, state)
	require.NoError(t, err)

	// Reload and verify failure state
	reloaded, err := stateManager.LoadState(ctx)
	require.NoError(t, err)

	assert.Equal(t, ports.ResumableStageFailed, reloaded.Stage)
	assert.Equal(t, "RPC connection lost", reloaded.Error)
	assert.True(t, reloaded.Stage.IsTerminal())

	// Verify we cannot transition from terminal state
	err = transitioner.TransitionTo(reloaded, ports.ResumableStageVoting, "try again")
	require.Error(t, err)
}
