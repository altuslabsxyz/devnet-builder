// Package genesis provides genesis fetching and export implementations.
package genesis

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/b-harvest/devnet-builder/internal/application/ports"
	legacysnapshot "github.com/b-harvest/devnet-builder/internal/snapshot"
	"github.com/b-harvest/devnet-builder/internal/output"
)

// FetcherAdapter adapts the legacy snapshot genesis functions to ports.GenesisFetcher.
type FetcherAdapter struct {
	homeDir      string
	binaryPath   string
	dockerImage  string
	useDocker    bool
	logger       *output.Logger
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
	cmd := exec.CommandContext(ctx, f.binaryPath, "export", "--home", homeDir)
	return cmd.Output()
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

	cmd := exec.CommandContext(ctx, "docker", args...)
	return cmd.Output()
}

// FetchFromRPC fetches genesis from an RPC endpoint.
func (f *FetcherAdapter) FetchFromRPC(ctx context.Context, endpoint string) ([]byte, error) {
	destPath := fmt.Sprintf("%s/tmp/genesis-%d.json", f.homeDir, time.Now().UnixNano())

	_, err := legacysnapshot.FetchGenesisFromRPC(ctx, endpoint, destPath, f.logger)
	if err != nil {
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
