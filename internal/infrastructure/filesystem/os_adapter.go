package filesystem

import (
	"io/fs"
	"os"
	"path/filepath"
)

// OSFileSystem implements FileSystem using the real operating system's filesystem.
// This is the production adapter that delegates to os and filepath packages.
//
// Usage in production:
//
//	fs := filesystem.NewOSFileSystem()
//	info, err := fs.Stat("/path/to/file")
//
// Usage in tests:
//
//	fs := &MockFileSystem{...}  // Test implementation
type OSFileSystem struct{}

// NewOSFileSystem creates a new filesystem adapter using the real OS filesystem.
// This is the default implementation used in production code.
func NewOSFileSystem() *OSFileSystem {
	return &OSFileSystem{}
}

// Stat returns file information using os.Stat.
func (OSFileSystem) Stat(name string) (fs.FileInfo, error) {
	return os.Stat(name)
}

// ReadDir reads directory contents using os.ReadDir.
func (OSFileSystem) ReadDir(name string) ([]fs.DirEntry, error) {
	return os.ReadDir(name)
}

// EvalSymlinks resolves symbolic links using filepath.EvalSymlinks.
func (OSFileSystem) EvalSymlinks(path string) (string, error) {
	return filepath.EvalSymlinks(path)
}
