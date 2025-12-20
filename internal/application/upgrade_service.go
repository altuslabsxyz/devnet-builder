package application

import (
	"context"

	"github.com/b-harvest/devnet-builder/internal/builder"
	"github.com/b-harvest/devnet-builder/internal/cache"
	"github.com/b-harvest/devnet-builder/internal/devnet"
	"github.com/b-harvest/devnet-builder/internal/network"
	"github.com/b-harvest/devnet-builder/internal/output"
	"github.com/b-harvest/devnet-builder/internal/upgrade"
)

// UpgradeServiceConfig holds configuration for UpgradeService.
type UpgradeServiceConfig struct {
	HomeDir       string
	Logger        *output.Logger
	NetworkModule network.NetworkModule
}

// UpgradeService orchestrates upgrade operations.
// It coordinates between build, proposal, and switch services.
type UpgradeService struct {
	config  UpgradeServiceConfig
	builder *builder.Builder
	logger  *output.Logger
}

// NewUpgradeService creates a new UpgradeService with the given configuration.
func NewUpgradeService(config UpgradeServiceConfig) *UpgradeService {
	logger := config.Logger
	if logger == nil {
		logger = output.DefaultLogger
	}

	var b *builder.Builder
	if config.NetworkModule != nil {
		b = builder.NewBuilder(config.HomeDir, logger, config.NetworkModule)
	}

	return &UpgradeService{
		config:  config,
		builder: b,
		logger:  logger,
	}
}

// BuildOptions contains options for building a binary.
type BuildOptions struct {
	Ref     string // Git ref (branch, tag, commit)
	Network string // Network type (mainnet, testnet)
}

// BuildResult contains the result of a build operation.
type BuildResult struct {
	BinaryPath string
	Ref        string
	CommitHash string
}

// Build builds a binary from source and makes it active.
func (s *UpgradeService) Build(ctx context.Context, opts BuildOptions) (*BuildResult, error) {
	if s.builder == nil {
		return nil, &ErrNoNetworkModule{}
	}

	result, err := s.builder.Build(ctx, builder.BuildOptions{
		Ref:     opts.Ref,
		Network: opts.Network,
	})
	if err != nil {
		return nil, err
	}

	return &BuildResult{
		BinaryPath: result.BinaryPath,
		Ref:        result.Ref,
		CommitHash: result.CommitHash,
	}, nil
}

// BuildToCache builds a binary and stores it in cache without making it active.
func (s *UpgradeService) BuildToCache(ctx context.Context, opts BuildOptions) (*cache.CachedBinary, error) {
	if s.builder == nil {
		return nil, &ErrNoNetworkModule{}
	}

	binaryName := s.config.NetworkModule.BinaryName()
	binaryCache := cache.NewBinaryCache(s.config.HomeDir, binaryName, s.logger)
	if err := binaryCache.Initialize(); err != nil {
		return nil, err
	}

	return s.builder.BuildToCache(ctx, builder.BuildOptions{
		Ref:     opts.Ref,
		Network: opts.Network,
	}, binaryCache)
}

// UpgradeOptions contains options for performing an upgrade.
type UpgradeOptions struct {
	TargetVersion string
	Network       string
	SkipBuild     bool
	DryRun        bool
}

// Upgrade performs a full upgrade workflow.
func (s *UpgradeService) Upgrade(ctx context.Context, d *devnet.Devnet, opts UpgradeOptions) error {
	// Build upgrade config
	cfg := &upgrade.UpgradeConfig{
		Name:          opts.TargetVersion,
		TargetVersion: opts.TargetVersion,
	}

	// Build execute options
	execOpts := &upgrade.ExecuteOptions{
		HomeDir:  s.config.HomeDir,
		Metadata: d.Metadata,
		Logger:   s.logger,
	}

	_, err := upgrade.ExecuteUpgrade(ctx, cfg, execOpts)
	return err
}

// ErrNoNetworkModule indicates that no network module is configured.
type ErrNoNetworkModule struct{}

func (e *ErrNoNetworkModule) Error() string {
	return "no network module configured - use a network plugin"
}
