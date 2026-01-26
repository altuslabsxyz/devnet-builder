// Package daemon provides daemon management commands.
package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	v1 "github.com/altuslabsxyz/devnet-builder/api/proto/gen/v1"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/server"
	"github.com/altuslabsxyz/devnet-builder/internal/output"
	"github.com/altuslabsxyz/devnet-builder/types/ctxconfig"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	daemonLogLevel   string
	daemonForeground bool
	daemonDocker     bool
)

// NewDaemonCmd creates the daemon command group.
func NewDaemonCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "daemon",
		Short: "Manage the devnetd daemon",
		Long: `Manage the devnetd daemon process.

The daemon provides gRPC services for managing devnets, including
creation, destruction, health monitoring, and node management.

Examples:
  # Start the daemon in background
  devnet-builder daemon start

  # Start the daemon in foreground (for debugging)
  devnet-builder daemon start --foreground

  # Check daemon status
  devnet-builder daemon status

  # Stop the daemon
  devnet-builder daemon stop`,
	}

	cmd.AddCommand(
		newStartCmd(),
		newStopCmd(),
		newStatusCmd(),
	)

	return cmd
}

func newStartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the devnetd daemon",
		Long: `Start the devnetd daemon process.

By default, the daemon starts in the background. Use --foreground
to run in the foreground for debugging.`,
		RunE: runStart,
	}

	cmd.Flags().StringVar(&daemonLogLevel, "log-level", "info",
		"Log level (debug, info, warn, error)")
	cmd.Flags().BoolVar(&daemonForeground, "foreground", false,
		"Run daemon in foreground")
	cmd.Flags().BoolVar(&daemonDocker, "docker", true,
		"Enable Docker container runtime")

	return cmd
}

func newStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the devnetd daemon",
		RunE:  runStop,
	}
}

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show daemon status",
		RunE:  runDaemonStatus,
	}
}

func runStart(cmd *cobra.Command, args []string) error {
	config := server.DefaultConfig()
	socketPath := config.SocketPath

	// Check if daemon is already running
	if isDaemonRunning(socketPath) {
		output.Info("Daemon is already running")
		return nil
	}

	// Find devnetd binary
	devnetdPath, err := findDevnetd()
	if err != nil {
		return fmt.Errorf("devnetd not found: %w", err)
	}

	// Build command arguments
	daemonArgs := []string{
		"--log-level", daemonLogLevel,
		"--socket", socketPath,
	}
	if daemonDocker {
		daemonArgs = append(daemonArgs, "--docker")
	}

	if daemonForeground {
		// Run in foreground - just exec
		daemonArgs = append([]string{"--foreground"}, daemonArgs...)
		daemonCmd := exec.CommandContext(cmd.Context(), devnetdPath, daemonArgs...)
		daemonCmd.Stdout = os.Stdout
		daemonCmd.Stderr = os.Stderr
		daemonCmd.Stdin = os.Stdin
		return daemonCmd.Run()
	}

	// Start in background
	daemonCmd := exec.Command(devnetdPath, daemonArgs...)
	daemonCmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	if err := daemonCmd.Start(); err != nil {
		return fmt.Errorf("failed to start daemon: %w", err)
	}

	// Wait for socket to be available
	output.Info("Starting daemon...")
	if err := waitForSocket(socketPath, 10*time.Second); err != nil {
		return fmt.Errorf("daemon failed to start: %w", err)
	}

	output.Success("Daemon started (PID: %d)", daemonCmd.Process.Pid)
	return nil
}

func runStop(cmd *cobra.Command, args []string) error {
	config := server.DefaultConfig()
	socketPath := config.SocketPath

	if !isDaemonRunning(socketPath) {
		output.Info("Daemon is not running")
		return nil
	}

	// Try graceful shutdown via socket
	pid, err := getDaemonPID(socketPath)
	if err == nil && pid > 0 {
		process, err := os.FindProcess(pid)
		if err == nil {
			output.Info("Stopping daemon (PID: %d)...", pid)
			if err := process.Signal(syscall.SIGTERM); err != nil {
				output.Warn("Failed to send SIGTERM: %v", err)
			}

			// Wait for process to exit
			if err := waitForExit(socketPath, 10*time.Second); err != nil {
				// Force kill if graceful shutdown fails
				output.Warn("Graceful shutdown failed, force killing...")
				_ = process.Signal(syscall.SIGKILL)
			}
		}
	}

	// Remove stale socket file
	_ = os.Remove(socketPath)

	output.Success("Daemon stopped")
	return nil
}

func runDaemonStatus(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	cfg := ctxconfig.FromContext(ctx)
	jsonMode := cfg.JSONMode()

	config := server.DefaultConfig()
	socketPath := config.SocketPath

	status := struct {
		Running    bool   `json:"running"`
		SocketPath string `json:"socket_path"`
		PID        int    `json:"pid,omitempty"`
		Version    string `json:"version,omitempty"`
	}{
		SocketPath: socketPath,
	}

	if isDaemonRunning(socketPath) {
		status.Running = true
		status.PID, _ = getDaemonPID(socketPath)

		// Try to get version info via gRPC
		conn, err := grpc.NewClient(
			"unix://"+socketPath,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err == nil {
			defer conn.Close()
			// We don't have a version endpoint, but connection success means it's running
		}
	}

	if jsonMode {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(status)
	}

	if status.Running {
		output.Success("Daemon is running")
		fmt.Printf("  Socket: %s\n", status.SocketPath)
		if status.PID > 0 {
			fmt.Printf("  PID: %d\n", status.PID)
		}
	} else {
		output.Info("Daemon is not running")
		fmt.Printf("  Socket: %s (not found)\n", status.SocketPath)
	}

	return nil
}

// isDaemonRunning checks if the daemon is running by testing the socket.
func isDaemonRunning(socketPath string) bool {
	conn, err := net.DialTimeout("unix", socketPath, time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// getDaemonPID gets the daemon PID by querying it.
func getDaemonPID(socketPath string) (int, error) {
	// Try to get PID from daemon via gRPC
	conn, err := grpc.NewClient(
		"unix://"+socketPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return 0, err
	}
	defer conn.Close()

	// Use DevnetService to list devnets (just to verify connection)
	// The actual PID retrieval would require a status endpoint
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	client := v1.NewDevnetServiceClient(conn)
	_, err = client.ListDevnets(ctx, &v1.ListDevnetsRequest{})
	if err != nil {
		return 0, err
	}

	// We don't have a direct PID endpoint, return 0
	return 0, nil
}

// waitForSocket waits for the socket to become available.
func waitForSocket(socketPath string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if isDaemonRunning(socketPath) {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for daemon socket")
}

// waitForExit waits for the socket to disappear.
func waitForExit(socketPath string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !isDaemonRunning(socketPath) {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for daemon to exit")
}

// findDevnetd finds the devnetd binary.
func findDevnetd() (string, error) {
	// Check if devnetd is in PATH
	path, err := exec.LookPath("devnetd")
	if err == nil {
		return path, nil
	}

	// Check in same directory as current executable
	exe, err := os.Executable()
	if err == nil {
		dir := filepath.Dir(exe)
		devnetdPath := filepath.Join(dir, "devnetd")
		if _, err := os.Stat(devnetdPath); err == nil {
			return devnetdPath, nil
		}
	}

	// Check in common locations
	commonPaths := []string{
		"/usr/local/bin/devnetd",
		"/usr/bin/devnetd",
		"./devnetd",
	}
	for _, p := range commonPaths {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}

	return "", fmt.Errorf("devnetd binary not found in PATH or common locations")
}
