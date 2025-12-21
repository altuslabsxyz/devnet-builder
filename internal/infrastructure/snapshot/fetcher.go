// Package snapshot provides snapshot fetching and extraction implementations.
package snapshot

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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

// Extract extracts a compressed snapshot.
func (f *FetcherAdapter) Extract(ctx context.Context, archivePath, destPath string) error {
	// Detect decompressor from file extension
	decompressor := detectDecompressorFromPath(archivePath)

	// Ensure destination directory exists
	if err := os.MkdirAll(destPath, 0755); err != nil {
		return &SnapshotError{
			Operation: "extract",
			Message:   fmt.Sprintf("failed to create destination directory: %v", err),
		}
	}

	var cmd *exec.Cmd

	switch decompressor {
	case "zstd":
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
		return &SnapshotError{
			Operation: "extract",
			Message:   fmt.Sprintf("extraction failed: %v\nOutput: %s", err, string(cmdOutput)),
		}
	}

	return nil
}

// GetLatestSnapshotURL retrieves the latest snapshot URL for a network.
func (f *FetcherAdapter) GetLatestSnapshotURL(ctx context.Context, network string) (string, error) {
	// Check if there's a cached snapshot URL
	cache, err := GetValidCache(f.homeDir, network)
	if err != nil || cache == nil {
		return "", &SnapshotError{
			Operation: "get_latest_url",
			Message:   fmt.Sprintf("no cached snapshot URL for network %s", network),
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

// StandardSnapshotPath returns the standard snapshot path for a network.
func StandardSnapshotPath(homeDir, network, extension string) string {
	return filepath.Join(homeDir, "snapshots", network, "snapshot"+extension)
}

// Ensure FetcherAdapter implements SnapshotFetcher.
var _ ports.SnapshotFetcher = (*FetcherAdapter)(nil)
