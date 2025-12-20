package devnet

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/b-harvest/devnet-builder/internal/output"
)

// ResetService handles devnet reset operations.
type ResetService struct {
	logger *output.Logger
}

// NewResetService creates a new ResetService.
func NewResetService(logger *output.Logger) *ResetService {
	if logger == nil {
		logger = output.DefaultLogger
	}
	return &ResetService{logger: logger}
}

// SoftReset clears chain data but preserves genesis and configuration.
func (s *ResetService) SoftReset(ctx context.Context, devnet *Devnet) error {
	return devnet.SoftReset(ctx)
}

// HardReset clears all data including genesis (requires re-provisioning).
func (s *ResetService) HardReset(ctx context.Context, devnet *Devnet) error {
	return devnet.HardReset(ctx)
}

// SoftReset clears chain data but preserves genesis and configuration.
func (d *Devnet) SoftReset(ctx context.Context) error {
	// Stop nodes first if running
	if d.Metadata.IsRunning() {
		if err := d.Stop(ctx, 30*time.Second); err != nil {
			return fmt.Errorf("failed to stop nodes: %w", err)
		}
	}

	// Clear data directories for each node
	for _, n := range d.Nodes {
		dataDir := n.DataPath()
		if err := os.RemoveAll(dataDir); err != nil {
			return fmt.Errorf("failed to clear data for %s: %w", n.Name, err)
		}
		if err := os.MkdirAll(dataDir, 0755); err != nil {
			return fmt.Errorf("failed to recreate data dir for %s: %w", n.Name, err)
		}

		// Recreate priv_validator_state.json with initial state
		// This file is required by CometBFT to track validator signing state
		privValStatePath := filepath.Join(dataDir, "priv_validator_state.json")
		initialState := `{
  "height": "0",
  "round": 0,
  "step": 0
}`
		if err := os.WriteFile(privValStatePath, []byte(initialState), 0644); err != nil {
			return fmt.Errorf("failed to create priv_validator_state.json for %s: %w", n.Name, err)
		}
	}

	d.Metadata.Status = StatusCreated
	d.Metadata.StartedAt = nil
	d.Metadata.StoppedAt = nil
	return d.Metadata.Save()
}

// HardReset clears all data including genesis (requires re-provisioning).
func (d *Devnet) HardReset(ctx context.Context) error {
	// Stop nodes first if running
	if d.Metadata.IsRunning() {
		if err := d.Stop(ctx, 30*time.Second); err != nil {
			return fmt.Errorf("failed to stop nodes: %w", err)
		}
	}

	// Remove entire devnet directory
	devnetDir := filepath.Join(d.Metadata.HomeDir, "devnet")
	if err := os.RemoveAll(devnetDir); err != nil {
		return fmt.Errorf("failed to remove devnet directory: %w", err)
	}

	return nil
}
