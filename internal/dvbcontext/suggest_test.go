package dvbcontext

import (
	"testing"
)

func TestSuggestUsage_NilClient(t *testing.T) {
	result := SuggestUsage(nil)
	if result != "" {
		t.Errorf("SuggestUsage(nil) = %q, want empty string", result)
	}
}

// Note: Testing with an actual client requires a running daemon,
// so comprehensive client integration tests should be in a separate _integration_test.go file.
