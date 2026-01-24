// pkg/network/cosmos/txbuilder_test.go
package cosmos

import (
	"context"
	"testing"

	"github.com/altuslabsxyz/devnet-builder/pkg/network"
	"github.com/stretchr/testify/require"
)

func TestDetectSDKVersion(t *testing.T) {
	ctx := context.Background()

	// This should fail initially because the function doesn't exist
	version, err := DetectSDKVersion(ctx, "http://localhost:26657")
	require.Error(t, err) // Expected to fail with no running node
	require.Nil(t, version)
}

func TestNewTxBuilder_ReturnsBuilder(t *testing.T) {
	cfg := &network.TxBuilderConfig{
		RPCEndpoint: "http://localhost:26657",
		ChainID:     "test-chain-1",
		SDKVersion: &network.SDKVersion{
			Framework: network.FrameworkCosmosSDK,
			Version:   "v0.50.0",
			Features:  []string{network.FeatureGovV1},
		},
	}

	builder, err := NewTxBuilder(context.Background(), cfg)
	require.NoError(t, err)
	require.NotNil(t, builder)
	require.Implements(t, (*network.TxBuilder)(nil), builder)
}

func TestNewTxBuilder_NilConfig(t *testing.T) {
	builder, err := NewTxBuilder(context.Background(), nil)
	require.Error(t, err)
	require.Nil(t, builder)
	require.Contains(t, err.Error(), "config is required")
}

func TestNewTxBuilder_MissingRPCEndpoint(t *testing.T) {
	cfg := &network.TxBuilderConfig{
		ChainID: "test-chain-1",
		SDKVersion: &network.SDKVersion{
			Framework: network.FrameworkCosmosSDK,
			Version:   "v0.50.0",
		},
	}

	builder, err := NewTxBuilder(context.Background(), cfg)
	require.Error(t, err)
	require.Nil(t, builder)
	require.Contains(t, err.Error(), "RPC endpoint is required")
}

func TestNewTxBuilder_MissingChainID(t *testing.T) {
	cfg := &network.TxBuilderConfig{
		RPCEndpoint: "http://localhost:26657",
		SDKVersion: &network.SDKVersion{
			Framework: network.FrameworkCosmosSDK,
			Version:   "v0.50.0",
		},
	}

	builder, err := NewTxBuilder(context.Background(), cfg)
	require.Error(t, err)
	require.Nil(t, builder)
	require.Contains(t, err.Error(), "chain ID is required")
}

func TestTxBuilder_SupportedTxTypes(t *testing.T) {
	cfg := &network.TxBuilderConfig{
		RPCEndpoint: "http://localhost:26657",
		ChainID:     "test-chain-1",
		SDKVersion: &network.SDKVersion{
			Framework: network.FrameworkCosmosSDK,
			Version:   "v0.50.0",
			Features:  []string{network.FeatureGovV1},
		},
	}

	builder, err := NewTxBuilder(context.Background(), cfg)
	require.NoError(t, err)

	txTypes := builder.SupportedTxTypes()
	require.NotEmpty(t, txTypes)

	// Verify expected transaction types are supported
	expectedTypes := []network.TxType{
		network.TxTypeGovVote,
		network.TxTypeBankSend,
		network.TxTypeStakingDelegate,
		network.TxTypeStakingUnbond,
	}

	for _, expected := range expectedTypes {
		require.Contains(t, txTypes, expected, "Expected tx type %s not found", expected)
	}
}

func TestParseSDKVersion(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantMajor int
		wantMinor int
		wantPatch int
		wantErr   bool
	}{
		{
			name:      "standard version",
			input:     "v0.50.2",
			wantMajor: 0,
			wantMinor: 50,
			wantPatch: 2,
		},
		{
			name:      "version without v prefix",
			input:     "0.47.0",
			wantMajor: 0,
			wantMinor: 47,
			wantPatch: 0,
		},
		{
			name:      "pre-release version",
			input:     "v0.50.0-rc.0",
			wantMajor: 0,
			wantMinor: 50,
			wantPatch: 0,
		},
		{
			name:    "invalid version",
			input:   "invalid",
			wantErr: true,
		},
		{
			name:    "empty version",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			major, minor, patch, err := parseSDKVersion(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantMajor, major)
			require.Equal(t, tt.wantMinor, minor)
			require.Equal(t, tt.wantPatch, patch)
		})
	}
}

func TestVersionAtLeast(t *testing.T) {
	tests := []struct {
		name          string
		version       string
		minMajor      int
		minMinor      int
		minPatch      int
		expectedValue bool
	}{
		{
			name:          "exact match",
			version:       "v0.50.0",
			minMajor:      0,
			minMinor:      50,
			minPatch:      0,
			expectedValue: true,
		},
		{
			name:          "higher patch",
			version:       "v0.50.2",
			minMajor:      0,
			minMinor:      50,
			minPatch:      0,
			expectedValue: true,
		},
		{
			name:          "higher minor",
			version:       "v0.51.0",
			minMajor:      0,
			minMinor:      50,
			minPatch:      0,
			expectedValue: true,
		},
		{
			name:          "lower minor",
			version:       "v0.47.0",
			minMajor:      0,
			minMinor:      50,
			minPatch:      0,
			expectedValue: false,
		},
		{
			name:          "lower patch",
			version:       "v0.50.0",
			minMajor:      0,
			minMinor:      50,
			minPatch:      1,
			expectedValue: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := versionAtLeast(tt.version, tt.minMajor, tt.minMinor, tt.minPatch)
			require.Equal(t, tt.expectedValue, result)
		})
	}
}

func TestDetectFeatures(t *testing.T) {
	tests := []struct {
		name             string
		version          string
		expectedFeatures []string
	}{
		{
			name:    "SDK v0.50+ has all features",
			version: "v0.50.0",
			expectedFeatures: []string{
				network.FeatureGovV1,
				network.FeatureAuthz,
				network.FeatureGroup,
				network.FeatureFeegrant,
			},
		},
		{
			name:    "SDK v0.46+ has gov-v1 and authz",
			version: "v0.46.0",
			expectedFeatures: []string{
				network.FeatureGovV1,
				network.FeatureAuthz,
				network.FeatureFeegrant,
			},
		},
		{
			name:    "SDK v0.45 has limited features",
			version: "v0.45.0",
			expectedFeatures: []string{
				network.FeatureAuthz,
				network.FeatureFeegrant,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			features := detectFeatures(tt.version)
			require.ElementsMatch(t, tt.expectedFeatures, features)
		})
	}
}
