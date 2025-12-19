package provision

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/stablelabs/stable-devnet/genesis"
	"github.com/stablelabs/stable-devnet/internal/output"
	"github.com/stablelabs/stable-devnet/internal/snapshot"
)

// ExecutionMode defines how nodes are executed.
type ExecutionMode string

const (
	ModeDocker ExecutionMode = "docker"
	ModeLocal  ExecutionMode = "local"
)

// ProvisionerOptions configures the provisioner.
type ProvisionerOptions struct {
	Network     string
	Blockchain  string // blockchain module (stable, ault)
	ChainID     string
	HomeDir     string
	DockerImage string
	Mode        ExecutionMode
	NoCache     bool
	Logger      *output.Logger
}

// ProvisionResult contains the result of provisioning.
type ProvisionResult struct {
	GenesisPath   string
	ChainID       string
	SnapshotCache *snapshot.SnapshotCache
}

// Provisioner handles chain provisioning from snapshot.
type Provisioner struct {
	opts   *ProvisionerOptions
	logger *output.Logger
}

// NewProvisioner creates a new Provisioner.
func NewProvisioner(opts *ProvisionerOptions) *Provisioner {
	logger := opts.Logger
	if logger == nil {
		logger = output.DefaultLogger
	}

	return &Provisioner{
		opts:   opts,
		logger: logger,
	}
}

// Provision performs the complete provisioning workflow.
// 1. Download snapshot (with retry)
// 2. Export genesis from snapshot using ExportGenesisFromSnapshot
func (p *Provisioner) Provision(ctx context.Context) (*ProvisionResult, error) {
	p.logger.Debug("Starting provisioning for network: %s", p.opts.Network)

	// Step 1: Download snapshot with retry
	var cache *snapshot.SnapshotCache
	retryCfg := &RetryConfig{
		MaxRetries:     DefaultMaxRetries,
		InitialBackoff: DefaultInitialBackoff,
		MaxBackoff:     DefaultMaxBackoff,
		Logger:         p.logger,
	}

	err := WithRetry(ctx, retryCfg, func() error {
		var downloadErr error
		cache, downloadErr = p.DownloadSnapshot(ctx)
		return downloadErr
	})
	if err != nil {
		return nil, fmt.Errorf("failed to download snapshot: %w", err)
	}

	// Step 2: Get embedded genesis data
	blockchain := p.opts.Blockchain
	if blockchain == "" {
		blockchain = "stable" // default blockchain module
	}
	genesisData, err := genesis.GetGenesisData(blockchain, p.opts.Network)
	if err != nil {
		return nil, fmt.Errorf("no genesis file for blockchain %s, network %s", blockchain, p.opts.Network)
	}

	// Step 3: Export genesis from snapshot
	destGenesisPath := filepath.Join(p.opts.HomeDir, "devnet", "genesis.json")

	exportOpts := snapshot.ExportOptions{
		SnapshotPath: cache.FilePath,
		DestPath:     destGenesisPath,
		Network:      p.opts.Network,
		Decompressor: cache.Decompressor,
		DockerImage:  p.opts.DockerImage,
		GenesisData:  genesisData,
		UseDocker:    p.opts.Mode == ModeDocker,
		Logger:       p.logger,
	}

	p.logger.Debug("Exporting genesis from snapshot...")
	metadata, err := snapshot.ExportGenesisFromSnapshot(ctx, exportOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to export genesis: %w", err)
	}

	// Determine chain ID
	chainID := p.opts.ChainID
	if chainID == "" {
		chainID = metadata.OriginalChainID
	}
	if chainID == "" {
		chainID = p.getChainIDFromNetwork()
	}

	p.logger.Debug("Genesis exported successfully: %s (%d bytes)", destGenesisPath, metadata.SizeBytes)

	return &ProvisionResult{
		GenesisPath:   destGenesisPath,
		ChainID:       chainID,
		SnapshotCache: cache,
	}, nil
}

// DownloadSnapshot downloads the network snapshot.
func (p *Provisioner) DownloadSnapshot(ctx context.Context) (*snapshot.SnapshotCache, error) {
	p.logger.Debug("Downloading snapshot...")

	snapshotURL := GetSnapshotURL(p.opts.Network)
	if snapshotURL == "" {
		return nil, fmt.Errorf("no snapshot URL for network: %s", p.opts.Network)
	}

	return snapshot.Download(ctx, snapshot.DownloadOptions{
		URL:     snapshotURL,
		Network: p.opts.Network,
		HomeDir: p.opts.HomeDir,
		NoCache: p.opts.NoCache,
		Logger:  p.logger,
	})
}

// Cleanup is a no-op since ExportGenesisFromSnapshot handles its own cleanup.
func (p *Provisioner) Cleanup(ctx context.Context) error {
	return nil
}

func (p *Provisioner) getChainIDFromNetwork() string {
	switch p.opts.Network {
	case "mainnet":
		return "stable_988-1"
	case "testnet":
		return "stabletestnet_2201-1"
	default:
		return "stable_988-1"
	}
}

// GetDockerImage returns the Docker image to use based on configuration.
func GetDockerImage(stableVersion string) string {
	if stableVersion == "" || stableVersion == "latest" {
		return "ghcr.io/stablelabs/stable:latest"
	}
	// Docker tags cannot contain slashes - replace with dashes
	// e.g., "feat/gas-waiver" becomes "feat-gas-waiver"
	sanitizedVersion := strings.ReplaceAll(stableVersion, "/", "-")
	return fmt.Sprintf("ghcr.io/stablelabs/stable:%s", sanitizedVersion)
}

// GetSnapshotURL returns the snapshot download URL for the given network.
func GetSnapshotURL(network string) string {
	switch network {
	case "mainnet":
		return "https://stable-mainnet-data.s3.amazonaws.com/snapshots/stable_pruned.tar.zst"
	case "testnet":
		return "https://stable-testnet-data.s3.amazonaws.com/snapshots/stable_pruned.tar.zst"
	default:
		return ""
	}
}

// GetGenesisPath returns the path to the embedded genesis file for the given network.
// Deprecated: Use genesis.GetGenesisData() instead for embedded genesis support.
func GetGenesisPath(network string) string {
	// Get the directory where the binary is located or use relative path
	// First try relative to working directory
	paths := []string{
		fmt.Sprintf("genesis/%s-genesis.json", network),
		fmt.Sprintf("/home/ubuntu/stable-devnet/genesis/%s-genesis.json", network),
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}

// GetRPCEndpoint returns the RPC endpoint for the given network.
func GetRPCEndpoint(network string) string {
	switch network {
	case "mainnet":
		return "https://cosmos-rpc-internal.stable.xyz"
	case "testnet":
		return "https://cosmos-rpc.testnet.stable.xyz"
	default:
		return ""
	}
}

// copyFile copies a file from src to dst.
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// copyDir copies all files from src directory to dst directory.
func copyDir(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := os.MkdirAll(dstPath, 0755); err != nil {
				return err
			}
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}
	return nil
}

// verifyGenesis checks if a genesis file is valid JSON with chain_id.
func verifyGenesis(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var genesis struct {
		ChainID string `json:"chain_id"`
	}
	if err := json.Unmarshal(data, &genesis); err != nil {
		return fmt.Errorf("invalid genesis JSON: %w", err)
	}
	if genesis.ChainID == "" {
		return fmt.Errorf("genesis missing chain_id")
	}
	return nil
}
