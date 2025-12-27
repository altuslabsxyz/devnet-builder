package helpers

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
)

// CleanupManager orchestrates resource cleanup for tests
type CleanupManager struct {
	t          *testing.T
	ctx        *TestContext
	containers []testcontainers.Container // Docker containers to cleanup
	processes  []*os.Process              // Background processes to cleanup
	dirs       []string                   // Directories to cleanup
}

// NewCleanupManager creates a new cleanup manager
func NewCleanupManager(t *testing.T, ctx *TestContext) *CleanupManager {
	cm := &CleanupManager{
		t:          t,
		ctx:        ctx,
		containers: make([]testcontainers.Container, 0),
		processes:  make([]*os.Process, 0),
		dirs:       make([]string, 0),
	}

	// Register cleanup to run on test completion
	t.Cleanup(func() {
		cm.Cleanup()
	})

	return cm
}

// TrackContainer registers a Docker container for cleanup
func (cm *CleanupManager) TrackContainer(container testcontainers.Container) {
	cm.containers = append(cm.containers, container)
}

// TrackProcess registers a process for cleanup
func (cm *CleanupManager) TrackProcess(process *os.Process) {
	cm.processes = append(cm.processes, process)
}

// TrackDirectory registers a directory for cleanup
func (cm *CleanupManager) TrackDirectory(dir string) {
	cm.dirs = append(cm.dirs, dir)
}

// Cleanup performs cleanup of all tracked resources
// Resources are cleaned up in order: containers → processes → directories
func (cm *CleanupManager) Cleanup() {
	cm.t.Helper()

	// Stop Docker containers
	for i, container := range cm.containers {
		if container == nil {
			continue
		}
		cm.t.Logf("Stopping container %d/%d", i+1, len(cm.containers))
		if err := cm.stopContainer(container); err != nil {
			cm.t.Logf("WARNING: failed to stop container: %v", err)
		}
	}

	// Kill processes
	for i, process := range cm.processes {
		if process == nil {
			continue
		}
		cm.t.Logf("Killing process %d/%d (PID: %d)", i+1, len(cm.processes), process.Pid)
		if err := cm.killProcess(process); err != nil {
			cm.t.Logf("WARNING: failed to kill process: %v", err)
		}
	}

	// Remove directories
	for i, dir := range cm.dirs {
		cm.t.Logf("Removing directory %d/%d: %s", i+1, len(cm.dirs), dir)
		if err := os.RemoveAll(dir); err != nil {
			cm.t.Logf("WARNING: failed to remove directory %s: %v", dir, err)
		}
	}

	cm.t.Log("Cleanup completed")
}

// stopContainer stops and removes a Docker container with timeout
func (cm *CleanupManager) stopContainer(container testcontainers.Container) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Try graceful stop first (10 second timeout)
	stopTimeout := 10 * time.Second
	if err := container.Stop(ctx, &stopTimeout); err != nil {
		cm.t.Logf("WARNING: graceful stop failed, will force terminate: %v", err)
	}

	// Terminate container
	if err := container.Terminate(ctx); err != nil {
		return fmt.Errorf("failed to terminate container: %w", err)
	}

	return nil
}

// killProcess kills a process and waits for it to exit
func (cm *CleanupManager) killProcess(process *os.Process) error {
	// Send SIGTERM first
	if err := process.Signal(os.Interrupt); err != nil {
		cm.t.Logf("WARNING: failed to send SIGTERM to PID %d: %v", process.Pid, err)
	}

	// Wait briefly for graceful shutdown
	done := make(chan error, 1)
	go func() {
		_, err := process.Wait()
		done <- err
	}()

	select {
	case <-time.After(5 * time.Second):
		// Graceful shutdown failed, send SIGKILL
		cm.t.Logf("Process %d did not exit gracefully, sending SIGKILL", process.Pid)
		if err := process.Kill(); err != nil {
			return fmt.Errorf("failed to kill process %d: %w", process.Pid, err)
		}
		<-done // Wait for process to actually exit
	case err := <-done:
		if err != nil {
			return fmt.Errorf("process %d exited with error: %w", process.Pid, err)
		}
	}

	return nil
}

// CleanupDevnet performs devnet-specific cleanup
// - Stops all running validators
// - Removes devnet data directory
// - Stops Docker containers matching devnet patterns
func (cm *CleanupManager) CleanupDevnet() error {
	cm.t.Helper()

	// Run devnet-builder destroy command if binary exists
	if _, err := os.Stat(cm.ctx.BinaryPath); err == nil {
		cm.t.Log("Running devnet-builder destroy...")
		cmd := exec.Command(cm.ctx.BinaryPath, "destroy", "--home", cm.ctx.HomeDir)
		cmd.Env = cm.ctx.GetEnv()
		// Ignore error - devnet might not be running
		_ = cmd.Run()
	}

	// Remove home directory
	if err := os.RemoveAll(cm.ctx.HomeDir); err != nil {
		return fmt.Errorf("failed to remove devnet home directory: %w", err)
	}

	// Cleanup orphaned Docker containers
	if err := cm.cleanupDockerContainers(); err != nil {
		cm.t.Logf("WARNING: failed to cleanup Docker containers: %v", err)
	}

	// Cleanup orphaned processes
	if err := cm.cleanupOrphanedProcesses(); err != nil {
		cm.t.Logf("WARNING: failed to cleanup orphaned processes: %v", err)
	}

	return nil
}

// cleanupDockerContainers removes Docker containers matching devnet patterns
func (cm *CleanupManager) cleanupDockerContainers() error {
	// List containers with devnet-builder labels/names
	cmd := exec.Command("docker", "ps", "-a", "-q", "--filter", "name=validator")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to list Docker containers: %w", err)
	}

	containerIDs := strings.Fields(string(output))
	for _, id := range containerIDs {
		cm.t.Logf("Removing Docker container: %s", id)
		cmd := exec.Command("docker", "rm", "-f", id)
		if err := cmd.Run(); err != nil {
			cm.t.Logf("WARNING: failed to remove container %s: %v", id, err)
		}
	}

	return nil
}

// cleanupOrphanedProcesses finds and kills processes matching devnet patterns
func (cm *CleanupManager) cleanupOrphanedProcesses() error {
	// Find processes running from the test home directory
	homeDir := cm.ctx.HomeDir
	pidFiles, err := filepath.Glob(filepath.Join(homeDir, "*.pid"))
	if err != nil {
		return fmt.Errorf("failed to find PID files: %w", err)
	}

	for _, pidFile := range pidFiles {
		cm.t.Logf("Reading PID file: %s", pidFile)
		data, err := os.ReadFile(pidFile)
		if err != nil {
			cm.t.Logf("WARNING: failed to read PID file %s: %v", pidFile, err)
			continue
		}

		var pid int
		if _, err := fmt.Sscanf(string(data), "%d", &pid); err != nil {
			cm.t.Logf("WARNING: invalid PID in file %s: %v", pidFile, err)
			continue
		}

		// Try to find and kill process
		process, err := os.FindProcess(pid)
		if err != nil {
			cm.t.Logf("WARNING: failed to find process %d: %v", pid, err)
			continue
		}

		cm.t.Logf("Killing orphaned process: %d", pid)
		if err := cm.killProcess(process); err != nil {
			cm.t.Logf("WARNING: failed to kill process %d: %v", pid, err)
		}

		// Remove PID file
		_ = os.Remove(pidFile)
	}

	return nil
}

// AssertNoLeaks checks for resource leaks after cleanup
// - No running Docker containers with devnet labels
// - No processes from test home directory
// - No leftover files in temp directory
func (cm *CleanupManager) AssertNoLeaks() {
	cm.t.Helper()

	// Check for Docker containers
	cmd := exec.Command("docker", "ps", "-q", "--filter", "name=validator")
	output, err := cmd.Output()
	if err == nil && len(strings.TrimSpace(string(output))) > 0 {
		cm.t.Errorf("LEAK: Found running Docker containers after cleanup: %s", output)
	}

	// Check for processes from test home directory
	homeDir := cm.ctx.HomeDir
	pidFiles, _ := filepath.Glob(filepath.Join(homeDir, "*.pid"))
	if len(pidFiles) > 0 {
		cm.t.Errorf("LEAK: Found PID files after cleanup: %v", pidFiles)
	}

	// Check for leftover files (excluding go test temp dirs which are auto-cleaned)
	if _, err := os.Stat(homeDir); err == nil {
		entries, _ := os.ReadDir(homeDir)
		if len(entries) > 0 {
			cm.t.Logf("WARNING: Found files in test home directory after cleanup: %d entries", len(entries))
		}
	}
}
