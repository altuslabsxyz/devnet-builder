// internal/daemon/server/ante/reference_validator.go
package ante

import (
	"context"
	"fmt"

	v1 "github.com/altuslabsxyz/devnet-builder/api/proto/gen/v1"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/types"
)

// Store provides access to persisted resources for validation.
type Store interface {
	GetDevnet(ctx context.Context, namespace, name string) (*types.Devnet, error)
	GetNode(ctx context.Context, namespace, devnetName string, index int) (*types.Node, error)
}

// NetworkService provides network plugin information for validation.
type NetworkService interface {
	GetNetworkInfo(ctx context.Context, req *v1.GetNetworkInfoRequest) (*v1.GetNetworkInfoResponse, error)
	ListBinaryVersions(ctx context.Context, req *v1.ListBinaryVersionsRequest) (*v1.ListBinaryVersionsResponse, error)
}

// ReferenceValidator validates cross-resource references and semantic constraints.
type ReferenceValidator interface {
	ValidateDevnetReferences(ctx context.Context, namespace string, spec *v1.DevnetSpec) error
	ValidateUpgradeReferences(ctx context.Context, namespace string, spec *v1.UpgradeSpec) error
	ValidateNodeReferences(ctx context.Context, namespace, devnetName string, index int) error
}

type referenceValidator struct {
	store      Store
	networkSvc NetworkService
}

// NewReferenceValidator creates a new reference validator with the given dependencies.
func NewReferenceValidator(store Store, networkSvc NetworkService) ReferenceValidator {
	return &referenceValidator{
		store:      store,
		networkSvc: networkSvc,
	}
}

// ValidateDevnetReferences validates that a DevnetSpec references valid resources.
// It checks that the plugin (network) exists via the NetworkService.
func (v *referenceValidator) ValidateDevnetReferences(ctx context.Context, namespace string, spec *v1.DevnetSpec) error {
	if spec == nil {
		return nil
	}

	var errs []*ValidationError

	// Validate plugin/network exists
	if spec.Plugin != "" {
		_, err := v.networkSvc.GetNetworkInfo(ctx, &v1.GetNetworkInfoRequest{Name: spec.Plugin})
		if err != nil {
			errs = append(errs, &ValidationError{
				Field:   "spec.plugin",
				Code:    CodeNotFound,
				Message: fmt.Sprintf("network plugin '%s' not found", spec.Plugin),
			})
		}
	}

	return toError(errs)
}

// ValidateUpgradeReferences validates that an UpgradeSpec references valid resources.
// It checks that the referenced devnet exists in the store.
func (v *referenceValidator) ValidateUpgradeReferences(ctx context.Context, namespace string, spec *v1.UpgradeSpec) error {
	if spec == nil {
		return nil
	}

	var errs []*ValidationError

	// Validate devnet exists
	if spec.DevnetRef != "" {
		_, err := v.store.GetDevnet(ctx, namespace, spec.DevnetRef)
		if err != nil {
			errs = append(errs, &ValidationError{
				Field:   "spec.devnet_ref",
				Code:    CodeNotFound,
				Message: fmt.Sprintf("devnet '%s' not found", spec.DevnetRef),
			})
		}
	}

	return toError(errs)
}

// ValidateNodeReferences validates that a node operation references valid resources.
// It checks that the referenced devnet exists in the store.
func (v *referenceValidator) ValidateNodeReferences(ctx context.Context, namespace, devnetName string, index int) error {
	var errs []*ValidationError

	// Validate devnet exists
	if devnetName != "" {
		_, err := v.store.GetDevnet(ctx, namespace, devnetName)
		if err != nil {
			errs = append(errs, &ValidationError{
				Field:   "devnet_name",
				Code:    CodeNotFound,
				Message: fmt.Sprintf("devnet '%s' not found", devnetName),
			})
		}
	}

	return toError(errs)
}
