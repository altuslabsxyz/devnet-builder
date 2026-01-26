// internal/daemon/controller/upgrade_test.go
package controller

import (
	"context"
	"testing"

	"github.com/altuslabsxyz/devnet-builder/internal/daemon/store"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/types"
)

func TestUpgradeController_Reconcile_PendingToProposing(t *testing.T) {
	ms := store.NewMemoryStore()
	uc := NewUpgradeController(ms, nil)

	// Create an upgrade in Pending phase
	upgrade := &types.Upgrade{
		Metadata: types.ResourceMeta{Name: "test-upgrade"},
		Spec: types.UpgradeSpec{
			DevnetRef:    "mydevnet",
			UpgradeName:  "v2.0",
			TargetHeight: 1000,
			NewBinary: types.BinarySource{
				Type:    "cache",
				Version: "v2.0.0",
			},
		},
		Status: types.UpgradeStatus{
			Phase: types.UpgradePhasePending,
		},
	}
	if err := ms.CreateUpgrade(context.Background(), upgrade); err != nil {
		t.Fatalf("CreateUpgrade: %v", err)
	}

	// Reconcile
	err := uc.Reconcile(context.Background(), "test-upgrade")
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	// Verify transition to Proposing
	got, _ := ms.GetUpgrade(context.Background(), "", "test-upgrade")
	if got.Status.Phase != types.UpgradePhaseProposing {
		t.Errorf("Phase = %q, want %q", got.Status.Phase, types.UpgradePhaseProposing)
	}
}

func TestUpgradeController_Reconcile_ProposingToVoting(t *testing.T) {
	ms := store.NewMemoryStore()
	uc := NewUpgradeController(ms, nil)

	// Create an upgrade in Proposing phase
	upgrade := &types.Upgrade{
		Metadata: types.ResourceMeta{Name: "test-upgrade"},
		Spec: types.UpgradeSpec{
			DevnetRef:    "mydevnet",
			UpgradeName:  "v2.0",
			TargetHeight: 1000,
		},
		Status: types.UpgradeStatus{
			Phase: types.UpgradePhaseProposing,
		},
	}
	if err := ms.CreateUpgrade(context.Background(), upgrade); err != nil {
		t.Fatalf("CreateUpgrade: %v", err)
	}

	// Reconcile
	err := uc.Reconcile(context.Background(), "test-upgrade")
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	// Verify transition to Voting
	got, _ := ms.GetUpgrade(context.Background(), "", "test-upgrade")
	if got.Status.Phase != types.UpgradePhaseVoting {
		t.Errorf("Phase = %q, want %q", got.Status.Phase, types.UpgradePhaseVoting)
	}
	if got.Status.ProposalID != 1 {
		t.Errorf("ProposalID = %d, want 1 (simulated)", got.Status.ProposalID)
	}
}

func TestUpgradeController_Reconcile_VotingToWaiting(t *testing.T) {
	ms := store.NewMemoryStore()
	uc := NewUpgradeController(ms, nil)

	// Create an upgrade in Voting phase with votes matching required
	upgrade := &types.Upgrade{
		Metadata: types.ResourceMeta{Name: "test-upgrade"},
		Spec: types.UpgradeSpec{
			DevnetRef:    "mydevnet",
			UpgradeName:  "v2.0",
			TargetHeight: 1000,
		},
		Status: types.UpgradeStatus{
			Phase:         types.UpgradePhaseVoting,
			ProposalID:    1,
			VotesReceived: 3,
			VotesRequired: 3,
		},
	}
	if err := ms.CreateUpgrade(context.Background(), upgrade); err != nil {
		t.Fatalf("CreateUpgrade: %v", err)
	}

	// Reconcile
	err := uc.Reconcile(context.Background(), "test-upgrade")
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	// Verify transition to Waiting
	got, _ := ms.GetUpgrade(context.Background(), "", "test-upgrade")
	if got.Status.Phase != types.UpgradePhaseWaiting {
		t.Errorf("Phase = %q, want %q", got.Status.Phase, types.UpgradePhaseWaiting)
	}
}

func TestUpgradeController_Reconcile_WaitingToSwitching(t *testing.T) {
	ms := store.NewMemoryStore()
	uc := NewUpgradeController(ms, nil)

	// Create an upgrade in Waiting phase (without runtime, auto-advances)
	upgrade := &types.Upgrade{
		Metadata: types.ResourceMeta{Name: "test-upgrade"},
		Spec: types.UpgradeSpec{
			DevnetRef:    "mydevnet",
			UpgradeName:  "v2.0",
			TargetHeight: 1000,
		},
		Status: types.UpgradeStatus{
			Phase:         types.UpgradePhaseWaiting,
			CurrentHeight: 999,
		},
	}
	if err := ms.CreateUpgrade(context.Background(), upgrade); err != nil {
		t.Fatalf("CreateUpgrade: %v", err)
	}

	// Reconcile
	err := uc.Reconcile(context.Background(), "test-upgrade")
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	// Verify transition to Switching (without runtime, auto-advances)
	got, _ := ms.GetUpgrade(context.Background(), "", "test-upgrade")
	if got.Status.Phase != types.UpgradePhaseSwitching {
		t.Errorf("Phase = %q, want %q", got.Status.Phase, types.UpgradePhaseSwitching)
	}
}

func TestUpgradeController_Reconcile_SwitchingToVerifying(t *testing.T) {
	ms := store.NewMemoryStore()
	uc := NewUpgradeController(ms, nil)

	// Create an upgrade in Switching phase
	upgrade := &types.Upgrade{
		Metadata: types.ResourceMeta{Name: "test-upgrade"},
		Spec: types.UpgradeSpec{
			DevnetRef:   "mydevnet",
			UpgradeName: "v2.0",
			NewBinary: types.BinarySource{
				Type:    "cache",
				Version: "v2.0.0",
			},
		},
		Status: types.UpgradeStatus{
			Phase: types.UpgradePhaseSwitching,
		},
	}
	if err := ms.CreateUpgrade(context.Background(), upgrade); err != nil {
		t.Fatalf("CreateUpgrade: %v", err)
	}

	// Reconcile
	err := uc.Reconcile(context.Background(), "test-upgrade")
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	// Verify transition to Verifying
	got, _ := ms.GetUpgrade(context.Background(), "", "test-upgrade")
	if got.Status.Phase != types.UpgradePhaseVerifying {
		t.Errorf("Phase = %q, want %q", got.Status.Phase, types.UpgradePhaseVerifying)
	}
}

func TestUpgradeController_Reconcile_VerifyingToCompleted(t *testing.T) {
	ms := store.NewMemoryStore()
	uc := NewUpgradeController(ms, nil)

	// Create an upgrade in Verifying phase
	upgrade := &types.Upgrade{
		Metadata: types.ResourceMeta{Name: "test-upgrade"},
		Spec: types.UpgradeSpec{
			DevnetRef:   "mydevnet",
			UpgradeName: "v2.0",
			NewBinary: types.BinarySource{
				Type:    "cache",
				Version: "v2.0.0",
			},
		},
		Status: types.UpgradeStatus{
			Phase: types.UpgradePhaseVerifying,
		},
	}
	if err := ms.CreateUpgrade(context.Background(), upgrade); err != nil {
		t.Fatalf("CreateUpgrade: %v", err)
	}

	// Reconcile
	err := uc.Reconcile(context.Background(), "test-upgrade")
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	// Verify transition to Completed
	got, _ := ms.GetUpgrade(context.Background(), "", "test-upgrade")
	if got.Status.Phase != types.UpgradePhaseCompleted {
		t.Errorf("Phase = %q, want %q", got.Status.Phase, types.UpgradePhaseCompleted)
	}
	if got.Status.Message != "Upgrade completed successfully" {
		t.Errorf("Message = %q, want %q", got.Status.Message, "Upgrade completed successfully")
	}
}

func TestUpgradeController_Reconcile_NotFound(t *testing.T) {
	ms := store.NewMemoryStore()
	uc := NewUpgradeController(ms, nil)

	// Reconcile a non-existent upgrade should not error
	err := uc.Reconcile(context.Background(), "nonexistent")
	if err != nil {
		t.Errorf("Expected no error for non-existent upgrade, got: %v", err)
	}
}

func TestUpgradeController_Reconcile_CompletedIsTerminal(t *testing.T) {
	ms := store.NewMemoryStore()
	uc := NewUpgradeController(ms, nil)

	// Create a completed upgrade
	upgrade := &types.Upgrade{
		Metadata: types.ResourceMeta{Name: "test-upgrade"},
		Spec: types.UpgradeSpec{
			DevnetRef:   "mydevnet",
			UpgradeName: "v2.0",
		},
		Status: types.UpgradeStatus{
			Phase:   types.UpgradePhaseCompleted,
			Message: "Done",
		},
	}
	if err := ms.CreateUpgrade(context.Background(), upgrade); err != nil {
		t.Fatalf("CreateUpgrade: %v", err)
	}

	// Reconcile should do nothing
	err := uc.Reconcile(context.Background(), "test-upgrade")
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	// Verify still Completed
	got, _ := ms.GetUpgrade(context.Background(), "", "test-upgrade")
	if got.Status.Phase != types.UpgradePhaseCompleted {
		t.Errorf("Phase = %q, want %q", got.Status.Phase, types.UpgradePhaseCompleted)
	}
}

func TestUpgradeController_Reconcile_FailedIsTerminal(t *testing.T) {
	ms := store.NewMemoryStore()
	uc := NewUpgradeController(ms, nil)

	// Create a failed upgrade
	upgrade := &types.Upgrade{
		Metadata: types.ResourceMeta{Name: "test-upgrade"},
		Spec: types.UpgradeSpec{
			DevnetRef:   "mydevnet",
			UpgradeName: "v2.0",
		},
		Status: types.UpgradeStatus{
			Phase: types.UpgradePhaseFailed,
			Error: "Something went wrong",
		},
	}
	if err := ms.CreateUpgrade(context.Background(), upgrade); err != nil {
		t.Fatalf("CreateUpgrade: %v", err)
	}

	// Reconcile should do nothing
	err := uc.Reconcile(context.Background(), "test-upgrade")
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	// Verify still Failed
	got, _ := ms.GetUpgrade(context.Background(), "", "test-upgrade")
	if got.Status.Phase != types.UpgradePhaseFailed {
		t.Errorf("Phase = %q, want %q", got.Status.Phase, types.UpgradePhaseFailed)
	}
}

func TestUpgradeController_Reconcile_FullLifecycle(t *testing.T) {
	ms := store.NewMemoryStore()
	uc := NewUpgradeController(ms, nil)

	// Create a new upgrade (will go through full lifecycle without runtime)
	upgrade := &types.Upgrade{
		Metadata: types.ResourceMeta{Name: "test-upgrade"},
		Spec: types.UpgradeSpec{
			DevnetRef:    "mydevnet",
			UpgradeName:  "v2.0",
			TargetHeight: 1000,
			NewBinary: types.BinarySource{
				Type:    "cache",
				Version: "v2.0.0",
			},
		},
		Status: types.UpgradeStatus{
			Phase: types.UpgradePhasePending,
		},
	}
	if err := ms.CreateUpgrade(context.Background(), upgrade); err != nil {
		t.Fatalf("CreateUpgrade: %v", err)
	}

	// Expected phase progression
	expectedPhases := []string{
		types.UpgradePhaseProposing,
		types.UpgradePhaseVoting,
		types.UpgradePhaseWaiting,
		types.UpgradePhaseSwitching,
		types.UpgradePhaseVerifying,
		types.UpgradePhaseCompleted,
	}

	for i, expectedPhase := range expectedPhases {
		if err := uc.Reconcile(context.Background(), "test-upgrade"); err != nil {
			t.Fatalf("Reconcile step %d: %v", i+1, err)
		}

		got, _ := ms.GetUpgrade(context.Background(), "", "test-upgrade")
		if got.Status.Phase != expectedPhase {
			t.Fatalf("After step %d: Phase = %q, want %q", i+1, got.Status.Phase, expectedPhase)
		}
	}

	// One more reconcile should keep it at Completed
	if err := uc.Reconcile(context.Background(), "test-upgrade"); err != nil {
		t.Fatalf("Final Reconcile: %v", err)
	}

	got, _ := ms.GetUpgrade(context.Background(), "", "test-upgrade")
	if got.Status.Phase != types.UpgradePhaseCompleted {
		t.Errorf("Final Phase = %q, want %q", got.Status.Phase, types.UpgradePhaseCompleted)
	}
}

func TestUpgradeController_Reconcile_EmptyPhaseToPending(t *testing.T) {
	ms := store.NewMemoryStore()
	uc := NewUpgradeController(ms, nil)

	// Create an upgrade with empty phase (treated as Pending)
	upgrade := &types.Upgrade{
		Metadata: types.ResourceMeta{Name: "test-upgrade"},
		Spec: types.UpgradeSpec{
			DevnetRef:    "mydevnet",
			UpgradeName:  "v2.0",
			TargetHeight: 1000,
		},
		Status: types.UpgradeStatus{
			Phase: "", // Empty should be treated as Pending
		},
	}
	if err := ms.CreateUpgrade(context.Background(), upgrade); err != nil {
		t.Fatalf("CreateUpgrade: %v", err)
	}

	// Reconcile
	err := uc.Reconcile(context.Background(), "test-upgrade")
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	// Verify transition to Proposing (from Pending handling)
	got, _ := ms.GetUpgrade(context.Background(), "", "test-upgrade")
	if got.Status.Phase != types.UpgradePhaseProposing {
		t.Errorf("Phase = %q, want %q", got.Status.Phase, types.UpgradePhaseProposing)
	}
}

func TestUpgradeController_Reconcile_WithExport(t *testing.T) {
	ms := store.NewMemoryStore()
	uc := NewUpgradeController(ms, nil)

	// Create an upgrade with export enabled
	upgrade := &types.Upgrade{
		Metadata: types.ResourceMeta{Name: "export-upgrade"},
		Spec: types.UpgradeSpec{
			DevnetRef:    "mydevnet",
			UpgradeName:  "v2.0",
			TargetHeight: 1000,
			WithExport:   true,
		},
		Status: types.UpgradeStatus{
			Phase: types.UpgradePhasePending,
		},
	}
	if err := ms.CreateUpgrade(context.Background(), upgrade); err != nil {
		t.Fatalf("CreateUpgrade: %v", err)
	}

	// Reconcile - without runtime, export is skipped but no error
	err := uc.Reconcile(context.Background(), "export-upgrade")
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	got, _ := ms.GetUpgrade(context.Background(), "", "export-upgrade")
	if got.Status.Phase != types.UpgradePhaseProposing {
		t.Errorf("Phase = %q, want %q", got.Status.Phase, types.UpgradePhaseProposing)
	}
}
