// pkg/network/evm/sign_test.go
package evm

import (
	"context"
	"math/big"
	"testing"

	"github.com/altuslabsxyz/devnet-builder/pkg/network"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

func TestSignTx(t *testing.T) {
	// Generate a test key
	privKey, err := crypto.GenerateKey()
	require.NoError(t, err)

	builder := &TxBuilder{
		chainID: big.NewInt(2200),
	}

	// Create unsigned tx (normally from BuildTx)
	unsignedTx := &network.UnsignedTx{
		TxBytes:  []byte{}, // Would be RLP-encoded tx
		SignDoc:  crypto.Keccak256([]byte("test")), // Would be tx hash
		Sequence: 0,
	}

	key := &network.SigningKey{
		Address: crypto.PubkeyToAddress(privKey.PublicKey).Hex(),
		PrivKey: crypto.FromECDSA(privKey),
	}

	signedTx, err := builder.SignTx(context.Background(), unsignedTx, key)
	require.NoError(t, err)
	require.NotNil(t, signedTx)
	require.NotEmpty(t, signedTx.Signature)
}
