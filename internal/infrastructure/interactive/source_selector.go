package interactive

import (
	"context"
	"os"

	"github.com/altuslabsxyz/devnet-builder/internal/domain"
	"github.com/manifoldco/promptui"
	"golang.org/x/term"
)

// SourceSelectorAdapter implements the SourceSelector port using promptui.
// This adapter follows the Adapter pattern from Clean Architecture,
// translating infrastructure concerns (promptui) to domain concepts (SourceType).
//
// Design Decision: Use promptui.Select for consistency with existing UI patterns.
// All interactive prompts in the application use promptui for uniform UX.
type SourceSelectorAdapter struct {
	// No state needed - this is a stateless adapter
}

// NewSourceSelectorAdapter creates a new source selector adapter.
//
// Returns:
//   - *SourceSelectorAdapter: Ready-to-use source selector
func NewSourceSelectorAdapter() *SourceSelectorAdapter {
	return &SourceSelectorAdapter{}
}

// SelectSource prompts the user to choose between local binary and GitHub release.
// This implements the SourceSelector port interface.
//
// User Flow:
//  1. Check if running in interactive environment (TTY detection)
//  2. If non-interactive → default to GitHub releases with log message
//  3. If interactive → display two options with arrow key navigation
//  4. User navigates with ↑/↓, confirms with Enter, cancels with Ctrl+C/ESC
//
// Parameters:
//   - ctx: Context for cancellation (currently unused, reserved for future timeout support)
//
// Returns:
//   - domain.SourceType: The selected source type (Local or GitHubRelease)
//   - error: promptui.ErrInterrupt/ErrEOF if user cancels, nil on success
//
// Implementation Notes:
//   - Non-TTY detection uses golang.org/x/term package
//   - Default option is "GitHub release" (first in list) for convenience
//   - Cursor starts on first option ("Use GitHub release")
func (s *SourceSelectorAdapter) SelectSource(ctx context.Context) (domain.SourceType, error) {
	// Non-interactive environment detection (EC-001)
	if !isTerminalInteractive() {
		// Default to GitHub releases in CI/CD environments
		// This is the safe default - doesn't require filesystem access
		return domain.SourceTypeGitHubRelease, nil
	}

	// Define source options for display
	// Format: Clear, action-oriented labels with context
	// GitHub release is first (default) as it's the most common use case
	sourceOptions := []sourceOption{
		{
			Label:       "Use GitHub release (download from repository)",
			Description: "Fetch and download an official release from GitHub",
			Type:        domain.SourceTypeGitHubRelease,
		},
		{
			Label:       "Use local binary (browse filesystem)",
			Description: "Select a binary file from your local filesystem with autocomplete",
			Type:        domain.SourceTypeLocal,
		},
	}

	// Create promptui.Select with custom templates for better UX
	templates := &promptui.SelectTemplates{
		Label:    "{{ . }}",
		Active:   "▸ {{ .Label | cyan }}",
		Inactive: "  {{ .Label }}",
		Selected: "✓ {{ .Label | green }}",
		Details: `
--------- Source Selection ---------
{{ "Description:" | faint }}	{{ .Description }}`,
	}

	prompt := promptui.Select{
		Label:     "Select binary source:",
		Items:     sourceOptions,
		Templates: templates,
		Size:      2, // Only 2 options, no need for scrolling
	}

	// Run the selection prompt
	index, _, err := prompt.Run()
	if err != nil {
		// User cancellation (Ctrl+C or ESC)
		return domain.SourceTypeLocal, err
	}

	// Return the selected source type
	return sourceOptions[index].Type, nil
}

// sourceOption represents a binary source option for display.
// This is an internal type used only for promptui rendering.
type sourceOption struct {
	Label       string            // Display text for the option
	Description string            // Detailed description shown in details pane
	Type        domain.SourceType // The domain source type this option represents
}

// isTerminalInteractive checks if the current environment supports interactive prompts.
// This implements EC-004 detection logic using golang.org/x/term package.
//
// Returns:
//   - true if stdout is a TTY (terminal) - interactive mode available
//   - false if stdout is piped/redirected - non-interactive mode (CI/CD)
//
// Examples:
//   - Interactive: `./devnet-builder deploy --mode local`
//   - Non-interactive: `echo "" | ./devnet-builder deploy --mode local`
//   - Non-interactive: Running in CI/CD pipeline (GitHub Actions, Jenkins, etc.)
//
// Implementation Note:
//   - This is a package-level helper function, not a method
//   - Can be called before creating SourceSelectorAdapter
//   - Consistent with IsTerminalInteractive in binary_selector.go
func isTerminalInteractive() bool {
	// Check stdin (not stdout) because promptui reads from stdin
	// If stdin is piped/redirected, interactive prompts won't work
	return term.IsTerminal(int(os.Stdin.Fd()))
}
