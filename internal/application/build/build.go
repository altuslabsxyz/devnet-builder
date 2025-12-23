// Package build contains UseCases for building binaries from source.
package build

import (
	"context"
	"fmt"

	"github.com/b-harvest/devnet-builder/internal/application/dto"
	"github.com/b-harvest/devnet-builder/internal/application/ports"
)

// BuildUseCase handles building binaries from source.
type BuildUseCase struct {
	builder     ports.Builder
	binaryCache ports.BinaryCache
	logger      ports.Logger
}

// NewBuildUseCase creates a new BuildUseCase.
func NewBuildUseCase(
	builder ports.Builder,
	binaryCache ports.BinaryCache,
	logger ports.Logger,
) *BuildUseCase {
	return &BuildUseCase{
		builder:     builder,
		binaryCache: binaryCache,
		logger:      logger,
	}
}

// Execute builds a binary from source.
func (uc *BuildUseCase) Execute(ctx context.Context, input dto.BuildInput) (*dto.BuildOutput, error) {
	uc.logger.Info("Building binary for ref: %s", input.Ref)

	// Check cache first if requested
	if input.UseCache {
		if cachedPath, ok := uc.binaryCache.Get(input.Ref); ok {
			uc.logger.Success("Using cached binary: %s", cachedPath)
			return &dto.BuildOutput{
				BinaryPath: cachedPath,
				Ref:        input.Ref,
				FromCache:  true,
			}, nil
		}
	}

	// Build the binary
	buildOpts := ports.BuildOptions{
		Ref:      input.Ref,
		Network:  input.Network,
		UseCache: false,
	}

	var result *ports.BuildResult
	var err error

	if input.ToCache {
		result, err = uc.builder.BuildToCache(ctx, buildOpts)
	} else {
		result, err = uc.builder.Build(ctx, buildOpts)
	}

	if err != nil {
		return nil, fmt.Errorf("build failed: %w", err)
	}

	output := &dto.BuildOutput{
		BinaryPath: result.BinaryPath,
		Ref:        result.Ref,
		CommitHash: result.CommitHash,
		CachedPath: result.CachedPath,
		FromCache:  false,
	}

	uc.logger.Success("Build complete: %s", result.BinaryPath)
	return output, nil
}

// CacheListUseCase handles listing cached binaries.
type CacheListUseCase struct {
	binaryCache ports.BinaryCache
	logger      ports.Logger
}

// NewCacheListUseCase creates a new CacheListUseCase.
func NewCacheListUseCase(binaryCache ports.BinaryCache, logger ports.Logger) *CacheListUseCase {
	return &CacheListUseCase{
		binaryCache: binaryCache,
		logger:      logger,
	}
}

// Execute lists cached binaries.
func (uc *CacheListUseCase) Execute(ctx context.Context, input dto.CacheListInput) (*dto.CacheListOutput, error) {
	entries := uc.binaryCache.ListDetailed()
	stats := uc.binaryCache.Stats()

	// Get active symlink info
	var activeRef string
	symlinkInfo, err := uc.binaryCache.SymlinkInfo()
	if err == nil && symlinkInfo.Exists {
		activeRef = symlinkInfo.CommitHash
	}

	output := &dto.CacheListOutput{
		Binaries:  make([]dto.CacheBinary, len(entries)),
		ActiveRef: activeRef,
		TotalSize: stats.TotalSize,
	}

	for i, entry := range entries {
		buildTime := ""
		if !entry.BuildTime.IsZero() {
			buildTime = entry.BuildTime.Format("2006-01-02 15:04:05")
		}
		output.Binaries[i] = dto.CacheBinary{
			Ref:        entry.Ref,
			Path:       entry.Path,
			CommitHash: entry.CommitHash,
			IsActive:   entry.CommitHash == activeRef,
			Size:       entry.Size,
			BuildTime:  buildTime,
			Network:    entry.Network,
		}
	}

	return output, nil
}

// CacheCleanUseCase handles cleaning the binary cache.
type CacheCleanUseCase struct {
	binaryCache ports.BinaryCache
	logger      ports.Logger
}

// NewCacheCleanUseCase creates a new CacheCleanUseCase.
func NewCacheCleanUseCase(binaryCache ports.BinaryCache, logger ports.Logger) *CacheCleanUseCase {
	return &CacheCleanUseCase{
		binaryCache: binaryCache,
		logger:      logger,
	}
}

// Execute cleans the binary cache.
func (uc *CacheCleanUseCase) Execute(ctx context.Context, input dto.CacheCleanInput) (*dto.CacheCleanOutput, error) {
	refs := uc.binaryCache.List()
	activeRef, _ := uc.binaryCache.GetActive()

	output := &dto.CacheCleanOutput{
		Removed: make([]string, 0),
		Kept:    make([]string, 0),
	}

	for _, ref := range refs {
		// Skip active if requested
		if input.KeepActive && ref == activeRef {
			output.Kept = append(output.Kept, ref)
			continue
		}

		// Skip if in explicit keep list
		shouldKeep := false
		for _, keepRef := range input.Refs {
			if ref == keepRef {
				shouldKeep = true
				break
			}
		}
		if len(input.Refs) > 0 && !shouldKeep {
			output.Kept = append(output.Kept, ref)
			continue
		}

		// Remove the binary
		if err := uc.binaryCache.Remove(ref); err != nil {
			uc.logger.Warn("Failed to remove %s: %v", ref, err)
			continue
		}
		output.Removed = append(output.Removed, ref)
	}

	uc.logger.Success("Cleaned %d cached binaries", len(output.Removed))
	return output, nil
}
