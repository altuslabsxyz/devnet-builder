package daemon

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// shortTempDir creates a short temp directory to avoid Unix socket path limits (~104 chars on macOS)
func shortTempDir(t *testing.T) string {
	dir, err := os.MkdirTemp("/tmp", "dt")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

func TestIsDaemonRunning(t *testing.T) {
	// Test with non-existent socket
	t.Run("non-existent socket", func(t *testing.T) {
		result := isDaemonRunning("/tmp/non-existent-socket-12345.sock")
		if result {
			t.Error("expected false for non-existent socket")
		}
	})

	// Test with actual listening socket
	t.Run("listening socket", func(t *testing.T) {
		dir := shortTempDir(t)
		socketPath := filepath.Join(dir, "t.sock")

		listener, err := net.Listen("unix", socketPath)
		if err != nil {
			t.Fatalf("failed to create test socket: %v", err)
		}
		defer listener.Close()

		result := isDaemonRunning(socketPath)
		if !result {
			t.Error("expected true for listening socket")
		}
	})

	// Test with stale socket file (no listener)
	t.Run("stale socket file", func(t *testing.T) {
		dir := shortTempDir(t)
		socketPath := filepath.Join(dir, "s.sock")

		// Create a regular file (not a socket)
		f, err := os.Create(socketPath)
		if err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}
		f.Close()

		result := isDaemonRunning(socketPath)
		if result {
			t.Error("expected false for stale socket file")
		}
	})
}

func TestWaitForSocket(t *testing.T) {
	t.Run("socket already exists", func(t *testing.T) {
		dir := shortTempDir(t)
		socketPath := filepath.Join(dir, "t.sock")

		listener, err := net.Listen("unix", socketPath)
		if err != nil {
			t.Fatalf("failed to create test socket: %v", err)
		}
		defer listener.Close()

		err = waitForSocket(socketPath, time.Second)
		if err != nil {
			t.Errorf("expected no error, got: %v", err)
		}
	})

	t.Run("socket created after delay", func(t *testing.T) {
		dir := shortTempDir(t)
		socketPath := filepath.Join(dir, "d.sock")

		// Create socket after a short delay
		listenerCh := make(chan net.Listener, 1)
		go func() {
			time.Sleep(200 * time.Millisecond)
			l, err := net.Listen("unix", socketPath)
			if err != nil {
				fmt.Printf("failed to create delayed socket: %v\n", err)
				listenerCh <- nil
				return
			}
			listenerCh <- l
		}()

		err := waitForSocket(socketPath, 2*time.Second)
		if l := <-listenerCh; l != nil {
			l.Close()
		}
		if err != nil {
			t.Errorf("expected no error, got: %v", err)
		}
	})

	t.Run("timeout when socket never created", func(t *testing.T) {
		dir := shortTempDir(t)
		socketPath := filepath.Join(dir, "n.sock")

		err := waitForSocket(socketPath, 200*time.Millisecond)
		if err == nil {
			t.Error("expected timeout error")
		}
	})
}

func TestWaitForExit(t *testing.T) {
	t.Run("socket already gone", func(t *testing.T) {
		dir := shortTempDir(t)
		socketPath := filepath.Join(dir, "g.sock")

		err := waitForExit(socketPath, time.Second)
		if err != nil {
			t.Errorf("expected no error, got: %v", err)
		}
	})

	t.Run("socket closes after delay", func(t *testing.T) {
		dir := shortTempDir(t)
		socketPath := filepath.Join(dir, "c.sock")

		listener, err := net.Listen("unix", socketPath)
		if err != nil {
			t.Fatalf("failed to create test socket: %v", err)
		}

		// Close socket after a short delay
		go func() {
			time.Sleep(200 * time.Millisecond)
			listener.Close()
		}()

		err = waitForExit(socketPath, 2*time.Second)
		if err != nil {
			t.Errorf("expected no error, got: %v", err)
		}
	})

	t.Run("timeout when socket never closes", func(t *testing.T) {
		dir := shortTempDir(t)
		socketPath := filepath.Join(dir, "p.sock")

		listener, err := net.Listen("unix", socketPath)
		if err != nil {
			t.Fatalf("failed to create test socket: %v", err)
		}
		defer listener.Close()

		err = waitForExit(socketPath, 200*time.Millisecond)
		if err == nil {
			t.Error("expected timeout error")
		}
	})
}

func TestFindDevnetd(t *testing.T) {
	// This test only verifies the function doesn't panic
	// Actual devnetd may or may not exist in test environment
	t.Run("does not panic", func(t *testing.T) {
		_, err := findDevnetd()
		// We expect an error if devnetd is not installed
		// The important thing is it doesn't panic
		_ = err
	})
}
