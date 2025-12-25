package export

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewExportExecutor(t *testing.T) {
	executor := NewExportExecutor()

	if executor == nil {
		t.Fatal("expected non-nil executor")
	}

	expectedTimeout := 5 * time.Minute
	if executor.timeout != expectedTimeout {
		t.Errorf("expected timeout %v, got %v", expectedTimeout, executor.timeout)
	}
}

func TestExportExecutor_WithTimeout(t *testing.T) {
	executor := NewExportExecutor()
	customTimeout := 10 * time.Minute

	executor = executor.WithTimeout(customTimeout)

	if executor.timeout != customTimeout {
		t.Errorf("expected timeout %v, got %v", customTimeout, executor.timeout)
	}
}

func TestExportAtHeight_ValidateInputs(t *testing.T) {
	ctx := context.Background()
	executor := NewExportExecutor()

	tests := []struct {
		name       string
		binaryPath string
		homeDir    string
		height     int64
		outputPath string
		expectErr  bool
	}{
		{
			name:       "empty binary path",
			binaryPath: "",
			homeDir:    "/home/user/.stable",
			height:     1000,
			outputPath: "/tmp/genesis.json",
			expectErr:  true,
		},
		{
			name:       "empty home directory",
			binaryPath: "/usr/bin/stabled",
			homeDir:    "",
			height:     1000,
			outputPath: "/tmp/genesis.json",
			expectErr:  true,
		},
		{
			name:       "zero height",
			binaryPath: "/usr/bin/stabled",
			homeDir:    "/home/user/.stable",
			height:     0,
			outputPath: "/tmp/genesis.json",
			expectErr:  true,
		},
		{
			name:       "negative height",
			binaryPath: "/usr/bin/stabled",
			homeDir:    "/home/user/.stable",
			height:     -1,
			outputPath: "/tmp/genesis.json",
			expectErr:  true,
		},
		{
			name:       "empty output path",
			binaryPath: "/usr/bin/stabled",
			homeDir:    "/home/user/.stable",
			height:     1000,
			outputPath: "",
			expectErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := executor.ExportAtHeight(
				ctx,
				tt.binaryPath,
				tt.homeDir,
				tt.height,
				tt.outputPath,
			)

			if tt.expectErr && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("expected no error but got: %v", err)
			}
		})
	}
}

func TestExtractGenesisJSON_Valid(t *testing.T) {
	tests := []struct {
		name   string
		output []byte
		valid  bool
	}{
		{
			name:   "valid JSON",
			output: []byte(`{"chain_id":"stable-1","app_state":{}}`),
			valid:  true,
		},
		{
			name: "JSON with prefix",
			output: []byte(`INFO: Starting export...
{"chain_id":"stable-1","app_state":{}}`),
			valid: true,
		},
		{
			name:   "no JSON",
			output: []byte(`INFO: Starting export...`),
			valid:  false,
		},
		{
			name:   "invalid JSON",
			output: []byte(`{invalid json}`),
			valid:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := extractGenesisJSON(tt.output)

			if tt.valid {
				if err != nil {
					t.Errorf("expected no error but got: %v", err)
				}
				if result == nil {
					t.Error("expected non-nil result")
				}
			} else {
				if err == nil {
					t.Error("expected error but got none")
				}
			}
		})
	}
}

func TestValidateGenesisStructure(t *testing.T) {
	tests := []struct {
		name      string
		genesis   []byte
		expectErr bool
	}{
		{
			name: "valid genesis",
			genesis: []byte(`{
				"chain_id": "stable-1",
				"app_state": {
					"auth": {},
					"bank": {},
					"staking": {}
				}
			}`),
			expectErr: false,
		},
		{
			name: "missing chain_id",
			genesis: []byte(`{
				"app_state": {
					"auth": {},
					"bank": {},
					"staking": {}
				}
			}`),
			expectErr: true,
		},
		{
			name: "missing app_state",
			genesis: []byte(`{
				"chain_id": "stable-1"
			}`),
			expectErr: true,
		},
		{
			name: "missing auth module",
			genesis: []byte(`{
				"chain_id": "stable-1",
				"app_state": {
					"bank": {},
					"staking": {}
				}
			}`),
			expectErr: true,
		},
		{
			name: "missing bank module",
			genesis: []byte(`{
				"chain_id": "stable-1",
				"app_state": {
					"auth": {},
					"staking": {}
				}
			}`),
			expectErr: true,
		},
		{
			name: "missing staking module",
			genesis: []byte(`{
				"chain_id": "stable-1",
				"app_state": {
					"auth": {},
					"bank": {}
				}
			}`),
			expectErr: true,
		},
		{
			name:      "invalid JSON",
			genesis:   []byte(`{invalid`),
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateGenesisStructure(tt.genesis)

			if tt.expectErr && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("expected no error but got: %v", err)
			}
		})
	}
}

func TestGetBinaryVersion_EmptyPath(t *testing.T) {
	ctx := context.Background()
	executor := NewExportExecutor()

	_, err := executor.GetBinaryVersion(ctx, "")

	if err == nil {
		t.Error("expected error for empty binary path")
	}
}

func TestExportError(t *testing.T) {
	err := &ExportError{
		Operation: "export",
		Height:    1000,
		Message:   "test error",
	}

	expected := "export export error at height 1000: test error"
	if err.Error() != expected {
		t.Errorf("expected error message '%s', got '%s'", expected, err.Error())
	}
}

func TestExportAtHeight_CreateOutputDirectory(t *testing.T) {
	ctx := context.Background()
	executor := NewExportExecutor().WithTimeout(1 * time.Second)

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "nested", "path", "genesis.json")

	// This will fail when trying to execute the binary, but should create the directory
	_, err := executor.ExportAtHeight(
		ctx,
		"/nonexistent/binary",
		"/tmp/home",
		1000,
		outputPath,
	)

	// Should have error (binary doesn't exist), but directory should be created
	if err == nil {
		t.Error("expected error for nonexistent binary")
	}

	// Check that the directory was created
	outputDir := filepath.Dir(outputPath)
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		t.Errorf("expected output directory to be created at %s", outputDir)
	}
}
