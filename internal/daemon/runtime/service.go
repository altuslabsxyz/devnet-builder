package runtime

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"sync"
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/daemon/types"
)

// ServiceRuntimeConfig configures the service runtime.
type ServiceRuntimeConfig struct {
	DataDir               string
	LogConfig             LogConfig
	PluginRuntimeProvider PluginRuntimeProvider
	Logger                *slog.Logger
}

// serviceInfo tracks a managed OS service.
type serviceInfo struct {
	serviceID string
	nodeID    string
}

// ServiceRuntime manages node processes via OS service managers
// (launchd on macOS, systemd on Linux).
type ServiceRuntime struct {
	config     ServiceRuntimeConfig
	backend    ServiceBackend
	logManager *LogManager
	services   map[string]serviceInfo // nodeID -> serviceInfo
	mu         sync.RWMutex
}

// NewServiceRuntime creates a new service runtime backed by the platform's
// service manager.
func NewServiceRuntime(config ServiceRuntimeConfig) (*ServiceRuntime, error) {
	if config.LogConfig.MaxSize == 0 {
		config.LogConfig = DefaultLogConfig()
	}
	if config.Logger == nil {
		config.Logger = slog.Default()
	}

	backend, err := NewServiceBackend()
	if err != nil {
		return nil, fmt.Errorf("failed to create service backend: %w", err)
	}

	logDir := filepath.Join(config.DataDir, "logs")

	return &ServiceRuntime{
		config:     config,
		backend:    backend,
		logManager: NewLogManager(logDir, config.LogConfig),
		services:   make(map[string]serviceInfo),
	}, nil
}

// StartNode installs and starts a node as an OS-managed service.
func (sr *ServiceRuntime) StartNode(ctx context.Context, node *types.Node, opts StartOptions) error {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	nodeID := node.Metadata.Name

	// Check if already managed
	if info, exists := sr.services[nodeID]; exists {
		status, err := sr.backend.GetServiceStatus(ctx, info.serviceID)
		if err == nil && status.Running {
			return fmt.Errorf("node %s is already running", nodeID)
		}
		// Not running — clean up stale entry
		_ = sr.backend.UninstallService(ctx, info.serviceID)
		delete(sr.services, nodeID)
	}

	// Resolve PluginRuntime
	pluginRuntime := opts.PluginRuntime
	if pluginRuntime == nil && sr.config.PluginRuntimeProvider != nil {
		pluginRuntime = sr.config.PluginRuntimeProvider.GetPluginRuntime(node.Spec.Network)
	}

	// Determine command
	var command []string
	if pluginRuntime != nil {
		command = pluginRuntime.StartCommand(node)
	} else {
		command = []string{node.Spec.BinaryPath, "start", "--home", node.Spec.HomeDir}
		chainID := node.Spec.ChainID
		if chainID == "" {
			chainID = readChainIDFromGenesis(node.Spec.HomeDir)
		}
		if chainID != "" {
			command = append(command, "--chain-id", chainID)
		}
	}

	// Build environment
	env := make(map[string]string)
	if pluginRuntime != nil {
		for k, v := range pluginRuntime.StartEnv(node) {
			env[k] = v
		}
	}
	for k, v := range opts.Env {
		env[k] = v
	}

	// Determine grace period
	gracePeriod := 30 * time.Second
	if pluginRuntime != nil {
		gracePeriod = pluginRuntime.GracePeriod()
	}

	// Determine restart behavior
	restartOnFailure := opts.RestartPolicy.Policy == "on-failure" || opts.RestartPolicy.Policy == "always"

	// Build service definition
	serviceID := sr.backend.ServiceID(nodeID)
	logPath := sr.logPath(node)
	def := &ServiceDefinition{
		ID:               serviceID,
		NodeID:           nodeID,
		Command:          command,
		WorkingDirectory: node.Spec.HomeDir,
		Environment:      env,
		StdoutPath:       logPath,
		StderrPath:       logPath,
		RestartOnFailure: restartOnFailure,
		GracePeriod:      gracePeriod,
	}

	// Install and start
	if err := sr.backend.InstallService(ctx, def); err != nil {
		return fmt.Errorf("failed to install service for node %s: %w", nodeID, err)
	}

	if err := sr.backend.StartService(ctx, serviceID); err != nil {
		_ = sr.backend.UninstallService(ctx, serviceID)
		return fmt.Errorf("failed to start service for node %s: %w", nodeID, err)
	}

	sr.services[nodeID] = serviceInfo{
		serviceID: serviceID,
		nodeID:    nodeID,
	}

	sr.config.Logger.Info("started node via service manager",
		"nodeID", nodeID,
		"serviceID", serviceID,
		"command", command)

	return nil
}

// StopNode stops and uninstalls a node's OS service.
func (sr *ServiceRuntime) StopNode(ctx context.Context, nodeID string, graceful bool) error {
	sr.mu.Lock()
	info, exists := sr.services[nodeID]
	if !exists {
		sr.mu.Unlock()
		return fmt.Errorf("node %s not found", nodeID)
	}
	delete(sr.services, nodeID)
	sr.mu.Unlock()

	// Stop the service
	if err := sr.backend.StopService(ctx, info.serviceID, !graceful); err != nil {
		sr.config.Logger.Warn("failed to stop service",
			"nodeID", nodeID,
			"error", err)
	}

	// Uninstall to prevent auto-restart and clean up
	if err := sr.backend.UninstallService(ctx, info.serviceID); err != nil {
		sr.config.Logger.Warn("failed to uninstall service",
			"nodeID", nodeID,
			"error", err)
	}

	sr.config.Logger.Info("stopped node service",
		"nodeID", nodeID,
		"graceful", graceful)

	return nil
}

// RestartNode restarts a node's OS service.
func (sr *ServiceRuntime) RestartNode(ctx context.Context, nodeID string) error {
	sr.mu.RLock()
	info, exists := sr.services[nodeID]
	sr.mu.RUnlock()

	if !exists {
		return fmt.Errorf("node %s not found", nodeID)
	}

	return sr.backend.RestartService(ctx, info.serviceID)
}

// GetNodeStatus queries the OS service manager for the node's status.
func (sr *ServiceRuntime) GetNodeStatus(ctx context.Context, nodeID string) (*NodeStatus, error) {
	sr.mu.RLock()
	info, exists := sr.services[nodeID]
	sr.mu.RUnlock()

	if !exists {
		return &NodeStatus{Running: false}, nil
	}

	svcStatus, err := sr.backend.GetServiceStatus(ctx, info.serviceID)
	if err != nil {
		return &NodeStatus{Running: false}, nil
	}

	return &NodeStatus{
		Running:   svcStatus.Running,
		PID:       svcStatus.PID,
		ExitCode:  svcStatus.ExitCode,
		StartedAt: svcStatus.StartedAt,
		UpdatedAt: time.Now(),
	}, nil
}

// GetLogs returns logs for a node (same log paths as ProcessRuntime).
func (sr *ServiceRuntime) GetLogs(ctx context.Context, nodeID string, opts LogOptions) (io.ReadCloser, error) {
	logPath := filepath.Join(sr.config.DataDir, "logs", nodeID+".log")
	return sr.logManager.GetReader(ctx, logPath, opts)
}

// ExecInNode is not supported for service runtime.
func (sr *ServiceRuntime) ExecInNode(ctx context.Context, nodeID string, command []string, timeout time.Duration) (*ExecResult, error) {
	return nil, fmt.Errorf("exec in node not supported for service runtime")
}

// Cleanup uninstalls all tracked services.
func (sr *ServiceRuntime) Cleanup(ctx context.Context) error {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	for nodeID, info := range sr.services {
		if err := sr.backend.UninstallService(ctx, info.serviceID); err != nil {
			sr.config.Logger.Warn("failed to uninstall service during cleanup",
				"nodeID", nodeID,
				"error", err)
		}
	}

	sr.services = make(map[string]serviceInfo)
	return nil
}

// DiscoverExisting checks which nodes still have OS services installed.
// Called on daemon startup to reattach to services that survived daemon restart.
// Returns the count of discovered services.
func (sr *ServiceRuntime) DiscoverExisting(ctx context.Context, nodes []*types.Node) (int, error) {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	sr.config.Logger.Info("discovering existing OS-managed services",
		"nodeCount", len(nodes))

	discovered := 0
	for _, node := range nodes {
		if node.Status.Phase != types.NodePhaseRunning {
			continue
		}

		nodeID := node.Metadata.Name
		serviceID := sr.backend.ServiceID(nodeID)

		installed, err := sr.backend.IsInstalled(ctx, serviceID)
		if err != nil {
			sr.config.Logger.Warn("failed to check service installation",
				"nodeID", nodeID,
				"error", err)
			continue
		}

		if !installed {
			sr.config.Logger.Info("service not found, will be restarted by controller",
				"nodeID", nodeID,
				"serviceID", serviceID)
			continue
		}

		sr.services[nodeID] = serviceInfo{
			serviceID: serviceID,
			nodeID:    nodeID,
		}
		discovered++

		sr.config.Logger.Info("discovered existing service",
			"nodeID", nodeID,
			"serviceID", serviceID)
	}

	sr.config.Logger.Info("service discovery complete",
		"discovered", discovered,
		"total", len(nodes))

	return discovered, nil
}

// logPath returns the log file path for a node.
func (sr *ServiceRuntime) logPath(node *types.Node) string {
	return filepath.Join(sr.config.DataDir, "logs", node.Metadata.Name+".log")
}

// Ensure ServiceRuntime implements NodeRuntime interface.
var _ NodeRuntime = (*ServiceRuntime)(nil)
