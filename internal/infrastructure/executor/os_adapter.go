package executor

import (
	"context"
	"os/exec"
)

// OSCommandExecutor implements CommandExecutor using os/exec package.
// This is the production adapter that executes real system commands.
//
// Usage in production:
//
//	executor := executor.NewOSCommandExecutor()
//	output, err := executor.ExecuteWithTimeout(ctx, "stabled", "version")
//
// Usage in tests:
//
//	executor := &MockCommandExecutor{...}  // Test implementation
type OSCommandExecutor struct{}

// NewOSCommandExecutor creates a new command executor using the real OS exec package.
// This is the default implementation used in production code.
func NewOSCommandExecutor() *OSCommandExecutor {
	return &OSCommandExecutor{}
}

// ExecuteWithTimeout executes a command using exec.CommandContext.
//
// Implementation Notes:
//   - Uses CommandContext for automatic cancellation on timeout
//   - Returns combined stdout and stderr via CombinedOutput()
//   - Command is automatically killed when context is cancelled/times out
//
// Error Handling:
//   - Returns exec.ExitError if command exits with non-zero status
//   - Returns context.DeadlineExceeded if timeout occurs
//   - Returns exec.Error if command cannot be found/executed
func (e *OSCommandExecutor) ExecuteWithTimeout(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.CombinedOutput()
}
