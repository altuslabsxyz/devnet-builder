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

	// Write to file atomically (write to temp file, then rename)
	tempPath := r.registryPath + ".tmp"
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write port registry: %w", err)
	}

	if err := os.Rename(tempPath, r.registryPath); err != nil {
		return fmt.Errorf("failed to rename port registry: %w", err)
	}

	return nil
}

// WithLock executes a function with an exclusive file lock
// This prevents race conditions when multiple processes access the registry
func (r *PortRegistryFile) WithLock(ctx context.Context, fn func() error) error {
	// Ensure registry directory exists
	dir := filepath.Dir(r.registryPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create registry directory: %w", err)
	}

	// Open or create registry file
	file, err := os.OpenFile(r.registryPath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("failed to open registry file: %w", err)
	}
	defer file.Close()

	// Acquire exclusive lock (blocks until available)
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("failed to acquire registry lock: %w", err)
	}
	defer func() {
		_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
	}()

	// Execute function with lock held
	return fn()
}
