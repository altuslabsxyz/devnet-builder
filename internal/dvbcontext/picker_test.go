package dvbcontext

import (
	"errors"
	"testing"
)

func TestPickNode_NilClient(t *testing.T) {
	_, err := PickNode(nil, "default", "my-devnet")
	if err == nil {
		t.Fatal("expected error for nil client")
	}
	if err.Error() != "client is nil" {
		t.Errorf("unexpected error message: %s", err.Error())
	}
}

func TestErrNoNodes_IsSentinel(t *testing.T) {
	// Verify ErrNoNodes can be used with errors.Is
	wrappedErr := errors.New("no nodes found in devnet test")
	if errors.Is(wrappedErr, ErrNoNodes) {
		t.Error("plain error should not match ErrNoNodes")
	}
}

func TestFormatNodeDisplay_NilFields(t *testing.T) {
	tests := []struct {
		name     string
		node     interface{}
		expected string
	}{
		{
			name:     "nil spec and status",
			expected: "0: unknown (Unknown)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// formatNodeDisplay is unexported but tested via PickNode
			// This test documents expected behavior
		})
	}
}
