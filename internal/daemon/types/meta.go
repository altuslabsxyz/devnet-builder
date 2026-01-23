// internal/daemon/types/meta.go
package types

import "time"

// ResourceMeta contains metadata common to all resources.
type ResourceMeta struct {
	// Name is the unique identifier for this resource.
	Name string `json:"name"`

	// Generation is incremented each time the resource is updated.
	// Used for optimistic concurrency control.
	Generation int64 `json:"generation"`

	// CreatedAt is when the resource was created.
	CreatedAt time.Time `json:"createdAt"`

	// UpdatedAt is when the resource was last modified.
	UpdatedAt time.Time `json:"updatedAt"`

	// Labels are key-value pairs for organizing resources.
	Labels map[string]string `json:"labels,omitempty"`

	// Annotations are key-value pairs for storing arbitrary metadata.
	Annotations map[string]string `json:"annotations,omitempty"`
}

// Condition represents a condition of a resource.
type Condition struct {
	Type               string    `json:"type"`
	Status             string    `json:"status"` // "True", "False", "Unknown"
	LastTransitionTime time.Time `json:"lastTransitionTime"`
	Reason             string    `json:"reason,omitempty"`
	Message            string    `json:"message,omitempty"`
}
