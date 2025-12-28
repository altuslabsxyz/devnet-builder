package helpers

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// CommandResult captures the output and metadata from a command execution
type CommandResult struct {
	ExitCode int           // Command exit code
	Stdout   string        // Standard output
	Stderr   string        // Standard error
	Duration time.Duration // Execution duration
	Error    error         // Error if command failed to start
}

// Success returns true if the command exited with code 0
func (r *CommandResult) Success() bool {
	return r.ExitCode == 0
}

// Failed returns true if the command exited with non-zero code
func (r *CommandResult) Failed() bool {
	return r.ExitCode != 0
}

// CommandRunner executes devnet-builder commands with various options
type CommandRunner struct {
	ctx *TestContext
	t   *testing.T
}

// NewCommandRunner creates a new command runner for the given test context
func NewCommandRunner(t *testing.T, ctx *TestContext) *CommandRunner {
	return &CommandRunner{
		ctx: ctx,
		t:   t,
	}
}

// Run executes a devnet-builder command and returns the result
// Example: Run("deploy", "--validators", "4", "--mode", "docker")
func (r *CommandRunner) Run(args ...string) *CommandResult {
	r.t.Helper()

	// Use longer timeout for commands that build binaries from source
	// Deploy builds blockchain binaries which takes ~60s
	timeout := 30 * time.Second
	if len(args) > 0 && args[0] == "deploy" {
		timeout = 3 * time.Minute
	}

	return r.RunWithTimeout(timeout, args...)
}

// RunWithTimeout executes a command with a custom timeout
func (r *CommandRunner) RunWithTimeout(timeout time.Duration, args ...string) *CommandResult {
	r.t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return r.RunWithContext(ctx, args...)
}

// RunWithContext executes a command with a custom context
func (r *CommandRunner) RunWithContext(ctx context.Context, args ...string) *CommandResult {
	r.t.Helper()

	// Add test-specific flags to deploy commands
	// - --fork=false: Create fresh genesis instead of downloading from GitHub
	// - --no-interactive: Skip version selection prompts
	// - --export-version/--start-version: Provide versions for non-interactive mode
	// - --no-cache: Skip snapshot cache
	if len(args) > 0 && args[0] == "deploy" {
		hasFork := false
		hasNoCache := false
		hasNoInteractive := false
		hasExportVersion := false
		hasStartVersion := false

		for _, arg := range args {
			if strings.HasPrefix(arg, "--fork") {
				hasFork = true
			}
			if arg == "--no-cache" {
				hasNoCache = true
			}
			if arg == "--no-interactive" {
				hasNoInteractive = true
			}
			if strings.HasPrefix(arg, "--export-version") {
				hasExportVersion = true
			}
			if strings.HasPrefix(arg, "--start-version") {
				hasStartVersion = true
			}
		}

		// Add --fork=false to skip GitHub downloads entirely
		if !hasFork {
			args = append(args, "--fork=false")
		}
		// Add --no-cache for safety
		if !hasNoCache {
			args = append(args, "--no-cache")
		}
		// Add --no-interactive to skip prompts
		if !hasNoInteractive {
			args = append(args, "--no-interactive")
		}
		// Add version flags for non-interactive mode (use latest as default)
		if !hasExportVersion {
			args = append(args, "--export-version=latest")
		}
		if !hasStartVersion {
			args = append(args, "--start-version=latest")
		}
	}

	// Prepend --config flag if ConfigPath is set
	if r.ctx.ConfigPath != "" {
		args = append([]string{"--config", r.ctx.ConfigPath}, args...)
	}

	// Build command
	cmd := exec.CommandContext(ctx, r.ctx.BinaryPath, args...)
	cmd.Env = r.ctx.GetEnv()
	cmd.Dir = r.ctx.HomeDir

	// Capture stdout and stderr
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Execute command and measure duration
	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start)

	// Determine exit code
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			// Command failed to start
			return &CommandResult{
				ExitCode: -1,
				Stdout:   stdout.String(),
				Stderr:   stderr.String(),
				Duration: duration,
				Error:    err,
			}
		}
	}

	return &CommandResult{
		ExitCode: exitCode,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Duration: duration,
		Error:    nil,
	}
}

// MustRun executes a command and fails the test if it doesn't succeed
func (r *CommandRunner) MustRun(args ...string) *CommandResult {
	r.t.Helper()
	result := r.Run(args...)
	if !result.Success() {
		r.t.Fatalf("command failed: %s\nExit code: %d\nStdout: %s\nStderr: %s",
			strings.Join(args, " "), result.ExitCode, result.Stdout, result.Stderr)
	}
	return result
}

// MustFail executes a command and fails the test if it succeeds
func (r *CommandRunner) MustFail(args ...string) *CommandResult {
	r.t.Helper()
	result := r.Run(args...)
	if result.Success() {
		r.t.Fatalf("expected command to fail but it succeeded: %s\nStdout: %s\nStderr: %s",
			strings.Join(args, " "), result.Stdout, result.Stderr)
	}
	return result
}

// ExpectExitCode executes a command and asserts the exit code matches
func (r *CommandRunner) ExpectExitCode(expectedCode int, args ...string) *CommandResult {
	r.t.Helper()
	result := r.Run(args...)
	if result.ExitCode != expectedCode {
		r.t.Fatalf("expected exit code %d but got %d for command: %s\nStdout: %s\nStderr: %s",
			expectedCode, result.ExitCode, strings.Join(args, " "), result.Stdout, result.Stderr)
	}
	return result
}

// AssertStdoutContains asserts that stdout contains the expected string
func (r *CommandRunner) AssertStdoutContains(result *CommandResult, expected string) {
	r.t.Helper()
	if !strings.Contains(result.Stdout, expected) {
		r.t.Fatalf("expected stdout to contain %q but it didn't.\nStdout: %s\nStderr: %s",
			expected, result.Stdout, result.Stderr)
	}
}

// AssertStderrContains asserts that stderr contains the expected string
func (r *CommandRunner) AssertStderrContains(result *CommandResult, expected string) {
	r.t.Helper()
	if !strings.Contains(result.Stderr, expected) {
		r.t.Fatalf("expected stderr to contain %q but it didn't.\nStdout: %s\nStderr: %s",
			expected, result.Stdout, result.Stderr)
	}
}

// StartBackground starts a command in the background and returns the process
// Caller is responsible for stopping the process (registered as cleanup)
func (r *CommandRunner) StartBackground(args ...string) *exec.Cmd {
	r.t.Helper()

	cmd := exec.Command(r.ctx.BinaryPath, args...)
	cmd.Env = r.ctx.GetEnv()
	cmd.Dir = r.ctx.HomeDir

	if err := cmd.Start(); err != nil {
		r.t.Fatalf("failed to start background command: %v", err)
	}

	// Register cleanup to kill process if test fails
	r.ctx.AddCleanup(func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	})

	return cmd
}

// WaitForOutput polls command output until expected string appears or timeout
func (r *CommandRunner) WaitForOutput(timeout time.Duration, pollInterval time.Duration, args []string, expected string) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		result := r.Run(args...)
		if strings.Contains(result.Stdout, expected) || strings.Contains(result.Stderr, expected) {
			return nil
		}
		time.Sleep(pollInterval)
	}
	return fmt.Errorf("timeout waiting for output containing %q", expected)
}
