package export

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNewExportMetadata_Valid(t *testing.T) {
	timestamp := time.Now()
	hash := "a1b2c3d4e5f67890123456789012345678901234567890123456789012345678"

	em, err := NewExportMetadata(
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
		t.Fatalf("expected no error, got %v", err)
	}

	if em.BlockHeight != 1000000 {
		t.Errorf("expected block height 1000000, got %d", em.BlockHeight)
	}

	if em.BinaryHashPrefix != "a1b2c3d4" {
		t.Errorf("expected hash prefix 'a1b2c3d4', got '%s'", em.BinaryHashPrefix)
	}

	if em.NetworkSource != "mainnet" {
		t.Errorf("expected network source 'mainnet', got '%s'", em.NetworkSource)
	}
}

func TestExportMetadata_ToJSON_FromJSON_RoundTrip(t *testing.T) {
	timestamp := time.Now().UTC().Truncate(time.Second) // Truncate to avoid precision issues
	hash := "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"

	original, err := NewExportMetadata(
		timestamp,
		5000000,
		"testnet",
		4999999,
		"",
		hash,
		"v2.0.0",
		"ghcr.io/stablelabs/stable:v2.0.0",
		"stable-testnet-1",
		3,
		50,
		"docker",
		"/home/user/.stable-testnet",
	)

	if err != nil {
		t.Fatalf("failed to create metadata: %v", err)
	}

	// Serialize to JSON
	jsonData, err := original.ToJSON()
	if err != nil {
		t.Fatalf("failed to serialize to JSON: %v", err)
	}

	// Deserialize from JSON
	restored, err := FromJSON(jsonData)
	if err != nil {
		t.Fatalf("failed to deserialize from JSON: %v", err)
	}

	// Verify fields match
	if restored.BlockHeight != original.BlockHeight {
		t.Errorf("block height mismatch: expected %d, got %d", original.BlockHeight, restored.BlockHeight)
	}

	if restored.NetworkSource != original.NetworkSource {
		t.Errorf("network source mismatch: expected %s, got %s", original.NetworkSource, restored.NetworkSource)
	}

	if restored.BinaryHash != original.BinaryHash {
		t.Errorf("binary hash mismatch: expected %s, got %s", original.BinaryHash, restored.BinaryHash)
	}

	if !restored.ExportTimestamp.Equal(original.ExportTimestamp) {
		t.Errorf("timestamp mismatch: expected %v, got %v", original.ExportTimestamp, restored.ExportTimestamp)
	}
}

func TestExportMetadata_Validate_BlockHeightZero(t *testing.T) {
	_, err := NewExportMetadata(
		time.Now(),
		0, // Invalid
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

func TestExportMetadata_Validate_InvalidNetworkSource(t *testing.T) {
	_, err := NewExportMetadata(
		time.Now(),
		1000000,
		"devnet", // Invalid
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

func TestExportMetadata_Validate_InvalidExecutionMode(t *testing.T) {
	_, err := NewExportMetadata(
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
		"hybrid", // Invalid
		"/home/user/.stable-devnet",
	)

	if err == nil {
		t.Fatal("expected error for invalid execution mode")
	}

	validationErr, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}

	if validationErr.Field != "ExecutionMode" {
		t.Errorf("expected field 'ExecutionMode', got '%s'", validationErr.Field)
	}
}

func TestExportMetadata_Validate_BothBinaryPathAndDockerImage(t *testing.T) {
	_, err := NewExportMetadata(
		time.Now(),
		1000000,
		"mainnet",
		0,
		"/usr/local/bin/stabled",
		"a1b2c3d4e5f67890123456789012345678901234567890123456789012345678",
		"v1.0.0",
		"ghcr.io/stablelabs/stable:v1.0.0", // Both set
		"stable-1",
		4,
		10,
		"local",
		"/home/user/.stable-devnet",
	)

	if err == nil {
		t.Fatal("expected error when both BinaryPath and DockerImage are set")
	}

	validationErr, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}

	if validationErr.Field != "BinaryPath/DockerImage" {
		t.Errorf("expected field 'BinaryPath/DockerImage', got '%s'", validationErr.Field)
	}
}

func TestExportMetadata_Validate_NeitherBinaryPathNorDockerImage(t *testing.T) {
	_, err := NewExportMetadata(
		time.Now(),
		1000000,
		"mainnet",
		0,
		"", // Neither set
		"a1b2c3d4e5f6789012345678901234567890123456789012345678901234567890",
		"v1.0.0",
		"",
		"stable-1",
		4,
		10,
		"local",
		"/home/user/.stable-devnet",
	)

	if err == nil {
		t.Fatal("expected error when neither BinaryPath nor DockerImage is set")
	}
}

func TestExportMetadata_Validate_LocalModeWithoutBinaryPath(t *testing.T) {
	_, err := NewExportMetadata(
		time.Now(),
		1000000,
		"mainnet",
		0,
		"",
		"a1b2c3d4e5f6789012345678901234567890123456789012345678901234567890",
		"v1.0.0",
		"",
		"stable-1",
		4,
		10,
		"local",
		"/home/user/.stable-devnet",
	)

	if err == nil {
		t.Fatal("expected error for local mode without binary path")
	}
}

func TestExportMetadata_Validate_DockerModeWithoutDockerImage(t *testing.T) {
	_, err := NewExportMetadata(
		time.Now(),
		1000000,
		"mainnet",
		0,
		"",
		"a1b2c3d4e5f6789012345678901234567890123456789012345678901234567890",
		"v1.0.0",
		"",
		"stable-1",
		4,
		10,
		"docker",
		"/home/user/.stable-devnet",
	)

	if err == nil {
		t.Fatal("expected error for docker mode without docker image")
	}
}

func TestExportMetadata_Validate_EmptyBinaryVersion(t *testing.T) {
	_, err := NewExportMetadata(
		time.Now(),
		1000000,
		"mainnet",
		0,
		"/usr/local/bin/stabled",
		"a1b2c3d4e5f67890123456789012345678901234567890123456789012345678",
		"", // Empty
		"",
		"stable-1",
		4,
		10,
		"local",
		"/home/user/.stable-devnet",
	)

	if err == nil {
		t.Fatal("expected error for empty binary version")
	}

	validationErr, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}

	if validationErr.Field != "BinaryVersion" {
		t.Errorf("expected field 'BinaryVersion', got '%s'", validationErr.Field)
	}
}

func TestExportMetadata_Validate_EmptyChainID(t *testing.T) {
	_, err := NewExportMetadata(
		time.Now(),
		1000000,
		"mainnet",
		0,
		"/usr/local/bin/stabled",
		"a1b2c3d4e5f67890123456789012345678901234567890123456789012345678",
		"v1.0.0",
		"",
		"", // Empty
		4,
		10,
		"local",
		"/home/user/.stable-devnet",
	)

	if err == nil {
		t.Fatal("expected error for empty chain ID")
	}

	validationErr, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}

	if validationErr.Field != "ChainID" {
		t.Errorf("expected field 'ChainID', got '%s'", validationErr.Field)
	}
}

func TestExportMetadata_Validate_ZeroValidators(t *testing.T) {
	_, err := NewExportMetadata(
		time.Now(),
		1000000,
		"mainnet",
		0,
		"/usr/local/bin/stabled",
		"a1b2c3d4e5f67890123456789012345678901234567890123456789012345678",
		"v1.0.0",
		"",
		"stable-1",
		0, // Invalid
		10,
		"local",
		"/home/user/.stable-devnet",
	)

	if err == nil {
		t.Fatal("expected error for 0 validators")
	}

	validationErr, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}

	if validationErr.Field != "NumValidators" {
		t.Errorf("expected field 'NumValidators', got '%s'", validationErr.Field)
	}
}

func TestExportMetadata_Validate_NegativeAccounts(t *testing.T) {
	_, err := NewExportMetadata(
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
		-1, // Invalid
		"local",
		"/home/user/.stable-devnet",
	)

	if err == nil {
		t.Fatal("expected error for negative accounts")
	}

	validationErr, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}

	if validationErr.Field != "NumAccounts" {
		t.Errorf("expected field 'NumAccounts', got '%s'", validationErr.Field)
	}
}

func TestExportMetadata_Validate_EmptyDevnetHomeDir(t *testing.T) {
	_, err := NewExportMetadata(
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
		"", // Empty
	)

	if err == nil {
		t.Fatal("expected error for empty devnet home dir")
	}

	validationErr, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}

	if validationErr.Field != "DevnetHomeDir" {
		t.Errorf("expected field 'DevnetHomeDir', got '%s'", validationErr.Field)
	}
}

func TestExportMetadata_JSONTags(t *testing.T) {
	timestamp := time.Now().UTC().Truncate(time.Second)
	hash := "a1b2c3d4e5f67890123456789012345678901234567890123456789012345678"

	em, err := NewExportMetadata(
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
		t.Fatalf("failed to create metadata: %v", err)
	}

	jsonData, err := em.ToJSON()
	if err != nil {
		t.Fatalf("failed to serialize: %v", err)
	}

	// Parse JSON to verify tags
	var rawJSON map[string]interface{}
	if err := json.Unmarshal(jsonData, &rawJSON); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	// Verify snake_case JSON tags are used
	requiredFields := []string{
		"export_timestamp",
		"block_height",
		"network_source",
		"binary_hash",
		"binary_hash_prefix",
		"binary_version",
		"chain_id",
		"num_validators",
		"num_accounts",
		"execution_mode",
		"devnet_home_dir",
	}

	for _, field := range requiredFields {
		if _, exists := rawJSON[field]; !exists {
			t.Errorf("expected field '%s' in JSON output", field)
		}
	}
}
