package ports

// PathCompleter provides autocomplete functionality for filesystem paths.
// This port follows the Dependency Inversion Principle - the application layer
// depends on this interface, while the infrastructure layer provides the implementation.
//
// Design Decision: Separate PathCompleter from FilesystemBrowser for Single Responsibility.
// PathCompleter handles completion logic, FilesystemBrowser handles UI interaction.
//
// Implementation Notes:
//   - Infrastructure adapter will use FileSystem port to read directories
//   - Completion logic should be fast (< 100ms for directories with < 1000 entries)
//   - Results are limited to 100 entries for performance in large directories
type PathCompleter interface {
	// Complete generates autocomplete suggestions for the given input path.
	//
	// Behavior:
	//  1. Parse input into directory + partial filename
	//     Example: "/Users/dev/my" → dir="/Users/dev/", partial="my"
	//  2. List directory contents
	//  3. Filter by prefix match on the partial filename
	//  4. Sort alphabetically
	//  5. Add trailing "/" to directories for visual distinction
	//  6. Limit to 100 results for performance
	//
	// Parameters:
	//   - input: Current user input (partial path)
	//
	// Returns:
	//   - []string: List of completion suggestions (max 100 entries)
	//     - Directories have trailing "/" (e.g., "/Users/dev/")
	//     - Files have no trailing slash (e.g., "/usr/bin/stabled")
	//     - Sorted alphabetically
	//     - Empty slice if directory doesn't exist or has no matches
	//
	// Edge Cases:
	//   - Empty input: Returns root directories (e.g., "/", "/Users/", "/tmp/")
	//   - Non-existent directory: Returns empty slice (no error, silent failure)
	//   - No matches: Returns empty slice
	//   - Permission denied: Returns empty slice (silent failure)
	//   - > 100 matches: Returns first 100 alphabetically
	//
	// Examples:
	//   - Input: "/" → Output: ["/bin/", "/etc/", "/home/", "/tmp/", "/usr/", "/var/"]
	//   - Input: "/Use" → Output: ["/Users/"]
	//   - Input: "/Users/" → Output: ["/Users/alice/", "/Users/bob/", "/Users/charlie/"]
	//   - Input: "/Users/a" → Output: ["/Users/alice/"]
	//   - Input: "/nonexistent/" → Output: []
	Complete(input string) []string
}
