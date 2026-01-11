package interactive

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/b-harvest/devnet-builder/internal/application/ports"
	"github.com/chzyer/readline"
	"github.com/manifoldco/promptui"
)

// FilesystemBrowserAdapter implements the FilesystemBrowser port interface using readline
// for interactive path selection with tab auto-completion.
//
// This adapter follows Clean Architecture principles by:
//  1. Implementing the port interface defined in the application layer
//  2. Using infrastructure-specific code (readline) isolated to this layer
//
// Design Decisions:
//   - Uses chzyer/readline for simple tab completion
//   - Leverages existing PathCompleter for file system navigation
//   - Tab key provides auto-completion suggestions
//   - Simple and lightweight implementation
//   - Validates selected path using os.Stat and executable permission checks
//
// User Experience:
//   - Type path and press Tab for auto-completion
//   - Type `/`: shows root directory contents
//   - Type `./`: shows current directory contents
//   - Type `~/`: shows home directory contents
//   - Arrow keys: navigate input history
//   - Enter: select and validate path
//   - Ctrl+C: cancel selection
//
// Performance:
//   - Efficient prefix matching
//   - On-demand directory scanning
type FilesystemBrowserAdapter struct {
	pathCompleter ports.PathCompleter
}

// NewFilesystemBrowserAdapter creates a new FilesystemBrowserAdapter instance.
func NewFilesystemBrowserAdapter(pathCompleter ports.PathCompleter) *FilesystemBrowserAdapter {
	return &FilesystemBrowserAdapter{
		pathCompleter: pathCompleter,
	}
}

// readlineCompleter implements readline.AutoCompleter interface
type readlineCompleter struct {
	pathCompleter ports.PathCompleter
}

// Do implements readline.AutoCompleter.Do
// Returns completion suggestions as suffixes to append to the current input.
//
// The readline library expects:
//   - newLine: Array of suffixes to append after removing 'length' characters
//   - length: Number of characters to remove from the end before appending suffix
//
// Algorithm:
//   - PathCompleter returns full paths like ["/home/", "/home2/"]
//   - Sort by modification time (newest first)
//   - Take top 5 most recent entries
//   - Extract the suffix by removing the common prefix (current input)
//   - For input "/ho" with completion "/home/", suffix is "me/"
//   - Return suffix "me/" with length 0 (don't remove anything, just append)
//   - readline automatically handles Tab cycling through suggestions
func (c *readlineCompleter) Do(line []rune, pos int) (newLine [][]rune, length int) {
	// Get current line as string up to cursor position
	lineStr := string(line[:pos])

	// Get completions from PathCompleter
	completions := c.pathCompleter.Complete(lineStr)

	// If no completions, return empty
	if len(completions) == 0 {
		return [][]rune{}, 0
	}

	// Sort by modification time (newest first) and take top 5
	completions = c.sortByModTimeAndLimit(completions, 5)

	// Extract suffixes by removing the current input prefix
	suggestions := make([][]rune, 0, len(completions))
	for _, comp := range completions {
		// Remove the common prefix (current input) to get the suffix
		suffix := comp
		if strings.HasPrefix(comp, lineStr) {
			suffix = comp[len(lineStr):]
		}

		// Only add non-empty suffixes
		if len(suffix) > 0 {
			suggestions = append(suggestions, []rune(suffix))
		}
	}

	// Return suffixes with length 0 (append only, don't remove anything)
	return suggestions, 0
}

// sortByModTimeAndLimit sorts paths by modification time (newest first) and limits to top N
func (c *readlineCompleter) sortByModTimeAndLimit(paths []string, limit int) []string {
	// Create a slice to hold path info with mod times
	type pathInfo struct {
		path    string
		modTime int64
	}

	infos := make([]pathInfo, 0, len(paths))
	for _, path := range paths {
		// Get file info
		info, err := os.Stat(path)
		if err != nil {
			// If we can't stat, use a very old time (will be sorted last)
			infos = append(infos, pathInfo{path: path, modTime: 0})
			continue
		}
		infos = append(infos, pathInfo{path: path, modTime: info.ModTime().Unix()})
	}

	// Sort by modification time (newest first)
	for i := 0; i < len(infos)-1; i++ {
		for j := i + 1; j < len(infos); j++ {
			if infos[j].modTime > infos[i].modTime {
				infos[i], infos[j] = infos[j], infos[i]
			}
		}
	}

	// Extract paths and limit to top N
	result := make([]string, 0, limit)
	for i := 0; i < len(infos) && i < limit; i++ {
		result = append(result, infos[i].path)
	}

	return result
}

// BrowsePath prompts the user to browse and select a filesystem path using an interactive prompt.
//
// This method implements the interactive file browsing flow using readline:
//  1. Display interactive input with auto-completion
//  2. User types path with tab completion
//  3. User presses Tab to see auto-completion suggestions
//  4. User presses Enter to select a file
//  5. Validate selected path:
//     - File exists (not directory)
//     - File is executable (mode & 0111 != 0)
//     - Symlinks are resolved (max depth 10)
//  6. If validation fails, show error and allow retry
//  7. If validation succeeds, return absolute path
//
// Parameters:
//   - ctx: Context for cancellation and timeout control
//   - initialPath: Starting directory hint
//
// Returns:
//   - string: Absolute path to the selected binary file
//   - error: Validation error, cancellation error, or system error
func (f *FilesystemBrowserAdapter) BrowsePath(ctx context.Context, initialPath string) (string, error) {
	// Check context cancellation before starting
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	// Determine starting directory for display hint
	startDir := initialPath
	if startDir == "" {
		if cwd, err := os.Getwd(); err == nil {
			startDir = cwd
		} else {
			startDir = "."
		}
	}

	// Loop until valid file is selected or user cancels
	for {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		// Show instructions
		fmt.Fprintln(os.Stderr, "\n📁 Binary File Browser")
		fmt.Fprintln(os.Stderr, "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Fprintf(os.Stderr, "Current directory: %s\n", startDir)
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Instructions:")
		fmt.Fprintln(os.Stderr, "  • Type path and press Tab for auto-completion")
		fmt.Fprintln(os.Stderr, "  • Type / for root, ./ for current, ~/ for home")
		fmt.Fprintln(os.Stderr, "  • Arrow keys to navigate history")
		fmt.Fprintln(os.Stderr, "  • Enter to select, Ctrl+C or Ctrl+D to cancel")
		fmt.Fprintln(os.Stderr, "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		fmt.Fprintln(os.Stderr, "")

		// Create readline instance with auto-completer
		rl, err := readline.NewEx(&readline.Config{
			Prompt:          "Binary path: ",
			AutoComplete:    &readlineCompleter{pathCompleter: f.pathCompleter},
			InterruptPrompt: "^C",
			EOFPrompt:       "exit",
			Stderr:          os.Stderr,
			Stdout:          os.Stderr,
		})
		if err != nil {
			return "", fmt.Errorf("failed to create readline: %w", err)
		}

		// Read input with auto-completion
		input, err := rl.Readline()
		if err != nil {
			rl.Close()
			if err == readline.ErrInterrupt || err == io.EOF {
				return "", promptui.ErrInterrupt
			}
			return "", fmt.Errorf("failed to read input: %w", err)
		}

		// Trim whitespace
		input = strings.TrimSpace(input)

		// Check if user cancelled (empty input is treated as cancel)
		if input == "" {
			rl.Close()
			return "", promptui.ErrInterrupt
		}

		// Close readline after successful read
		rl.Close()

		// Validate the selected path
		validatedPath, err := f.validatePath(input)
		if err != nil {
			// Show error and allow retry
			fmt.Fprintf(os.Stderr, "\n❌ %v\n", err)
			fmt.Fprintln(os.Stderr, "   Press Enter to try again or Ctrl+C to cancel...")
			fmt.Fprintln(os.Stderr, "")

			// Wait for Enter key
			rl2, err := readline.NewEx(&readline.Config{
				Prompt: "",
				Stderr: os.Stderr,
				Stdout: os.Stderr,
			})
			if err == nil && rl2 != nil {
				_, _ = rl2.Readline() // Ignore error - just waiting for Enter
				rl2.Close()
			}

			// Update start directory to the directory of the failed path
			if absPath, err := filepath.Abs(input); err == nil {
				startDir = filepath.Dir(absPath)
			}
			continue
		}

		return validatedPath, nil
	}
}

// validatePath validates the selected path and returns the absolute, resolved path.
//
// This method implements comprehensive validation:
//  1. Check if path exists
//  2. Resolve symlinks (max depth 10 to prevent circular links)
//  3. Check if path points to a file (not a directory)
//  4. Check if file has executable permissions
//  5. Return absolute path
//
// Parameters:
//   - input: The user's selected path (may be relative, contain symlinks, or have spaces)
//
// Returns:
//   - string: Absolute, resolved path to the binary
//   - error: Validation error with user-friendly message
func (f *FilesystemBrowserAdapter) validatePath(input string) (string, error) {
	// Expand tilde (~) to home directory if present
	if strings.HasPrefix(input, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to expand home directory: %w", err)
		}
		input = filepath.Join(home, input[2:])
	}

	// Convert to absolute path
	absPath, err := filepath.Abs(input)
	if err != nil {
		return "", fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	// Resolve symlinks with max depth 10 to prevent circular links
	resolvedPath, err := f.resolveSymlinks(absPath, 10)
	if err != nil {
		return "", err
	}

	// Check if path exists
	info, err := os.Stat(resolvedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("file not found: %s", input)
		}
		if os.IsPermission(err) {
			return "", fmt.Errorf("permission denied: %s", input)
		}
		return "", fmt.Errorf("failed to access path: %w", err)
	}

	// Check if path is a file (not a directory)
	if info.IsDir() {
		return "", fmt.Errorf("selected path is a directory, not a file: %s", input)
	}

	// Check if file is executable
	if !f.isExecutable(info) {
		return "", fmt.Errorf("binary is not executable: %s (try: chmod +x %s)", input, input)
	}

	// All validations passed
	return resolvedPath, nil
}

// resolveSymlinks follows symlink chains up to maxDepth levels.
func (f *FilesystemBrowserAdapter) resolveSymlinks(path string, maxDepth int) (string, error) {
	// Base case: maxDepth reached, prevent infinite loops
	if maxDepth <= 0 {
		return "", fmt.Errorf("symlink chain too deep (max 10 levels): %s", path)
	}

	// Check if path is a symlink using Lstat (doesn't follow symlinks)
	info, err := os.Lstat(path)
	if err != nil {
		return "", fmt.Errorf("failed to stat path: %w", err)
	}

	// If not a symlink, return the path as-is
	if info.Mode()&os.ModeSymlink == 0 {
		return path, nil
	}

	// Read the symlink target
	target, err := os.Readlink(path)
	if err != nil {
		return "", fmt.Errorf("failed to read symlink: %w", err)
	}

	// If target is relative, resolve it relative to the symlink's directory
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(path), target)
	}

	// Recursively resolve the target
	return f.resolveSymlinks(target, maxDepth-1)
}

// isExecutable checks if a file has executable permissions.
func (f *FilesystemBrowserAdapter) isExecutable(info os.FileInfo) bool {
	// On Unix-like systems, check if any executable bit is set
	return info.Mode()&0111 != 0
}
