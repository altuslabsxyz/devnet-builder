// pkg/network/cosmos/signing.go
package cosmos

import (
	"fmt"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
)

// LoadPrivateKey loads a secp256k1 private key from bytes.
// Expects 32 bytes for secp256k1.
func LoadPrivateKey(privKeyBytes []byte) (cryptotypes.PrivKey, error) {
	if len(privKeyBytes) != 32 {
		return nil, fmt.Errorf("invalid private key length: expected 32, got %d", len(privKeyBytes))
	}
	privKey := &secp256k1.PrivKey{Key: privKeyBytes}
	return privKey, nil
}

// SignBytes signs arbitrary bytes with the private key.
func SignBytes(privKey cryptotypes.PrivKey, signDoc []byte) ([]byte, error) {
	signature, err := privKey.Sign(signDoc)
	if err != nil {
		return nil, fmt.Errorf("sign: %w", err)
	}
	return signature, nil
}
