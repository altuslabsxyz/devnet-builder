package devnet

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"cosmossdk.io/log"

	"github.com/b-harvest/devnet-builder/internal/cache"
	"github.com/b-harvest/devnet-builder/internal/network"
	"github.com/b-harvest/devnet-builder/internal/node"
	"github.com/b-harvest/devnet-builder/internal/nodeconfig"
	"github.com/b-harvest/devnet-builder/internal/output"
	"github.com/b-harvest/devnet-builder/internal/provision"
)

// RunService handles running nodes from a provisioned devnet.
type RunService struct {
	logger *output.Logger
}

// NewRunService creates a new RunService.
func NewRunService(logger *output.Logger) *RunService {
	if logger == nil {
		logger = output.DefaultLogger
	}
	return &RunService{logger: logger}
}

// Run starts nodes from a provisioned devnet.
// Requires: ProvisionState == "provisioned"
func (s *RunService) Run(ctx context.Context, opts RunOptions) (*RunResult, error) {
	logger := opts.Logger
	if logger == nil {
		logger = s.logger
	}

	// Load existing metadata
	metadata, err := LoadDevnetMetadata(opts.HomeDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load devnet metadata: %w\nHint: Run 'devnet-builder provision' first", err)
	}

	// Validate provision state
	if err := s.validateProvisionState(metadata); err != nil {
		return nil, err
	}

	// Apply run options to metadata if provided (CLI --mode flag only)
	s.applyRunOptions(metadata, opts, logger)

	// Load nodes
	nodes := s.loadNodes(opts.HomeDir, metadata)

	devnet := NewDevnet(metadata, logger)
	devnet.Nodes = nodes

	progress := output.NewProgress(3)

	// For docker mode, validate image before starting nodes
	if err := s.validateDockerImage(ctx, metadata, progress, logger); err != nil {
		return nil, err
	}

	// Start nodes
	progress.Stage("Starting nodes")
	result := s.startAllNodes(ctx, devnet, nodes, metadata, logger)

	// Wait for nodes to become healthy
	healthTimeout := opts.HealthTimeout
	if healthTimeout == 0 {
		healthTimeout = HealthCheckTimeout
	}

	progress.Stage("Waiting for nodes to become healthy")
	logger.Debug("Waiting for nodes to become healthy (timeout: %v)...", healthTimeout)

	s.waitForHealthy(ctx, nodes, healthTimeout, result, logger)

	// Update metadata
	metadata.SetRunning()
	if err := metadata.Save(); err != nil {
		logger.Warn("Failed to update metadata: %v", err)
	}

	if result.AllHealthy {
		progress.Done("All nodes started successfully!")
	} else if len(result.SuccessfulNodes) > 0 {
		progress.Done(fmt.Sprintf("Started %d/%d nodes (some failures)", len(result.SuccessfulNodes), metadata.NumValidators))
	}

	return result, nil
}

// validateProvisionState checks if devnet can be run.
func (s *RunService) validateProvisionState(metadata *DevnetMetadata) error {
	if !metadata.CanRun() {
		switch metadata.ProvisionState {
		case ProvisionStateNone:
			return fmt.Errorf("devnet not provisioned\nRun 'devnet-builder provision' first")
		case ProvisionStateSyncing:
			return fmt.Errorf("provisioning in progress\nWait for provision to complete")
		case ProvisionStateFailed:
			return fmt.Errorf("provisioning failed: %s\nRun 'devnet-builder clean' then 'devnet-builder provision' to retry", metadata.ProvisionError)
		default:
			return fmt.Errorf("invalid provision state: %s", metadata.ProvisionState)
		}
	}
	return nil
}

// applyRunOptions applies CLI options to metadata.
func (s *RunService) applyRunOptions(metadata *DevnetMetadata, opts RunOptions, logger *output.Logger) {
	if opts.Mode != "" {
		logger.Debug("Overriding execution mode from CLI flag: %s -> %s", metadata.ExecutionMode, opts.Mode)
		metadata.ExecutionMode = opts.Mode
	}
	if opts.StableVersion != "" {
		metadata.StableVersion = opts.StableVersion
	}
	if opts.IsCustomRef {
		metadata.IsCustomRef = opts.IsCustomRef
	}
	if opts.CustomBinaryPath != "" {
		metadata.CustomBinaryPath = opts.CustomBinaryPath
	}
}

// loadNodes loads node configurations from disk.
func (s *RunService) loadNodes(homeDir string, metadata *DevnetMetadata) []*node.Node {
	devnetDir := filepath.Join(homeDir, "devnet")
	nodes := make([]*node.Node, metadata.NumValidators)
	for i := 0; i < metadata.NumValidators; i++ {
		nodeDir := filepath.Join(devnetDir, fmt.Sprintf("node%d", i))
		n, err := node.LoadNode(nodeDir)
		if err != nil {
			n = node.NewNode(i, nodeDir)
		}
		nodes[i] = n
	}
	return nodes
}

// validateDockerImage validates docker image if in docker mode.
func (s *RunService) validateDockerImage(ctx context.Context, metadata *DevnetMetadata, progress *output.Progress, logger *output.Logger) error {
	if metadata.ExecutionMode == ModeDocker && metadata.DockerImage != "" {
		progress.Stage("Validating docker image")
		dm := node.NewDockerManager(metadata.DockerImage, logger)
		if err := dm.ValidateImage(ctx); err != nil {
			return fmt.Errorf("docker image validation failed: %w", err)
		}
		logger.Debug("Docker image validated: %s", metadata.DockerImage)
	}
	return nil
}

// startAllNodes starts each node and tracks success/failure.
func (s *RunService) startAllNodes(ctx context.Context, devnet *Devnet, nodes []*node.Node, metadata *DevnetMetadata, logger *output.Logger) *RunResult {
	result := &RunResult{
		Devnet:          devnet,
		SuccessfulNodes: make([]int, 0),
		FailedNodes:     make([]FailedNode, 0),
		AllHealthy:      true,
	}

	for _, n := range nodes {
		if err := devnet.startNode(ctx, n, metadata.GenesisPath); err != nil {
			result.FailedNodes = append(result.FailedNodes, FailedNode{
				Index:   n.Index,
				Error:   err.Error(),
				LogPath: n.LogFilePath(),
			})
			result.AllHealthy = false
			logger.Warn("Failed to start node%d: %v", n.Index, err)
		} else {
			result.SuccessfulNodes = append(result.SuccessfulNodes, n.Index)
			logger.Debug("Started node%d", n.Index)
		}

		// Small delay between node starts
		time.Sleep(500 * time.Millisecond)
	}

	// If no nodes started, mark error
	if len(result.SuccessfulNodes) == 0 {
		metadata.SetError()
		metadata.Save()
	}

	return result
}

// waitForHealthy waits for nodes to become healthy and updates result.
func (s *RunService) waitForHealthy(ctx context.Context, nodes []*node.Node, timeout time.Duration, result *RunResult, logger *output.Logger) {
	if err := node.WaitForAllNodesHealthy(ctx, nodes, timeout); err != nil {
		logger.Warn("Not all nodes are healthy yet: %v", err)
		result.AllHealthy = false

		// Update failed nodes with log tail
		healthResults := node.CheckAllNodesHealth(ctx, nodes)
		for i, health := range healthResults {
			if health.Status != node.NodeStatusRunning && health.Status != node.NodeStatusSyncing {
				// Check if already in failed list
				found := false
				for j := range result.FailedNodes {
					if result.FailedNodes[j].Index == i {
						found = true
						break
					}
				}
				if !found {
					logTail, _ := output.ReadLastLines(nodes[i].LogFilePath(), output.DefaultLogLines)
					result.FailedNodes = append(result.FailedNodes, FailedNode{
						Index:   i,
						Error:   fmt.Sprintf("unhealthy: %s", health.Status),
						LogPath: nodes[i].LogFilePath(),
						LogTail: logTail,
					})
				}
			}
		}

		// Print failed node logs for diagnosis
		printFailedNodeLogs(ctx, nodes, logger)
	}
}

// Start provisions and starts a devnet in one operation.
// This is a convenience function that combines provision and run.
func (s *RunService) Start(ctx context.Context, opts StartOptions) (*Devnet, error) {
	logger := opts.Logger
	if logger == nil {
		logger = s.logger
	}

	progress := output.NewProgress(7)

	// Step 1: Check prerequisites
	progress.Stage("Checking prerequisites")
	provisionSvc := NewProvisionService(logger)
	if err := provisionSvc.checkPrerequisites(opts.Mode); err != nil {
		return nil, err
	}

	// Create devnet directory
	devnetDir := filepath.Join(opts.HomeDir, "devnet")
	if err := os.MkdirAll(devnetDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create devnet directory: %w", err)
	}

	// Step 2: Provision (sync to latest and export genesis)
	progress.Stage("Provisioning chain state")
	dockerImage := provision.GetDockerImage(opts.StableVersion)

	// Convert ExecutionMode to provision.ExecutionMode
	var provisionMode provision.ExecutionMode
	if opts.Mode == ModeDocker {
		provisionMode = provision.ModeDocker
	} else {
		provisionMode = provision.ModeLocal
	}

	provisioner := provision.NewProvisioner(&provision.ProvisionerOptions{
		Network:     opts.Network,
		Blockchain:  opts.Blockchain,
		HomeDir:     opts.HomeDir,
		DockerImage: dockerImage,
		Mode:        provisionMode,
		NoCache:     opts.NoCache,
		Logger:      logger,
	})

	provisionResult, err := provisioner.Provision(ctx)
	if err != nil {
		provisioner.Cleanup(ctx)
		return nil, fmt.Errorf("provisioning failed: %w", err)
	}

	provisioner.Cleanup(ctx)
	logger.Debug("Provisioning complete. Genesis at: %s", provisionResult.GenesisPath)

	// Step 3: Generate validators and modify genesis
	progress.Stage("Generating validators")

	netModule, err := network.Default()
	if err != nil {
		return nil, fmt.Errorf("failed to get network module: %w", err)
	}

	genConfig := netModule.DefaultGeneratorConfig()
	genConfig.NumValidators = opts.NumValidators
	genConfig.NumAccounts = opts.NumAccounts
	genConfig.OutputDir = devnetDir
	genConfig.ChainID = provisionResult.ChainID

	genLogger := log.NewNopLogger()
	gen, err := netModule.NewGenerator(genConfig, genLogger)
	if err != nil {
		return nil, fmt.Errorf("failed to create generator: %w", err)
	}

	if err := gen.Build(provisionResult.GenesisPath); err != nil {
		return nil, fmt.Errorf("failed to generate validators: %w", err)
	}

	logger.Debug("Generator created %d validators", opts.NumValidators)

	// Create devnet metadata
	chainID := genConfig.ChainID
	metadata := NewDevnetMetadata(opts.HomeDir)
	metadata.ChainID = chainID
	metadata.NetworkSource = opts.Network
	metadata.NumValidators = opts.NumValidators
	metadata.NumAccounts = opts.NumAccounts
	metadata.ExecutionMode = opts.Mode
	metadata.StableVersion = opts.StableVersion
	metadata.GenesisPath = filepath.Join(devnetDir, "node0", "config", "genesis.json")
	metadata.IsCustomRef = opts.IsCustomRef
	metadata.CustomBinaryPath = opts.CustomBinaryPath

	if err := metadata.SetInitialVersionFromGenesis(); err != nil {
		logger.Debug("Warning: Failed to read version from genesis: %v", err)
	}

	// Step 4: Initialize nodes and get node IDs
	progress.Stage("Initializing nodes")

	var initMode nodeconfig.ExecutionMode
	if opts.Mode == ModeDocker {
		initMode = nodeconfig.ModeDocker
	} else {
		initMode = nodeconfig.ModeLocal
	}

	var initializer *nodeconfig.NodeInitializer
	if opts.Mode == ModeLocal {
		binaryName := netModule.BinaryName()
		symlinkMgr := cache.NewSymlinkManager(opts.HomeDir, binaryName)
		localBinaryPath := symlinkMgr.SymlinkPath()
		initializer = nodeconfig.NewNodeInitializerWithBinary(initMode, dockerImage, localBinaryPath, logger)
	} else {
		initializer = nodeconfig.NewNodeInitializer(initMode, dockerImage, logger)
	}

	nodeIDs := make([]string, opts.NumValidators)
	nodes := make([]*node.Node, opts.NumValidators)

	for i := 0; i < opts.NumValidators; i++ {
		nodeDir := filepath.Join(devnetDir, fmt.Sprintf("node%d", i))
		moniker := fmt.Sprintf("node%d", i)

		// Backup and restore validator keys
		backups := provisionSvc.backupValidatorKeys(nodeDir)

		if err := initializer.Initialize(ctx, nodeDir, moniker, chainID); err != nil {
			return nil, fmt.Errorf("failed to initialize node%d: %w", i, err)
		}

		provisionSvc.restoreValidatorKeys(nodeDir, backups)

		nodeID, err := initializer.GetNodeID(ctx, nodeDir)
		if err != nil {
			return nil, fmt.Errorf("failed to get node ID for node%d: %w", i, err)
		}
		nodeIDs[i] = nodeID
		logger.Debug("Node %d ID: %s", i, nodeID)

		n := node.NewNode(i, nodeDir)
		nodes[i] = n
	}

	// Step 5: Configure nodes (ports, persistent peers, etc.)
	progress.Stage("Configuring nodes")

	peers := nodeconfig.BuildPersistentPeers(nodeIDs, BaseP2PPort)
	logger.Debug("Persistent peers: %s", peers)

	for i := 0; i < opts.NumValidators; i++ {
		nodeDir := filepath.Join(devnetDir, fmt.Sprintf("node%d", i))
		nodePeers := nodeconfig.BuildPersistentPeersWithExclusion(nodeIDs, BaseP2PPort, i)

		if err := nodeconfig.ConfigureNode(nodeDir, i, nodePeers, i == 0, logger); err != nil {
			return nil, fmt.Errorf("failed to configure node%d: %w", i, err)
		}

		if err := nodes[i].Save(); err != nil {
			return nil, fmt.Errorf("failed to save node%d config: %w", i, err)
		}
	}

	if err := metadata.Save(); err != nil {
		return nil, fmt.Errorf("failed to save metadata: %w", err)
	}

	devnet := NewDevnet(metadata, logger)
	devnet.Nodes = nodes

	// Step 6: Start nodes
	progress.Stage("Starting nodes")
	if err := devnet.StartNodes(ctx, provisionResult.GenesisPath); err != nil {
		return nil, fmt.Errorf("failed to start nodes: %w", err)
	}

	// Wait for nodes to become healthy
	logger.Debug("Waiting for nodes to become healthy...")
	if err := node.WaitForAllNodesHealthy(ctx, nodes, HealthCheckTimeout); err != nil {
		logger.Warn("Not all nodes are healthy yet: %v", err)
		printFailedNodeLogs(ctx, nodes, logger)
	}

	metadata.SetRunning()
	if err := metadata.Save(); err != nil {
		logger.Warn("Failed to update metadata: %v", err)
	}

	progress.Done("Devnet started successfully!")

	return devnet, nil
}

// Run is a package-level function for backward compatibility.
func Run(ctx context.Context, opts RunOptions) (*RunResult, error) {
	svc := NewRunService(opts.Logger)
	return svc.Run(ctx, opts)
}

// Start is a package-level function for backward compatibility.
func Start(ctx context.Context, opts StartOptions) (*Devnet, error) {
	svc := NewRunService(opts.Logger)
	return svc.Start(ctx, opts)
}
