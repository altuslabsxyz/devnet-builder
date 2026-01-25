// pkg/network/cosmos/sign_test.go
package cosmos

import (
	"context"
	"net/http"
	"testing"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/stretchr/testify/require"

	"github.com/altuslabsxyz/devnet-builder/pkg/network"
)

func TestLoadPrivateKey(t *testing.T) {
	tests := []struct {
		name        string
		keyBytes    []byte
		expectError bool
		errContains string
	}{
		{
			name:        "valid 32-byte key",
			keyBytes:    make([]byte, 32),
			expectError: false,
		},
		{
			name:        "too short key",
			keyBytes:    make([]byte, 16),
			expectError: true,
			errContains: "invalid private key length",
		},
		{
			name:        "too long key",
			keyBytes:    make([]byte, 64),
			expectError: true,
			errContains: "invalid private key length",
		},
		{
			name:        "empty key",
			keyBytes:    []byte{},
			expectError: true,
			errContains: "invalid private key length",
		},
		{
			name:        "nil key",
			keyBytes:    nil,
			expectError: true,
			errContains: "invalid private key length",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			privKey, err := LoadPrivateKey(tt.keyBytes)
			if tt.expectError {
				require.Error(t, err)
				require.Nil(t, privKey)
				require.Contains(t, err.Error(), tt.errContains)
			} else {
				require.NoError(t, err)
				require.NotNil(t, privKey)
			}
		})
	}
}

func TestLoadPrivateKey_ValidKey(t *testing.T) {
	// Generate a real key and test loading it
	origKey := secp256k1.GenPrivKey()
	keyBytes := origKey.Bytes()

	loadedKey, err := LoadPrivateKey(keyBytes)
	require.NoError(t, err)
	require.NotNil(t, loadedKey)

	// Verify the loaded key produces the same public key
	require.Equal(t, origKey.PubKey().Bytes(), loadedKey.PubKey().Bytes())
}

func TestSignBytes(t *testing.T) {
	privKey := secp256k1.GenPrivKey()
	signDoc := []byte("test sign document")

	signature, err := SignBytes(privKey, signDoc)
	require.NoError(t, err)
	require.NotEmpty(t, signature)

	// Verify the signature is valid
	pubKey := privKey.PubKey()
	isValid := pubKey.VerifySignature(signDoc, signature)
	require.True(t, isValid, "signature should be valid")
}

func TestSignBytes_DifferentDocuments(t *testing.T) {
	privKey := secp256k1.GenPrivKey()
	doc1 := []byte("document 1")
	doc2 := []byte("document 2")

	sig1, err := SignBytes(privKey, doc1)
	require.NoError(t, err)

	sig2, err := SignBytes(privKey, doc2)
	require.NoError(t, err)

	// Signatures should be different for different documents
	require.NotEqual(t, sig1, sig2)

	// Each signature should only verify against its own document
	pubKey := privKey.PubKey()
	require.True(t, pubKey.VerifySignature(doc1, sig1))
	require.True(t, pubKey.VerifySignature(doc2, sig2))
	require.False(t, pubKey.VerifySignature(doc1, sig2))
	require.False(t, pubKey.VerifySignature(doc2, sig1))
}

func TestSignBytes_EmptyDocument(t *testing.T) {
	privKey := secp256k1.GenPrivKey()
	emptyDoc := []byte{}

	signature, err := SignBytes(privKey, emptyDoc)
	require.NoError(t, err)
	require.NotEmpty(t, signature)

	// Verify the signature
	pubKey := privKey.PubKey()
	require.True(t, pubKey.VerifySignature(emptyDoc, signature))
}

func TestSignBytes_NilPrivateKey(t *testing.T) {
	signDoc := []byte("test sign document")

	signature, err := SignBytes(nil, signDoc)
	require.Error(t, err)
	require.Nil(t, signature)
	require.Contains(t, err.Error(), "private key is required")
}

func TestSignBytes_NilSignDoc(t *testing.T) {
	privKey := secp256k1.GenPrivKey()

	signature, err := SignBytes(privKey, nil)
	require.Error(t, err)
	require.Nil(t, signature)
	require.Contains(t, err.Error(), "sign document cannot be nil")
}

func TestSignTx(t *testing.T) {
	// Generate a test key
	privKey := secp256k1.GenPrivKey()
	pubKey := privKey.PubKey()

	// Create TxConfig for proper protobuf encoding
	txConfig := NewTxConfig()

	// Build a real unsigned tx using SDK's TxBuilder
	sdkTxBuilder := txConfig.NewTxBuilder()
	sdkTxBuilder.SetMemo("test memo")
	sdkTxBuilder.SetGasLimit(200000)

	// Encode to get valid protobuf bytes
	txBytes, err := txConfig.TxEncoder()(sdkTxBuilder.GetTx())
	require.NoError(t, err)

	builder := &TxBuilder{
		rpcEndpoint: "http://localhost:26657",
		chainID:     "test-1",
		client:      &http.Client{},
		sdkVersion: &network.SDKVersion{
			Framework: network.FrameworkCosmosSDK,
			Version:   "v0.50.0",
			Features:  []string{network.FeatureGovV1},
		},
		txConfig: txConfig,
	}

	unsignedTx := &network.UnsignedTx{
		TxBytes:       txBytes,
		SignDoc:       []byte("test sign document"),
		AccountNumber: 5,
		Sequence:      10,
	}

	key := &network.SigningKey{
		Address: "cosmos1test",
		PrivKey: privKey.Bytes(),
	}

	signedTx, err := builder.SignTx(context.Background(), unsignedTx, key)
	require.NoError(t, err)
	require.NotNil(t, signedTx)

	// Assert: signature not empty
	require.NotEmpty(t, signedTx.Signature)

	// Assert: pubkey matches
	require.Equal(t, pubKey.Bytes(), signedTx.PubKey)

	// Assert: signature is valid
	isValid := pubKey.VerifySignature(unsignedTx.SignDoc, signedTx.Signature)
	require.True(t, isValid, "signature should be valid")

	// Assert: signed tx bytes are different (they include the signature)
	require.NotEqual(t, txBytes, signedTx.TxBytes)
}

func TestSignTx_MissingPrivateKey(t *testing.T) {
	builder := &TxBuilder{
		rpcEndpoint: "http://localhost:26657",
		chainID:     "test-1",
		client:      &http.Client{},
		sdkVersion: &network.SDKVersion{
			Framework: network.FrameworkCosmosSDK,
			Version:   "v0.50.0",
		},
		txConfig: NewTxConfig(),
	}

	unsignedTx := &network.UnsignedTx{
		TxBytes:       []byte("unsigned-tx"),
		SignDoc:       []byte("sign doc"),
		AccountNumber: 1,
		Sequence:      0,
	}

	// Empty PrivKey should error
	key := &network.SigningKey{
		Address: "cosmos1test",
		PrivKey: []byte{},
	}

	signedTx, err := builder.SignTx(context.Background(), unsignedTx, key)
	require.Error(t, err)
	require.Nil(t, signedTx)
	require.Contains(t, err.Error(), "private key required")
}

func TestSignTx_NilPrivateKey(t *testing.T) {
	builder := &TxBuilder{
		rpcEndpoint: "http://localhost:26657",
		chainID:     "test-1",
		client:      &http.Client{},
		sdkVersion: &network.SDKVersion{
			Framework: network.FrameworkCosmosSDK,
			Version:   "v0.50.0",
		},
		txConfig: NewTxConfig(),
	}

	unsignedTx := &network.UnsignedTx{
		TxBytes:       []byte("unsigned-tx"),
		SignDoc:       []byte("sign doc"),
		AccountNumber: 1,
		Sequence:      0,
	}

	// Nil PrivKey should error
	key := &network.SigningKey{
		Address: "cosmos1test",
		PrivKey: nil,
	}

	signedTx, err := builder.SignTx(context.Background(), unsignedTx, key)
	require.Error(t, err)
	require.Nil(t, signedTx)
	require.Contains(t, err.Error(), "private key required")
}

func TestSignTx_InvalidKeyLength(t *testing.T) {
	builder := &TxBuilder{
		rpcEndpoint: "http://localhost:26657",
		chainID:     "test-1",
		client:      &http.Client{},
		sdkVersion: &network.SDKVersion{
			Framework: network.FrameworkCosmosSDK,
			Version:   "v0.50.0",
		},
		txConfig: NewTxConfig(),
	}

	unsignedTx := &network.UnsignedTx{
		TxBytes:       []byte("unsigned-tx"),
		SignDoc:       []byte("sign doc"),
		AccountNumber: 1,
		Sequence:      0,
	}

	tests := []struct {
		name    string
		keyLen  int
		wantErr string
	}{
		{
			name:    "too short key (16 bytes)",
			keyLen:  16,
			wantErr: "invalid private key length",
		},
		{
			name:    "too long key (64 bytes)",
			keyLen:  64,
			wantErr: "invalid private key length",
		},
		{
			name:    "wrong length (31 bytes)",
			keyLen:  31,
			wantErr: "invalid private key length",
		},
		{
			name:    "wrong length (33 bytes)",
			keyLen:  33,
			wantErr: "invalid private key length",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := &network.SigningKey{
				Address: "cosmos1test",
				PrivKey: make([]byte, tt.keyLen),
			}

			signedTx, err := builder.SignTx(context.Background(), unsignedTx, key)
			require.Error(t, err)
			require.Nil(t, signedTx)
			require.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestSignTx_EncodesSignedTx(t *testing.T) {
	privKey := secp256k1.GenPrivKey()

	// Create TxConfig for proper protobuf encoding
	txConfig := NewTxConfig()

	// Build a real unsigned tx using SDK's TxBuilder
	sdkTxBuilder := txConfig.NewTxBuilder()
	sdkTxBuilder.SetMemo("test memo")
	sdkTxBuilder.SetGasLimit(100000)

	// Encode to get valid protobuf bytes
	originalTxBytes, err := txConfig.TxEncoder()(sdkTxBuilder.GetTx())
	require.NoError(t, err)

	builder := &TxBuilder{
		rpcEndpoint: "http://localhost:26657",
		chainID:     "test-1",
		client:      &http.Client{},
		sdkVersion: &network.SDKVersion{
			Framework: network.FrameworkCosmosSDK,
			Version:   "v0.50.0",
		},
		txConfig: txConfig,
	}

	unsignedTx := &network.UnsignedTx{
		TxBytes:       originalTxBytes,
		SignDoc:       []byte("sign doc"),
		AccountNumber: 1,
		Sequence:      0,
	}

	key := &network.SigningKey{
		Address: "cosmos1test",
		PrivKey: privKey.Bytes(),
	}

	signedTx, err := builder.SignTx(context.Background(), unsignedTx, key)
	require.NoError(t, err)

	// TxBytes should be different - they now include the signature
	require.NotEqual(t, originalTxBytes, signedTx.TxBytes)

	// Signed tx bytes should be longer (includes signature data)
	require.Greater(t, len(signedTx.TxBytes), len(originalTxBytes))

	// Should be able to decode the signed tx
	decodedTx, err := txConfig.TxDecoder()(signedTx.TxBytes)
	require.NoError(t, err)
	require.NotNil(t, decodedTx)
}

func TestSignTx_DifferentKeys(t *testing.T) {
	// Generate two different keys
	privKey1 := secp256k1.GenPrivKey()
	privKey2 := secp256k1.GenPrivKey()

	// Create TxConfig for proper protobuf encoding
	txConfig := NewTxConfig()

	// Build a real unsigned tx using SDK's TxBuilder
	sdkTxBuilder := txConfig.NewTxBuilder()
	sdkTxBuilder.SetMemo("test memo for different keys")
	sdkTxBuilder.SetGasLimit(100000)

	// Encode to get valid protobuf bytes
	txBytes, err := txConfig.TxEncoder()(sdkTxBuilder.GetTx())
	require.NoError(t, err)

	builder := &TxBuilder{
		rpcEndpoint: "http://localhost:26657",
		chainID:     "test-1",
		client:      &http.Client{},
		sdkVersion: &network.SDKVersion{
			Framework: network.FrameworkCosmosSDK,
			Version:   "v0.50.0",
		},
		txConfig: txConfig,
	}

	unsignedTx := &network.UnsignedTx{
		TxBytes:       txBytes,
		SignDoc:       []byte("same sign document"),
		AccountNumber: 1,
		Sequence:      0,
	}

	key1 := &network.SigningKey{
		Address: "cosmos1test1",
		PrivKey: privKey1.Bytes(),
	}

	key2 := &network.SigningKey{
		Address: "cosmos1test2",
		PrivKey: privKey2.Bytes(),
	}

	signedTx1, err := builder.SignTx(context.Background(), unsignedTx, key1)
	require.NoError(t, err)

	signedTx2, err := builder.SignTx(context.Background(), unsignedTx, key2)
	require.NoError(t, err)

	// Signatures should be different
	require.NotEqual(t, signedTx1.Signature, signedTx2.Signature)

	// Public keys should be different
	require.NotEqual(t, signedTx1.PubKey, signedTx2.PubKey)

	// Each signature should only verify with its own public key
	pubKey1 := privKey1.PubKey()
	pubKey2 := privKey2.PubKey()

	require.True(t, pubKey1.VerifySignature(unsignedTx.SignDoc, signedTx1.Signature))
	require.True(t, pubKey2.VerifySignature(unsignedTx.SignDoc, signedTx2.Signature))
	require.False(t, pubKey1.VerifySignature(unsignedTx.SignDoc, signedTx2.Signature))
	require.False(t, pubKey2.VerifySignature(unsignedTx.SignDoc, signedTx1.Signature))
}
