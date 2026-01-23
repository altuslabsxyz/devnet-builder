package unit

import (
	"testing"
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/application/ports"
	"github.com/altuslabsxyz/devnet-builder/internal/application/upgrade"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStateTransitioner_CanTransition tests valid and invalid transitions.
func TestStateTransitioner_CanTransition(t *testing.T) {
	transitioner := upgrade.NewStateTransitioner()

	tests := []struct {
		name     string
		from     ports.ResumableStage
		to       ports.ResumableStage
		expected bool
	}{
		// Governance path valid transitions
		{"initialized to proposal_submitted", ports.ResumableStageInitialized, ports.ResumableStageProposalSubmitted, true},
		{"initialized to switching_binary (skip-gov)", ports.ResumableStageInitialized, ports.ResumableStageSwitchingBinary, true},
		{"proposal_submitted to voting", ports.ResumableStageProposalSubmitted, ports.ResumableStageVoting, true},
		{"voting to waiting_for_height", ports.ResumableStageVoting, ports.ResumableStageWaitingForHeight, true},
		{"voting to proposal_rejected", ports.ResumableStageVoting, ports.ResumableStageProposalRejected, true},
		{"waiting_for_height to chain_halted", ports.ResumableStageWaitingForHeight, ports.ResumableStageChainHalted, true},
		{"chain_halted to switching_binary", ports.ResumableStageChainHalted, ports.ResumableStageSwitchingBinary, true},
		{"switching_binary to verifying_resume", ports.ResumableStageSwitchingBinary, ports.ResumableStageVerifyingResume, true},
		{"verifying_resume to completed", ports.ResumableStageVerifyingResume, ports.ResumableStageCompleted, true},

		// All stages can go to failed (except terminals)
		{"initialized to failed", ports.ResumableStageInitialized, ports.ResumableStageFailed, true},
		{"proposal_submitted to failed", ports.ResumableStageProposalSubmitted, ports.ResumableStageFailed, true},
		{"voting to failed", ports.ResumableStageVoting, ports.ResumableStageFailed, true},
		{"waiting_for_height to failed", ports.ResumableStageWaitingForHeight, ports.ResumableStageFailed, true},
		{"chain_halted to failed", ports.ResumableStageChainHalted, ports.ResumableStageFailed, true},
		{"switching_binary to failed", ports.ResumableStageSwitchingBinary, ports.ResumableStageFailed, true},
		{"verifying_resume to failed", ports.ResumableStageVerifyingResume, ports.ResumableStageFailed, true},

		// Invalid transitions - skipping stages
		{"initialized to voting (skip)", ports.ResumableStageInitialized, ports.ResumableStageVoting, false},
		{"initialized to chain_halted (skip)", ports.ResumableStageInitialized, ports.ResumableStageChainHalted, false},
		{"proposal_submitted to chain_halted (skip)", ports.ResumableStageProposalSubmitted, ports.ResumableStageChainHalted, false},

		// Invalid transitions - backward transitions
		{"voting to proposal_submitted (backward)", ports.ResumableStageVoting, ports.ResumableStageProposalSubmitted, false},
		{"completed to voting (backward)", ports.ResumableStageCompleted, ports.ResumableStageVoting, false},

		// Terminal stages cannot transition
		{"completed to anything", ports.ResumableStageCompleted, ports.ResumableStageInitialized, false},
		{"failed to anything", ports.ResumableStageFailed, ports.ResumableStageInitialized, false},
		{"proposal_rejected to anything", ports.ResumableStageProposalRejected, ports.ResumableStageInitialized, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := transitioner.CanTransition(tt.from, tt.to)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestStateTransitioner_TransitionTo tests state transitions.
func TestStateTransitioner_TransitionTo(t *testing.T) {
	transitioner := upgrade.NewStateTransitioner()

	t.Run("valid transition updates state", func(t *testing.T) {
		state := ports.NewUpgradeState("v2.0.0", "local", false)
		originalUpdatedAt := state.UpdatedAt

		// Wait a moment to ensure timestamp differs
		time.Sleep(time.Millisecond)

		err := transitioner.TransitionTo(state, ports.ResumableStageProposalSubmitted, "proposal 42 submitted")
		require.NoError(t, err)

		assert.Equal(t, ports.ResumableStageProposalSubmitted, state.Stage)
		assert.True(t, state.UpdatedAt.After(originalUpdatedAt))

		// Verify history recorded
		require.Len(t, state.StageHistory, 2)
		assert.Equal(t, ports.ResumableStageInitialized, state.StageHistory[1].From)
		assert.Equal(t, ports.ResumableStageProposalSubmitted, state.StageHistory[1].To)
		assert.Equal(t, "proposal 42 submitted", state.StageHistory[1].Reason)
	})

	t.Run("invalid transition returns error", func(t *testing.T) {
		state := ports.NewUpgradeState("v2.0.0", "local", false)

		err := transitioner.TransitionTo(state, ports.ResumableStageVoting, "invalid jump")
		require.Error(t, err)

		var invalidErr *ports.InvalidTransitionError
		require.ErrorAs(t, err, &invalidErr)
		assert.Equal(t, ports.ResumableStageInitialized, invalidErr.From)
		assert.Equal(t, ports.ResumableStageVoting, invalidErr.To)

		// State should not change
		assert.Equal(t, ports.ResumableStageInitialized, state.Stage)
		assert.Len(t, state.StageHistory, 1) // Only initial entry
	})

	t.Run("transition to failed sets error field", func(t *testing.T) {
		state := ports.NewUpgradeState("v2.0.0", "local", false)

		err := transitioner.TransitionTo(state, ports.ResumableStageFailed, "connection timeout")
		require.NoError(t, err)

		assert.Equal(t, ports.ResumableStageFailed, state.Stage)
		assert.Equal(t, "connection timeout", state.Error)
	})

	t.Run("nil state returns error", func(t *testing.T) {
		err := transitioner.TransitionTo(nil, ports.ResumableStageProposalSubmitted, "test")
		require.Error(t, err)
	})
}

// TestStateTransitioner_GetValidTransitions tests getting valid transitions.
func TestStateTransitioner_GetValidTransitions(t *testing.T) {
	transitioner := upgrade.NewStateTransitioner()

	tests := []struct {
		name          string
		from          ports.ResumableStage
		expectedCount int
		mustInclude   []ports.ResumableStage
	}{
		{
			name:          "initialized has multiple paths",
			from:          ports.ResumableStageInitialized,
			expectedCount: 3, // ProposalSubmitted, SwitchingBinary, Failed
			mustInclude:   []ports.ResumableStage{ports.ResumableStageProposalSubmitted, ports.ResumableStageSwitchingBinary},
		},
		{
			name:          "voting can pass, reject, or fail",
			from:          ports.ResumableStageVoting,
			expectedCount: 3,
			mustInclude:   []ports.ResumableStage{ports.ResumableStageWaitingForHeight, ports.ResumableStageProposalRejected, ports.ResumableStageFailed},
		},
		{
			name:          "completed has no transitions",
			from:          ports.ResumableStageCompleted,
			expectedCount: 0,
			mustInclude:   []ports.ResumableStage{},
		},
		{
			name:          "failed has no transitions",
			from:          ports.ResumableStageFailed,
			expectedCount: 0,
			mustInclude:   []ports.ResumableStage{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transitions := transitioner.GetValidTransitions(tt.from)
			assert.Len(t, transitions, tt.expectedCount)

			for _, expected := range tt.mustInclude {
				assert.Contains(t, transitions, expected)
			}
		})
	}
}

// TestStateTransitioner_GovernancePath tests full governance path transitions.
func TestStateTransitioner_GovernancePath(t *testing.T) {
	transitioner := upgrade.NewStateTransitioner()
	state := ports.NewUpgradeState("v2.0.0", "local", false)

	stages := []ports.ResumableStage{
		ports.ResumableStageProposalSubmitted,
		ports.ResumableStageVoting,
		ports.ResumableStageWaitingForHeight,
		ports.ResumableStageChainHalted,
		ports.ResumableStageSwitchingBinary,
		ports.ResumableStageVerifyingResume,
		ports.ResumableStageCompleted,
	}

	for _, stage := range stages {
		err := transitioner.TransitionTo(state, stage, "test transition to "+stage.String())
		require.NoError(t, err, "failed to transition to %s", stage)
		assert.Equal(t, stage, state.Stage)
	}

	// Verify history has all transitions
	assert.Len(t, state.StageHistory, len(stages)+1) // +1 for initial entry
}

// TestStateTransitioner_SkipGovPath tests skip-governance path transitions.
func TestStateTransitioner_SkipGovPath(t *testing.T) {
	transitioner := upgrade.NewStateTransitioner()
	state := ports.NewUpgradeState("v2.0.0", "local", true)

	stages := []ports.ResumableStage{
		ports.ResumableStageSwitchingBinary,
		ports.ResumableStageVerifyingResume,
		ports.ResumableStageCompleted,
	}

	for _, stage := range stages {
		err := transitioner.TransitionTo(state, stage, "test transition to "+stage.String())
		require.NoError(t, err, "failed to transition to %s", stage)
		assert.Equal(t, stage, state.Stage)
	}

	// Verify history has all transitions
	assert.Len(t, state.StageHistory, len(stages)+1) // +1 for initial entry
}

// TestStateTransitioner_TerminalStages tests terminal stages behavior.
func TestStateTransitioner_TerminalStages(t *testing.T) {
	transitioner := upgrade.NewStateTransitioner()

	terminalStages := []ports.ResumableStage{
		ports.ResumableStageCompleted,
		ports.ResumableStageFailed,
		ports.ResumableStageProposalRejected,
	}

	for _, terminal := range terminalStages {
		t.Run(terminal.String()+"_is_terminal", func(t *testing.T) {
			state := ports.NewUpgradeState("v2.0.0", "local", false)
			state.Stage = terminal

			// Try to transition to any non-terminal stage
			err := transitioner.TransitionTo(state, ports.ResumableStageInitialized, "try to escape")
			require.Error(t, err)

			// Stage should not change
			assert.Equal(t, terminal, state.Stage)
		})
	}
}

// TestStateTransitioner_IsTerminal tests the IsTerminal helper.
func TestStateTransitioner_IsTerminal(t *testing.T) {
	tests := []struct {
		stage    ports.ResumableStage
		terminal bool
	}{
		{ports.ResumableStageInitialized, false},
		{ports.ResumableStageProposalSubmitted, false},
		{ports.ResumableStageVoting, false},
		{ports.ResumableStageWaitingForHeight, false},
		{ports.ResumableStageChainHalted, false},
		{ports.ResumableStageSwitchingBinary, false},
		{ports.ResumableStageVerifyingResume, false},
		{ports.ResumableStageCompleted, true},
		{ports.ResumableStageFailed, true},
		{ports.ResumableStageProposalRejected, true},
	}

	for _, tt := range tests {
		t.Run(tt.stage.String(), func(t *testing.T) {
			assert.Equal(t, tt.terminal, tt.stage.IsTerminal())
		})
	}
}

// TestStateTransitioner_GetNextStageForGovPath tests governance path helper.
func TestStateTransitioner_GetNextStageForGovPath(t *testing.T) {
	transitioner := upgrade.NewStateTransitioner()

	tests := []struct {
		current  ports.ResumableStage
		expected ports.ResumableStage
	}{
		{ports.ResumableStageInitialized, ports.ResumableStageProposalSubmitted},
		{ports.ResumableStageProposalSubmitted, ports.ResumableStageVoting},
		{ports.ResumableStageVoting, ports.ResumableStageWaitingForHeight},
		{ports.ResumableStageWaitingForHeight, ports.ResumableStageChainHalted},
		{ports.ResumableStageChainHalted, ports.ResumableStageSwitchingBinary},
		{ports.ResumableStageSwitchingBinary, ports.ResumableStageVerifyingResume},
		{ports.ResumableStageVerifyingResume, ports.ResumableStageCompleted},
		{ports.ResumableStageCompleted, ""},
	}

	for _, tt := range tests {
		t.Run(tt.current.String(), func(t *testing.T) {
			next := transitioner.GetNextStageForGovPath(tt.current)
			assert.Equal(t, tt.expected, next)
		})
	}
}

// TestStateTransitioner_GetNextStageForSkipGovPath tests skip-gov path helper.
func TestStateTransitioner_GetNextStageForSkipGovPath(t *testing.T) {
	transitioner := upgrade.NewStateTransitioner()

	tests := []struct {
		current  ports.ResumableStage
		expected ports.ResumableStage
	}{
		{ports.ResumableStageInitialized, ports.ResumableStageSwitchingBinary},
		{ports.ResumableStageSwitchingBinary, ports.ResumableStageVerifyingResume},
		{ports.ResumableStageVerifyingResume, ports.ResumableStageCompleted},
		{ports.ResumableStageCompleted, ""},
	}

	for _, tt := range tests {
		t.Run(tt.current.String(), func(t *testing.T) {
			next := transitioner.GetNextStageForSkipGovPath(tt.current)
			assert.Equal(t, tt.expected, next)
		})
	}
}

// TestStateTransitioner_MultipleTransitions tests complex state journey.
func TestStateTransitioner_MultipleTransitions(t *testing.T) {
	transitioner := upgrade.NewStateTransitioner()
	state := ports.NewUpgradeState("v2.0.0", "local", false)

	// Simulate a journey with some successful steps then a failure
	err := transitioner.TransitionTo(state, ports.ResumableStageProposalSubmitted, "proposal submitted")
	require.NoError(t, err)

	err = transitioner.TransitionTo(state, ports.ResumableStageVoting, "voting started")
	require.NoError(t, err)

	// Proposal gets rejected
	err = transitioner.TransitionTo(state, ports.ResumableStageProposalRejected, "proposal rejected by vote")
	require.NoError(t, err)

	// Verify final state
	assert.Equal(t, ports.ResumableStageProposalRejected, state.Stage)
	assert.True(t, state.Stage.IsTerminal())

	// Verify complete history
	assert.Len(t, state.StageHistory, 4) // Initial + 3 transitions
	assert.Equal(t, ports.ResumableStageProposalRejected, state.StageHistory[3].To)
	assert.Equal(t, "proposal rejected by vote", state.StageHistory[3].Reason)
}
