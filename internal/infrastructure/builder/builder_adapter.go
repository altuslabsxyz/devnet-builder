// Package builder provides builder implementations.
package builder

import (
	"context"
	"fmt"

	"github.com/b-harvest/devnet-builder/internal/application/ports"
	"github.com/b-harvest/devnet-builder/internal/infrastructure/cache"
	"github.com/b-harvest/devnet-builder/internal/infrastructure/network"
	"github.com/b-harvest/devnet-builder/internal/output"
)

// BuilderAdapter implements ports.Builder.
type BuilderAdapter struct {
	homeDir string
	logger  *output.Logger
	module  network.NetworkModule
	builder *Builder
}

// NewBuilderAdapter creates a new BuilderAdapter.
func NewBuilderAdapter(homeDir string, logger *output.Logger, module network.NetworkModule) *BuilderAdapter {
	if logger == nil {
		logger = output.DefaultLogger
	}

	var b *Builder
	if module != nil {
		b = NewBuilder(homeDir, logger, module)
	}

	return &BuilderAdapter{
		homeDir: homeDir,
		logger:  logger,
		module:  module,
		builder: b,
	}
}

// Build builds a binary from source.
func (a *BuilderAdapter) Build(ctx context.Context, opts ports.BuildOptions) (*ports.BuildResult, error) {
	if a.builder == nil {
		return nil, &BuilderError{
			Operation: "build",
			Message:   "no network module configured",
		}
	}

	builderOpts := BuildOptions{
		Ref:     opts.Ref,
		Network: opts.Network,
	}

	result, err := a.builder.Build(ctx, builderOpts)
	if err != nil {
		return nil, &BuilderError{
			Operation: "build",
			Message:   err.Error(),
		}
	}

	return &ports.BuildResult{
		BinaryPath: result.BinaryPath,
		Ref:        result.Ref,
		CommitHash: result.CommitHash,
	}, nil
}

// BuildToCache builds and stores in cache WITHOUT updating the symlink.
// The symlink should only be updated after the upgrade is complete.
func (a *BuilderAdapter) BuildToCache(ctx context.Context, opts ports.BuildOptions) (*ports.BuildResult, error) {
	if a.builder == nil {
		return nil, &BuilderError{
			Operation: "build_to_cache",
			Message:   "no network module configured",
		}
	}

	binaryName := a.module.BinaryName()
	binaryCache := cache.NewBinaryCache(a.homeDir, binaryName, a.logger)
	if err := binaryCache.Initialize(); err != nil {
		return nil, &BuilderError{
			Operation: "build_to_cache",
			Message:   fmt.Sprintf("failed to initialize cache: %v", err),
		}
	}

	builderOpts := BuildOptions{
		Ref:     opts.Ref,
		Network: opts.Network,
	}

	cached, err := a.builder.BuildToCache(ctx, builderOpts, binaryCache)
	if err != nil {
		return nil, &BuilderError{
			Operation: "build_to_cache",
			Message:   err.Error(),
		}
	}

	// NOTE: Do NOT update symlink here - that should only happen after upgrade completes
	// The cached binary path is returned so the upgrade flow can use it directly

	cacheKey := cache.MakeCacheKeyLegacy(cached.CommitHash, cached.BuildTags)
	cachedPath := binaryCache.GetBinaryPath(cacheKey)
	return &ports.BuildResult{
		BinaryPath: cachedPath, // Return cached path, not symlink path
		Ref:        cached.Ref,
		CommitHash: cached.CommitHash,
		CachedPath: cachedPath,
	}, nil
}

// ResolveCommitHash resolves a ref to a commit hash.
func (a *BuilderAdapter) ResolveCommitHash(ctx context.Context, ref string) (string, error) {
	if a.builder == nil {
		return "", &BuilderError{
			Operation: "resolve_commit",
			Message:   "no network module configured",
		}
	}
	return a.builder.ResolveCommitHash(ctx, ref)
}

// IsBinaryBuilt checks if a binary exists for the given ref.
func (a *BuilderAdapter) IsBinaryBuilt(ref string) (string, bool) {
	if a.builder == nil {
		return "", false
	}
	return a.builder.IsBinaryBuilt(ref)
}

// GetBinaryPath returns the path where the binary would be.
func (a *BuilderAdapter) GetBinaryPath() string {
	if a.builder == nil {
		return ""
	}
	return a.builder.GetBinaryPath()
}

// Ensure BuilderAdapter implements ports.Builder.
var _ ports.Builder = (*BuilderAdapter)(nil)
