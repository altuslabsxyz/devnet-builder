package unit

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/application/ports"
	"github.com/altuslabsxyz/devnet-builder/internal/infrastructure/persistence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFileUpgradeStateManager_SaveAndLoad tests basic save and load functionality.
func TestFileUpgradeStateManager_SaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	manager := persistence.NewFileUpgradeStateManager(tmpDir)
	ctx := context.Background()

	// Create test state
	state := ports.NewUpgradeState("v2.0.0", "local", false)
	state.ProposalID = 42
	state.UpgradeHeight = 150000
	state.TargetBinary = "/path/to/binary"
	state.TargetVersion = "v2.0.0"

	// Save state
	err := manager.SaveState(ctx, state)
	require.NoError(t, err)

	// Load state
	loaded, err := manager.LoadState(ctx)
	require.NoError(t, err)
	require.NotNil(t, loaded)

	// Verify fields match
	assert.Equal(t, state.UpgradeName, loaded.UpgradeName)
	assert.Equal(t, state.Stage, loaded.Stage)
	assert.Equal(t, state.Mode, loaded.Mode)
	assert.Equal(t, state.SkipGovernance, loaded.SkipGovernance)
	assert.Equal(t, state.ProposalID, loaded.ProposalID)
	assert.Equal(t, state.UpgradeHeight, loaded.UpgradeHeight)
	assert.Equal(t, state.TargetBinary, loaded.TargetBinary)
	assert.Equal(t, state.TargetVersion, loaded.TargetVersion)
	assert.NotEmpty(t, loaded.Checksum)
}

// TestFileUpgradeStateManager_LoadNonExistent tests loading when no state exists.
func TestFileUpgradeStateManager_LoadNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	manager := persistence.NewFileUpgradeStateManager(tmpDir)
	ctx := context.Background()

	// Load should return nil, nil for non-existent state
	loaded, err := manager.LoadState(ctx)
	require.NoError(t, err)
	assert.Nil(t, loaded)
}

// TestFileUpgradeStateManager_StateExists tests state existence check.
func TestFileUpgradeStateManager_StateExists(t *testing.T) {
	tmpDir := t.TempDir()
	manager := persistence.NewFileUpgradeStateManager(tmpDir)
	ctx := context.Background()

	// Initially should not exist
	exists, err := manager.StateExists(ctx)
	require.NoError(t, err)
	assert.False(t, exists)

	// Save state
	state := ports.NewUpgradeState("v2.0.0", "local", false)
	err = manager.SaveState(ctx, state)
	require.NoError(t, err)

	// Now should exist
	exists, err = manager.StateExists(ctx)
	require.NoError(t, err)
	assert.True(t, exists)
}

// TestFileUpgradeStateManager_DeleteState tests state deletion.
func TestFileUpgradeStateManager_DeleteState(t *testing.T) {
	tmpDir := t.TempDir()
	manager := persistence.NewFileUpgradeStateManager(tmpDir)
	ctx := context.Background()

	// Save state
	state := ports.NewUpgradeState("v2.0.0", "local", false)
	err := manager.SaveState(ctx, state)
	require.NoError(t, err)

	// Verify exists
	exists, err := manager.StateExists(ctx)
	require.NoError(t, err)
	assert.True(t, exists)

	// Delete
	err = manager.DeleteState(ctx)
	require.NoError(t, err)

	// Verify no longer exists
	exists, err = manager.StateExists(ctx)
	require.NoError(t, err)
	assert.False(t, exists)

	// Delete again should not error (idempotent)
	err = manager.DeleteState(ctx)
	require.NoError(t, err)
}

// TestFileUpgradeStateManager_AtomicWrite tests that save is atomic.
func TestFileUpgradeStateManager_AtomicWrite(t *testing.T) {
	tmpDir := t.TempDir()
	manager := persistence.NewFileUpgradeStateManager(tmpDir)
	ctx := context.Background()

	// Save initial state
	state1 := ports.NewUpgradeState("v1.0.0", "local", false)
	err := manager.SaveState(ctx, state1)
	require.NoError(t, err)

	// Save updated state
	state2 := ports.NewUpgradeState("v2.0.0", "docker", true)
	err = manager.SaveState(ctx, state2)
	require.NoError(t, err)

	// Load should return the updated state
	loaded, err := manager.LoadState(ctx)
	require.NoError(t, err)
	assert.Equal(t, "v2.0.0", loaded.UpgradeName)
	assert.Equal(t, "docker", loaded.Mode)
	assert.True(t, loaded.SkipGovernance)

	// Verify no temp file left behind
	files, err := os.ReadDir(tmpDir)
	require.NoError(t, err)
	for _, f := range files {
		assert.NotContains(t, f.Name(), ".tmp", "temp file should not exist after save")
	}
}

// TestFileUpgradeStateManager_ChecksumValidation tests checksum integrity.
func TestFileUpgradeStateManager_ChecksumValidation(t *testing.T) {
	tmpDir := t.TempDir()
	manager := persistence.NewFileUpgradeStateManager(tmpDir)
	ctx := context.Background()

	// Save state
	state := ports.NewUpgradeState("v2.0.0", "local", false)
	err := manager.SaveState(ctx, state)
	require.NoError(t, err)

	// Tamper with the file by modifying a field
	statePath := filepath.Join(tmpDir, ".upgrade-state.json")
	data, err := os.ReadFile(statePath)
	require.NoError(t, err)

	var tamperedState map[string]interface{}
	err = json.Unmarshal(data, &tamperedState)
	require.NoError(t, err)

	// Modify a field without updating checksum
	tamperedState["upgradeName"] = "tampered-version"
	tamperedData, err := json.MarshalIndent(tamperedState, "", "  ")
	require.NoError(t, err)
	err = os.WriteFile(statePath, tamperedData, 0600)
	require.NoError(t, err)

	// Load should detect corruption
	_, err = manager.LoadState(ctx)
	require.Error(t, err)
	assert.IsType(t, &ports.StateCorruptionError{}, err)
	assert.Contains(t, err.Error(), "checksum mismatch")
}

// TestFileUpgradeStateManager_CorruptedJSON tests handling of invalid JSON.
func TestFileUpgradeStateManager_CorruptedJSON(t *testing.T) {
	tmpDir := t.TempDir()
	manager := persistence.NewFileUpgradeStateManager(tmpDir)
	ctx := context.Background()

	// Write invalid JSON
	statePath := filepath.Join(tmpDir, ".upgrade-state.json")
	err := os.WriteFile(statePath, []byte("{invalid json"), 0600)
	require.NoError(t, err)

	// Load should return error
	_, err = manager.LoadState(ctx)
	require.Error(t, err)
	assert.IsType(t, &ports.StateCorruptionError{}, err)
	assert.Contains(t, err.Error(), "invalid JSON")
}

// TestFileUpgradeStateManager_ValidateState tests state validation.
func TestFileUpgradeStateManager_ValidateState(t *testing.T) {
	tmpDir := t.TempDir()
	manager := persistence.NewFileUpgradeStateManager(tmpDir)

	tests := []struct {
		name        string
		state       *ports.UpgradeState
		expectError bool
		errorMsg    string
	}{
		{
			name:        "nil state",
			state:       nil,
			expectError: true,
			errorMsg:    "state is nil",
		},
		{
			name: "valid state",
			state: &ports.UpgradeState{
				Version:      1,
				UpgradeName:  "v2.0.0",
				Stage:        ports.ResumableStageInitialized,
				Mode:         "local",
				CreatedAt:    time.Now(),
				UpdatedAt:    time.Now(),
				StageHistory: []ports.StageTransition{{From: "", To: ports.ResumableStageInitialized}},
			},
			expectError: false,
		},
		{
			name: "invalid version",
			state: &ports.UpgradeState{
				Version:      0,
				UpgradeName:  "v2.0.0",
				Stage:        ports.ResumableStageInitialized,
				Mode:         "local",
				StageHistory: []ports.StageTransition{{From: "", To: ports.ResumableStageInitialized}},
			},
			expectError: true,
			errorMsg:    "invalid schema version",
		},
		{
			name: "empty upgrade name",
			state: &ports.UpgradeState{
				Version:      1,
				UpgradeName:  "",
				Stage:        ports.ResumableStageInitialized,
				Mode:         "local",
				StageHistory: []ports.StageTransition{{From: "", To: ports.ResumableStageInitialized}},
			},
			expectError: true,
			errorMsg:    "upgradeName is required",
		},
		{
			name: "empty stage",
			state: &ports.UpgradeState{
				Version:      1,
				UpgradeName:  "v2.0.0",
				Stage:        "",
				Mode:         "local",
				StageHistory: []ports.StageTransition{{From: "", To: ports.ResumableStageInitialized}},
			},
			expectError: true,
			errorMsg:    "stage is required",
		},
		{
			name: "invalid mode",
			state: &ports.UpgradeState{
				Version:      1,
				UpgradeName:  "v2.0.0",
				Stage:        ports.ResumableStageInitialized,
				Mode:         "invalid",
				StageHistory: []ports.StageTransition{{From: "", To: ports.ResumableStageInitialized}},
			},
			expectError: true,
			errorMsg:    "invalid mode",
		},
		{
			name: "empty stage history",
			state: &ports.UpgradeState{
				Version:      1,
				UpgradeName:  "v2.0.0",
				Stage:        ports.ResumableStageInitialized,
				Mode:         "local",
				StageHistory: []ports.StageTransition{},
			},
			expectError: true,
			errorMsg:    "stageHistory must not be empty",
		},
		{
			name: "invalid first stage history entry",
			state: &ports.UpgradeState{
				Version:     1,
				UpgradeName: "v2.0.0",
				Stage:       ports.ResumableStageInitialized,
				Mode:        "local",
				StageHistory: []ports.StageTransition{
					{From: ports.ResumableStageInitialized, To: ports.ResumableStageVoting},
				},
			},
			expectError: true,
			errorMsg:    "first stageHistory entry must have empty 'from' field",
		},
		{
			name: "validator vote without tx hash",
			state: &ports.UpgradeState{
				Version:     1,
				UpgradeName: "v2.0.0",
				Stage:       ports.ResumableStageVoting,
				Mode:        "local",
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
				StageHistory: []ports.StageTransition{
					{From: "", To: ports.ResumableStageInitialized},
				},
				ValidatorVotes: []ports.ValidatorVoteState{
					{Address: "0x1234", Voted: true, TxHash: ""},
				},
			},
			expectError: true,
			errorMsg:    "txHash required when voted=true",
		},
		{
			name: "node switch without started",
			state: &ports.UpgradeState{
				Version:     1,
				UpgradeName: "v2.0.0",
				Stage:       ports.ResumableStageSwitchingBinary,
				Mode:        "local",
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
				StageHistory: []ports.StageTransition{
					{From: "", To: ports.ResumableStageInitialized},
				},
				NodeSwitches: []ports.NodeSwitchState{
					{NodeName: "node0", Switched: true, Stopped: true, Started: false, NewBinary: "/path"},
				},
			},
			expectError: true,
			errorMsg:    "stopped and started must be true when switched=true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.ValidateState(tt.state)
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestFileUpgradeStateManager_LockAcquireRelease tests file locking.
func TestFileUpgradeStateManager_LockAcquireRelease(t *testing.T) {
	tmpDir := t.TempDir()
	manager := persistence.NewFileUpgradeStateManager(tmpDir)
	ctx := context.Background()

	// Acquire lock
	err := manager.AcquireLock(ctx)
	require.NoError(t, err)

	// Release lock
	err = manager.ReleaseLock(ctx)
	require.NoError(t, err)

	// Can acquire again
	err = manager.AcquireLock(ctx)
	require.NoError(t, err)

	// Release again
	err = manager.ReleaseLock(ctx)
	require.NoError(t, err)
}

// TestFileUpgradeStateManager_LockPreventsDouble tests that lock prevents concurrent access.
func TestFileUpgradeStateManager_LockPreventsDouble(t *testing.T) {
	tmpDir := t.TempDir()
	manager1 := persistence.NewFileUpgradeStateManager(tmpDir)
	manager2 := persistence.NewFileUpgradeStateManager(tmpDir)
	ctx := context.Background()

	// First manager acquires lock
	err := manager1.AcquireLock(ctx)
	require.NoError(t, err)

	// Second manager should fail to acquire
	err = manager2.AcquireLock(ctx)
	require.Error(t, err)

	// Release first lock
	err = manager1.ReleaseLock(ctx)
	require.NoError(t, err)

	// Now second manager can acquire
	err = manager2.AcquireLock(ctx)
	require.NoError(t, err)

	err = manager2.ReleaseLock(ctx)
	require.NoError(t, err)
}

// TestFileUpgradeStateManager_LockErrorWithState tests that lock error includes state info.
func TestFileUpgradeStateManager_LockErrorWithState(t *testing.T) {
	tmpDir := t.TempDir()
	manager1 := persistence.NewFileUpgradeStateManager(tmpDir)
	manager2 := persistence.NewFileUpgradeStateManager(tmpDir)
	ctx := context.Background()

	// Save state and acquire lock with first manager
	state := ports.NewUpgradeState("v2.0.0", "local", false)
	err := manager1.SaveState(ctx, state)
	require.NoError(t, err)

	err = manager1.AcquireLock(ctx)
	require.NoError(t, err)

	// Second manager should get informative error
	err = manager2.AcquireLock(ctx)
	require.Error(t, err)
	var upgradeErr *ports.UpgradeInProgressError
	require.ErrorAs(t, err, &upgradeErr)
	assert.Equal(t, "v2.0.0", upgradeErr.UpgradeName)
	assert.Equal(t, ports.ResumableStageInitialized, upgradeErr.Stage)

	err = manager1.ReleaseLock(ctx)
	require.NoError(t, err)
}

// TestFileUpgradeStateManager_ValidatorVotes tests tracking validator votes.
func TestFileUpgradeStateManager_ValidatorVotes(t *testing.T) {
	tmpDir := t.TempDir()
	manager := persistence.NewFileUpgradeStateManager(tmpDir)
	ctx := context.Background()

	state := ports.NewUpgradeState("v2.0.0", "local", false)
	now := time.Now()
	state.ValidatorVotes = []ports.ValidatorVoteState{
		{Address: "0x1234", Moniker: "validator-0", Voted: true, TxHash: "0xabcd", Timestamp: &now},
		{Address: "0x5678", Moniker: "validator-1", Voted: false},
	}

	err := manager.SaveState(ctx, state)
	require.NoError(t, err)

	loaded, err := manager.LoadState(ctx)
	require.NoError(t, err)
	require.Len(t, loaded.ValidatorVotes, 2)

	assert.Equal(t, "0x1234", loaded.ValidatorVotes[0].Address)
	assert.True(t, loaded.ValidatorVotes[0].Voted)
	assert.Equal(t, "0xabcd", loaded.ValidatorVotes[0].TxHash)

	assert.Equal(t, "0x5678", loaded.ValidatorVotes[1].Address)
	assert.False(t, loaded.ValidatorVotes[1].Voted)
}

// TestFileUpgradeStateManager_NodeSwitches tests tracking node switches.
func TestFileUpgradeStateManager_NodeSwitches(t *testing.T) {
	tmpDir := t.TempDir()
	manager := persistence.NewFileUpgradeStateManager(tmpDir)
	ctx := context.Background()

	state := ports.NewUpgradeState("v2.0.0", "docker", false)
	now := time.Now()
	state.NodeSwitches = []ports.NodeSwitchState{
		{NodeName: "node0", Switched: true, Stopped: true, Started: true, OldBinary: "old:v1", NewBinary: "new:v2", Timestamp: &now},
		{NodeName: "node1", Switched: false, Stopped: false, Started: false},
	}

	err := manager.SaveState(ctx, state)
	require.NoError(t, err)

	loaded, err := manager.LoadState(ctx)
	require.NoError(t, err)
	require.Len(t, loaded.NodeSwitches, 2)

	assert.Equal(t, "node0", loaded.NodeSwitches[0].NodeName)
	assert.True(t, loaded.NodeSwitches[0].Switched)
	assert.Equal(t, "new:v2", loaded.NodeSwitches[0].NewBinary)

	assert.Equal(t, "node1", loaded.NodeSwitches[1].NodeName)
	assert.False(t, loaded.NodeSwitches[1].Switched)
}

// TestFileUpgradeStateManager_StageHistory tests stage history tracking.
func TestFileUpgradeStateManager_StageHistory(t *testing.T) {
	tmpDir := t.TempDir()
	manager := persistence.NewFileUpgradeStateManager(tmpDir)
	ctx := context.Background()

	state := ports.NewUpgradeState("v2.0.0", "local", false)

	// Add more transitions
	state.StageHistory = append(state.StageHistory, ports.StageTransition{
		From:      ports.ResumableStageInitialized,
		To:        ports.ResumableStageProposalSubmitted,
		Timestamp: time.Now(),
		Reason:    "proposal 42 submitted",
	})
	state.Stage = ports.ResumableStageProposalSubmitted

	err := manager.SaveState(ctx, state)
	require.NoError(t, err)

	loaded, err := manager.LoadState(ctx)
	require.NoError(t, err)
	require.Len(t, loaded.StageHistory, 2)

	assert.Equal(t, "", string(loaded.StageHistory[0].From))
	assert.Equal(t, ports.ResumableStageInitialized, loaded.StageHistory[0].To)
	assert.Equal(t, "upgrade initiated", loaded.StageHistory[0].Reason)

	assert.Equal(t, ports.ResumableStageInitialized, loaded.StageHistory[1].From)
	assert.Equal(t, ports.ResumableStageProposalSubmitted, loaded.StageHistory[1].To)
	assert.Equal(t, "proposal 42 submitted", loaded.StageHistory[1].Reason)
}

// TestFileUpgradeStateManager_Persistence tests that state survives "process restart".
func TestFileUpgradeStateManager_Persistence(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	// Create and save state with first manager instance
	manager1 := persistence.NewFileUpgradeStateManager(tmpDir)
	state := ports.NewUpgradeState("v2.0.0", "docker", true)
	state.TargetImage = "myrepo/myimage:v2.0.0"
	err := manager1.SaveState(ctx, state)
	require.NoError(t, err)

	// Create new manager instance (simulates process restart)
	manager2 := persistence.NewFileUpgradeStateManager(tmpDir)

	// Load state with new instance
	loaded, err := manager2.LoadState(ctx)
	require.NoError(t, err)
	require.NotNil(t, loaded)

	assert.Equal(t, "v2.0.0", loaded.UpgradeName)
	assert.Equal(t, "docker", loaded.Mode)
	assert.True(t, loaded.SkipGovernance)
	assert.Equal(t, "myrepo/myimage:v2.0.0", loaded.TargetImage)
}
