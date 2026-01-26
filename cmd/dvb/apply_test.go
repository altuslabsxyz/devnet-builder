// cmd/dvb/apply_test.go
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/altuslabsxyz/devnet-builder/internal/config"
)

func TestApplyValidation_ValidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	yamlPath := filepath.Join(tmpDir, "valid-devnet.yaml")

	content := `apiVersion: devnet.lagos/v1
kind: Devnet
metadata:
  name: test-devnet
spec:
  network: stable
  validators: 4
  mode: docker
`
	if err := os.WriteFile(yamlPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Load and validate
	loader := config.NewYAMLLoader()
	devnets, err := loader.Load(yamlPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if len(devnets) != 1 {
		t.Fatalf("expected 1 devnet, got %d", len(devnets))
	}

	// Validate with source
	validator := config.NewYAMLValidator()
	result := validator.ValidateWithSource(&devnets[0], yamlPath)

	if !result.Valid {
		t.Errorf("expected valid config, got errors: %s", result.Error())
	}
}

func TestApplyValidation_InvalidMode(t *testing.T) {
	tmpDir := t.TempDir()
	yamlPath := filepath.Join(tmpDir, "invalid-mode.yaml")

	content := `apiVersion: devnet.lagos/v1
kind: Devnet
metadata:
  name: test-devnet
spec:
  network: stable
  validators: 2
  mode: kubernetes
`
	if err := os.WriteFile(yamlPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// The loader validates during load, so invalid mode should fail at load time
	loader := config.NewYAMLLoader()
	_, err := loader.Load(yamlPath)

	// Should fail validation
	if err == nil {
		t.Error("expected Load to fail for invalid mode")
	}

	// Check that error message contains mode info
	errMsg := err.Error()
	if !strings.Contains(errMsg, "mode") {
		t.Errorf("expected error to mention 'mode', got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "kubernetes") {
		t.Errorf("expected error to mention 'kubernetes', got: %s", errMsg)
	}
}

func TestApplyValidation_TooManyValidators(t *testing.T) {
	tmpDir := t.TempDir()
	yamlPath := filepath.Join(tmpDir, "too-many-validators.yaml")

	content := `apiVersion: devnet.lagos/v1
kind: Devnet
metadata:
  name: test-devnet
spec:
  network: stable
  validators: 10
  mode: docker
`
	if err := os.WriteFile(yamlPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	loader := config.NewYAMLLoader()
	devnets, err := loader.Load(yamlPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	validator := config.NewYAMLValidator()
	result := validator.ValidateWithSource(&devnets[0], yamlPath)

	if result.Valid {
		t.Error("expected validation to fail for too many validators")
	}

	errMsg := result.Error()
	if !strings.Contains(errMsg, "validators") {
		t.Errorf("expected error to mention 'validators', got: %s", errMsg)
	}
}

func TestApplyValidation_MissingName(t *testing.T) {
	tmpDir := t.TempDir()
	yamlPath := filepath.Join(tmpDir, "missing-name.yaml")

	content := `apiVersion: devnet.lagos/v1
kind: Devnet
metadata:
  name: ""
spec:
  network: stable
  validators: 2
  mode: docker
`
	if err := os.WriteFile(yamlPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	loader := config.NewYAMLLoader()
	_, err := loader.Load(yamlPath)
	if err == nil {
		t.Error("expected Load to fail for missing name")
	}
}

func TestApplyValidation_MissingNetwork(t *testing.T) {
	tmpDir := t.TempDir()
	yamlPath := filepath.Join(tmpDir, "missing-network.yaml")

	content := `apiVersion: devnet.lagos/v1
kind: Devnet
metadata:
  name: test-devnet
spec:
  validators: 2
  mode: docker
`
	if err := os.WriteFile(yamlPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	loader := config.NewYAMLLoader()
	_, err := loader.Load(yamlPath)
	if err == nil {
		t.Error("expected Load to fail for missing network")
	}
}

func TestMultiDocumentYAML_ValidMultiDoc(t *testing.T) {
	tmpDir := t.TempDir()
	yamlPath := filepath.Join(tmpDir, "multi-devnet.yaml")

	content := `apiVersion: devnet.lagos/v1
kind: Devnet
metadata:
  name: devnet-one
spec:
  network: stable
  validators: 2
  mode: docker
---
apiVersion: devnet.lagos/v1
kind: Devnet
metadata:
  name: devnet-two
spec:
  network: stable
  validators: 4
  mode: local
`
	if err := os.WriteFile(yamlPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	loader := config.NewYAMLLoader()
	devnets, err := loader.Load(yamlPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if len(devnets) != 2 {
		t.Fatalf("expected 2 devnets, got %d", len(devnets))
	}

	// Validate both
	validator := config.NewYAMLValidator()

	result1 := validator.ValidateWithSource(&devnets[0], yamlPath)
	if !result1.Valid {
		t.Errorf("devnet-one should be valid: %s", result1.Error())
	}
	if devnets[0].Metadata.Name != "devnet-one" {
		t.Errorf("expected name devnet-one, got %s", devnets[0].Metadata.Name)
	}

	result2 := validator.ValidateWithSource(&devnets[1], yamlPath)
	if !result2.Valid {
		t.Errorf("devnet-two should be valid: %s", result2.Error())
	}
	if devnets[1].Metadata.Name != "devnet-two" {
		t.Errorf("expected name devnet-two, got %s", devnets[1].Metadata.Name)
	}
}

func TestMultiDocumentYAML_OneInvalid(t *testing.T) {
	tmpDir := t.TempDir()
	yamlPath := filepath.Join(tmpDir, "multi-one-invalid.yaml")

	content := `apiVersion: devnet.lagos/v1
kind: Devnet
metadata:
  name: devnet-valid
spec:
  network: stable
  validators: 2
  mode: docker
---
apiVersion: devnet.lagos/v1
kind: Devnet
metadata:
  name: devnet-invalid
spec:
  network: stable
  validators: 100
  mode: docker
`
	if err := os.WriteFile(yamlPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	loader := config.NewYAMLLoader()
	devnets, err := loader.Load(yamlPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if len(devnets) != 2 {
		t.Fatalf("expected 2 devnets, got %d", len(devnets))
	}

	validator := config.NewYAMLValidator()

	// First should be valid
	result1 := validator.ValidateWithSource(&devnets[0], yamlPath)
	if !result1.Valid {
		t.Errorf("devnet-valid should be valid: %s", result1.Error())
	}

	// Second should be invalid (too many validators)
	result2 := validator.ValidateWithSource(&devnets[1], yamlPath)
	if result2.Valid {
		t.Error("devnet-invalid should fail validation")
	}
}

func TestFormatValidationErrors_KubectlStyle(t *testing.T) {
	result := &config.ValidationResult{
		Valid: false,
		Errors: []config.ValidationError{
			{Field: "spec.mode", Message: "must be 'docker' or 'local', got \"kubernetes\""},
			{Field: "spec.validators", Message: "must be between 1 and 4, got 10"},
		},
	}

	output := config.FormatValidationErrors(result, "/tmp/test.yaml")

	// Check kubectl-style format
	if !strings.Contains(output, "error: error validating") {
		t.Errorf("expected 'error: error validating' prefix, got: %s", output)
	}
	if !strings.Contains(output, "/tmp/test.yaml") {
		t.Errorf("expected filename in output, got: %s", output)
	}
	if !strings.Contains(output, "spec.mode") {
		t.Errorf("expected 'spec.mode' in output, got: %s", output)
	}
	if !strings.Contains(output, "spec.validators") {
		t.Errorf("expected 'spec.validators' in output, got: %s", output)
	}
}

func TestFormatValidationErrors_ValidReturnsEmpty(t *testing.T) {
	result := &config.ValidationResult{
		Valid:  true,
		Errors: []config.ValidationError{},
	}

	output := config.FormatValidationErrors(result, "/tmp/test.yaml")

	if output != "" {
		t.Errorf("expected empty string for valid result, got: %s", output)
	}
}

func TestYAMLToProtoSpec_Conversion(t *testing.T) {
	yamlSpec := &config.YAMLDevnetSpec{
		Network:        "stable",
		NetworkType:    "mainnet",
		NetworkVersion: "v1.2.3",
		Validators:     3,
		FullNodes:      2,
		Mode:           "local",
	}

	protoSpec := yamlToProtoSpec(yamlSpec)

	if protoSpec.Plugin != "stable" {
		t.Errorf("expected Plugin 'stable', got '%s'", protoSpec.Plugin)
	}
	if protoSpec.NetworkType != "mainnet" {
		t.Errorf("expected NetworkType 'mainnet', got '%s'", protoSpec.NetworkType)
	}
	if protoSpec.SdkVersion != "v1.2.3" {
		t.Errorf("expected SdkVersion 'v1.2.3', got '%s'", protoSpec.SdkVersion)
	}
	if protoSpec.Validators != 3 {
		t.Errorf("expected Validators 3, got %d", protoSpec.Validators)
	}
	if protoSpec.FullNodes != 2 {
		t.Errorf("expected FullNodes 2, got %d", protoSpec.FullNodes)
	}
	if protoSpec.Mode != "local" {
		t.Errorf("expected Mode 'local', got '%s'", protoSpec.Mode)
	}
}

func TestYAMLToProtoSpec_Defaults(t *testing.T) {
	yamlSpec := &config.YAMLDevnetSpec{
		Network: "stable",
		// Leave validators and mode empty to test defaults
	}

	protoSpec := yamlToProtoSpec(yamlSpec)

	if protoSpec.Validators != 4 {
		t.Errorf("expected default Validators 4, got %d", protoSpec.Validators)
	}
	if protoSpec.Mode != "docker" {
		t.Errorf("expected default Mode 'docker', got '%s'", protoSpec.Mode)
	}
}
