package binary

import (
	"context"
	"fmt"
	"os"

	"github.com/b-harvest/devnet-builder/internal/application/ports"
)

// PassthroughUseCase orchestrates the binary passthrough flow.
// It coordinates between BinaryResolver and BinaryExecutor to:
//  1. Resolve the plugin name to a binary path
//  2. Execute the binary with provided arguments
//  3. Handle errors and return exit codes
//
// This use case follows Clean Architecture principles:
//   - Depends only on abstractions (ports interfaces)
//   - Contains core business logic
//   - Independent of infrastructure details
//
// SOLID Compliance:
//   - SRP: Single responsibility of orchestrating binary passthrough
//   - DIP: Depends on abstractions (BinaryResolver, BinaryExecutor)
//   - OCP: Can be extended without modification (e.g., add logging, metrics)
type PassthroughUseCase struct {
	resolver ports.BinaryResolver
	executor ports.BinaryExecutor
}

// NewPassthroughUseCase creates a new PassthroughUseCase.
//
// Parameters:
//   - resolver: Resolver for finding plugin binaries
//   - executor: Executor for running binaries
//
// Returns:
//   - *PassthroughUseCase: Configured use case instance
func NewPassthroughUseCase(
	resolver ports.BinaryResolver,
	executor ports.BinaryExecutor,
) *PassthroughUseCase {
	return &PassthroughUseCase{
		resolver: resolver,
		executor: executor,
	}
}

// ExecuteRequest contains the parameters for executing a binary passthrough.
type ExecuteRequest struct {
	// PluginName is the name of the plugin whose binary will be used.
	// If empty, the active plugin binary will be used.
	PluginName string

	// Args are the arguments to pass to the binary.
	Args []string

	// WorkDir is the working directory for execution.
	// If empty, uses the current working directory.
	WorkDir string

	// Interactive indicates whether to run in interactive mode (with TTY).
	Interactive bool
}

// ExecuteResponse contains the result of a binary passthrough execution.
type ExecuteResponse struct {
	// ExitCode is the exit code returned by the binary.
	ExitCode int

	// PluginName is the name of the plugin that was executed.
	PluginName string

	// BinaryPath is the path to the binary that was executed.
	BinaryPath string
}

// Execute performs the binary passthrough operation.
//
// Flow:
//  1. Validate request
//  2. Resolve plugin name to binary path
//  3. Create passthrough command
//  4. Execute the binary
//  5. Return result with exit code
//
// Error Handling:
//   - Returns error if plugin not found
//   - Returns error if binary not cached/available
//   - Returns error if binary execution fails to start
//   - Returns exit code if binary runs but returns non-zero
func (uc *PassthroughUseCase) Execute(ctx context.Context, req ExecuteRequest) (*ExecuteResponse, error) {
	// Validate request
	if err := uc.validateRequest(req); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	// Resolve binary path
	binaryPath, pluginName, err := uc.resolveBinary(ctx, req.PluginName)
	if err != nil {
		return nil, err
	}

	// Prepare passthrough command
	passthroughCmd := ports.BinaryPassthroughCommand{
		PluginName: binaryPath, // Use the resolved binary path
		Args:       req.Args,
		WorkDir:    req.WorkDir,
		Stdin:      os.Stdin,
		Stdout:     os.Stdout,
		Stderr:     os.Stderr,
	}

	// Execute binary
	var exitCode int
	if req.Interactive {
		exitCode, err = uc.executor.ExecuteInteractive(ctx, passthroughCmd)
	} else {
		exitCode, err = uc.executor.Execute(ctx, passthroughCmd)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to execute binary: %w", err)
	}

	// Return response
	return &ExecuteResponse{
		ExitCode:   exitCode,
		PluginName: pluginName,
		BinaryPath: binaryPath,
	}, nil
}

// GetBinaryName returns the binary name for a plugin.
// This is useful for command completion and help text.
func (uc *PassthroughUseCase) GetBinaryName(ctx context.Context, pluginName string) (string, error) {
	if pluginName == "" {
		return "", fmt.Errorf("plugin name is required")
	}

	binaryName, err := uc.resolver.GetBinaryName(ctx, pluginName)
	if err != nil {
		return "", fmt.Errorf("failed to get binary name for plugin %q: %w", pluginName, err)
	}

	return binaryName, nil
}

// ListAvailablePlugins returns all plugins that have binaries available.
// This is useful for command completion and help text.
func (uc *PassthroughUseCase) ListAvailablePlugins(ctx context.Context) ([]string, error) {
	plugins, err := uc.resolver.ListAvailablePlugins(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list available plugins: %w", err)
	}

	return plugins, nil
}

// validateRequest validates the execute request.
func (uc *PassthroughUseCase) validateRequest(req ExecuteRequest) error {
	// No args is technically valid (e.g., just running "stabled" shows help)
	// So we don't validate args

	// WorkDir will default to current directory if empty
	// So we don't need to validate it here

	return nil
}

// resolveBinary resolves the plugin name to a binary path.
// If pluginName is empty, uses the active binary.
func (uc *PassthroughUseCase) resolveBinary(ctx context.Context, pluginName string) (binaryPath, resolvedPluginName string, err error) {
	if pluginName == "" {
		// Use active binary
		binaryPath, resolvedPluginName, err = uc.resolver.GetActiveBinary(ctx)
		if err != nil {
			return "", "", fmt.Errorf("failed to get active binary: %w (hint: set active binary with 'devnet-builder cache set-active <version>')", err)
		}
		return binaryPath, resolvedPluginName, nil
	}

	// Resolve specific plugin
	binaryPath, err = uc.resolver.ResolveBinary(ctx, pluginName)
	if err != nil {
		return "", "", fmt.Errorf("failed to resolve binary for plugin %q: %w", pluginName, err)
	}

	return binaryPath, pluginName, nil
}
