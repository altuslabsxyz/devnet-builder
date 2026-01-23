package interactive

import (
	"context"
	"fmt"
	"os"

	"github.com/altuslabsxyz/devnet-builder/internal/infrastructure/cache"
	"github.com/manifoldco/promptui"
	"golang.org/x/term"
)

// BinarySelectionOptions configures the binary selection behavior.
// This struct encapsulates all configuration needed for different selection scenarios.
type BinarySelectionOptions struct {
	// AllowBuildFromSource adds "Build from source" option to the list
	// Set to true for deploy command, false for selection-only scenarios
	AllowBuildFromSource bool

	// AutoSelectSingle automatically selects when only one valid binary exists
	// Per CLARIFICATION 1: Option A - Auto-select silently with info log
	AutoSelectSingle bool

	// IsInteractive indicates if running in TTY (interactive terminal)
	// Per CLARIFICATION 2: Non-TTY environments auto-select first valid binary
	IsInteractive bool
}

// BinarySelectionResult represents the outcome of binary selection.
// This follows the Result pattern for explicit success/failure handling.
type BinarySelectionResult struct {
	// SelectedBinary points to the chosen binary metadata (nil if build selected)
	SelectedBinary *cache.CachedBinaryMetadata

	// BinaryPath is the absolute path to the selected binary
	BinaryPath string

	// ShouldBuild is true if user selected "Build from source"
	ShouldBuild bool

	// BuildVersion is the version/ref to build (only if ShouldBuild=true)
	BuildVersion string

	// WasCancelled is true if user pressed Ctrl+C or ESC
	WasCancelled bool
}

// BinarySelector handles interactive binary selection with all edge cases.
// This implements the Selector component from the implementation plan (Task T032-T033).
//
// Responsibilities:
//   - Display formatted binary list with arrow key navigation
//   - Handle all edge cases defined in spec (EC-001 to EC-012)
//   - Support both interactive and non-interactive modes
//   - Integrate with promptui for consistent UX
//
// Design Decision: Separate selector from scanner/validator for Single Responsibility.
// Scanner finds binaries, Validator checks them, Selector presents choices.
type BinarySelector struct {
	prompter Prompter
}

// NewBinarySelector creates a new binary selector with the given prompter.
//
// Parameters:
//   - prompter: Prompt adapter (use NewPrompterAdapter() for production, mock for tests)
//
// Example:
//
//	selector := NewBinarySelector(NewPrompterAdapter())
//	result, err := selector.RunBinarySelectionFlow(ctx, binaries, opts)
func NewBinarySelector(prompter Prompter) *BinarySelector {
	return &BinarySelector{
		prompter: prompter,
	}
}

// RunBinarySelectionFlow orchestrates the complete binary selection process.
//
// This method implements all functional requirements and edge cases from the spec:
//   - FR-003: Interactive binary list display with formatting
//   - FR-005: Binary resolution priority order
//   - FR-010: Non-interactive mode handling
//   - FR-011: Build from source option
//   - EC-001 to EC-012: All edge case scenarios
//
// Edge Cases Handled:
//   - EC-001: Zero binaries → returns empty result (caller decides to build)
//   - EC-002: Single binary + auto-select → returns immediately with info log
//   - EC-004: Non-TTY environment → auto-selects first valid binary
//   - EC-005: User cancellation → returns WasCancelled=true
//   - EC-008: Large cache (>50 binaries) → all displayed with scrolling
//
// Parameters:
//   - ctx: Context for cancellation (currently unused but reserved)
//   - binaries: Valid binaries from scanner (pre-filtered by validator)
//   - opts: Selection options (auto-select, interactive mode, build option)
//
// Returns:
//   - BinarySelectionResult with selection outcome
//   - Error only for unexpected failures (not for cancellation)
//
// User Interaction Flow:
//  1. Check for edge cases (zero, single, non-TTY)
//  2. Display formatted list with arrow key navigation
//  3. User selects binary or "Build from source"
//  4. If build selected, prompt for version/ref
//  5. Return result with selected binary or build request
func (s *BinarySelector) RunBinarySelectionFlow(
	ctx context.Context,
	binaries []cache.CachedBinaryMetadata,
	opts BinarySelectionOptions,
) (*BinarySelectionResult, error) {

	// EC-001: Zero cached binaries
	// Spec: "No error, return empty result indicating build is needed"
	if len(binaries) == 0 {
		// Caller will decide to build from source based on empty result
		return &BinarySelectionResult{
			ShouldBuild: false, // Not explicitly requesting build
		}, nil
	}

	// EC-002: Single cached binary + auto-select enabled
	// Per CLARIFICATION 1: Option A - Auto-select silently with info log
	if len(binaries) == 1 && opts.AutoSelectSingle {
		b := binaries[0]
		// Note: Logging is done by the caller (deploy.go/upgrade.go)
		// to maintain separation of concerns
		return &BinarySelectionResult{
			SelectedBinary: &b,
			BinaryPath:     b.Path,
			ShouldBuild:    false,
			WasCancelled:   false,
		}, nil
	}

	// EC-004: Non-TTY environment (CI/CD, piped input)
	// Per CLARIFICATION 2: Option A - Auto-select first valid binary with log
	if !opts.IsInteractive {
		b := binaries[0] // Already sorted by most recent (scanner does this)
		// Note: Caller logs "Non-interactive environment detected, using {path}"
		return &BinarySelectionResult{
			SelectedBinary: &b,
			BinaryPath:     b.Path,
			ShouldBuild:    false,
			WasCancelled:   false,
		}, nil
	}

	// Interactive selection: Build display items
	// EC-008: Large cache (>50 binaries) - display all with scrolling (promptui handles this)
	items := make([]string, len(binaries))
	for i := range binaries {
		items[i] = formatBinaryForDisplay(binaries[i])
	}

	// Add "Build from source" option if allowed
	// FR-011: Users can choose to build instead of using cache
	if opts.AllowBuildFromSource {
		items = append(items, formatBuildFromSourceOption())
	}

	// Display interactive prompt with arrow key navigation
	// FR-003: Display list with binary name, version, commit, cache key, size, time
	index, _, err := s.prompter.SelectFromList("Select binary for deployment:", items, nil)
	if err != nil {
		// EC-005: User cancelled (Ctrl+C, ESC)
		// promptui.ErrInterrupt and promptui.ErrEOF indicate cancellation
		if err == promptui.ErrInterrupt || err == promptui.ErrEOF {
			return &BinarySelectionResult{
				WasCancelled: true,
			}, nil
		}
		// Unexpected error (should not happen with promptui)
		return nil, fmt.Errorf("selection prompt failed: %w", err)
	}

	// Check if "Build from source" was selected
	// This will be the last item if opts.AllowBuildFromSource=true
	if opts.AllowBuildFromSource && index == len(binaries) {
		// Prompt for version/ref to build
		// FR-011: User can specify branch name, tag, or commit hash
		version, err := s.prompter.InputText("Enter version to build (tag/branch/commit):")
		if err != nil {
			// User cancelled version input
			if err == promptui.ErrInterrupt || err == promptui.ErrEOF {
				return &BinarySelectionResult{
					WasCancelled: true,
				}, nil
			}
			return nil, fmt.Errorf("version input prompt failed: %w", err)
		}

		return &BinarySelectionResult{
			ShouldBuild:  true,
			BuildVersion: version,
			WasCancelled: false,
		}, nil
	}

	// Return selected binary
	// Index is valid because promptui validates it
	selected := binaries[index]
	return &BinarySelectionResult{
		SelectedBinary: &selected,
		BinaryPath:     selected.Path,
		ShouldBuild:    false,
		WasCancelled:   false,
	}, nil
}

// IsTerminalInteractive checks if the current environment supports interactive prompts.
//
// This implements EC-004 detection logic using golang.org/x/term package.
//
// Returns:
//   - true if stdout is a TTY (terminal) - interactive mode
//   - false if stdout is piped/redirected - non-interactive mode (CI/CD)
//
// Examples:
//   - Interactive: `./devnet-builder deploy --mode local`
//   - Non-interactive: `echo "" | ./devnet-builder deploy --mode local`
//   - Non-interactive: Running in CI/CD pipeline
//
// Note: This is a helper function that can be called from deploy.go/upgrade.go
// before calling RunBinarySelectionFlow to set opts.IsInteractive.
func IsTerminalInteractive() bool {
	// Check stdin (not stdout) because promptui reads from stdin
	// If stdin is piped/redirected, interactive prompts won't work
	return term.IsTerminal(int(os.Stdin.Fd()))
}
