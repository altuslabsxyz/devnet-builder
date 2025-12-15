package node

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/stablelabs/stable-devnet/internal/output"
)

const (
	// DefaultLocalBinary is the default binary name for local mode.
	DefaultLocalBinary = "stabled"

	// LocalStopTimeout is the timeout for gracefully stopping a local process.
	LocalStopTimeout = 30 * time.Second
)

// LocalManager manages nodes running as local processes.
type LocalManager struct {
	Binary     string
	EVMChainID string
	Logger     *output.Logger
}

// NewLocalManager creates a new LocalManager.
func NewLocalManager(binary string, logger *output.Logger) *LocalManager {
	if binary == "" {
		binary = DefaultLocalBinary
	}
	if logger == nil {
		logger = output.DefaultLogger
	}
	return &LocalManager{
		Binary: binary,
		Logger: logger,
	}
}

// NewLocalManagerWithEVMChainID creates a new LocalManager with EVM chain ID.
func NewLocalManagerWithEVMChainID(binary string, evmChainID string, logger *output.Logger) *LocalManager {
	m := NewLocalManager(binary, logger)
	m.EVMChainID = evmChainID
	return m
}

// ExtractEVMChainID extracts the EVM chain ID from a Cosmos chain ID.
// For example, "stable_988-1" returns "988".
func ExtractEVMChainID(chainID string) string {
	// Format: {name}_{evmChainID}-{version}
	// Example: stable_988-1 -> 988
	parts := strings.Split(chainID, "_")
	if len(parts) < 2 {
		return ""
	}
	evmPart := parts[len(parts)-1] // Get last part after underscore
	// Remove version suffix (e.g., "-1")
	if idx := strings.Index(evmPart, "-"); idx > 0 {
		return evmPart[:idx]
	}
	return evmPart
}

// Start starts a node as a local process.
func (m *LocalManager) Start(ctx context.Context, node *Node, genesisPath string) error {
	// Check if already running
	if m.IsRunning(ctx, node) {
		return fmt.Errorf("node %s is already running", node.Name)
	}

	// Validate binary path
	binaryPath := m.Binary
	if binaryPath == "" {
		return fmt.Errorf("no binary path specified for local mode")
	}

	// Check if binary exists at the specified path
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		return fmt.Errorf("binary not found at %s\nHint: Build the stabled binary first with 'devnet-builder build-binary'", binaryPath)
	}

	// Verify binary is executable
	info, err := os.Stat(binaryPath)
	if err != nil {
		return fmt.Errorf("failed to stat binary: %w", err)
	}
	if info.Mode()&0111 == 0 {
		return fmt.Errorf("binary at %s is not executable", binaryPath)
	}

	// Ensure node home directory exists
	if err := os.MkdirAll(node.HomeDir, 0755); err != nil {
		return fmt.Errorf("failed to create node directory: %w", err)
	}

	// Open log file
	logFile, err := os.OpenFile(node.LogFilePath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	// Build start command
	args := []string{
		"start",
		"--home", node.HomeDir,
		fmt.Sprintf("--rpc.laddr=tcp://0.0.0.0:%d", node.Ports.RPC),
		fmt.Sprintf("--p2p.laddr=tcp://0.0.0.0:%d", node.Ports.P2P),
		fmt.Sprintf("--grpc.address=0.0.0.0:%d", node.Ports.GRPC),
		"--api.enabled-unsafe-cors=true",
		"--api.enable=true",
		fmt.Sprintf("--json-rpc.address=0.0.0.0:%d", node.Ports.EVMRPC),
		fmt.Sprintf("--json-rpc.ws-address=0.0.0.0:%d", node.Ports.EVMWS),
	}

	// Add EVM chain ID if set
	if m.EVMChainID != "" {
		args = append(args, fmt.Sprintf("--evm.evm-chain-id=%s", m.EVMChainID))
	}

	m.Logger.Debug("Starting local process: %s %v", binaryPath, args)

	cmd := exec.Command(binaryPath, args...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Dir = node.HomeDir

	// Start process in background
	if err := cmd.Start(); err != nil {
		logFile.Close()
		// Print command error info for debugging
		m.Logger.PrintCommandError(&output.CommandErrorInfo{
			Command:  binaryPath,
			Args:     args,
			WorkDir:  node.HomeDir,
			ExitCode: -1,
			Error:    err,
		})
		return fmt.Errorf("failed to start process: %w", err)
	}

	// Write PID file
	pid := cmd.Process.Pid
	if err := os.WriteFile(node.PIDFilePath(), []byte(strconv.Itoa(pid)), 0644); err != nil {
		m.Logger.Warn("Failed to write PID file: %v", err)
	}

	node.PID = &pid
	node.LogFile = node.LogFilePath()
	node.Status = NodeStatusStarting

	// Don't wait for the process - let it run in background
	go func() {
		cmd.Wait()
		logFile.Close()
	}()

	return nil
}

// Stop stops a running local process.
func (m *LocalManager) Stop(ctx context.Context, node *Node, timeout time.Duration) error {
	if node.PID == nil {
		// Try to read PID from file
		pid, err := m.readPIDFile(node.PIDFilePath())
		if err != nil {
			return nil // No process to stop
		}
		node.PID = &pid
	}

	process, err := os.FindProcess(*node.PID)
	if err != nil {
		node.SetStopped()
		return nil
	}

	// Send SIGTERM for graceful shutdown
	if err := process.Signal(syscall.SIGTERM); err != nil {
		// Process might already be dead
		node.SetStopped()
		m.cleanupPIDFile(node)
		return nil
	}

	// Wait for process to exit with timeout
	done := make(chan error, 1)
	go func() {
		_, err := process.Wait()
		done <- err
	}()

	if timeout == 0 {
		timeout = LocalStopTimeout
	}

	select {
	case <-done:
		// Process exited gracefully
	case <-time.After(timeout):
		// Force kill
		m.Logger.Warn("Process %d did not exit gracefully, sending SIGKILL", *node.PID)
		process.Signal(syscall.SIGKILL)
		<-done
	case <-ctx.Done():
		process.Signal(syscall.SIGKILL)
		return ctx.Err()
	}

	node.SetStopped()
	m.cleanupPIDFile(node)
	return nil
}

// IsRunning checks if a local process is running.
func (m *LocalManager) IsRunning(ctx context.Context, node *Node) bool {
	pid := node.PID

	// Try to read from PID file if not set
	if pid == nil {
		p, err := m.readPIDFile(node.PIDFilePath())
		if err != nil {
			return false
		}
		pid = &p
	}

	// Check if process exists
	process, err := os.FindProcess(*pid)
	if err != nil {
		return false
	}

	// Send signal 0 to check if process is alive
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// readPIDFile reads and parses a PID file.
func (m *LocalManager) readPIDFile(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}

	pid, err := strconv.Atoi(string(data))
	if err != nil {
		return 0, err
	}

	return pid, nil
}

// cleanupPIDFile removes the PID file.
func (m *LocalManager) cleanupPIDFile(node *Node) {
	os.Remove(node.PIDFilePath())
}

// GetLogs reads logs from the log file.
func (m *LocalManager) GetLogs(ctx context.Context, node *Node, tail int) (string, error) {
	logPath := node.LogFilePath()

	file, err := os.Open(logPath)
	if err != nil {
		return "", fmt.Errorf("failed to open log file: %w", err)
	}
	defer file.Close()

	// Read all if tail is 0
	if tail <= 0 {
		data, err := io.ReadAll(file)
		if err != nil {
			return "", fmt.Errorf("failed to read log file: %w", err)
		}
		return string(data), nil
	}

	// Read last N lines
	return readLastLines(file, tail)
}

// readLastLines reads the last n lines from a file.
func readLastLines(file *os.File, n int) (string, error) {
	stat, err := file.Stat()
	if err != nil {
		return "", err
	}

	size := stat.Size()
	if size == 0 {
		return "", nil
	}

	// Start from end and work backwards
	var lines []string
	bufSize := int64(4096)
	offset := size

	for len(lines) < n && offset > 0 {
		readSize := bufSize
		if offset < bufSize {
			readSize = offset
		}
		offset -= readSize

		buf := make([]byte, readSize)
		_, err := file.ReadAt(buf, offset)
		if err != nil && err != io.EOF {
			return "", err
		}

		// Split into lines and prepend
		content := string(buf)
		parts := splitLines(content)
		lines = append(parts, lines...)
	}

	// Take last n lines
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}

	result := ""
	for _, line := range lines {
		result += line + "\n"
	}
	return result, nil
}

// splitLines splits content into lines.
func splitLines(content string) []string {
	var lines []string
	start := 0
	for i, c := range content {
		if c == '\n' {
			lines = append(lines, content[start:i])
			start = i + 1
		}
	}
	if start < len(content) {
		lines = append(lines, content[start:])
	}
	return lines
}

// IsLocalBinaryAvailable checks if the local binary is available at the given path.
// For local mode, the binary must be an absolute path (e.g., ~/.stable-devnet/bin/stabled).
func IsLocalBinaryAvailable(binaryPath string) bool {
	if binaryPath == "" {
		return false
	}

	info, err := os.Stat(binaryPath)
	if err != nil {
		return false
	}

	// Check if executable
	return info.Mode()&0111 != 0
}
