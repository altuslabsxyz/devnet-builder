package snapshot

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/b-harvest/devnet-builder/internal/output"
)

const (
	// MaxRetries is the maximum number of download retry attempts.
	MaxRetries = 3

	// RetryDelay is the delay between retry attempts.
	RetryDelay = 5 * time.Second

	// DownloadTimeout is the maximum time allowed for a download.
	DownloadTimeout = 30 * time.Minute
)

// DownloadOptions configures the download behavior.
type DownloadOptions struct {
	URL      string
	DestPath string
	Network  string
	HomeDir  string
	NoCache  bool
	Logger   *output.Logger
}

// Download downloads a snapshot file with retry logic.
// Returns the SnapshotCache entry on success.
func Download(ctx context.Context, opts DownloadOptions) (*SnapshotCache, error) {
	logger := opts.Logger
	if logger == nil {
		logger = output.DefaultLogger
	}

	// Check cache first (unless NoCache is set)
	if !opts.NoCache {
		cache, err := GetValidCache(opts.HomeDir, opts.Network)
		if err != nil {
			logger.Debug("Cache check failed: %v", err)
		}
		if cache != nil {
			logger.Debug("Using cached snapshot (expires in %s)", cache.TimeUntilExpiry().Round(time.Minute))
			return cache, nil
		}
	}

	// Determine decompressor and file extension
	decompressor, extension := detectDecompressor(opts.URL)

	// Set destination path if not provided
	destPath := opts.DestPath
	if destPath == "" {
		destPath = SnapshotPath(opts.HomeDir, opts.Network, extension)
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create download directory: %w", err)
	}

	// Download with retries
	var lastErr error
	for attempt := 1; attempt <= MaxRetries; attempt++ {
		if attempt > 1 {
			logger.Warn("Retry attempt %d/%d...", attempt, MaxRetries)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(RetryDelay):
			}
		}

		err := downloadFile(ctx, opts.URL, destPath, logger)
		if err == nil {
			// Get file size
			info, err := os.Stat(destPath)
			if err != nil {
				return nil, fmt.Errorf("failed to stat downloaded file: %w", err)
			}

			// Create cache entry
			cache := NewSnapshotCache(opts.Network, destPath, opts.URL, decompressor, info.Size())

			// Save cache metadata
			if err := cache.Save(opts.HomeDir); err != nil {
				logger.Warn("Failed to save cache metadata: %v", err)
			}

			return cache, nil
		}

		lastErr = err
		logger.Warn("Download failed: %v", err)
	}

	return nil, fmt.Errorf("failed to download snapshot after %d attempts: %w", MaxRetries, lastErr)
}

// downloadFile performs the actual HTTP download.
func downloadFile(ctx context.Context, url, destPath string, logger *output.Logger) error {
	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: DownloadTimeout,
	}

	// Create request with context
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Start download
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to start download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	// Create destination file
	tmpPath := destPath + ".tmp"
	out, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()

	// Copy with progress (simplified)
	contentLength := resp.ContentLength
	var downloaded int64

	// Create progress reporter
	progressReader := &progressReader{
		reader:         resp.Body,
		total:          contentLength,
		downloaded:     &downloaded,
		logger:         logger,
		lastReport:     time.Now(),
		reportInterval: 5 * time.Second,
	}

	_, err = io.Copy(out, progressReader)
	if err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to write file: %w", err)
	}

	// Close file before rename
	out.Close()

	// Rename temp file to final destination
	if err := os.Rename(tmpPath, destPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to rename file: %w", err)
	}

	return nil
}

// progressReader wraps an io.Reader to report download progress.
type progressReader struct {
	reader         io.Reader
	total          int64
	downloaded     *int64
	logger         *output.Logger
	lastReport     time.Time
	reportInterval time.Duration
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	*pr.downloaded += int64(n)

	// Report progress periodically
	if time.Since(pr.lastReport) >= pr.reportInterval {
		pr.lastReport = time.Now()
		if pr.total > 0 {
			percent := float64(*pr.downloaded) / float64(pr.total) * 100
			pr.logger.Debug("Downloaded %.1f%% (%d MB / %d MB)",
				percent,
				*pr.downloaded/(1024*1024),
				pr.total/(1024*1024))
		} else {
			pr.logger.Debug("Downloaded %d MB", *pr.downloaded/(1024*1024))
		}
	}

	return n, err
}

// detectDecompressor determines the decompressor and extension from URL.
func detectDecompressor(url string) (decompressor string, extension string) {
	if strings.HasSuffix(url, ".tar.zst") || strings.HasSuffix(url, ".zst") {
		return "zstd", ".tar.zst"
	}
	if strings.HasSuffix(url, ".tar.lz4") || strings.HasSuffix(url, ".lz4") {
		return "lz4", ".tar.lz4"
	}
	if strings.HasSuffix(url, ".tar.gz") || strings.HasSuffix(url, ".tgz") {
		return "gzip", ".tar.gz"
	}
	if strings.HasSuffix(url, ".tar") {
		return "none", ".tar"
	}
	return "zstd", ".tar.zst" // Default to zstd
}

// GetSnapshotSize returns the size of a remote snapshot without downloading it.
func GetSnapshotSize(ctx context.Context, url string) (int64, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to get size: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("failed with status %d", resp.StatusCode)
	}

	return resp.ContentLength, nil
}
