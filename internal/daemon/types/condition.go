// internal/daemon/types/condition.go
package types

import "time"

// SetCondition adds or updates a condition in the slice.
// If the condition already exists and the status changed, the transition time is updated.
// Returns the updated slice.
func SetCondition(conditions []Condition, condType, status, reason, message string) []Condition {
	now := time.Now()

	for i, c := range conditions {
		if c.Type == condType {
			// Only update transition time if status changed
			if c.Status != status {
				conditions[i].LastTransitionTime = now
			}
			conditions[i].Status = status
			conditions[i].Reason = reason
			conditions[i].Message = message
			return conditions
		}
	}

	// Condition not found, add new
	return append(conditions, Condition{
		Type:               condType,
		Status:             status,
		LastTransitionTime: now,
		Reason:             reason,
		Message:            message,
	})
}

// GetCondition returns the condition with the given type, or nil if not found.
func GetCondition(conditions []Condition, condType string) *Condition {
	for i := range conditions {
		if conditions[i].Type == condType {
			return &conditions[i]
		}
	}
	return nil
}

// IsConditionTrue returns true if the condition exists and has status True.
func IsConditionTrue(conditions []Condition, condType string) bool {
	c := GetCondition(conditions, condType)
	return c != nil && c.Status == ConditionTrue
}

// IsConditionFalse returns true if the condition exists and has status False.
func IsConditionFalse(conditions []Condition, condType string) bool {
	c := GetCondition(conditions, condType)
	return c != nil && c.Status == ConditionFalse
}

// RemoveCondition removes a condition by type. Returns the updated slice.
func RemoveCondition(conditions []Condition, condType string) []Condition {
	result := make([]Condition, 0, len(conditions))
	for _, c := range conditions {
		if c.Type != condType {
			result = append(result, c)
		}
	}
	return result
}
