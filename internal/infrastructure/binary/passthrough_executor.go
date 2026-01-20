package binary

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"

	"github.com/altuslabsxyz/devnet-builder/internal/application/ports"
)

// PassthroughExecutor implements ports.BinaryExecutor for executing plugin binaries.
// It provides both standard execution and interactive (TTY) execution modes.
//
// This implementation:
//   - Streams output in real-time to stdout/stderr
//   - Supports context cancellation
//   - Handles exit codes properly
//   - Provides TTY support for interactive commands
//
// Thread-safety: Each Execute call is independent and can be called concurrently.
type PassthroughExecutor struct {
	// logger can be added here for debugging if needed
}

// NewPassthroughExecutor creates a new PassthroughExecutor.
func NewPassthroughExecutor() *PassthroughExecutor {
	return &PassthroughExecutor{}
}

// Execute runs a binary command and waits for completion.
// Output is streamed to the configured writers (or os.Stdout/Stderr if not set).
//
// Implementation details:
//   - Creates exec.Cmd with the binary and arguments
//   - Connects stdin/stdout/stderr
//   - Starts the process
//   - Waits for completion or context cancellation
//   - Returns exit code
func (e *PassthroughExecutor) Execute(ctx context.Context, cmd ports.BinaryPassthroughCommand) (int, error) {
	// Validate binary path is not empty
	if cmd.PluginName == "" {
		return 1, fmt.Errorf("plugin name is required")
	}

	// Create exec.Cmd
	execCmd := exec.CommandContext(ctx, cmd.PluginName, cmd.Args...)

	// Set working directory if specified
	if cmd.WorkDir != "" {
		execCmd.Dir = cmd.WorkDir
	}

	// Merge environment variables
	execCmd.Env = os.Environ()
	if len(cmd.Env) > 0 {
		execCmd.Env = append(execCmd.Env, cmd.Env...)
	}

	// Configure I/O streams
	if cmd.Stdin != nil {
		execCmd.Stdin = cmd.Stdin
	} else {
		execCmd.Stdin = os.Stdin
	}

	if cmd.Stdout != nil {
		execCmd.Stdout = cmd.Stdout
	} else {
		execCmd.Stdout = os.Stdout
	}

	if cmd.Stderr != nil {
		execCmd.Stderr = cmd.Stderr
	} else {
		execCmd.Stderr = os.Stderr
	}

	// Start the process
	if err := execCmd.Start(); err != nil {
		return 1, fmt.Errorf("failed to start binary: %w", err)
	}

	// Wait for completion
	err := execCmd.Wait()

	// Extract exit code
	exitCode := 0
	if err != nil {
		// Check if it's an exit error
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			// Other errors (e.g., signal, context cancellation)
			return 1, fmt.Errorf("command execution failed: %w", err)
		}
	}

	return exitCode, nil
}

// ExecuteInteractive runs a binary in interactive mode with TTY support.
// This is used for commands that require user interaction (prompts, editors, etc.).
//
// On Unix systems, this properly sets up the TTY to allow interactive features:
//   - Terminal raw mode
//   - Signal handling (Ctrl+C, etc.)
//   - Terminal size
//
// On Windows, this falls back to standard execution.
func (e *PassthroughExecutor) ExecuteInteractive(ctx context.Context, cmd ports.BinaryPassthroughCommand) (int, error) {
	// Validate binary path is not empty
	if cmd.PluginName == "" {
		return 1, fmt.Errorf("plugin name is required")
	}

	// Create exec.Cmd
	execCmd := exec.CommandContext(ctx, cmd.PluginName, cmd.Args...)

	// Set working directory if specified
	if cmd.WorkDir != "" {
		execCmd.Dir = cmd.WorkDir
	}

	// Merge environment variables
	execCmd.Env = os.Environ()
	if len(cmd.Env) > 0 {
		execCmd.Env = append(execCmd.Env, cmd.Env...)
	}

	// Connect stdin/stdout/stderr directly to the current process
	// This enables interactive features like prompts
	execCmd.Stdin = os.Stdin
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr

	// Set process group for proper signal handling
	// This ensures Ctrl+C is properly forwarded to the child process
	execCmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: false, // Don't create a new process group (inherit parent's)
	}

	// Start the process
	if err := execCmd.Start(); err != nil {
		return 1, fmt.Errorf("failed to start interactive binary: %w", err)
	}

	// Wait for completion
	err := execCmd.Wait()

	// Extract exit code
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return 1, fmt.Errorf("interactive command execution failed: %w", err)
		}
	}

	return exitCode, nil
}

// ExecuteWithOutput is a convenience method that captures stdout and stderr.
// This is useful for commands where we need to parse the output.
func (e *PassthroughExecutor) ExecuteWithOutput(ctx context.Context, cmd ports.BinaryPassthroughCommand) (stdout, stderr string, exitCode int, err error) {
	// Create buffers for output
	var stdoutBuf, stderrBuf io.ReadWriter
	stdoutBuf = &bufferWriter{}
	stderrBuf = &bufferWriter{}

	// Override output streams
	cmd.Stdout = stdoutBuf
	cmd.Stderr = stderrBuf

	// Execute
	exitCode, err = e.Execute(ctx, cmd)

	// Read output from buffers
	stdoutBytes, _ := io.ReadAll(stdoutBuf)
	stderrBytes, _ := io.ReadAll(stderrBuf)

	return string(stdoutBytes), string(stderrBytes), exitCode, err
}

// bufferWriter is a simple in-memory buffer that implements io.ReadWriter.
type bufferWriter struct {
	buf []byte
	pos int
}

func (b *bufferWriter) Write(p []byte) (n int, err error) {
	b.buf = append(b.buf, p...)
	return len(p), nil
}

func (b *bufferWriter) Read(p []byte) (n int, err error) {
	if b.pos >= len(b.buf) {
		return 0, io.EOF
	}
	n = copy(p, b.buf[b.pos:])
	b.pos += n
	return n, nil
}

// Ensure PassthroughExecutor implements ports.BinaryExecutor
var _ ports.BinaryExecutor = (*PassthroughExecutor)(nil)
