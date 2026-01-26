// internal/daemon/types/event.go
package types

import (
	"sync"
	"time"
)

// Event represents a significant occurrence during resource lifecycle.
type Event struct {
	Timestamp time.Time `json:"timestamp"`
	Type      string    `json:"type"`      // Normal, Warning
	Reason    string    `json:"reason"`    // CamelCase reason code
	Message   string    `json:"message"`   // Human-readable message
	Component string    `json:"component"` // Source component
}

// NewEvent creates a new event with the current timestamp.
func NewEvent(eventType, reason, message, component string) Event {
	return Event{
		Timestamp: time.Now(),
		Type:      eventType,
		Reason:    reason,
		Message:   message,
		Component: component,
	}
}

// EventRing is a fixed-size ring buffer for events.
// It keeps the most recent N events.
type EventRing struct {
	events []Event
	size   int
	mu     sync.RWMutex
}

// NewEventRing creates a new event ring with the given capacity.
func NewEventRing(capacity int) *EventRing {
	return &EventRing{
		events: make([]Event, 0, capacity),
		size:   capacity,
	}
}

// Add adds an event to the ring, evicting the oldest if at capacity.
func (r *EventRing) Add(e Event) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.events) >= r.size {
		// Shift left to evict oldest
		copy(r.events, r.events[1:])
		r.events = r.events[:len(r.events)-1]
	}
	r.events = append(r.events, e)
}

// List returns a copy of all events in chronological order.
func (r *EventRing) List() []Event {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]Event, len(r.events))
	copy(result, r.events)
	return result
}

// Clear removes all events from the ring.
func (r *EventRing) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = r.events[:0]
}
