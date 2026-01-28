// internal/daemon/genesis/service.go
package genesis

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/application/ports"
	"github.com/altuslabsxyz/devnet-builder/internal/infrastructure/network"
)

// =============================================================================
// Fork Options
// =============================================================================

// ForkMode specifies how to obtain genesis.
type ForkMode string

const (
	// ForkModeRPC fetches genesis directly from an RPC endpoint.
	ForkModeRPC ForkMode = "rpc"

	// ForkModeSnapshot downloads snapshot and exports genesis from state.
	ForkModeSnapshot ForkMode = "snapshot"

	// ForkModeLocal uses a local genesis file.
	ForkModeLocal ForkMode = "local"
)

// ForkOptions contains options for forking genesis.
type ForkOptions struct {
	// Mode specifies how to obtain genesis (rpc, snapshot, or local).
	Mode ForkMode

	// NetworkType specifies the network type for endpoint lookup (e.g., "mainnet", "testnet").
	// Used when RPCURL or SnapshotURL are not explicitly provided.
	NetworkType string

	// RPCURL is an optional explicit RPC URL override.
	// If empty and Mode is ForkModeRPC, uses module.RPCEndpoint(NetworkType).
	RPCURL string

	// SnapshotURL is an optional explicit snapshot URL override.
	// If empty and Mode is ForkModeSnapshot, uses module.SnapshotURL(NetworkType).
	SnapshotURL string

	// LocalPath is the path to a local genesis file (required for ForkModeLocal).
	LocalPath string

	// BinaryPath is the path to the chain binary (required for ForkModeSnapshot).
	BinaryPath string

	// ChainID is the new chain ID to set in the forked genesis.
	ChainID string

	// GenesisOptions contains additional options for genesis modification.
	GenesisOptions network.GenesisOptions

	// NoCache skips caching when true.
	NoCache bool
}

// ForkResult contains the result of a genesis fork operation.
type ForkResult struct {
	// Genesis is the forked and modified genesis JSON bytes.
	Genesis []byte

	// SourceChainID is the original chain ID from the source genesis.
	SourceChainID string

	// NewChainID is the chain ID after modification.
	NewChainID string

	// ForkMode indicates how the genesis was obtained.
	ForkMode ForkMode

	// FetchedAt is when the genesis was fetched.
	FetchedAt time.Time
}

// =============================================================================
// Genesis Service
// =============================================================================

// GenesisService handles genesis forking using NetworkModule for configuration.
// This service is GENERIC - it works with ANY network via the NetworkModule interface.
// The NetworkModule provides all chain-specific configuration (RPC endpoints, snapshot URLs,
// genesis modification logic), while this service provides the actual fetch/modify behavior.
type GenesisService struct {
	client             *http.Client
	snapshotFetcher    ports.SnapshotFetcher    // optional: for snapshot download
	stateExportService ports.StateExportService // optional: for snapshot export
	genesisFetcher     ports.GenesisFetcher     // optional: for RPC fetch
	dataDir            string
	logger             *slog.Logger
}

// GenesisServiceConfig configures the GenesisService.
type GenesisServiceConfig struct {
	// DataDir is the base directory for working files.
	DataDir string

	// SnapshotFetcher handles snapshot downloads (optional).
	SnapshotFetcher ports.SnapshotFetcher

	// StateExportService handles genesis export from snapshots (optional).
	StateExportService ports.StateExportService

	// GenesisFetcher handles RPC genesis fetch (optional).
	GenesisFetcher ports.GenesisFetcher

	// HTTPClient is an optional custom HTTP client.
	HTTPClient *http.Client

	// Logger for logging progress.
	Logger *slog.Logger
}

// NewGenesisService creates a new GenesisService.
func NewGenesisService(config GenesisServiceConfig) *GenesisService {
	logger := config.Logger
	if logger == nil {
		logger = slog.Default()
	}

	client := config.HTTPClient
	if client == nil {
		client = &http.Client{
			Timeout: 30 * time.Second,
		}
	}

	return &GenesisService{
		client:             client,
		snapshotFetcher:    config.SnapshotFetcher,
		stateExportService: config.StateExportService,
		genesisFetcher:     config.GenesisFetcher,
		dataDir:            config.DataDir,
		logger:             logger,
	}
}

// Fork forks genesis using configuration from the NetworkModule.
// The module provides:
//   - RPCEndpoint(networkType) - RPC endpoint for genesis fetch
//   - SnapshotURL(networkType) - snapshot URL for state export
//   - ModifyGenesis(genesis, opts) - apply network-specific modifications
//
// Returns the forked genesis bytes, or an error.
func (s *GenesisService) Fork(ctx context.Context, module network.NetworkModule, opts ForkOptions) (*ForkResult, error) {
	s.logger.Info("forking genesis",
		"network", module.Name(),
		"mode", opts.Mode,
		"networkType", opts.NetworkType,
	)

	var genesis []byte
	var err error

	switch opts.Mode {
	case ForkModeRPC:
		genesis, err = s.forkFromRPC(ctx, module, opts)
	case ForkModeSnapshot:
		genesis, err = s.forkFromSnapshot(ctx, module, opts)
	case ForkModeLocal:
		genesis, err = s.forkFromLocal(ctx, opts)
	default:
		return nil, fmt.Errorf("unsupported fork mode: %s", opts.Mode)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to fetch genesis: %w", err)
	}

	// Extract original chain ID
	sourceChainID, err := extractChainID(genesis)
	if err != nil {
		s.logger.Warn("failed to extract source chain ID", "error", err)
		sourceChainID = ""
	}

	// Apply genesis modifications via the module
	modifyOpts := opts.GenesisOptions
	if opts.ChainID != "" {
		modifyOpts.ChainID = opts.ChainID
	}

	modified, err := module.ModifyGenesis(genesis, modifyOpts)
	if err != nil {
		return nil, fmt.Errorf("genesis modification failed: %w", err)
	}

	// Determine new chain ID
	newChainID := opts.ChainID
	if newChainID == "" {
		newChainID, _ = extractChainID(modified)
	}

	return &ForkResult{
		Genesis:       modified,
		SourceChainID: sourceChainID,
		NewChainID:    newChainID,
		ForkMode:      opts.Mode,
		FetchedAt:     time.Now(),
	}, nil
}

// =============================================================================
// Internal Methods - RPC Mode
// =============================================================================

// forkFromRPC fetches genesis from an RPC endpoint.
func (s *GenesisService) forkFromRPC(ctx context.Context, module network.NetworkModule, opts ForkOptions) ([]byte, error) {
	// Determine RPC endpoint
	rpcURL := opts.RPCURL
	if rpcURL == "" {
		rpcURL = module.RPCEndpoint(opts.NetworkType)
	}
	if rpcURL == "" {
		return nil, fmt.Errorf("no RPC URL specified and module has no endpoint for network type %q", opts.NetworkType)
	}

	s.logger.Debug("fetching genesis from RPC", "url", rpcURL)

	// Use existing infrastructure if available
	if s.genesisFetcher != nil {
		return s.genesisFetcher.FetchFromRPC(ctx, rpcURL)
	}

	// Fallback: direct HTTP fetch
	return s.fetchGenesisHTTP(ctx, rpcURL+"/genesis")
}

// fetchGenesisHTTP fetches genesis via HTTP (fallback when no infrastructure).
func (s *GenesisService) fetchGenesisHTTP(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
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

// =============================================================================
// Internal Methods - Snapshot Mode
// =============================================================================

// forkFromSnapshot downloads snapshot and exports genesis.
func (s *GenesisService) forkFromSnapshot(ctx context.Context, module network.NetworkModule, opts ForkOptions) ([]byte, error) {
	if opts.BinaryPath == "" {
		return nil, fmt.Errorf("binary path required for snapshot export")
	}

	// Determine snapshot URL
	snapshotURL := opts.SnapshotURL
	if snapshotURL == "" {
		snapshotURL = module.SnapshotURL(opts.NetworkType)
	}
	if snapshotURL == "" {
		return nil, fmt.Errorf("no snapshot URL specified and module has no URL for network type %q", opts.NetworkType)
	}

	s.logger.Debug("forking from snapshot", "url", snapshotURL)

	// Use existing infrastructure for snapshot download/export
	if s.snapshotFetcher != nil && s.stateExportService != nil {
		return s.forkFromSnapshotInfra(ctx, module, opts, snapshotURL)
	}

	return nil, fmt.Errorf("snapshot forking requires SnapshotFetcher and StateExportService")
}

// forkFromSnapshotInfra uses existing infrastructure for snapshot-based forking.
func (s *GenesisService) forkFromSnapshotInfra(ctx context.Context, module network.NetworkModule, opts ForkOptions, snapshotURL string) ([]byte, error) {
	// Create work directory
	workDir := filepath.Join(s.dataDir, "genesis-work", fmt.Sprintf("fork-%d", time.Now().UnixNano()))
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create work dir: %w", err)
	}
	defer os.RemoveAll(workDir)

	// Determine cache key
	cacheKey := fmt.Sprintf("%s-%s", module.Name(), opts.NetworkType)

	// Download snapshot
	snapshotPath, fromCache, err := s.snapshotFetcher.DownloadWithCache(ctx, snapshotURL, cacheKey, opts.NoCache)
	if err != nil {
		return nil, fmt.Errorf("failed to download snapshot: %w", err)
	}

	s.logger.Info("snapshot downloaded",
		"path", snapshotPath,
		"fromCache", fromCache,
		"snapshotURL", snapshotURL)

	// Extract snapshot
	if err := s.snapshotFetcher.Extract(ctx, snapshotPath, workDir); err != nil {
		return nil, fmt.Errorf("failed to extract snapshot: %w", err)
	}

	// Fetch RPC genesis for chain params
	// The RPC genesis is required for the export command to read chain parameters
	rpcURL := opts.RPCURL
	if rpcURL == "" {
		rpcURL = module.RPCEndpoint(opts.NetworkType)
	}

	if rpcURL == "" {
		return nil, fmt.Errorf("no RPC URL available for network type %q: plugin must implement RPCEndpoint() or provide RPCURL in options", opts.NetworkType)
	}

	if s.genesisFetcher == nil {
		return nil, fmt.Errorf("genesis fetcher not configured: cannot fetch RPC genesis from %s", rpcURL)
	}

	s.logger.Debug("fetching RPC genesis for chain params", "rpcURL", rpcURL)

	rpcGenesis, err := s.genesisFetcher.FetchFromRPC(ctx, rpcURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch RPC genesis from %s: %w", rpcURL, err)
	}

	if len(rpcGenesis) == 0 {
		return nil, fmt.Errorf("RPC genesis is empty: fetched from %s but received no data", rpcURL)
	}

	s.logger.Debug("RPC genesis fetched successfully", "size", len(rpcGenesis))

	// Export genesis from snapshot
	exportOpts := ports.StateExportOptions{
		HomeDir:           workDir,
		BinaryPath:        opts.BinaryPath,
		RpcGenesis:        rpcGenesis,
		CacheKey:          cacheKey,
		SnapshotURL:       snapshotURL,
		SnapshotFromCache: fromCache,
	}

	genesis, err := s.stateExportService.ExportFromSnapshot(ctx, exportOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to export genesis from snapshot: %w", err)
	}

	return genesis, nil
}

// =============================================================================
// Internal Methods - Local Mode
// =============================================================================

// forkFromLocal reads genesis from a local file.
func (s *GenesisService) forkFromLocal(ctx context.Context, opts ForkOptions) ([]byte, error) {
	if opts.LocalPath == "" {
		return nil, fmt.Errorf("local path required for local mode")
	}

	// Validate path is absolute and clean to prevent path traversal
	cleanPath := filepath.Clean(opts.LocalPath)
	if !filepath.IsAbs(cleanPath) {
		return nil, fmt.Errorf("local path must be absolute: %s", opts.LocalPath)
	}

	s.logger.Debug("reading local genesis", "path", cleanPath)

	genesis, err := os.ReadFile(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read local genesis: %w", err)
	}

	return genesis, nil
}

// =============================================================================
// Additional Methods
// =============================================================================

// ExportGenesis exports genesis from a running node using the module's export command.
func (s *GenesisService) ExportGenesis(ctx context.Context, module network.NetworkModule, homeDir, binaryPath string) ([]byte, error) {
	s.logger.Info("exporting genesis",
		"network", module.Name(),
		"homeDir", homeDir,
	)

	// Get export command from module
	args := module.ExportCommand(homeDir)

	// Run export command
	cmd := exec.CommandContext(ctx, binaryPath, args...)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("export command failed: %w", err)
	}

	return output, nil
}

// =============================================================================
// Utility Functions
// =============================================================================

// extractChainID extracts chain_id from genesis JSON.
func extractChainID(genesis []byte) (string, error) {
	var gen struct {
		ChainID string `json:"chain_id"`
	}
	if err := json.Unmarshal(genesis, &gen); err != nil {
		return "", err
	}
	return gen.ChainID, nil
}
