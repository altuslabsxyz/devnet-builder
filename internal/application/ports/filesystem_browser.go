package ports

import "context"

// FilesystemBrowser provides interactive filesystem navigation with autocomplete.
// This port abstracts the path browsing UI from the application logic.
//
// Design Decision: FilesystemBrowser depends on PathCompleter port for completion logic.
// This follows Interface Segregation Principle - separate concerns of completion logic
// from UI interaction logic.
//
// Responsibility:
//   - Display path input prompt with inline help
//   - Handle Tab key for autocomplete (delegates to PathCompleter)
//   - Handle Enter key for selection
//   - Handle ESC key for cancellation
//   - Validate final selection (file exists, is executable)
//
// Implementation Notes:
//   - Infrastructure adapter will use promptui.Prompt with AutoComplete function
//   - PathCompleter provides the actual completion suggestions
//   - Final validation uses BinaryVersionDetector port to verify binary
type FilesystemBrowser interface {
	// BrowsePath prompts user to enter/browse filesystem path with Tab autocomplete.
	//
	// User Flow:
	//  1. Display prompt: "Enter binary path (use Tab for autocomplete, ESC to go back):"
	//  2. User types partial path (e.g., "/Use")
	//  3. User presses Tab → system shows completions (e.g., "/Users/")
	//  4. User continues typing or selects from completions
	//  5. User presses Enter to confirm final selection
	//  6. System validates the path:
	//     - File exists (not directory)
	//     - File is executable (mode & 0111 != 0)
	//     - File is not corrupted (basic checks)
	//  7. If validation fails, prompt again with error message
	//  8. If validation succeeds, return the absolute path
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout control
	//   - initialPath: Starting path for the prompt (default: current directory or "/")
	//
	// Returns:
	//   - string: Absolute path to the selected binary file
	//   - error: Validation error, cancellation error, or system error
	//
	// Error Types:
	//   - promptui.ErrInterrupt: User pressed Ctrl+C
	//   - promptui.ErrEOF: User pressed ESC
	//   - Validation errors: "path is a directory", "binary is not executable", etc.
	//
	// Validation Rules:
	//   - Path MUST exist on filesystem
	//   - Path MUST point to a file (not a directory)
	//   - File MUST have executable permissions
	//   - Symlinks are followed (max depth: 10 to prevent circular links)
	//
	// Edge Cases:
	//   - Path with spaces: Supports both quoted and unquoted input
	//     Example: "/Users/my folder/binary" or "/Users/my\ folder/binary"
	//   - Symlinks: Follows symlink chains up to 10 levels
	//   - Permission denied: Returns clear error message with file path
	//   - Large directories (> 1000 files): Autocomplete shows first 100 alphabetically
	//   - Non-existent path: Shows inline error "Path not found"
	//   - Directory selected: Shows error "Selected path is a directory, not a file"
	//   - Non-executable file: Shows error "Binary is not executable (try: chmod +x <path>)"
	//
	// Examples:
	//   - User enters "/usr/local/bin/stabled" → validates and returns path
	//   - User enters "/home/user/Downloads" → returns error "path is a directory"
	//   - User enters "/tmp/notexist" → returns error "file not found"
	//   - User presses ESC → returns promptui.ErrEOF
	BrowsePath(ctx context.Context, initialPath string) (string, error)
}
