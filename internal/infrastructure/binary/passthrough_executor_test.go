package binary

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/b-harvest/devnet-builder/internal/application/ports"
)

func TestPassthroughExecutor_Execute_Success(t *testing.T) {
	executor := NewPassthroughExecutor()

	// Use echo command which is available on all systems
	var stdout bytes.Buffer
	cmd := ports.BinaryPassthroughCommand{
		PluginName: "echo",
		Args:       []string{"hello", "world"},
		Stdout:     &stdout,
	}

	exitCode, err := executor.Execute(context.Background(), cmd)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}

	output := strings.TrimSpace(stdout.String())
	if output != "hello world" {
		t.Errorf("Expected 'hello world', got '%s'", output)
	}
}

func TestPassthroughExecutor_Execute_NonZeroExitCode(t *testing.T) {
	executor := NewPassthroughExecutor()

	// Create a temporary script that exits with code 42
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "test-script.sh")
	scriptContent := "#!/bin/sh\nexit 42\n"
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("Failed to create test script: %v", err)
	}

	cmd := ports.BinaryPassthroughCommand{
		PluginName: scriptPath,
		Args:       []string{},
	}

	exitCode, err := executor.Execute(context.Background(), cmd)
	if err != nil {
		t.Fatalf("Expected no error for non-zero exit, got: %v", err)
	}

	if exitCode != 42 {
		t.Errorf("Expected exit code 42, got %d", exitCode)
	}
}

func TestPassthroughExecutor_Execute_EmptyPluginName(t *testing.T) {
	executor := NewPassthroughExecutor()

	cmd := ports.BinaryPassthroughCommand{
		PluginName: "",
		Args:       []string{},
	}

	_, err := executor.Execute(context.Background(), cmd)
	if err == nil {
		t.Fatal("Expected error for empty plugin name, got nil")
	}
}

func TestPassthroughExecutor_Execute_CommandNotFound(t *testing.T) {
	executor := NewPassthroughExecutor()

	cmd := ports.BinaryPassthroughCommand{
		PluginName: "nonexistent-command-12345",
		Args:       []string{},
	}

	_, err := executor.Execute(context.Background(), cmd)
	if err == nil {
		t.Fatal("Expected error for nonexistent command, got nil")
	}
}

func TestPassthroughExecutor_Execute_WithWorkDir(t *testing.T) {
	executor := NewPassthroughExecutor()

	tmpDir := t.TempDir()
	var stdout bytes.Buffer

	cmd := ports.BinaryPassthroughCommand{
		PluginName: "pwd",
		Args:       []string{},
		WorkDir:    tmpDir,
		Stdout:     &stdout,
	}

	exitCode, err := executor.Execute(context.Background(), cmd)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}

	output := strings.TrimSpace(stdout.String())
	// macOS may add /private prefix to temp directories
	expectedDirs := []string{tmpDir, "/private" + tmpDir}
	matched := false
	for _, expected := range expectedDirs {
		if output == expected {
			matched = true
			break
		}
	}
	if !matched {
		t.Errorf("Expected working directory to be %s or /private%s, got %s", tmpDir, tmpDir, output)
	}
}

func TestPassthroughExecutor_Execute_WithEnv(t *testing.T) {
	executor := NewPassthroughExecutor()

	var stdout bytes.Buffer
	cmd := ports.BinaryPassthroughCommand{
		PluginName: "sh",
		Args:       []string{"-c", "echo $TEST_VAR"},
		Env:        []string{"TEST_VAR=test_value"},
		Stdout:     &stdout,
	}

	exitCode, err := executor.Execute(context.Background(), cmd)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}

	output := strings.TrimSpace(stdout.String())
	if output != "test_value" {
		t.Errorf("Expected 'test_value', got '%s'", output)
	}
}

func TestPassthroughExecutor_Execute_ContextCancellation(t *testing.T) {
	executor := NewPassthroughExecutor()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	cmd := ports.BinaryPassthroughCommand{
		PluginName: "sleep",
		Args:       []string{"10"},
	}

	_, err := executor.Execute(ctx, cmd)
	if err == nil {
		t.Fatal("Expected context cancellation error, got nil")
	}

	// The error should be related to context cancellation
	if !strings.Contains(err.Error(), "context") && err.Error() != "signal: killed" {
		t.Errorf("Expected context-related error, got: %v", err)
	}
}

func TestPassthroughExecutor_Execute_StderrCapture(t *testing.T) {
	executor := NewPassthroughExecutor()

	var stderr bytes.Buffer
	cmd := ports.BinaryPassthroughCommand{
		PluginName: "sh",
		Args:       []string{"-c", "echo error_message >&2"},
		Stderr:     &stderr,
	}

	exitCode, err := executor.Execute(context.Background(), cmd)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}

	output := strings.TrimSpace(stderr.String())
	if output != "error_message" {
		t.Errorf("Expected 'error_message' on stderr, got '%s'", output)
	}
}

func TestPassthroughExecutor_ExecuteInteractive_Success(t *testing.T) {
	executor := NewPassthroughExecutor()

	// Create a simple script that doesn't require actual interaction
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "test-script.sh")
	scriptContent := "#!/bin/sh\necho 'interactive test'\n"
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("Failed to create test script: %v", err)
	}

	cmd := ports.BinaryPassthroughCommand{
		PluginName: scriptPath,
		Args:       []string{},
	}

	exitCode, err := executor.ExecuteInteractive(context.Background(), cmd)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}
}

func TestPassthroughExecutor_ExecuteInteractive_EmptyPluginName(t *testing.T) {
	executor := NewPassthroughExecutor()

	cmd := ports.BinaryPassthroughCommand{
		PluginName: "",
		Args:       []string{},
	}

	_, err := executor.ExecuteInteractive(context.Background(), cmd)
	if err == nil {
		t.Fatal("Expected error for empty plugin name, got nil")
	}
}

func TestPassthroughExecutor_ExecuteWithOutput(t *testing.T) {
	executor := NewPassthroughExecutor()

	cmd := ports.BinaryPassthroughCommand{
		PluginName: "echo",
		Args:       []string{"test output"},
	}

	stdout, stderr, exitCode, err := executor.ExecuteWithOutput(context.Background(), cmd)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}

	if !strings.Contains(stdout, "test output") {
		t.Errorf("Expected stdout to contain 'test output', got '%s'", stdout)
	}

	if stderr != "" {
		t.Errorf("Expected empty stderr, got '%s'", stderr)
	}
}

func TestPassthroughExecutor_ExecuteWithOutput_Stderr(t *testing.T) {
	executor := NewPassthroughExecutor()

	cmd := ports.BinaryPassthroughCommand{
		PluginName: "sh",
		Args:       []string{"-c", "echo stdout_msg; echo stderr_msg >&2"},
	}

	stdout, stderr, exitCode, err := executor.ExecuteWithOutput(context.Background(), cmd)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}

	if !strings.Contains(stdout, "stdout_msg") {
		t.Errorf("Expected stdout to contain 'stdout_msg', got '%s'", stdout)
	}

	if !strings.Contains(stderr, "stderr_msg") {
		t.Errorf("Expected stderr to contain 'stderr_msg', got '%s'", stderr)
	}
}

// Integration test: Test with actual binary execution
func TestPassthroughExecutor_Integration_RealCommand(t *testing.T) {
	// Skip if running in environments without ls command
	if _, err := exec.LookPath("ls"); err != nil {
		t.Skip("ls command not available, skipping integration test")
	}

	executor := NewPassthroughExecutor()

	var stdout bytes.Buffer
	cmd := ports.BinaryPassthroughCommand{
		PluginName: "ls",
		Args:       []string{"-la"},
		WorkDir:    os.TempDir(),
		Stdout:     &stdout,
	}

	exitCode, err := executor.Execute(context.Background(), cmd)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}

	output := stdout.String()
	if len(output) == 0 {
		t.Error("Expected some output from ls command, got empty string")
	}
}
