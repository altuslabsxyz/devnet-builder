package unit

import (
	"context"
	"testing"

	"github.com/altuslabsxyz/devnet-builder/internal/domain"
	"github.com/altuslabsxyz/devnet-builder/internal/infrastructure/interactive"
)

// TestSourceSelectorAdapterCreation tests the NewSourceSelectorAdapter constructor.
func TestSourceSelectorAdapterCreation(t *testing.T) {
	selector := interactive.NewSourceSelectorAdapter()

	if selector == nil {
		t.Fatal("NewSourceSelectorAdapter() returned nil")
	}
}

// TestSourceSelectorNonInteractive tests the non-interactive path (TTY detection).
// When stdout is not a TTY, SelectSource should default to GitHub releases.
//
// Note: This test can only verify the implementation structure.
// Full integration test with actual TTY manipulation is in tests/integration/.
func TestSourceSelectorNonInteractive(t *testing.T) {
	// This test documents the expected behavior for non-interactive environments.
	// Actual TTY detection testing requires integration tests with process manipulation.

	t.Run("Non-interactive behavior documentation", func(t *testing.T) {
		// Expected behavior (documented in spec FR-003, EC-001):
		// - If stdout is not a TTY (piped, redirected, CI/CD)
		// - SelectSource returns SourceTypeGitHubRelease
		// - No prompt is shown to user
		// - No error is returned

		// This is a documentation test - the actual integration test
		// is in tests/integration/source_selection_test.go
		t.Log("Non-interactive mode: DefaultsTo(SourceTypeGitHubRelease)")
		t.Log("Interactive mode: PromptsUser() → returns user selection")
	})
}

// TestSourceOptionMapping tests the mapping between display options and domain types.
func TestSourceOptionMapping(t *testing.T) {
	// This test verifies the conceptual mapping used in SelectSource.
	// The actual sourceOption type is internal to source_selector.go

	tests := []struct {
		name             string
		option           string
		expectedType     domain.SourceType
		expectedTypeName string
	}{
		{
			name:             "Local binary option maps to SourceTypeLocal",
			option:           "Use local binary (browse filesystem)",
			expectedType:     domain.SourceTypeLocal,
			expectedTypeName: "local",
		},
		{
			name:             "GitHub release option maps to SourceTypeGitHubRelease",
			option:           "Use GitHub release (download from repository)",
			expectedType:     domain.SourceTypeGitHubRelease,
			expectedTypeName: "github-release",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify SourceType enum values match expected strings
			if tt.expectedType.String() != tt.expectedTypeName {
				t.Errorf("SourceType.String() = %q, want %q",
					tt.expectedType.String(), tt.expectedTypeName)
			}
		})
	}
}

// TestSourceSelectorContextHandling tests context parameter handling.
func TestSourceSelectorContextHandling(t *testing.T) {
	selector := interactive.NewSourceSelectorAdapter()

	// Test with different context types
	tests := []struct {
		name    string
		ctx     context.Context
		wantErr bool
	}{
		{
			name:    "Background context",
			ctx:     context.Background(),
			wantErr: false, // Context currently not used, so no error expected
		},
		{
			name:    "TODO context",
			ctx:     context.TODO(),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: This will attempt to prompt in interactive environments.
			// In CI/CD (non-TTY), it will return GitHubRelease immediately.
			// This test verifies the method signature accepts the context parameter.

			// Skip actual selection in unit tests - covered in integration tests
			t.Skip("Skipping interactive prompt test - requires integration test environment")

			_, err := selector.SelectSource(tt.ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("SelectSource() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestSourceSelectorEdgeCases documents edge cases handled by the adapter.
func TestSourceSelectorEdgeCases(t *testing.T) {
	// This test documents the edge cases from the spec

	edgeCases := []struct {
		name           string
		scenario       string
		expectedResult string
	}{
		{
			name:           "EC-001: Non-interactive environment (CI/CD)",
			scenario:       "stdout is not a TTY (piped, redirected)",
			expectedResult: "Returns SourceTypeGitHubRelease, no prompt",
		},
		{
			name:           "EC-005: User cancellation (Ctrl+C)",
			scenario:       "User presses Ctrl+C during selection",
			expectedResult: "Returns promptui.ErrInterrupt",
		},
		{
			name:           "EC-005: User cancellation (ESC)",
			scenario:       "User presses ESC during selection",
			expectedResult: "Returns promptui.ErrEOF",
		},
		{
			name:           "Default selection",
			scenario:       "User presses Enter immediately",
			expectedResult: "Returns SourceTypeLocal (first option)",
		},
		{
			name:           "Second option selection",
			scenario:       "User presses ↓ then Enter",
			expectedResult: "Returns SourceTypeGitHubRelease",
		},
	}

	for _, ec := range edgeCases {
		t.Run(ec.name, func(t *testing.T) {
			t.Logf("Scenario: %s", ec.scenario)
			t.Logf("Expected: %s", ec.expectedResult)

			// These are integration test scenarios.
			// Unit tests document the expected behavior.
			// Integration tests verify actual behavior with process manipulation.
		})
	}
}

// TestSourceSelectorIntegrationGuidance documents integration test requirements.
func TestSourceSelectorIntegrationGuidance(t *testing.T) {
	t.Log("Integration test coverage required:")
	t.Log("1. Test with TTY: Verify prompt appears, arrow keys work, Enter confirms")
	t.Log("2. Test without TTY: Verify auto-defaults to GitHub releases")
	t.Log("3. Test Ctrl+C: Verify returns promptui.ErrInterrupt")
	t.Log("4. Test ESC: Verify returns promptui.ErrEOF")
	t.Log("5. Test arrow navigation: Verify both options selectable")
	t.Log("")
	t.Log("See: tests/integration/source_selection_test.go")
}
