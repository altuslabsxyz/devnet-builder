package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/b-harvest/devnet-builder/internal/application/dto"
	"github.com/b-harvest/devnet-builder/internal/application/ports"
	"github.com/b-harvest/devnet-builder/internal/di"
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
	ctx := context.Background()

	// Create DI container for cache operations (uses default binary name)
	container, err := createCacheContainer()
	if err != nil {
		return fmt.Errorf("failed to initialize: %w", err)
	}

	// Execute CacheListUseCase
	result, err := container.CacheListUseCase().Execute(ctx, dto.CacheListInput{
		ShowDetails: true,
	})
	if err != nil {
		return fmt.Errorf("failed to list cache: %w", err)
	}

	if jsonMode {
		return outputCacheListJSON(result)
	}
	return outputCacheListText(result)
}

func outputCacheListJSON(result *dto.CacheListOutput) error {
	entries := make([]CacheEntryJSON, len(result.Binaries))
	for i, b := range result.Binaries {
		entries[i] = CacheEntryJSON{
			CommitHash: b.CommitHash,
			Ref:        b.Ref,
			BuildTime:  b.BuildTime,
			Size:       b.Size,
			SizeHuman:  formatBytes(b.Size),
			Network:    b.Network,
			Path:       b.Path,
		}
	}

	jsonResult := CacheListJSON{
		TotalEntries: len(result.Binaries),
		TotalSize:    result.TotalSize,
		TotalHuman:   formatBytes(result.TotalSize),
		Entries:      entries,
	}

	data, err := json.MarshalIndent(jsonResult, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

func outputCacheListText(result *dto.CacheListOutput) error {
	if len(result.Binaries) == 0 {
		fmt.Println("No cached binaries found.")
		fmt.Println()
		fmt.Println("Binaries are cached when you run:")
		fmt.Println("  devnet-builder upgrade --version <branch-or-commit>")
		return nil
	}

	// Sort by build time (newest first) - parse time strings for sorting
	binaries := make([]dto.CacheBinary, len(result.Binaries))
	copy(binaries, result.Binaries)
	sort.Slice(binaries, func(i, j int) bool {
		return binaries[i].BuildTime > binaries[j].BuildTime
	})

	output.Bold("Cached Binaries")
	fmt.Println("─────────────────────────────────────────────────────────────────────────")
	fmt.Printf("%-12s  %-20s  %-19s  %-10s  %s\n",
		"COMMIT", "REF", "BUILD TIME", "SIZE", "NETWORK")
	fmt.Println("─────────────────────────────────────────────────────────────────────────")

	for _, entry := range binaries {
		commitShort := entry.CommitHash
		if len(commitShort) > 12 {
			commitShort = commitShort[:12]
		}

		ref := entry.Ref
		if len(ref) > 20 {
			ref = ref[:17] + "..."
		}

		buildTime := entry.BuildTime
		size := formatBytes(entry.Size)
		network := entry.Network
		if network == "" {
			network = "mainnet"
		}

		fmt.Printf("%-12s  %-20s  %-19s  %-10s  %s\n",
			commitShort, ref, buildTime, size, network)
	}

	fmt.Println("─────────────────────────────────────────────────────────────────────────")
	fmt.Printf("Total: %d entries, %s\n", len(result.Binaries), formatBytes(result.TotalSize))
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
	ctx := context.Background()

	// Create DI container for cache operations
	container, err := createCacheContainer()
	if err != nil {
		return fmt.Errorf("failed to initialize: %w", err)
	}

	// Get cache info for confirmation
	cache := container.BinaryCache()
	stats := cache.Stats()

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

	// Execute CacheCleanUseCase
	result, err := container.CacheCleanUseCase().Execute(ctx, dto.CacheCleanInput{
		KeepActive: false,
	})
	if err != nil {
		return fmt.Errorf("failed to clean cache: %w", err)
	}

	if jsonMode {
		jsonResult := map[string]interface{}{
			"status":          "cleaned",
			"entries_removed": len(result.Removed),
			"bytes_freed":     result.SpaceFreed,
		}
		data, _ := json.MarshalIndent(jsonResult, "", "  ")
		fmt.Println(string(data))
	} else {
		output.Success("Cache cleaned: %d entries removed (%s freed)", len(result.Removed), formatBytes(result.SpaceFreed))
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
	// Create DI container for cache operations
	container, err := createCacheContainer()
	if err != nil {
		return fmt.Errorf("failed to initialize: %w", err)
	}

	cache := container.BinaryCache()
	stats := cache.Stats()
	symlinkInfo, _ := cache.SymlinkInfo()

	if jsonMode {
		return outputCacheInfoJSONClean(cache, symlinkInfo, stats)
	}
	return outputCacheInfoTextClean(cache, symlinkInfo, stats)
}

func outputCacheInfoJSONClean(cache ports.BinaryCache, symlinkInfo *ports.SymlinkInfo, stats ports.CacheStats) error {
	result := CacheInfoJSON{
		CacheDir:       cache.CacheDir(),
		SymlinkPath:    cache.SymlinkPath(),
		TotalEntries:   stats.TotalEntries,
		TotalSize:      stats.TotalSize,
		TotalSizeHuman: formatBytes(stats.TotalSize),
	}

	if symlinkInfo != nil {
		result.SymlinkExists = symlinkInfo.Exists
		result.SymlinkTarget = symlinkInfo.Target
		result.ActiveCommit = symlinkInfo.CommitHash
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

func outputCacheInfoTextClean(cache ports.BinaryCache, symlinkInfo *ports.SymlinkInfo, stats ports.CacheStats) error {
	output.Bold("Cache Information")
	fmt.Println("─────────────────────────────────────────────────────────────────────────")

	fmt.Printf("Cache Directory:  %s\n", cache.CacheDir())
	fmt.Printf("Symlink Path:     %s\n", cache.SymlinkPath())

	if symlinkInfo != nil {
		if symlinkInfo.Exists {
			fmt.Printf("Symlink Target:   %s\n", symlinkInfo.Target)
			if symlinkInfo.CommitHash != "" {
				commitShort := symlinkInfo.CommitHash
				if len(commitShort) > 12 {
					commitShort = commitShort[:12]
				}
				fmt.Printf("Active Commit:    %s\n", color.GreenString(commitShort))
			}
		} else if symlinkInfo.IsRegular {
			fmt.Printf("Binary Status:    %s (not a symlink)\n", color.YellowString("Direct file"))
		} else {
			fmt.Printf("Binary Status:    %s\n", color.YellowString("Not found"))
		}
	}

	fmt.Println()
	fmt.Printf("Cached Entries:   %d\n", stats.TotalEntries)
	fmt.Printf("Total Size:       %s\n", formatBytes(stats.TotalSize))

	fmt.Println("─────────────────────────────────────────────────────────────────────────")
	fmt.Println()

	return nil
}

// createCacheContainer creates a DI container configured for cache operations.
// It uses default binary name since cache operations don't require a network module.
func createCacheContainer() (*di.Container, error) {
	factory := di.NewInfrastructureFactory(homeDir, output.DefaultLogger)
	return factory.WireContainer()
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
