// Package persistence provides file-based storage implementations.
package persistence

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/altuslabsxyz/devnet-builder/internal/application/ports"
)

// stateFilename is the name of the upgrade state file.
const stateFilename = ".upgrade-state.json"

// lockFilename is the name of the lock file.
const lockFilename = ".upgrade-state.lock"

// FileUpgradeStateManager implements UpgradeStateManager using the filesystem.
// It provides atomic writes, checksum validation, and file locking.
type FileUpgradeStateManager struct {
	homeDir  string
	lockFile *os.File
}

// NewFileUpgradeStateManager creates a new FileUpgradeStateManager.
func NewFileUpgradeStateManager(homeDir string) *FileUpgradeStateManager {
	return &FileUpgradeStateManager{
		homeDir: homeDir,
	}
}

// statePath returns the path to the state file.
func (m *FileUpgradeStateManager) statePath() string {
	return filepath.Join(m.homeDir, stateFilename)
}

// lockPath returns the path to the lock file.
func (m *FileUpgradeStateManager) lockPath() string {
	return filepath.Join(m.homeDir, lockFilename)
}

// LoadState loads the current upgrade state from disk.
// Returns nil, nil if no state exists.
// Returns error if state file is corrupted or unreadable.
func (m *FileUpgradeStateManager) LoadState(ctx context.Context) (*ports.UpgradeState, error) {
	path := m.statePath()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, &ReadError{Path: path, Message: err.Error()}
	}

	var state ports.UpgradeState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, &ports.StateCorruptionError{Reason: fmt.Sprintf("invalid JSON: %v", err)}
	}

	// Validate checksum
	if err := m.validateChecksum(&state, data); err != nil {
		return nil, err
	}

	return &state, nil
}

// SaveState persists the upgrade state to disk atomically.
// Uses atomic write (temp file + rename) to prevent corruption.
func (m *FileUpgradeStateManager) SaveState(ctx context.Context, state *ports.UpgradeState) error {
	if state == nil {
		return fmt.Errorf("state is nil")
	}

	// Calculate checksum before serialization
	state.Checksum = m.calculateChecksum(state)

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	path := m.statePath()
	tempPath := path + ".tmp"

	// Write to temp file
	if err := os.WriteFile(tempPath, data, 0600); err != nil {
		return &WriteError{Path: tempPath, Message: err.Error()}
	}

	// Sync to ensure data is flushed to disk
	f, err := os.Open(tempPath)
	if err == nil {
		_ = f.Sync()
		_ = f.Close()
	}

	// Atomic rename
	if err := os.Rename(tempPath, path); err != nil {
		// Clean up temp file on failure
		_ = os.Remove(tempPath)
		return &WriteError{Path: path, Message: fmt.Sprintf("atomic rename failed: %v", err)}
	}

	return nil
}

// DeleteState removes the upgrade state file.
// Used after successful completion or when clearing state.
func (m *FileUpgradeStateManager) DeleteState(ctx context.Context) error {
	path := m.statePath()
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return nil // Already deleted, no error
		}
		return fmt.Errorf("failed to delete state file: %w", err)
	}
	return nil
}

// StateExists checks if an upgrade state file exists.
func (m *FileUpgradeStateManager) StateExists(ctx context.Context) (bool, error) {
	path := m.statePath()
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check state file: %w", err)
	}
	return true, nil
}

// ValidateState checks state integrity (schema + checksum).
// Returns validation errors if state is corrupted.
func (m *FileUpgradeStateManager) ValidateState(state *ports.UpgradeState) error {
	if state == nil {
		return fmt.Errorf("state is nil")
	}

	// Validate schema version
	if state.Version < 1 {
		return &ports.StateCorruptionError{Reason: fmt.Sprintf("invalid schema version: %d", state.Version)}
	}

	// Validate required fields
	if state.UpgradeName == "" {
		return &ports.StateCorruptionError{Reason: "upgradeName is required"}
	}

	if state.Stage == "" {
		return &ports.StateCorruptionError{Reason: "stage is required"}
	}

	if state.Mode != "docker" && state.Mode != "local" {
		return &ports.StateCorruptionError{Reason: fmt.Sprintf("invalid mode: %s (must be 'docker' or 'local')", state.Mode)}
	}

	// Validate stage history
	if len(state.StageHistory) == 0 {
		return &ports.StateCorruptionError{Reason: "stageHistory must not be empty"}
	}

	// First entry must have empty "from"
	if state.StageHistory[0].From != "" {
		return &ports.StateCorruptionError{Reason: "first stageHistory entry must have empty 'from' field"}
	}

	// Validate timestamps
	if state.CreatedAt.After(state.UpdatedAt) {
		return &ports.StateCorruptionError{Reason: "createdAt cannot be after updatedAt"}
	}

	// Validate validator votes
	for i, vote := range state.ValidatorVotes {
		if vote.Address == "" {
			return &ports.StateCorruptionError{Reason: fmt.Sprintf("validatorVotes[%d]: address is required", i)}
		}
		if vote.Voted && vote.TxHash == "" {
			return &ports.StateCorruptionError{Reason: fmt.Sprintf("validatorVotes[%d]: txHash required when voted=true", i)}
		}
	}

	// Validate node switches
	for i, ns := range state.NodeSwitches {
		if ns.NodeName == "" {
			return &ports.StateCorruptionError{Reason: fmt.Sprintf("nodeSwitches[%d]: nodeName is required", i)}
		}
		if ns.Switched && (!ns.Stopped || !ns.Started) {
			return &ports.StateCorruptionError{Reason: fmt.Sprintf("nodeSwitches[%d]: stopped and started must be true when switched=true", i)}
		}
		if ns.Switched && ns.NewBinary == "" {
			return &ports.StateCorruptionError{Reason: fmt.Sprintf("nodeSwitches[%d]: newBinary required when switched=true", i)}
		}
	}

	return nil
}

// AcquireLock acquires an exclusive lock to prevent concurrent upgrades.
// Returns error if another upgrade is in progress.
func (m *FileUpgradeStateManager) AcquireLock(ctx context.Context) error {
	path := m.lockPath()

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create lock directory: %w", err)
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return fmt.Errorf("failed to open lock file: %w", err)
	}

	// Try to acquire exclusive lock (non-blocking)
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		// Check if there's an existing state to provide better error message
		state, loadErr := m.LoadState(ctx)
		if loadErr == nil && state != nil {
			return &ports.UpgradeInProgressError{
				UpgradeName: state.UpgradeName,
				Stage:       state.Stage,
			}
		}
		return fmt.Errorf("another upgrade is in progress (could not acquire lock)")
	}

	m.lockFile = f
	return nil
}

// ReleaseLock releases the exclusive lock.
func (m *FileUpgradeStateManager) ReleaseLock(ctx context.Context) error {
	if m.lockFile == nil {
		return nil
	}

	if err := syscall.Flock(int(m.lockFile.Fd()), syscall.LOCK_UN); err != nil {
		return fmt.Errorf("failed to release lock: %w", err)
	}

	if err := m.lockFile.Close(); err != nil {
		return fmt.Errorf("failed to close lock file: %w", err)
	}

	m.lockFile = nil

	// Remove lock file
	_ = os.Remove(m.lockPath())

	return nil
}

// calculateChecksum computes SHA256 of state (excluding checksum field).
func (m *FileUpgradeStateManager) calculateChecksum(state *ports.UpgradeState) string {
	// Create a copy without checksum for hashing
	stateCopy := *state
	stateCopy.Checksum = ""

	data, err := json.Marshal(stateCopy)
	if err != nil {
		return ""
	}

	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// validateChecksum verifies the stored checksum matches the computed value.
func (m *FileUpgradeStateManager) validateChecksum(state *ports.UpgradeState, rawData []byte) error {
	if state.Checksum == "" {
		// Allow missing checksum for backward compatibility
		return nil
	}

	expected := m.calculateChecksum(state)
	if state.Checksum != expected {
		return &ports.StateCorruptionError{
			Reason: fmt.Sprintf("checksum mismatch: stored=%s, computed=%s", state.Checksum, expected),
		}
	}

	return nil
}

// Ensure FileUpgradeStateManager implements UpgradeStateManager.
var _ ports.UpgradeStateManager = (*FileUpgradeStateManager)(nil)
