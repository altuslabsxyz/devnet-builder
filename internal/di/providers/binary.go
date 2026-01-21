package providers

import (
	"sync"

	"github.com/altuslabsxyz/devnet-builder/internal/application/binary"
)

// BinaryUseCasesProvider provides access to binary-related use cases.
// This provider groups all use cases related to binary management:
// passthrough execution and importing custom binaries.
type BinaryUseCasesProvider interface {
	PassthroughUseCase() *binary.PassthroughUseCase
	ImportCustomBinaryUseCase(homeDir string) *binary.ImportCustomBinaryUseCase
}

// binaryUseCases is the concrete implementation of BinaryUseCasesProvider.
type binaryUseCases struct {
	mu    sync.RWMutex
	infra InfrastructureProvider

	// Lazy-initialized use cases
	passthroughUC        *binary.PassthroughUseCase
	importCustomBinaryUC *binary.ImportCustomBinaryUseCase
}

// NewBinaryUseCases creates a new BinaryUseCasesProvider.
func NewBinaryUseCases(infra InfrastructureProvider) BinaryUseCasesProvider {
	return &binaryUseCases{
		infra: infra,
	}
}

// PassthroughUseCase returns the binary passthrough use case (lazy init).
func (b *binaryUseCases) PassthroughUseCase() *binary.PassthroughUseCase {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.passthroughUC == nil {
		b.passthroughUC = binary.NewPassthroughUseCase(
			b.infra.BinaryResolver(),
			b.infra.BinaryExecutor(),
		)
	}
	return b.passthroughUC
}

// ImportCustomBinaryUseCase returns the import custom binary use case (lazy init).
// The homeDir and binaryName are determined from the NetworkModule if available.
func (b *binaryUseCases) ImportCustomBinaryUseCase(homeDir string) *binary.ImportCustomBinaryUseCase {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.importCustomBinaryUC == nil {
		// Get binary name from network module
		binaryName := ""
		if nm := b.infra.NetworkModule(); nm != nil {
			binaryName = nm.BinaryName()
		}

		b.importCustomBinaryUC = binary.NewImportCustomBinaryUseCase(
			b.infra.BinaryVersionDetector(),
			b.infra.BinaryCache(),
			homeDir,
			binaryName,
		)
	}
	return b.importCustomBinaryUC
}
