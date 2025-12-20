// Package process provides process execution implementations.
package process

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"time"

	"github.com/b-harvest/devnet-builder/internal/application/ports"
)

// LocalExecutor implements ProcessExecutor for local process execution.
type LocalExecutor struct{}

// NewLocalExecutor creates a new LocalExecutor.
func NewLocalExecutor() *LocalExecutor {
	return &LocalExecutor{}
}

// localHandle implements ProcessHandle for local processes.
type localHandle struct {
	pid     int
	process *os.Process
	cmd     *exec.Cmd
	logPath string
	done    chan struct{}
	err     error
}

// PID returns the process ID.
func (h *localHandle) PID() int {
	return h.pid
}

// IsRunning checks if the process is still running.
func (h *localHandle) IsRunning() bool {
	if h.process == nil {
		return false
	}

	// Send signal 0 to check if process is alive
	err := h.process.Signal(syscall.Signal(0))
	return err == nil
}

// Wait blocks until the process exits.
func (h *localHandle) Wait() error {
	if h.done != nil {
		<-h.done
	}
	return h.err
}

// Kill terminates the process.
func (h *localHandle) Kill() error {
	if h.process == nil {
		return nil
	}
	return h.process.Kill()
}

// Start launches a new process.
func (e *LocalExecutor) Start(ctx context.Context, cmd ports.Command) (ports.ProcessHandle, error) {
	// Validate binary exists
	if _, err := os.Stat(cmd.Binary); os.IsNotExist(err) {
		return nil, &ExecutionError{
			Operation: "start",
			Message:   fmt.Sprintf("binary not found: %s", cmd.Binary),
		}
	}

	// Ensure work directory exists
	if cmd.WorkDir != "" {
		if err := os.MkdirAll(cmd.WorkDir, 0755); err != nil {
			return nil, &ExecutionError{
				Operation: "start",
				Message:   fmt.Sprintf("failed to create work directory: %v", err),
			}
		}
	}

	// Open log file if specified
	var logFile *os.File
	if cmd.LogPath != "" {
		var err error
		logFile, err = os.OpenFile(cmd.LogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, &ExecutionError{
				Operation: "start",
				Message:   fmt.Sprintf("failed to open log file: %v", err),
			}
		}
	}

	// Build exec command
	execCmd := exec.CommandContext(ctx, cmd.Binary, cmd.Args...)
	if cmd.WorkDir != "" {
		execCmd.Dir = cmd.WorkDir
	}
	if len(cmd.Env) > 0 {
		execCmd.Env = append(os.Environ(), cmd.Env...)
	}

	if logFile != nil {
		execCmd.Stdout = logFile
		execCmd.Stderr = logFile
	}

	// Start process
	if err := execCmd.Start(); err != nil {
		if logFile != nil {
			logFile.Close()
		}
		return nil, &ExecutionError{
			Operation: "start",
			Message:   fmt.Sprintf("failed to start process: %v", err),
		}
	}

	handle := &localHandle{
		pid:     execCmd.Process.Pid,
		process: execCmd.Process,
		cmd:     execCmd,
		logPath: cmd.LogPath,
		done:    make(chan struct{}),
	}

	// Write PID file if specified
	if cmd.PIDPath != "" {
		if err := os.WriteFile(cmd.PIDPath, []byte(strconv.Itoa(handle.pid)), 0644); err != nil {
			// Log warning but don't fail
			fmt.Fprintf(os.Stderr, "warning: failed to write PID file: %v\n", err)
		}
	}

	// Wait for process in background
	go func() {
		handle.err = execCmd.Wait()
		if logFile != nil {
			logFile.Close()
		}
		close(handle.done)
	}()

	return handle, nil
}

// Stop gracefully stops a process with timeout.
func (e *LocalExecutor) Stop(ctx context.Context, handle ports.ProcessHandle, timeout time.Duration) error {
	lh, ok := handle.(*localHandle)
	if !ok {
		return &ExecutionError{
			Operation: "stop",
			Message:   "invalid handle type",
		}
	}

	if lh.process == nil {
		return nil
	}

	// Send SIGTERM for graceful shutdown
	if err := lh.process.Signal(syscall.SIGTERM); err != nil {
		// Process might already be dead
		return nil
	}

	// Wait for process to exit with timeout
	select {
	case <-lh.done:
		return nil
	case <-time.After(timeout):
		// Force kill
		lh.process.Signal(syscall.SIGKILL)
		<-lh.done
		return nil
	case <-ctx.Done():
		lh.process.Signal(syscall.SIGKILL)
		return ctx.Err()
	}
}

// Kill forcefully terminates a process.
func (e *LocalExecutor) Kill(handle ports.ProcessHandle) error {
	return handle.Kill()
}

// IsRunning checks if a process is running.
func (e *LocalExecutor) IsRunning(handle ports.ProcessHandle) bool {
	return handle.IsRunning()
}

// Logs retrieves the last N lines from the process log.
func (e *LocalExecutor) Logs(handle ports.ProcessHandle, lines int) ([]string, error) {
	lh, ok := handle.(*localHandle)
	if !ok {
		return nil, &ExecutionError{
			Operation: "logs",
			Message:   "invalid handle type",
		}
	}

	if lh.logPath == "" {
		return nil, &ExecutionError{
			Operation: "logs",
			Message:   "no log path configured",
		}
	}

	return readLastLines(lh.logPath, lines)
}

// readLastLines reads the last N lines from a file.
func readLastLines(path string, n int) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}

	if stat.Size() == 0 {
		return nil, nil
	}

	// Read entire file and take last N lines (simple approach)
	var allLines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		allLines = append(allLines, scanner.Text())
	}

	if err := scanner.Err(); err != nil && err != io.EOF {
		return nil, err
	}

	if len(allLines) <= n {
		return allLines, nil
	}

	return allLines[len(allLines)-n:], nil
}

// ReadPIDFile reads a PID from a file.
func ReadPIDFile(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}

	pid, err := strconv.Atoi(string(data))
	if err != nil {
		return 0, fmt.Errorf("invalid PID format: %w", err)
	}

	return pid, nil
}

// FindProcessByPID finds a process by PID and returns a handle.
func (e *LocalExecutor) FindProcessByPID(pid int, logPath string) (ports.ProcessHandle, error) {
	process, err := os.FindProcess(pid)
	if err != nil {
		return nil, err
	}

	// Check if process is actually running
	if err := process.Signal(syscall.Signal(0)); err != nil {
		return nil, &NotRunningError{PID: pid}
	}

	return &localHandle{
		pid:     pid,
		process: process,
		logPath: logPath,
		done:    make(chan struct{}),
	}, nil
}

// Ensure LocalExecutor implements ProcessExecutor.
var _ ports.ProcessExecutor = (*LocalExecutor)(nil)
