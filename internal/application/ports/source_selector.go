package ports

import (
	"context"

	"github.com/altuslabsxyz/devnet-builder/internal/domain"
)

// SourceSelector provides UI for choosing between binary source types.
// This port abstracts the interactive selection UI from the application logic.
//
// Design Decision: This interface uses domain types (domain.SourceType) rather than
// primitive types, following Domain-Driven Design principles.
//
// Responsibility:
//   - Display source selection options to the user
//   - Handle user navigation (arrow keys, Enter, ESC)
//   - Return the selected source type or cancellation error
//
// Implementation Notes:
//   - Infrastructure adapter will use promptui for consistent UX
//   - Options are displayed in order: Local Binary (default), GitHub Release
//   - Arrow keys for navigation, Enter to confirm, Ctrl+C/ESC to cancel
//   - Non-interactive environments should default to GitHub Release
type SourceSelector interface {
	// SelectSource prompts the user to choose between local binary and GitHub release.
	//
	// User Flow:
	//  1. Display two options with arrow key navigation:
	//     - "Use local binary (browse filesystem)"
	//     - "Use GitHub release (download from repository)"
	//  2. User navigates with ↑/↓ arrow keys
	//  3. User confirms with Enter key
	//  4. User cancels with Ctrl+C or ESC
	//
	// Parameters:
	//   - ctx: Context for cancellation (currently unused, reserved for future timeouts)
	//
	// Returns:
	//   - domain.SourceType: The selected source type (Local or GitHubRelease)
	//   - error: promptui.ErrInterrupt or promptui.ErrEOF if user cancels, nil on success
	//
	// Error Handling:
	//   - User cancellation (Ctrl+C): returns promptui.ErrInterrupt
	//   - User cancellation (ESC): returns promptui.ErrEOF
	//   - System errors (rare): returns wrapped error
	//
	// Non-Interactive Behavior:
	//   - If stdout is not a TTY, should detect and return default (GitHubRelease)
	//   - Logs info message: "Non-interactive environment detected, defaulting to GitHub releases"
	//
	// Examples:
	//   - User presses Enter immediately → returns SourceTypeLocal (first option)
	//   - User presses ↓ then Enter → returns SourceTypeGitHubRelease
	//   - User presses Ctrl+C → returns promptui.ErrInterrupt
	//   - CI/CD environment (non-TTY) → returns SourceTypeGitHubRelease with log
	SelectSource(ctx context.Context) (domain.SourceType, error)
}
