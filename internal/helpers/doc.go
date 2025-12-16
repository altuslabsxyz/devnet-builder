// Package helpers provides shared utility functions for the devnet-builder CLI.
//
// This package is designed to consolidate common patterns used across cmd/ files,
// reducing code duplication and ensuring consistent behavior. The helpers package
// follows these design principles:
//
//  1. No Internal Dependencies: helpers does not import other internal packages
//     to prevent circular imports. Required values are passed as parameters.
//
//  2. Parameter Injection: Functions accept callbacks or values rather than
//     importing domain-specific packages directly.
//
//  3. Thread Safety: All functions are safe for concurrent use.
//
//  4. Error Preservation: Error messages maintain context for debugging.
//
// The package provides utilities in these areas:
//
//   - File I/O: LoadJSON, SaveJSON, FileExists, DirExists, EnsureDir
//   - Binary Path: ResolveBinaryPath, ResolveDockerImage
//   - Devnet Loading: DevnetLoader for loading devnet with validation
//
// Usage example:
//
//	// Load JSON configuration
//	config, err := helpers.LoadJSON[Config]("/path/to/config.json")
//	if err != nil {
//	    return err
//	}
//
//	// Resolve binary path with fallback
//	binaryPath := helpers.ResolveBinaryPath(customPath, homeDir)
package helpers
