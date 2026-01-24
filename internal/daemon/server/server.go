// internal/daemon/server/server.go
package server

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	v1 "github.com/altuslabsxyz/devnet-builder/api/proto/gen/v1"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/checker"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/controller"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/provisioner"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/runtime"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/store"
	"google.golang.org/grpc"
)

// Config holds server configuration.
type Config struct {
	// SocketPath is the Unix socket path.
	SocketPath string
	// DataDir is the data directory.
	DataDir string
	// Foreground runs in foreground (don't daemonize).
	Foreground bool
	// Workers is the number of workers per controller.
	Workers int
	// LogLevel is the log level (debug, info, warn, error).
	LogLevel string
	// EnableDocker enables Docker container runtime for nodes.
	EnableDocker bool
	// DockerImage is the default Docker image for nodes.
	DockerImage string
}

// DefaultConfig returns default configuration.
func DefaultConfig() *Config {
	home, _ := os.UserHomeDir()
	dataDir := filepath.Join(home, ".devnet-builder")
	return &Config{
		SocketPath: filepath.Join(dataDir, "devnetd.sock"),
		DataDir:    dataDir,
		Foreground: false,
		Workers:    2,
		LogLevel:   "info",
	}
}

// Server is the devnetd daemon server.
type Server struct {
	config     *Config
	store      store.Store
	manager    *controller.Manager
	healthCtrl *controller.HealthController
	grpcServer *grpc.Server
	listener   net.Listener
	logger     *slog.Logger
}

// New creates a new server.
func New(config *Config) (*Server, error) {
	// Set up logger
	level := slog.LevelInfo
	switch config.LogLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level}))

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

	// Create controller manager
	mgr := controller.NewManager()
	mgr.SetLogger(logger)

	// Create devnet provisioner
	devnetProv := provisioner.NewDevnetProvisioner(st, provisioner.Config{
		DataDir: config.DataDir,
		Logger:  logger,
	})

	// Register controllers
	devnetCtrl := controller.NewDevnetController(st, devnetProv)
	devnetCtrl.SetLogger(logger)
	mgr.Register("devnets", devnetCtrl)

	// Create node runtime (Docker or nil)
	var nodeRuntime controller.NodeRuntime
	if config.EnableDocker {
		dockerRuntime, err := runtime.NewDockerRuntime(runtime.DockerConfig{
			DefaultImage: config.DockerImage,
			Logger:       logger,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create docker runtime: %w", err)
		}
		nodeRuntime = dockerRuntime
		logger.Info("docker runtime enabled", "image", config.DockerImage)
	}

	nodeCtrl := controller.NewNodeController(st, nodeRuntime)
	nodeCtrl.SetLogger(logger)
	mgr.Register("nodes", nodeCtrl)

	// Create health checker
	healthChecker := checker.NewRPCHealthChecker(checker.Config{
		Logger: logger,
	})

	// Create and register health controller
	healthConfig := controller.DefaultHealthControllerConfig()
	healthCtrl := controller.NewHealthController(st, healthChecker, mgr, healthConfig)
	healthCtrl.SetLogger(logger)
	mgr.Register("health", healthCtrl)

	// Create and register upgrade controller
	upgradeCtrl := controller.NewUpgradeController(st, nil) // No runtime yet
	upgradeCtrl.SetLogger(logger)
	mgr.Register("upgrades", upgradeCtrl)

	// Create gRPC server
	grpcServer := grpc.NewServer()

	// Register services
	devnetSvc := NewDevnetService(st, mgr)
	devnetSvc.SetLogger(logger)
	v1.RegisterDevnetServiceServer(grpcServer, devnetSvc)

	nodeSvc := NewNodeService(st, mgr)
	nodeSvc.SetLogger(logger)
	v1.RegisterNodeServiceServer(grpcServer, nodeSvc)

	upgradeSvc := NewUpgradeService(st, mgr)
	upgradeSvc.SetLogger(logger)
	v1.RegisterUpgradeServiceServer(grpcServer, upgradeSvc)

	return &Server{
		config:     config,
		store:      st,
		manager:    mgr,
		healthCtrl: healthCtrl,
		grpcServer: grpcServer,
		logger:     logger,
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

	s.logger.Info("devnetd started",
		"socket", s.config.SocketPath,
		"dataDir", s.config.DataDir,
		"pid", os.Getpid(),
		"workers", s.config.Workers)

	// Create cancellable context
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Start controller manager in background
	go s.manager.Start(ctx, s.config.Workers)

	// Start health controller's periodic health check loop
	s.healthCtrl.Start(ctx)

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Start gRPC server in background
	errCh := make(chan error, 1)
	go func() {
		errCh <- s.grpcServer.Serve(listener)
	}()

	// Wait for shutdown
	select {
	case <-ctx.Done():
		s.logger.Info("context cancelled, shutting down")
	case sig := <-sigCh:
		s.logger.Info("received signal, shutting down", "signal", sig)
	case err := <-errCh:
		if err != nil {
			s.logger.Error("gRPC server error", "error", err)
			return err
		}
	}

	return s.Shutdown()
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown() error {
	s.logger.Info("shutting down")

	// Stop health controller
	if s.healthCtrl != nil {
		s.healthCtrl.Stop()
	}

	// Graceful gRPC shutdown
	if s.grpcServer != nil {
		s.grpcServer.GracefulStop()
	}

	// Close listener
	if s.listener != nil {
		s.listener.Close()
	}

	// Close store
	if s.store != nil {
		s.store.Close()
	}

	// Clean up socket
	os.Remove(s.config.SocketPath)

	s.logger.Info("devnetd stopped")
	return nil
}
