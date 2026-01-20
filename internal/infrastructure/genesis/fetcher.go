// Package genesis provides genesis fetching and export implementations.
package genesis

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

	"github.com/altuslabsxyz/devnet-builder/internal/application/ports"
	"github.com/altuslabsxyz/devnet-builder/internal/output"
)

// FetcherAdapter implements ports.GenesisFetcher.
type FetcherAdapter struct {
	homeDir     string
	binaryPath  string
	dockerImage string
	useDocker   bool
	logger      *output.Logger
}

// NewFetcherAdapter creates a new FetcherAdapter.
func NewFetcherAdapter(homeDir, binaryPath, dockerImage string, useDocker bool, logger *output.Logger) *FetcherAdapter {
	if logger == nil {
		logger = output.DefaultLogger
	}
	return &FetcherAdapter{
		homeDir:     homeDir,
		binaryPath:  binaryPath,
		dockerImage: dockerImage,
		useDocker:   useDocker,
		logger:      logger,
	}
}

// ExportFromChain exports genesis from a running chain.
func (f *FetcherAdapter) ExportFromChain(ctx context.Context, nodeHomeDir string) ([]byte, error) {
	var exportOutput []byte
	var err error

	if f.useDocker && f.dockerImage != "" {
		exportOutput, err = f.exportFromDocker(ctx, nodeHomeDir)
	} else if f.binaryPath != "" {
		exportOutput, err = f.exportFromBinary(ctx, nodeHomeDir)
	} else {
		return nil, &GenesisError{
			Operation: "export",
			Message:   "no binary or docker image configured",
		}
	}

	if err != nil {
		return nil, &GenesisError{
			Operation: "export",
			Message:   err.Error(),
		}
	}

	return exportOutput, nil
}

func (f *FetcherAdapter) exportFromBinary(ctx context.Context, homeDir string) ([]byte, error) {
	f.logger.Debug("Exporting genesis using binary: %s export --home %s", f.binaryPath, homeDir)

	cmd := exec.CommandContext(ctx, f.binaryPath, "export", "--home", homeDir)

	// Capture both stdout and stderr
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Include the actual error output in the error message
		return nil, fmt.Errorf("genesis export failed: %w\nOutput: %s", err, string(output))
	}

	// The export command writes genesis to stdout
	// But CombinedOutput includes stderr too, which might have warnings
	// We need to extract just the JSON part
	return extractGenesisJSON(output)
}

// extractGenesisJSON extracts valid JSON from command output.
// The export command might include warnings/logs before the actual JSON.
func extractGenesisJSON(output []byte) ([]byte, error) {
	// Find the start of JSON (first '{')
	jsonStart := -1
	for i, b := range output {
		if b == '{' {
			jsonStart = i
			break
		}
	}

	if jsonStart == -1 {
		return nil, fmt.Errorf("no JSON found in export output: %s", string(output))
	}

	// Find the matching closing brace
	jsonData := output[jsonStart:]

	// Validate it's valid JSON
	var js json.RawMessage
	if err := json.Unmarshal(jsonData, &js); err != nil {
		return nil, fmt.Errorf("invalid JSON in export output: %w", err)
	}

	return jsonData, nil
}

func (f *FetcherAdapter) exportFromDocker(ctx context.Context, homeDir string) ([]byte, error) {
	uid := os.Getuid()
	gid := os.Getgid()

	args := []string{
		"run", "--rm",
		"--user", fmt.Sprintf("%d:%d", uid, gid),
		"-e", "HOME=/data",
		"-v", homeDir + ":/data",
		f.dockerImage,
		"export", "--home", "/data",
	}

	f.logger.Debug("Exporting genesis using docker: docker %v", args)

	cmd := exec.CommandContext(ctx, "docker", args...)

	// Capture both stdout and stderr
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("genesis export failed: %w\nOutput: %s", err, string(output))
	}

	return extractGenesisJSON(output)
}

// FetchFromRPC fetches genesis from an RPC endpoint.
func (f *FetcherAdapter) FetchFromRPC(ctx context.Context, endpoint string) ([]byte, error) {
	destPath := filepath.Join(f.homeDir, "tmp", fmt.Sprintf("genesis-%d.json", time.Now().UnixNano()))

	// Ensure tmp directory exists
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return nil, &GenesisError{
			Operation: "fetch_rpc",
			Message:   fmt.Sprintf("failed to create tmp directory: %v", err),
		}
	}

	// Fetch genesis from RPC
	if err := f.fetchGenesisFromRPC(ctx, endpoint, destPath); err != nil {
		return nil, &GenesisError{
			Operation: "fetch_rpc",
			Message:   err.Error(),
		}
	}

	data, err := os.ReadFile(destPath)
	if err != nil {
		return nil, &GenesisError{
			Operation: "fetch_rpc",
			Message:   fmt.Sprintf("failed to read fetched genesis: %v", err),
		}
	}

	// Clean up temp file
	os.Remove(destPath)

	return data, nil
}

// fetchGenesisFromRPC fetches genesis from an RPC endpoint and saves to destPath.
func (f *FetcherAdapter) fetchGenesisFromRPC(ctx context.Context, rpcEndpoint, destPath string) error {
	// Construct genesis endpoint URL
	genesisURL := strings.TrimSuffix(rpcEndpoint, "/") + "/genesis"

	f.logger.Debug("Fetching genesis from %s", genesisURL)

	// Create HTTP client
	client := &http.Client{
		Timeout: 5 * time.Minute,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, genesisURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch genesis: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to fetch genesis: status %d", resp.StatusCode)
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read genesis response: %w", err)
	}

	// Parse the RPC response
	var rpcResponse struct {
		Result struct {
			Genesis json.RawMessage `json:"genesis"`
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &rpcResponse); err != nil {
		return fmt.Errorf("failed to parse RPC response: %w", err)
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write genesis file
	if err := os.WriteFile(destPath, rpcResponse.Result.Genesis, 0644); err != nil {
		return fmt.Errorf("failed to write genesis file: %w", err)
	}

	return nil
}

// ModifyGenesis applies modifications to a genesis file.
func (f *FetcherAdapter) ModifyGenesis(genesis []byte, opts ports.GenesisModifyOptions) ([]byte, error) {
	// Parse genesis
	var genesisMap map[string]interface{}
	if err := json.Unmarshal(genesis, &genesisMap); err != nil {
		return nil, &GenesisError{
			Operation: "modify",
			Message:   fmt.Sprintf("failed to parse genesis: %v", err),
		}
	}

	// Modify chain_id
	if opts.ChainID != "" {
		genesisMap["chain_id"] = opts.ChainID
	}

	// Reset initial_height to 1
	genesisMap["initial_height"] = "1"

	// Modify app_state for governance parameters
	if appState, ok := genesisMap["app_state"].(map[string]interface{}); ok {
		// Modify voting period
		if opts.VotingPeriod > 0 {
			if gov, ok := appState["gov"].(map[string]interface{}); ok {
				if params, ok := gov["params"].(map[string]interface{}); ok {
					params["voting_period"] = fmt.Sprintf("%dns", opts.VotingPeriod.Nanoseconds())
				}
			}
		}

		// Modify unbonding time
		if opts.UnbondingTime > 0 {
			if staking, ok := appState["staking"].(map[string]interface{}); ok {
				if params, ok := staking["params"].(map[string]interface{}); ok {
					params["unbonding_time"] = fmt.Sprintf("%dns", opts.UnbondingTime.Nanoseconds())
				}
			}
		}

		// Modify inflation rate
		if opts.InflationRate != "" {
			if mint, ok := appState["mint"].(map[string]interface{}); ok {
				if minter, ok := mint["minter"].(map[string]interface{}); ok {
					minter["inflation"] = opts.InflationRate
				}
			}
		}
	}

	// Marshal back
	modifiedGenesis, err := json.MarshalIndent(genesisMap, "", "  ")
	if err != nil {
		return nil, &GenesisError{
			Operation: "modify",
			Message:   fmt.Sprintf("failed to marshal genesis: %v", err),
		}
	}

	return modifiedGenesis, nil
}

// Ensure FetcherAdapter implements GenesisFetcher.
var _ ports.GenesisFetcher = (*FetcherAdapter)(nil)
