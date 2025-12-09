package devnet

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// Lock represents a file-based mutex for preventing concurrent operations.
type Lock struct {
	LockDir    string    `json:"-"`
	PID        int       `json:"pid"`
	AcquiredAt time.Time `json:"acquired_at"`
	Hostname   string    `json:"hostname"`
	Purpose    string    `json:"purpose"`
}

// AcquireLock attempts to acquire a lock in the specified directory.
// It will wait up to timeout duration for the lock to become available.
func AcquireLock(dir string, purpose string, timeout time.Duration) (*Lock, error) {
	lockDir := filepath.Join(dir, ".lock")
	lockFile := filepath.Join(lockDir, "lock.json")

	deadline := time.Now().Add(timeout)

	for {
		// Try to create lock directory
		if err := os.MkdirAll(lockDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create lock directory: %w", err)
		}

		// Check for existing lock
		existingLock, err := readLockFile(lockFile)
		if err == nil && existingLock != nil {
			// Lock exists, check if stale
			if existingLock.IsStale() {
				// Remove stale lock
				if err := os.Remove(lockFile); err != nil {
					return nil, fmt.Errorf("failed to remove stale lock: %w", err)
				}
			} else {
				// Lock is held by another process
				if time.Now().After(deadline) {
					return nil, fmt.Errorf("timeout waiting for lock (held by PID %d for %s)",
						existingLock.PID, existingLock.Purpose)
				}
				time.Sleep(100 * time.Millisecond)
				continue
			}
		}

		// Create new lock
		hostname, _ := os.Hostname()
		lock := &Lock{
			LockDir:    lockDir,
			PID:        os.Getpid(),
			AcquiredAt: time.Now(),
			Hostname:   hostname,
			Purpose:    purpose,
		}

		// Write lock file atomically
		if err := writeLockFile(lockFile, lock); err != nil {
			return nil, fmt.Errorf("failed to write lock file: %w", err)
		}

		// Verify we got the lock (double-check)
		verifyLock, err := readLockFile(lockFile)
		if err != nil || verifyLock.PID != lock.PID {
			// Someone else got the lock
			if time.Now().After(deadline) {
				return nil, fmt.Errorf("timeout waiting for lock")
			}
			time.Sleep(100 * time.Millisecond)
			continue
		}

		return lock, nil
	}
}

// Release releases the lock.
func (l *Lock) Release() error {
	lockFile := filepath.Join(l.LockDir, "lock.json")

	// Verify we still own the lock
	existingLock, err := readLockFile(lockFile)
	if err != nil {
		return nil // Lock file doesn't exist, already released
	}

	if existingLock.PID != l.PID {
		return fmt.Errorf("lock is owned by different process (PID %d)", existingLock.PID)
	}

	// Remove lock file
	if err := os.Remove(lockFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove lock file: %w", err)
	}

	return nil
}

// IsStale checks if the lock is held by a dead process.
func (l *Lock) IsStale() bool {
	// Check if process exists using kill -0
	process, err := os.FindProcess(l.PID)
	if err != nil {
		return true // Process doesn't exist
	}

	// On Unix, FindProcess always succeeds, so we need to send signal 0
	err = process.Signal(syscall.Signal(0))
	if err != nil {
		return true // Process doesn't exist or we can't signal it
	}

	return false
}

// readLockFile reads and parses a lock file.
func readLockFile(path string) (*Lock, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var lock Lock
	if err := json.Unmarshal(data, &lock); err != nil {
		return nil, err
	}

	return &lock, nil
}

// writeLockFile writes a lock file atomically.
func writeLockFile(path string, lock *Lock) error {
	data, err := json.MarshalIndent(lock, "", "  ")
	if err != nil {
		return err
	}

	// Write to temp file first
	tmpFile := path + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return err
	}

	// Rename atomically
	return os.Rename(tmpFile, path)
}

// TryAcquireLock attempts to acquire a lock without waiting.
// Returns nil if lock could not be acquired immediately.
func TryAcquireLock(dir string, purpose string) (*Lock, error) {
	return AcquireLock(dir, purpose, 0)
}

// LockInfo returns information about an existing lock without acquiring it.
func LockInfo(dir string) (*Lock, error) {
	lockFile := filepath.Join(dir, ".lock", "lock.json")
	return readLockFile(lockFile)
}
