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
// It tries Docker first, then local binary, and falls back to using genesis directly.
func ExportGenesisFromSnapshot(ctx context.Context, opts ExportOptions) (*GenesisMetadata, error) {
	logger := opts.Logger
	if logger == nil {
		logger = output.DefaultLogger
	}

	if opts.GenesisPath == "" {
		return nil, fmt.Errorf("GenesisPath is required for snapshot export")
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

	// Snapshot extracts to tmpDir/data/, so use tmpDir as the export home
	// and just create the config directory
	exportHome := tmpDir
	configDir := filepath.Join(exportHome, "config")
	dataDir := filepath.Join(exportHome, "data")

	// Create config directory
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	// Copy source genesis to config directory
	configGenesis := filepath.Join(configDir, "genesis.json")
	if err := copyFile(opts.GenesisPath, configGenesis); err != nil {
		return nil, fmt.Errorf("failed to copy genesis to config: %w", err)
	}
	logger.Debug("Copied genesis from %s to %s", opts.GenesisPath, configGenesis)

	// Verify data directory exists from snapshot extraction
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("snapshot data directory not found at %s", dataDir)
	}

	// Initialize priv_validator_state.json if not present
	privValState := filepath.Join(dataDir, "priv_validator_state.json")
	if _, err := os.Stat(privValState); os.IsNotExist(err) {
		if err := os.WriteFile(privValState, []byte(`{"height":"0","round":0,"step":0}`), 0644); err != nil {
			logger.Debug("Failed to create priv_validator_state.json: %v", err)
		}
	}

	// Ensure destination directory exists
	if err := os.MkdirAll(filepath.Dir(opts.DestPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create destination directory: %w", err)
	}

	exportPath := filepath.Join(tmpDir, "exported_genesis.json")
	exportSuccess := false

	// Try Docker first (only if UseDocker is true)
	if opts.UseDocker {
		if _, err := exec.LookPath("docker"); err == nil {
			logger.Debug("Trying Docker export...")
			dockerImage := opts.DockerImage
			if dockerImage == "" {
				dockerImage = "ghcr.io/stablelabs/stable:latest"
			}

			dockerArgs := []string{
				"run", "--rm",
				"-v", exportHome + ":/data",
				dockerImage,
			}
			// GHCR images have stabled as entrypoint, others need explicit command
			if !strings.HasPrefix(dockerImage, "ghcr.io/") {
				dockerArgs = append(dockerArgs, "stabled")
			}
			dockerArgs = append(dockerArgs, "export", "--home", "/data")

			cmd := exec.CommandContext(ctx, "docker", dockerArgs...)

			cmdOutput, err := cmd.Output()
			if err == nil && len(cmdOutput) > 0 {
				// Docker export outputs to stdout
				if err := os.WriteFile(exportPath, cmdOutput, 0644); err == nil {
					// Verify it's valid JSON with chain_id
					var testGenesis struct {
						ChainID string `json:"chain_id"`
					}
					if json.Unmarshal(cmdOutput, &testGenesis) == nil && testGenesis.ChainID != "" {
						exportSuccess = true
						logger.Debug("Docker export successful")
					}
				}
			} else {
				logger.Debug("Docker export failed: %v", err)
				// Print detailed error for diagnosis
				if exitErr, ok := err.(*exec.ExitError); ok {
					logger.PrintCommandError(&output.CommandErrorInfo{
						Command:  "docker",
						Args:     dockerArgs,
						WorkDir:  exportHome,
						Stderr:   string(exitErr.Stderr),
						ExitCode: exitErr.ExitCode(),
						Error:    err,
					})
				}
			}
		}
	}

	// Try local binary if Docker failed
	if !exportSuccess {
		stableBinary := opts.StableBinary
		if stableBinary == "" {
			stableBinary = "stabled"
		}

		if _, err := exec.LookPath(stableBinary); err == nil {
			logger.Debug("Trying local binary export...")
			localArgs := []string{"export", "--home", exportHome}
			cmd := exec.CommandContext(ctx, stableBinary, localArgs...)
			cmdOutput, err := cmd.Output()
			if err == nil && len(cmdOutput) > 0 {
				if err := os.WriteFile(exportPath, cmdOutput, 0644); err == nil {
					var testGenesis struct {
						ChainID string `json:"chain_id"`
					}
					if json.Unmarshal(cmdOutput, &testGenesis) == nil && testGenesis.ChainID != "" {
						exportSuccess = true
						logger.Debug("Local binary export successful")
					}
				}
			} else {
				logger.Debug("Local binary export failed: %v", err)
				// Print detailed error for diagnosis
				if exitErr, ok := err.(*exec.ExitError); ok {
					logger.PrintCommandError(&output.CommandErrorInfo{
						Command:  stableBinary,
						Args:     localArgs,
						WorkDir:  exportHome,
						Stderr:   string(exitErr.Stderr),
						ExitCode: exitErr.ExitCode(),
						Error:    err,
					})
				}
			}
		}
	}

	// Fallback: use genesis directly
	if !exportSuccess {
		logger.Warn("Export failed, using genesis directly")
		if err := copyFile(opts.GenesisPath, opts.DestPath); err != nil {
			return nil, fmt.Errorf("failed to copy genesis: %w", err)
		}
	} else {
		// Copy exported genesis to destination
		if err := copyFile(exportPath, opts.DestPath); err != nil {
			return nil, fmt.Errorf("failed to copy exported genesis: %w", err)
		}
	}

	// Extract metadata
	var genesis struct {
		ChainID       string `json:"chain_id"`
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

// copyDir copies all files from src directory to dst directory.
func copyDir(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := os.MkdirAll(dstPath, 0755); err != nil {
				return err
			}
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}
	return nil
}

// ExportOptions configures genesis export.
type ExportOptions struct {
	SnapshotPath string
	DestPath     string
	Network      string
	Decompressor string
	StableBinary string
	DockerImage  string // Docker image for stabled (e.g., ghcr.io/stablelabs/stable:latest)
	GenesisPath  string // Path to source genesis.json (required for stabled export)
	UseDocker    bool   // If true, try Docker first; if false, use local binary only
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
