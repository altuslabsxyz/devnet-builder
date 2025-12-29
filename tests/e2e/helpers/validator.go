package helpers

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// StateValidator validates devnet state (filesystem, processes, Docker containers)
type StateValidator struct {
	t   *testing.T
	ctx *TestContext
}

// NewStateValidator creates a new state validator
func NewStateValidator(t *testing.T, ctx *TestContext) *StateValidator {
	return &StateValidator{
		t:   t,
		ctx: ctx,
	}
}

// AssertFileExists asserts that a file exists at the given path
func (v *StateValidator) AssertFileExists(subpath string) {
	v.t.Helper()
	path := filepath.Join(v.ctx.HomeDir, subpath)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		v.t.Fatalf("expected file to exist but it doesn't: %s", path)
	}
}

// AssertFileNotExists asserts that a file does not exist at the given path
func (v *StateValidator) AssertFileNotExists(subpath string) {
	v.t.Helper()
	path := filepath.Join(v.ctx.HomeDir, subpath)
	if _, err := os.Stat(path); err == nil {
		v.t.Fatalf("expected file to not exist but it does: %s", path)
	}
}

// AssertDirectoryExists asserts that a directory exists
// For validator directories, automatically checks under .devnet-builder/
func (v *StateValidator) AssertDirectoryExists(subpath string) {
	v.t.Helper()
	// If subpath starts with "validator", check under .devnet-builder/
	var path string
	if strings.HasPrefix(subpath, "validator") {
		path = filepath.Join(v.ctx.HomeDir, ".devnet-builder", subpath)
	} else {
		path = filepath.Join(v.ctx.HomeDir, subpath)
	}
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		v.t.Fatalf("expected directory to exist but it doesn't: %s", path)
	}
	if !info.IsDir() {
		v.t.Fatalf("expected %s to be a directory but it's a file", path)
	}
}

// AssertDirectoryNotExists asserts that a directory does not exist
func (v *StateValidator) AssertDirectoryNotExists(subpath string) {
	v.t.Helper()
	path := filepath.Join(v.ctx.HomeDir, subpath)
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		v.t.Fatalf("expected directory to not exist but it does: %s", path)
	}
}

// AssertFileContains asserts that a file contains the expected string
func (v *StateValidator) AssertFileContains(subpath string, expected string) {
	v.t.Helper()
	path := filepath.Join(v.ctx.HomeDir, subpath)
	content, err := os.ReadFile(path)
	if err != nil {
		v.t.Fatalf("failed to read file %s: %v", path, err)
	}
	if !strings.Contains(string(content), expected) {
		v.t.Fatalf("expected file %s to contain %q but it doesn't.\nContent: %s",
			path, expected, string(content))
	}
}

// AssertJSONFileValid asserts that a file contains valid JSON
func (v *StateValidator) AssertJSONFileValid(subpath string) {
	v.t.Helper()
	path := filepath.Join(v.ctx.HomeDir, subpath)
	content, err := os.ReadFile(path)
	if err != nil {
		v.t.Fatalf("failed to read file %s: %v", path, err)
	}
	var js json.RawMessage
	if err := json.Unmarshal(content, &js); err != nil {
		v.t.Fatalf("file %s contains invalid JSON: %v\nContent: %s", path, err, string(content))
	}
}

// AssertProcessRunning asserts that a process with the given PID is running
func (v *StateValidator) AssertProcessRunning(pid int) {
	v.t.Helper()
	process, err := os.FindProcess(pid)
	if err != nil {
		v.t.Fatalf("failed to find process %d: %v", pid, err)
	}
	// Send signal 0 to check if process exists
	if err := process.Signal(os.Signal(nil)); err != nil {
		v.t.Fatalf("process %d is not running: %v", pid, err)
	}
}

// AssertProcessNotRunning asserts that a process with the given PID is not running
func (v *StateValidator) AssertProcessNotRunning(pid int) {
	v.t.Helper()
	process, err := os.FindProcess(pid)
	if err != nil {
		// Process doesn't exist - success
		return
	}
	// Send signal 0 to check if process exists
	if err := process.Signal(os.Signal(nil)); err == nil {
		v.t.Fatalf("expected process %d to not be running but it is", pid)
	}
}

// AssertDockerContainerRunning asserts that a Docker container is running
func (v *StateValidator) AssertDockerContainerRunning(containerName string) {
	v.t.Helper()
	cmd := exec.Command("docker", "inspect", "-f", "{{.State.Running}}", containerName)
	output, err := cmd.Output()
	if err != nil {
		v.t.Fatalf("failed to inspect Docker container %s: %v", containerName, err)
	}
	running := strings.TrimSpace(string(output))
	if running != "true" {
		v.t.Fatalf("expected Docker container %s to be running but it's not", containerName)
	}
}

// AssertDockerContainerNotRunning asserts that a Docker container is not running
func (v *StateValidator) AssertDockerContainerNotRunning(containerName string) {
	v.t.Helper()
	cmd := exec.Command("docker", "inspect", "-f", "{{.State.Running}}", containerName)
	output, err := cmd.Output()
	if err != nil {
		// Container doesn't exist - success
		return
	}
	running := strings.TrimSpace(string(output))
	if running == "true" {
		v.t.Fatalf("expected Docker container %s to not be running but it is", containerName)
	}
}

// AssertDockerContainerExists asserts that a Docker container exists (running or stopped)
func (v *StateValidator) AssertDockerContainerExists(containerName string) {
	v.t.Helper()
	cmd := exec.Command("docker", "inspect", containerName)
	if err := cmd.Run(); err != nil {
		v.t.Fatalf("expected Docker container %s to exist but it doesn't", containerName)
	}
}

// AssertDockerContainerNotExists asserts that a Docker container does not exist
func (v *StateValidator) AssertDockerContainerNotExists(containerName string) {
	v.t.Helper()
	cmd := exec.Command("docker", "inspect", containerName)
	if err := cmd.Run(); err == nil {
		v.t.Fatalf("expected Docker container %s to not exist but it does", containerName)
	}
}

// AssertValidatorCount asserts the number of validator directories
func (v *StateValidator) AssertValidatorCount(expected int) {
	v.t.Helper()
	// Validators are created in .devnet-builder directory
	devnetDir := filepath.Join(v.ctx.HomeDir, ".devnet-builder")
	pattern := filepath.Join(devnetDir, "validator*")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		v.t.Fatalf("failed to glob validator directories: %v", err)
	}
	if len(matches) != expected {
		v.t.Fatalf("expected %d validator directories but found %d: %v",
			expected, len(matches), matches)
	}
}

// AssertPortListening asserts that a port is listening
func (v *StateValidator) AssertPortListening(port int) {
	v.t.Helper()
	// Use lsof to check if port is listening
	cmd := exec.Command("lsof", "-i", fmt.Sprintf(":%d", port), "-sTCP:LISTEN")
	if err := cmd.Run(); err != nil {
		v.t.Fatalf("expected port %d to be listening but it's not", port)
	}
}

// AssertPortNotListening asserts that a port is not listening
func (v *StateValidator) AssertPortNotListening(port int) {
	v.t.Helper()
	cmd := exec.Command("lsof", "-i", fmt.Sprintf(":%d", port), "-sTCP:LISTEN")
	if err := cmd.Run(); err == nil {
		v.t.Fatalf("expected port %d to not be listening but it is", port)
	}
}

// WaitForFile waits for a file to exist (with timeout)
func (v *StateValidator) WaitForFile(subpath string, timeout time.Duration) error {
	v.t.Helper()
	path := filepath.Join(v.ctx.HomeDir, subpath)
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for file %s to exist", path)
}

// WaitForProcess waits for a process to start (with timeout)
func (v *StateValidator) WaitForProcess(pidFile string, timeout time.Duration) (int, error) {
	v.t.Helper()
	path := filepath.Join(v.ctx.HomeDir, pidFile)
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			// PID file exists, read it
			content, err := os.ReadFile(path)
			if err != nil {
				time.Sleep(100 * time.Millisecond)
				continue
			}
			var pid int
			if _, err := fmt.Sscanf(string(content), "%d", &pid); err != nil {
				time.Sleep(100 * time.Millisecond)
				continue
			}
			// Verify process is running
			process, err := os.FindProcess(pid)
			if err != nil {
				time.Sleep(100 * time.Millisecond)
				continue
			}
			if err := process.Signal(os.Signal(nil)); err == nil {
				return pid, nil
			}
		}
		time.Sleep(100 * time.Millisecond)
	}

	return 0, fmt.Errorf("timeout waiting for process from PID file %s", pidFile)
}

// WaitForPortListening waits for a port to start listening (with timeout)
func (v *StateValidator) WaitForPortListening(port int, timeout time.Duration) error {
	v.t.Helper()
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		cmd := exec.Command("lsof", "-i", fmt.Sprintf(":%d", port), "-sTCP:LISTEN")
		if err := cmd.Run(); err == nil {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("timeout waiting for port %d to start listening", port)
}

// WaitForDockerContainer waits for a Docker container to be running (with timeout)
func (v *StateValidator) WaitForDockerContainer(containerName string, timeout time.Duration) error {
	v.t.Helper()
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		cmd := exec.Command("docker", "inspect", "-f", "{{.State.Running}}", containerName)
		output, err := cmd.Output()
		if err == nil && strings.TrimSpace(string(output)) == "true" {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("timeout waiting for Docker container %s to be running", containerName)
}

// GetValidatorDirs returns all validator directory paths
func (v *StateValidator) GetValidatorDirs() []string {
	v.t.Helper()
	// Validators are created in .devnet-builder directory
	devnetDir := filepath.Join(v.ctx.HomeDir, ".devnet-builder")
	pattern := filepath.Join(devnetDir, "validator*")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		v.t.Fatalf("failed to glob validator directories: %v", err)
	}
	return matches
}

// ReadPIDFile reads a PID from a PID file
func (v *StateValidator) ReadPIDFile(pidFile string) int {
	v.t.Helper()
	path := filepath.Join(v.ctx.HomeDir, pidFile)
	content, err := os.ReadFile(path)
	if err != nil {
		v.t.Fatalf("failed to read PID file %s: %v", path, err)
	}
	var pid int
	if _, err := fmt.Sscanf(string(content), "%d", &pid); err != nil {
		v.t.Fatalf("failed to parse PID from file %s: %v", path, err)
	}
	return pid
}
