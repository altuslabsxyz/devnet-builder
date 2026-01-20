package export

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/altuslabsxyz/devnet-builder/types"
)

func createValidBinaryInfo(t *testing.T) *BinaryInfo {
	t.Helper()
	bi, err := NewBinaryInfo(
		"/usr/local/bin/stabled",
		"",
		"a1b2c3d4e5f67890123456789012345678901234567890123456789012345678",
		"v1.0.0",
		types.ExecutionModeLocal,
	)
	if err != nil {
		t.Fatalf("failed to create BinaryInfo: %v", err)
	}
	return bi
}

func createValidMetadata(t *testing.T) *ExportMetadata {
	t.Helper()
	em, err := NewExportMetadata(
		time.Now(),
		1000000,
		"mainnet",
		0,
		"/usr/local/bin/stabled",
		"a1b2c3d4e5f67890123456789012345678901234567890123456789012345678",
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
	return em
}

func TestNewExport_Valid(t *testing.T) {
	timestamp := time.Now()
	binaryInfo := createValidBinaryInfo(t)
	metadata := createValidMetadata(t)

	// Use valid directory name format
	dirPath := "/home/user/.stable-devnet/exports/mainnet-a1b2c3d4-1000000-20240115120000"

	export, err := NewExport(
		dirPath,
		timestamp,
		1000000,
		"mainnet",
		binaryInfo,
		metadata,
		filepath.Join(dirPath, "genesis-1000000-a1b2c3d4.json"),
	)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if export.BlockHeight != 1000000 {
		t.Errorf("expected block height 1000000, got %d", export.BlockHeight)
	}

	if export.NetworkSource != "mainnet" {
		t.Errorf("expected network source 'mainnet', got '%s'", export.NetworkSource)
	}
}

func TestExport_Validate_EmptyDirectoryPath(t *testing.T) {
	binaryInfo := createValidBinaryInfo(t)
	metadata := createValidMetadata(t)

	_, err := NewExport(
		"", // Empty
		time.Now(),
		1000000,
		"mainnet",
		binaryInfo,
		metadata,
		"/path/to/genesis.json",
	)

	if err == nil {
		t.Fatal("expected error for empty directory path")
	}

	validationErr, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}

	if validationErr.Field != "DirectoryPath" {
		t.Errorf("expected field 'DirectoryPath', got '%s'", validationErr.Field)
	}
}

func TestExport_Validate_InvalidDirectoryName(t *testing.T) {
	binaryInfo := createValidBinaryInfo(t)
	metadata := createValidMetadata(t)

	tests := []struct {
		name    string
		dirPath string
	}{
		{"no network prefix", "/path/to/abc12345-1000000-20240115120000"},
		{"invalid network", "/path/to/devnet-abc12345-1000000-20240115120000"},
		{"short hash", "/path/to/mainnet-abc123-1000000-20240115120000"},
		{"missing height", "/path/to/mainnet-abc12345-20240115120000"},
		{"invalid timestamp", "/path/to/mainnet-abc12345-1000000-2024011512"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewExport(
				tt.dirPath,
				time.Now(),
				1000000,
				"mainnet",
				binaryInfo,
				metadata,
				filepath.Join(tt.dirPath, "genesis.json"),
			)

			if err == nil {
				t.Fatalf("expected error for invalid directory name: %s", tt.dirPath)
			}
		})
	}
}

func TestExport_Validate_InvalidBlockHeight(t *testing.T) {
	binaryInfo := createValidBinaryInfo(t)
	metadata := createValidMetadata(t)

	_, err := NewExport(
		"/home/user/.stable-devnet/exports/mainnet-a1b2c3d4-1000000-20240115120000",
		time.Now(),
		0, // Invalid
		"mainnet",
		binaryInfo,
		metadata,
		"/path/to/genesis.json",
	)

	if err == nil {
		t.Fatal("expected error for block height 0")
	}

	validationErr, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}

	if validationErr.Field != "BlockHeight" {
		t.Errorf("expected field 'BlockHeight', got '%s'", validationErr.Field)
	}
}

func TestExport_Validate_InvalidNetworkSource(t *testing.T) {
	binaryInfo := createValidBinaryInfo(t)
	metadata := createValidMetadata(t)

	_, err := NewExport(
		"/home/user/.stable-devnet/exports/mainnet-a1b2c3d4-1000000-20240115120000",
		time.Now(),
		1000000,
		"devnet", // Invalid
		binaryInfo,
		metadata,
		"/path/to/genesis.json",
	)

	if err == nil {
		t.Fatal("expected error for invalid network source")
	}

	validationErr, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}

	if validationErr.Field != "NetworkSource" {
		t.Errorf("expected field 'NetworkSource', got '%s'", validationErr.Field)
	}
}

func TestExport_Validate_NilBinaryInfo(t *testing.T) {
	metadata := createValidMetadata(t)

	_, err := NewExport(
		"/home/user/.stable-devnet/exports/mainnet-a1b2c3d4-1000000-20240115120000",
		time.Now(),
		1000000,
		"mainnet",
		nil, // Nil
		metadata,
		"/path/to/genesis.json",
	)

	if err == nil {
		t.Fatal("expected error for nil BinaryInfo")
	}

	validationErr, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}

	if validationErr.Field != "BinaryInfo" {
		t.Errorf("expected field 'BinaryInfo', got '%s'", validationErr.Field)
	}
}

func TestExport_Validate_NilMetadata(t *testing.T) {
	binaryInfo := createValidBinaryInfo(t)

	_, err := NewExport(
		"/home/user/.stable-devnet/exports/mainnet-a1b2c3d4-1000000-20240115120000",
		time.Now(),
		1000000,
		"mainnet",
		binaryInfo,
		nil, // Nil
		"/path/to/genesis.json",
	)

	if err == nil {
		t.Fatal("expected error for nil Metadata")
	}

	validationErr, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}

	if validationErr.Field != "Metadata" {
		t.Errorf("expected field 'Metadata', got '%s'", validationErr.Field)
	}
}

func TestExport_Validate_EmptyGenesisFilePath(t *testing.T) {
	binaryInfo := createValidBinaryInfo(t)
	metadata := createValidMetadata(t)

	_, err := NewExport(
		"/home/user/.stable-devnet/exports/mainnet-a1b2c3d4-1000000-20240115120000",
		time.Now(),
		1000000,
		"mainnet",
		binaryInfo,
		metadata,
		"", // Empty
	)

	if err == nil {
		t.Fatal("expected error for empty genesis file path")
	}

	validationErr, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}

	if validationErr.Field != "GenesisFilePath" {
		t.Errorf("expected field 'GenesisFilePath', got '%s'", validationErr.Field)
	}
}

func TestExport_DirectoryName(t *testing.T) {
	timestamp, _ := time.Parse("20060102150405", "20240115120000")
	binaryInfo := createValidBinaryInfo(t)
	metadata := createValidMetadata(t)

	export, err := NewExport(
		"/home/user/.stable-devnet/exports/mainnet-a1b2c3d4-1000000-20240115120000",
		timestamp,
		1000000,
		"mainnet",
		binaryInfo,
		metadata,
		"/path/to/genesis.json",
	)

	if err != nil {
		t.Fatalf("failed to create export: %v", err)
	}

	dirName := export.DirectoryName()
	expected := "mainnet-a1b2c3d4-1000000-20240115120000"

	if dirName != expected {
		t.Errorf("expected directory name '%s', got '%s'", expected, dirName)
	}
}

func TestExport_GetMetadataPath(t *testing.T) {
	binaryInfo := createValidBinaryInfo(t)
	metadata := createValidMetadata(t)
	dirPath := "/home/user/.stable-devnet/exports/mainnet-a1b2c3d4-1000000-20240115120000"

	export, err := NewExport(
		dirPath,
		time.Now(),
		1000000,
		"mainnet",
		binaryInfo,
		metadata,
		filepath.Join(dirPath, "genesis.json"),
	)

	if err != nil {
		t.Fatalf("failed to create export: %v", err)
	}

	metadataPath := export.GetMetadataPath()
	expected := filepath.Join(dirPath, "metadata.json")

	if metadataPath != expected {
		t.Errorf("expected metadata path '%s', got '%s'", expected, metadataPath)
	}
}

func TestExport_GetGenesisFileName(t *testing.T) {
	binaryInfo := createValidBinaryInfo(t)
	metadata := createValidMetadata(t)

	export, err := NewExport(
		"/home/user/.stable-devnet/exports/mainnet-a1b2c3d4-1000000-20240115120000",
		time.Now(),
		1000000,
		"mainnet",
		binaryInfo,
		metadata,
		"/path/to/genesis.json",
	)

	if err != nil {
		t.Fatalf("failed to create export: %v", err)
	}

	genesisName := export.GetGenesisFileName()
	expected := "genesis-1000000-a1b2c3d4.json"

	if genesisName != expected {
		t.Errorf("expected genesis file name '%s', got '%s'", expected, genesisName)
	}
}

func TestExport_IsComplete(t *testing.T) {
	// Create temporary directory structure
	tmpDir := t.TempDir()
	exportDir := filepath.Join(tmpDir, "mainnet-a1b2c3d4-1000000-20240115120000")
	if err := os.MkdirAll(exportDir, 0755); err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}

	genesisPath := filepath.Join(exportDir, "genesis-1000000-a1b2c3d4.json")
	metadataPath := filepath.Join(exportDir, "metadata.json")

	binaryInfo := createValidBinaryInfo(t)
	metadata := createValidMetadata(t)

	export, err := NewExport(
		exportDir,
		time.Now(),
		1000000,
		"mainnet",
		binaryInfo,
		metadata,
		genesisPath,
	)

	if err != nil {
		t.Fatalf("failed to create export: %v", err)
	}

	// Initially incomplete (no files)
	if export.IsComplete() {
		t.Error("expected export to be incomplete (no files)")
	}

	// Create metadata file
	if err := os.WriteFile(metadataPath, []byte("{}"), 0644); err != nil {
		t.Fatalf("failed to write metadata file: %v", err)
	}

	// Still incomplete (no genesis file)
	if export.IsComplete() {
		t.Error("expected export to be incomplete (no genesis file)")
	}

	// Create genesis file
	if err := os.WriteFile(genesisPath, []byte("{}"), 0644); err != nil {
		t.Fatalf("failed to write genesis file: %v", err)
	}

	// Now complete
	if !export.IsComplete() {
		t.Error("expected export to be complete")
	}
}

func TestIsValidDirectoryName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		valid bool
	}{
		{"valid mainnet", "mainnet-a1b2c3d4-1000000-20240115120000", true},
		{"valid testnet", "testnet-12345678-5000000-20230101000000", true},
		{"valid with sequence", "mainnet-abc12345-1000000-20240115120000-1", true},
		{"invalid network", "devnet-a1b2c3d4-1000000-20240115120000", false},
		{"short hash", "mainnet-a1b2c3-1000000-20240115120000", false},
		{"uppercase hash", "mainnet-A1B2C3D4-1000000-20240115120000", false},
		{"zero height", "mainnet-a1b2c3d4-0-20240115120000", false},
		{"negative height", "mainnet-a1b2c3d4--1-20240115120000", false},
		{"short timestamp", "mainnet-a1b2c3d4-1000000-2024011512", false},
		{"long timestamp", "mainnet-a1b2c3d4-1000000-202401151200000", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidDirectoryName(tt.input)
			if result != tt.valid {
				t.Errorf("expected isValidDirectoryName(%s) = %v, got %v", tt.input, tt.valid, result)
			}
		})
	}
}
