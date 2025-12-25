package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	binaryapp "github.com/b-harvest/devnet-builder/internal/application/binary"
	"github.com/b-harvest/devnet-builder/internal/di"
	"github.com/b-harvest/devnet-builder/internal/output"
	"github.com/spf13/cobra"
)

// createBinaryPassthroughCommand creates a dynamic command for passthrough to a plugin binary.
// This is used to create commands like "devnet-builder stabled status" which will
// execute the stabled binary with "status" as an argument.
//
// The command name is the binary name (e.g., "stabled", "gaiad").
// All arguments after the command name are passed directly to the binary.
//
// Example:
//
//	devnet-builder stabled status
//	-> Executes: <active-binary-path> status
//
//	devnet-builder stabled tx bank send ...
//	-> Executes: <active-binary-path> tx bank send ...
func createBinaryPassthroughCommand(binaryName, pluginName string) *cobra.Command {
	cmd := &cobra.Command{
		Use:                binaryName,
		Short:              fmt.Sprintf("Execute %s binary commands", binaryName),
		Long:               fmt.Sprintf("Pass through commands to the active %s binary for plugin %q", binaryName, pluginName),
		DisableFlagParsing: true, // Pass all flags to the binary
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeBinaryPassthrough(pluginName, args)
		},
	}

	return cmd
}

// executeBinaryPassthrough executes a binary passthrough command.
func executeBinaryPassthrough(pluginName string, args []string) error {
	ctx := context.Background()

	// Initialize container lazily
	container, err := InitContainerForCommand(pluginName, "local")
	if err != nil {
		return fmt.Errorf("failed to initialize: %w", err)
	}

	// Get the passthrough use case from container
	passthroughUC := container.PassthroughUseCase()

	// Prepare execute request
	req := binaryapp.ExecuteRequest{
		PluginName:  pluginName,
		Args:        args,
		WorkDir:     "",   // Use current directory
		Interactive: true, // Enable TTY for interactive commands
	}

	// Execute the binary
	resp, err := passthroughUC.Execute(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to execute binary: %w", err)
	}

	// Exit with the same code as the binary
	if resp.ExitCode != 0 {
		os.Exit(resp.ExitCode)
	}

	return nil
}

// enhanceRootWithBinaryPassthrough adds dynamic binary passthrough commands to the root command.
// This discovers available plugins, gets their binary names, and creates passthrough commands.
//
// Flow:
//  1. Discover available plugins
//  2. For each plugin, get its binary name
//  3. Create a dynamic command for that binary
//  4. Add to root command
//
// This enables commands like:
//
//	devnet-builder stabled status
//	devnet-builder aultd query bank balances ...
func enhanceRootWithBinaryPassthrough(rootCmd *cobra.Command, _ *di.Container) error {
	// Get plugin loader directly instead of using container
	loader := GetPluginLoader()
	if loader == nil {
		output.DefaultLogger.Debug("Plugin loader not initialized, skipping binary passthrough")
		return nil
	}

	// Discover available plugins
	plugins, err := loader.Discover()
	if err != nil {
		// Log warning but don't fail - binary passthrough is optional
		output.DefaultLogger.Warn("Failed to discover plugins for binary passthrough: %v", err)
		return nil
	}

	// Create passthrough commands for each plugin
	for _, pluginName := range plugins {
		// Load plugin to get binary name
		pluginClient, err := loader.Load(pluginName)
		if err != nil {
			output.DefaultLogger.Debug("Failed to load plugin %q: %v", pluginName, err)
			continue
		}

		binaryName := pluginClient.Module().BinaryName()
		if binaryName == "" {
			output.DefaultLogger.Debug("Plugin %q does not specify a binary name", pluginName)
			continue
		}

		// Check if command already exists (avoid conflicts)
		if hasCommand(rootCmd, binaryName) {
			output.DefaultLogger.Debug("Skipping binary passthrough for %q - command already exists", binaryName)
			continue
		}

		// Create and add the passthrough command
		passthroughCmd := createBinaryPassthroughCommand(binaryName, pluginName)
		rootCmd.AddCommand(passthroughCmd)

		output.DefaultLogger.Debug("Added binary passthrough command: %s (plugin: %s)", binaryName, pluginName)
	}

	return nil
}

// hasCommand checks if a command with the given name already exists.
func hasCommand(rootCmd *cobra.Command, name string) bool {
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == name || cmd.HasAlias(name) {
			return true
		}
	}
	return false
}

// parseBinaryNameFromArgs attempts to parse the binary name from command-line args.
// This is used as a fallback for unknown commands.
//
// Returns: binaryName, remainingArgs, found
func parseBinaryNameFromArgs(args []string) (string, []string, bool) {
	if len(args) == 0 {
		return "", nil, false
	}

	// The first argument might be a binary name
	potentialBinary := args[0]

	// Check if it looks like a binary name (ends with 'd' typically)
	// Common patterns: stabled, gaiad, osmosisd, aultd
	if !strings.HasSuffix(potentialBinary, "d") && !strings.HasSuffix(potentialBinary, "-plugin") {
		return "", nil, false
	}

	// Remove any path components
	binaryName := potentialBinary
	if strings.Contains(binaryName, "/") {
		return "", nil, false
	}

	return binaryName, args[1:], true
}

// handleUnknownCommand is called when Cobra encounters an unknown command.
// We use this to implement fallback binary passthrough.
//
// This enables:
//
//	devnet-builder stabled status
//
// Even if "stabled" wasn't discovered during initialization.
func handleUnknownCommand(rootCmd *cobra.Command, container *di.Container) {
	// Cobra doesn't have a built-in unknown command handler,
	// but we can use SilenceErrors and check args manually in main.go
	// This function serves as documentation for the pattern.

	// Implementation note: This should be called from main.go
	// when cmd.Execute() returns an "unknown command" error.
}
