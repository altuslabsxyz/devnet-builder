package upgrade

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/b-harvest/devnet-builder/internal/devnet"
	"github.com/b-harvest/devnet-builder/internal/output"
)

// LoadValidatorKeys exports ETH keys from the devnet keyring for all validators.
func LoadValidatorKeys(ctx context.Context, metadata *devnet.DevnetMetadata, logger *output.Logger) ([]ValidatorAccount, error) {
	accountsDir := filepath.Join(metadata.HomeDir, "devnet", "accounts")
	validators := make([]ValidatorAccount, 0, metadata.NumValidators)

	for i := 0; i < metadata.NumValidators; i++ {
		account, err := loadValidatorKey(ctx, metadata, accountsDir, i, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to load validator%d keys: %w", i, err)
		}
		validators = append(validators, *account)
	}

	return validators, nil
}

func loadValidatorKey(ctx context.Context, metadata *devnet.DevnetMetadata, accountsDir string, index int, logger *output.Logger) (*ValidatorAccount, error) {
	validatorName := fmt.Sprintf("validator%d", index)

	// Get private key
	privateKey, err := exportETHKey(ctx, metadata, accountsDir, validatorName, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to export ETH key: %w", err)
	}

	// Get bech32 address
	bech32Addr, err := getBech32Address(ctx, metadata, accountsDir, validatorName, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to get bech32 address: %w", err)
	}

	// Convert bech32 to hex address
	hexAddr, err := convertToHexAddress(ctx, metadata, accountsDir, bech32Addr, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to convert to hex address: %w", err)
	}

	return &ValidatorAccount{
		Index:         index,
		Name:          validatorName,
		Bech32Address: bech32Addr,
		HexAddress:    hexAddr,
		PrivateKey:    privateKey,
	}, nil
}

func exportETHKey(ctx context.Context, metadata *devnet.DevnetMetadata, accountsDir, validatorName string, logger *output.Logger) (string, error) {
	var cmd *exec.Cmd

	if metadata.ExecutionMode == devnet.ModeDocker {
		// Docker mode: use docker run
		image := fmt.Sprintf("ghcr.io/stablelabs/stable:%s", metadata.StableVersion)
		cmd = exec.CommandContext(ctx, "docker", "run", "--rm", "-u", "0",
			"-v", accountsDir+":/data",
			image,
			"keys", "unsafe-export-eth-key", validatorName,
			"--keyring-backend", "test",
			"--home", "/data",
		)
	} else {
		// Local mode: use stabled directly
		cmd = exec.CommandContext(ctx, "stabled",
			"keys", "unsafe-export-eth-key", validatorName,
			"--keyring-backend", "test",
			"--home", accountsDir,
		)
	}

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("export-eth-key failed: %w", err)
	}

	// Private key is output directly (without 0x prefix)
	privateKey := strings.TrimSpace(string(output))
	return privateKey, nil
}

func getBech32Address(ctx context.Context, metadata *devnet.DevnetMetadata, accountsDir, validatorName string, logger *output.Logger) (string, error) {
	var cmd *exec.Cmd

	if metadata.ExecutionMode == devnet.ModeDocker {
		image := fmt.Sprintf("ghcr.io/stablelabs/stable:%s", metadata.StableVersion)
		cmd = exec.CommandContext(ctx, "docker", "run", "--rm", "-u", "0",
			"-v", accountsDir+":/data",
			image,
			"keys", "show", validatorName,
			"--keyring-backend", "test",
			"--home", "/data",
			"--output", "json",
		)
	} else {
		cmd = exec.CommandContext(ctx, "stabled",
			"keys", "show", validatorName,
			"--keyring-backend", "test",
			"--home", accountsDir,
			"--output", "json",
		)
	}

	output, err := cmd.Output()
	if err != nil {
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

func convertToHexAddress(ctx context.Context, metadata *devnet.DevnetMetadata, accountsDir, bech32Addr string, logger *output.Logger) (string, error) {
	var cmd *exec.Cmd

	if metadata.ExecutionMode == devnet.ModeDocker {
		image := fmt.Sprintf("ghcr.io/stablelabs/stable:%s", metadata.StableVersion)
		cmd = exec.CommandContext(ctx, "docker", "run", "--rm", "-u", "0",
			"-v", accountsDir+":/data",
			image,
			"debug", "addr", bech32Addr,
		)
	} else {
		cmd = exec.CommandContext(ctx, "stabled",
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
