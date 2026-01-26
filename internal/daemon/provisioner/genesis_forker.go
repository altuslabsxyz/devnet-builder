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

// ForkOptions specifies options for forking genesis
type ForkOptions struct {
	Source     types.GenesisSource
	PatchOpts  types.GenesisPatchOptions
	BinaryPath string // required for snapshot export
	NoCache    bool   // skip caching
}

// ForkResult contains the result of a genesis fork
type ForkResult struct {
	Genesis       []byte
	SourceChainID string
	NewChainID    string
	SourceMode    types.GenesisMode
	FetchedAt     time.Time
}

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

// Fork forks genesis from the specified source
func (f *GenesisForker) Fork(ctx context.Context, opts ForkOptions) (*ForkResult, error) {
	f.logger.Info("forking genesis",
		"mode", opts.Source.Mode,
		"networkType", opts.Source.NetworkType,
	)

	var genesis []byte
	var err error

	switch opts.Source.Mode {
	case types.GenesisModeRPC:
		genesis, err = f.forkFromRPC(ctx, opts)
	case types.GenesisModeSnapshot:
		genesis, err = f.forkFromSnapshot(ctx, opts)
	case types.GenesisModeLocal:
		genesis, err = f.forkFromLocal(ctx, opts)
	default:
		return nil, fmt.Errorf("unsupported genesis mode: %s", opts.Source.Mode)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to fetch genesis: %w", err)
	}

	// Extract original chain ID
	sourceChainID, err := extractChainID(genesis)
	if err != nil {
		f.logger.Warn("failed to extract source chain ID", "error", err)
		sourceChainID = ""
	}

	// Validate the fetched genesis
	if f.config.PluginGenesis != nil {
		if err := f.config.PluginGenesis.ValidateGenesis(genesis); err != nil {
			return nil, fmt.Errorf("genesis validation failed: %w", err)
		}
	}

	// Apply patches
	patched, err := f.applyPatches(genesis, opts.PatchOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to apply patches: %w", err)
	}

	// Apply plugin-specific patches
	if f.config.PluginGenesis != nil {
		patched, err = f.config.PluginGenesis.PatchGenesis(patched, opts.PatchOpts)
		if err != nil {
			return nil, fmt.Errorf("plugin patch failed: %w", err)
		}
	}

	return &ForkResult{
		Genesis:       patched,
		SourceChainID: sourceChainID,
		NewChainID:    opts.PatchOpts.ChainID,
		SourceMode:    opts.Source.Mode,
		FetchedAt:     time.Now(),
	}, nil
}

// forkFromRPC fetches genesis from an RPC endpoint
func (f *GenesisForker) forkFromRPC(ctx context.Context, opts ForkOptions) ([]byte, error) {
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
func (f *GenesisForker) forkFromSnapshot(ctx context.Context, opts ForkOptions) ([]byte, error) {
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
		return f.forkFromSnapshotInfra(ctx, opts, snapshotURL)
	}

	return nil, fmt.Errorf("snapshot forking requires SnapshotFetcher and StateExportService")
}

// forkFromSnapshotInfra uses existing infrastructure for snapshot-based forking
func (f *GenesisForker) forkFromSnapshotInfra(ctx context.Context, opts ForkOptions, snapshotURL string) ([]byte, error) {
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
	snapshotPath, fromCache, err := f.config.SnapshotFetcher.DownloadWithCache(
		ctx, snapshotURL, cacheKey, opts.NoCache)
	if err != nil {
		return nil, fmt.Errorf("failed to download snapshot: %w", err)
	}

	f.logger.Info("snapshot downloaded",
		"path", snapshotPath,
		"fromCache", fromCache)

	// Extract snapshot
	extractDir := filepath.Join(workDir, "data")
	if err := f.config.SnapshotFetcher.Extract(ctx, snapshotPath, extractDir); err != nil {
		return nil, fmt.Errorf("failed to extract snapshot: %w", err)
	}

	// First, fetch RPC genesis for chain params
	rpcURL := opts.Source.RPCURL
	if rpcURL == "" && f.config.PluginGenesis != nil {
		rpcURL = f.config.PluginGenesis.GetRPCEndpoint(opts.Source.NetworkType)
	}

	var rpcGenesis []byte
	if rpcURL != "" && f.config.GenesisFetcher != nil {
		rpcGenesis, _ = f.config.GenesisFetcher.FetchFromRPC(ctx, rpcURL)
	}

	// Export genesis from snapshot
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
		return nil, fmt.Errorf("failed to export genesis from snapshot: %w", err)
	}

	return genesis, nil
}

// forkFromLocal reads genesis from a local file
func (f *GenesisForker) forkFromLocal(ctx context.Context, opts ForkOptions) ([]byte, error) {
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
