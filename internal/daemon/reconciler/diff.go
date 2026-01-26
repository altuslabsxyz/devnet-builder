package reconciler

import (
	"fmt"
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/config"
)

// DiffCalculator computes differences between states
type DiffCalculator struct{}

// NewDiffCalculator creates a new diff calculator
func NewDiffCalculator() *DiffCalculator {
	return &DiffCalculator{}
}

// Calculate computes the reconciliation plan from current to desired state
func (d *DiffCalculator) Calculate(current, desired *State) *Plan {
	plan := &Plan{
		CreatedAt: time.Now(),
	}

	// Check for creates and updates
	for name := range desired.Devnets {
		desiredDevnet := desired.Devnets[name]
		currentDevnet, exists := current.Devnets[name]
		if !exists {
			// New devnet
			plan.Actions = append(plan.Actions, Action{
				Type:        ActionCreate,
				Resource:    "devnet",
				Name:        name,
				Description: fmt.Sprintf("Create devnet %s with %d validators", name, desiredDevnet.Spec.Validators),
				Details: map[string]interface{}{
					"network":    desiredDevnet.Spec.Network,
					"validators": desiredDevnet.Spec.Validators,
					"mode":       desiredDevnet.Spec.Mode,
				},
			})
			plan.DevnetName = name
			continue
		}

		// Check for changes
		actions := d.compareDevnets(currentDevnet, desiredDevnet)
		plan.Actions = append(plan.Actions, actions...)
		if len(actions) > 0 {
			plan.DevnetName = name
		}
	}

	// Check for deletes
	for name := range current.Devnets {
		if _, exists := desired.Devnets[name]; !exists {
			plan.Actions = append(plan.Actions, Action{
				Type:        ActionDelete,
				Resource:    "devnet",
				Name:        name,
				Description: fmt.Sprintf("Delete devnet %s", name),
			})
		}
	}

	return plan
}

func (d *DiffCalculator) compareDevnets(current, desired config.YAMLDevnet) []Action {
	var actions []Action

	// Check validator count
	if current.Spec.Validators != desired.Spec.Validators {
		actions = append(actions, Action{
			Type:        ActionScale,
			Resource:    "devnet",
			Name:        desired.Metadata.Name,
			Description: fmt.Sprintf("Scale validators from %d to %d", current.Spec.Validators, desired.Spec.Validators),
			Details: map[string]interface{}{
				"from": current.Spec.Validators,
				"to":   desired.Spec.Validators,
			},
		})
	}

	// Check version
	if current.Spec.NetworkVersion != desired.Spec.NetworkVersion && desired.Spec.NetworkVersion != "" {
		actions = append(actions, Action{
			Type:        ActionUpgrade,
			Resource:    "devnet",
			Name:        desired.Metadata.Name,
			Description: fmt.Sprintf("Upgrade from %s to %s", current.Spec.NetworkVersion, desired.Spec.NetworkVersion),
			Details: map[string]interface{}{
				"from": current.Spec.NetworkVersion,
				"to":   desired.Spec.NetworkVersion,
			},
		})
	}

	// Check mode change (requires recreate)
	if current.Spec.Mode != desired.Spec.Mode && desired.Spec.Mode != "" && current.Spec.Mode != "" {
		actions = append(actions, Action{
			Type:        ActionUpdate,
			Resource:    "devnet",
			Name:        desired.Metadata.Name,
			Description: fmt.Sprintf("Change mode from %s to %s (requires recreate)", current.Spec.Mode, desired.Spec.Mode),
			Details: map[string]interface{}{
				"from":             current.Spec.Mode,
				"to":               desired.Spec.Mode,
				"requiresRecreate": true,
			},
		})
	}

	return actions
}
