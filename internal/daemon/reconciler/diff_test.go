// internal/daemon/reconciler/diff_test.go
package reconciler

import (
	"testing"

	"github.com/altuslabsxyz/devnet-builder/internal/config"
)

func TestDiffCalculator_NewDevnet(t *testing.T) {
	current := NewState()
	desired := NewState()

	desired.Devnets["test"] = config.YAMLDevnet{
		Metadata: config.YAMLMetadata{Name: "test"},
		Spec: config.YAMLDevnetSpec{
			Network:    "stable",
			Validators: 4,
		},
	}

	calc := NewDiffCalculator()
	plan := calc.Calculate(current, desired)

	if plan.IsEmpty() {
		t.Fatal("plan should not be empty for new devnet")
	}

	creates, _, _ := plan.Summary()
	if creates != 1 {
		t.Errorf("expected 1 create action, got %d", creates)
	}

	if plan.Actions[0].Type != ActionCreate {
		t.Errorf("expected CREATE action, got %s", plan.Actions[0].Type)
	}
}

func TestDiffCalculator_DeleteDevnet(t *testing.T) {
	current := NewState()
	desired := NewState()

	current.Devnets["test"] = config.YAMLDevnet{
		Metadata: config.YAMLMetadata{Name: "test"},
		Spec: config.YAMLDevnetSpec{
			Network:    "stable",
			Validators: 4,
		},
	}
	// desired is empty

	calc := NewDiffCalculator()
	plan := calc.Calculate(current, desired)

	_, _, deletes := plan.Summary()
	if deletes != 1 {
		t.Errorf("expected 1 delete action, got %d", deletes)
	}
}

func TestDiffCalculator_ScaleUp(t *testing.T) {
	current := NewState()
	desired := NewState()

	current.Devnets["test"] = config.YAMLDevnet{
		Metadata: config.YAMLMetadata{Name: "test"},
		Spec: config.YAMLDevnetSpec{
			Network:    "stable",
			Validators: 2,
		},
	}

	desired.Devnets["test"] = config.YAMLDevnet{
		Metadata: config.YAMLMetadata{Name: "test"},
		Spec: config.YAMLDevnetSpec{
			Network:    "stable",
			Validators: 4,
		},
	}

	calc := NewDiffCalculator()
	plan := calc.Calculate(current, desired)

	if plan.IsEmpty() {
		t.Fatal("plan should not be empty for scale change")
	}

	if plan.Actions[0].Type != ActionScale {
		t.Errorf("expected SCALE action, got %s", plan.Actions[0].Type)
	}
}

func TestDiffCalculator_ScaleDown(t *testing.T) {
	current := NewState()
	desired := NewState()

	current.Devnets["test"] = config.YAMLDevnet{
		Metadata: config.YAMLMetadata{Name: "test"},
		Spec: config.YAMLDevnetSpec{
			Network:    "stable",
			Validators: 4,
		},
	}

	desired.Devnets["test"] = config.YAMLDevnet{
		Metadata: config.YAMLMetadata{Name: "test"},
		Spec: config.YAMLDevnetSpec{
			Network:    "stable",
			Validators: 2,
		},
	}

	calc := NewDiffCalculator()
	plan := calc.Calculate(current, desired)

	if plan.IsEmpty() {
		t.Fatal("plan should not be empty for scale down")
	}

	if plan.Actions[0].Type != ActionScale {
		t.Errorf("expected SCALE action, got %s", plan.Actions[0].Type)
	}
}

func TestDiffCalculator_VersionUpgrade(t *testing.T) {
	current := NewState()
	desired := NewState()

	current.Devnets["test"] = config.YAMLDevnet{
		Metadata: config.YAMLMetadata{Name: "test"},
		Spec: config.YAMLDevnetSpec{
			Network:        "stable",
			Validators:     4,
			NetworkVersion: "v1.0.0",
		},
	}

	desired.Devnets["test"] = config.YAMLDevnet{
		Metadata: config.YAMLMetadata{Name: "test"},
		Spec: config.YAMLDevnetSpec{
			Network:        "stable",
			Validators:     4,
			NetworkVersion: "v1.2.0",
		},
	}

	calc := NewDiffCalculator()
	plan := calc.Calculate(current, desired)

	if plan.IsEmpty() {
		t.Fatal("plan should not be empty for version upgrade")
	}

	if plan.Actions[0].Type != ActionUpgrade {
		t.Errorf("expected UPGRADE action, got %s", plan.Actions[0].Type)
	}
}

func TestDiffCalculator_NoChanges(t *testing.T) {
	current := NewState()
	desired := NewState()

	devnet := config.YAMLDevnet{
		Metadata: config.YAMLMetadata{Name: "test"},
		Spec: config.YAMLDevnetSpec{
			Network:    "stable",
			Validators: 4,
			Mode:       "docker",
		},
	}

	current.Devnets["test"] = devnet
	desired.Devnets["test"] = devnet

	calc := NewDiffCalculator()
	plan := calc.Calculate(current, desired)

	if !plan.IsEmpty() {
		t.Errorf("plan should be empty when states match, got %d actions", len(plan.Actions))
	}
}

func TestDiffCalculator_ModeChange(t *testing.T) {
	current := NewState()
	desired := NewState()

	current.Devnets["test"] = config.YAMLDevnet{
		Metadata: config.YAMLMetadata{Name: "test"},
		Spec: config.YAMLDevnetSpec{
			Network:    "stable",
			Validators: 4,
			Mode:       "docker",
		},
	}

	desired.Devnets["test"] = config.YAMLDevnet{
		Metadata: config.YAMLMetadata{Name: "test"},
		Spec: config.YAMLDevnetSpec{
			Network:    "stable",
			Validators: 4,
			Mode:       "local",
		},
	}

	calc := NewDiffCalculator()
	plan := calc.Calculate(current, desired)

	if plan.IsEmpty() {
		t.Fatal("plan should not be empty for mode change")
	}

	if plan.Actions[0].Type != ActionUpdate {
		t.Errorf("expected UPDATE action, got %s", plan.Actions[0].Type)
	}

	// Verify requires recreate flag
	if recreate, ok := plan.Actions[0].Details["requiresRecreate"].(bool); !ok || !recreate {
		t.Error("mode change should require recreate")
	}
}
