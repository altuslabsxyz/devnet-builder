// Package devnet provides functionality for managing local blockchain development networks.
//
// The package is structured into several services:
//   - ProvisionService: handles devnet provisioning (provision.go)
//   - RunService: handles starting and running nodes (runner.go)
//   - HealthService: handles health checking (health.go)
//   - ResetService: handles data reset operations (reset.go)
//
// The Devnet struct is the main entry point for interacting with a running devnet.
package devnet

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/b-harvest/devnet-builder/internal/helpers"
	"github.com/b-harvest/devnet-builder/internal/node"
	"github.com/b-harvest/devnet-builder/internal/output"
	"github.com/b-harvest/devnet-builder/internal/provision"
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
	Blockchain       string // blockchain module (stable, ault)
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

// resolveBinaryPath returns the binary path for local mode.
// Uses CustomBinaryPath if set, otherwise defaults to ~/.stable-devnet/bin/{binaryName}.
func (d *Devnet) resolveBinaryPath() string {
	binaryPath := helpers.ResolveBinaryPath(d.Metadata.CustomBinaryPath, d.Metadata.HomeDir, d.Metadata.GetBinaryName())
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

// copyFile copies a file from src to dst.
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}
