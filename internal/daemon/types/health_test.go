package types

import (
	"testing"
)

func TestConditionTypes(t *testing.T) {
	// Verify all expected condition types are defined
	expectedTypes := []string{
		ConditionTypeReady,
		ConditionTypeProgressing,
		ConditionTypePluginAvailable,
		ConditionTypeNodesCreated,
		ConditionTypeNodesRunning,
		ConditionTypeDegraded,
	}

	for _, ct := range expectedTypes {
		if ct == "" {
			t.Errorf("condition type should not be empty")
		}
	}
}

func TestConditionStatusValues(t *testing.T) {
	validStatuses := []string{ConditionTrue, ConditionFalse, ConditionUnknown}
	for _, s := range validStatuses {
		if s == "" {
			t.Errorf("condition status should not be empty")
		}
	}
}

func TestEventTypes(t *testing.T) {
	if EventTypeNormal == "" || EventTypeWarning == "" {
		t.Error("event types should not be empty")
	}
}
