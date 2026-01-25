// pkg/network/evm/sign_test.go
package evm

import (
	"context"
	"math/big"
	"testing"

	"github.com/altuslabsxyz/devnet-builder/pkg/network"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/stretchr/testify/require"
)

func TestSignTx(t *testing.T) {
	// Generate a test key
	privKey, err := crypto.GenerateKey()
	require.NoError(t, err)

	builder := &TxBuilder{
		chainID: big.NewInt(2200),
	}

	// Build a real unsigned transaction first
	ctx := context.Background()
	toAddr := "0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb0" // Valid 20-byte hex address
	req := &network.TxBuildRequest{
		TxType:   network.TxTypeBankSend,
		Sender:   crypto.PubkeyToAddress(privKey.PublicKey).Hex(),
		GasLimit: 21000,
		Payload:  []byte(`{"to_address":"` + toAddr + `","amount":"1000000000000000000"}`),
	}

	unsignedTx, err := builder.buildNativeTransfer(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, unsignedTx)
	require.NotEmpty(t, unsignedTx.TxBytes)
	require.NotEmpty(t, unsignedTx.SignDoc)

	key := &network.SigningKey{
		Address: crypto.PubkeyToAddress(privKey.PublicKey).Hex(),
		PrivKey: crypto.FromECDSA(privKey),
	}

	// Sign the transaction
	signedTx, err := builder.SignTx(context.Background(), unsignedTx, key)
	require.NoError(t, err)
	require.NotNil(t, signedTx)
	require.NotEmpty(t, signedTx.Signature)
	require.NotEmpty(t, signedTx.TxBytes)

	// CRITICAL TEST: Signed tx bytes must be different from unsigned (signature embedded)
	// This is the main fix - before the fix, signedTx.TxBytes == unsignedTx.TxBytes
	require.NotEqual(t, unsignedTx.TxBytes, signedTx.TxBytes,
		"signed transaction bytes must differ from unsigned (signature should be embedded)")

	// Verify the signed transaction bytes are longer (includes signature)
	require.Greater(t, len(signedTx.TxBytes), len(unsignedTx.TxBytes),
		"signed transaction should be larger than unsigned (includes r, s, v signature components)")

	// Verify the signed transaction can be decoded (proves it's valid RLP)
	var decodedTx types.Transaction
	err = rlp.DecodeBytes(signedTx.TxBytes, &decodedTx)
	require.NoError(t, err, "signed transaction should be valid RLP-encoded")

	// Verify the decoded transaction has a valid signature
	v, r, s := decodedTx.RawSignatureValues()
	require.NotNil(t, v, "v signature component should be present")
	require.NotNil(t, r, "r signature component should be present")
	require.NotNil(t, s, "s signature component should be present")
	require.True(t, r.Sign() > 0, "r should be non-zero")
	require.True(t, s.Sign() > 0, "s should be non-zero")

	// Verify transaction fields are preserved (nonce, gas, etc)
	require.Equal(t, unsignedTx.Sequence, decodedTx.Nonce(),
		"nonce should be preserved in signed transaction")
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
