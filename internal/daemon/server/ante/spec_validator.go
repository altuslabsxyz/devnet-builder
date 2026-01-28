// internal/daemon/server/ante/spec_validator.go
package ante

import (
	"context"

	v1 "github.com/altuslabsxyz/devnet-builder/api/proto/gen/v1"
)

const (
	// MaxValidators is the maximum number of validators allowed per devnet.
	// Limited to 4 to keep local development resources manageable while
	// still allowing realistic consensus testing scenarios.
	MaxValidators = 4

	// MaxFullNodes is the maximum number of full nodes allowed per devnet.
	// Limited to 10 to prevent excessive resource consumption on development
	// machines while allowing meaningful network topology testing.
	MaxFullNodes = 10
)

// SpecValidator validates business rules and constraints.
type SpecValidator interface {
	ValidateDevnetSpec(ctx context.Context, spec *v1.DevnetSpec) error
	ValidateUpgradeSpec(ctx context.Context, spec *v1.UpgradeSpec) error
}

type specValidator struct{}

func NewSpecValidator() SpecValidator {
	return &specValidator{}
}

func (v *specValidator) ValidateDevnetSpec(ctx context.Context, spec *v1.DevnetSpec) error {
	if spec == nil {
		return nil
	}

	var errs []*ValidationError

	// Mode validation
	if spec.Mode != "" && spec.Mode != "docker" && spec.Mode != "local" {
		errs = append(errs, &ValidationError{
			Field:   "spec.mode",
			Code:    CodeInvalidValue,
			Message: "mode must be 'docker' or 'local'",
		})
	}

	// Validators count
	if spec.Validators > MaxValidators {
		errs = append(errs, &ValidationError{
			Field:   "spec.validators",
			Code:    CodeInvalidRange,
			Message: "validators must be between 0 and 4",
		})
	}

	// FullNodes count
	if spec.FullNodes > MaxFullNodes {
		errs = append(errs, &ValidationError{
			Field:   "spec.full_nodes",
			Code:    CodeInvalidRange,
			Message: "full_nodes must be between 0 and 10",
		})
	}

	// NetworkType validation
	if spec.NetworkType != "" && spec.NetworkType != "mainnet" && spec.NetworkType != "testnet" {
		errs = append(errs, &ValidationError{
			Field:   "spec.network_type",
			Code:    CodeInvalidValue,
			Message: "network_type must be 'mainnet' or 'testnet'",
		})
	}

	return toError(errs)
}

func (v *specValidator) ValidateUpgradeSpec(ctx context.Context, spec *v1.UpgradeSpec) error {
	if spec == nil {
		return nil
	}

	var errs []*ValidationError

	if spec.TargetHeight < 0 {
		errs = append(errs, &ValidationError{
			Field:   "spec.target_height",
			Code:    CodeInvalidRange,
			Message: "target_height must be non-negative",
		})
	}

	return toError(errs)
}
