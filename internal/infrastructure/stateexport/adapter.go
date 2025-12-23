// Package stateexport provides state export implementations for snapshot-based genesis.
package stateexport

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/b-harvest/devnet-builder/internal/application/ports"
	"github.com/b-harvest/devnet-builder/internal/output"
	pkgNetwork "github.com/b-harvest/devnet-builder/pkg/network"
)

// Adapter implements ports.StateExportService.
// It orchestrates the full export flow: prepare, export, validate.
type Adapter struct {
	homeDir   string
	logger    *output.Logger
	exporter  pkgNetwork.StateExporter // Optional: network-specific exporter from plugin
	binaryCmd func(homeDir string) []string // Default export command builder
}

// NewAdapter creates a new StateExportAdapter.
func NewAdapter(homeDir string, logger *output.Logger) *Adapter {
	if logger == nil {
		logger = output.DefaultLogger
	}
	return &Adapter{
		homeDir: homeDir,
		logger:  logger,
		binaryCmd: func(homeDir string) []string {
			return []string{"export", "--home", homeDir}
		},
	}
}

// WithStateExporter sets a network-specific state exporter from the plugin.
func (a *Adapter) WithStateExporter(exporter pkgNetwork.StateExporter) *Adapter {
	a.exporter = exporter
	return a
}

// WithDefaultCommand sets the default export command builder.
func (a *Adapter) WithDefaultCommand(cmdBuilder func(homeDir string) []string) *Adapter {
	a.binaryCmd = cmdBuilder
	return a
}

// ExportFromSnapshot exports genesis from a snapshot's application state.
// This method supports caching: if the snapshot was loaded from cache AND a valid
// genesis cache exists for the same network, the cached genesis is returned immediately.
func (a *Adapter) ExportFromSnapshot(ctx context.Context, opts ports.StateExportOptions) ([]byte, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Check cache if snapshot was from cache
	if opts.SnapshotFromCache && opts.Network != "" {
		a.logger.Debug("Snapshot was cached, checking for cached genesis...")
		cache, err := GetValidGenesisCache(a.homeDir, opts.Network)
		if err != nil {
			a.logger.Debug("Genesis cache check failed: %v", err)
		}
		if cache != nil {
			// Verify the cached genesis is from the same snapshot
			if cache.SnapshotURL == opts.SnapshotURL || opts.SnapshotURL == "" {
				// Read cached genesis
				genesis, err := os.ReadFile(cache.FilePath)
				if err != nil {
					a.logger.Debug("Failed to read cached genesis: %v", err)
				} else {
					a.logger.Info("Using cached genesis export (expires in %s)", cache.TimeUntilExpiry().Round(time.Minute))
					return genesis, nil
				}
			} else {
				a.logger.Debug("Cached genesis is from different snapshot, will re-export")
			}
		}
	}

	// Step 1: Prepare for export
	a.logger.Debug("Preparing for export at %s", opts.HomeDir)
	if err := a.PrepareForExport(ctx, opts.HomeDir, opts.RpcGenesis); err != nil {
		return nil, fmt.Errorf("failed to prepare for export: %w", err)
	}

	// Step 2: Build export command
	var cmdArgs []string
	if a.exporter != nil && opts.ExportOpts != nil {
		// Use plugin's export command with options
		pkgOpts := convertToPkgExportOptions(opts.ExportOpts)
		cmdArgs = a.exporter.ExportCommandWithOptions(opts.HomeDir, pkgOpts)
	} else {
		// Use default export command
		cmdArgs = a.buildExportCommand(opts.HomeDir, opts.ExportOpts)
	}

	// Step 3: Run export command
	a.logger.Info("Exporting genesis from snapshot...")
	a.logger.Debug("Running: %s %v", opts.BinaryPath, cmdArgs)

	cmd := exec.CommandContext(ctx, opts.BinaryPath, cmdArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, &StateExportError{
			Operation: "export",
			Message:   fmt.Sprintf("export failed: %v\nOutput: %s", err, string(output)),
		}
	}

	// Step 4: Extract JSON from output (may have warnings/logs before JSON)
	genesis, err := extractGenesisJSON(output)
	if err != nil {
		return nil, &StateExportError{
			Operation: "extract_json",
			Message:   err.Error(),
		}
	}

	// Step 5: Validate exported genesis
	a.logger.Debug("Validating exported genesis...")
	if err := a.ValidateExportedGenesis(genesis); err != nil {
		return nil, &StateExportError{
			Operation: "validate",
			Message:   err.Error(),
		}
	}

	// Step 6: Save to cache if snapshot was cached
	if opts.SnapshotFromCache && opts.Network != "" && opts.SnapshotURL != "" {
		a.logger.Debug("Saving genesis export to cache...")
		if err := SaveGenesisToCacheWithSnapshot(a.homeDir, opts.Network, opts.SnapshotURL, genesis); err != nil {
			a.logger.Debug("Failed to save genesis to cache: %v", err)
			// Don't fail the operation if caching fails
		} else {
			a.logger.Debug("Genesis export cached successfully")
		}
	}

	a.logger.Success("Genesis exported successfully (%d bytes)", len(genesis))
	return genesis, nil
}

// PrepareForExport prepares the node home directory for export.
func (a *Adapter) PrepareForExport(ctx context.Context, homeDir string, rpcGenesis []byte) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Ensure config directory exists
	configDir := filepath.Join(homeDir, "config")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config dir: %w", err)
	}

	// Write genesis.json from RPC
	// The export command needs this to read chain parameters
	genesisPath := filepath.Join(configDir, "genesis.json")
	if err := os.WriteFile(genesisPath, rpcGenesis, 0644); err != nil {
		return fmt.Errorf("failed to write genesis: %w", err)
	}
	a.logger.Debug("Wrote RPC genesis to %s", genesisPath)

	// Ensure data directory exists
	dataDir := filepath.Join(homeDir, "data")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data dir: %w", err)
	}

	// Create priv_validator_state.json if it doesn't exist
	// This file tracks the last signed height/round/step
	// Export needs it even if we're not signing anything
	privValidatorStatePath := filepath.Join(dataDir, "priv_validator_state.json")
	if _, err := os.Stat(privValidatorStatePath); os.IsNotExist(err) {
		initialState := `{
  "height": "0",
  "round": 0,
  "step": 0
}`
		if err := os.WriteFile(privValidatorStatePath, []byte(initialState), 0644); err != nil {
			return fmt.Errorf("failed to write priv_validator_state: %w", err)
		}
		a.logger.Debug("Created priv_validator_state.json")
	}

	return nil
}

// ValidateExportedGenesis validates the exported genesis.
func (a *Adapter) ValidateExportedGenesis(genesis []byte) error {
	if len(genesis) == 0 {
		return fmt.Errorf("empty genesis")
	}

	// If plugin provides validator, use it
	if a.exporter != nil {
		return a.exporter.ValidateExportedGenesis(genesis)
	}

	// Default validation
	return a.defaultValidateGenesis(genesis)
}

// defaultValidateGenesis performs basic genesis validation.
func (a *Adapter) defaultValidateGenesis(genesis []byte) error {
	var g struct {
		ChainID  string         `json:"chain_id"`
		AppState map[string]any `json:"app_state"`
	}
	if err := json.Unmarshal(genesis, &g); err != nil {
		return fmt.Errorf("failed to parse genesis: %w", err)
	}

	if g.ChainID == "" {
		return fmt.Errorf("chain_id is empty")
	}

	if g.AppState == nil {
		return fmt.Errorf("missing app_state")
	}

	// Check required Cosmos SDK modules
	requiredModules := []string{
		"auth",
		"bank",
		"staking",
		"slashing",
		"gov",
	}

	for _, module := range requiredModules {
		if _, ok := g.AppState[module]; !ok {
			return fmt.Errorf("missing required module: %s", module)
		}
	}

	return nil
}

// DefaultExportOptions returns the default export options for devnet.
func (a *Adapter) DefaultExportOptions() *ports.ExportOptions {
	return &ports.ExportOptions{
		ForZeroHeight: true,
		JailWhitelist: nil,
		ModulesToSkip: nil,
		Height:        0,
		OutputPath:    "",
	}
}

// buildExportCommand builds the export command with options.
func (a *Adapter) buildExportCommand(homeDir string, opts *ports.ExportOptions) []string {
	args := []string{
		"export",
		"--home", homeDir,
	}

	if opts == nil {
		opts = a.DefaultExportOptions()
	}

	if opts.ForZeroHeight {
		args = append(args, "--for-zero-height")
	}

	if opts.Height > 0 {
		args = append(args, "--height", fmt.Sprintf("%d", opts.Height))
	}

	for _, addr := range opts.JailWhitelist {
		args = append(args, "--jail-allowed-addrs", addr)
	}

	return args
}

// extractGenesisJSON extracts valid JSON from command output.
func extractGenesisJSON(output []byte) ([]byte, error) {
	// Find the start of JSON (first '{')
	jsonStart := -1
	for i, b := range output {
		if b == '{' {
			jsonStart = i
			break
		}
	}

	if jsonStart == -1 {
		return nil, fmt.Errorf("no JSON found in export output: %s", string(output))
	}

	jsonData := output[jsonStart:]

	// Validate it's valid JSON
	var js json.RawMessage
	if err := json.Unmarshal(jsonData, &js); err != nil {
		return nil, fmt.Errorf("invalid JSON in export output: %w", err)
	}

	return jsonData, nil
}

// convertToPkgExportOptions converts ports.ExportOptions to pkg/network.ExportOptions.
func convertToPkgExportOptions(opts *ports.ExportOptions) pkgNetwork.ExportOptions {
	if opts == nil {
		return pkgNetwork.ExportOptions{
			ForZeroHeight: true,
		}
	}
	return pkgNetwork.ExportOptions{
		ForZeroHeight: opts.ForZeroHeight,
		JailWhitelist: opts.JailWhitelist,
		ModulesToSkip: opts.ModulesToSkip,
		Height:        opts.Height,
		OutputPath:    opts.OutputPath,
	}
}

// StateExportError represents an error during state export.
type StateExportError struct {
	Operation string
	Message   string
}

func (e *StateExportError) Error() string {
	return fmt.Sprintf("state export %s error: %s", e.Operation, e.Message)
}

// GetChainIDFromGenesis extracts the chain_id from genesis bytes.
func GetChainIDFromGenesis(genesis []byte) (string, error) {
	var g struct {
		ChainID string `json:"chain_id"`
	}
	if err := json.Unmarshal(genesis, &g); err != nil {
		return "", fmt.Errorf("failed to parse genesis: %w", err)
	}
	return g.ChainID, nil
}

// DetectSnapshotFormat detects the snapshot format from URL or file path.
func DetectSnapshotFormat(path string) pkgNetwork.SnapshotFormat {
	lower := strings.ToLower(path)
	if strings.HasSuffix(lower, ".tar.zst") || strings.HasSuffix(lower, ".zst") {
		return pkgNetwork.SnapshotFormatTarZst
	}
	if strings.HasSuffix(lower, ".tar.lz4") || strings.HasSuffix(lower, ".lz4") {
		return pkgNetwork.SnapshotFormatTarLz4
	}
	if strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz") {
		return pkgNetwork.SnapshotFormatTarGz
	}
	return pkgNetwork.SnapshotFormatTarLz4 // Default
}

// Ensure Adapter implements StateExportService.
var _ ports.StateExportService = (*Adapter)(nil)
