package filesystem

import (
	"io/fs"
)

// FileSystem abstracts filesystem operations for testing.
// This interface follows the Dependency Inversion Principle (DIP) by allowing
// the infrastructure layer to be mocked during unit tests while using the real
// filesystem in production.
//
// Design Decision: Minimal interface with only the methods needed for binary scanning
// and validation, avoiding over-engineering while maintaining testability.
type FileSystem interface {
	// Stat returns file information for the given path.
	// Equivalent to os.Stat().
	//
	// Returns:
	//   - fs.FileInfo containing size, mode, modification time
	//   - fs.ErrNotExist if file doesn't exist
	//   - fs.ErrPermission if access is denied
	Stat(name string) (fs.FileInfo, error)

	// ReadDir reads the directory named by dirname and returns
	// a list of directory entries sorted by filename.
	// Equivalent to os.ReadDir().
	//
	// Returns:
	//   - Slice of fs.DirEntry for all files/directories
	//   - fs.ErrNotExist if directory doesn't exist
	//   - fs.ErrPermission if access is denied
	ReadDir(name string) ([]fs.DirEntry, error)

	// EvalSymlinks returns the path name after the evaluation of any symbolic links.
	// Equivalent to filepath.EvalSymlinks().
	//
	// If the path is absolute, the result will be absolute.
	// If the path is relative, the result will be relative to the current directory.
	//
	// Returns:
	//   - Resolved absolute or relative path
	//   - Error if path doesn't exist or evaluation fails
	EvalSymlinks(path string) (string, error)
}
