// internal/daemon/server/server.go
package server

import (
	"context"
	"fmt"
	"io"
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
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/upgrader"
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
	config        *Config
	store         store.Store
	manager       *controller.Manager
	healthCtrl    *controller.HealthController
	pluginManager *PluginManager
	grpcServer    *grpc.Server
	listener      net.Listener
	logger        *slog.Logger
	logFile       *os.File // Log file handle for cleanup
}

// New creates a new server.
func New(config *Config) (*Server, error) {
	// Ensure data directory exists first (needed for log file)
	if err := os.MkdirAll(config.DataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	// Set up logger - write to both stdout and log file for debugging
	level := slog.LevelInfo
	switch config.LogLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}

	// Create log file for persistent logging (used by 'dvb daemon logs')
	logFilePath := filepath.Join(config.DataDir, "daemon.log")
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open daemon log file: %w", err)
	}

	// Write logs to both stdout and file
	multiWriter := io.MultiWriter(os.Stdout, logFile)
	logger := slog.New(slog.NewTextHandler(multiWriter, &slog.HandlerOptions{Level: level}))

	// Load network plugins from plugin directories
	// Plugins are discovered from ~/.devnet-builder/plugins/ and registered
	// with the global network registry so they can be queried via NetworkService
	pluginMgr := NewPluginManager(PluginManagerConfig{
		PluginDirs: []string{filepath.Join(config.DataDir, "plugins")},
		Logger:     logger,
	})

	result, err := pluginMgr.LoadAndRegister()
	if err != nil {
		return nil, fmt.Errorf("failed to load plugins: %w", err)
	}

	if len(result.Loaded) > 0 {
		logger.Info("network plugins loaded",
			"count", len(result.Loaded),
			"plugins", result.Loaded)
	}
	if len(result.Errors) > 0 {
		for _, e := range result.Errors {
			logger.Warn("plugin load error",
				"plugin", e.Name,
				"error", e.Error)
		}
	}

	// Open state store
	dbPath := filepath.Join(config.DataDir, "devnetd.db")
	st, err := store.NewBoltStore(dbPath)
	if err != nil {
		pluginMgr.Close()
		return nil, fmt.Errorf("failed to open state store: %w", err)
	}

	// Create controller manager
	mgr := controller.NewManager()
	mgr.SetLogger(logger)

	// Create orchestrator factory for full provisioning flow (build, fork, init)
	orchFactory := NewOrchestratorFactory(config.DataDir, logger)

	// Create devnet provisioner with orchestrator factory
	// The factory enables full provisioning (build, fork, init) before creating Node resources
	devnetProv := provisioner.NewDevnetProvisioner(st, provisioner.Config{
		DataDir:             config.DataDir,
		Logger:              logger,
		OrchestratorFactory: orchFactory,
	})

	// Register controllers
	devnetCtrl := controller.NewDevnetController(st, devnetProv)
	devnetCtrl.SetLogger(logger)
	mgr.Register("devnets", devnetCtrl)

	// Create node runtime (Docker or nil)
	var nodeRuntime runtime.NodeRuntime
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

	// Create upgrade runtime
	upgradeRuntime := upgrader.NewRuntime(st, upgrader.Config{
		Logger: logger,
	})

	// Create and register upgrade controller
	upgradeCtrl := controller.NewUpgradeController(st, upgradeRuntime)
	upgradeCtrl.SetLogger(logger)
	mgr.Register("upgrades", upgradeCtrl)

	// Create and register transaction controller
	// TxRuntime is nil for now - will be connected when network plugins are loaded
	txCtrl := controller.NewTxController(st, nil)
	txCtrl.SetLogger(logger)
	mgr.Register("transactions", txCtrl)

	// Create gRPC server
	grpcServer := grpc.NewServer()

	// Register services
	devnetSvc := NewDevnetService(st, mgr)
	devnetSvc.SetLogger(logger)
	v1.RegisterDevnetServiceServer(grpcServer, devnetSvc)

	nodeSvc := NewNodeService(st, mgr, nodeRuntime)
	nodeSvc.SetLogger(logger)
	v1.RegisterNodeServiceServer(grpcServer, nodeSvc)

	upgradeSvc := NewUpgradeService(st, mgr)
	upgradeSvc.SetLogger(logger)
	v1.RegisterUpgradeServiceServer(grpcServer, upgradeSvc)

	txSvc := NewTransactionService(st, mgr)
	txSvc.SetLogger(logger)
	v1.RegisterTransactionServiceServer(grpcServer, txSvc)

	networkSvc := NewNetworkService()
	networkSvc.SetLogger(logger)
	v1.RegisterNetworkServiceServer(grpcServer, networkSvc)

	return &Server{
		config:        config,
		store:         st,
		manager:       mgr,
		healthCtrl:    healthCtrl,
		pluginManager: pluginMgr,
		grpcServer:    grpcServer,
		logger:        logger,
		logFile:       logFile,
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

	// Close plugin manager
	if s.pluginManager != nil {
		s.pluginManager.Close()
	}

	// Close log file
	if s.logFile != nil {
		s.logFile.Close()
	}

	// Clean up socket
	os.Remove(s.config.SocketPath)

	s.logger.Info("devnetd stopped")
	return nil
}
