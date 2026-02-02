// internal/daemon/provisioner/genesis_forker.go
package provisioner

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/application/ports"
	"github.com/altuslabsxyz/devnet-builder/internal/plugin/types"
)

// GenesisForkerConfig configures the genesis forker
type GenesisForkerConfig struct {
	DataDir            string
	PluginGenesis      types.PluginGenesis
	GenesisFetcher     ports.GenesisFetcher     // optional: existing infrastructure
	SnapshotFetcher    ports.SnapshotFetcher    // optional: existing infrastructure
	StateExportService ports.StateExportService // optional: existing infrastructure
	Logger             *slog.Logger
}

// GenesisForker handles genesis forking from various sources
type GenesisForker struct {
	config GenesisForkerConfig
	logger *slog.Logger
}

// Compile-time interface compliance check
var _ ports.GenesisForker = (*GenesisForker)(nil)

// NewGenesisForker creates a new genesis forker
func NewGenesisForker(config GenesisForkerConfig) *GenesisForker {
	logger := config.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &GenesisForker{
		config: config,
		logger: logger,
	}
}

// reportStep is a helper to safely report progress.
func reportStep(progress ports.ProgressReporter, name, status string, detail string) {
	if progress == nil {
		return
	}
	progress.ReportStep(ports.StepProgress{
		Name:   name,
		Status: status,
		Detail: detail,
	})
}

// Fork forks genesis from the specified source
func (f *GenesisForker) Fork(ctx context.Context, opts ports.ForkOptions, progress ports.ProgressReporter) (*ports.ForkResult, error) {
	f.logger.Info("forking genesis",
		"mode", opts.Source.Mode,
		"networkType", opts.Source.NetworkType,
	)

	var genesis []byte
	var err error

	switch opts.Source.Mode {
	case types.GenesisModeRPC:
		reportStep(progress, "Fetching genesis from RPC", "running", opts.Source.RPCURL)
		genesis, err = f.forkFromRPC(ctx, opts)
		if err != nil {
			reportStep(progress, "Fetching genesis from RPC", "failed", err.Error())
			return nil, fmt.Errorf("failed to fetch genesis: %w", err)
		}
		reportStep(progress, "Fetching genesis from RPC", "completed", "")
	case types.GenesisModeSnapshot:
		reportStep(progress, "Forking from snapshot", "running", opts.Source.SnapshotURL)
		genesis, err = f.forkFromSnapshot(ctx, opts, progress)
		if err != nil {
			reportStep(progress, "Forking from snapshot", "failed", err.Error())
			return nil, fmt.Errorf("failed to fetch genesis: %w", err)
		}
		reportStep(progress, "Forking from snapshot", "completed", "")
	case types.GenesisModeLocal:
		reportStep(progress, "Loading genesis from local file", "running", opts.Source.LocalPath)
		genesis, err = f.forkFromLocal(ctx, opts)
		if err != nil {
			reportStep(progress, "Loading genesis from local file", "failed", err.Error())
			return nil, fmt.Errorf("failed to fetch genesis: %w", err)
		}
		reportStep(progress, "Loading genesis from local file", "completed", "")
	default:
		return nil, fmt.Errorf("unsupported genesis mode: %s", opts.Source.Mode)
	}

	// Extract original chain ID
	sourceChainID, err := extractChainID(genesis)
	if err != nil {
		f.logger.Warn("failed to extract source chain ID", "error", err)
		sourceChainID = ""
	}

	// Check if genesis is large (>1GB) and requires file-based patching
	// gRPC has a ~2GB message size limit, so use file-based approach for safety
	const largeGenesisThreshold = 1 << 30 // 1GB
	if len(genesis) > largeGenesisThreshold {
		f.logger.Info("large genesis detected, using file-based patching",
			"size", len(genesis),
			"threshold", largeGenesisThreshold)
		return f.patchLargeGenesis(genesis, sourceChainID, opts)
	}

	// Validate the fetched genesis
	if f.config.PluginGenesis != nil {
		if err := f.config.PluginGenesis.ValidateGenesis(genesis); err != nil {
			return nil, fmt.Errorf("genesis validation failed: %w", err)
		}
	}

	// Apply patches
	reportStep(progress, "Applying genesis patches", "running", "")
	patched, err := f.applyPatches(genesis, opts.PatchOpts)
	if err != nil {
		reportStep(progress, "Applying genesis patches", "failed", err.Error())
		return nil, fmt.Errorf("failed to apply patches: %w", err)
	}

	// Apply plugin-specific patches
	if f.config.PluginGenesis != nil {
		patched, err = f.config.PluginGenesis.PatchGenesis(patched, opts.PatchOpts)
		if err != nil {
			reportStep(progress, "Applying genesis patches", "failed", err.Error())
			return nil, fmt.Errorf("plugin patch failed: %w", err)
		}
	}
	reportStep(progress, "Applying genesis patches", "completed", "")

	return &ports.ForkResult{
		Genesis:       patched,
		SourceChainID: sourceChainID,
		NewChainID:    opts.PatchOpts.ChainID,
		SourceMode:    opts.Source.Mode,
		FetchedAt:     time.Now(),
	}, nil
}

// patchLargeGenesis handles patching for genesis files that exceed gRPC message size limits.
// It writes genesis to a temporary file, uses file-based patching, and reads the result back.
func (f *GenesisForker) patchLargeGenesis(genesis []byte, sourceChainID string, opts ports.ForkOptions) (*ports.ForkResult, error) {
	// Check if plugin supports file-based patching
	fileBasedPlugin, ok := f.config.PluginGenesis.(types.FileBasedPluginGenesis)
	if !ok {
		return nil, fmt.Errorf("genesis size (%d bytes) exceeds gRPC limit but plugin does not support file-based patching", len(genesis))
	}

	// Create temporary directory for file-based operations
	workDir := filepath.Join(f.config.DataDir, "genesis-work", fmt.Sprintf("patch-%d", time.Now().UnixNano()))
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create work dir for large genesis: %w", err)
	}
	defer os.RemoveAll(workDir)

	// Write genesis to input file
	inputPath := filepath.Join(workDir, "genesis-input.json")
	if err := os.WriteFile(inputPath, genesis, 0644); err != nil {
		return nil, fmt.Errorf("failed to write genesis to temp file: %w", err)
	}

	// Apply generic patches first (chain_id, etc.) by reading and writing
	patched, err := f.applyPatches(genesis, opts.PatchOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to apply patches: %w", err)
	}

	// Write patched genesis for plugin processing
	patchedInputPath := filepath.Join(workDir, "genesis-patched.json")
	if err := os.WriteFile(patchedInputPath, patched, 0644); err != nil {
		return nil, fmt.Errorf("failed to write patched genesis: %w", err)
	}

	// Use file-based plugin patching
	outputPath := filepath.Join(workDir, "genesis-output.json")
	f.logger.Info("applying file-based plugin patches",
		"inputPath", patchedInputPath,
		"outputPath", outputPath)

	outputSize, err := fileBasedPlugin.PatchGenesisFile(patchedInputPath, outputPath, opts.PatchOpts)
	if err != nil {
		return nil, fmt.Errorf("file-based plugin patch failed: %w", err)
	}

	f.logger.Info("file-based patching complete", "outputSize", outputSize)

	// Read the result back
	result, err := os.ReadFile(outputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read patched genesis: %w", err)
	}

	return &ports.ForkResult{
		Genesis:       result,
		SourceChainID: sourceChainID,
		NewChainID:    opts.PatchOpts.ChainID,
		SourceMode:    opts.Source.Mode,
		FetchedAt:     time.Now(),
	}, nil
}

// forkFromRPC fetches genesis from an RPC endpoint
func (f *GenesisForker) forkFromRPC(ctx context.Context, opts ports.ForkOptions) ([]byte, error) {
	// Determine RPC endpoint
	rpcURL := opts.Source.RPCURL
	if rpcURL == "" && f.config.PluginGenesis != nil {
		rpcURL = f.config.PluginGenesis.GetRPCEndpoint(opts.Source.NetworkType)
	}
	if rpcURL == "" {
		return nil, fmt.Errorf("no RPC URL specified")
	}

	// Use existing infrastructure if available
	if f.config.GenesisFetcher != nil {
		return f.config.GenesisFetcher.FetchFromRPC(ctx, rpcURL)
	}

	// Fallback: direct HTTP fetch
	return f.fetchGenesisHTTP(ctx, rpcURL+"/genesis")
}

// forkFromSnapshot downloads snapshot and exports genesis
func (f *GenesisForker) forkFromSnapshot(ctx context.Context, opts ports.ForkOptions, progress ports.ProgressReporter) ([]byte, error) {
	if opts.BinaryPath == "" {
		return nil, fmt.Errorf("binary path required for snapshot export")
	}

	// Determine snapshot URL
	snapshotURL := opts.Source.SnapshotURL
	if snapshotURL == "" && f.config.PluginGenesis != nil {
		snapshotURL = f.config.PluginGenesis.GetSnapshotURL(opts.Source.NetworkType)
	}
	if snapshotURL == "" {
		return nil, fmt.Errorf("no snapshot URL specified")
	}

	// Use existing infrastructure for snapshot download/export
	if f.config.SnapshotFetcher != nil && f.config.StateExportService != nil {
		return f.forkFromSnapshotInfra(ctx, opts, snapshotURL, progress)
	}

	return nil, fmt.Errorf("snapshot forking requires SnapshotFetcher and StateExportService")
}

// forkFromSnapshotInfra uses existing infrastructure for snapshot-based forking
func (f *GenesisForker) forkFromSnapshotInfra(ctx context.Context, opts ports.ForkOptions, snapshotURL string, progress ports.ProgressReporter) ([]byte, error) {
	// Create work directory
	workDir := filepath.Join(f.config.DataDir, "genesis-work", fmt.Sprintf("fork-%d", time.Now().UnixNano()))
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create work dir: %w", err)
	}
	defer os.RemoveAll(workDir)

	// Determine cache key
	cacheKey := ""
	if f.config.PluginGenesis != nil {
		cacheKey = fmt.Sprintf("%s-%s",
			f.config.PluginGenesis.BinaryName(),
			opts.Source.NetworkType)
	}

	// Download snapshot
	reportStep(progress, "Downloading snapshot", "running", snapshotURL)
	snapshotPath, fromCache, err := f.config.SnapshotFetcher.DownloadWithCache(
		ctx, snapshotURL, cacheKey, opts.NoCache)
	if err != nil {
		reportStep(progress, "Downloading snapshot", "failed", err.Error())
		return nil, fmt.Errorf("failed to download snapshot: %w", err)
	}

	cacheDetail := ""
	if fromCache {
		cacheDetail = "from cache"
	}
	reportStep(progress, "Downloading snapshot", "completed", cacheDetail)

	f.logger.Info("snapshot downloaded",
		"path", snapshotPath,
		"fromCache", fromCache,
		"snapshotURL", snapshotURL)

	// Extract snapshot
	reportStep(progress, "Extracting snapshot", "running", "")
	if err := f.config.SnapshotFetcher.Extract(ctx, snapshotPath, workDir); err != nil {
		reportStep(progress, "Extracting snapshot", "failed", err.Error())
		return nil, fmt.Errorf("failed to extract snapshot: %w", err)
	}
	reportStep(progress, "Extracting snapshot", "completed", "")

	// First, fetch RPC genesis for chain params
	// The RPC genesis is required for the export command to read chain parameters
	rpcURL := opts.Source.RPCURL
	if rpcURL == "" && f.config.PluginGenesis != nil {
		rpcURL = f.config.PluginGenesis.GetRPCEndpoint(opts.Source.NetworkType)
	}

	if rpcURL == "" {
		return nil, fmt.Errorf("no RPC URL available for network type %q: plugin must implement GetRPCEndpoint() or provide RPCURL in source options", opts.Source.NetworkType)
	}

	if f.config.GenesisFetcher == nil {
		return nil, fmt.Errorf("genesis fetcher not configured: cannot fetch RPC genesis from %s", rpcURL)
	}

	reportStep(progress, "Fetching RPC genesis", "running", rpcURL)
	f.logger.Debug("fetching RPC genesis for chain params", "rpcURL", rpcURL)

	rpcGenesis, err := f.config.GenesisFetcher.FetchFromRPC(ctx, rpcURL)
	if err != nil {
		reportStep(progress, "Fetching RPC genesis", "failed", err.Error())
		return nil, fmt.Errorf("failed to fetch RPC genesis from %s: %w", rpcURL, err)
	}

	if len(rpcGenesis) == 0 {
		reportStep(progress, "Fetching RPC genesis", "failed", "empty response")
		return nil, fmt.Errorf("RPC genesis is empty: fetched from %s but received no data", rpcURL)
	}
	reportStep(progress, "Fetching RPC genesis", "completed", "")

	f.logger.Debug("RPC genesis fetched successfully", "size", len(rpcGenesis))

	// Export genesis from snapshot
	reportStep(progress, "Exporting state from snapshot", "running", "")
	exportOpts := ports.StateExportOptions{
		HomeDir:           workDir,
		BinaryPath:        opts.BinaryPath,
		RpcGenesis:        rpcGenesis,
		CacheKey:          cacheKey,
		SnapshotURL:       snapshotURL,
		SnapshotFromCache: fromCache,
	}

	genesis, err := f.config.StateExportService.ExportFromSnapshot(ctx, exportOpts)
	if err != nil {
		reportStep(progress, "Exporting state from snapshot", "failed", err.Error())
		return nil, fmt.Errorf("failed to export genesis from snapshot: %w", err)
	}
	reportStep(progress, "Exporting state from snapshot", "completed", "")

	return genesis, nil
}

// forkFromLocal reads genesis from a local file
func (f *GenesisForker) forkFromLocal(ctx context.Context, opts ports.ForkOptions) ([]byte, error) {
	if opts.Source.LocalPath == "" {
		return nil, fmt.Errorf("local path required for local mode")
	}

	// Validate path is absolute and clean to prevent path traversal
	cleanPath := filepath.Clean(opts.Source.LocalPath)
	if !filepath.IsAbs(cleanPath) {
		return nil, fmt.Errorf("local path must be absolute: %s", opts.Source.LocalPath)
	}

	genesis, err := os.ReadFile(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read local genesis: %w", err)
	}

	return genesis, nil
}

// httpClientTimeout is the timeout for HTTP requests to RPC endpoints
const httpClientTimeout = 30 * time.Second

// fetchGenesisHTTP fetches genesis via HTTP (fallback when no infrastructure)
func (f *GenesisForker) fetchGenesisHTTP(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	// Use client with explicit timeout to prevent hanging on slow endpoints
	client := &http.Client{
		Timeout: httpClientTimeout,
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// The RPC response wraps genesis in {"result": {"genesis": {...}}}
	var rpcResp struct {
		Result struct {
			Genesis json.RawMessage `json:"genesis"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &rpcResp); err == nil && len(rpcResp.Result.Genesis) > 0 {
		return rpcResp.Result.Genesis, nil
	}

	// Try as direct genesis
	return body, nil
}

// applyPatches applies generic patches to genesis
func (f *GenesisForker) applyPatches(genesis []byte, opts types.GenesisPatchOptions) ([]byte, error) {
	if opts.ChainID == "" && opts.VotingPeriod == 0 && opts.UnbondingTime == 0 {
		return genesis, nil
	}

	var gen map[string]interface{}
	if err := json.Unmarshal(genesis, &gen); err != nil {
		return nil, fmt.Errorf("failed to parse genesis: %w", err)
	}

	// Patch chain_id
	if opts.ChainID != "" {
		gen["chain_id"] = opts.ChainID
	}

	return json.MarshalIndent(gen, "", "  ")
}

// extractChainID extracts chain_id from genesis
func extractChainID(genesis []byte) (string, error) {
	var gen struct {
		ChainID string `json:"chain_id"`
	}
	if err := json.Unmarshal(genesis, &gen); err != nil {
		return "", err
	}
	return gen.ChainID, nil
}
