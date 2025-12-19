//go:build private

package devnet

import (
	"bufio"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"cosmossdk.io/log"
	dbm "github.com/cosmos/cosmos-db"
	sdkkeyring "github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	"github.com/cosmos/cosmos-sdk/testutil/sims"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/stablelabs/stable/app"
	appcfg "github.com/stablelabs/stable/app/config"
	"github.com/stablelabs/stable/crypto/keyring"
)

// getCodec creates a codec for keyring operations.
func getCodec() (*app.App, error) {
	db := dbm.NewMemDB()
	appLogger := log.NewNopLogger()
	appOpts := sims.NewAppOptionsWithFlagHome(app.DefaultNodeHome)
	tempApp := app.NewApp(
		appLogger,
		db,
		nil,
		false,
		appOpts,
		appcfg.GetEVMChainID(),
		appcfg.EvmAppOptions,
	)
	return tempApp, nil
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
					Address: fmt.Sprintf("(error: %v)", err),
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
					Address: fmt.Sprintf("(error: %v)", err),
				}
			}
			export.Accounts = append(export.Accounts, *key)
		}
	}

	return export, nil
}

// extractValidatorKey extracts key information for a validator using the keyring API.
func extractValidatorKey(homeDir string, index int) (*ValidatorKey, error) {
	nodeDir := filepath.Join(homeDir, "devnet", fmt.Sprintf("node%d", index))

	key := &ValidatorKey{
		Index: index,
	}

	// Get codec for keyring
	tempApp, err := getCodec()
	if err != nil {
		return nil, fmt.Errorf("failed to get codec: %w", err)
	}

	// Create keyring instance for the node directory
	inBuf := bufio.NewReader(strings.NewReader(""))
	kr, err := sdkkeyring.New(
		sdk.KeyringServiceName(),
		sdkkeyring.BackendTest,
		nodeDir,
		inBuf,
		tempApp.AppCodec(),
		keyring.Option(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to open keyring: %w", err)
	}

	// Read validator key by name
	keyName := fmt.Sprintf("validator%d", index)
	record, err := kr.Key(keyName)
	if err != nil {
		return nil, fmt.Errorf("key %s not found: %w", keyName, err)
	}

	// Get account address from record
	addr, err := record.GetAddress()
	if err != nil {
		return nil, fmt.Errorf("failed to get address: %w", err)
	}
	key.Address = addr.String()

	// Get HEX address (EVM compatible)
	key.AddressHex = "0x" + hex.EncodeToString(addr.Bytes())

	// Get valoper address
	valoperAddr := sdk.ValAddress(addr)
	key.ValoperAddr = valoperAddr.String()

	// Get account public key
	pubKey, err := record.GetPubKey()
	if err == nil && pubKey != nil {
		key.AccountPubKey = base64.StdEncoding.EncodeToString(pubKey.Bytes())
	}

	// Try to extract private key from local record
	localInfo := record.GetLocal()
	if localInfo != nil {
		privKeyAny := localInfo.PrivKey
		if privKeyAny != nil {
			// The private key bytes are stored in the Any.Value field
			// For ethsecp256k1, the raw key is 32 bytes
			privKeyBytes := privKeyAny.GetValue()
			if len(privKeyBytes) >= 32 {
				// For ethsecp256k1 keys, extract the actual key bytes
				// The value contains the protobuf-encoded key, we need the last 32 bytes
				rawKey := privKeyBytes
				if len(privKeyBytes) > 32 {
					rawKey = privKeyBytes[len(privKeyBytes)-32:]
				}
				key.AccountPrivKey = base64.StdEncoding.EncodeToString(rawKey)
				key.AccountPrivKeyHex = hex.EncodeToString(rawKey)
			}
		}
	}

	// Read consensus key from priv_validator_key.json
	privKeyPath := filepath.Join(nodeDir, "config", "priv_validator_key.json")
	if data, err := os.ReadFile(privKeyPath); err == nil {
		var privKeyData struct {
			Address string `json:"address"`
			PubKey  struct {
				Type  string `json:"type"`
				Value string `json:"value"`
			} `json:"pub_key"`
			PrivKey struct {
				Type  string `json:"type"`
				Value string `json:"value"`
			} `json:"priv_key"`
		}
		if err := json.Unmarshal(data, &privKeyData); err == nil {
			// Consensus pubkey (base64)
			key.ConsPubKey = privKeyData.PubKey.Value

			// Consensus address (hex from file)
			key.ConsAddrHex = privKeyData.Address

			// Consensus private key (if available)
			if privKeyData.PrivKey.Value != "" {
				key.ConsPrivKeyHex = privKeyData.PrivKey.Value
			}

			// Derive valcons address from consensus pubkey
			if pubKeyBytes, err := base64.StdEncoding.DecodeString(privKeyData.PubKey.Value); err == nil {
				ed25519PubKey := &ed25519.PubKey{Key: pubKeyBytes}
				consAddr := sdk.ConsAddress(ed25519PubKey.Address())
				key.ValconsAddr = consAddr.String()
			}
		}
	}

	// Try to read mnemonic from validator JSON file (if available)
	validatorFile := filepath.Join(nodeDir, fmt.Sprintf("validator%d.json", index))
	if data, err := os.ReadFile(validatorFile); err == nil {
		var validatorData struct {
			Address  string `json:"address"`
			Mnemonic string `json:"mnemonic"`
		}
		if err := json.Unmarshal(data, &validatorData); err == nil {
			key.Mnemonic = validatorData.Mnemonic
		}
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

			// Derive HEX address from bech32
			if accAddr, err := sdk.AccAddressFromBech32(account.Address); err == nil {
				key.AddressHex = "0x" + hex.EncodeToString(accAddr.Bytes())
			}
			return key, nil
		}
	}

	// If JSON file doesn't exist, try reading from keyring
	tempApp, err := getCodec()
	if err != nil {
		return nil, fmt.Errorf("failed to get codec: %w", err)
	}

	inBuf := bufio.NewReader(strings.NewReader(""))
	kr, err := sdkkeyring.New(
		sdk.KeyringServiceName(),
		sdkkeyring.BackendTest,
		accountsDir,
		inBuf,
		tempApp.AppCodec(),
		keyring.Option(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to open accounts keyring: %w", err)
	}

	keyName := fmt.Sprintf("account%d", index)
	record, err := kr.Key(keyName)
	if err != nil {
		return nil, fmt.Errorf("account %s not found: %w", keyName, err)
	}

	addr, err := record.GetAddress()
	if err != nil {
		return nil, fmt.Errorf("failed to get address: %w", err)
	}
	key.Address = addr.String()
	key.AddressHex = "0x" + hex.EncodeToString(addr.Bytes())

	// Get public key
	pubKey, err := record.GetPubKey()
	if err == nil && pubKey != nil {
		key.PubKey = base64.StdEncoding.EncodeToString(pubKey.Bytes())
	}

	return key, nil
}
