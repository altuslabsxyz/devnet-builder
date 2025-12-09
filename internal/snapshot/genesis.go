package snapshot

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/stablelabs/stable-devnet/internal/output"
)

// GenesisMetadata contains metadata about an exported genesis.
type GenesisMetadata struct {
	// Source
	Network         string `json:"network"`
	OriginalChainID string `json:"original_chain_id"`

	// Export Info
	ExportHeight int64     `json:"export_height"`
	ExportedAt   time.Time `json:"exported_at"`

	// File Info
	FilePath  string `json:"file_path"`
	SizeBytes int64  `json:"size_bytes"`

	// Devnet Modifications
	NewChainID   string   `json:"new_chain_id"`
	ValidatorSet []string `json:"validator_set"` // Validator addresses
}

// FetchGenesisFromRPC fetches genesis from an RPC endpoint.
func FetchGenesisFromRPC(ctx context.Context, rpcEndpoint, destPath string, logger *output.Logger) (*GenesisMetadata, error) {
	if logger == nil {
		logger = output.DefaultLogger
	}

	// Construct genesis endpoint URL
	genesisURL := strings.TrimSuffix(rpcEndpoint, "/") + "/genesis"

	logger.Debug("Fetching genesis from %s", genesisURL)

	// Create HTTP client
	client := &http.Client{
		Timeout: 5 * time.Minute,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, genesisURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch genesis: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch genesis: status %d", resp.StatusCode)
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read genesis response: %w", err)
	}

	// Parse the RPC response
	var rpcResponse struct {
		Result struct {
			Genesis json.RawMessage `json:"genesis"`
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &rpcResponse); err != nil {
		return nil, fmt.Errorf("failed to parse RPC response: %w", err)
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	// Write genesis file
	if err := os.WriteFile(destPath, rpcResponse.Result.Genesis, 0644); err != nil {
		return nil, fmt.Errorf("failed to write genesis file: %w", err)
	}

	// Extract chain_id from genesis
	var genesis struct {
		ChainID string `json:"chain_id"`
	}
	if err := json.Unmarshal(rpcResponse.Result.Genesis, &genesis); err != nil {
		logger.Warn("Failed to parse chain_id from genesis: %v", err)
	}

	// Get file size
	info, err := os.Stat(destPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat genesis file: %w", err)
	}

	metadata := &GenesisMetadata{
		OriginalChainID: genesis.ChainID,
		ExportedAt:      time.Now(),
		FilePath:        destPath,
		SizeBytes:       info.Size(),
	}

	return metadata, nil
}

// ExportGenesisFromSnapshot exports genesis state from a snapshot using stabled.
func ExportGenesisFromSnapshot(ctx context.Context, opts ExportOptions) (*GenesisMetadata, error) {
	logger := opts.Logger
	if logger == nil {
		logger = output.DefaultLogger
	}

	// Check if stabled binary exists
	stableBinary := opts.StableBinary
	if stableBinary == "" {
		stableBinary = "stabled"
	}

	if _, err := exec.LookPath(stableBinary); err != nil {
		return nil, fmt.Errorf("stabled binary not found: %w", err)
	}

	// Create temporary directory for extraction
	tmpDir, err := os.MkdirTemp("", "devnet-genesis-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Extract snapshot
	logger.Debug("Extracting snapshot to %s", tmpDir)
	if err := extractSnapshot(ctx, opts.SnapshotPath, tmpDir, opts.Decompressor); err != nil {
		return nil, fmt.Errorf("failed to extract snapshot: %w", err)
	}

	// Find the data directory
	dataDir := filepath.Join(tmpDir, "data")
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		// Try without data subdirectory
		dataDir = tmpDir
	}

	// Export genesis using stabled export command
	exportPath := filepath.Join(tmpDir, "exported_genesis.json")
	cmd := exec.CommandContext(ctx, stableBinary, "export",
		"--home", tmpDir,
		"--output-document", exportPath,
	)
	cmd.Env = append(os.Environ(), "HOME="+tmpDir)

	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Debug("Export output: %s", string(output))
		return nil, fmt.Errorf("failed to export genesis: %w", err)
	}

	// Ensure destination directory exists
	if err := os.MkdirAll(filepath.Dir(opts.DestPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Copy exported genesis to destination
	if err := copyFile(exportPath, opts.DestPath); err != nil {
		return nil, fmt.Errorf("failed to copy genesis: %w", err)
	}

	// Extract metadata
	var genesis struct {
		ChainID      string `json:"chain_id"`
		InitialHeight string `json:"initial_height"`
	}
	genesisData, err := os.ReadFile(opts.DestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read genesis: %w", err)
	}
	if err := json.Unmarshal(genesisData, &genesis); err != nil {
		logger.Warn("Failed to parse genesis metadata: %v", err)
	}

	info, _ := os.Stat(opts.DestPath)

	metadata := &GenesisMetadata{
		Network:         opts.Network,
		OriginalChainID: genesis.ChainID,
		ExportedAt:      time.Now(),
		FilePath:        opts.DestPath,
		SizeBytes:       info.Size(),
	}

	return metadata, nil
}

// ExportOptions configures genesis export.
type ExportOptions struct {
	SnapshotPath string
	DestPath     string
	Network      string
	Decompressor string
	StableBinary string
	Logger       *output.Logger
}

// extractSnapshot extracts a compressed snapshot archive.
func extractSnapshot(ctx context.Context, snapshotPath, destDir, decompressor string) error {
	var cmd *exec.Cmd

	switch decompressor {
	case "zstd":
		// zstd -d -c snapshot.tar.zst | tar xf - -C destDir
		cmd = exec.CommandContext(ctx, "bash", "-c",
			fmt.Sprintf("zstd -d -c %q | tar xf - -C %q", snapshotPath, destDir))
	case "lz4":
		// lz4 -d -c snapshot.tar.lz4 | tar xf - -C destDir
		cmd = exec.CommandContext(ctx, "bash", "-c",
			fmt.Sprintf("lz4 -d -c %q | tar xf - -C %q", snapshotPath, destDir))
	case "gzip":
		cmd = exec.CommandContext(ctx, "tar", "xzf", snapshotPath, "-C", destDir)
	case "none":
		cmd = exec.CommandContext(ctx, "tar", "xf", snapshotPath, "-C", destDir)
	default:
		return fmt.Errorf("unknown decompressor: %s", decompressor)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("extraction failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// copyFile copies a file from src to dst.
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// ModifyGenesis modifies genesis for devnet use (changes chain_id, validators, etc.)
func ModifyGenesis(genesisPath, newChainID string, numValidators int) error {
	// Read genesis
	data, err := os.ReadFile(genesisPath)
	if err != nil {
		return fmt.Errorf("failed to read genesis: %w", err)
	}

	// Parse as generic JSON
	var genesis map[string]interface{}
	if err := json.Unmarshal(data, &genesis); err != nil {
		return fmt.Errorf("failed to parse genesis: %w", err)
	}

	// Modify chain_id
	genesis["chain_id"] = newChainID

	// Set initial_height to 1
	genesis["initial_height"] = "1"

	// Write back
	modifiedData, err := json.MarshalIndent(genesis, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal genesis: %w", err)
	}

	if err := os.WriteFile(genesisPath, modifiedData, 0644); err != nil {
		return fmt.Errorf("failed to write genesis: %w", err)
	}

	return nil
}

// SaveGenesisMetadata saves genesis metadata to a JSON file.
func (m *GenesisMetadata) Save(homeDir string) error {
	metaPath := filepath.Join(homeDir, "devnet", "genesis.meta.json")

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	if err := os.WriteFile(metaPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write metadata: %w", err)
	}

	return nil
}
