// internal/daemon/runtime/process.go
package runtime

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/daemon/types"
)

// ProcessRuntimeConfig configures the process runtime
type ProcessRuntimeConfig struct {
	DataDir       string
	LogConfig     LogConfig
	PluginRuntime PluginRuntime
	Logger        *slog.Logger
}

// ProcessRuntime manages local processes
type ProcessRuntime struct {
	config      ProcessRuntimeConfig
	logManager  *LogManager
	supervisors map[string]*supervisor
	cmdOverride map[string][]string // for testing
	mu          sync.RWMutex
}

// NewProcessRuntime creates a new process runtime
func NewProcessRuntime(config ProcessRuntimeConfig) *ProcessRuntime {
	if config.LogConfig.MaxSize == 0 {
		config.LogConfig = DefaultLogConfig()
	}
	if config.Logger == nil {
		config.Logger = slog.Default()
	}

	logDir := filepath.Join(config.DataDir, "logs")

	return &ProcessRuntime{
		config:      config,
		logManager:  NewLogManager(logDir, config.LogConfig),
		supervisors: make(map[string]*supervisor),
		cmdOverride: make(map[string][]string),
	}
}

// SetCommandOverride sets a command override for testing
func (pr *ProcessRuntime) SetCommandOverride(nodeID string, cmd []string) {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	pr.cmdOverride[nodeID] = cmd
}

// StartNode starts a node process
func (pr *ProcessRuntime) StartNode(ctx context.Context, node *types.Node, opts StartOptions) error {
	pr.mu.Lock()
	defer pr.mu.Unlock()

	nodeID := node.Metadata.Name

	// Check if already running
	if sup, exists := pr.supervisors[nodeID]; exists {
		status := sup.status()
		if status.Running {
			return fmt.Errorf("node %s is already running", nodeID)
		}
	}

	// Determine command
	var command []string
	if override, ok := pr.cmdOverride[nodeID]; ok {
		command = override
	} else if pr.config.PluginRuntime != nil {
		command = pr.config.PluginRuntime.StartCommand(node)
	} else {
		// Default command
		command = []string{node.Spec.BinaryPath, "start", "--home", node.Spec.HomeDir}
	}

	// Set up log writer
	logPath := pr.logPath(node)
	logWriter, err := pr.logManager.GetWriter(nodeID, logPath)
	if err != nil {
		return fmt.Errorf("failed to create log writer: %w", err)
	}

	// Build environment
	env := make(map[string]string)
	if pr.config.PluginRuntime != nil {
		for k, v := range pr.config.PluginRuntime.StartEnv(node) {
			env[k] = v
		}
	}
	for k, v := range opts.Env {
		env[k] = v
	}

	// Determine signals
	stopSignal := syscall.SIGTERM
	gracePeriod := 10 * time.Second
	if pr.config.PluginRuntime != nil {
		stopSignal = pr.config.PluginRuntime.StopSignal()
		gracePeriod = pr.config.PluginRuntime.GracePeriod()
	}

	// Create supervisor
	sup := newSupervisor(supervisorConfig{
		command:     command,
		workDir:     node.Spec.HomeDir,
		env:         env,
		policy:      opts.RestartPolicy,
		logWriter:   logWriter,
		stopSignal:  stopSignal,
		gracePeriod: gracePeriod,
		logger:      pr.config.Logger,
	})

	pr.supervisors[nodeID] = sup

	// Start supervisor in background
	go sup.run(ctx)

	pr.config.Logger.Info("started node", "nodeID", nodeID, "command", command)
	return nil
}

// StopNode stops a node process
func (pr *ProcessRuntime) StopNode(ctx context.Context, nodeID string, graceful bool) error {
	pr.mu.Lock()
	sup, exists := pr.supervisors[nodeID]
	if !exists {
		pr.mu.Unlock()
		return fmt.Errorf("node %s not found", nodeID)
	}
	delete(pr.supervisors, nodeID)
	pr.mu.Unlock()

	if graceful {
		sup.stop()
	} else {
		sup.forceStop()
	}

	// Close log writer
	pr.logManager.Close(nodeID)

	pr.config.Logger.Info("stopped node", "nodeID", nodeID, "graceful", graceful)
	return nil
}

// RestartNode restarts a node process
func (pr *ProcessRuntime) RestartNode(ctx context.Context, nodeID string) error {
	pr.mu.RLock()
	sup, exists := pr.supervisors[nodeID]
	pr.mu.RUnlock()

	if !exists {
		return fmt.Errorf("node %s not found", nodeID)
	}

	status := sup.status()
	if !status.Running {
		return fmt.Errorf("node %s is not running", nodeID)
	}

	// Send stop signal - supervisor will restart based on policy
	sup.stopProcess()

	return nil
}

// GetNodeStatus returns the current status of a node
func (pr *ProcessRuntime) GetNodeStatus(ctx context.Context, nodeID string) (*NodeStatus, error) {
	pr.mu.RLock()
	sup, exists := pr.supervisors[nodeID]
	pr.mu.RUnlock()

	if !exists {
		return &NodeStatus{Running: false}, nil
	}

	status := sup.status()
	return &status, nil
}

// GetLogs returns logs for a node
func (pr *ProcessRuntime) GetLogs(ctx context.Context, nodeID string, opts LogOptions) (io.ReadCloser, error) {
	logPath := filepath.Join(pr.config.DataDir, "logs", nodeID+".log")
	return pr.logManager.GetReader(ctx, logPath, opts)
}

// Cleanup cleans up all resources
func (pr *ProcessRuntime) Cleanup(ctx context.Context) error {
	pr.mu.Lock()
	defer pr.mu.Unlock()

	for nodeID, sup := range pr.supervisors {
		sup.stop()
		pr.logManager.Close(nodeID)
	}

	pr.supervisors = make(map[string]*supervisor)
	return nil
}

// ExecInNode executes a command in a running node process.
// For ProcessRuntime, this is not directly supported since processes
// don't have the same isolation model as containers.
func (pr *ProcessRuntime) ExecInNode(ctx context.Context, nodeID string, command []string, timeout time.Duration) (*ExecResult, error) {
	return nil, fmt.Errorf("exec in node not supported for process runtime")
}

// logPath returns the log file path for a node
func (pr *ProcessRuntime) logPath(node *types.Node) string {
	return filepath.Join(pr.config.DataDir, "logs", node.Metadata.Name+".log")
}

// Ensure ProcessRuntime implements NodeRuntime interface
var _ NodeRuntime = (*ProcessRuntime)(nil)
