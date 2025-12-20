package application

import (
	"context"

	"github.com/b-harvest/devnet-builder/internal/builder"
	"github.com/b-harvest/devnet-builder/internal/cache"
	"github.com/b-harvest/devnet-builder/internal/network"
	"github.com/b-harvest/devnet-builder/internal/output"
)

// BuildServiceConfig holds configuration for BuildService.
type BuildServiceConfig struct {
	HomeDir       string
	Logger        *output.Logger
	NetworkModule network.NetworkModule
}

// BuildService handles binary building operations.
type BuildService struct {
	config  BuildServiceConfig
	builder *builder.Builder
	logger  *output.Logger
}

// NewBuildService creates a new BuildService with the given configuration.
func NewBuildService(config BuildServiceConfig) *BuildService {
	logger := config.Logger
	if logger == nil {
		logger = output.DefaultLogger
	}

	var b *builder.Builder
	if config.NetworkModule != nil {
		b = builder.NewBuilder(config.HomeDir, logger, config.NetworkModule)
	}

	return &BuildService{
		config:  config,
		builder: b,
		logger:  logger,
	}
}

// Build builds a binary from source and updates the symlink.
func (s *BuildService) Build(ctx context.Context, ref, networkType string) (*BuildResult, error) {
	if s.builder == nil {
		return nil, &ErrNoNetworkModule{}
	}

	result, err := s.builder.Build(ctx, builder.BuildOptions{
		Ref:     ref,
		Network: networkType,
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

// BuildToCache builds a binary and stores it in cache.
func (s *BuildService) BuildToCache(ctx context.Context, ref, networkType string) (*cache.CachedBinary, error) {
	if s.builder == nil {
		return nil, &ErrNoNetworkModule{}
	}

	binaryName := s.config.NetworkModule.BinaryName()
	binaryCache := cache.NewBinaryCache(s.config.HomeDir, binaryName, s.logger)
	if err := binaryCache.Initialize(); err != nil {
		return nil, err
	}

	return s.builder.BuildToCache(ctx, builder.BuildOptions{
		Ref:     ref,
		Network: networkType,
	}, binaryCache)
}

// IsBinaryBuilt checks if a binary exists for the given ref.
func (s *BuildService) IsBinaryBuilt(ref string) (string, bool) {
	if s.builder == nil {
		return "", false
	}
	return s.builder.IsBinaryBuilt(ref)
}

// GetBinaryPath returns the path where the binary would be.
func (s *BuildService) GetBinaryPath() string {
	if s.builder == nil {
		return ""
	}
	return s.builder.GetBinaryPath()
}

// ResolveCommitHash resolves a ref to a commit hash.
func (s *BuildService) ResolveCommitHash(ctx context.Context, ref string) (string, error) {
	if s.builder == nil {
		return "", &ErrNoNetworkModule{}
	}
	return s.builder.ResolveCommitHash(ctx, ref)
}
