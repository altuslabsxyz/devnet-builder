package providers

import (
	"sync"

	"github.com/altuslabsxyz/devnet-builder/internal/application/build"
)

// BuildUseCasesProvider provides access to build-related use cases.
// This provider groups all use cases related to building binaries:
// building from source, listing cached binaries, and cleaning the cache.
type BuildUseCasesProvider interface {
	BuildUseCase() *build.BuildUseCase
	CacheListUseCase() *build.CacheListUseCase
	CacheCleanUseCase() *build.CacheCleanUseCase
}

// buildUseCases is the concrete implementation of BuildUseCasesProvider.
type buildUseCases struct {
	mu    sync.RWMutex
	infra InfrastructureProvider

	// Lazy-initialized use cases
	buildUC      *build.BuildUseCase
	cacheListUC  *build.CacheListUseCase
	cacheCleanUC *build.CacheCleanUseCase
}

// NewBuildUseCases creates a new BuildUseCasesProvider.
func NewBuildUseCases(infra InfrastructureProvider) BuildUseCasesProvider {
	return &buildUseCases{
		infra: infra,
	}
}

// BuildUseCase returns the build use case (lazy init).
func (b *buildUseCases) BuildUseCase() *build.BuildUseCase {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.buildUC == nil {
		b.buildUC = build.NewBuildUseCase(
			b.infra.Builder(),
			b.infra.BinaryCache(),
			b.infra.Logger(),
		)
	}
	return b.buildUC
}

// CacheListUseCase returns the cache list use case (lazy init).
func (b *buildUseCases) CacheListUseCase() *build.CacheListUseCase {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.cacheListUC == nil {
		b.cacheListUC = build.NewCacheListUseCase(
			b.infra.BinaryCache(),
			b.infra.Logger(),
		)
	}
	return b.cacheListUC
}

// CacheCleanUseCase returns the cache clean use case (lazy init).
func (b *buildUseCases) CacheCleanUseCase() *build.CacheCleanUseCase {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.cacheCleanUC == nil {
		b.cacheCleanUC = build.NewCacheCleanUseCase(
			b.infra.BinaryCache(),
			b.infra.Logger(),
		)
	}
	return b.cacheCleanUC
}
