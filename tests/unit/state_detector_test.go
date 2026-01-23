package unit

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/application/ports"
	"github.com/altuslabsxyz/devnet-builder/internal/application/upgrade"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockRPCClient implements ports.RPCClient for testing.
type MockRPCClient struct {
	BlockHeight   int64
	BlockHeights  []int64 // For simulating changing heights
	heightCallIdx int
	ChainRunning  bool
	Proposal      *ports.Proposal
	ProposalErr   error
	UpgradePlan   *ports.UpgradePlan
	GovParams     *ports.GovParams
}

func (m *MockRPCClient) GetBlockHeight(ctx context.Context) (int64, error) {
	if len(m.BlockHeights) > 0 {
		if m.heightCallIdx < len(m.BlockHeights) {
			h := m.BlockHeights[m.heightCallIdx]
			m.heightCallIdx++
			return h, nil
		}
		return m.BlockHeights[len(m.BlockHeights)-1], nil
	}
	return m.BlockHeight, nil
}

func (m *MockRPCClient) GetBlockTime(ctx context.Context, sampleSize int) (time.Duration, error) {
	return time.Second, nil
}

func (m *MockRPCClient) IsChainRunning(ctx context.Context) bool {
	return m.ChainRunning
}

func (m *MockRPCClient) WaitForBlock(ctx context.Context, height int64) error {
	return nil
}

func (m *MockRPCClient) GetProposal(ctx context.Context, id uint64) (*ports.Proposal, error) {
	if m.ProposalErr != nil {
		return nil, m.ProposalErr
	}
	return m.Proposal, nil
}

func (m *MockRPCClient) GetUpgradePlan(ctx context.Context) (*ports.UpgradePlan, error) {
	return m.UpgradePlan, nil
}

func (m *MockRPCClient) GetAppVersion(ctx context.Context) (string, error) {
	return "v1.0.0", nil
}

func (m *MockRPCClient) GetGovParams(ctx context.Context) (*ports.GovParams, error) {
	return m.GovParams, nil
}

// TestStateDetector_DetectProposalStatus tests proposal status detection.
func TestStateDetector_DetectProposalStatus(t *testing.T) {
	tests := []struct {
		name           string
		proposalID     uint64
		proposal       *ports.Proposal
		proposalErr    error
		expectedStatus string
		expectErr      bool
	}{
		{
			name:       "voting status",
			proposalID: 1,
			proposal: &ports.Proposal{
				ID:     1,
				Status: ports.ProposalStatusVoting,
			},
			expectedStatus: "voting",
		},
		{
			name:       "passed status",
			proposalID: 2,
			proposal: &ports.Proposal{
				ID:     2,
				Status: ports.ProposalStatusPassed,
			},
			expectedStatus: "passed",
		},
		{
			name:       "rejected status",
			proposalID: 3,
			proposal: &ports.Proposal{
				ID:     3,
				Status: ports.ProposalStatusRejected,
			},
			expectedStatus: "rejected",
		},
		{
			name:       "failed status",
			proposalID: 4,
			proposal: &ports.Proposal{
				ID:     4,
				Status: ports.ProposalStatusFailed,
			},
			expectedStatus: "failed",
		},
		{
			name:       "pending status",
			proposalID: 5,
			proposal: &ports.Proposal{
				ID:     5,
				Status: ports.ProposalStatusPending,
			},
			expectedStatus: "pending",
		},
		{
			name:           "invalid proposal ID",
			proposalID:     0,
			expectedStatus: "unknown",
			expectErr:      true,
		},
		{
			name:           "proposal query error",
			proposalID:     1,
			proposalErr:    fmt.Errorf("network error"),
			expectedStatus: "unknown",
			expectErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRPC := &MockRPCClient{
				Proposal:    tt.proposal,
				ProposalErr: tt.proposalErr,
			}
			detector := upgrade.NewStateDetector(mockRPC)

			status, err := detector.DetectProposalStatus(context.Background(), tt.proposalID)

			if tt.expectErr {
				require.Error(t, err)
			}
			assert.Equal(t, tt.expectedStatus, status)
		})
	}
}

// TestStateDetector_DetectChainStatus tests chain status detection.
func TestStateDetector_DetectChainStatus(t *testing.T) {
	t.Run("chain unreachable", func(t *testing.T) {
		mockRPC := &MockRPCClient{
			ChainRunning: false,
		}
		detector := upgrade.NewStateDetector(mockRPC)

		status, err := detector.DetectChainStatus(context.Background())
		require.NoError(t, err)
		assert.Equal(t, "unreachable", status)
	})

	// Note: Testing "halted" and "running" states requires timing-based tests
	// which are better suited for integration tests. The logic relies on
	// comparing block heights after a delay, which is hard to mock accurately.
}

// TestStateDetector_DetectCurrentStage_NilState tests nil state handling.
func TestStateDetector_DetectCurrentStage_NilState(t *testing.T) {
	mockRPC := &MockRPCClient{}
	detector := upgrade.NewStateDetector(mockRPC)

	_, err := detector.DetectCurrentStage(context.Background(), nil)
	require.Error(t, err)
}

// TestStateDetector_DetectCurrentStage_Initialized tests initialized state detection.
func TestStateDetector_DetectCurrentStage_Initialized(t *testing.T) {
	mockRPC := &MockRPCClient{}
	detector := upgrade.NewStateDetector(mockRPC)

	state := ports.NewUpgradeState("v2.0.0", "local", false)
	// No proposal ID means initialized

	stage, err := detector.DetectCurrentStage(context.Background(), state)
	require.NoError(t, err)
	assert.Equal(t, ports.ResumableStageInitialized, stage)
}

// TestStateDetector_DetectCurrentStage_SkipGov tests skip-governance stage detection.
func TestStateDetector_DetectCurrentStage_SkipGov(t *testing.T) {
	t.Run("no switches recorded", func(t *testing.T) {
		mockRPC := &MockRPCClient{}
		detector := upgrade.NewStateDetector(mockRPC)

		state := ports.NewUpgradeState("v2.0.0", "local", true)

		stage, err := detector.DetectCurrentStage(context.Background(), state)
		require.NoError(t, err)
		assert.Equal(t, ports.ResumableStageInitialized, stage)
	})

	t.Run("some switches recorded", func(t *testing.T) {
		mockRPC := &MockRPCClient{}
		detector := upgrade.NewStateDetector(mockRPC)

		state := ports.NewUpgradeState("v2.0.0", "local", true)
		state.NodeSwitches = []ports.NodeSwitchState{
			{NodeName: "node0", Switched: false},
			{NodeName: "node1", Switched: false},
		}

		stage, err := detector.DetectCurrentStage(context.Background(), state)
		require.NoError(t, err)
		assert.Equal(t, ports.ResumableStageSwitchingBinary, stage)
	})
}

// TestStateDetector_DetectCurrentStage_GovPath tests governance path stage detection.
func TestStateDetector_DetectCurrentStage_GovPath(t *testing.T) {
	t.Run("proposal voting", func(t *testing.T) {
		mockRPC := &MockRPCClient{
			Proposal: &ports.Proposal{
				ID:     1,
				Status: ports.ProposalStatusVoting,
			},
		}
		detector := upgrade.NewStateDetector(mockRPC)

		state := ports.NewUpgradeState("v2.0.0", "local", false)
		state.ProposalID = 1

		stage, err := detector.DetectCurrentStage(context.Background(), state)
		require.NoError(t, err)
		assert.Equal(t, ports.ResumableStageVoting, stage)
	})

	t.Run("proposal rejected", func(t *testing.T) {
		mockRPC := &MockRPCClient{
			Proposal: &ports.Proposal{
				ID:     1,
				Status: ports.ProposalStatusRejected,
			},
		}
		detector := upgrade.NewStateDetector(mockRPC)

		state := ports.NewUpgradeState("v2.0.0", "local", false)
		state.ProposalID = 1

		stage, err := detector.DetectCurrentStage(context.Background(), state)
		require.NoError(t, err)
		assert.Equal(t, ports.ResumableStageProposalRejected, stage)
	})

	t.Run("proposal pending", func(t *testing.T) {
		mockRPC := &MockRPCClient{
			Proposal: &ports.Proposal{
				ID:     1,
				Status: ports.ProposalStatusPending,
			},
		}
		detector := upgrade.NewStateDetector(mockRPC)

		state := ports.NewUpgradeState("v2.0.0", "local", false)
		state.ProposalID = 1

		stage, err := detector.DetectCurrentStage(context.Background(), state)
		require.NoError(t, err)
		assert.Equal(t, ports.ResumableStageProposalSubmitted, stage)
	})

	t.Run("proposal query fails fallback to saved state", func(t *testing.T) {
		mockRPC := &MockRPCClient{
			ProposalErr: fmt.Errorf("network error"),
		}
		detector := upgrade.NewStateDetector(mockRPC)

		state := ports.NewUpgradeState("v2.0.0", "local", false)
		state.ProposalID = 1
		state.Stage = ports.ResumableStageVoting

		stage, err := detector.DetectCurrentStage(context.Background(), state)
		require.NoError(t, err)
		// Should return saved stage when query fails
		assert.Equal(t, ports.ResumableStageVoting, stage)
	})
}

// TestStateDetector_DetectValidatorVotes tests validator vote detection.
func TestStateDetector_DetectValidatorVotes(t *testing.T) {
	t.Run("invalid proposal ID", func(t *testing.T) {
		mockRPC := &MockRPCClient{}
		detector := upgrade.NewStateDetector(mockRPC)

		_, err := detector.DetectValidatorVotes(context.Background(), 0)
		require.Error(t, err)
	})

	t.Run("valid proposal ID returns empty slice", func(t *testing.T) {
		mockRPC := &MockRPCClient{}
		detector := upgrade.NewStateDetector(mockRPC)

		votes, err := detector.DetectValidatorVotes(context.Background(), 1)
		require.NoError(t, err)
		assert.Empty(t, votes)
	})
}
