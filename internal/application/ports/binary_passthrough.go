package ports

import (
	"context"
	"io"
)

// BinaryPassthroughCommand represents a command to be passed through to a plugin binary.
type BinaryPassthroughCommand struct {
	// PluginName is the name of the plugin whose binary will be used.
	PluginName string

	// Args are the arguments to pass to the binary.
	Args []string

	// WorkDir is the working directory for command execution.
	WorkDir string

	// Env contains additional environment variables.
	Env []string

	// Stdin is the input stream (optional).
	Stdin io.Reader

	// Stdout is the output stream (optional, defaults to os.Stdout).
	Stdout io.Writer

	// Stderr is the error stream (optional, defaults to os.Stderr).
	Stderr io.Writer
}

// BinaryResolver defines operations for resolving plugin binaries.
// This interface abstracts the logic of finding the correct binary path
// from a plugin name, whether from cache, custom paths, or other sources.
//
// SRP: Single responsibility of resolving binary paths.
// DIP: High-level use cases depend on this abstraction, not concrete implementations.
type BinaryResolver interface {
	// ResolveBinary resolves a plugin name to its binary path.
	// Returns the absolute path to the binary executable.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout
	//   - pluginName: Name of the plugin (e.g., "stable", "ault")
	//
	// Returns:
	//   - binaryPath: Absolute path to the binary executable
	//   - error: ErrPluginNotFound, ErrBinaryNotCached, or other resolution errors
	ResolveBinary(ctx context.Context, pluginName string) (binaryPath string, err error)

	// GetActiveBinary returns the path to the currently active binary.
	// The active binary is determined by the SetActive() call on BinaryCache.
	//
	// Returns:
	//   - binaryPath: Absolute path to the active binary
	//   - pluginName: Name of the plugin that owns this binary
	//   - error: Error if no active binary is set
	GetActiveBinary(ctx context.Context) (binaryPath string, pluginName string, err error)

	// GetBinaryName returns the binary name for a plugin.
	// This queries the plugin's Module interface to get the binary name.
	//
	// Parameters:
	//   - ctx: Context for cancellation
	//   - pluginName: Name of the plugin
	//
	// Returns:
	//   - binaryName: Name of the binary (e.g., "stabled", "gaia")
	//   - error: Error if plugin not found or cannot be loaded
	GetBinaryName(ctx context.Context, pluginName string) (binaryName string, err error)

	// ListAvailablePlugins returns names of all available plugins.
	//
	// Returns:
	//   - pluginNames: List of available plugin names
	//   - error: Error during plugin discovery
	ListAvailablePlugins(ctx context.Context) (pluginNames []string, err error)
}

// BinaryExecutor defines operations for executing plugin binaries.
// This interface abstracts the execution of external binaries with proper
// I/O handling and process management.
//
// SRP: Single responsibility of executing binary commands.
// DIP: High-level use cases depend on this abstraction, not concrete implementations.
type BinaryExecutor interface {
	// Execute runs a binary command and waits for it to complete.
	// The command's stdout/stderr are streamed to the provided writers.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout
	//   - cmd: Command configuration including binary path and arguments
	//
	// Returns:
	//   - exitCode: Exit code of the process (0 for success)
	//   - error: Error during execution (context cancellation, binary not found, etc.)
	Execute(ctx context.Context, cmd BinaryPassthroughCommand) (exitCode int, err error)

	// ExecuteInteractive runs a binary in interactive mode with TTY.
	// This is used for commands that require user interaction (e.g., prompts).
	//
	// Parameters:
	//   - ctx: Context for cancellation
	//   - cmd: Command configuration
	//
	// Returns:
	//   - exitCode: Exit code of the process
	//   - error: Error during execution
	ExecuteInteractive(ctx context.Context, cmd BinaryPassthroughCommand) (exitCode int, err error)
}

// PluginBinaryMapper defines operations for mapping plugin names to binary names.
// This is a lightweight interface focused on the mapping logic only.
//
// SRP: Single responsibility of plugin-to-binary name mapping.
// ISP: Small, focused interface that clients can depend on.
type PluginBinaryMapper interface {
	// GetBinaryName returns the binary name for a given plugin.
	//
	// Parameters:
	//   - ctx: Context for cancellation
	//   - pluginName: Name of the plugin
	//
	// Returns:
	//   - binaryName: Name of the binary executable
	//   - error: Error if plugin not found or cannot be loaded
	GetBinaryName(ctx context.Context, pluginName string) (binaryName string, err error)

	// GetPluginForBinary returns the plugin name that provides a binary.
	//
	// Parameters:
	//   - ctx: Context for cancellation
	//   - binaryName: Name of the binary to search for
	//
	// Returns:
	//   - pluginName: Name of the plugin that provides this binary
	//   - error: ErrPluginNotFound if no plugin provides this binary
	GetPluginForBinary(ctx context.Context, binaryName string) (pluginName string, err error)
}
