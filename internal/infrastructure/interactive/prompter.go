package interactive

import (
	"github.com/manifoldco/promptui"
)

// Prompter abstracts interactive prompt operations for testing.
// This interface allows the interactive layer to remain testable by
// mocking prompt behavior during unit tests.
//
// Design Decision: Minimal interface focused on binary selection use case
// rather than exposing full promptui capabilities.
//
// The prompter handles two types of interactions:
//  1. List selection with arrow key navigation
//  2. Text input for custom version entry
type Prompter interface {
	// SelectFromList displays an interactive list and returns the selected index.
	//
	// Parameters:
	//   - label: Prompt message shown to user (e.g., "Select binary for deployment:")
	//   - items: List of display strings (e.g., formatted binary metadata)
	//   - cursorPos: Optional starting cursor position (nil for 0)
	//
	// Returns:
	//   - index: Selected item index (0-based)
	//   - value: Selected item string (items[index])
	//   - error: promptui.ErrInterrupt if user cancels (Ctrl+C, ESC)
	//
	// User Interaction:
	//   - Arrow keys: Navigate up/down
	//   - Enter: Confirm selection
	//   - Ctrl+C or ESC: Cancel (returns error)
	//   - Search: Type to filter (if supported by implementation)
	SelectFromList(label string, items []string, cursorPos *int) (index int, value string, err error)

	// InputText prompts for text input with a label.
	//
	// Parameters:
	//   - label: Prompt message (e.g., "Enter version to build:")
	//
	// Returns:
	//   - input: User-entered text
	//   - error: promptui.ErrInterrupt if user cancels
	//
	// User Interaction:
	//   - Type text and press Enter to confirm
	//   - Ctrl+C to cancel (returns error)
	InputText(label string) (input string, err error)
}

// PrompterAdapter implements Prompter using promptui library.
// This is the production adapter that provides real interactive prompts.
//
// Note: promptui is already used throughout the codebase (see selector.go),
// so we're maintaining consistency by reusing the same library.
type PrompterAdapter struct{}

// NewPrompterAdapter creates a new promptui-based prompter adapter.
// This is the default implementation used in production.
func NewPrompterAdapter() *PrompterAdapter {
	return &PrompterAdapter{}
}

// SelectFromList implements Prompter.SelectFromList using promptui.Select.
//
// Implementation details:
//   - Uses promptui.Select with custom templates matching existing style
//   - Search mode disabled by default
//   - Size limited to 10 items visible at once (scrollable)
func (p *PrompterAdapter) SelectFromList(label string, items []string, cursorPos *int) (int, string, error) {
	// Import promptui - already used in this package (see selector.go)
	// This maintains consistency with existing interactive flows

	// Configure select prompt
	cursor := 0
	if cursorPos != nil {
		cursor = *cursorPos
	}

	templates := &promptui.SelectTemplates{
		Active:   "▸ {{ . | cyan }}",  // Selected item (cyan like version selection)
		Inactive: "  {{ . }}",         // Unselected items
		Selected: "✓ {{ . | green }}", // After selection (green checkmark)
	}

	prompt := promptui.Select{
		Label:             label,
		Items:             items,
		Size:              10,     // Show 10 items at once (scrollable)
		CursorPos:         cursor, // Start at specified position
		Templates:         templates,
		StartInSearchMode: false, // Don't start in search mode
	}

	return prompt.Run()
}

// InputText implements Prompter.InputText using promptui.Prompt.
//
// Implementation details:
//   - Uses promptui.Prompt for text input
//   - No validation to allow any text input (branch names, tags, commit hashes)
//   - Supports Ctrl+C cancellation
func (p *PrompterAdapter) InputText(label string) (string, error) {
	prompt := promptui.Prompt{
		Label: label,
	}

	return prompt.Run()
}
