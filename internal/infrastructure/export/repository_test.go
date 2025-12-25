package export

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	domainExport "github.com/b-harvest/devnet-builder/internal/domain/export"
)

func createTestExport(t *testing.T, dirPath string) *domainExport.Export {
	t.Helper()

	timestamp := time.Now()
	hash := "a1b2c3d4e5f67890123456789012345678901234567890123456789012345678"

	binaryInfo, err := domainExport.NewBinaryInfo(
		"/usr/local/bin/stabled",
		"",
		hash,
		"v1.0.0",
		domainExport.ExecutionModeLocal,
	)
	if err != nil {
		t.Fatalf("failed to create BinaryInfo: %v", err)
	}

	metadata, err := domainExport.NewExportMetadata(
		timestamp,
		1000000,
		"mainnet",
		0,
		"/usr/local/bin/stabled",
		hash,
		"v1.0.0",
		"",
		"stable-1",
		4,
		10,
		"local",
		"/home/user/.stable-devnet",
	)
	if err != nil {
		t.Fatalf("failed to create ExportMetadata: %v", err)
	}

	genesisPath := filepath.Join(dirPath, "genesis-1000000-a1b2c3d4.json")

	export, err := domainExport.NewExport(
		dirPath,
		timestamp,
		1000000,
		"mainnet",
		binaryInfo,
		metadata,
		genesisPath,
	)
	if err != nil {
		t.Fatalf("failed to create Export: %v", err)
	}

	return export
}

func TestNewRepository(t *testing.T) {
	repo := NewRepository("/tmp/exports")

	if repo == nil {
		t.Fatal("expected non-nil repository")
	}

	if repo.baseDir != "/tmp/exports" {
		t.Errorf("expected baseDir '/tmp/exports', got '%s'", repo.baseDir)
	}
}

func TestRepository_Save(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	repo := NewRepository(tmpDir)
	exportDir := filepath.Join(tmpDir, "mainnet-a1b2c3d4-1000000-20240115120000")
	export := createTestExport(t, exportDir)

	err := repo.Save(ctx, export)
	if err != nil {
		t.Fatalf("failed to save export: %v", err)
	}

	// Verify directory was created
	if _, err := os.Stat(exportDir); os.IsNotExist(err) {
		t.Error("expected export directory to be created")
	}

	// Verify metadata file was created
	metadataPath := filepath.Join(exportDir, "metadata.json")
	if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
		t.Error("expected metadata.json to be created")
	}

	// Verify metadata content
	content, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatalf("failed to read metadata file: %v", err)
	}

	if len(content) == 0 {
		t.Error("expected non-empty metadata file")
	}
}

func TestRepository_Save_InvalidType(t *testing.T) {
	ctx := context.Background()
	repo := NewRepository(t.TempDir())

	err := repo.Save(ctx, "invalid type")

	if err == nil {
		t.Fatal("expected error for invalid type")
	}
}

func TestRepository_Save_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	tmpDir := t.TempDir()
	repo := NewRepository(tmpDir)
	exportDir := filepath.Join(tmpDir, "mainnet-a1b2c3d4-1000000-20240115120000")
	export := createTestExport(t, exportDir)

	err := repo.Save(ctx, export)

	if err != context.Canceled {
		t.Errorf("expected context.Canceled error, got %v", err)
	}
}

func TestRepository_Load(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	repo := NewRepository(tmpDir)
	exportDir := filepath.Join(tmpDir, "mainnet-a1b2c3d4-1000000-20240115120000")
	originalExport := createTestExport(t, exportDir)

	// Save first
	if err := repo.Save(ctx, originalExport); err != nil {
		t.Fatalf("failed to save export: %v", err)
	}

	// Load back
	loadedInterface, err := repo.Load(ctx, exportDir)
	if err != nil {
		t.Fatalf("failed to load export: %v", err)
	}

	loadedExport, ok := loadedInterface.(*domainExport.Export)
	if !ok {
		t.Fatalf("expected *domainExport.Export, got %T", loadedInterface)
	}

	// Verify loaded data matches original
	if loadedExport.BlockHeight != originalExport.BlockHeight {
		t.Errorf("block height mismatch: expected %d, got %d", originalExport.BlockHeight, loadedExport.BlockHeight)
	}

	if loadedExport.NetworkSource != originalExport.NetworkSource {
		t.Errorf("network source mismatch: expected %s, got %s", originalExport.NetworkSource, loadedExport.NetworkSource)
	}

	if loadedExport.BinaryInfo.HashPrefix != originalExport.BinaryInfo.HashPrefix {
		t.Errorf("hash prefix mismatch: expected %s, got %s", originalExport.BinaryInfo.HashPrefix, loadedExport.BinaryInfo.HashPrefix)
	}
}

func TestRepository_Load_NotFound(t *testing.T) {
	ctx := context.Background()
	repo := NewRepository(t.TempDir())

	_, err := repo.Load(ctx, "/nonexistent/path")

	if err != domainExport.ErrExportNotFound {
		t.Errorf("expected ErrExportNotFound, got %v", err)
	}
}

func TestRepository_Load_EmptyPath(t *testing.T) {
	ctx := context.Background()
	repo := NewRepository(t.TempDir())

	_, err := repo.Load(ctx, "")

	if err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestRepository_Load_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	repo := NewRepository(t.TempDir())

	_, err := repo.Load(ctx, "/some/path")

	if err != context.Canceled {
		t.Errorf("expected context.Canceled error, got %v", err)
	}
}

func TestRepository_ListForDevnet(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	// Create exports directory
	exportsDir := filepath.Join(tmpDir, "exports")
	repo := NewRepository(exportsDir)

	// Create multiple exports with different timestamps
	timestamp1, err := time.Parse("20060102150405", "20240115120000")
	if err != nil {
		t.Fatalf("failed to parse timestamp1: %v", err)
	}
	timestamp2, err := time.Parse("20060102150405", "20240116120000")
	if err != nil {
		t.Fatalf("failed to parse timestamp2: %v", err)
	}

	hash1 := "a1b2c3d4e5f67890123456789012345678901234567890123456789012345678"
	hash2 := "b2c3d4e5f67890123456789012345678901234567890123456789012345678ab"

	// Create export1 (older)
	export1Dir := filepath.Join(exportsDir, "mainnet-a1b2c3d4-1000000-20240115120000")
	binaryInfo1, err := domainExport.NewBinaryInfo("/usr/local/bin/stabled", "", hash1, "v1.0.0", domainExport.ExecutionModeLocal)
	if err != nil {
		t.Fatalf("failed to create binaryInfo1: %v", err)
	}
	metadata1, err := domainExport.NewExportMetadata(timestamp1, 1000000, "mainnet", 0, "/usr/local/bin/stabled", hash1, "v1.0.0", "", "stable-1", 4, 10, "local", "/home/user/.stable-devnet")
	if err != nil {
		t.Fatalf("failed to create metadata1: %v", err)
	}
	export1, err := domainExport.NewExport(export1Dir, timestamp1, 1000000, "mainnet", binaryInfo1, metadata1, filepath.Join(export1Dir, "genesis-1000000-a1b2c3d4.json"))
	if err != nil {
		t.Fatalf("failed to create export1: %v", err)
	}

	// Create export2 (newer)
	export2Dir := filepath.Join(exportsDir, "mainnet-b2c3d4e5-2000000-20240116120000")
	binaryInfo2, err := domainExport.NewBinaryInfo("/usr/local/bin/stabled", "", hash2, "v1.0.0", domainExport.ExecutionModeLocal)
	if err != nil {
		t.Fatalf("failed to create binaryInfo2: %v", err)
	}
	metadata2, err := domainExport.NewExportMetadata(timestamp2, 2000000, "mainnet", 0, "/usr/local/bin/stabled", hash2, "v1.0.0", "", "stable-1", 4, 10, "local", "/home/user/.stable-devnet")
	if err != nil {
		t.Fatalf("failed to create metadata2: %v", err)
	}
	export2, err := domainExport.NewExport(export2Dir, timestamp2, 2000000, "mainnet", binaryInfo2, metadata2, filepath.Join(export2Dir, "genesis-2000000-b2c3d4e5.json"))
	if err != nil {
		t.Fatalf("failed to create export2: %v", err)
	}

	// Save both
	if err := repo.Save(ctx, export1); err != nil {
		t.Fatalf("failed to save export1: %v", err)
	}
	if err := repo.Save(ctx, export2); err != nil {
		t.Fatalf("failed to save export2: %v", err)
	}

	// List exports
	listInterface, err := repo.ListForDevnet(ctx, tmpDir)
	if err != nil {
		t.Fatalf("failed to list exports: %v", err)
	}

	exports, ok := listInterface.([]*domainExport.Export)
	if !ok {
		t.Fatalf("expected []*domainExport.Export, got %T", listInterface)
	}

	if len(exports) != 2 {
		t.Fatalf("expected 2 exports, got %d", len(exports))
	}

	// Verify exports are sorted by timestamp (newest first)
	// Export2 should come first since it has a later timestamp
	if exports[0].BlockHeight != 2000000 {
		t.Errorf("expected first export to have height 2000000, got %d", exports[0].BlockHeight)
	}
}

func TestRepository_ListForDevnet_NoExportsDir(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	repo := NewRepository(filepath.Join(tmpDir, "exports"))

	// List when exports directory doesn't exist
	listInterface, err := repo.ListForDevnet(ctx, tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	exports, ok := listInterface.([]*domainExport.Export)
	if !ok {
		t.Fatalf("expected []*domainExport.Export, got %T", listInterface)
	}

	if len(exports) != 0 {
		t.Errorf("expected empty list, got %d exports", len(exports))
	}
}

func TestRepository_ListForDevnet_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	repo := NewRepository(t.TempDir())

	_, err := repo.ListForDevnet(ctx, "/tmp")

	if err != context.Canceled {
		t.Errorf("expected context.Canceled error, got %v", err)
	}
}

func TestRepository_Delete(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	repo := NewRepository(tmpDir)
	exportDir := filepath.Join(tmpDir, "mainnet-a1b2c3d4-1000000-20240115120000")
	export := createTestExport(t, exportDir)

	// Save first
	if err := repo.Save(ctx, export); err != nil {
		t.Fatalf("failed to save export: %v", err)
	}

	// Verify it exists
	if _, err := os.Stat(exportDir); os.IsNotExist(err) {
		t.Fatal("export directory should exist before delete")
	}

	// Delete
	err := repo.Delete(ctx, exportDir)
	if err != nil {
		t.Fatalf("failed to delete export: %v", err)
	}

	// Verify it's deleted
	if _, err := os.Stat(exportDir); !os.IsNotExist(err) {
		t.Error("export directory should not exist after delete")
	}
}

func TestRepository_Delete_NotFound(t *testing.T) {
	ctx := context.Background()
	repo := NewRepository(t.TempDir())

	err := repo.Delete(ctx, "/nonexistent/path")

	if err != domainExport.ErrExportNotFound {
		t.Errorf("expected ErrExportNotFound, got %v", err)
	}
}

func TestRepository_Delete_EmptyPath(t *testing.T) {
	ctx := context.Background()
	repo := NewRepository(t.TempDir())

	err := repo.Delete(ctx, "")

	if err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestRepository_Delete_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	repo := NewRepository(t.TempDir())

	err := repo.Delete(ctx, "/some/path")

	if err != context.Canceled {
		t.Errorf("expected context.Canceled error, got %v", err)
	}
}

func TestRepository_Validate(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	repo := NewRepository(tmpDir)
	exportDir := filepath.Join(tmpDir, "mainnet-a1b2c3d4-1000000-20240115120000")
	export := createTestExport(t, exportDir)

	// Save export
	if err := repo.Save(ctx, export); err != nil {
		t.Fatalf("failed to save export: %v", err)
	}

	// Create genesis file to make it complete
	genesisPath := export.GenesisFilePath
	genesisContent := []byte(`{"chain_id":"stable-1","app_state":{}}`)
	if err := os.WriteFile(genesisPath, genesisContent, 0644); err != nil {
		t.Fatalf("failed to write genesis file: %v", err)
	}

	// Validate
	resultInterface, err := repo.Validate(ctx, exportDir)
	if err != nil {
		t.Fatalf("failed to validate export: %v", err)
	}

	result, ok := resultInterface.(*ValidationResult)
	if !ok {
		t.Fatalf("expected *ValidationResult, got %T", resultInterface)
	}

	if !result.IsComplete {
		t.Error("expected export to be complete")
	}

	if len(result.MissingFiles) != 0 {
		t.Errorf("expected no missing files, got %v", result.MissingFiles)
	}
}

func TestRepository_Validate_Incomplete(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	repo := NewRepository(tmpDir)
	exportDir := filepath.Join(tmpDir, "mainnet-a1b2c3d4-1000000-20240115120000")
	export := createTestExport(t, exportDir)

	// Save export (but don't create genesis file)
	if err := repo.Save(ctx, export); err != nil {
		t.Fatalf("failed to save export: %v", err)
	}

	// Validate
	resultInterface, err := repo.Validate(ctx, exportDir)

	// Should return ErrExportIncomplete
	if err != domainExport.ErrExportIncomplete {
		t.Errorf("expected ErrExportIncomplete, got %v", err)
	}

	result, ok := resultInterface.(*ValidationResult)
	if !ok {
		t.Fatalf("expected *ValidationResult, got %T", resultInterface)
	}

	if result.IsComplete {
		t.Error("expected export to be incomplete")
	}

	if len(result.MissingFiles) == 0 {
		t.Error("expected missing files")
	}
}

func TestRepository_Validate_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	repo := NewRepository(t.TempDir())

	_, err := repo.Validate(ctx, "/some/path")

	if err != context.Canceled {
		t.Errorf("expected context.Canceled error, got %v", err)
	}
}

func TestRepository_GetExportsDirectory(t *testing.T) {
	repo := NewRepository("/tmp/base")

	exportsDir := repo.GetExportsDirectory("/home/user/.stable-devnet")
	expected := "/home/user/.stable-devnet/exports"

	if exportsDir != expected {
		t.Errorf("expected '%s', got '%s'", expected, exportsDir)
	}
}

func TestRepository_GenerateExportPath(t *testing.T) {
	tmpDir := t.TempDir()
	repo := NewRepository(tmpDir)

	exportDir := filepath.Join(tmpDir, "mainnet-a1b2c3d4-1000000-20240115120000")
	export := createTestExport(t, exportDir)

	generatedPath := repo.GenerateExportPath("/home/user/.stable-devnet", export)
	expectedPath := "/home/user/.stable-devnet/exports/" + export.DirectoryName()

	if generatedPath != expectedPath {
		t.Errorf("expected '%s', got '%s'", expectedPath, generatedPath)
	}
}

func TestParseDirectoryName(t *testing.T) {
	tests := []struct {
		name           string
		dirName        string
		expectError    bool
		expectedNet    string
		expectedHash   string
		expectedHeight int64
	}{
		{
			name:           "valid format",
			dirName:        "mainnet-abc12345-1000000-20240115120000",
			expectError:    false,
			expectedNet:    "mainnet",
			expectedHash:   "abc12345",
			expectedHeight: 1000000,
		},
		{
			name:        "invalid format - too few parts",
			dirName:     "mainnet-abc12345-1000000",
			expectError: true,
		},
		{
			name:        "invalid format - bad height",
			dirName:     "mainnet-abc12345-abc-20240115120000",
			expectError: true,
		},
		{
			name:        "invalid format - bad timestamp",
			dirName:     "mainnet-abc12345-1000000-invalid",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			network, hashPrefix, height, timestamp, err := ParseDirectoryName(tt.dirName)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if network != tt.expectedNet {
				t.Errorf("expected network '%s', got '%s'", tt.expectedNet, network)
			}

			if hashPrefix != tt.expectedHash {
				t.Errorf("expected hash prefix '%s', got '%s'", tt.expectedHash, hashPrefix)
			}

			if height != tt.expectedHeight {
				t.Errorf("expected height %d, got %d", tt.expectedHeight, height)
			}

			if timestamp.IsZero() {
				t.Error("expected non-zero timestamp")
			}
		})
	}
}
