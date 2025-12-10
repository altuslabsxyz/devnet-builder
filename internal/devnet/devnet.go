package devnet

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"cosmossdk.io/log"

	"github.com/stablelabs/stable-devnet/internal/cache"
	"github.com/stablelabs/stable-devnet/internal/generator"
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
	HomeDir            string
	Network            string
	NumValidators      int
	NumAccounts        int
	Mode               ExecutionMode
	StableVersion      string
	NoCache            bool
	Logger             *output.Logger
	IsCustomRef        bool   // True if StableVersion is a custom branch/commit
	CustomBinaryPath   string // Path to custom-built binary (set after build)
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

	// Configure generator
	genConfig := generator.DefaultConfig()
	genConfig.NumValidators = opts.NumValidators
	genConfig.NumAccounts = opts.NumAccounts
	genConfig.OutputDir = devnetDir
	genConfig.ChainID = provisionResult.ChainID

	// Create generator with a proper logger
	genLogger := log.NewNopLogger() // Use NopLogger to avoid duplicate output
	gen := generator.NewDevnetGenerator(genConfig, genLogger)

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

	// Step 4: Get node IDs and create node objects
	progress.Stage("Initializing nodes")

	// Convert ExecutionMode to nodeconfig.ExecutionMode
	var initMode nodeconfig.ExecutionMode
	if opts.Mode == ModeDocker {
		initMode = nodeconfig.ModeDocker
	} else {
		initMode = nodeconfig.ModeLocal
	}
	initializer := nodeconfig.NewNodeInitializer(initMode, dockerImage, logger)
	nodeIDs := make([]string, opts.NumValidators)
	nodes := make([]*node.Node, opts.NumValidators)

	for i := 0; i < opts.NumValidators; i++ {
		nodeDir := filepath.Join(devnetDir, fmt.Sprintf("node%d", i))

		// Get node ID from the generated priv_validator_key.json
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

// startNode starts a single node.
func (d *Devnet) startNode(ctx context.Context, n *node.Node, genesisPath string) error {
	evmChainID := node.ExtractEVMChainID(d.Metadata.ChainID)

	switch d.Metadata.ExecutionMode {
	case ModeDocker:
		manager := node.NewDockerManagerWithEVMChainID(provision.GetDockerImage(d.Metadata.StableVersion), evmChainID, d.Logger)
		return manager.Start(ctx, n, genesisPath)
	case ModeLocal:
		// Determine binary path
		binary := d.resolveBinaryPath()
		manager := node.NewLocalManagerWithEVMChainID(binary, evmChainID, d.Logger)
		return manager.Start(ctx, n, genesisPath)
	default:
		return fmt.Errorf("unknown execution mode: %s", d.Metadata.ExecutionMode)
	}
}

// resolveBinaryPath determines the binary path to use for local mode.
// Priority: symlink path (if exists) > custom binary path > default "stabled"
func (d *Devnet) resolveBinaryPath() string {
	// Check if symlink exists
	symlinkMgr := cache.NewSymlinkManager(d.Metadata.HomeDir)
	if symlinkMgr.IsSymlink() {
		return symlinkMgr.SymlinkPath()
	}

	// Check if there's a custom binary path set
	if d.Metadata.CustomBinaryPath != "" {
		return d.Metadata.CustomBinaryPath
	}

	// Fall back to default
	return ""
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

// stopNode stops a single node.
func (d *Devnet) stopNode(ctx context.Context, n *node.Node, timeout time.Duration) error {
	switch d.Metadata.ExecutionMode {
	case ModeDocker:
		manager := node.NewDockerManager("", d.Logger)
		return manager.Stop(ctx, n, timeout)
	case ModeLocal:
		manager := node.NewLocalManager("", d.Logger)
		return manager.Stop(ctx, n, timeout)
	default:
		return fmt.Errorf("unknown execution mode: %s", d.Metadata.ExecutionMode)
	}
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
