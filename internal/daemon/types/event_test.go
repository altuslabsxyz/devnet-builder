package types

import (
	"testing"
)

func TestNewEvent(t *testing.T) {
	e := NewEvent(EventTypeNormal, ReasonAllNodesReady, "All 4 nodes are running", "devnet-controller")

	if e.Type != EventTypeNormal {
		t.Errorf("expected type %s, got %s", EventTypeNormal, e.Type)
	}
	if e.Reason != ReasonAllNodesReady {
		t.Errorf("expected reason %s, got %s", ReasonAllNodesReady, e.Reason)
	}
	if e.Message != "All 4 nodes are running" {
		t.Errorf("unexpected message: %s", e.Message)
	}
	if e.Component != "devnet-controller" {
		t.Errorf("expected component devnet-controller, got %s", e.Component)
	}
	if e.Timestamp.IsZero() {
		t.Error("timestamp should be set")
	}
}

func TestEventRing(t *testing.T) {
	ring := NewEventRing(3)

	ring.Add(NewEvent(EventTypeNormal, "R1", "msg1", "c1"))
	ring.Add(NewEvent(EventTypeNormal, "R2", "msg2", "c2"))
	ring.Add(NewEvent(EventTypeNormal, "R3", "msg3", "c3"))
	ring.Add(NewEvent(EventTypeNormal, "R4", "msg4", "c4"))

	events := ring.List()
	if len(events) != 3 {
		t.Errorf("expected 3 events, got %d", len(events))
	}

	// Should have R2, R3, R4 (R1 evicted)
	if events[0].Reason != "R2" {
		t.Errorf("expected R2 first, got %s", events[0].Reason)
	}
}
