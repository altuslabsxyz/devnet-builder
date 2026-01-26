// internal/daemon/reconciler/types_test.go
package reconciler

import (
	"testing"
)

func TestAction_String(t *testing.T) {
	tests := []struct {
		action ActionType
		want   string
	}{
		{ActionCreate, "CREATE"},
		{ActionUpdate, "UPDATE"},
		{ActionDelete, "DELETE"},
		{ActionScale, "SCALE"},
		{ActionUpgrade, "UPGRADE"},
	}

	for _, tt := range tests {
		if got := tt.action.String(); got != tt.want {
			t.Errorf("ActionType.String() = %v, want %v", got, tt.want)
		}
	}
}

func TestPlan_IsEmpty(t *testing.T) {
	empty := &Plan{}
	if !empty.IsEmpty() {
		t.Error("empty plan should return true for IsEmpty()")
	}

	withActions := &Plan{
		Actions: []Action{{Type: ActionCreate}},
	}
	if withActions.IsEmpty() {
		t.Error("plan with actions should return false for IsEmpty()")
	}
}

func TestPlan_Summary(t *testing.T) {
	plan := &Plan{
		Actions: []Action{
			{Type: ActionCreate},
			{Type: ActionCreate},
			{Type: ActionUpdate},
			{Type: ActionDelete},
			{Type: ActionScale},
			{Type: ActionUpgrade},
		},
	}

	creates, updates, deletes := plan.Summary()

	if creates != 2 {
		t.Errorf("expected 2 creates, got %d", creates)
	}
	if updates != 3 { // Update + Scale + Upgrade
		t.Errorf("expected 3 updates, got %d", updates)
	}
	if deletes != 1 {
		t.Errorf("expected 1 delete, got %d", deletes)
	}
}

func TestNewState(t *testing.T) {
	state := NewState()
	if state == nil {
		t.Fatal("NewState() returned nil")
	}
	if state.Devnets == nil {
		t.Error("NewState() should initialize Devnets map")
	}
}
