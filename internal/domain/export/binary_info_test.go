package export

import (
	"testing"

	"github.com/b-harvest/devnet-builder/types"
)

func TestNewBinaryInfo_LocalMode(t *testing.T) {
	hash := "a1b2c3d4e5f67890123456789012345678901234567890123456789012345678"

	bi, err := NewBinaryInfo(
		"/usr/local/bin/stabled",
		"",
		hash,
		"v1.0.0",
		types.ExecutionModeLocal,
	)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if bi.Path != "/usr/local/bin/stabled" {
		t.Errorf("expected path '/usr/local/bin/stabled', got '%s'", bi.Path)
	}

	if bi.HashPrefix != "a1b2c3d4" {
		t.Errorf("expected hash prefix 'a1b2c3d4', got '%s'", bi.HashPrefix)
	}

	if bi.Version != "v1.0.0" {
		t.Errorf("expected version 'v1.0.0', got '%s'", bi.Version)
	}
}

func TestNewBinaryInfo_DockerMode(t *testing.T) {
	hash := "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"

	bi, err := NewBinaryInfo(
		"",
		"ghcr.io/stablelabs/stable:v1.0.0",
		hash,
		"v1.0.0",
		types.ExecutionModeDocker,
	)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if bi.DockerImage != "ghcr.io/stablelabs/stable:v1.0.0" {
		t.Errorf("expected docker image 'ghcr.io/stablelabs/stable:v1.0.0', got '%s'", bi.DockerImage)
	}

	if bi.HashPrefix != "12345678" {
		t.Errorf("expected hash prefix '12345678', got '%s'", bi.HashPrefix)
	}
}

func TestBinaryInfo_Validate_BothPathAndImage(t *testing.T) {
	_, err := NewBinaryInfo(
		"/usr/local/bin/stabled",
		"ghcr.io/stablelabs/stable:v1.0.0",
		"a1b2c3d4e5f67890123456789012345678901234567890123456789012345678",
		"v1.0.0",
		types.ExecutionModeLocal,
	)

	if err == nil {
		t.Fatal("expected error when both Path and DockerImage are set")
	}

	validationErr, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}

	if validationErr.Field != "Path/DockerImage" {
		t.Errorf("expected field 'Path/DockerImage', got '%s'", validationErr.Field)
	}
}

func TestBinaryInfo_Validate_NeitherPathNorImage(t *testing.T) {
	_, err := NewBinaryInfo(
		"",
		"",
		"a1b2c3d4e5f67890123456789012345678901234567890123456789012345678",
		"v1.0.0",
		types.ExecutionModeLocal,
	)

	if err == nil {
		t.Fatal("expected error when neither Path nor DockerImage is set")
	}

	validationErr, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}

	if validationErr.Field != "Path/DockerImage" {
		t.Errorf("expected field 'Path/DockerImage', got '%s'", validationErr.Field)
	}
}

func TestBinaryInfo_Validate_LocalModeWithoutPath(t *testing.T) {
	_, err := NewBinaryInfo(
		"",
		"",
		"a1b2c3d4e5f67890123456789012345678901234567890123456789012345678",
		"v1.0.0",
		types.ExecutionModeLocal,
	)

	if err == nil {
		t.Fatal("expected error when ExecutionMode is local but Path is empty")
	}
}

func TestBinaryInfo_Validate_DockerModeWithoutImage(t *testing.T) {
	_, err := NewBinaryInfo(
		"",
		"",
		"a1b2c3d4e5f67890123456789012345678901234567890123456789012345678",
		"v1.0.0",
		types.ExecutionModeDocker,
	)

	if err == nil {
		t.Fatal("expected error when ExecutionMode is docker but DockerImage is empty")
	}
}

func TestBinaryInfo_Validate_InvalidHashFormat(t *testing.T) {
	tests := []struct {
		name string
		hash string
	}{
		{"too short", "a1b2c3d4"},
		{"too long", "a1b2c3d4e5f6789012345678901234567890123456789012345678901234567890abc"},
		{"invalid characters", "g1b2c3d4e5f6789012345678901234567890123456789012345678901234567890"},
		{"uppercase", "A1B2C3D4E5F6789012345678901234567890123456789012345678901234567890"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewBinaryInfo(
				"/usr/local/bin/stabled",
				"",
				tt.hash,
				"v1.0.0",
				types.ExecutionModeLocal,
			)

			if err == nil {
				t.Fatalf("expected error for hash '%s'", tt.hash)
			}
		})
	}
}

func TestBinaryInfo_Validate_EmptyVersion(t *testing.T) {
	_, err := NewBinaryInfo(
		"/usr/local/bin/stabled",
		"",
		"a1b2c3d4e5f67890123456789012345678901234567890123456789012345678",
		"",
		types.ExecutionModeLocal,
	)

	if err == nil {
		t.Fatal("expected error when Version is empty")
	}

	validationErr, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}

	if validationErr.Field != "Version" {
		t.Errorf("expected field 'Version', got '%s'", validationErr.Field)
	}
}

func TestBinaryInfo_Validate_InvalidExecutionMode(t *testing.T) {
	bi := &BinaryInfo{
		Path:          "/usr/local/bin/stabled",
		Hash:          "a1b2c3d4e5f6789012345678901234567890123456789012345678901234567890",
		HashPrefix:    "a1b2c3d4",
		Version:       "v1.0.0",
		ExecutionMode: "invalid",
	}

	err := bi.Validate()
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

func TestBinaryInfo_HashPrefixGeneration(t *testing.T) {
	hash := "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"

	bi, err := NewBinaryInfo(
		"/usr/local/bin/stabled",
		"",
		hash,
		"v1.0.0",
		types.ExecutionModeLocal,
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedPrefix := "abcdef12"
	if bi.HashPrefix != expectedPrefix {
		t.Errorf("expected hash prefix '%s', got '%s'", expectedPrefix, bi.HashPrefix)
	}

	// Verify hash prefix matches first 8 chars
	if bi.HashPrefix != hash[0:8] {
		t.Errorf("hash prefix doesn't match first 8 characters of hash")
	}
}

func TestBinaryInfo_GetIdentifier_WithHashPrefix(t *testing.T) {
	bi, err := NewBinaryInfo(
		"/usr/local/bin/stabled",
		"",
		"a1b2c3d4e5f67890123456789012345678901234567890123456789012345678",
		"v1.0.0",
		types.ExecutionModeLocal,
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	identifier := bi.GetIdentifier()
	if identifier != "a1b2c3d4" {
		t.Errorf("expected identifier 'a1b2c3d4', got '%s'", identifier)
	}
}

func TestBinaryInfo_GetIdentifier_WithoutHashPrefix(t *testing.T) {
	// Create BinaryInfo without hash (HashPrefix will be empty)
	bi := &BinaryInfo{
		Path:          "/usr/local/bin/stabled",
		Version:       "1.0.0",
		ExecutionMode: types.ExecutionModeLocal,
	}

	identifier := bi.GetIdentifier()
	expected := "v1.0.0"
	if identifier != expected {
		t.Errorf("expected identifier '%s', got '%s'", expected, identifier)
	}
}
