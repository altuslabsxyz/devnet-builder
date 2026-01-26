package types

import (
	"testing"
	"time"
)

func TestSetCondition(t *testing.T) {
	var conditions []Condition

	// Add first condition
	conditions = SetCondition(conditions, ConditionTypeReady, ConditionFalse, ReasonNodesNotReady, "0/4 nodes ready")

	if len(conditions) != 1 {
		t.Fatalf("expected 1 condition, got %d", len(conditions))
	}
	if conditions[0].Type != ConditionTypeReady {
		t.Errorf("expected type Ready, got %s", conditions[0].Type)
	}
	if conditions[0].Status != ConditionFalse {
		t.Errorf("expected status False, got %s", conditions[0].Status)
	}

	// Update same condition
	time.Sleep(10 * time.Millisecond) // ensure different timestamp
	conditions = SetCondition(conditions, ConditionTypeReady, ConditionTrue, ReasonAllNodesReady, "4/4 nodes ready")

	if len(conditions) != 1 {
		t.Fatalf("expected 1 condition after update, got %d", len(conditions))
	}
	if conditions[0].Status != ConditionTrue {
		t.Errorf("expected status True after update, got %s", conditions[0].Status)
	}

	// Add different condition
	conditions = SetCondition(conditions, ConditionTypeProgressing, ConditionFalse, "", "")

	if len(conditions) != 2 {
		t.Fatalf("expected 2 conditions, got %d", len(conditions))
	}
}

func TestGetCondition(t *testing.T) {
	conditions := []Condition{
		{Type: ConditionTypeReady, Status: ConditionTrue},
		{Type: ConditionTypeProgressing, Status: ConditionFalse},
	}

	c := GetCondition(conditions, ConditionTypeReady)
	if c == nil {
		t.Fatal("expected to find Ready condition")
	}
	if c.Status != ConditionTrue {
		t.Errorf("expected True, got %s", c.Status)
	}

	c = GetCondition(conditions, ConditionTypeDegraded)
	if c != nil {
		t.Error("expected nil for non-existent condition")
	}
}

func TestIsConditionTrue(t *testing.T) {
	conditions := []Condition{
		{Type: ConditionTypeReady, Status: ConditionTrue},
		{Type: ConditionTypeProgressing, Status: ConditionFalse},
	}

	if !IsConditionTrue(conditions, ConditionTypeReady) {
		t.Error("Ready should be true")
	}
	if IsConditionTrue(conditions, ConditionTypeProgressing) {
		t.Error("Progressing should not be true")
	}
	if IsConditionTrue(conditions, ConditionTypeDegraded) {
		t.Error("missing condition should not be true")
	}
}
