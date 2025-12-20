package devnet

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"cosmossdk.io/log"

	"github.com/b-harvest/devnet-builder/internal/cache"
	"github.com/b-harvest/devnet-builder/internal/network"
	"github.com/b-harvest/devnet-builder/internal/node"
	"github.com/b-harvest/devnet-builder/internal/nodeconfig"
	"github.com/b-harvest/devnet-builder/internal/output"
	"github.com/b-harvest/devnet-builder/internal/prereq"
	"github.com/b-harvest/devnet-builder/internal/provision"
)

// ProvisionService handles devnet provisioning operations.
type ProvisionService struct {
	logger *output.Logger
}

// NewProvisionService creates a new ProvisionService.
func NewProvisionService(logger *output.Logger) *ProvisionService {
	if logger == nil {
		logger = output.DefaultLogger
	}
	return &ProvisionService{logger: logger}
}

// Provision creates devnet configuration and generates validators without starting nodes.
// This allows users to modify config files before running.
func (s *ProvisionService) Provision(ctx context.Context, opts ProvisionOptions) (*ProvisionResult, error) {
	logger := opts.Logger
	if logger == nil {
		logger = s.logger
	}

	progress := output.NewProgress(5)

	// Step 1: Check prerequisites
	progress.Stage("Checking prerequisites")
	if err := s.checkPrerequisites(opts.Mode); err != nil {
		return nil, err
	}

	// Create devnet directory
	devnetDir := filepath.Join(opts.HomeDir, "devnet")
	if err := os.MkdirAll(devnetDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create devnet directory: %w", err)
	}

	// Create initial metadata and mark provision as started
	metadata := s.createInitialMetadata(opts)
	metadata.SetProvisionStarted()

	if err := metadata.Save(); err != nil {
		return nil, fmt.Errorf("failed to save initial metadata: %w", err)
	}

	// Step 2: Provision (download snapshot and export genesis)
	progress.Stage("Provisioning chain state")
	provisionResult, dockerImage, err := s.provisionChainState(ctx, opts, metadata, logger)
	if err != nil {
		metadata.SetProvisionFailed(err)
		metadata.Save()
		return nil, err
	}

	logger.Debug("Provisioning complete. Genesis at: %s", provisionResult.GenesisPath)

	// Step 3: Generate validators and modify genesis
	progress.Stage("Generating validators")
	netModule, genConfig, err := s.generateValidators(ctx, opts, metadata, devnetDir, provisionResult.ChainID, logger)
	if err != nil {
		metadata.SetProvisionFailed(err)
		metadata.Save()
		return nil, err
	}

	// Update metadata with chain info
	metadata.ChainID = genConfig.ChainID
	metadata.GenesisPath = filepath.Join(devnetDir, "node0", "config", "genesis.json")

	// Step 4: Get node IDs and create node objects
	progress.Stage("Initializing nodes")
	nodes, nodeIDs, err := s.initializeNodes(ctx, opts, metadata, netModule, devnetDir, dockerImage, genConfig.ChainID, logger)
	if err != nil {
		metadata.SetProvisionFailed(err)
		metadata.Save()
		return nil, err
	}

	// Step 5: Configure nodes (ports, persistent peers, etc.)
	progress.Stage("Configuring nodes")
	if err := s.configureNodes(opts, metadata, devnetDir, nodes, nodeIDs, logger); err != nil {
		metadata.SetProvisionFailed(err)
		metadata.Save()
		return nil, err
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

// checkPrerequisites validates system requirements.
func (s *ProvisionService) checkPrerequisites(mode ExecutionMode) error {
	checker := prereq.NewChecker()
	if mode == ModeDocker {
		checker.RequireDocker()
	} else {
		checker.RequireLocal()
	}

	results, err := checker.Check()
	if err != nil {
		return fmt.Errorf("prerequisites not met: %w", err)
	}
	for _, r := range results {
		if !r.Found && r.Required {
			return fmt.Errorf("%s: %s\nSuggestion: %s", r.Name, r.Message, r.Suggestion)
		}
	}
	return nil
}

// createInitialMetadata creates metadata from provision options.
func (s *ProvisionService) createInitialMetadata(opts ProvisionOptions) *DevnetMetadata {
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

	return metadata
}

// provisionChainState downloads snapshot and exports genesis.
func (s *ProvisionService) provisionChainState(ctx context.Context, opts ProvisionOptions, metadata *DevnetMetadata, logger *output.Logger) (*provision.ProvisionResult, string, error) {
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
		Blockchain:  opts.BlockchainNetwork,
		HomeDir:     opts.HomeDir,
		DockerImage: dockerImage,
		Mode:        provisionMode,
		NoCache:     opts.NoCache,
		Logger:      logger,
	})

	provisionResult, err := provisioner.Provision(ctx)
	if err != nil {
		provisioner.Cleanup(ctx)
		return nil, "", fmt.Errorf("provisioning failed: %w", err)
	}

	// Cleanup provisioner after getting genesis
	provisioner.Cleanup(ctx)

	return provisionResult, dockerImage, nil
}

// generateValidators creates validators and modifies genesis.
func (s *ProvisionService) generateValidators(ctx context.Context, opts ProvisionOptions, metadata *DevnetMetadata, devnetDir, chainID string, logger *output.Logger) (network.NetworkModule, *network.GeneratorConfig, error) {
	// Get network module for generator
	netModule, err := network.Get(metadata.BlockchainNetwork)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get network module: %w", err)
	}

	// Configure generator using network module defaults
	genConfig := netModule.DefaultGeneratorConfig()
	genConfig.NumValidators = opts.NumValidators
	genConfig.NumAccounts = opts.NumAccounts
	genConfig.OutputDir = devnetDir
	genConfig.ChainID = chainID

	// Create generator with a proper logger
	genLogger := log.NewNopLogger() // Use NopLogger to avoid duplicate output
	gen, err := netModule.NewGenerator(genConfig, genLogger)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create generator: %w", err)
	}

	// Build devnet from exported genesis
	genesisPath := filepath.Join(devnetDir, "genesis.json")
	if err := gen.Build(genesisPath); err != nil {
		return nil, nil, fmt.Errorf("failed to generate validators: %w", err)
	}

	logger.Debug("Generator created %d validators", opts.NumValidators)

	return netModule, genConfig, nil
}

// initializeNodes creates and initializes node configurations.
func (s *ProvisionService) initializeNodes(ctx context.Context, opts ProvisionOptions, metadata *DevnetMetadata, netModule network.NetworkModule, devnetDir, dockerImage, chainID string, logger *output.Logger) ([]*node.Node, []string, error) {
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

		// Backup validator keys before init
		backups := s.backupValidatorKeys(nodeDir)

		// Run binary init to create config files
		if err := initializer.Initialize(ctx, nodeDir, moniker, chainID); err != nil {
			return nil, nil, fmt.Errorf("failed to initialize node%d: %w", i, err)
		}

		// Restore validator keys
		s.restoreValidatorKeys(nodeDir, backups)

		// Get node ID
		nodeID, err := initializer.GetNodeID(ctx, nodeDir)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get node ID for node%d: %w", i, err)
		}
		nodeIDs[i] = nodeID
		logger.Debug("Node %d ID: %s", i, nodeID)

		// Create node object
		n := node.NewNode(i, nodeDir)
		nodes[i] = n
	}

	return nodes, nodeIDs, nil
}

// validatorKeyBackups holds backed up validator key files.
type validatorKeyBackups struct {
	nodeKey      []byte
	privValKey   []byte
	privValState []byte
	genesis      []byte
}

// backupValidatorKeys reads and stores validator key files.
func (s *ProvisionService) backupValidatorKeys(nodeDir string) *validatorKeyBackups {
	backups := &validatorKeyBackups{}
	backups.nodeKey, _ = os.ReadFile(filepath.Join(nodeDir, "config", "node_key.json"))
	backups.privValKey, _ = os.ReadFile(filepath.Join(nodeDir, "config", "priv_validator_key.json"))
	backups.privValState, _ = os.ReadFile(filepath.Join(nodeDir, "data", "priv_validator_state.json"))
	backups.genesis, _ = os.ReadFile(filepath.Join(nodeDir, "config", "genesis.json"))
	return backups
}

// restoreValidatorKeys writes backed up validator key files.
func (s *ProvisionService) restoreValidatorKeys(nodeDir string, backups *validatorKeyBackups) {
	if len(backups.nodeKey) > 0 {
		_ = os.WriteFile(filepath.Join(nodeDir, "config", "node_key.json"), backups.nodeKey, 0600)
	}
	if len(backups.privValKey) > 0 {
		_ = os.WriteFile(filepath.Join(nodeDir, "config", "priv_validator_key.json"), backups.privValKey, 0600)
	}
	if len(backups.privValState) > 0 {
		_ = os.WriteFile(filepath.Join(nodeDir, "data", "priv_validator_state.json"), backups.privValState, 0644)
	}
	if len(backups.genesis) > 0 {
		_ = os.WriteFile(filepath.Join(nodeDir, "config", "genesis.json"), backups.genesis, 0644)
	}
}

// configureNodes sets up node ports and persistent peers.
func (s *ProvisionService) configureNodes(opts ProvisionOptions, metadata *DevnetMetadata, devnetDir string, nodes []*node.Node, nodeIDs []string, logger *output.Logger) error {
	// Build persistent peers string
	peers := nodeconfig.BuildPersistentPeers(nodeIDs, BaseP2PPort)
	logger.Debug("Persistent peers: %s", peers)

	for i := 0; i < opts.NumValidators; i++ {
		nodeDir := filepath.Join(devnetDir, fmt.Sprintf("node%d", i))

		// Build peers excluding this node
		nodePeers := nodeconfig.BuildPersistentPeersWithExclusion(nodeIDs, BaseP2PPort, i)

		// Configure node
		if err := nodeconfig.ConfigureNode(nodeDir, i, nodePeers, i == 0, logger); err != nil {
			return fmt.Errorf("failed to configure node%d: %w", i, err)
		}

		// Save node config
		if err := nodes[i].Save(); err != nil {
			return fmt.Errorf("failed to save node%d config: %w", i, err)
		}
	}

	return nil
}

// Provision is a package-level function for backward compatibility.
// It delegates to ProvisionService.
func Provision(ctx context.Context, opts ProvisionOptions) (*ProvisionResult, error) {
	svc := NewProvisionService(opts.Logger)
	return svc.Provision(ctx, opts)
}
