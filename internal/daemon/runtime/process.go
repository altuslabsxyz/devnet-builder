// internal/daemon/runtime/process.go
package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/daemon/types"
)

// ProcessRuntimeConfig configures the process runtime
type ProcessRuntimeConfig struct {
	DataDir               string
	LogConfig             LogConfig
	PluginRuntime         PluginRuntime         // Deprecated: use PluginRuntimeProvider
	PluginRuntimeProvider PluginRuntimeProvider // Provider to get PluginRuntime per network
	Logger                *slog.Logger
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
		// Clean up dead supervisor before creating a new one
		delete(pr.supervisors, nodeID)
		pr.logManager.Close(nodeID)
	}

	// Determine PluginRuntime: opts > provider (by network) > config default
	pluginRuntime := opts.PluginRuntime
	if pluginRuntime == nil && pr.config.PluginRuntimeProvider != nil {
		// Look up the node's network from the devnet
		pluginRuntime = pr.config.PluginRuntimeProvider.GetPluginRuntime(node.Spec.Network)
	}
	if pluginRuntime == nil {
		pluginRuntime = pr.config.PluginRuntime
	}

	// Determine command: PluginRuntime.StartCommand() returns args only
	// (designed for Docker entrypoint), so we prepend the binary path for
	// bare-metal execution.
	var command []string
	if override, ok := pr.cmdOverride[nodeID]; ok {
		command = override
	} else if pluginRuntime != nil {
		command = append([]string{node.Spec.BinaryPath}, pluginRuntime.StartCommand(node)...)
	} else {
		// Default command - used when PluginRuntime is not available
		// (e.g., existing nodes without Network field in NodeSpec)
		command = []string{node.Spec.BinaryPath, "start", "--home", node.Spec.HomeDir}

		// Append chain-id: prefer NodeSpec.ChainID, fallback to genesis file
		chainID := node.Spec.ChainID
		if chainID == "" {
			chainID = readChainIDFromGenesis(node.Spec.HomeDir)
		}
		if chainID != "" {
			command = append(command, "--chain-id", chainID)
		}
	}

	// Set up log writer
	logPath := pr.logPath(node)
	logWriter, err := pr.logManager.GetWriter(nodeID, logPath)
	if err != nil {
		return fmt.Errorf("failed to create log writer: %w", err)
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

	// Determine signals
	stopSignal := syscall.SIGTERM
	gracePeriod := 10 * time.Second
	if pluginRuntime != nil {
		stopSignal = pluginRuntime.StopSignal()
		gracePeriod = pluginRuntime.GracePeriod()
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

// Cleanup cleans up all resources by stopping all processes
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

// Detach detaches from all running processes without stopping them.
// Processes continue running as orphans (reparented to PID 1).
// Used for graceful devnetd shutdown where devnets should persist.
func (pr *ProcessRuntime) Detach() error {
	pr.mu.Lock()
	defer pr.mu.Unlock()

	pr.config.Logger.Info("detaching from all processes", "count", len(pr.supervisors))

	for nodeID, sup := range pr.supervisors {
		sup.detach()
		pr.logManager.Close(nodeID)
	}

	pr.supervisors = make(map[string]*supervisor)
	return nil
}

// ReconnectNode attempts to reconnect to an already-running node process.
// Returns true if the process is still running and was reconnected.
// Returns false if the process is not running (caller should restart it).
func (pr *ProcessRuntime) ReconnectNode(ctx context.Context, node *types.Node, storedPID int) (bool, error) {
	pr.mu.Lock()
	defer pr.mu.Unlock()

	nodeID := node.Metadata.Name

	// Check if already managed
	if _, exists := pr.supervisors[nodeID]; exists {
		return true, nil
	}

	// Check if process is still alive using signal 0
	proc, err := os.FindProcess(storedPID)
	if err != nil {
		pr.config.Logger.Debug("process not found",
			"nodeID", nodeID,
			"pid", storedPID,
			"error", err)
		return false, nil
	}

	if err := proc.Signal(syscall.Signal(0)); err != nil {
		pr.config.Logger.Debug("process not running",
			"nodeID", nodeID,
			"pid", storedPID,
			"error", err)
		return false, nil
	}

	// Validate it's the same process (guard against PID reuse)
	if !pr.validateProcess(storedPID, node) {
		pr.config.Logger.Warn("PID reused by different process",
			"nodeID", nodeID,
			"pid", storedPID)
		return false, nil
	}

	// Reopen log writer in append mode
	logPath := pr.logPath(node)
	logWriter, err := pr.logManager.GetWriter(nodeID, logPath)
	if err != nil {
		pr.config.Logger.Warn("failed to reopen log writer",
			"nodeID", nodeID,
			"error", err)
		// Continue anyway - process is running, logs might not work perfectly
	}

	// Create a monitoring-only supervisor
	sup := newMonitoringSupervisor(storedPID, logWriter, pr.config.Logger)
	pr.supervisors[nodeID] = sup

	// Start monitoring in background
	go sup.runMonitor(ctx)

	pr.config.Logger.Info("reconnected to running process",
		"nodeID", nodeID,
		"pid", storedPID)

	return true, nil
}

// ReconnectAll attempts to reconnect to all previously-running nodes.
// Called at daemon startup to reattach to orphaned processes.
// Returns the count of successfully reconnected nodes.
func (pr *ProcessRuntime) ReconnectAll(ctx context.Context, nodes []*types.Node) (int, error) {
	pr.config.Logger.Info("attempting to reconnect to running processes",
		"nodeCount", len(nodes))

	reconnected := 0
	for _, node := range nodes {
		// Skip nodes without stored PID
		if node.Status.PID == 0 {
			continue
		}
		// Skip nodes that weren't running
		if node.Status.Phase != types.NodePhaseRunning {
			continue
		}

		ok, err := pr.ReconnectNode(ctx, node, node.Status.PID)
		if err != nil {
			pr.config.Logger.Warn("failed to reconnect to node",
				"nodeID", node.Metadata.Name,
				"pid", node.Status.PID,
				"error", err)
			continue
		}
		if ok {
			reconnected++
		} else {
			pr.config.Logger.Info("process no longer running, will be restarted by controller",
				"nodeID", node.Metadata.Name,
				"pid", node.Status.PID)
		}
	}

	pr.config.Logger.Info("process reconnection complete",
		"reconnected", reconnected,
		"total", len(nodes))

	return reconnected, nil
}

// validateProcess checks if the PID belongs to the expected node process.
// This guards against PID reuse edge cases.
func (pr *ProcessRuntime) validateProcess(pid int, node *types.Node) bool {
	cmdline, err := readProcessCmdline(pid)
	if err != nil {
		// Can't read cmdline (permission denied, /proc not available on macOS)
		// Fall back to less reliable validation
		return pr.validateProcessFallback(pid, node)
	}

	if len(cmdline) == 0 {
		return pr.validateProcessFallback(pid, node)
	}

	// Check if the command matches expected binary
	if node.Spec.BinaryPath != "" && strings.Contains(cmdline[0], node.Spec.BinaryPath) {
		return true
	}

	// Check if home directory appears in arguments
	if node.Spec.HomeDir != "" {
		for _, arg := range cmdline {
			if strings.Contains(arg, node.Spec.HomeDir) {
				return true
			}
		}
	}

	return false
}

// validateProcessFallback uses ps command when /proc is not available (macOS)
func (pr *ProcessRuntime) validateProcessFallback(pid int, node *types.Node) bool {
	// Use ps to get command line
	cmd := exec.Command("ps", "-o", "command=", "-p", fmt.Sprintf("%d", pid))
	out, err := cmd.Output()
	if err != nil {
		// Can't validate, assume it's the right process
		// (better to reconnect than to restart unnecessarily)
		return true
	}

	cmdline := string(out)

	// Check if binary path appears in command
	if node.Spec.BinaryPath != "" && strings.Contains(cmdline, node.Spec.BinaryPath) {
		return true
	}

	// Check if home directory appears in command
	if node.Spec.HomeDir != "" && strings.Contains(cmdline, node.Spec.HomeDir) {
		return true
	}

	return false
}

// readProcessCmdline reads the command line of a process from /proc (Linux only)
func readProcessCmdline(pid int) ([]string, error) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil {
		return nil, err
	}

	// Parse null-separated args
	if len(data) == 0 {
		return nil, nil
	}

	// Remove trailing null if present
	if data[len(data)-1] == 0 {
		data = data[:len(data)-1]
	}

	args := strings.Split(string(data), "\x00")
	return args, nil
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

// readChainIDFromGenesis reads the chain_id from the genesis.json file in the node's home directory.
// This is used as a fallback for existing nodes that don't have PluginRuntime available.
// Returns empty string if the file cannot be read or parsed.
func readChainIDFromGenesis(homeDir string) string {
	genesisPath := filepath.Join(homeDir, "config", "genesis.json")
	data, err := os.ReadFile(genesisPath)
	if err != nil {
		return ""
	}

	var genesis struct {
		ChainID string `json:"chain_id"`
	}
	if err := json.Unmarshal(data, &genesis); err != nil {
		return ""
	}

	return genesis.ChainID
}
