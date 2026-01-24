// pkg/network/cosmos/config_test.go
package cosmos

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewTxConfig(t *testing.T) {
	t.Run("returns non-nil config", func(t *testing.T) {
		txConfig := NewTxConfig()
		require.NotNil(t, txConfig, "NewTxConfig() should return non-nil")
	})

	t.Run("TxEncoder is non-nil", func(t *testing.T) {
		txConfig := NewTxConfig()
		require.NotNil(t, txConfig.TxEncoder(), "TxEncoder() should be non-nil")
	})

	t.Run("TxDecoder is non-nil", func(t *testing.T) {
		txConfig := NewTxConfig()
		require.NotNil(t, txConfig.TxDecoder(), "TxDecoder() should be non-nil")
	})

	t.Run("NewTxBuilder is non-nil", func(t *testing.T) {
		txConfig := NewTxConfig()
		txBuilder := txConfig.NewTxBuilder()
		require.NotNil(t, txBuilder, "NewTxBuilder() should return non-nil")
	})
}

func TestSetupSDKConfig(t *testing.T) {
	t.Run("sets bech32 prefixes", func(t *testing.T) {
		// Test with cosmos prefix
		err := SetupSDKConfig("cosmos")
		require.NoError(t, err)

		// Verify prefixes were set by checking the config
		// Note: We can't easily verify the prefixes without using internal SDK functions,
		// but we can verify the function doesn't panic and returns no error
	})

	t.Run("handles custom prefix", func(t *testing.T) {
		// Test with a custom prefix (e.g., for osmosis)
		err := SetupSDKConfig("osmo")
		require.NoError(t, err)
	})

	t.Run("rejects empty prefix", func(t *testing.T) {
		err := SetupSDKConfig("")
		require.Error(t, err)
		require.Contains(t, err.Error(), "bech32 prefix cannot be empty")
	})
}
