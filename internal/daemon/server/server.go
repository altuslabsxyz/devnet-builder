// internal/daemon/server/server.go
package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	v1 "github.com/altuslabsxyz/devnet-builder/api/proto/gen/v1"
	"github.com/altuslabsxyz/devnet-builder/internal/application/ports"
	"github.com/altuslabsxyz/devnet-builder/internal/auth"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/checker"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/controller"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/provisioner"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/runtime"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/server/ante"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/store"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/subnet"
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
	// ShutdownTimeout is the graceful shutdown timeout.
	ShutdownTimeout time.Duration
	// HealthCheckTimeout is the RPC health check timeout.
	HealthCheckTimeout time.Duration
	// GitHubToken is the GitHub API token.
	GitHubToken string

	// Remote listener settings (optional - enables remote access)
	// Listen is the TCP address to listen on (e.g., "0.0.0.0:9000").
	// Empty means local-only mode (Unix socket only).
	Listen string
	// TLSCert is the path to the TLS certificate file.
	TLSCert string
	// TLSKey is the path to the TLS private key file.
	TLSKey string

	// Authentication settings
	// AuthEnabled enables API key authentication for remote connections.
	AuthEnabled bool
	// AuthKeysFile is the path to the API keys file.
	AuthKeysFile string
}

// DefaultConfig returns default configuration.
func DefaultConfig() *Config {
	home, _ := os.UserHomeDir()
	dataDir := filepath.Join(home, ".devnet-builder")
	return &Config{
		SocketPath:         filepath.Join(dataDir, "devnetd.sock"),
		DataDir:            dataDir,
		Foreground:         false,
		Workers:            2,
		LogLevel:           "info",
		ShutdownTimeout:    30 * time.Second,
		HealthCheckTimeout: 5 * time.Second,
		GitHubToken:        "",
	}
}

// Server is the devnetd daemon server.
type Server struct {
	config          *Config
	store           store.Store
	manager         *controller.Manager
	healthCtrl      *controller.HealthController
	pluginManager   *PluginManager
	subnetAllocator *subnet.Allocator
	grpcServer      *grpc.Server
	listener        net.Listener // Unix socket listener
	tcpListener     net.Listener // TCP/TLS listener (optional)
	logger          *slog.Logger
	logFile         *os.File // Log file handle for cleanup
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

	// Initialize subnet allocator for loopback network aliasing
	subnetAllocatorPath := filepath.Join(config.DataDir, "subnets.json")
	subnetAlloc, err := subnet.LoadOrCreate(subnetAllocatorPath)
	if err != nil {
		st.Close()
		pluginMgr.Close()
		return nil, fmt.Errorf("failed to initialize subnet allocator: %w", err)
	}
	logger.Info("subnet allocator initialized", "path", subnetAllocatorPath)

	// Create controller manager
	mgr := controller.NewManager()
	mgr.SetLogger(logger)

	// Create orchestrator factory for full provisioning flow (build, fork, init)
	orchFactory := NewOrchestratorFactory(config.DataDir, logger)

	// Create devnet provisioner with orchestrator factory and subnet allocator
	// The factory enables full provisioning (build, fork, init) before creating Node resources
	// The subnet allocator assigns unique loopback subnets to each devnet
	devnetProv := provisioner.NewDevnetProvisioner(st, provisioner.Config{
		DataDir:             config.DataDir,
		Logger:              logger,
		OrchestratorFactory: orchFactory,
		SubnetAllocator:     subnetAlloc,
	})

	// Register controllers
	devnetCtrl := controller.NewDevnetController(st, devnetProv)
	devnetCtrl.SetLogger(logger)
	mgr.Register("devnets", devnetCtrl)

	// Wire step progress reporter to broadcast provision logs to CLI clients
	devnetProv.SetStepProgressReporterFactory(func(namespace, name string) ports.ProgressReporter {
		return ports.ProgressFunc(func(step ports.StepProgress) {
			devnetCtrl.BroadcastProvisionLog(namespace, name, &controller.ProvisionLogEntry{
				Timestamp:       time.Now(),
				Level:           "info",
				Message:         step.Name,
				Phase:           "genesis-fork",
				StepName:        step.Name,
				StepStatus:      step.Status,
				ProgressCurrent: step.Current,
				ProgressTotal:   step.Total,
				ProgressUnit:    step.Unit,
				StepDetail:      step.Detail,
			})
		})
	})

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
		Logger:  logger,
		Timeout: config.HealthCheckTimeout,
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

	// Create gRPC server with optional auth interceptors for remote mode
	var grpcServer *grpc.Server
	if config.Listen != "" && config.AuthEnabled {
		// Load API key store for authentication.
		// NOTE: Keys are loaded once at startup. After creating or revoking keys
		// with `devnetd keys create/revoke`, the server must be restarted for
		// changes to take effect. Consider implementing hot-reload in the future.
		keysFile := config.AuthKeysFile
		if keysFile == "" {
			keysFile = filepath.Join(config.DataDir, "api-keys.yaml")
		}
		keyStore := auth.NewFileKeyStore(keysFile)
		if err := keyStore.Load(); err != nil {
			logger.Warn("failed to load API keys, starting with empty key store", "error", err)
		}

		// Create gRPC server with auth interceptors
		grpcServer = grpc.NewServer(
			grpc.ChainUnaryInterceptor(auth.NewAuthInterceptor(keyStore, IsLocalConnection)),
			grpc.ChainStreamInterceptor(auth.NewStreamAuthInterceptor(keyStore, IsLocalConnection)),
		)
		logger.Info("authentication enabled for remote connections")
	} else {
		grpcServer = grpc.NewServer()
	}

	// Create network service first (needed by ante handler)
	githubFactory := NewDefaultGitHubClientFactory(config.DataDir, logger)
	networkSvc := NewNetworkService(githubFactory)
	networkSvc.SetLogger(logger)

	// Create ante handler for request validation
	anteHandler := ante.New(st, networkSvc)

	// Register services
	devnetSvc := NewDevnetServiceWithAnte(st, mgr, anteHandler, subnetAlloc)
	devnetSvc.SetLogger(logger)
	v1.RegisterDevnetServiceServer(grpcServer, devnetSvc)

	nodeSvc := NewNodeServiceWithAnte(st, mgr, nodeRuntime, anteHandler)
	nodeSvc.SetLogger(logger)
	v1.RegisterNodeServiceServer(grpcServer, nodeSvc)

	upgradeSvc := NewUpgradeServiceWithAnte(st, mgr, anteHandler)
	upgradeSvc.SetLogger(logger)
	v1.RegisterUpgradeServiceServer(grpcServer, upgradeSvc)

	txSvc := NewTransactionService(st, mgr)
	txSvc.SetLogger(logger)
	v1.RegisterTransactionServiceServer(grpcServer, txSvc)

	v1.RegisterNetworkServiceServer(grpcServer, networkSvc)

	// Register auth service for ping/whoami
	authSvc := NewAuthService()
	v1.RegisterAuthServiceServer(grpcServer, authSvc)

	return &Server{
		config:          config,
		store:           st,
		manager:         mgr,
		healthCtrl:      healthCtrl,
		pluginManager:   pluginMgr,
		subnetAllocator: subnetAlloc,
		grpcServer:      grpcServer,
		logger:          logger,
		logFile:         logFile,
	}, nil
}

// Run starts the server and blocks until shutdown.
func (s *Server) Run(ctx context.Context) error {
	// Remove stale socket
	os.Remove(s.config.SocketPath)

	// Create Unix socket listener (always available for local access)
	listener, err := net.Listen("unix", s.config.SocketPath)
	if err != nil {
		return fmt.Errorf("failed to listen on unix socket: %w", err)
	}
	s.listener = listener

	// Create TCP/TLS listener if configured (for remote access)
	if s.config.Listen != "" {
		tcpListener, err := s.createTLSListener()
		if err != nil {
			s.listener.Close()
			return fmt.Errorf("failed to create TCP/TLS listener: %w", err)
		}
		s.tcpListener = tcpListener
	}

	// Write PID file
	pidPath := filepath.Join(s.config.DataDir, "devnetd.pid")
	if err := os.WriteFile(pidPath, []byte(fmt.Sprintf("%d", os.Getpid())), 0644); err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}
	defer os.Remove(pidPath)

	logAttrs := []any{
		"socket", s.config.SocketPath,
		"dataDir", s.config.DataDir,
		"pid", os.Getpid(),
		"workers", s.config.Workers,
	}
	if s.config.Listen != "" {
		logAttrs = append(logAttrs, "listen", s.config.Listen)
	}
	s.logger.Info("devnetd started", logAttrs...)

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

	// Start gRPC server on Unix socket in background
	errCh := make(chan error, 2) // Buffer for both listeners
	go func() {
		errCh <- s.grpcServer.Serve(listener)
	}()

	// Start gRPC server on TCP/TLS listener if configured
	if s.tcpListener != nil {
		go func() {
			errCh <- s.grpcServer.Serve(s.tcpListener)
		}()
	}

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

	// Cancel context BEFORE shutdown to allow workers to exit gracefully.
	// This is critical: Manager.Start() waits on ctx.Done() before signaling
	// that workers have stopped. If we don't cancel here, Shutdown() will
	// deadlock waiting for workers that are waiting for context cancellation.
	cancel()

	return s.Shutdown()
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown() error {
	s.logger.Info("shutting down")

	// Stop health controller
	if s.healthCtrl != nil {
		s.healthCtrl.Stop()
	}

	// Stop controller manager and wait for all workers to complete.
	// This MUST happen before closing the store to prevent "database not open" errors.
	// Use a timeout to prevent hanging if workers are blocked on external processes
	// (e.g., a Cosmos SDK binary deadlocked during genesis export).
	if s.manager != nil {
		s.logger.Debug("waiting for controller workers to stop", "timeout", s.config.ShutdownTimeout)
		graceful := s.manager.StopWithTimeout(s.config.ShutdownTimeout)
		if graceful {
			s.logger.Debug("controller workers stopped gracefully")
		} else {
			s.logger.Warn("controller workers did not stop within timeout, proceeding with shutdown")
		}
	}

	// Graceful gRPC shutdown
	if s.grpcServer != nil {
		s.grpcServer.GracefulStop()
	}

	// Close listeners
	if s.listener != nil {
		s.listener.Close()
	}
	if s.tcpListener != nil {
		s.tcpListener.Close()
	}

	// Close store (safe now that all workers have stopped)
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

// createTLSListener creates a TCP listener with TLS configured.
// NOTE: TLS certificates are loaded once at startup. For certificate rotation,
// the server must be restarted. For production deployments requiring zero-downtime
// rotation, consider using tls.Config.GetCertificate callback or a reverse proxy.
func (s *Server) createTLSListener() (net.Listener, error) {
	// Load TLS certificate and key
	cert, err := tls.LoadX509KeyPair(s.config.TLSCert, s.config.TLSKey)
	if err != nil {
		return nil, fmt.Errorf("failed to load TLS credentials: %w", err)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}

	// Create TCP listener with TLS
	listener, err := tls.Listen("tcp", s.config.Listen, tlsConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on %s: %w", s.config.Listen, err)
	}

	s.logger.Info("TCP/TLS listener started", "address", s.config.Listen)
	return listener, nil
}
