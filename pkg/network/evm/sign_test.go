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
		TxBytes:  []byte{},                         // Would be RLP-encoded tx
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

	// Verify signature is valid using Ecrecover
	recoveredPubKey, err := crypto.Ecrecover(unsignedTx.SignDoc, signedTx.Signature)
	require.NoError(t, err)
	require.Equal(t, crypto.FromECDSAPub(&privKey.PublicKey), recoveredPubKey)
}

func TestSignTx_NilUnsignedTx(t *testing.T) {
	builder := &TxBuilder{
		chainID: big.NewInt(2200),
	}
	privKey, err := crypto.GenerateKey()
	require.NoError(t, err)

	key := &network.SigningKey{
		PrivKey: crypto.FromECDSA(privKey),
	}

	_, err = builder.SignTx(context.Background(), nil, key)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsigned transaction is required")
}

func TestSignTx_NilKey(t *testing.T) {
	builder := &TxBuilder{
		chainID: big.NewInt(2200),
	}
	unsignedTx := &network.UnsignedTx{
		SignDoc: crypto.Keccak256([]byte("test")),
	}

	_, err := builder.SignTx(context.Background(), unsignedTx, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "signing key is required")
}

func TestSignTx_EmptySignDoc(t *testing.T) {
	builder := &TxBuilder{
		chainID: big.NewInt(2200),
	}
	privKey, err := crypto.GenerateKey()
	require.NoError(t, err)

	unsignedTx := &network.UnsignedTx{
		SignDoc: []byte{}, // Empty SignDoc
	}
	key := &network.SigningKey{
		PrivKey: crypto.FromECDSA(privKey),
	}

	_, err = builder.SignTx(context.Background(), unsignedTx, key)
	require.Error(t, err)
	require.Contains(t, err.Error(), "sign document is required")
}

func TestSignTx_EmptyPrivateKey(t *testing.T) {
	builder := &TxBuilder{
		chainID: big.NewInt(2200),
	}
	unsignedTx := &network.UnsignedTx{
		SignDoc: crypto.Keccak256([]byte("test")),
	}
	key := &network.SigningKey{
		PrivKey: []byte{}, // Empty
	}

	_, err := builder.SignTx(context.Background(), unsignedTx, key)
	require.Error(t, err)
	require.Contains(t, err.Error(), "private key required")
}
