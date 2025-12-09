package devnet

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/stablelabs/stable-devnet/internal/node"
	"github.com/stablelabs/stable-devnet/internal/output"
	"github.com/stablelabs/stable-devnet/internal/prereq"
	"github.com/stablelabs/stable-devnet/internal/snapshot"
)

const (
	// NodeStartTimeout is the timeout for waiting for a node to start.
	NodeStartTimeout = 2 * time.Minute

	// HealthCheckTimeout is the timeout for health checks after starting.
	HealthCheckTimeout = 5 * time.Minute
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
	HomeDir       string
	Network       string
	NumValidators int
	NumAccounts   int
	Mode          ExecutionMode
	StableVersion string
	NoCache       bool
	Logger        *output.Logger
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
func Start(ctx context.Context, opts StartOptions) (*Devnet, error) {
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

	// Step 2: Download snapshot (or use cache)
	progress.Stage(fmt.Sprintf("Downloading %s snapshot", opts.Network))
	snapshotURL := GetSnapshotURL(opts.Network)
	if snapshotURL == "" {
		return nil, fmt.Errorf("unknown network: %s", opts.Network)
	}

	cache, err := snapshot.Download(ctx, snapshot.DownloadOptions{
		URL:     snapshotURL,
		Network: opts.Network,
		HomeDir: opts.HomeDir,
		NoCache: opts.NoCache,
		Logger:  logger,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to download snapshot: %w", err)
	}

	// Step 3: Export genesis from snapshot or fetch from RPC
	progress.Stage("Exporting genesis state")
	genesisPath := filepath.Join(opts.HomeDir, "devnet", "genesis.json")

	// Try to fetch from RPC first (faster)
	rpcEndpoint := GetRPCEndpoint(opts.Network)
	genesisMeta, err := snapshot.FetchGenesisFromRPC(ctx, rpcEndpoint, genesisPath, logger)
	if err != nil {
		logger.Debug("Failed to fetch genesis from RPC, trying snapshot export: %v", err)
		// Fall back to snapshot export
		genesisMeta, err = snapshot.ExportGenesisFromSnapshot(ctx, snapshot.ExportOptions{
			SnapshotPath: cache.FilePath,
			DestPath:     genesisPath,
			Network:      opts.Network,
			Decompressor: cache.Decompressor,
			Logger:       logger,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to export genesis: %w", err)
		}
	}

	// Modify genesis for devnet
	chainID := "stable-devnet-1"
	if err := snapshot.ModifyGenesis(genesisPath, chainID, opts.NumValidators); err != nil {
		return nil, fmt.Errorf("failed to modify genesis: %w", err)
	}

	// Save genesis metadata
	genesisMeta.NewChainID = chainID
	genesisMeta.Network = opts.Network
	if err := genesisMeta.Save(opts.HomeDir); err != nil {
		logger.Warn("Failed to save genesis metadata: %v", err)
	}

	// Step 4: Generate devnet configuration
	progress.Stage("Generating devnet configuration")

	// Create devnet metadata
	metadata := NewDevnetMetadata(opts.HomeDir)
	metadata.ChainID = chainID
	metadata.NetworkSource = opts.Network
	metadata.NumValidators = opts.NumValidators
	metadata.NumAccounts = opts.NumAccounts
	metadata.ExecutionMode = opts.Mode
	metadata.StableVersion = opts.StableVersion
	metadata.GenesisPath = genesisPath

	// Create nodes
	nodes := make([]*node.Node, opts.NumValidators)
	for i := 0; i < opts.NumValidators; i++ {
		nodeDir := filepath.Join(opts.HomeDir, "devnet", fmt.Sprintf("node%d", i))
		n := node.NewNode(i, nodeDir)

		// Initialize node directory structure
		if err := initializeNodeDirectory(n, genesisPath); err != nil {
			return nil, fmt.Errorf("failed to initialize node%d: %w", i, err)
		}

		if err := n.Save(); err != nil {
			return nil, fmt.Errorf("failed to save node%d config: %w", i, err)
		}

		nodes[i] = n
	}

	// Save metadata
	if err := metadata.Save(); err != nil {
		return nil, fmt.Errorf("failed to save metadata: %w", err)
	}

	devnet := NewDevnet(metadata, logger)
	devnet.Nodes = nodes

	// Step 5: Start nodes
	progress.Stage("Starting nodes")
	if err := devnet.StartNodes(ctx, genesisPath); err != nil {
		return nil, fmt.Errorf("failed to start nodes: %w", err)
	}

	// Wait for nodes to become healthy
	logger.Debug("Waiting for nodes to become healthy...")
	if err := node.WaitForAllNodesHealthy(ctx, nodes, HealthCheckTimeout); err != nil {
		logger.Warn("Not all nodes are healthy yet: %v", err)
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
	}
	return nil
}

// startNode starts a single node.
func (d *Devnet) startNode(ctx context.Context, n *node.Node, genesisPath string) error {
	switch d.Metadata.ExecutionMode {
	case ModeDocker:
		manager := node.NewDockerManager("", d.Logger)
		return manager.Start(ctx, n, genesisPath)
	case ModeLocal:
		manager := node.NewLocalManager("", d.Logger)
		return manager.Start(ctx, n, genesisPath)
	default:
		return fmt.Errorf("unknown execution mode: %s", d.Metadata.ExecutionMode)
	}
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

// initializeNodeDirectory creates the required directory structure for a node.
func initializeNodeDirectory(n *node.Node, genesisPath string) error {
	// Create directories
	dirs := []string{
		n.HomeDir,
		n.ConfigPath(),
		n.DataPath(),
		n.KeyringPath(),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create %s: %w", dir, err)
		}
	}

	// Copy genesis to node config directory
	destGenesis := filepath.Join(n.ConfigPath(), "genesis.json")
	if err := copyFile(genesisPath, destGenesis); err != nil {
		return fmt.Errorf("failed to copy genesis: %w", err)
	}

	return nil
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
