// internal/daemon/provisioner/orchestrator.go
package provisioner

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/application/ports"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/builder"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/controller"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/runtime"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/types"
)

// =============================================================================
// Provisioning Phase Constants
// =============================================================================

// ProvisioningPhase represents the current phase of the provisioning process
type ProvisioningPhase string

const (
	// PhasePending is the initial state before provisioning starts
	PhasePending ProvisioningPhase = "Pending"

	// PhaseBuilding indicates the binary is being built from git
	PhaseBuilding ProvisioningPhase = "Building"

	// PhaseForking indicates genesis is being forked from source
	PhaseForking ProvisioningPhase = "Forking"

	// PhaseInitializing indicates node directories are being initialized
	PhaseInitializing ProvisioningPhase = "Initializing"

	// PhaseStarting indicates node processes are being started
	PhaseStarting ProvisioningPhase = "Starting"

	// PhaseHealthChecking indicates nodes are being verified healthy
	PhaseHealthChecking ProvisioningPhase = "HealthChecking"

	// PhaseRunning indicates the devnet is operational
	PhaseRunning ProvisioningPhase = "Running"

	// PhaseDegraded indicates one or more nodes are unhealthy
	PhaseDegraded ProvisioningPhase = "Degraded"

	// PhaseFailed indicates provisioning failed
	PhaseFailed ProvisioningPhase = "Failed"
)

// =============================================================================
// Progress Callback
// =============================================================================

// ProgressCallback is called when the provisioning phase changes or progress is made
type ProgressCallback func(phase ProvisioningPhase, message string)

// =============================================================================
// Binary Path Updater Interface
// =============================================================================

// BinaryPathUpdater is an optional interface that NodeInitializer implementations
// can implement to support deferred binary path injection. This is needed in daemon
// mode where the orchestrator is created before the binary is built.
//
// If a NodeInitializer implements this interface, the orchestrator will call
// SetBinaryPath after the build phase completes, before the init phase begins.
type BinaryPathUpdater interface {
	// SetBinaryPath sets the binary path for use in subsequent operations.
	SetBinaryPath(path string)
}

// =============================================================================
// Orchestrator Configuration
// =============================================================================

// OrchestratorConfig contains the injectable dependencies for ProvisioningOrchestrator
type OrchestratorConfig struct {
	// BinaryBuilder builds binaries from git sources
	BinaryBuilder builder.BinaryBuilder

	// GenesisForker handles genesis forking from various sources
	GenesisForker ports.GenesisForker

	// NodeInitializer initializes node directories
	NodeInitializer ports.NodeInitializer

	// NodeRuntime manages node processes/containers
	NodeRuntime runtime.NodeRuntime

	// HealthChecker verifies node health during provisioning (optional)
	HealthChecker controller.HealthChecker

	// DataDir is the base directory for devnet data
	DataDir string

	// Logger for logging provisioning progress
	Logger *slog.Logger
}

// =============================================================================
// Provisioning Orchestrator
// =============================================================================

// ProvisioningOrchestrator coordinates the full provisioning flow using injected
// component interfaces. It manages the state machine that progresses through
// building, forking, initializing, and starting phases.
type ProvisioningOrchestrator struct {
	config   OrchestratorConfig
	logger   *slog.Logger
	mu       sync.RWMutex
	phase    ProvisioningPhase
	lastErr  error
	progress ProgressCallback
}

// NewProvisioningOrchestrator creates a new provisioning orchestrator with the given config
func NewProvisioningOrchestrator(config OrchestratorConfig) *ProvisioningOrchestrator {
	logger := config.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &ProvisioningOrchestrator{
		config: config,
		logger: logger,
		phase:  PhasePending,
	}
}

// CurrentPhase returns the current provisioning phase
func (o *ProvisioningOrchestrator) CurrentPhase() ProvisioningPhase {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.phase
}

// GetError returns the last error that occurred during provisioning
func (o *ProvisioningOrchestrator) GetError() error {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.lastErr
}

// OnProgress sets the progress callback
func (o *ProvisioningOrchestrator) OnProgress(callback ProgressCallback) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.progress = callback
}

// setPhase updates the current phase and notifies the progress callback
func (o *ProvisioningOrchestrator) setPhase(phase ProvisioningPhase, message string) {
	o.mu.Lock()
	o.phase = phase
	callback := o.progress
	o.mu.Unlock()

	o.logger.Info("provisioning phase changed",
		"phase", phase,
		"message", message,
	)

	if callback != nil {
		callback(phase, message)
	}
}

// setError records an error and transitions to Failed phase
func (o *ProvisioningOrchestrator) setError(err error) {
	o.mu.Lock()
	o.lastErr = err
	o.phase = PhaseFailed
	callback := o.progress
	o.mu.Unlock()

	o.logger.Error("provisioning failed",
		"error", err,
	)

	if callback != nil {
		callback(PhaseFailed, err.Error())
	}
}

// Execute runs the full provisioning flow
func (o *ProvisioningOrchestrator) Execute(ctx context.Context, opts ports.ProvisionOptions) (*ports.ProvisionResult, error) {
	o.logger.Info("orchestrator starting provisioning",
		"devnet", opts.DevnetName,
		"chainID", opts.ChainID,
		"network", opts.Network,
		"validators", opts.NumValidators,
		"fullnodes", opts.NumFullNodes,
	)

	// Check for context cancellation before starting
	if err := ctx.Err(); err != nil {
		o.setError(fmt.Errorf("context cancelled: %w", err))
		return nil, o.lastErr
	}

	// Track the binary path (may be provided or built)
	binaryPath := opts.BinaryPath

	// Phase 1: Building (if no binary provided)
	if binaryPath == "" {
		if err := ctx.Err(); err != nil {
			o.setError(fmt.Errorf("context cancelled: %w", err))
			return nil, o.lastErr
		}

		o.setPhase(PhaseBuilding, "Building binary from source")

		buildResult, err := o.executeBuildPhase(ctx, opts)
		if err != nil {
			o.setError(fmt.Errorf("building phase failed: %w", err))
			return nil, o.lastErr
		}
		binaryPath = buildResult.BinaryPath
	}

	// Update NodeInitializer with binary path if it supports deferred injection.
	// This is needed in daemon mode where the adapter is created before build.
	if updater, ok := o.config.NodeInitializer.(BinaryPathUpdater); ok {
		o.logger.Debug("updating node initializer with binary path", "binaryPath", binaryPath)
		updater.SetBinaryPath(binaryPath)
	}

	// Phase 2: Forking
	if err := ctx.Err(); err != nil {
		o.setError(fmt.Errorf("context cancelled: %w", err))
		return nil, o.lastErr
	}

	o.setPhase(PhaseForking, "Forking genesis from source")

	forkResult, err := o.executeForkPhase(ctx, opts, binaryPath)
	if err != nil {
		o.setError(fmt.Errorf("forking phase failed: %w", err))
		return nil, o.lastErr
	}

	// Phase 3: Initializing
	if err := ctx.Err(); err != nil {
		o.setError(fmt.Errorf("context cancelled: %w", err))
		return nil, o.lastErr
	}

	o.setPhase(PhaseInitializing, "Initializing node directories")

	nodes, err := o.executeInitPhase(ctx, opts, binaryPath, forkResult)
	if err != nil {
		o.setError(fmt.Errorf("initializing phase failed: %w", err))
		return nil, o.lastErr
	}

	// Phase 4: Starting (skip if SkipStart is true - daemon mode)
	if opts.SkipStart {
		o.logger.Info("skipping start phase (SkipStart=true, daemon mode)")
		o.setPhase(PhasePending, "Provisioning complete (nodes will be started by controller)")
	} else {
		if err := ctx.Err(); err != nil {
			o.setError(fmt.Errorf("context cancelled: %w", err))
			return nil, o.lastErr
		}

		o.setPhase(PhaseStarting, "Starting node processes")

		if err := o.executeStartPhase(ctx, nodes); err != nil {
			o.setError(fmt.Errorf("starting phase failed: %w", err))
			return nil, o.lastErr
		}

		// Phase 5: Health Checking
		if err := ctx.Err(); err != nil {
			o.setError(fmt.Errorf("context cancelled: %w", err))
			return nil, o.lastErr
		}

		// Skip health checking if timeout is negative (explicit opt-out)
		if opts.HealthCheckTimeout >= 0 {
			o.setPhase(PhaseHealthChecking, "Verifying node health")

			healthResult, err := o.executeHealthPhase(ctx, nodes, opts.HealthCheckTimeout)
			if err != nil {
				o.setError(fmt.Errorf("health checking phase failed: %w", err))
				return nil, o.lastErr
			}

			// Determine final phase based on health check result
			if healthResult.AllHealthy {
				o.setPhase(PhaseRunning, "Devnet is operational")
			} else {
				o.setPhase(PhaseDegraded, fmt.Sprintf("Devnet running but degraded (%d/%d nodes healthy)", healthResult.HealthyCount, healthResult.TotalCount))
			}
		} else {
			// Health checking skipped by explicit opt-out
			o.setPhase(PhaseRunning, "Devnet is operational (health check skipped)")
		}
	}

	// Build and return result
	result := &ports.ProvisionResult{
		DevnetName:     opts.DevnetName,
		ChainID:        opts.ChainID,
		BinaryPath:     binaryPath,
		GenesisPath:    filepath.Join(opts.DataDir, "genesis.json"),
		NodeCount:      opts.NumValidators + opts.NumFullNodes,
		ValidatorCount: opts.NumValidators,
		FullNodeCount:  opts.NumFullNodes,
		DataDir:        opts.DataDir,
	}

	o.logger.Info("orchestrator provisioning complete",
		"devnet", opts.DevnetName,
		"binaryPath", binaryPath,
		"nodeCount", result.NodeCount,
	)

	return result, nil
}

// executeBuildPhase handles the building phase
func (o *ProvisioningOrchestrator) executeBuildPhase(ctx context.Context, opts ports.ProvisionOptions) (*builder.BuildResult, error) {
	o.logger.Info("starting build phase",
		"version", opts.BinaryVersion,
		"network", opts.Network,
	)

	spec := builder.BuildSpec{
		GitRef:     opts.BinaryVersion,
		PluginName: opts.Network,
	}

	result, err := o.config.BinaryBuilder.Build(ctx, spec)
	if err != nil {
		return nil, fmt.Errorf("binary build failed: %w", err)
	}

	o.logger.Info("build phase completed",
		"binaryPath", result.BinaryPath,
	)

	return result, nil
}

// executeForkPhase handles the genesis forking phase
func (o *ProvisioningOrchestrator) executeForkPhase(ctx context.Context, opts ports.ProvisionOptions, binaryPath string) (*ports.ForkResult, error) {
	o.logger.Info("starting fork phase",
		"mode", opts.GenesisSource.Mode,
		"chainID", opts.ChainID,
	)

	forkOpts := ports.ForkOptions{
		Source:     opts.GenesisSource,
		BinaryPath: binaryPath,
		PatchOpts:  opts.GenesisPatchOpts,
	}

	// Ensure chain ID is set in patch options
	if forkOpts.PatchOpts.ChainID == "" {
		forkOpts.PatchOpts.ChainID = opts.ChainID
	}

	// Propagate binary version to patch options for genesis modification
	if forkOpts.PatchOpts.BinaryVersion == "" {
		forkOpts.PatchOpts.BinaryVersion = opts.BinaryVersion
	}

	result, err := o.config.GenesisForker.Fork(ctx, forkOpts)
	if err != nil {
		return nil, fmt.Errorf("genesis fork failed: %w", err)
	}

	// Save genesis to data directory
	genesisPath := filepath.Join(opts.DataDir, "genesis.json")
	if err := os.MkdirAll(opts.DataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}
	if err := os.WriteFile(genesisPath, result.Genesis, 0644); err != nil {
		return nil, fmt.Errorf("failed to write genesis file: %w", err)
	}

	o.logger.Info("fork phase completed",
		"sourceChainID", result.SourceChainID,
		"newChainID", result.NewChainID,
		"genesisPath", genesisPath,
	)

	return result, nil
}

// executeInitPhase handles the node initialization phase
func (o *ProvisioningOrchestrator) executeInitPhase(ctx context.Context, opts ports.ProvisionOptions, binaryPath string, forkResult *ports.ForkResult) ([]*types.Node, error) {
	o.logger.Info("starting init phase",
		"validators", opts.NumValidators,
		"fullNodes", opts.NumFullNodes,
	)

	totalNodes := opts.NumValidators + opts.NumFullNodes
	nodes := make([]*types.Node, 0, totalNodes)

	// Initialize validators
	for i := 0; i < opts.NumValidators; i++ {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("context cancelled during initialization: %w", err)
		}

		o.logger.Info("initializing validator",
			"index", i+1,
			"total", opts.NumValidators,
		)

		node, err := o.initializeNode(ctx, opts, binaryPath, i, "validator")
		if err != nil {
			return nil, fmt.Errorf("failed to initialize validator %d: %w", i, err)
		}
		nodes = append(nodes, node)
	}

	// Initialize full nodes
	for i := 0; i < opts.NumFullNodes; i++ {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("context cancelled during initialization: %w", err)
		}

		o.logger.Info("initializing fullnode",
			"index", i+1,
			"total", opts.NumFullNodes,
		)

		nodeIndex := opts.NumValidators + i
		node, err := o.initializeNode(ctx, opts, binaryPath, nodeIndex, "fullnode")
		if err != nil {
			return nil, fmt.Errorf("failed to initialize fullnode %d: %w", i, err)
		}
		nodes = append(nodes, node)
	}

	// Write forked genesis to all node config directories
	// This is critical: the chain init command creates a placeholder genesis,
	// but we need to overwrite it with the actual forked genesis from the fork phase.
	if forkResult != nil && len(forkResult.Genesis) > 0 {
		o.logger.Info("distributing forked genesis to nodes",
			"nodeCount", len(nodes),
			"genesisSize", len(forkResult.Genesis),
		)

		for _, node := range nodes {
			genesisPath := filepath.Join(node.Spec.HomeDir, "config", "genesis.json")
			if err := os.WriteFile(genesisPath, forkResult.Genesis, 0644); err != nil {
				return nil, fmt.Errorf("failed to write genesis to node %s: %w", node.Metadata.Name, err)
			}
			o.logger.Debug("genesis written to node",
				"node", node.Metadata.Name,
				"path", genesisPath,
			)
		}
	}

	o.logger.Info("init phase completed",
		"nodeCount", len(nodes),
	)

	return nodes, nil
}

// initializeNode initializes a single node
func (o *ProvisioningOrchestrator) initializeNode(ctx context.Context, opts ports.ProvisionOptions, binaryPath string, index int, role string) (*types.Node, error) {
	moniker := fmt.Sprintf("%s-%s-%d", opts.DevnetName, role, index)
	nodeDir := filepath.Join(opts.DataDir, "nodes", moniker)

	o.logger.Debug("initializing node",
		"moniker", moniker,
		"nodeDir", nodeDir,
		"role", role,
	)

	// Create node directory
	if err := os.MkdirAll(nodeDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create node directory: %w", err)
	}

	// Initialize node
	if err := o.config.NodeInitializer.Initialize(ctx, nodeDir, moniker, opts.ChainID); err != nil {
		return nil, fmt.Errorf("node initialization failed: %w", err)
	}

	// Create Node resource
	node := &types.Node{
		Metadata: types.ResourceMeta{
			Name: moniker,
		},
		Spec: types.NodeSpec{
			DevnetRef:  opts.DevnetName,
			Index:      index,
			Role:       role,
			BinaryPath: binaryPath,
			HomeDir:    nodeDir,
			Desired:    types.NodePhaseRunning,
		},
		Status: types.NodeStatus{
			Phase: types.NodePhasePending,
		},
	}

	return node, nil
}

// executeStartPhase handles the node starting phase
func (o *ProvisioningOrchestrator) executeStartPhase(ctx context.Context, nodes []*types.Node) error {
	o.logger.Info("starting nodes",
		"count", len(nodes),
	)

	for _, node := range nodes {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("context cancelled during startup: %w", err)
		}

		o.logger.Debug("starting node",
			"name", node.Metadata.Name,
			"role", node.Spec.Role,
		)

		startOpts := runtime.StartOptions{
			RestartPolicy: runtime.DefaultRestartPolicy(),
		}

		if err := o.config.NodeRuntime.StartNode(ctx, node, startOpts); err != nil {
			return fmt.Errorf("failed to start node %s: %w", node.Metadata.Name, err)
		}
	}

	o.logger.Info("all nodes started",
		"count", len(nodes),
	)

	return nil
}

// DefaultHealthCheckTimeout is the default duration to wait for nodes to become healthy.
const DefaultHealthCheckTimeout = 2 * time.Minute

// DefaultHealthCheckInterval is how often to poll node health during the health check phase.
const DefaultHealthCheckInterval = 5 * time.Second

// HealthPhaseResult indicates the outcome of the health checking phase.
type HealthPhaseResult struct {
	// AllHealthy is true if all nodes passed health checks.
	AllHealthy bool
	// HealthyCount is the number of healthy nodes.
	HealthyCount int
	// TotalCount is the total number of nodes checked.
	TotalCount int
	// TimedOut is true if the health check timed out.
	TimedOut bool
}

// executeHealthPhase handles the health checking phase after nodes are started.
// It polls nodes until all are healthy or the timeout is reached.
// Returns a HealthPhaseResult indicating the outcome.
func (o *ProvisioningOrchestrator) executeHealthPhase(ctx context.Context, nodes []*types.Node, timeout time.Duration) (*HealthPhaseResult, error) {
	// Skip if no health checker configured
	if o.config.HealthChecker == nil {
		o.logger.Info("skipping health check phase (no health checker configured)")
		return &HealthPhaseResult{
			AllHealthy:   true,
			HealthyCount: len(nodes),
			TotalCount:   len(nodes),
			TimedOut:     false,
		}, nil
	}

	// Use default timeout if not specified
	if timeout <= 0 {
		timeout = DefaultHealthCheckTimeout
	}

	o.logger.Info("starting health check phase",
		"nodeCount", len(nodes),
		"timeout", timeout,
	)

	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(DefaultHealthCheckInterval)
	defer ticker.Stop()

	for {
		// Check for context cancellation
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("context cancelled during health check: %w", err)
		}

		// Check all nodes
		healthyCount := 0
		for _, node := range nodes {
			result, err := o.config.HealthChecker.CheckHealth(ctx, node)
			if err != nil {
				o.logger.Debug("health check failed for node",
					"node", node.Metadata.Name,
					"error", err,
				)
				continue
			}

			if result.Healthy && !result.CatchingUp {
				healthyCount++
				o.logger.Debug("node is healthy",
					"node", node.Metadata.Name,
					"blockHeight", result.BlockHeight,
					"peerCount", result.PeerCount,
				)
			} else {
				o.logger.Debug("node not yet healthy",
					"node", node.Metadata.Name,
					"healthy", result.Healthy,
					"catchingUp", result.CatchingUp,
					"error", result.Error,
				)
			}
		}

		// Check if all nodes are healthy
		if healthyCount == len(nodes) {
			o.logger.Info("all nodes healthy",
				"healthyCount", healthyCount,
				"totalCount", len(nodes),
			)
			return &HealthPhaseResult{
				AllHealthy:   true,
				HealthyCount: healthyCount,
				TotalCount:   len(nodes),
				TimedOut:     false,
			}, nil
		}

		// Check for timeout
		if time.Now().After(deadline) {
			o.logger.Warn("health check timeout",
				"healthyCount", healthyCount,
				"totalCount", len(nodes),
				"timeout", timeout,
			)
			return &HealthPhaseResult{
				AllHealthy:   false,
				HealthyCount: healthyCount,
				TotalCount:   len(nodes),
				TimedOut:     true,
			}, nil
		}

		// Report progress
		o.setPhase(PhaseHealthChecking, fmt.Sprintf("Waiting for nodes to become healthy (%d/%d)", healthyCount, len(nodes)))

		// Wait for next poll
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("context cancelled during health check: %w", ctx.Err())
		case <-ticker.C:
			// Continue to next poll
		}
	}
}
