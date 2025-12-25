package export

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/b-harvest/devnet-builder/internal/domain/export"
)

// ExportExecutor executes blockchain export commands and captures genesis output.
type ExportExecutor struct {
	timeout time.Duration
}

// NewExportExecutor creates a new ExportExecutor with default timeout.
func NewExportExecutor() *ExportExecutor {
	return &ExportExecutor{
		timeout: 5 * time.Minute, // Export can take a while
	}
}

// WithTimeout sets a custom timeout for export commands.
func (e *ExportExecutor) WithTimeout(timeout time.Duration) *ExportExecutor {
	e.timeout = timeout
	return e
}

// ExportAtHeight executes the export command and saves genesis to file.
// Returns the path to the saved genesis file.
func (e *ExportExecutor) ExportAtHeight(
	ctx context.Context,
	binaryPath string,
	homeDir string,
	height int64,
	outputPath string,
) (string, error) {
	if binaryPath == "" {
		return "", fmt.Errorf("binary path cannot be empty")
	}
	if homeDir == "" {
		return "", fmt.Errorf("home directory cannot be empty")
	}
	if height <= 0 {
		return "", fmt.Errorf("height must be greater than 0")
	}
	if outputPath == "" {
		return "", fmt.Errorf("output path cannot be empty")
	}

	// Create output directory if it doesn't exist
	outputDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	// Create command context with timeout
	cmdCtx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	// Build export command
	args := []string{
		"export",
		"--home", homeDir,
		"--height", fmt.Sprintf("%d", height),
		"--for-zero-height",
	}

	// Execute command and capture stdout
	cmd := exec.CommandContext(cmdCtx, binaryPath, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", &ExportError{
			Operation: "export",
			Height:    height,
			Message:   fmt.Sprintf("export command failed: %v\nStderr: %s", err, stderr.String()),
		}
	}

	// Extract genesis JSON from stdout
	genesis, err := extractGenesisJSON(stdout.Bytes())
	if err != nil {
		return "", &ExportError{
			Operation: "extract_json",
			Height:    height,
			Message:   fmt.Sprintf("failed to extract genesis JSON: %v", err),
		}
	}

	// Validate genesis structure
	if err := validateGenesisStructure(genesis); err != nil {
		return "", &ExportError{
			Operation: "validate",
			Height:    height,
			Message:   fmt.Sprintf("invalid genesis structure: %v", err),
		}
	}

	// Write genesis to file
	if err := os.WriteFile(outputPath, genesis, 0644); err != nil {
		return "", fmt.Errorf("failed to write genesis file: %w", err)
	}

	return outputPath, nil
}

// GetBinaryVersion queries the binary version.
func (e *ExportExecutor) GetBinaryVersion(ctx context.Context, binaryPath string) (string, error) {
	if binaryPath == "" {
		return "", fmt.Errorf("binary path cannot be empty")
	}

	cmdCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, binaryPath, "version")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get binary version: %w", err)
	}

	version := string(bytes.TrimSpace(output))
	if version == "" {
		return "unknown", nil
	}

	return version, nil
}

// extractGenesisJSON extracts valid JSON from command output.
// The export command may output warnings/logs before the JSON.
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
		return nil, fmt.Errorf("no JSON found in export output")
	}

	jsonData := output[jsonStart:]

	// Validate it's valid JSON
	var js json.RawMessage
	if err := json.Unmarshal(jsonData, &js); err != nil {
		return nil, fmt.Errorf("invalid JSON in export output: %w", err)
	}

	return jsonData, nil
}

// validateGenesisStructure performs basic validation of genesis structure.
func validateGenesisStructure(genesis []byte) error {
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

	// Check for required Cosmos SDK modules
	requiredModules := []string{"auth", "bank", "staking"}
	for _, module := range requiredModules {
		if _, ok := g.AppState[module]; !ok {
			return fmt.Errorf("missing required module: %s", module)
		}
	}

	return nil
}

// ExportError represents an error during export execution.
type ExportError struct {
	Operation string
	Height    int64
	Message   string
}

func (e *ExportError) Error() string {
	return fmt.Sprintf("export %s error at height %d: %s", e.Operation, e.Height, e.Message)
}

// Ensure ExportError implements error interface
var _ error = (*ExportError)(nil)
