package helpers

import "path/filepath"

// ResolveBinaryPath returns the binary path based on priority:
// 1. customPath if non-empty
// 2. filepath.Join(homeDir, "bin", "stabled") otherwise
//
// This function does not check if the file exists - that is the caller's responsibility.
// An empty customPath means "use default".
func ResolveBinaryPath(customPath, homeDir string) string {
	if customPath != "" {
		return customPath
	}
	return filepath.Join(homeDir, "bin", "stabled")
}

// ResolveDockerImage returns the docker image based on priority:
// 1. customImage if non-empty
// 2. defaultImage otherwise
//
// This function is a simple fallback helper for docker image selection.
func ResolveDockerImage(customImage, defaultImage string) string {
	if customImage != "" {
		return customImage
	}
	return defaultImage
}
