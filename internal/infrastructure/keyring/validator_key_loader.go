// Package keyring provides key management implementations.
package keyring

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/b-harvest/devnet-builder/internal/application/ports"
	"github.com/b-harvest/devnet-builder/types"
)

// ValidatorKeyLoader loads validator keys from the devnet keyring.
type ValidatorKeyLoader struct {
	dockerImage string // Default docker image for docker mode
}

// NewValidatorKeyLoader creates a new ValidatorKeyLoader.
func NewValidatorKeyLoader(dockerImage string) *ValidatorKeyLoader {
	return &ValidatorKeyLoader{
		dockerImage: dockerImage,
	}
}

// LoadValidatorKeys loads all validator keys from the devnet.
func (l *ValidatorKeyLoader) LoadValidatorKeys(ctx context.Context, opts ports.ValidatorKeyOptions) ([]ports.ValidatorKey, error) {
	accountsDir := filepath.Join(opts.HomeDir, "devnet", "accounts")
	validators := make([]ports.ValidatorKey, 0, opts.NumValidators)

	for i := 0; i < opts.NumValidators; i++ {
		key, err := l.loadValidatorKey(ctx, opts, accountsDir, i)
		if err != nil {
			return nil, fmt.Errorf("failed to load validator%d keys: %w", i, err)
		}
		validators = append(validators, *key)
	}

	return validators, nil
}

func (l *ValidatorKeyLoader) loadValidatorKey(ctx context.Context, opts ports.ValidatorKeyOptions, accountsDir string, index int) (*ports.ValidatorKey, error) {
	validatorName := fmt.Sprintf("validator%d", index)

	// Get private key
	privateKey, err := l.exportETHKey(ctx, opts, accountsDir, validatorName)
	if err != nil {
		return nil, fmt.Errorf("failed to export ETH key: %w", err)
	}

	// Get bech32 address
	bech32Addr, err := l.getBech32Address(ctx, opts, accountsDir, validatorName)
	if err != nil {
		return nil, fmt.Errorf("failed to get bech32 address: %w", err)
	}

	// Convert bech32 to hex address
	hexAddr, err := l.convertToHexAddress(ctx, opts, accountsDir, bech32Addr)
	if err != nil {
		return nil, fmt.Errorf("failed to convert to hex address: %w", err)
	}

	return &ports.ValidatorKey{
		Index:         index,
		Name:          validatorName,
		Bech32Address: bech32Addr,
		HexAddress:    hexAddr,
		PrivateKey:    privateKey,
	}, nil
}

func (l *ValidatorKeyLoader) exportETHKey(ctx context.Context, opts ports.ValidatorKeyOptions, accountsDir, validatorName string) (string, error) {
	var cmd *exec.Cmd

	if opts.ExecutionMode == types.ExecutionModeDocker {
		// Docker mode: use docker run
		image := l.getDockerImage(opts)
		cmd = exec.CommandContext(ctx, "docker", "run", "--rm", "-u", "0",
			"-v", accountsDir+":/data",
			image,
			"keys", "unsafe-export-eth-key", validatorName,
			"--keyring-backend", "test",
			"--home", "/data",
		)
	} else {
		// Local mode: use binary directly
		binaryName := opts.BinaryName
		if binaryName == "" {
			binaryName = "stabled"
		}
		cmd = exec.CommandContext(ctx, binaryName,
			"keys", "unsafe-export-eth-key", validatorName,
			"--keyring-backend", "test",
			"--home", accountsDir,
		)
	}

	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("export-eth-key failed: %s", string(exitErr.Stderr))
		}
		return "", fmt.Errorf("export-eth-key failed: %w", err)
	}

	// Private key is output directly (without 0x prefix)
	privateKey := strings.TrimSpace(string(output))
	return privateKey, nil
}

func (l *ValidatorKeyLoader) getBech32Address(ctx context.Context, opts ports.ValidatorKeyOptions, accountsDir, validatorName string) (string, error) {
	var cmd *exec.Cmd

	if opts.ExecutionMode == types.ExecutionModeDocker {
		image := l.getDockerImage(opts)
		cmd = exec.CommandContext(ctx, "docker", "run", "--rm", "-u", "0",
			"-v", accountsDir+":/data",
			image,
			"keys", "show", validatorName,
			"--keyring-backend", "test",
			"--home", "/data",
			"--output", "json",
		)
	} else {
		binaryName := opts.BinaryName
		if binaryName == "" {
			binaryName = "stabled"
		}
		cmd = exec.CommandContext(ctx, binaryName,
			"keys", "show", validatorName,
			"--keyring-backend", "test",
			"--home", accountsDir,
			"--output", "json",
		)
	}

	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("keys show failed: %s", string(exitErr.Stderr))
		}
		return "", fmt.Errorf("keys show failed: %w", err)
	}

	var keyInfo struct {
		Address string `json:"address"`
	}
	if err := json.Unmarshal(output, &keyInfo); err != nil {
		return "", fmt.Errorf("failed to parse key info: %w", err)
	}

	return keyInfo.Address, nil
}

func (l *ValidatorKeyLoader) convertToHexAddress(ctx context.Context, opts ports.ValidatorKeyOptions, accountsDir, bech32Addr string) (string, error) {
	var cmd *exec.Cmd

	if opts.ExecutionMode == types.ExecutionModeDocker {
		image := l.getDockerImage(opts)
		cmd = exec.CommandContext(ctx, "docker", "run", "--rm", "-u", "0",
			"-v", accountsDir+":/data",
			image,
			"debug", "addr", bech32Addr,
		)
	} else {
		binaryName := opts.BinaryName
		if binaryName == "" {
			binaryName = "stabled"
		}
		cmd = exec.CommandContext(ctx, binaryName,
			"debug", "addr", bech32Addr,
		)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("debug addr failed: %w", err)
	}

	// Parse output: "Address (hex): A41BEA69E25BC2DD43A329AB5EF27D7012D79377"
	hexPattern := regexp.MustCompile(`Address \(hex\):\s*([A-Fa-f0-9]+)`)
	matches := hexPattern.FindStringSubmatch(string(output))
	if len(matches) < 2 {
		return "", fmt.Errorf("failed to extract hex address from output: %s", string(output))
	}

	// Return with 0x prefix, lowercase
	return "0x" + strings.ToLower(matches[1]), nil
}

func (l *ValidatorKeyLoader) getDockerImage(opts ports.ValidatorKeyOptions) string {
	if opts.Version != "" {
		// Use the docker image with the specific version
		if l.dockerImage != "" {
			return fmt.Sprintf("%s:%s", l.dockerImage, opts.Version)
		}
		return fmt.Sprintf("ghcr.io/stablelabs/stable:%s", opts.Version)
	}
	return l.dockerImage
}

// Ensure ValidatorKeyLoader implements ports.ValidatorKeyLoader.
var _ ports.ValidatorKeyLoader = (*ValidatorKeyLoader)(nil)
