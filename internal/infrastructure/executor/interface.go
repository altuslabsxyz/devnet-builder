package executor

import (
	"context"
	"time"
)

// CommandExecutor abstracts command execution for testing.
// This interface allows the infrastructure layer to execute system commands
// while remaining testable through mocking.
//
// Design Decision: Minimal interface focused on our use case (version detection)
// rather than exposing full exec.Cmd capabilities.
//
// Security Note: Caller is responsible for validating and sanitizing command arguments
// to prevent command injection vulnerabilities.
type CommandExecutor interface {
	// ExecuteWithTimeout executes a command with a timeout.
	//
	// Parameters:
	//   - ctx: Context for cancellation (timeout should be set via context.WithTimeout)
	//   - name: Command name or path (e.g., "stabled", "/path/to/binary")
	//   - args: Command arguments (e.g., "version", "--format", "json")
	//
	// Returns:
	//   - Combined stdout and stderr output as bytes
	//   - Error if command fails, times out, or cannot be executed
	//
	// Timeout Behavior:
	//   - Command is killed if context deadline is exceeded
	//   - Returns context.DeadlineExceeded error on timeout
	//
	// Example:
	//   ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	//   defer cancel()
	//   output, err := executor.ExecuteWithTimeout(ctx, "stabled", "version")
	ExecuteWithTimeout(ctx context.Context, name string, args ...string) ([]byte, error)
}

// DefaultTimeout is the recommended timeout for version detection commands.
// This matches the spec requirement (FR-006) of 5 seconds per binary.
const DefaultTimeout = 5 * time.Second
