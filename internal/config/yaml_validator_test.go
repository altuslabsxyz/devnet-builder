// internal/config/yaml_validator_test.go
package config

import (
	"strings"
	"testing"
)

func TestValidationError_Error(t *testing.T) {
	err := ValidationError{
		Field:   "metadata.name",
		Message: "is required",
	}

	got := err.Error()
	if got != "metadata.name: is required" {
		t.Errorf("ValidationError.Error() = %q, want %q", got, "metadata.name: is required")
	}
}

func TestValidationResult_Error(t *testing.T) {
	result := &ValidationResult{
		Valid: false,
		Errors: []ValidationError{
			{Field: "metadata.name", Message: "is required"},
			{Field: "spec.network", Message: "is required"},
		},
	}

	got := result.Error()
	if !strings.Contains(got, "metadata.name: is required") {
		t.Errorf("ValidationResult.Error() should contain metadata.name error, got %q", got)
	}
	if !strings.Contains(got, "spec.network: is required") {
		t.Errorf("ValidationResult.Error() should contain spec.network error, got %q", got)
	}
}

func TestValidationResult_Error_WhenValid(t *testing.T) {
	result := &ValidationResult{
		Valid:  true,
		Errors: []ValidationError{},
	}

	got := result.Error()
	if got != "" {
		t.Errorf("ValidationResult.Error() for valid result should be empty, got %q", got)
	}
}

func TestNewYAMLValidator_Defaults(t *testing.T) {
	v := NewYAMLValidator()

	if v.MaxValidators != 4 {
		t.Errorf("NewYAMLValidator().MaxValidators = %d, want 4", v.MaxValidators)
	}
	if v.MinValidators != 1 {
		t.Errorf("NewYAMLValidator().MinValidators = %d, want 1", v.MinValidators)
	}
	if v.MaxFullNodes != 10 {
		t.Errorf("NewYAMLValidator().MaxFullNodes = %d, want 10", v.MaxFullNodes)
	}
	if len(v.ValidModes) != 2 {
		t.Errorf("NewYAMLValidator().ValidModes length = %d, want 2", len(v.ValidModes))
	}
	if v.ValidModes[0] != "docker" || v.ValidModes[1] != "local" {
		t.Errorf("NewYAMLValidator().ValidModes = %v, want [docker local]", v.ValidModes)
	}
}

func TestYAMLValidator_Validate_ValidMinimalConfig(t *testing.T) {
	v := NewYAMLValidator()
	devnet := &YAMLDevnet{
		APIVersion: SupportedAPIVersion,
		Kind:       SupportedKind,
		Metadata:   YAMLMetadata{Name: "test-devnet"},
		Spec: YAMLDevnetSpec{
			Network:    "stable",
			Validators: 1,
		},
	}

	result := v.Validate(devnet)

	if !result.Valid {
		t.Errorf("Validate() should pass for valid minimal config, errors: %v", result.Errors)
	}
}

func TestYAMLValidator_Validate_MissingName(t *testing.T) {
	v := NewYAMLValidator()
	devnet := &YAMLDevnet{
		APIVersion: SupportedAPIVersion,
		Kind:       SupportedKind,
		Metadata:   YAMLMetadata{Name: ""},
		Spec: YAMLDevnetSpec{
			Network:    "stable",
			Validators: 1,
		},
	}

	result := v.Validate(devnet)

	if result.Valid {
		t.Error("Validate() should fail for missing name")
	}
	foundError := false
	for _, err := range result.Errors {
		if err.Field == "metadata.name" && strings.Contains(err.Message, "required") {
			foundError = true
			break
		}
	}
	if !foundError {
		t.Errorf("Validate() should contain error 'metadata.name is required', got: %v", result.Errors)
	}
}

func TestYAMLValidator_Validate_InvalidValidatorsCount(t *testing.T) {
	v := NewYAMLValidator()
	devnet := &YAMLDevnet{
		APIVersion: SupportedAPIVersion,
		Kind:       SupportedKind,
		Metadata:   YAMLMetadata{Name: "test"},
		Spec: YAMLDevnetSpec{
			Network:    "stable",
			Validators: 10, // exceeds max of 4
		},
	}

	result := v.Validate(devnet)

	if result.Valid {
		t.Error("Validate() should fail for validators count of 10")
	}
	foundError := false
	for _, err := range result.Errors {
		if err.Field == "spec.validators" && strings.Contains(err.Message, "between 1 and 4") {
			foundError = true
			break
		}
	}
	if !foundError {
		t.Errorf("Validate() should contain error about validators between 1 and 4, got: %v", result.Errors)
	}
}

func TestYAMLValidator_Validate_InvalidMode(t *testing.T) {
	v := NewYAMLValidator()
	devnet := &YAMLDevnet{
		APIVersion: SupportedAPIVersion,
		Kind:       SupportedKind,
		Metadata:   YAMLMetadata{Name: "test"},
		Spec: YAMLDevnetSpec{
			Network:    "stable",
			Validators: 2,
			Mode:       "kubernetes", // invalid mode
		},
	}

	result := v.Validate(devnet)

	if result.Valid {
		t.Error("Validate() should fail for invalid mode 'kubernetes'")
	}
	foundError := false
	for _, err := range result.Errors {
		if err.Field == "spec.mode" && strings.Contains(err.Message, "'docker' or 'local'") {
			foundError = true
			break
		}
	}
	if !foundError {
		t.Errorf("Validate() should contain error about mode being 'docker' or 'local', got: %v", result.Errors)
	}
}

func TestYAMLValidator_Validate_InvalidAPIVersion(t *testing.T) {
	v := NewYAMLValidator()
	devnet := &YAMLDevnet{
		APIVersion: "invalid/v1",
		Kind:       SupportedKind,
		Metadata:   YAMLMetadata{Name: "test"},
		Spec: YAMLDevnetSpec{
			Network:    "stable",
			Validators: 2,
		},
	}

	result := v.Validate(devnet)

	if result.Valid {
		t.Error("Validate() should fail for invalid apiVersion")
	}
	foundError := false
	for _, err := range result.Errors {
		if err.Field == "apiVersion" && strings.Contains(err.Message, "unsupported") {
			foundError = true
			break
		}
	}
	if !foundError {
		t.Errorf("Validate() should contain error about unsupported apiVersion, got: %v", result.Errors)
	}
}

func TestYAMLValidator_Validate_InvalidKind(t *testing.T) {
	v := NewYAMLValidator()
	devnet := &YAMLDevnet{
		APIVersion: SupportedAPIVersion,
		Kind:       "Invalid",
		Metadata:   YAMLMetadata{Name: "test"},
		Spec: YAMLDevnetSpec{
			Network:    "stable",
			Validators: 2,
		},
	}

	result := v.Validate(devnet)

	if result.Valid {
		t.Error("Validate() should fail for invalid kind")
	}
	foundError := false
	for _, err := range result.Errors {
		if err.Field == "kind" && strings.Contains(err.Message, "unsupported") {
			foundError = true
			break
		}
	}
	if !foundError {
		t.Errorf("Validate() should contain error about unsupported kind, got: %v", result.Errors)
	}
}

func TestYAMLValidator_Validate_MissingNetwork(t *testing.T) {
	v := NewYAMLValidator()
	devnet := &YAMLDevnet{
		APIVersion: SupportedAPIVersion,
		Kind:       SupportedKind,
		Metadata:   YAMLMetadata{Name: "test"},
		Spec: YAMLDevnetSpec{
			Network:    "",
			Validators: 2,
		},
	}

	result := v.Validate(devnet)

	if result.Valid {
		t.Error("Validate() should fail for missing network")
	}
	foundError := false
	for _, err := range result.Errors {
		if err.Field == "spec.network" && strings.Contains(err.Message, "required") {
			foundError = true
			break
		}
	}
	if !foundError {
		t.Errorf("Validate() should contain error about spec.network required, got: %v", result.Errors)
	}
}

func TestYAMLValidator_Validate_InvalidNetworkType(t *testing.T) {
	v := NewYAMLValidator()
	devnet := &YAMLDevnet{
		APIVersion: SupportedAPIVersion,
		Kind:       SupportedKind,
		Metadata:   YAMLMetadata{Name: "test"},
		Spec: YAMLDevnetSpec{
			Network:     "stable",
			Validators:  2,
			NetworkType: "invalid",
		},
	}

	result := v.Validate(devnet)

	if result.Valid {
		t.Error("Validate() should fail for invalid networkType")
	}
	foundError := false
	for _, err := range result.Errors {
		if err.Field == "spec.networkType" && strings.Contains(err.Message, "'mainnet' or 'testnet'") {
			foundError = true
			break
		}
	}
	if !foundError {
		t.Errorf("Validate() should contain error about networkType, got: %v", result.Errors)
	}
}

func TestYAMLValidator_Validate_InvalidFullNodesCount(t *testing.T) {
	v := NewYAMLValidator()
	devnet := &YAMLDevnet{
		APIVersion: SupportedAPIVersion,
		Kind:       SupportedKind,
		Metadata:   YAMLMetadata{Name: "test"},
		Spec: YAMLDevnetSpec{
			Network:    "stable",
			Validators: 2,
			FullNodes:  15, // exceeds max of 10
		},
	}

	result := v.Validate(devnet)

	if result.Valid {
		t.Error("Validate() should fail for fullNodes count of 15")
	}
	foundError := false
	for _, err := range result.Errors {
		if err.Field == "spec.fullNodes" && strings.Contains(err.Message, "between 0 and 10") {
			foundError = true
			break
		}
	}
	if !foundError {
		t.Errorf("Validate() should contain error about fullNodes, got: %v", result.Errors)
	}
}

func TestYAMLValidator_ValidateInvariants_Valid(t *testing.T) {
	v := NewYAMLValidator()
	devnet := &YAMLDevnet{
		APIVersion: SupportedAPIVersion,
		Kind:       SupportedKind,
		Metadata:   YAMLMetadata{Name: "test"},
		Spec: YAMLDevnetSpec{
			Network:    "stable",
			Validators: 2,
		},
	}

	err := v.ValidateInvariants(devnet)

	if err != nil {
		t.Errorf("ValidateInvariants() should return nil for valid config, got: %v", err)
	}
}

func TestYAMLValidator_ValidateInvariants_Invalid(t *testing.T) {
	v := NewYAMLValidator()
	devnet := &YAMLDevnet{
		APIVersion: "invalid/v1",
		Kind:       SupportedKind,
		Metadata:   YAMLMetadata{Name: ""},
		Spec: YAMLDevnetSpec{
			Network:    "",
			Validators: 10,
		},
	}

	err := v.ValidateInvariants(devnet)

	if err == nil {
		t.Error("ValidateInvariants() should return error for invalid config")
	}
}

func TestYAMLValidator_Validate_ZeroValidators(t *testing.T) {
	v := NewYAMLValidator()
	devnet := &YAMLDevnet{
		APIVersion: SupportedAPIVersion,
		Kind:       SupportedKind,
		Metadata:   YAMLMetadata{Name: "test"},
		Spec: YAMLDevnetSpec{
			Network:    "stable",
			Validators: 0, // below min of 1
		},
	}

	result := v.Validate(devnet)

	if result.Valid {
		t.Error("Validate() should fail for zero validators")
	}
	foundError := false
	for _, err := range result.Errors {
		if err.Field == "spec.validators" && strings.Contains(err.Message, "between 1 and 4") {
			foundError = true
			break
		}
	}
	if !foundError {
		t.Errorf("Validate() should contain error about validators, got: %v", result.Errors)
	}
}
