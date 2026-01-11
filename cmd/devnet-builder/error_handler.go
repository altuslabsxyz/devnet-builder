package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/b-harvest/devnet-builder/internal/domain/common"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

// =============================================================================
// Error Handler (Clean Architecture - Presentation Layer)
// =============================================================================
// This file provides centralized error handling for CLI commands.
// It follows the Clean Architecture principle by keeping presentation logic
// (how to display errors) separate from business logic (what the errors mean).

// handleCommandError handles errors from CLI commands in a user-friendly way.
// It checks for domain error interfaces and adjusts Cobra behavior accordingly.
//
// Usage in command handlers:
//
//	func runMyCommand(cmd *cobra.Command, args []string) error {
//	    result, err := doSomething()
//	    if err != nil {
//	        return handleCommandError(cmd, err)
//	    }
//	    return nil
//	}
func handleCommandError(cmd *cobra.Command, err error) error {
	if err == nil {
		return nil
	}

	// Check if this error should silence usage output
	// (e.g., authentication errors, network errors, etc.)
	if common.ShouldSilenceUsage(err) {
		cmd.SilenceUsage = true
	}

	// Get user-friendly message if available
	userMessage := common.GetUserMessage(err)

	// Print the error with proper formatting
	fmt.Fprintf(os.Stderr, "Error: %s\n", userMessage)

	// Check for recovery hints
	if hint := common.GetRecoveryHint(err); hint != "" {
		fmt.Fprintf(os.Stderr, "\nHint: %s\n", hint)
	}

	// Silence Cobra's error printing since we already printed it
	cmd.SilenceErrors = true

	// Return a simple error to indicate failure (exit code 1)
	// The actual error message was already printed above
	return fmt.Errorf("") // Empty error message since we already printed it
}

// wrapInteractiveError wraps errors from interactive operations.
// It preserves cancellation handling while applying error formatting.
// Returns exit code 130 for user cancellation (Ctrl+C/ESC), exit code 1 for other errors.
func wrapInteractiveError(cmd *cobra.Command, err error, context string) error {
	if err == nil {
		return nil
	}

	// Check for user cancellation (T014: EC-005)
	// User cancellation should exit with code 130 (standard UNIX convention)
	if isCancellation(err) {
		cmd.SilenceUsage = true
		cmd.SilenceErrors = true
		// Exit with code 130 (128 + 2 for SIGINT)
		os.Exit(130)
		return nil // Unreachable, but required for type safety
	}

	// Add context to the error if provided
	if context != "" {
		wrappedErr := fmt.Errorf("%s: %w", context, err)
		return handleCommandError(cmd, wrappedErr)
	}

	return handleCommandError(cmd, err)
}

// isCancellation checks if an error represents a user cancellation.
// This implements EC-005 (User Cancellation edge case) from the specification.
//
// User Cancellation Detection:
//   - promptui.ErrInterrupt: Ctrl+C pressed during prompt
//   - promptui.ErrEOF: ESC pressed during prompt
//   - Error message contains "exit code 130": Wrapped cancellation error
//   - Error message contains "cancelled": Generic cancellation
//
// Returns:
//   - true if error is user cancellation (should exit with code 130)
//   - false for all other errors (should exit with code 1)
func isCancellation(err error) bool {
	if err == nil {
		return false
	}

	// Check for promptui cancellation errors directly (US1: T014)
	if err == promptui.ErrInterrupt || err == promptui.ErrEOF {
		return true
	}

	// Check error message for cancellation indicators
	errMsg := strings.ToLower(err.Error())
	return strings.Contains(errMsg, "exit code 130") ||
		strings.Contains(errMsg, "cancelled") ||
		strings.Contains(errMsg, "user cancelled") ||
		strings.Contains(errMsg, "operation cancelled") ||
		strings.Contains(errMsg, "^c") ||
		strings.Contains(errMsg, "interrupt")
}
