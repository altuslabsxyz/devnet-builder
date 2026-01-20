package binary

import (
	"context"
	"fmt"
	"os"

	"github.com/altuslabsxyz/devnet-builder/internal/application/dto"
	"github.com/altuslabsxyz/devnet-builder/internal/application/ports"
	"github.com/altuslabsxyz/devnet-builder/pkg/network"
)

// ImportCustomBinaryUseCase orchestrates the import of a custom-built binary into the cache system.
// It coordinates between version detection, cache storage, and symlink management to:
//  1. Detect version information from the custom binary
//  2. Generate cache metadata based on detected version and build config
//  3. Store the binary in the cache with proper permissions
//  4. Activate the binary by creating a "current" symlink
//
// This use case follows Clean Architecture principles:
//   - Depends only on abstractions (ports interfaces)
//   - Contains core business logic for binary import
//   - Independent of infrastructure details (OS, filesystem, execution)
//
// SOLID Compliance:
//   - SRP: Single responsibility of importing custom binaries
//   - OCP: Extensible via dependency injection without modification
//   - LSP: Can be used wherever a binary import interface is expected
//   - ISP: Depends only on necessary interfaces (detector, cache)
//   - DIP: Depends on abstractions (BinaryVersionDetector, BinaryCache)
type ImportCustomBinaryUseCase struct {
	versionDetector ports.BinaryVersionDetector
	binaryCache     ports.BinaryCache
	homeDir         string
	binaryName      string
}

// NewImportCustomBinaryUseCase creates a new ImportCustomBinaryUseCase.
//
// Parameters:
//   - versionDetector: Detector for extracting version info from binaries
//   - binaryCache: Cache for storing and managing binaries
//   - homeDir: User's home directory for cache paths
//   - binaryName: Name of the binary (e.g., "stabled")
//
// Returns:
//   - *ImportCustomBinaryUseCase: Configured use case instance
func NewImportCustomBinaryUseCase(
	versionDetector ports.BinaryVersionDetector,
	binaryCache ports.BinaryCache,
	homeDir string,
	binaryName string,
) *ImportCustomBinaryUseCase {
	return &ImportCustomBinaryUseCase{
		versionDetector: versionDetector,
		binaryCache:     binaryCache,
		homeDir:         homeDir,
		binaryName:      binaryName,
	}
}

// Execute performs the custom binary import operation.
//
// Flow:
//  1. Validate input parameters
//  2. Detect version and commit hash from binary
//  3. Generate cache key from commit hash and build config
//  4. Store binary in cache (copy + permissions + metadata)
//  5. Set binary as active (create symlink)
//  6. Return import result with cache information
//
// Parameters:
//   - ctx: Context for cancellation and timeout control
//   - input: Import parameters including binary path, network type, build config
//
// Returns:
//   - *dto.CustomBinaryImportResult: Result containing cache paths and version info
//   - error: If any step fails
//
// Error Scenarios:
//   - Binary execution fails or times out
//   - Version output cannot be parsed
//   - File copy fails (disk space, permissions)
//   - Cache metadata write fails
//   - Symlink creation fails
func (u *ImportCustomBinaryUseCase) Execute(
	ctx context.Context,
	input *dto.CustomBinaryImportInput,
) (*dto.CustomBinaryImportResult, error) {
	// Step 1: Validate input
	if err := u.validateInput(input); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	// Step 2: Detect version information from binary
	versionInfo, err := u.versionDetector.DetectVersion(ctx, input.BinaryPath)
	if err != nil {
		return nil, fmt.Errorf("failed to detect binary version: %w", err)
	}

	// Step 3: Generate cache key for this binary
	// Cache key format: {networkType}/{commitHash}-{configHash}
	// If no build config provided, use empty config
	buildConfig := input.BuildConfig
	if buildConfig == nil {
		buildConfig = &network.BuildConfig{}
	}

	configHash := buildConfig.Hash()
	cacheKey := fmt.Sprintf("%s/%s-%s", input.NetworkType, versionInfo.CommitHash, configHash)

	// Step 4: Store binary in cache
	// The BinaryCache.Store() method:
	//  - Copies the binary file to cache directory
	//  - Sets executable permissions (0755)
	//  - Writes metadata.json with build config and version info
	//  - Updates the cache index
	//  - Returns the full path to the cached binary
	cachedBinaryPath, err := u.binaryCache.Store(ctx, cacheKey, input.BinaryPath)
	if err != nil {
		return nil, fmt.Errorf("failed to store binary in cache: %w", err)
	}

	// Step 5: Set as active binary (create symlink)
	// This creates an atomic symlink: ~/.devnet-builder/bin/{binaryName} -> cached binary
	if err := u.binaryCache.SetActive(cacheKey); err != nil {
		return nil, fmt.Errorf("failed to set binary as active: %w", err)
	}

	// Step 6: Get symlink path and file size for result
	symlinkPath := u.binaryCache.SymlinkPath()

	fileInfo, err := os.Stat(cachedBinaryPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat cached binary: %w", err)
	}

	return &dto.CustomBinaryImportResult{
		CacheKey:         cacheKey,
		CachedBinaryPath: cachedBinaryPath,
		SymlinkPath:      symlinkPath,
		Version:          versionInfo.Version,
		CommitHash:       versionInfo.CommitHash,
		Size:             fileInfo.Size(),
	}, nil
}

// validateInput performs validation on the import input parameters.
func (u *ImportCustomBinaryUseCase) validateInput(input *dto.CustomBinaryImportInput) error {
	if input == nil {
		return fmt.Errorf("input is nil")
	}

	if input.BinaryPath == "" {
		return fmt.Errorf("binary path is required")
	}

	if input.NetworkType == "" {
		return fmt.Errorf("network type is required")
	}

	// Verify binary exists (path should already be validated by CLI layer, but double-check)
	if _, err := os.Stat(input.BinaryPath); err != nil {
		return fmt.Errorf("binary not found at path %s: %w", input.BinaryPath, err)
	}

	return nil
}
