// internal/daemon/server/server.go
package server

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/altuslabsxyz/devnet-builder/internal/daemon/store"
)

// Config holds server configuration.
type Config struct {
	// SocketPath is the Unix socket path.
	SocketPath string
	// DataDir is the data directory.
	DataDir string
	// Foreground runs in foreground (don't daemonize).
	Foreground bool
}

// DefaultConfig returns default configuration.
func DefaultConfig() *Config {
	home, _ := os.UserHomeDir()
	dataDir := filepath.Join(home, ".devnet-builder")
	return &Config{
		SocketPath: filepath.Join(dataDir, "devnetd.sock"),
		DataDir:    dataDir,
		Foreground: false,
	}
}

// Server is the devnetd daemon server.
type Server struct {
	config   *Config
	store    store.Store
	listener net.Listener
}

// New creates a new server.
func New(config *Config) (*Server, error) {
	// Ensure data directory exists
	if err := os.MkdirAll(config.DataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	// Open state store
	dbPath := filepath.Join(config.DataDir, "devnetd.db")
	st, err := store.NewBoltStore(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open state store: %w", err)
	}

	return &Server{
		config: config,
		store:  st,
	}, nil
}

// Run starts the server and blocks until shutdown.
func (s *Server) Run(ctx context.Context) error {
	// Remove stale socket
	os.Remove(s.config.SocketPath)

	// Create listener
	listener, err := net.Listen("unix", s.config.SocketPath)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}
	s.listener = listener

	// Write PID file
	pidPath := filepath.Join(s.config.DataDir, "devnetd.pid")
	if err := os.WriteFile(pidPath, []byte(fmt.Sprintf("%d", os.Getpid())), 0644); err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}
	defer os.Remove(pidPath)

	fmt.Printf("devnetd started\n")
	fmt.Printf("  Socket: %s\n", s.config.SocketPath)
	fmt.Printf("  Data: %s\n", s.config.DataDir)
	fmt.Printf("  PID: %d\n", os.Getpid())

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Wait for context cancellation or signal
	select {
	case <-ctx.Done():
	case sig := <-sigCh:
		fmt.Printf("\nReceived %s, shutting down...\n", sig)
	}

	return s.Shutdown()
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown() error {
	if s.listener != nil {
		s.listener.Close()
	}
	if s.store != nil {
		s.store.Close()
	}
	os.Remove(s.config.SocketPath)
	fmt.Println("devnetd stopped")
	return nil
}
