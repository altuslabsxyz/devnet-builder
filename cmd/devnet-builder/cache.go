package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/b-harvest/devnet-builder/internal/cache"
	"github.com/b-harvest/devnet-builder/internal/output"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

// NewCacheCmd creates the cache command group.
func NewCacheCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cache",
		Short: "Manage the binary cache",
		Long: `Manage the binary cache used for upgrade operations.

The cache stores pre-built binaries indexed by commit hash, allowing:
- Fast upgrades without rebuilding
- Atomic symlink switching to avoid "text file busy" errors
- Binary reuse across multiple upgrades to the same version

Examples:
  # List all cached binaries
  devnet-builder cache list

  # Show current symlink target
  devnet-builder cache info

  # Clean all cached binaries
  devnet-builder cache clean`,
	}

	cmd.AddCommand(
		NewCacheListCmd(),
		NewCacheCleanCmd(),
		NewCacheInfoCmd(),
	)

	return cmd
}

// CacheEntryJSON represents a cache entry in JSON format.
type CacheEntryJSON struct {
	CommitHash string `json:"commit_hash"`
	Ref        string `json:"ref"`
	BuildTime  string `json:"build_time"`
	Size       int64  `json:"size"`
	SizeHuman  string `json:"size_human"`
	Network    string `json:"network"`
	Path       string `json:"path"`
}

// CacheListJSON represents the cache list in JSON format.
type CacheListJSON struct {
	TotalEntries int              `json:"total_entries"`
	TotalSize    int64            `json:"total_size"`
	TotalHuman   string           `json:"total_size_human"`
	Entries      []CacheEntryJSON `json:"entries"`
}

// NewCacheListCmd creates the cache list command.
func NewCacheListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all cached binaries",
		Long: `List all cached binaries in the cache directory.

Shows commit hash, ref, build time, size, and network for each cached binary.`,
		RunE: runCacheList,
	}
}

func runCacheList(cmd *cobra.Command, args []string) error {
	logger := output.DefaultLogger

	// Initialize cache
	// Note: For cache list, we use default binary name since we list all cached entries
	binaryCache := cache.NewBinaryCache(homeDir, "", logger)
	if err := binaryCache.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize cache: %w", err)
	}

	entries := binaryCache.List()
	stats := binaryCache.Stats()

	// Sort by build time (newest first)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].BuildTime.After(entries[j].BuildTime)
	})

	if jsonMode {
		return outputCacheListJSON(entries, stats)
	}
	return outputCacheListText(entries, stats)
}

func outputCacheListJSON(entries []*cache.CachedBinary, stats *cache.CacheStats) error {
	result := CacheListJSON{
		TotalEntries: stats.TotalEntries,
		TotalSize:    stats.TotalSize,
		TotalHuman:   formatBytes(stats.TotalSize),
		Entries:      make([]CacheEntryJSON, len(entries)),
	}

	for i, entry := range entries {
		result.Entries[i] = CacheEntryJSON{
			CommitHash: entry.CommitHash,
			Ref:        entry.Ref,
			BuildTime:  entry.BuildTime.Format(time.RFC3339),
			Size:       entry.Size,
			SizeHuman:  formatBytes(entry.Size),
			Network:    entry.Network,
			Path:       entry.BinaryPath,
		}
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

func outputCacheListText(entries []*cache.CachedBinary, stats *cache.CacheStats) error {
	if len(entries) == 0 {
		fmt.Println("No cached binaries found.")
		fmt.Println()
		fmt.Println("Binaries are cached when you run:")
		fmt.Println("  devnet-builder upgrade --version <branch-or-commit>")
		return nil
	}

	output.Bold("Cached Binaries")
	fmt.Println("─────────────────────────────────────────────────────────────────────────")
	fmt.Printf("%-12s  %-20s  %-19s  %-10s  %s\n",
		"COMMIT", "REF", "BUILD TIME", "SIZE", "NETWORK")
	fmt.Println("─────────────────────────────────────────────────────────────────────────")

	for _, entry := range entries {
		commitShort := entry.CommitHash
		if len(commitShort) > 12 {
			commitShort = commitShort[:12]
		}

		ref := entry.Ref
		if len(ref) > 20 {
			ref = ref[:17] + "..."
		}

		buildTime := entry.BuildTime.Format("2006-01-02 15:04:05")
		size := formatBytes(entry.Size)
		network := entry.Network
		if network == "" {
			network = "mainnet"
		}

		fmt.Printf("%-12s  %-20s  %-19s  %-10s  %s\n",
			commitShort, ref, buildTime, size, network)
	}

	fmt.Println("─────────────────────────────────────────────────────────────────────────")
	fmt.Printf("Total: %d entries, %s\n", stats.TotalEntries, formatBytes(stats.TotalSize))
	fmt.Println()

	return nil
}

// NewCacheCleanCmd creates the cache clean command.
func NewCacheCleanCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "clean",
		Short: "Clean all cached binaries",
		Long: `Remove all cached binaries from the cache directory.

This frees up disk space but will require rebuilding binaries on next upgrade.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCacheClean(force)
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation prompt")

	return cmd
}

func runCacheClean(force bool) error {
	logger := output.DefaultLogger

	// Initialize cache
	// Note: For cache clean, we use default binary name since we clean all cached entries
	binaryCache := cache.NewBinaryCache(homeDir, "", logger)
	if err := binaryCache.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize cache: %w", err)
	}

	stats := binaryCache.Stats()

	if stats.TotalEntries == 0 {
		fmt.Println("Cache is already empty.")
		return nil
	}

	// Confirm unless forced
	if !force && !jsonMode {
		fmt.Printf("This will remove %d cached binaries (%s).\n", stats.TotalEntries, formatBytes(stats.TotalSize))
		confirmed, err := confirmPrompt("Proceed with cache clean?")
		if err != nil {
			return err
		}
		if !confirmed {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	// Clean cache
	if err := binaryCache.Clean(); err != nil {
		return fmt.Errorf("failed to clean cache: %w", err)
	}

	if jsonMode {
		result := map[string]interface{}{
			"status":          "cleaned",
			"entries_removed": stats.TotalEntries,
			"bytes_freed":     stats.TotalSize,
		}
		data, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(data))
	} else {
		output.Success("Cache cleaned: %d entries removed (%s freed)", stats.TotalEntries, formatBytes(stats.TotalSize))
	}

	return nil
}

// CacheInfoJSON represents cache info in JSON format.
type CacheInfoJSON struct {
	CacheDir       string `json:"cache_dir"`
	SymlinkPath    string `json:"symlink_path"`
	SymlinkExists  bool   `json:"symlink_exists"`
	SymlinkTarget  string `json:"symlink_target,omitempty"`
	ActiveCommit   string `json:"active_commit,omitempty"`
	TotalEntries   int    `json:"total_entries"`
	TotalSize      int64  `json:"total_size"`
	TotalSizeHuman string `json:"total_size_human"`
}

// NewCacheInfoCmd creates the cache info command.
func NewCacheInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info",
		Short: "Show cache information and current symlink target",
		Long: `Show information about the binary cache and current symlink.

Displays:
- Cache directory location
- Current symlink path and target
- Active commit hash
- Total cached entries and size`,
		RunE: runCacheInfo,
	}
}

func runCacheInfo(cmd *cobra.Command, args []string) error {
	logger := output.DefaultLogger

	// Initialize cache
	// Note: For cache info, we use default binary name for general overview
	binaryCache := cache.NewBinaryCache(homeDir, "", logger)
	if err := binaryCache.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize cache: %w", err)
	}

	// Get symlink info
	// Note: Using default binary name; actual binary name depends on network
	symlinkMgr := cache.NewSymlinkManager(homeDir, "")
	symlink, err := symlinkMgr.GetCurrent()
	if err != nil {
		logger.Debug("Failed to get symlink info: %v", err)
	}

	stats := binaryCache.Stats()

	if jsonMode {
		return outputCacheInfoJSON(binaryCache, symlinkMgr, symlink, stats)
	}
	return outputCacheInfoText(binaryCache, symlinkMgr, symlink, stats)
}

func outputCacheInfoJSON(binaryCache *cache.BinaryCache, symlinkMgr *cache.SymlinkManager, symlink *cache.ActiveSymlink, stats *cache.CacheStats) error {
	result := CacheInfoJSON{
		CacheDir:       binaryCache.CacheDir(),
		SymlinkPath:    symlinkMgr.SymlinkPath(),
		SymlinkExists:  symlink != nil,
		TotalEntries:   stats.TotalEntries,
		TotalSize:      stats.TotalSize,
		TotalSizeHuman: formatBytes(stats.TotalSize),
	}

	if symlink != nil {
		result.SymlinkTarget = symlink.Target
		result.ActiveCommit = symlink.CommitHash
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

func outputCacheInfoText(binaryCache *cache.BinaryCache, symlinkMgr *cache.SymlinkManager, symlink *cache.ActiveSymlink, stats *cache.CacheStats) error {
	output.Bold("Cache Information")
	fmt.Println("─────────────────────────────────────────────────────────────────────────")

	fmt.Printf("Cache Directory:  %s\n", binaryCache.CacheDir())
	fmt.Printf("Symlink Path:     %s\n", symlinkMgr.SymlinkPath())

	if symlink != nil {
		fmt.Printf("Symlink Target:   %s\n", symlink.Target)
		if symlink.CommitHash != "" {
			fmt.Printf("Active Commit:    %s\n", color.GreenString(symlink.CommitHash[:12]))
		}
	} else if symlinkMgr.IsRegularFile() {
		fmt.Printf("Binary Status:    %s (not a symlink)\n", color.YellowString("Direct file"))
	} else {
		fmt.Printf("Binary Status:    %s\n", color.YellowString("Not found"))
	}

	fmt.Println()
	fmt.Printf("Cached Entries:   %d\n", stats.TotalEntries)
	fmt.Printf("Total Size:       %s\n", formatBytes(stats.TotalSize))

	fmt.Println("─────────────────────────────────────────────────────────────────────────")
	fmt.Println()

	return nil
}

// formatBytes formats bytes as human-readable string.
func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
