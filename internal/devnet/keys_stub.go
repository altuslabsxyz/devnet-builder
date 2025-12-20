package devnet

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ExportKeys exports validator and account keys from the devnet.
// This stub implementation reads key information from JSON files.
// For full keyring access, use a network-specific plugin.
func ExportKeys(homeDir string, keyType string) (*KeyExport, error) {
	export := &KeyExport{
		Validators: make([]ValidatorKey, 0),
		Accounts:   make([]AccountKey, 0),
	}

	// Load metadata to get node count
	metadata, err := LoadDevnetMetadata(homeDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load metadata: %w", err)
	}

	devnetDir := filepath.Join(homeDir, "devnet")

	// Export validator keys from JSON files
	if keyType == "all" || keyType == "validators" {
		for i := 0; i < metadata.NumValidators; i++ {
			key, err := readValidatorKeyFromFile(devnetDir, i)
			if err != nil {
				key = &ValidatorKey{
					Index:   i,
					Address: fmt.Sprintf("(error: %v)", err),
				}
			}
			export.Validators = append(export.Validators, *key)
		}
	}

	// Export account keys from JSON files
	if keyType == "all" || keyType == "accounts" {
		for i := 0; i < metadata.NumAccounts; i++ {
			key, err := readAccountKeyFromFile(devnetDir, i)
			if err != nil {
				key = &AccountKey{
					Index:   i,
					Address: fmt.Sprintf("(error: %v)", err),
				}
			}
			export.Accounts = append(export.Accounts, *key)
		}
	}

	return export, nil
}

// readValidatorKeyFromFile reads validator key info from JSON file.
func readValidatorKeyFromFile(devnetDir string, index int) (*ValidatorKey, error) {
	nodeDir := filepath.Join(devnetDir, fmt.Sprintf("node%d", index))
	validatorFile := filepath.Join(nodeDir, fmt.Sprintf("validator%d.json", index))

	key := &ValidatorKey{Index: index}

	// Read validator info file
	data, err := os.ReadFile(validatorFile)
	if err != nil {
		return nil, fmt.Errorf("validator file not found: %w", err)
	}

	var validatorData struct {
		Address  string `json:"address"`
		Mnemonic string `json:"mnemonic"`
	}
	if err := json.Unmarshal(data, &validatorData); err != nil {
		return nil, fmt.Errorf("invalid validator file: %w", err)
	}

	key.Address = validatorData.Address
	key.Mnemonic = validatorData.Mnemonic

	// Read consensus key from priv_validator_key.json
	privKeyPath := filepath.Join(nodeDir, "config", "priv_validator_key.json")
	if privData, err := os.ReadFile(privKeyPath); err == nil {
		var privKeyInfo struct {
			Address string `json:"address"`
			PubKey  struct {
				Value string `json:"value"`
			} `json:"pub_key"`
		}
		if err := json.Unmarshal(privData, &privKeyInfo); err == nil {
			key.ConsAddrHex = privKeyInfo.Address
			key.ConsPubKey = privKeyInfo.PubKey.Value
		}
	}

	return key, nil
}

// readAccountKeyFromFile reads account key info from JSON file.
func readAccountKeyFromFile(devnetDir string, index int) (*AccountKey, error) {
	accountsDir := filepath.Join(devnetDir, "accounts")
	accountFile := filepath.Join(accountsDir, fmt.Sprintf("account%d.json", index))

	key := &AccountKey{Index: index}

	data, err := os.ReadFile(accountFile)
	if err != nil {
		return nil, fmt.Errorf("account file not found: %w", err)
	}

	var accountData struct {
		Address  string `json:"address"`
		Mnemonic string `json:"mnemonic"`
	}
	if err := json.Unmarshal(data, &accountData); err != nil {
		return nil, fmt.Errorf("invalid account file: %w", err)
	}

	key.Address = accountData.Address
	key.Mnemonic = accountData.Mnemonic

	return key, nil
}
