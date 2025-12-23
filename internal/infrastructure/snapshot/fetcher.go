// Package snapshot provides snapshot fetching and extraction implementations.
package snapshot

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/b-harvest/devnet-builder/internal/application/ports"
	"github.com/b-harvest/devnet-builder/internal/output"
)

// FetcherAdapter implements ports.SnapshotFetcher.
type FetcherAdapter struct {
	homeDir string
	logger  *output.Logger
}

// NewFetcherAdapter creates a new FetcherAdapter.
func NewFetcherAdapter(homeDir string, logger *output.Logger) *FetcherAdapter {
	if logger == nil {
		logger = output.DefaultLogger
	}
	return &FetcherAdapter{
		homeDir: homeDir,
		logger:  logger,
	}
}

// Download downloads a snapshot from the given URL.
func (f *FetcherAdapter) Download(ctx context.Context, url string, destPath string) error {
	opts := DownloadOptions{
		URL:      url,
		DestPath: destPath,
		HomeDir:  f.homeDir,
		NoCache:  false,
		Logger:   f.logger,
	}

	_, err := Download(ctx, opts)
	if err != nil {
		return &SnapshotError{
			Operation: "download",
			Message:   err.Error(),
		}
	}

	return nil
}

// DownloadWithCache downloads a snapshot with caching support.
// If a valid cached snapshot exists, returns the cached path without downloading.
// The cache is stored in ~/.devnet-builder/snapshots/<cacheKey>/ with 30-minute expiration.
// cacheKey format: "plugin-network" (e.g., "stable-mainnet", "ault-testnet")
func (f *FetcherAdapter) DownloadWithCache(ctx context.Context, url, cacheKey string, noCache bool) (string, bool, error) {
	// Check cache first (unless noCache is set)
	if !noCache {
		cache, err := GetValidCache(f.homeDir, cacheKey)
		if err != nil {
			f.logger.Debug("Cache check failed: %v", err)
		}
		if cache != nil {
			// Verify the file still exists
			if _, err := os.Stat(cache.FilePath); err == nil {
				f.logger.Info("Using cached snapshot (expires in %s)", cache.TimeUntilExpiry().Round(time.Minute))
				return cache.FilePath, true, nil
			}
			// File doesn't exist, clear invalid cache
			f.logger.Debug("Cached file not found, will re-download")
		}
	}

	// Download to cache directory
	opts := DownloadOptions{
		URL:      url,
		CacheKey: cacheKey,
		HomeDir:  f.homeDir,
		NoCache:  noCache,
		Logger:   f.logger,
	}

	cache, err := Download(ctx, opts)
	if err != nil {
		return "", false, &SnapshotError{
			Operation: "download",
			Message:   err.Error(),
		}
	}

	return cache.FilePath, false, nil
}

// Extract extracts a compressed snapshot.
// If extraction fails due to a corrupted archive, the cache is automatically cleared.
func (f *FetcherAdapter) Extract(ctx context.Context, archivePath, destPath string) error {
	// Detect decompressor from file extension
	decompressor := detectDecompressorFromPath(archivePath)

	// Get archive size for progress estimation
	archiveInfo, err := os.Stat(archivePath)
	if err != nil {
		return &SnapshotError{
			Operation: "extract",
			Message:   fmt.Sprintf("failed to stat archive: %v", err),
		}
	}

	// Ensure destination directory exists
	if err := os.MkdirAll(destPath, 0755); err != nil {
		return &SnapshotError{
			Operation: "extract",
			Message:   fmt.Sprintf("failed to create destination directory: %v", err),
		}
	}

	f.logger.Info("Extracting snapshot (%.1f MB)...", float64(archiveInfo.Size())/(1024*1024))

	var cmd *exec.Cmd

	switch decompressor {
	case "zstd":
		// Use pv for progress if available, otherwise fallback
		cmd = exec.CommandContext(ctx, "bash", "-c",
			fmt.Sprintf("zstd -d -c %q | tar xf - -C %q", archivePath, destPath))
	case "lz4":
		cmd = exec.CommandContext(ctx, "bash", "-c",
			fmt.Sprintf("lz4 -d -c %q | tar xf - -C %q", archivePath, destPath))
	case "gzip":
		cmd = exec.CommandContext(ctx, "tar", "xzf", archivePath, "-C", destPath)
	case "none":
		cmd = exec.CommandContext(ctx, "tar", "xf", archivePath, "-C", destPath)
	default:
		return &SnapshotError{
			Operation: "extract",
			Message:   fmt.Sprintf("unknown decompressor: %s", decompressor),
		}
	}

	cmdOutput, err := cmd.CombinedOutput()
	if err != nil {
		// Extraction failed - likely corrupted archive
		// Clear the cache so the next attempt will re-download
		cacheKey := extractCacheKeyFromPath(archivePath)
		if cacheKey != "" {
			f.logger.Warn("Extraction failed, clearing corrupted cache: %s", cacheKey)
			if clearErr := ClearCache(f.homeDir, cacheKey); clearErr != nil {
				f.logger.Debug("Failed to clear cache: %v", clearErr)
			}
		}

		return &SnapshotError{
			Operation: "extract",
			Message:   fmt.Sprintf("extraction failed: %v\nOutput: %s", err, string(cmdOutput)),
		}
	}

	f.logger.Success("Extraction complete")
	return nil
}

// extractCacheKeyFromPath extracts the cache key from a snapshot path.
// Expected path format: ~/.devnet-builder/snapshots/{cache-key}/snapshot.tar.{ext}
// cache-key format: "plugin-network" (e.g., "stable-mainnet", "ault-testnet")
func extractCacheKeyFromPath(archivePath string) string {
	// Get parent directory (should be cache key like "stable-mainnet")
	dir := filepath.Dir(archivePath)
	cacheKey := filepath.Base(dir)

	// Check if grandparent is "snapshots" to confirm path structure
	grandparent := filepath.Base(filepath.Dir(dir))
	if grandparent == "snapshots" {
		return cacheKey
	}

	return ""
}

// GetLatestSnapshotURL retrieves the latest snapshot URL for a cache key.
func (f *FetcherAdapter) GetLatestSnapshotURL(ctx context.Context, cacheKey string) (string, error) {
	// Check if there's a cached snapshot URL
	cache, err := GetValidCache(f.homeDir, cacheKey)
	if err != nil || cache == nil {
		return "", &SnapshotError{
			Operation: "get_latest_url",
			Message:   fmt.Sprintf("no cached snapshot URL for cache key %s", cacheKey),
		}
	}

	return cache.SourceURL, nil
}

// detectDecompressorFromPath determines the decompressor from file path.
func detectDecompressorFromPath(path string) string {
	lower := strings.ToLower(path)
	if strings.HasSuffix(lower, ".tar.zst") || strings.HasSuffix(lower, ".zst") {
		return "zstd"
	}
	if strings.HasSuffix(lower, ".tar.lz4") || strings.HasSuffix(lower, ".lz4") {
		return "lz4"
	}
	if strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz") {
		return "gzip"
	}
	if strings.HasSuffix(lower, ".tar") {
		return "none"
	}
	return "zstd" // Default
}

// StandardSnapshotPath returns the standard snapshot path for a cache key.
// cacheKey format: "plugin-network" (e.g., "stable-mainnet", "ault-testnet")
func StandardSnapshotPath(homeDir, cacheKey, extension string) string {
	return filepath.Join(homeDir, "snapshots", cacheKey, "snapshot"+extension)
}

// Ensure FetcherAdapter implements SnapshotFetcher.
var _ ports.SnapshotFetcher = (*FetcherAdapter)(nil)
