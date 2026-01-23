// internal/client/detect.go
package client

import (
	"net"
	"os"
	"path/filepath"
	"time"
)

// DefaultSocketPath returns the default daemon socket path.
func DefaultSocketPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".devnet-builder", "devnetd.sock")
}

// IsDaemonRunning checks if the daemon is accessible.
func IsDaemonRunning() bool {
	return IsDaemonRunningAt(DefaultSocketPath())
}

// IsDaemonRunningAt checks if the daemon is accessible at the given socket path.
func IsDaemonRunningAt(socketPath string) bool {
	conn, err := net.DialTimeout("unix", socketPath, 500*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
