// Package builder provides builder implementations.
package builder

import (
	"context"
	"fmt"

	"github.com/b-harvest/devnet-builder/internal/application/ports"
	legacybuilder "github.com/b-harvest/devnet-builder/internal/builder"
	legacycache "github.com/b-harvest/devnet-builder/internal/cache"
	"github.com/b-harvest/devnet-builder/internal/network"
	"github.com/b-harvest/devnet-builder/internal/output"
)

// BuilderAdapter adapts the legacy builder.Builder to ports.Builder interface.
type BuilderAdapter struct {
	homeDir string
	logger  *output.Logger
	module  network.NetworkModule
	builder *legacybuilder.Builder
}

// NewBuilderAdapter creates a new BuilderAdapter.
func NewBuilderAdapter(homeDir string, logger *output.Logger, module network.NetworkModule) *BuilderAdapter {
	if logger == nil {
		logger = output.DefaultLogger
	}

	var b *legacybuilder.Builder
	if module != nil {
		b = legacybuilder.NewBuilder(homeDir, logger, module)
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

	legacyOpts := legacybuilder.BuildOptions{
		Ref:     opts.Ref,
		Network: opts.Network,
	}

	result, err := a.builder.Build(ctx, legacyOpts)
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

// BuildToCache builds and stores in cache without activating.
func (a *BuilderAdapter) BuildToCache(ctx context.Context, opts ports.BuildOptions) (*ports.BuildResult, error) {
	if a.builder == nil {
		return nil, &BuilderError{
			Operation: "build_to_cache",
			Message:   "no network module configured",
		}
	}

	binaryName := a.module.BinaryName()
	binaryCache := legacycache.NewBinaryCache(a.homeDir, binaryName, a.logger)
	if err := binaryCache.Initialize(); err != nil {
		return nil, &BuilderError{
			Operation: "build_to_cache",
			Message:   fmt.Sprintf("failed to initialize cache: %v", err),
		}
	}

	legacyOpts := legacybuilder.BuildOptions{
		Ref:     opts.Ref,
		Network: opts.Network,
	}

	cached, err := a.builder.BuildToCache(ctx, legacyOpts, binaryCache)
	if err != nil {
		return nil, &BuilderError{
			Operation: "build_to_cache",
			Message:   err.Error(),
		}
	}

	return &ports.BuildResult{
		BinaryPath: cached.BinaryPath,
		Ref:        cached.Ref,
		CommitHash: cached.CommitHash,
		CachedPath: binaryCache.GetBinaryPath(cached.CommitHash),
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
