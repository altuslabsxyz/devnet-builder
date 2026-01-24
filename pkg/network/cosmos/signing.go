// pkg/network/cosmos/signing.go
package cosmos

import (
	"fmt"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
)

// secp256k1PrivKeySize is the expected length of a secp256k1 private key in bytes.
const secp256k1PrivKeySize = 32

// LoadPrivateKey loads a secp256k1 private key from bytes.
// Expects 32 bytes for secp256k1.
func LoadPrivateKey(privKeyBytes []byte) (cryptotypes.PrivKey, error) {
	if len(privKeyBytes) != secp256k1PrivKeySize {
		return nil, fmt.Errorf("invalid private key length: expected %d, got %d", secp256k1PrivKeySize, len(privKeyBytes))
	}
	privKey := &secp256k1.PrivKey{Key: privKeyBytes}
	return privKey, nil
}

// SignBytes signs arbitrary bytes with the private key.
func SignBytes(privKey cryptotypes.PrivKey, signDoc []byte) ([]byte, error) {
	if privKey == nil {
		return nil, fmt.Errorf("private key is required")
	}
	if signDoc == nil {
		return nil, fmt.Errorf("sign document cannot be nil")
	}
	signature, err := privKey.Sign(signDoc)
	if err != nil {
		return nil, fmt.Errorf("sign: %w", err)
	}
	return signature, nil
}
