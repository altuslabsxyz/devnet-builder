// internal/daemon/store/bolt_upgrade_test.go
package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/altuslabsxyz/devnet-builder/internal/daemon/types"
)

func TestBoltStore_CreateUpgrade(t *testing.T) {
	dir := t.TempDir()
	s, err := NewBoltStore(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("NewBoltStore: %v", err)
	}
	defer s.Close()

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
	}

	err = s.CreateUpgrade(context.Background(), upgrade)
	if err != nil {
		t.Fatalf("CreateUpgrade: %v", err)
	}

	// Verify
	got, err := s.GetUpgrade(context.Background(), "test-upgrade")
	if err != nil {
		t.Fatalf("GetUpgrade: %v", err)
	}
	if got.Spec.UpgradeName != "v2.0" {
		t.Errorf("UpgradeName = %q, want %q", got.Spec.UpgradeName, "v2.0")
	}
	if got.Metadata.Generation != 1 {
		t.Errorf("Generation = %d, want 1", got.Metadata.Generation)
	}
}

func TestBoltStore_CreateUpgrade_AlreadyExists(t *testing.T) {
	dir := t.TempDir()
	s, err := NewBoltStore(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("NewBoltStore: %v", err)
	}
	defer s.Close()

	upgrade := &types.Upgrade{
		Metadata: types.ResourceMeta{Name: "dup-upgrade"},
		Spec: types.UpgradeSpec{
			DevnetRef:   "mydevnet",
			UpgradeName: "v2.0",
		},
	}

	if err := s.CreateUpgrade(context.Background(), upgrade); err != nil {
		t.Fatalf("first CreateUpgrade: %v", err)
	}

	// Second create should fail
	err = s.CreateUpgrade(context.Background(), upgrade)
	if !IsAlreadyExists(err) {
		t.Errorf("expected AlreadyExists error, got: %v", err)
	}
}

func TestBoltStore_GetUpgrade_NotFound(t *testing.T) {
	dir := t.TempDir()
	s, err := NewBoltStore(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("NewBoltStore: %v", err)
	}
	defer s.Close()

	_, err = s.GetUpgrade(context.Background(), "nonexistent")
	if !IsNotFound(err) {
		t.Errorf("expected NotFound error, got: %v", err)
	}
}

func TestBoltStore_UpdateUpgrade(t *testing.T) {
	dir := t.TempDir()
	s, err := NewBoltStore(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("NewBoltStore: %v", err)
	}
	defer s.Close()

	upgrade := &types.Upgrade{
		Metadata: types.ResourceMeta{Name: "update-upgrade"},
		Spec: types.UpgradeSpec{
			DevnetRef:   "mydevnet",
			UpgradeName: "v2.0",
		},
		Status: types.UpgradeStatus{
			Phase: types.UpgradePhasePending,
		},
	}

	if err := s.CreateUpgrade(context.Background(), upgrade); err != nil {
		t.Fatalf("CreateUpgrade: %v", err)
	}

	// Get and update
	got, err := s.GetUpgrade(context.Background(), "update-upgrade")
	if err != nil {
		t.Fatalf("GetUpgrade: %v", err)
	}

	got.Status.Phase = types.UpgradePhaseVoting
	got.Status.ProposalID = 123
	if err := s.UpdateUpgrade(context.Background(), got); err != nil {
		t.Fatalf("UpdateUpgrade: %v", err)
	}

	// Verify
	updated, err := s.GetUpgrade(context.Background(), "update-upgrade")
	if err != nil {
		t.Fatalf("GetUpgrade after update: %v", err)
	}
	if updated.Status.Phase != types.UpgradePhaseVoting {
		t.Errorf("Phase = %q, want %q", updated.Status.Phase, types.UpgradePhaseVoting)
	}
	if updated.Status.ProposalID != 123 {
		t.Errorf("ProposalID = %d, want 123", updated.Status.ProposalID)
	}
	if updated.Metadata.Generation != 2 {
		t.Errorf("Generation = %d, want 2", updated.Metadata.Generation)
	}
}

func TestBoltStore_UpdateUpgrade_ConflictDetection(t *testing.T) {
	dir := t.TempDir()
	s, err := NewBoltStore(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("NewBoltStore: %v", err)
	}
	defer s.Close()

	upgrade := &types.Upgrade{
		Metadata: types.ResourceMeta{Name: "conflict-upgrade"},
		Spec: types.UpgradeSpec{
			DevnetRef:   "mydevnet",
			UpgradeName: "v2.0",
		},
	}

	if err := s.CreateUpgrade(context.Background(), upgrade); err != nil {
		t.Fatalf("CreateUpgrade: %v", err)
	}

	// Get two copies
	copy1, _ := s.GetUpgrade(context.Background(), "conflict-upgrade")
	copy2, _ := s.GetUpgrade(context.Background(), "conflict-upgrade")

	// Update first copy
	copy1.Status.Phase = types.UpgradePhaseVoting
	if err := s.UpdateUpgrade(context.Background(), copy1); err != nil {
		t.Fatalf("first update: %v", err)
	}

	// Try to update second copy (should fail - stale generation)
	copy2.Status.Phase = types.UpgradePhaseCompleted
	err = s.UpdateUpgrade(context.Background(), copy2)
	if !IsConflict(err) {
		t.Errorf("expected Conflict error, got: %v", err)
	}
}

func TestBoltStore_ListUpgrades(t *testing.T) {
	dir := t.TempDir()
	s, err := NewBoltStore(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("NewBoltStore: %v", err)
	}
	defer s.Close()

	// Create upgrades for different devnets
	upgrades := []*types.Upgrade{
		{
			Metadata: types.ResourceMeta{Name: "upgrade-1"},
			Spec:     types.UpgradeSpec{DevnetRef: "devnet-a", UpgradeName: "v1"},
		},
		{
			Metadata: types.ResourceMeta{Name: "upgrade-2"},
			Spec:     types.UpgradeSpec{DevnetRef: "devnet-a", UpgradeName: "v2"},
		},
		{
			Metadata: types.ResourceMeta{Name: "upgrade-3"},
			Spec:     types.UpgradeSpec{DevnetRef: "devnet-b", UpgradeName: "v1"},
		},
	}

	for _, u := range upgrades {
		if err := s.CreateUpgrade(context.Background(), u); err != nil {
			t.Fatalf("CreateUpgrade: %v", err)
		}
	}

	// List all upgrades (empty devnetName)
	all, err := s.ListUpgrades(context.Background(), "")
	if err != nil {
		t.Fatalf("ListUpgrades (all): %v", err)
	}
	if len(all) != 3 {
		t.Errorf("ListUpgrades (all) = %d, want 3", len(all))
	}

	// List upgrades for devnet-a
	devnetA, err := s.ListUpgrades(context.Background(), "devnet-a")
	if err != nil {
		t.Fatalf("ListUpgrades (devnet-a): %v", err)
	}
	if len(devnetA) != 2 {
		t.Errorf("ListUpgrades (devnet-a) = %d, want 2", len(devnetA))
	}

	// List upgrades for devnet-b
	devnetB, err := s.ListUpgrades(context.Background(), "devnet-b")
	if err != nil {
		t.Fatalf("ListUpgrades (devnet-b): %v", err)
	}
	if len(devnetB) != 1 {
		t.Errorf("ListUpgrades (devnet-b) = %d, want 1", len(devnetB))
	}
}

func TestBoltStore_DeleteUpgrade(t *testing.T) {
	dir := t.TempDir()
	s, err := NewBoltStore(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("NewBoltStore: %v", err)
	}
	defer s.Close()

	upgrade := &types.Upgrade{
		Metadata: types.ResourceMeta{Name: "delete-upgrade"},
		Spec: types.UpgradeSpec{
			DevnetRef:   "mydevnet",
			UpgradeName: "v2.0",
		},
	}

	if err := s.CreateUpgrade(context.Background(), upgrade); err != nil {
		t.Fatalf("CreateUpgrade: %v", err)
	}

	// Delete
	if err := s.DeleteUpgrade(context.Background(), "delete-upgrade"); err != nil {
		t.Fatalf("DeleteUpgrade: %v", err)
	}

	// Verify deleted
	_, err = s.GetUpgrade(context.Background(), "delete-upgrade")
	if !IsNotFound(err) {
		t.Errorf("expected NotFound after delete, got: %v", err)
	}
}

func TestBoltStore_DeleteUpgrade_NotFound(t *testing.T) {
	dir := t.TempDir()
	s, err := NewBoltStore(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("NewBoltStore: %v", err)
	}
	defer s.Close()

	err = s.DeleteUpgrade(context.Background(), "nonexistent")
	if !IsNotFound(err) {
		t.Errorf("expected NotFound error, got: %v", err)
	}
}

func TestBoltStore_DeleteUpgradesByDevnet(t *testing.T) {
	dir := t.TempDir()
	s, err := NewBoltStore(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("NewBoltStore: %v", err)
	}
	defer s.Close()

	// Create upgrades for different devnets
	upgrades := []*types.Upgrade{
		{
			Metadata: types.ResourceMeta{Name: "cascade-1"},
			Spec:     types.UpgradeSpec{DevnetRef: "cascade-devnet", UpgradeName: "v1"},
		},
		{
			Metadata: types.ResourceMeta{Name: "cascade-2"},
			Spec:     types.UpgradeSpec{DevnetRef: "cascade-devnet", UpgradeName: "v2"},
		},
		{
			Metadata: types.ResourceMeta{Name: "keep-me"},
			Spec:     types.UpgradeSpec{DevnetRef: "other-devnet", UpgradeName: "v1"},
		},
	}

	for _, u := range upgrades {
		if err := s.CreateUpgrade(context.Background(), u); err != nil {
			t.Fatalf("CreateUpgrade: %v", err)
		}
	}

	// Delete all upgrades for cascade-devnet
	if err := s.DeleteUpgradesByDevnet(context.Background(), "cascade-devnet"); err != nil {
		t.Fatalf("DeleteUpgradesByDevnet: %v", err)
	}

	// Verify cascade-devnet upgrades are deleted
	list, err := s.ListUpgrades(context.Background(), "cascade-devnet")
	if err != nil {
		t.Fatalf("ListUpgrades: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("expected 0 upgrades for cascade-devnet, got %d", len(list))
	}

	// Verify other-devnet upgrade is still there
	other, err := s.GetUpgrade(context.Background(), "keep-me")
	if err != nil {
		t.Fatalf("GetUpgrade (keep-me): %v", err)
	}
	if other.Spec.DevnetRef != "other-devnet" {
		t.Errorf("wrong devnet ref: %q", other.Spec.DevnetRef)
	}
}
