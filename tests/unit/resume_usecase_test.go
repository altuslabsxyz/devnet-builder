package unit

import (
	"context"
	"testing"

	"github.com/altuslabsxyz/devnet-builder/internal/application/dto"
	"github.com/altuslabsxyz/devnet-builder/internal/application/ports"
	"github.com/altuslabsxyz/devnet-builder/internal/application/upgrade"
	"github.com/altuslabsxyz/devnet-builder/internal/output"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockStateManager implements ports.UpgradeStateManager for testing.
type MockStateManager struct {
	State       *ports.UpgradeState
	LoadErr     error
	SaveErr     error
	ValidateErr error
	DeleteErr   error
	Locked      bool
}

func (m *MockStateManager) LoadState(ctx context.Context) (*ports.UpgradeState, error) {
	if m.LoadErr != nil {
		return nil, m.LoadErr
	}
	return m.State, nil
}

func (m *MockStateManager) SaveState(ctx context.Context, state *ports.UpgradeState) error {
	if m.SaveErr != nil {
		return m.SaveErr
	}
	m.State = state
	return nil
}

func (m *MockStateManager) DeleteState(ctx context.Context) error {
	if m.DeleteErr != nil {
		return m.DeleteErr
	}
	m.State = nil
	return nil
}

func (m *MockStateManager) StateExists(ctx context.Context) (bool, error) {
	return m.State != nil, nil
}

func (m *MockStateManager) ValidateState(state *ports.UpgradeState) error {
	return m.ValidateErr
}

func (m *MockStateManager) AcquireLock(ctx context.Context) error {
	m.Locked = true
	return nil
}

func (m *MockStateManager) ReleaseLock(ctx context.Context) error {
	m.Locked = false
	return nil
}

// MockStateDetector implements ports.UpgradeStateDetector for testing.
type MockStateDetector struct {
	DetectedStage  ports.ResumableStage
	DetectErr      error
	ProposalStatus string
	ChainStatus    string
}

func (m *MockStateDetector) DetectCurrentStage(ctx context.Context, state *ports.UpgradeState) (ports.ResumableStage, error) {
	if m.DetectErr != nil {
		return "", m.DetectErr
	}
	if m.DetectedStage != "" {
		return m.DetectedStage, nil
	}
	return state.Stage, nil
}

func (m *MockStateDetector) DetectProposalStatus(ctx context.Context, proposalID uint64) (string, error) {
	return m.ProposalStatus, nil
}

func (m *MockStateDetector) DetectChainStatus(ctx context.Context) (string, error) {
	return m.ChainStatus, nil
}

func (m *MockStateDetector) DetectValidatorVotes(ctx context.Context, proposalID uint64) ([]ports.ValidatorVoteState, error) {
	return nil, nil
}

// TestResumeUseCase_CheckState tests state checking.
func TestResumeUseCase_CheckState(t *testing.T) {
	t.Run("no state exists", func(t *testing.T) {
		mockManager := &MockStateManager{State: nil}
		mockDetector := &MockStateDetector{}
		transitioner := upgrade.NewStateTransitioner()
		logger := output.DefaultLogger

		uc := upgrade.NewResumeUseCase(mockManager, mockDetector, transitioner, nil, logger)

		state, err := uc.CheckState(context.Background())
		require.NoError(t, err)
		assert.Nil(t, state)
	})

	t.Run("state exists", func(t *testing.T) {
		existingState := ports.NewUpgradeState("v2.0.0", "local", false)
		mockManager := &MockStateManager{State: existingState}
		mockDetector := &MockStateDetector{}
		transitioner := upgrade.NewStateTransitioner()
		logger := output.DefaultLogger

		uc := upgrade.NewResumeUseCase(mockManager, mockDetector, transitioner, nil, logger)

		state, err := uc.CheckState(context.Background())
		require.NoError(t, err)
		assert.NotNil(t, state)
		assert.Equal(t, "v2.0.0", state.UpgradeName)
	})
}

// TestResumeUseCase_ClearState tests state clearing.
func TestResumeUseCase_ClearState(t *testing.T) {
	existingState := ports.NewUpgradeState("v2.0.0", "local", false)
	mockManager := &MockStateManager{State: existingState}
	mockDetector := &MockStateDetector{}
	transitioner := upgrade.NewStateTransitioner()
	logger := output.DefaultLogger

	uc := upgrade.NewResumeUseCase(mockManager, mockDetector, transitioner, nil, logger)

	err := uc.ClearState(context.Background())
	require.NoError(t, err)
	assert.Nil(t, mockManager.State)
}

// TestResumeUseCase_Resume_ClearStateOption tests the clear state option.
func TestResumeUseCase_Resume_ClearStateOption(t *testing.T) {
	existingState := ports.NewUpgradeState("v2.0.0", "local", false)
	mockManager := &MockStateManager{State: existingState}
	mockDetector := &MockStateDetector{}
	transitioner := upgrade.NewStateTransitioner()
	logger := output.DefaultLogger

	uc := upgrade.NewResumeUseCase(mockManager, mockDetector, transitioner, nil, logger)

	options := ports.ResumeOptions{ClearState: true}
	result, err := uc.Resume(context.Background(), dto.ExecuteUpgradeInput{}, options)

	require.NoError(t, err)
	assert.False(t, result.Resumed)
	assert.Equal(t, "State cleared successfully", result.Message)
	assert.Nil(t, mockManager.State)
}

// TestResumeUseCase_Resume_ShowStatusOption tests the show status option.
func TestResumeUseCase_Resume_ShowStatusOption(t *testing.T) {
	existingState := ports.NewUpgradeState("v2.0.0", "local", false)
	existingState.Stage = ports.ResumableStageVoting
	mockManager := &MockStateManager{State: existingState}
	mockDetector := &MockStateDetector{}
	transitioner := upgrade.NewStateTransitioner()
	logger := output.DefaultLogger

	uc := upgrade.NewResumeUseCase(mockManager, mockDetector, transitioner, nil, logger)

	options := ports.ResumeOptions{ShowStatus: true}
	result, err := uc.Resume(context.Background(), dto.ExecuteUpgradeInput{}, options)

	require.NoError(t, err)
	assert.False(t, result.Resumed)
	assert.NotNil(t, result.State)
	assert.Equal(t, ports.ResumableStageVoting, result.State.Stage)
	assert.Equal(t, "Current upgrade state", result.Message)
}

// TestResumeUseCase_Resume_ForceRestartOption tests the force restart option.
func TestResumeUseCase_Resume_ForceRestartOption(t *testing.T) {
	existingState := ports.NewUpgradeState("v2.0.0", "local", false)
	mockManager := &MockStateManager{State: existingState}
	mockDetector := &MockStateDetector{}
	transitioner := upgrade.NewStateTransitioner()
	logger := output.DefaultLogger

	uc := upgrade.NewResumeUseCase(mockManager, mockDetector, transitioner, nil, logger)

	options := ports.ResumeOptions{ForceRestart: true}
	result, err := uc.Resume(context.Background(), dto.ExecuteUpgradeInput{}, options)

	require.NoError(t, err)
	assert.False(t, result.Resumed)
	assert.Equal(t, "Starting fresh (--force-restart)", result.Message)
	assert.Nil(t, mockManager.State)
}

// TestResumeUseCase_Resume_NoExistingState tests when there's no state to resume.
func TestResumeUseCase_Resume_NoExistingState(t *testing.T) {
	mockManager := &MockStateManager{State: nil}
	mockDetector := &MockStateDetector{}
	transitioner := upgrade.NewStateTransitioner()
	logger := output.DefaultLogger

	uc := upgrade.NewResumeUseCase(mockManager, mockDetector, transitioner, nil, logger)

	options := ports.ResumeOptions{}
	result, err := uc.Resume(context.Background(), dto.ExecuteUpgradeInput{}, options)

	require.NoError(t, err)
	assert.False(t, result.Resumed)
	assert.Equal(t, "No existing upgrade state found", result.Message)
}

// TestResumeUseCase_Resume_TerminalState tests when upgrade is already in terminal state.
func TestResumeUseCase_Resume_TerminalState(t *testing.T) {
	tests := []struct {
		name    string
		stage   ports.ResumableStage
		message string
	}{
		{
			name:    "completed state",
			stage:   ports.ResumableStageCompleted,
			message: "Previous upgrade is in terminal state: Completed",
		},
		{
			name:    "failed state",
			stage:   ports.ResumableStageFailed,
			message: "Previous upgrade is in terminal state: Failed",
		},
		{
			name:    "rejected state",
			stage:   ports.ResumableStageProposalRejected,
			message: "Previous upgrade is in terminal state: ProposalRejected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			existingState := ports.NewUpgradeState("v2.0.0", "local", false)
			existingState.Stage = tt.stage
			mockManager := &MockStateManager{State: existingState}
			mockDetector := &MockStateDetector{}
			transitioner := upgrade.NewStateTransitioner()
			logger := output.DefaultLogger

			uc := upgrade.NewResumeUseCase(mockManager, mockDetector, transitioner, nil, logger)

			options := ports.ResumeOptions{}
			result, err := uc.Resume(context.Background(), dto.ExecuteUpgradeInput{}, options)

			require.NoError(t, err)
			assert.False(t, result.Resumed)
			assert.Equal(t, tt.message, result.Message)
		})
	}
}

// TestResumeUseCase_Resume_CorruptedState tests corrupted state handling.
func TestResumeUseCase_Resume_CorruptedState(t *testing.T) {
	mockManager := &MockStateManager{
		LoadErr: &ports.StateCorruptionError{Reason: "invalid JSON"},
	}
	mockDetector := &MockStateDetector{}
	transitioner := upgrade.NewStateTransitioner()
	logger := output.DefaultLogger

	uc := upgrade.NewResumeUseCase(mockManager, mockDetector, transitioner, nil, logger)

	options := ports.ResumeOptions{}
	_, err := uc.Resume(context.Background(), dto.ExecuteUpgradeInput{}, options)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "corrupted")
}

// TestResumeUseCase_Resume_DetectsStageProgression tests that stage progression is detected.
func TestResumeUseCase_Resume_DetectsStageProgression(t *testing.T) {
	// State saved at ProposalSubmitted but chain shows Voting
	existingState := ports.NewUpgradeState("v2.0.0", "local", false)
	existingState.Stage = ports.ResumableStageProposalSubmitted
	existingState.ProposalID = 1

	mockManager := &MockStateManager{State: existingState}
	mockDetector := &MockStateDetector{
		DetectedStage: ports.ResumableStageVoting,
	}
	transitioner := upgrade.NewStateTransitioner()
	logger := output.DefaultLogger

	// Note: We can't fully test resume execution without a mock ResumableExecuteUpgradeUseCase
	// This test verifies the detection and state update logic

	uc := upgrade.NewResumeUseCase(mockManager, mockDetector, transitioner, nil, logger)

	// Use Reconcile instead of Resume to test detection without execution
	state, err := uc.Reconcile(context.Background())

	require.NoError(t, err)
	assert.NotNil(t, state)
	// State should be updated to Voting
	assert.Equal(t, ports.ResumableStageVoting, state.Stage)
}

// TestResumeUseCase_Reconcile tests the reconcile function.
func TestResumeUseCase_Reconcile(t *testing.T) {
	t.Run("no state to reconcile", func(t *testing.T) {
		mockManager := &MockStateManager{State: nil}
		mockDetector := &MockStateDetector{}
		transitioner := upgrade.NewStateTransitioner()
		logger := output.DefaultLogger

		uc := upgrade.NewResumeUseCase(mockManager, mockDetector, transitioner, nil, logger)

		state, err := uc.Reconcile(context.Background())
		require.NoError(t, err)
		assert.Nil(t, state)
	})

	t.Run("state updated on reconcile", func(t *testing.T) {
		existingState := ports.NewUpgradeState("v2.0.0", "local", false)
		existingState.Stage = ports.ResumableStageVoting
		existingState.ProposalID = 1

		mockManager := &MockStateManager{State: existingState}
		mockDetector := &MockStateDetector{
			DetectedStage: ports.ResumableStageWaitingForHeight,
		}
		transitioner := upgrade.NewStateTransitioner()
		logger := output.DefaultLogger

		uc := upgrade.NewResumeUseCase(mockManager, mockDetector, transitioner, nil, logger)

		state, err := uc.Reconcile(context.Background())
		require.NoError(t, err)
		assert.NotNil(t, state)
		assert.Equal(t, ports.ResumableStageWaitingForHeight, state.Stage)
	})
}
