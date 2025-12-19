//go:build private

package generator

import (
	"bufio"
	"os"
	"strings"
	"testing"

	"cosmossdk.io/log"
	dbm "github.com/cosmos/cosmos-db"
	sdkkeyring "github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/testutil/sims"
	sdk "github.com/cosmos/cosmos-sdk/types"
	evmhd "github.com/cosmos/evm/crypto/hd"
	"github.com/stablelabs/stable/app"
	appcfg "github.com/stablelabs/stable/app/config"
	"github.com/stablelabs/stable/crypto/keyring"
)

// getTestCodec creates a codec for test operations.
func getTestCodec() (*app.App, error) {
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

// TestDeterministicMnemonics verifies that validators 0-3 use hardcoded mnemonics
// and generate the same addresses every time.
func TestDeterministicMnemonics(t *testing.T) {
	// Create a temporary directory for the keyring
	tempDir, err := os.MkdirTemp("", "test-keyring-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Get the app codec
	tempApp, err := getTestCodec()
	if err != nil {
		t.Fatalf("Failed to get codec: %v", err)
	}

	// Create keyring
	inBuf := bufio.NewReader(strings.NewReader(""))
	kr, err := sdkkeyring.New(
		sdk.KeyringServiceName(),
		sdkkeyring.BackendTest,
		tempDir,
		inBuf,
		tempApp.AppCodec(),
		keyring.Option(),
	)
	if err != nil {
		t.Fatalf("Failed to create keyring: %v", err)
	}

	// Get supported algorithms
	keyringAlgos, _ := kr.SupportedAlgorithms()
	algo, err := sdkkeyring.NewSigningAlgoFromString(string(evmhd.EthSecp256k1Type), keyringAlgos)
	if err != nil {
		t.Fatalf("Failed to get signing algo: %v", err)
	}

	// Generate keys for validators 0-3 using deterministic mnemonics
	addresses := make([]string, 4)
	for i := 0; i < 4; i++ {
		if i >= len(deterministicValidatorMnemonics) {
			t.Fatalf("Missing mnemonic for validator %d", i)
		}

		mnemonic := deterministicValidatorMnemonics[i]
		valName := "validator" + string(rune('0'+i))

		record, err := kr.NewAccount(valName, mnemonic, sdkkeyring.DefaultBIP39Passphrase, sdk.GetConfig().GetFullBIP44Path(), algo)
		if err != nil {
			t.Fatalf("Failed to create key for validator%d: %v", i, err)
		}

		addr, err := record.GetAddress()
		if err != nil {
			t.Fatalf("Failed to get address for validator%d: %v", i, err)
		}

		addresses[i] = addr.String()
		t.Logf("Validator %d: %s (mnemonic: %s...)", i, addresses[i], mnemonic[:30])
	}

	// Verify all addresses are unique
	seen := make(map[string]bool)
	for i, addr := range addresses {
		if seen[addr] {
			t.Errorf("Duplicate address for validator %d: %s", i, addr)
		}
		seen[addr] = true
	}

	// Now verify determinism by recreating the same keys
	tempDir2, err := os.MkdirTemp("", "test-keyring-2-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir 2: %v", err)
	}
	defer os.RemoveAll(tempDir2)

	kr2, err := sdkkeyring.New(
		sdk.KeyringServiceName(),
		sdkkeyring.BackendTest,
		tempDir2,
		inBuf,
		tempApp.AppCodec(),
		keyring.Option(),
	)
	if err != nil {
		t.Fatalf("Failed to create keyring 2: %v", err)
	}

	for i := 0; i < 4; i++ {
		mnemonic := deterministicValidatorMnemonics[i]
		valName := "validator" + string(rune('0'+i))

		record, err := kr2.NewAccount(valName, mnemonic, sdkkeyring.DefaultBIP39Passphrase, sdk.GetConfig().GetFullBIP44Path(), algo)
		if err != nil {
			t.Fatalf("Failed to create key 2 for validator%d: %v", i, err)
		}

		addr, err := record.GetAddress()
		if err != nil {
			t.Fatalf("Failed to get address 2 for validator%d: %v", i, err)
		}

		if addr.String() != addresses[i] {
			t.Errorf("Determinism failed for validator %d: first=%s, second=%s", i, addresses[i], addr.String())
		}
	}

	t.Log("All deterministic mnemonics generate consistent addresses")
}

// TestDeterministicMnemonicsCount verifies we have exactly 4 deterministic mnemonics
func TestDeterministicMnemonicsCount(t *testing.T) {
	if len(deterministicValidatorMnemonics) != 4 {
		t.Errorf("Expected 4 deterministic mnemonics, got %d", len(deterministicValidatorMnemonics))
	}
}

// TestDeterministicMnemonicsAreValid verifies all mnemonics are valid BIP39
func TestDeterministicMnemonicsAreValid(t *testing.T) {
	for i, mnemonic := range deterministicValidatorMnemonics {
		words := strings.Fields(mnemonic)
		// BIP39 mnemonics are 12, 15, 18, 21, or 24 words
		validLengths := []int{12, 15, 18, 21, 24}
		valid := false
		for _, l := range validLengths {
			if len(words) == l {
				valid = true
				break
			}
		}
		if !valid {
			t.Errorf("Validator %d mnemonic has invalid word count: %d (expected 12, 15, 18, 21, or 24)", i, len(words))
		}
	}
}
