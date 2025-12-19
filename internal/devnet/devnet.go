package devnet

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"cosmossdk.io/log"

	"github.com/stablelabs/stable-devnet/internal/cache"
	"github.com/stablelabs/stable-devnet/internal/helpers"
	"github.com/stablelabs/stable-devnet/internal/network"
	"github.com/stablelabs/stable-devnet/internal/node"
	"github.com/stablelabs/stable-devnet/internal/nodeconfig"
	"github.com/stablelabs/stable-devnet/internal/output"
	"github.com/stablelabs/stable-devnet/internal/prereq"
	"github.com/stablelabs/stable-devnet/internal/provision"
)

const (
	// NodeStartTimeout is the timeout for waiting for a node to start.
	NodeStartTimeout = 2 * time.Minute

	// HealthCheckTimeout is the timeout for health checks after starting.
	HealthCheckTimeout = 5 * time.Minute

	// BaseP2PPort is the base P2P port for node0.
	BaseP2PPort = 26656
)

// Devnet represents a running devnet instance.
type Devnet struct {
	Metadata *DevnetMetadata
	Nodes    []*node.Node
	Config   *Config
	Logger   *output.Logger
}

// StartOptions configures devnet startup.
type StartOptions struct {
	HomeDir          string
	Network          string
	NumValidators    int
	NumAccounts      int
	Mode             ExecutionMode
	StableVersion    string
	NoCache          bool
	Logger           *output.Logger
	IsCustomRef      bool   // True if StableVersion is a custom branch/commit
	CustomBinaryPath string // Path to custom-built binary (set after build)
}

// ProvisionOptions configures devnet provisioning (without starting nodes).
type ProvisionOptions struct {
	HomeDir           string
	Network           string // Snapshot source: "mainnet" or "testnet"
	BlockchainNetwork string // Network module: "stable", "ault", etc.
	NumValidators     int
	NumAccounts       int
	Mode              ExecutionMode
	StableVersion     string
	NetworkVersion    string // Version for the selected blockchain network
	DockerImage       string // Docker image to use (only for docker mode)
	NoCache           bool
	Logger            *output.Logger
}

// ProvisionResult contains the result of provisioning.
type ProvisionResult struct {
	Metadata    *DevnetMetadata
	GenesisPath string
	Nodes       []*node.Node
}

// RunOptions configures devnet run (starting nodes from provisioned state).
type RunOptions struct {
	HomeDir          string
	Mode             ExecutionMode
	StableVersion    string
	BinaryRef        string // Binary reference from cache
	HealthTimeout    time.Duration
	Logger           *output.Logger
	IsCustomRef      bool   // True if StableVersion is a custom branch/commit
	CustomBinaryPath string // Path to custom-built binary
}

// RunResult contains the result of running nodes.
type RunResult struct {
	Devnet          *Devnet
	SuccessfulNodes []int
	FailedNodes     []FailedNode
	AllHealthy      bool
}

// FailedNode contains information about a failed node.
type FailedNode struct {
	Index   int
	Error   string
	LogPath string
	LogTail []string
}

// NewDevnet creates a new Devnet instance.
func NewDevnet(metadata *DevnetMetadata, logger *output.Logger) *Devnet {
	if logger == nil {
		logger = output.DefaultLogger
	}
	return &Devnet{
		Metadata: metadata,
		Nodes:    make([]*node.Node, 0),
		Logger:   logger,
	}
}

// Provision creates devnet configuration and generates validators without starting nodes.
// This allows users to modify config files before running.
func Provision(ctx context.Context, opts ProvisionOptions) (*ProvisionResult, error) {
	logger := opts.Logger
	if logger == nil {
		logger = output.DefaultLogger
	}

	progress := output.NewProgress(5)

	// Step 1: Check prerequisites
	progress.Stage("Checking prerequisites")
	checker := prereq.NewChecker()
	if opts.Mode == ModeDocker {
		checker.RequireDocker()
	} else {
		checker.RequireLocal()
	}

	results, err := checker.Check()
	if err != nil {
		return nil, fmt.Errorf("prerequisites not met: %w", err)
	}
	for _, r := range results {
		if !r.Found && r.Required {
			return nil, fmt.Errorf("%s: %s\nSuggestion: %s", r.Name, r.Message, r.Suggestion)
		}
	}

	// Create devnet directory
	devnetDir := filepath.Join(opts.HomeDir, "devnet")
	if err := os.MkdirAll(devnetDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create devnet directory: %w", err)
	}

	// Create initial metadata and mark provision as started
	metadata := NewDevnetMetadata(opts.HomeDir)
	metadata.NetworkSource = opts.Network
	metadata.NumValidators = opts.NumValidators
	metadata.NumAccounts = opts.NumAccounts
	metadata.ExecutionMode = opts.Mode
	metadata.StableVersion = opts.StableVersion

	// Set blockchain network (network module) - default to "stable" for backward compatibility
	if opts.BlockchainNetwork != "" {
		metadata.BlockchainNetwork = opts.BlockchainNetwork
	} else {
		metadata.BlockchainNetwork = "stable"
	}
	if opts.NetworkVersion != "" {
		metadata.NetworkVersion = opts.NetworkVersion
	}

	metadata.SetProvisionStarted()

	if err := metadata.Save(); err != nil {
		return nil, fmt.Errorf("failed to save initial metadata: %w", err)
	}

	// Step 2: Provision (download snapshot and export genesis)
	progress.Stage("Provisioning chain state")

	// Determine docker image: use provided image or fall back to default based on version
	dockerImage := opts.DockerImage
	if dockerImage == "" {
		dockerImage = provision.GetDockerImage(opts.StableVersion)
	}

	// Store docker image in metadata for docker mode
	if opts.Mode == ModeDocker {
		metadata.DockerImage = dockerImage
	}

	// Convert ExecutionMode to provision.ExecutionMode
	var provisionMode provision.ExecutionMode
	if opts.Mode == ModeDocker {
		provisionMode = provision.ModeDocker
	} else {
		provisionMode = provision.ModeLocal
	}

	provisioner := provision.NewProvisioner(&provision.ProvisionerOptions{
		Network:     opts.Network,
		HomeDir:     opts.HomeDir,
		DockerImage: dockerImage,
		Mode:        provisionMode,
		NoCache:     opts.NoCache,
		Logger:      logger,
	})

	provisionResult, err := provisioner.Provision(ctx)
	if err != nil {
		provisioner.Cleanup(ctx)
		metadata.SetProvisionFailed(err)
		metadata.Save()
		return nil, fmt.Errorf("provisioning failed: %w", err)
	}

	// Cleanup provisioner after getting genesis
	provisioner.Cleanup(ctx)

	logger.Debug("Provisioning complete. Genesis at: %s", provisionResult.GenesisPath)

	// Step 3: Generate validators and modify genesis
	progress.Stage("Generating validators")

	// Get network module for generator
	netModule, err := network.Get(metadata.BlockchainNetwork)
	if err != nil {
		metadata.SetProvisionFailed(err)
		metadata.Save()
		return nil, fmt.Errorf("failed to get network module: %w", err)
	}

	// Configure generator using network module defaults
	genConfig := netModule.DefaultGeneratorConfig()
	genConfig.NumValidators = opts.NumValidators
	genConfig.NumAccounts = opts.NumAccounts
	genConfig.OutputDir = devnetDir
	genConfig.ChainID = provisionResult.ChainID

	// Create generator with a proper logger
	genLogger := log.NewNopLogger() // Use NopLogger to avoid duplicate output
	gen, err := netModule.NewGenerator(genConfig, genLogger)
	if err != nil {
		metadata.SetProvisionFailed(err)
		metadata.Save()
		return nil, fmt.Errorf("failed to create generator: %w", err)
	}

	// Build devnet from exported genesis - this creates validators, modifies genesis, and saves to node dirs
	if err := gen.Build(provisionResult.GenesisPath); err != nil {
		metadata.SetProvisionFailed(err)
		metadata.Save()
		return nil, fmt.Errorf("failed to generate validators: %w", err)
	}

	logger.Debug("Generator created %d validators", opts.NumValidators)

	// Update metadata with chain info
	metadata.ChainID = genConfig.ChainID
	metadata.GenesisPath = filepath.Join(devnetDir, "node0", "config", "genesis.json")

	// Step 4: Get node IDs and create node objects
	progress.Stage("Initializing nodes")

	// Convert ExecutionMode to nodeconfig.ExecutionMode
	var initMode nodeconfig.ExecutionMode
	if opts.Mode == ModeDocker {
		initMode = nodeconfig.ModeDocker
	} else {
		initMode = nodeconfig.ModeLocal
	}

	// Create initializer - for local mode, use managed binary path
	var initializer *nodeconfig.NodeInitializer
	if opts.Mode == ModeLocal {
		// For local mode, always use the managed binary at ~/.stable-devnet/bin/stabled
		symlinkMgr := cache.NewSymlinkManager(opts.HomeDir)
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

		// First, backup the validator keys generated by our generator
		// (these will be overwritten by stabled init)
		nodeKeyBackup, _ := os.ReadFile(filepath.Join(nodeDir, "config", "node_key.json"))
		privValKeyBackup, _ := os.ReadFile(filepath.Join(nodeDir, "config", "priv_validator_key.json"))
		privValStateBackup, _ := os.ReadFile(filepath.Join(nodeDir, "data", "priv_validator_state.json"))
		genesisBackup, _ := os.ReadFile(filepath.Join(nodeDir, "config", "genesis.json"))

		// Run stabled init to create config.toml, app.toml, client.toml, etc.
		if err := initializer.Initialize(ctx, nodeDir, moniker, genConfig.ChainID); err != nil {
			metadata.SetProvisionFailed(err)
			metadata.Save()
			return nil, fmt.Errorf("failed to initialize node%d: %w", i, err)
		}

		// Restore our validator keys (overwrite the ones created by stabled init)
		if len(nodeKeyBackup) > 0 {
			_ = os.WriteFile(filepath.Join(nodeDir, "config", "node_key.json"), nodeKeyBackup, 0600)
		}
		if len(privValKeyBackup) > 0 {
			_ = os.WriteFile(filepath.Join(nodeDir, "config", "priv_validator_key.json"), privValKeyBackup, 0600)
		}
		if len(privValStateBackup) > 0 {
			_ = os.WriteFile(filepath.Join(nodeDir, "data", "priv_validator_state.json"), privValStateBackup, 0644)
		}
		if len(genesisBackup) > 0 {
			_ = os.WriteFile(filepath.Join(nodeDir, "config", "genesis.json"), genesisBackup, 0644)
		}

		// Get node ID from the generated priv_validator_key.json
		nodeID, err := initializer.GetNodeID(ctx, nodeDir)
		if err != nil {
			metadata.SetProvisionFailed(err)
			metadata.Save()
			return nil, fmt.Errorf("failed to get node ID for node%d: %w", i, err)
		}
		nodeIDs[i] = nodeID
		logger.Debug("Node %d ID: %s", i, nodeID)

		// Create node object
		n := node.NewNode(i, nodeDir)
		nodes[i] = n
	}

	// Step 5: Configure nodes (ports, persistent peers, etc.)
	progress.Stage("Configuring nodes")

	// Build persistent peers string
	peers := nodeconfig.BuildPersistentPeers(nodeIDs, BaseP2PPort)
	logger.Debug("Persistent peers: %s", peers)

	for i := 0; i < opts.NumValidators; i++ {
		nodeDir := filepath.Join(devnetDir, fmt.Sprintf("node%d", i))

		// Build peers excluding this node
		nodePeers := nodeconfig.BuildPersistentPeersWithExclusion(nodeIDs, BaseP2PPort, i)

		// Configure node
		if err := nodeconfig.ConfigureNode(nodeDir, i, nodePeers, i == 0, logger); err != nil {
			metadata.SetProvisionFailed(err)
			metadata.Save()
			return nil, fmt.Errorf("failed to configure node%d: %w", i, err)
		}

		// Save node config
		if err := nodes[i].Save(); err != nil {
			metadata.SetProvisionFailed(err)
			metadata.Save()
			return nil, fmt.Errorf("failed to save node%d config: %w", i, err)
		}
	}

	// Mark provision as complete
	metadata.SetProvisionComplete()
	if err := metadata.Save(); err != nil {
		return nil, fmt.Errorf("failed to save metadata: %w", err)
	}

	progress.Done("Provision complete!")

	return &ProvisionResult{
		Metadata:    metadata,
		GenesisPath: metadata.GenesisPath,
		Nodes:       nodes,
	}, nil
}

// Run starts nodes from a provisioned devnet.
// Requires: ProvisionState == "provisioned"
func Run(ctx context.Context, opts RunOptions) (*RunResult, error) {
	logger := opts.Logger
	if logger == nil {
		logger = output.DefaultLogger
	}

	// Load existing metadata
	metadata, err := LoadDevnetMetadata(opts.HomeDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load devnet metadata: %w\nHint: Run 'devnet-builder provision' first", err)
	}

	// Validate provision state
	if !metadata.CanRun() {
		switch metadata.ProvisionState {
		case ProvisionStateNone:
			return nil, fmt.Errorf("devnet not provisioned\nRun 'devnet-builder provision' first")
		case ProvisionStateSyncing:
			return nil, fmt.Errorf("provisioning in progress\nWait for provision to complete")
		case ProvisionStateFailed:
			return nil, fmt.Errorf("provisioning failed: %s\nRun 'devnet-builder clean' then 'devnet-builder provision' to retry", metadata.ProvisionError)
		default:
			return nil, fmt.Errorf("invalid provision state: %s", metadata.ProvisionState)
		}
	}

	// Apply run options to metadata if provided (CLI --mode flag only)
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

	// Load nodes
	devnetDir := filepath.Join(opts.HomeDir, "devnet")
	nodes := make([]*node.Node, metadata.NumValidators)
	for i := 0; i < metadata.NumValidators; i++ {
		nodeDir := filepath.Join(devnetDir, fmt.Sprintf("node%d", i))
		n, err := node.LoadNode(nodeDir)
		if err != nil {
			n = node.NewNode(i, nodeDir)
		}
		nodes[i] = n
	}

	devnet := NewDevnet(metadata, logger)
	devnet.Nodes = nodes

	progress := output.NewProgress(3)

	// For docker mode, validate image before starting nodes
	if metadata.ExecutionMode == ModeDocker && metadata.DockerImage != "" {
		progress.Stage("Validating docker image")
		dm := node.NewDockerManager(metadata.DockerImage, logger)
		if err := dm.ValidateImage(ctx); err != nil {
			return nil, fmt.Errorf("docker image validation failed: %w", err)
		}
		logger.Debug("Docker image validated: %s", metadata.DockerImage)
	}

	// Start nodes
	progress.Stage("Starting nodes")

	result := &RunResult{
		Devnet:          devnet,
		SuccessfulNodes: make([]int, 0),
		FailedNodes:     make([]FailedNode, 0),
		AllHealthy:      true,
	}

	// Start each node, tracking success/failure
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

	// If no nodes started, return error
	if len(result.SuccessfulNodes) == 0 {
		metadata.SetError()
		metadata.Save()
		return result, fmt.Errorf("failed to start any nodes")
	}

	// Wait for nodes to become healthy
	healthTimeout := opts.HealthTimeout
	if healthTimeout == 0 {
		healthTimeout = HealthCheckTimeout
	}

	progress.Stage("Waiting for nodes to become healthy")
	logger.Debug("Waiting for nodes to become healthy (timeout: %v)...", healthTimeout)

	if err := node.WaitForAllNodesHealthy(ctx, nodes, healthTimeout); err != nil {
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

// Start provisions and starts a devnet.
// This follows the workflow from deploy-devnet-upgrade.yml:
// 1. Check prerequisites
// 2. Initialize provisioner node
// 3. Download genesis from RPC
// 4. Download and extract snapshot
// 5. Sync to latest block (or skip)
// 6. Export genesis after sync
// 7. Run devnet-builder build (create validators)
// 8. Initialize each node with stabled init
// 9. Copy validator keys from build
// 10. Configure config.toml/app.toml
// 11. Start nodes
func Start(ctx context.Context, opts StartOptions) (*Devnet, error) {
	logger := opts.Logger
	if logger == nil {
		logger = output.DefaultLogger
	}

	progress := output.NewProgress(7)

	// Step 1: Check prerequisites
	progress.Stage("Checking prerequisites")
	checker := prereq.NewChecker()
	if opts.Mode == ModeDocker {
		checker.RequireDocker()
	} else {
		checker.RequireLocal()
	}

	results, err := checker.Check()
	if err != nil {
		return nil, fmt.Errorf("prerequisites not met: %w", err)
	}
	for _, r := range results {
		if !r.Found && r.Required {
			return nil, fmt.Errorf("%s: %s\nSuggestion: %s", r.Name, r.Message, r.Suggestion)
		}
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

	// Cleanup provisioner after getting genesis
	provisioner.Cleanup(ctx)

	logger.Debug("Provisioning complete. Genesis at: %s", provisionResult.GenesisPath)

	// Step 3: Generate validators and modify genesis
	progress.Stage("Generating validators")

	// Get network module for generator (use default for backward compatibility)
	netModule, err := network.Default()
	if err != nil {
		return nil, fmt.Errorf("failed to get network module: %w", err)
	}

	// Configure generator using network module defaults
	genConfig := netModule.DefaultGeneratorConfig()
	genConfig.NumValidators = opts.NumValidators
	genConfig.NumAccounts = opts.NumAccounts
	genConfig.OutputDir = devnetDir
	genConfig.ChainID = provisionResult.ChainID

	// Create generator with a proper logger
	genLogger := log.NewNopLogger() // Use NopLogger to avoid duplicate output
	gen, err := netModule.NewGenerator(genConfig, genLogger)
	if err != nil {
		return nil, fmt.Errorf("failed to create generator: %w", err)
	}

	// Build devnet from exported genesis - this creates validators, modifies genesis, and saves to node dirs
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

	// Read initial version from genesis
	if err := metadata.SetInitialVersionFromGenesis(); err != nil {
		logger.Debug("Warning: Failed to read version from genesis: %v", err)
	}

	// Step 4: Initialize nodes and get node IDs
	progress.Stage("Initializing nodes")

	// Convert ExecutionMode to nodeconfig.ExecutionMode
	var initMode nodeconfig.ExecutionMode
	if opts.Mode == ModeDocker {
		initMode = nodeconfig.ModeDocker
	} else {
		initMode = nodeconfig.ModeLocal
	}

	// Create initializer - for local mode, use managed binary path
	var initializer *nodeconfig.NodeInitializer
	if opts.Mode == ModeLocal {
		// For local mode, always use the managed binary at ~/.stable-devnet/bin/stabled
		symlinkMgr := cache.NewSymlinkManager(opts.HomeDir)
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

		// First, backup the validator keys generated by our generator
		// (these will be overwritten by stabled init)
		nodeKeyBackup, _ := os.ReadFile(filepath.Join(nodeDir, "config", "node_key.json"))
		privValKeyBackup, _ := os.ReadFile(filepath.Join(nodeDir, "config", "priv_validator_key.json"))
		privValStateBackup, _ := os.ReadFile(filepath.Join(nodeDir, "data", "priv_validator_state.json"))
		genesisBackup, _ := os.ReadFile(filepath.Join(nodeDir, "config", "genesis.json"))

		// Run stabled init to create config.toml, app.toml, client.toml, etc.
		if err := initializer.Initialize(ctx, nodeDir, moniker, chainID); err != nil {
			return nil, fmt.Errorf("failed to initialize node%d: %w", i, err)
		}

		// Restore our validator keys (overwrite the ones created by stabled init)
		if len(nodeKeyBackup) > 0 {
			_ = os.WriteFile(filepath.Join(nodeDir, "config", "node_key.json"), nodeKeyBackup, 0600)
		}
		if len(privValKeyBackup) > 0 {
			_ = os.WriteFile(filepath.Join(nodeDir, "config", "priv_validator_key.json"), privValKeyBackup, 0600)
		}
		if len(privValStateBackup) > 0 {
			_ = os.WriteFile(filepath.Join(nodeDir, "data", "priv_validator_state.json"), privValStateBackup, 0644)
		}
		if len(genesisBackup) > 0 {
			_ = os.WriteFile(filepath.Join(nodeDir, "config", "genesis.json"), genesisBackup, 0644)
		}

		// Get node ID from the node_key.json
		nodeID, err := initializer.GetNodeID(ctx, nodeDir)
		if err != nil {
			return nil, fmt.Errorf("failed to get node ID for node%d: %w", i, err)
		}
		nodeIDs[i] = nodeID
		logger.Debug("Node %d ID: %s", i, nodeID)

		// Create node object
		n := node.NewNode(i, nodeDir)
		nodes[i] = n
	}

	// Step 5: Configure nodes (ports, persistent peers, etc.)
	progress.Stage("Configuring nodes")

	// Build persistent peers string
	peers := nodeconfig.BuildPersistentPeers(nodeIDs, BaseP2PPort)
	logger.Debug("Persistent peers: %s", peers)

	for i := 0; i < opts.NumValidators; i++ {
		nodeDir := filepath.Join(devnetDir, fmt.Sprintf("node%d", i))

		// Build peers excluding this node
		nodePeers := nodeconfig.BuildPersistentPeersWithExclusion(nodeIDs, BaseP2PPort, i)

		// Configure node
		if err := nodeconfig.ConfigureNode(nodeDir, i, nodePeers, i == 0, logger); err != nil {
			return nil, fmt.Errorf("failed to configure node%d: %w", i, err)
		}

		// Save node config
		if err := nodes[i].Save(); err != nil {
			return nil, fmt.Errorf("failed to save node%d config: %w", i, err)
		}
	}

	// Save metadata before starting nodes
	if err := metadata.Save(); err != nil {
		return nil, fmt.Errorf("failed to save metadata: %w", err)
	}

	devnet := NewDevnet(metadata, logger)
	devnet.Nodes = nodes

	// Step 7: Start nodes
	progress.Stage("Starting nodes")
	if err := devnet.StartNodes(ctx, provisionResult.GenesisPath); err != nil {
		return nil, fmt.Errorf("failed to start nodes: %w", err)
	}

	// Wait for nodes to become healthy
	logger.Debug("Waiting for nodes to become healthy...")
	if err := node.WaitForAllNodesHealthy(ctx, nodes, HealthCheckTimeout); err != nil {
		logger.Warn("Not all nodes are healthy yet: %v", err)
		// Print failed node logs for diagnosis
		printFailedNodeLogs(ctx, nodes, logger)
	}

	// Update metadata
	metadata.SetRunning()
	if err := metadata.Save(); err != nil {
		logger.Warn("Failed to update metadata: %v", err)
	}

	progress.Done("Devnet started successfully!")

	return devnet, nil
}

// StartNodes starts all nodes in the devnet.
func (d *Devnet) StartNodes(ctx context.Context, genesisPath string) error {
	for _, n := range d.Nodes {
		if err := d.startNode(ctx, n, genesisPath); err != nil {
			return fmt.Errorf("failed to start %s: %w", n.Name, err)
		}
		d.Logger.Debug("Started %s", n.Name)

		// Small delay between node starts
		time.Sleep(500 * time.Millisecond)
	}
	return nil
}

// startNode starts a single node using the factory.
func (d *Devnet) startNode(ctx context.Context, n *node.Node, genesisPath string) error {
	factory := d.createNodeManagerFactory()
	manager, err := factory.Create()
	if err != nil {
		return fmt.Errorf("failed to create node manager: %w", err)
	}
	return manager.Start(ctx, n, genesisPath)
}

// resolveBinaryPath returns the binary path for local mode.
// Uses CustomBinaryPath if set, otherwise defaults to ~/.stable-devnet/bin/stabled.
func (d *Devnet) resolveBinaryPath() string {
	binaryPath := helpers.ResolveBinaryPath(d.Metadata.CustomBinaryPath, d.Metadata.HomeDir)
	d.Logger.Debug("Using binary: %s", binaryPath)
	return binaryPath
}

// createNodeManagerFactory creates a NodeManagerFactory from devnet metadata.
// This is the single point of truth for creating node managers in devnet package.
func (d *Devnet) createNodeManagerFactory() *node.NodeManagerFactory {
	// Convert devnet.ExecutionMode to node.ExecutionMode
	var mode node.ExecutionMode
	switch d.Metadata.ExecutionMode {
	case ModeDocker:
		mode = node.ModeDocker
	case ModeLocal:
		mode = node.ModeLocal
	}

	// Get docker image - use metadata value, fallback to NetworkModule, then StableVersion
	dockerImage := d.Metadata.DockerImage
	if dockerImage == "" {
		// Try to get from network module
		if mod, err := d.Metadata.GetNetworkModule(); err == nil && mod != nil {
			dockerImage = mod.DockerImage()
			if d.Metadata.StableVersion != "" && d.Metadata.StableVersion != "latest" {
				dockerImage = mod.DockerImage() + ":" + mod.DockerImageTag(d.Metadata.StableVersion)
			}
		} else {
			// Fallback for backward compatibility
			dockerImage = provision.GetDockerImage(d.Metadata.StableVersion)
		}
	}

	config := node.FactoryConfig{
		Mode:        mode,
		BinaryPath:  d.resolveBinaryPath(),
		DockerImage: dockerImage,
		EVMChainID:  node.ExtractEVMChainID(d.Metadata.ChainID),
		Logger:      d.Logger,
	}

	return node.NewNodeManagerFactory(config)
}

// Stop stops all nodes in the devnet.
func (d *Devnet) Stop(ctx context.Context, timeout time.Duration) error {
	for _, n := range d.Nodes {
		if err := d.stopNode(ctx, n, timeout); err != nil {
			d.Logger.Warn("Failed to stop %s: %v", n.Name, err)
		} else {
			d.Logger.Info("  %s: stopped", n.Name)
		}
	}

	d.Metadata.SetStopped()
	if err := d.Metadata.Save(); err != nil {
		d.Logger.Warn("Failed to update metadata: %v", err)
	}

	return nil
}

// stopNode stops a single node using the factory.
func (d *Devnet) stopNode(ctx context.Context, n *node.Node, timeout time.Duration) error {
	factory := d.createNodeManagerFactory()
	manager, err := factory.Create()
	if err != nil {
		return fmt.Errorf("failed to create node manager: %w", err)
	}
	return manager.Stop(ctx, n, timeout)
}

// copyFile copies a file from src to dst.
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

// LoadDevnetWithNodes loads a devnet and its nodes from disk.
func LoadDevnetWithNodes(homeDir string, logger *output.Logger) (*Devnet, error) {
	metadata, err := LoadDevnetMetadata(homeDir)
	if err != nil {
		return nil, err
	}

	devnet := NewDevnet(metadata, logger)

	// Load nodes
	for i := 0; i < metadata.NumValidators; i++ {
		nodeDir := filepath.Join(homeDir, "devnet", fmt.Sprintf("node%d", i))
		n, err := node.LoadNode(nodeDir)
		if err != nil {
			// Create node if config doesn't exist
			n = node.NewNode(i, nodeDir)
		}
		devnet.Nodes = append(devnet.Nodes, n)
	}

	return devnet, nil
}

// GetHealth returns the health status of all nodes.
func (d *Devnet) GetHealth(ctx context.Context) []*node.NodeHealth {
	return node.CheckAllNodesHealth(ctx, d.Nodes)
}

// SoftReset clears chain data but preserves genesis and configuration.
func (d *Devnet) SoftReset(ctx context.Context) error {
	// Stop nodes first if running
	if d.Metadata.IsRunning() {
		if err := d.Stop(ctx, 30*time.Second); err != nil {
			return fmt.Errorf("failed to stop nodes: %w", err)
		}
	}

	// Clear data directories for each node
	for _, n := range d.Nodes {
		dataDir := n.DataPath()
		if err := os.RemoveAll(dataDir); err != nil {
			return fmt.Errorf("failed to clear data for %s: %w", n.Name, err)
		}
		if err := os.MkdirAll(dataDir, 0755); err != nil {
			return fmt.Errorf("failed to recreate data dir for %s: %w", n.Name, err)
		}

		// Recreate priv_validator_state.json with initial state
		// This file is required by CometBFT to track validator signing state
		privValStatePath := filepath.Join(dataDir, "priv_validator_state.json")
		initialState := `{
  "height": "0",
  "round": 0,
  "step": 0
}`
		if err := os.WriteFile(privValStatePath, []byte(initialState), 0644); err != nil {
			return fmt.Errorf("failed to create priv_validator_state.json for %s: %w", n.Name, err)
		}
	}

	d.Metadata.Status = StatusCreated
	d.Metadata.StartedAt = nil
	d.Metadata.StoppedAt = nil
	return d.Metadata.Save()
}

// HardReset clears all data including genesis (requires re-provisioning).
func (d *Devnet) HardReset(ctx context.Context) error {
	// Stop nodes first if running
	if d.Metadata.IsRunning() {
		if err := d.Stop(ctx, 30*time.Second); err != nil {
			return fmt.Errorf("failed to stop nodes: %w", err)
		}
	}

	// Remove entire devnet directory
	devnetDir := filepath.Join(d.Metadata.HomeDir, "devnet")
	if err := os.RemoveAll(devnetDir); err != nil {
		return fmt.Errorf("failed to remove devnet directory: %w", err)
	}

	return nil
}

// printFailedNodeLogs checks all nodes and prints log files for any that failed health checks.
func printFailedNodeLogs(ctx context.Context, nodes []*node.Node, logger *output.Logger) {
	healthResults := node.CheckAllNodesHealth(ctx, nodes)

	for i, health := range healthResults {
		// Only print logs for unhealthy nodes
		if health.Status == node.NodeStatusRunning || health.Status == node.NodeStatusSyncing {
			continue
		}

		n := nodes[i]
		logPath := n.LogFilePath()

		// Read last N lines from log file
		logLines, err := output.ReadLastLines(logPath, output.DefaultLogLines)

		errorInfo := &output.NodeErrorInfo{
			NodeName: n.Name,
			NodeDir:  n.HomeDir,
			LogPath:  logPath,
			LogLines: logLines,
		}

		// Add PID if available (for verbose mode)
		if n.PID != nil {
			errorInfo.PID = *n.PID
		}

		// Handle read errors gracefully - still show what we can
		if err != nil {
			switch err.(type) {
			case *output.FileNotFoundError:
				errorInfo.LogLines = []string{"(Log file not found: " + logPath + ")"}
			case *output.EmptyFileError:
				errorInfo.LogLines = []string{"(Log file is empty)"}
			case *output.PermissionDeniedError:
				errorInfo.LogLines = []string{"(Cannot read log file: permission denied)"}
			default:
				errorInfo.LogLines = []string{"(Error reading log file: " + err.Error() + ")"}
			}
		}

		logger.PrintNodeError(errorInfo)
	}
}
