// internal/config/yaml_validator.go
package config

import (
	"fmt"
	"strings"
)

// ValidationError represents a single validation error
type ValidationError struct {
	Field   string
	Message string
}

// Error implements the error interface for ValidationError
func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidationResult contains the result of validation
type ValidationResult struct {
	Valid    bool
	Errors   []ValidationError
	Warnings []ValidationError
}

// Error returns a formatted string of all validation errors
func (r *ValidationResult) Error() string {
	if r.Valid || len(r.Errors) == 0 {
		return ""
	}
	var errStrs []string
	for _, e := range r.Errors {
		errStrs = append(errStrs, e.Error())
	}
	return strings.Join(errStrs, "; ")
}

// YAMLValidator validates YAMLDevnet configurations
type YAMLValidator struct {
	MaxValidators int
	MinValidators int
	MaxFullNodes  int
	ValidModes    []string
}

// NewYAMLValidator creates a new validator with default settings
func NewYAMLValidator() *YAMLValidator {
	return &YAMLValidator{
		MaxValidators: 4,
		MinValidators: 1,
		MaxFullNodes:  10,
		ValidModes:    []string{"docker", "local"},
	}
}

// Validate validates a YAMLDevnet and returns a ValidationResult
func (v *YAMLValidator) Validate(devnet *YAMLDevnet) *ValidationResult {
	if devnet == nil {
		return &ValidationResult{
			Valid:  false,
			Errors: []ValidationError{{Field: "devnet", Message: "cannot be nil"}},
		}
	}

	result := &ValidationResult{
		Valid:  true,
		Errors: []ValidationError{},
	}

	// Validate apiVersion
	if devnet.APIVersion != SupportedAPIVersion {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "apiVersion",
			Message: fmt.Sprintf("unsupported apiVersion %q, expected %q", devnet.APIVersion, SupportedAPIVersion),
		})
	}

	// Validate kind
	if devnet.Kind != SupportedKind {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "kind",
			Message: fmt.Sprintf("unsupported kind %q, expected %q", devnet.Kind, SupportedKind),
		})
	}

	// Validate metadata.name
	if devnet.Metadata.Name == "" {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "metadata.name",
			Message: "is required",
		})
	}

	// Validate spec.network
	if devnet.Spec.Network == "" {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "spec.network",
			Message: "is required",
		})
	}

	// Validate spec.validators
	if devnet.Spec.Validators < v.MinValidators || devnet.Spec.Validators > v.MaxValidators {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "spec.validators",
			Message: fmt.Sprintf("must be between %d and %d, got %d", v.MinValidators, v.MaxValidators, devnet.Spec.Validators),
		})
	}

	// Validate spec.fullNodes
	if devnet.Spec.FullNodes < 0 || devnet.Spec.FullNodes > v.MaxFullNodes {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Field:   "spec.fullNodes",
			Message: fmt.Sprintf("must be between 0 and %d, got %d", v.MaxFullNodes, devnet.Spec.FullNodes),
		})
	}

	// Validate spec.mode if provided
	if devnet.Spec.Mode != "" {
		validMode := false
		for _, mode := range v.ValidModes {
			if devnet.Spec.Mode == mode {
				validMode = true
				break
			}
		}
		if !validMode {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Field:   "spec.mode",
				Message: fmt.Sprintf("must be 'docker' or 'local', got %q", devnet.Spec.Mode),
			})
		}
	}

	// Validate spec.networkType if provided
	if devnet.Spec.NetworkType != "" {
		if devnet.Spec.NetworkType != "mainnet" && devnet.Spec.NetworkType != "testnet" {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Field:   "spec.networkType",
				Message: fmt.Sprintf("must be 'mainnet' or 'testnet', got %q", devnet.Spec.NetworkType),
			})
		}
	}

	return result
}

// ValidateInvariants performs validation and returns a simple error
func (v *YAMLValidator) ValidateInvariants(devnet *YAMLDevnet) error {
	result := v.Validate(devnet)
	if !result.Valid {
		return fmt.Errorf("validation errors: %s", result.Error())
	}
	return nil
}

// ValidateWithSource validates and includes source file in errors.
func (v *YAMLValidator) ValidateWithSource(devnet *YAMLDevnet, source string) *ValidationResult {
	result := v.Validate(devnet)

	// Prepend source to field paths for context
	for i := range result.Errors {
		if result.Errors[i].Field != "" {
			result.Errors[i].Field = fmt.Sprintf("%s: %s", source, result.Errors[i].Field)
		}
	}

	return result
}

// FormatValidationErrors returns a kubectl-style error message.
func FormatValidationErrors(result *ValidationResult, filename string) string {
	if result.Valid {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("error: error validating %q:\n", filename))

	for _, err := range result.Errors {
		sb.WriteString(fmt.Sprintf("  - %s\n", err.Error()))
	}

	if len(result.Warnings) > 0 {
		sb.WriteString("\nwarnings:\n")
		for _, w := range result.Warnings {
			sb.WriteString(fmt.Sprintf("  - %s\n", w.Error()))
		}
	}

	return sb.String()
}
