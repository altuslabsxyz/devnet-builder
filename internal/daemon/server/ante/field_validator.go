// internal/daemon/server/ante/field_validator.go
package ante

import (
	"context"

	v1 "github.com/altuslabsxyz/devnet-builder/api/proto/gen/v1"
)

// FieldValidator validates required fields are present.
type FieldValidator interface {
	ValidateCreateDevnetRequest(ctx context.Context, req *v1.CreateDevnetRequest) error
	ValidateApplyDevnetRequest(ctx context.Context, req *v1.ApplyDevnetRequest) error
	ValidateUpdateDevnetRequest(ctx context.Context, req *v1.UpdateDevnetRequest) error
	ValidateCreateUpgradeRequest(ctx context.Context, req *v1.CreateUpgradeRequest) error
	ValidateStartNodeRequest(ctx context.Context, req *v1.StartNodeRequest) error
	ValidateStopNodeRequest(ctx context.Context, req *v1.StopNodeRequest) error
	ValidateRestartNodeRequest(ctx context.Context, req *v1.RestartNodeRequest) error
	ValidateGetNodeRequest(ctx context.Context, req *v1.GetNodeRequest) error
	ValidateGetNodeHealthRequest(ctx context.Context, req *v1.GetNodeHealthRequest) error
}

type fieldValidator struct{}

// NewFieldValidator creates a new FieldValidator instance.
func NewFieldValidator() FieldValidator {
	return &fieldValidator{}
}

// ValidateCreateDevnetRequest validates required fields for creating a devnet.
func (v *fieldValidator) ValidateCreateDevnetRequest(ctx context.Context, req *v1.CreateDevnetRequest) error {
	var errs []*ValidationError

	if req.Name == "" {
		errs = append(errs, &ValidationError{Field: "name", Code: CodeRequired, Message: "name is required"})
	}

	if req.Spec == nil {
		errs = append(errs, &ValidationError{Field: "spec", Code: CodeRequired, Message: "spec is required"})
	} else {
		if req.Spec.Plugin == "" {
			errs = append(errs, &ValidationError{Field: "spec.plugin", Code: CodeRequired, Message: "plugin is required"})
		}
		if req.Spec.Mode == "" {
			errs = append(errs, &ValidationError{Field: "spec.mode", Code: CodeRequired, Message: "mode is required"})
		}
	}

	return toError(errs)
}

// ValidateApplyDevnetRequest validates required fields for applying a devnet.
func (v *fieldValidator) ValidateApplyDevnetRequest(ctx context.Context, req *v1.ApplyDevnetRequest) error {
	var errs []*ValidationError

	if req.Name == "" {
		errs = append(errs, &ValidationError{Field: "name", Code: CodeRequired, Message: "name is required"})
	}

	if req.Spec == nil {
		errs = append(errs, &ValidationError{Field: "spec", Code: CodeRequired, Message: "spec is required"})
	} else {
		if req.Spec.Plugin == "" {
			errs = append(errs, &ValidationError{Field: "spec.plugin", Code: CodeRequired, Message: "plugin is required"})
		}
		if req.Spec.Mode == "" {
			errs = append(errs, &ValidationError{Field: "spec.mode", Code: CodeRequired, Message: "mode is required"})
		}
	}

	return toError(errs)
}

// ValidateUpdateDevnetRequest validates required fields for updating a devnet.
// Note: UpdateDevnetRequest supports partial updates, so spec is not required.
func (v *fieldValidator) ValidateUpdateDevnetRequest(ctx context.Context, req *v1.UpdateDevnetRequest) error {
	var errs []*ValidationError

	if req.Name == "" {
		errs = append(errs, &ValidationError{Field: "name", Code: CodeRequired, Message: "name is required"})
	}

	return toError(errs)
}

// ValidateCreateUpgradeRequest validates required fields for creating an upgrade.
func (v *fieldValidator) ValidateCreateUpgradeRequest(ctx context.Context, req *v1.CreateUpgradeRequest) error {
	var errs []*ValidationError

	if req.Name == "" {
		errs = append(errs, &ValidationError{Field: "name", Code: CodeRequired, Message: "name is required"})
	}

	if req.Spec == nil {
		errs = append(errs, &ValidationError{Field: "spec", Code: CodeRequired, Message: "spec is required"})
	} else {
		if req.Spec.DevnetRef == "" {
			errs = append(errs, &ValidationError{Field: "spec.devnet_ref", Code: CodeRequired, Message: "devnet_ref is required"})
		}
		if req.Spec.UpgradeName == "" {
			errs = append(errs, &ValidationError{Field: "spec.upgrade_name", Code: CodeRequired, Message: "upgrade_name is required"})
		}
	}

	return toError(errs)
}

// ValidateStartNodeRequest validates required fields for starting a node.
func (v *fieldValidator) ValidateStartNodeRequest(ctx context.Context, req *v1.StartNodeRequest) error {
	var errs []*ValidationError

	if req.DevnetName == "" {
		errs = append(errs, &ValidationError{Field: "devnet_name", Code: CodeRequired, Message: "devnet_name is required"})
	}

	if req.Index < 0 {
		errs = append(errs, &ValidationError{Field: "index", Code: CodeInvalidRange, Message: "index must be non-negative"})
	}

	return toError(errs)
}

// ValidateStopNodeRequest validates required fields for stopping a node.
func (v *fieldValidator) ValidateStopNodeRequest(ctx context.Context, req *v1.StopNodeRequest) error {
	var errs []*ValidationError

	if req.DevnetName == "" {
		errs = append(errs, &ValidationError{Field: "devnet_name", Code: CodeRequired, Message: "devnet_name is required"})
	}

	if req.Index < 0 {
		errs = append(errs, &ValidationError{Field: "index", Code: CodeInvalidRange, Message: "index must be non-negative"})
	}

	return toError(errs)
}

// ValidateRestartNodeRequest validates required fields for restarting a node.
func (v *fieldValidator) ValidateRestartNodeRequest(ctx context.Context, req *v1.RestartNodeRequest) error {
	var errs []*ValidationError

	if req.DevnetName == "" {
		errs = append(errs, &ValidationError{Field: "devnet_name", Code: CodeRequired, Message: "devnet_name is required"})
	}

	if req.Index < 0 {
		errs = append(errs, &ValidationError{Field: "index", Code: CodeInvalidRange, Message: "index must be non-negative"})
	}

	return toError(errs)
}

// ValidateGetNodeRequest validates required fields for getting a node.
func (v *fieldValidator) ValidateGetNodeRequest(ctx context.Context, req *v1.GetNodeRequest) error {
	var errs []*ValidationError

	if req.DevnetName == "" {
		errs = append(errs, &ValidationError{Field: "devnet_name", Code: CodeRequired, Message: "devnet_name is required"})
	}

	if req.Index < 0 {
		errs = append(errs, &ValidationError{Field: "index", Code: CodeInvalidRange, Message: "index must be non-negative"})
	}

	return toError(errs)
}

// ValidateGetNodeHealthRequest validates required fields for getting node health.
func (v *fieldValidator) ValidateGetNodeHealthRequest(ctx context.Context, req *v1.GetNodeHealthRequest) error {
	var errs []*ValidationError

	if req.DevnetName == "" {
		errs = append(errs, &ValidationError{Field: "devnet_name", Code: CodeRequired, Message: "devnet_name is required"})
	}

	if req.Index < 0 {
		errs = append(errs, &ValidationError{Field: "index", Code: CodeInvalidRange, Message: "index must be non-negative"})
	}

	return toError(errs)
}
