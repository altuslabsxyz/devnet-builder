package devnet

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ValidatorKey represents a validator's key information.
type ValidatorKey struct {
	Index    int    `json:"index"`
	Address  string `json:"address"`
	PubKey   string `json:"pubkey"`
	Mnemonic string `json:"mnemonic,omitempty"`
}

// AccountKey represents an account's key information.
type AccountKey struct {
	Index    int    `json:"index"`
	Address  string `json:"address"`
	Mnemonic string `json:"mnemonic,omitempty"`
}

// KeyExport contains all exported keys.
type KeyExport struct {
	Validators []ValidatorKey `json:"validators"`
	Accounts   []AccountKey   `json:"accounts"`
}

// ExportKeys exports validator and account keys from the devnet.
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

	// Export validator keys
	if keyType == "all" || keyType == "validators" {
		for i := 0; i < metadata.NumValidators; i++ {
			key, err := extractValidatorKey(homeDir, i)
			if err != nil {
				// Use placeholder if extraction fails
				key = &ValidatorKey{
					Index:   i,
					Address: fmt.Sprintf("stablevaloper1validator%d...", i),
					PubKey:  "N/A",
				}
			}
			export.Validators = append(export.Validators, *key)
		}
	}

	// Export account keys
	if keyType == "all" || keyType == "accounts" {
		for i := 0; i < metadata.NumAccounts; i++ {
			key, err := extractAccountKey(homeDir, i)
			if err != nil {
				// Use placeholder if extraction fails
				key = &AccountKey{
					Index:   i,
					Address: fmt.Sprintf("stable1account%d...", i),
				}
			}
			export.Accounts = append(export.Accounts, *key)
		}
	}

	return export, nil
}

// extractValidatorKey extracts key information for a validator.
func extractValidatorKey(homeDir string, index int) (*ValidatorKey, error) {
	nodeDir := filepath.Join(homeDir, "devnet", fmt.Sprintf("node%d", index))
	keyringDir := filepath.Join(nodeDir, "keyring-test")

	key := &ValidatorKey{
		Index: index,
	}

	// Try to read validator info from keyring
	// The keyring-test directory contains .info files
	infoFiles, err := filepath.Glob(filepath.Join(keyringDir, "*.info"))
	if err != nil || len(infoFiles) == 0 {
		// Try to read from priv_validator_key.json
		privKeyPath := filepath.Join(nodeDir, "config", "priv_validator_key.json")
		if data, err := os.ReadFile(privKeyPath); err == nil {
			var privKey struct {
				Address string `json:"address"`
				PubKey  struct {
					Type  string `json:"type"`
					Value string `json:"value"`
				} `json:"pub_key"`
			}
			if err := json.Unmarshal(data, &privKey); err == nil {
				key.Address = privKey.Address
				key.PubKey = privKey.PubKey.Value
			}
		}
	}

	// Try to read mnemonic from a stored file (if available)
	mnemonicPath := filepath.Join(nodeDir, "mnemonic.txt")
	if data, err := os.ReadFile(mnemonicPath); err == nil {
		key.Mnemonic = string(data)
	}

	return key, nil
}

// extractAccountKey extracts key information for an account.
func extractAccountKey(homeDir string, index int) (*AccountKey, error) {
	accountsDir := filepath.Join(homeDir, "devnet", "accounts")

	key := &AccountKey{
		Index: index,
	}

	// Try to read from account info file
	accountFile := filepath.Join(accountsDir, fmt.Sprintf("account%d.json", index))
	if data, err := os.ReadFile(accountFile); err == nil {
		var account struct {
			Address  string `json:"address"`
			Mnemonic string `json:"mnemonic"`
		}
		if err := json.Unmarshal(data, &account); err == nil {
			key.Address = account.Address
			key.Mnemonic = account.Mnemonic
		}
	}

	return key, nil
}

// FormatKeysText formats keys for text output.
func FormatKeysText(export *KeyExport) string {
	var output string

	if len(export.Validators) > 0 {
		output += "Validator Keys\n"
		output += "──────────────\n"
		for _, v := range export.Validators {
			output += fmt.Sprintf("Node %d:\n", v.Index)
			output += fmt.Sprintf("  Address:    %s\n", v.Address)
			if v.PubKey != "" {
				output += fmt.Sprintf("  Pubkey:     %s\n", v.PubKey)
			}
			if v.Mnemonic != "" {
				output += fmt.Sprintf("  Mnemonic:   %s\n", v.Mnemonic)
			}
			output += "\n"
		}
	}

	if len(export.Accounts) > 0 {
		output += "Account Keys\n"
		output += "────────────\n"
		for _, a := range export.Accounts {
			output += fmt.Sprintf("Account %d:\n", a.Index)
			output += fmt.Sprintf("  Address:    %s\n", a.Address)
			if a.Mnemonic != "" {
				output += fmt.Sprintf("  Mnemonic:   %s\n", a.Mnemonic)
			}
			output += "\n"
		}
	}

	return output
}

// FormatKeysEnv formats keys for environment variable export.
func FormatKeysEnv(export *KeyExport) string {
	var output string

	for _, v := range export.Validators {
		output += fmt.Sprintf("export VALIDATOR_%d_ADDRESS=\"%s\"\n", v.Index, v.Address)
		if v.PubKey != "" {
			output += fmt.Sprintf("export VALIDATOR_%d_PUBKEY=\"%s\"\n", v.Index, v.PubKey)
		}
		if v.Mnemonic != "" {
			output += fmt.Sprintf("export VALIDATOR_%d_MNEMONIC=\"%s\"\n", v.Index, v.Mnemonic)
		}
	}

	for _, a := range export.Accounts {
		output += fmt.Sprintf("export ACCOUNT_%d_ADDRESS=\"%s\"\n", a.Index, a.Address)
		if a.Mnemonic != "" {
			output += fmt.Sprintf("export ACCOUNT_%d_MNEMONIC=\"%s\"\n", a.Index, a.Mnemonic)
		}
	}

	return output
}
