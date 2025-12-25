package persistence

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/b-harvest/devnet-builder/internal/domain/ports"
)

// PortRegistryFile handles persistence of port allocations to disk
type PortRegistryFile struct {
	registryPath string
}

// PortRegistryData represents the structure of the registry file
type PortRegistryData struct {
	Version     string                  `json:"version"`
	Allocations []*ports.PortAllocation `json:"allocations"`
}

// NewPortRegistryFile creates a new port registry file handler
func NewPortRegistryFile(homeDir string) *PortRegistryFile {
	registryPath := filepath.Join(homeDir, "port-registry.json")
	return &PortRegistryFile{
		registryPath: registryPath,
	}
}

// Load reads all port allocations from the registry file
func (r *PortRegistryFile) Load(ctx context.Context) ([]*ports.PortAllocation, error) {
	// Check if file exists
	if _, err := os.Stat(r.registryPath); os.IsNotExist(err) {
		// No registry file yet, return empty list
		return []*ports.PortAllocation{}, nil
	}

	// Read file
	data, err := os.ReadFile(r.registryPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read port registry: %w", err)
	}

	// Handle empty file (file was created but never written to)
	if len(data) == 0 {
		return []*ports.PortAllocation{}, nil
	}

	// Parse JSON
	var registryData PortRegistryData
	if err := json.Unmarshal(data, &registryData); err != nil {
		return nil, fmt.Errorf("failed to parse port registry: %w", err)
	}

	return registryData.Allocations, nil
}

// Save writes all port allocations to the registry file
func (r *PortRegistryFile) Save(ctx context.Context, allocations []*ports.PortAllocation) error {
	// Create registry data
	registryData := PortRegistryData{
		Version:     "1.0",
		Allocations: allocations,
	}

	// Marshal to JSON with indentation
	data, err := json.MarshalIndent(registryData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal port registry: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(r.registryPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create registry directory: %w", err)
	}

	// Write to file atomically using a unique temp file
	// Create temp file in the same directory to ensure atomic rename
	tempFile, err := os.CreateTemp(dir, ".port-registry.*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tempPath := tempFile.Name()

	// Write data and close
	if _, err := tempFile.Write(data); err != nil {
		tempFile.Close()
		os.Remove(tempPath)
		return fmt.Errorf("failed to write port registry: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tempPath, r.registryPath); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("failed to rename port registry: %w", err)
	}

	return nil
}

// WithLock executes a function with an exclusive file lock
// This prevents race conditions when multiple processes access the registry
//
// Uses a separate .lock file to avoid issues with atomic rename operations
// replacing the registry file while holding a lock on the old file descriptor.
func (r *PortRegistryFile) WithLock(ctx context.Context, fn func() error) error {
	// Ensure registry directory exists
	dir := filepath.Dir(r.registryPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create registry directory: %w", err)
	}

	// Use a separate lock file to avoid race conditions with atomic rename
	lockPath := r.registryPath + ".lock"
	lockFile, err := os.OpenFile(lockPath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("failed to open lock file: %w", err)
	}
	defer lockFile.Close()

	// Acquire exclusive lock (blocks until available)
	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("failed to acquire registry lock: %w", err)
	}
	defer func() {
		_ = syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)
	}()

	// Execute function with lock held
	return fn()
}
