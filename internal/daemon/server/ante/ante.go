// internal/daemon/server/ante/ante.go
package ante

import (
	"context"

	v1 "github.com/altuslabsxyz/devnet-builder/api/proto/gen/v1"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/types"
)

// AnteHandler validates requests before they reach service logic.
// It chains three validation layers: field → spec → reference.
type AnteHandler struct {
	field     FieldValidator
	spec      SpecValidator
	reference ReferenceValidator
}

// New creates an AnteHandler with all validation layers.
func New(store Store, networkSvc NetworkService) *AnteHandler {
	return &AnteHandler{
		field:     NewFieldValidator(),
		spec:      NewSpecValidator(),
		reference: NewReferenceValidator(store, networkSvc),
	}
}

// ValidateCreateDevnet validates a CreateDevnetRequest through all layers.
func (h *AnteHandler) ValidateCreateDevnet(ctx context.Context, req *v1.CreateDevnetRequest) error {
	if err := h.field.ValidateCreateDevnetRequest(ctx, req); err != nil {
		return err
	}

	if err := h.spec.ValidateDevnetSpec(ctx, req.Spec); err != nil {
		return err
	}

	namespace := req.Namespace
	if namespace == "" {
		namespace = types.DefaultNamespace
	}
	if err := h.reference.ValidateDevnetReferences(ctx, namespace, req.Spec); err != nil {
		return err
	}

	return nil
}

// ValidateApplyDevnet validates an ApplyDevnetRequest through all layers.
func (h *AnteHandler) ValidateApplyDevnet(ctx context.Context, req *v1.ApplyDevnetRequest) error {
	if err := h.field.ValidateApplyDevnetRequest(ctx, req); err != nil {
		return err
	}

	if err := h.spec.ValidateDevnetSpec(ctx, req.Spec); err != nil {
		return err
	}

	namespace := req.Namespace
	if namespace == "" {
		namespace = types.DefaultNamespace
	}
	if err := h.reference.ValidateDevnetReferences(ctx, namespace, req.Spec); err != nil {
		return err
	}

	return nil
}

// ValidateUpdateDevnet validates an UpdateDevnetRequest through all layers.
func (h *AnteHandler) ValidateUpdateDevnet(ctx context.Context, req *v1.UpdateDevnetRequest) error {
	if err := h.field.ValidateUpdateDevnetRequest(ctx, req); err != nil {
		return err
	}

	if req.Spec != nil {
		if err := h.spec.ValidateDevnetSpec(ctx, req.Spec); err != nil {
			return err
		}
	}

	return nil
}

// ValidateCreateUpgrade validates a CreateUpgradeRequest through all layers.
func (h *AnteHandler) ValidateCreateUpgrade(ctx context.Context, req *v1.CreateUpgradeRequest) error {
	if err := h.field.ValidateCreateUpgradeRequest(ctx, req); err != nil {
		return err
	}

	if err := h.spec.ValidateUpgradeSpec(ctx, req.Spec); err != nil {
		return err
	}

	namespace := req.Namespace
	if namespace == "" {
		namespace = types.DefaultNamespace
	}
	if err := h.reference.ValidateUpgradeReferences(ctx, namespace, req.Spec); err != nil {
		return err
	}

	return nil
}

// ValidateStartNode validates a StartNodeRequest.
func (h *AnteHandler) ValidateStartNode(ctx context.Context, req *v1.StartNodeRequest) error {
	if err := h.field.ValidateStartNodeRequest(ctx, req); err != nil {
		return err
	}

	namespace := req.Namespace
	if namespace == "" {
		namespace = types.DefaultNamespace
	}
	return h.reference.ValidateNodeReferences(ctx, namespace, req.DevnetName, int(req.Index))
}

// ValidateStopNode validates a StopNodeRequest.
func (h *AnteHandler) ValidateStopNode(ctx context.Context, req *v1.StopNodeRequest) error {
	if err := h.field.ValidateStopNodeRequest(ctx, req); err != nil {
		return err
	}

	namespace := req.Namespace
	if namespace == "" {
		namespace = types.DefaultNamespace
	}
	return h.reference.ValidateNodeReferences(ctx, namespace, req.DevnetName, int(req.Index))
}

// ValidateRestartNode validates a RestartNodeRequest.
func (h *AnteHandler) ValidateRestartNode(ctx context.Context, req *v1.RestartNodeRequest) error {
	if err := h.field.ValidateRestartNodeRequest(ctx, req); err != nil {
		return err
	}

	namespace := req.Namespace
	if namespace == "" {
		namespace = types.DefaultNamespace
	}
	return h.reference.ValidateNodeReferences(ctx, namespace, req.DevnetName, int(req.Index))
}

// ValidateGetNode validates a GetNodeRequest.
func (h *AnteHandler) ValidateGetNode(ctx context.Context, req *v1.GetNodeRequest) error {
	return h.field.ValidateGetNodeRequest(ctx, req)
}

// ValidateGetNodeHealth validates a GetNodeHealthRequest.
func (h *AnteHandler) ValidateGetNodeHealth(ctx context.Context, req *v1.GetNodeHealthRequest) error {
	return h.field.ValidateGetNodeHealthRequest(ctx, req)
}
