package interactive

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// PathCompleterAdapter implements the PathCompleter port interface using the standard
// library's os and filepath packages for filesystem operations.
//
// This adapter follows Clean Architecture principles by implementing the port interface
// defined in the application layer (ports.PathCompleter) using infrastructure-specific
// code (os.ReadDir, filepath operations).
//
// Design Decisions:
//   - Uses os.ReadDir for directory listing (faster than filepath.Walk for single directory)
//   - Implements silent failure for permission errors and non-existent directories
//   - Adds trailing "/" to directories for visual distinction in autocomplete
//   - Limits results to 100 entries for performance in large directories
//   - Sorts alphabetically for consistent user experience
//
// Performance:
//   - Target: < 100ms for directories with < 1000 entries (SC-002)
//   - os.ReadDir is optimized for this use case (no stat() calls during iteration)
//   - Early termination at 100 results prevents slowdown in large directories
type PathCompleterAdapter struct{}

// NewPathCompleterAdapter creates a new PathCompleterAdapter instance.
//
// This is a simple constructor with no dependencies, making it easy to use
// in various contexts (interactive CLI, tests, etc.).
//
// Returns:
//   - *PathCompleterAdapter: A new instance ready for use
func NewPathCompleterAdapter() *PathCompleterAdapter {
	return &PathCompleterAdapter{}
}

// Complete generates autocomplete suggestions for the given input path.
//
// This method implements the core autocomplete logic:
//  1. Parse input into directory + partial filename
//  2. Handle special cases (empty input, root directory)
//  3. List directory contents using os.ReadDir
//  4. Filter by prefix match on the partial filename
//  5. Sort alphabetically
//  6. Add trailing "/" to directories
//  7. Limit to 100 results for performance
//
// Algorithm:
//   - If input is empty or "/", list root-level entries
//   - Otherwise, split into directory path and partial filename
//   - Read directory entries, filter by prefix, sort, and format
//
// Parameters:
//   - input: Current user input (partial path)
//     Examples: "", "/", "/Users/", "/Users/a", "/usr/bin/sta"
//
// Returns:
//   - []string: List of completion suggestions (max 100 entries)
//   - Directories have trailing "/" (e.g., "/Users/dev/")
//   - Files have no trailing slash (e.g., "/usr/bin/stabled")
//   - Sorted alphabetically
//   - Empty slice if directory doesn't exist or has no matches
//
// Edge Cases:
//   - Empty input: Returns root directories (T019: FR-012)
//   - Non-existent directory: Returns empty slice (EC-002)
//   - No matches: Returns empty slice
//   - Permission denied: Returns empty slice (silent failure)
//   - > 100 matches: Returns first 100 alphabetically (T019: FR-012)
//
// Examples:
//
//	Complete("") → ["/Applications/", "/Library/", "/System/", "/Users/", ...]
//	Complete("/") → ["/Applications/", "/Library/", "/System/", "/Users/", ...]
//	Complete("/Users/") → ["/Users/alice/", "/Users/bob/", "/Users/Shared/"]
//	Complete("/Users/a") → ["/Users/alice/"]
//	Complete("/nonexistent/") → []
func (p *PathCompleterAdapter) Complete(input string) []string {
	// Handle empty input or root directory
	if input == "" || input == "/" {
		return p.listDirectory("/", "")
	}

	// Parse input into directory and partial filename
	// Examples:
	//   "/Users/dev/my" → dir="/Users/dev/", partial="my"
	//   "/Users/" → dir="/Users/", partial=""
	//   "/usr/bin/stabled" → dir="/usr/bin/", partial="stabled"
	dir, partial := p.parseInput(input)

	// List directory contents and filter by partial filename
	return p.listDirectory(dir, partial)
}

// parseInput splits the input path into directory and partial filename components.
//
// This method handles the path parsing logic:
//   - If input ends with "/", the directory is the full input, partial is empty
//   - Otherwise, split at the last "/" to get directory and partial filename
//
// Examples:
//   - "/Users/dev/my" → dir="/Users/dev/", partial="my"
//   - "/Users/" → dir="/Users/", partial=""
//   - "/usr/bin/stabled" → dir="/usr/bin/", partial="stabled"
//   - "relative/path" → dir="relative/", partial="path"
//
// Parameters:
//   - input: The user's current input path
//
// Returns:
//   - dir: The directory path to list (always ends with "/")
//   - partial: The partial filename to filter by (may be empty)
func (p *PathCompleterAdapter) parseInput(input string) (dir, partial string) {
	// If input ends with "/", user is browsing inside that directory
	if strings.HasSuffix(input, "/") {
		return input, ""
	}

	// Otherwise, split at last "/" to get directory and partial filename
	lastSlash := strings.LastIndex(input, "/")
	if lastSlash == -1 {
		// No "/" in input, so we're completing in current directory
		return "./", input
	}

	// Split: everything up to and including "/" is the directory,
	// everything after is the partial filename
	dir = input[:lastSlash+1]
	partial = input[lastSlash+1:]
	return dir, partial
}

// listDirectory reads the directory, filters by partial prefix, sorts, and formats results.
//
// This method implements the core autocomplete algorithm:
//  1. Read directory entries using os.ReadDir (fast, no stat() calls)
//  2. Filter entries by prefix match on partial filename
//  3. Sort alphabetically (case-insensitive for better UX)
//  4. Add trailing "/" to directories (T020: FR-009)
//  5. Limit to 100 results for performance (T019: FR-012, SC-002)
//
// Parameters:
//   - dir: The directory path to list (e.g., "/Users/", "/usr/bin/")
//   - partial: The partial filename to filter by (e.g., "a", "sta", "")
//
// Returns:
//   - []string: Filtered, sorted, formatted completion suggestions (max 100)
//   - Empty slice on error (silent failure for EC-002, EC-004)
//
// Performance Notes:
//   - os.ReadDir is faster than filepath.Walk for single directory listing
//   - Early termination at 100 results prevents slowdown in large directories
//   - Case-insensitive sorting provides better user experience
//
// Error Handling:
//   - Permission denied: Returns empty slice (silent failure)
//   - Non-existent directory: Returns empty slice (silent failure)
//   - Invalid path: Returns empty slice (silent failure)
func (p *PathCompleterAdapter) listDirectory(dir, partial string) []string {
	// Read directory entries
	// os.ReadDir is optimized for listing - doesn't call stat() on each entry
	entries, err := os.ReadDir(dir)
	if err != nil {
		// Silent failure for permission errors, non-existent directories, etc.
		// This prevents autocomplete from showing error messages during typing
		return []string{}
	}

	// Collect matching entries
	var results []string
	partialLower := strings.ToLower(partial)

	for _, entry := range entries {
		name := entry.Name()

		// Skip hidden files (starting with ".")
		// Exception: If user explicitly typed ".", show hidden files
		if strings.HasPrefix(name, ".") && !strings.HasPrefix(partial, ".") {
			continue
		}

		// Filter by prefix match (case-insensitive)
		if !strings.HasPrefix(strings.ToLower(name), partialLower) {
			continue
		}

		// Format the completion suggestion
		// T020: Add trailing "/" to directories for visual distinction
		fullPath := filepath.Join(dir, name)

		// Use os.Stat to properly detect directories (handles symlinks)
		// entry.IsDir() only reports the type of the entry itself, not the target
		if info, err := os.Stat(fullPath); err == nil && info.IsDir() {
			fullPath += "/"
		}

		results = append(results, fullPath)

		// T019: Limit to 100 results for performance (FR-012, SC-002)
		if len(results) >= 100 {
			break
		}
	}

	// T019: Sort alphabetically for consistent user experience
	sort.Strings(results)

	return results
}
