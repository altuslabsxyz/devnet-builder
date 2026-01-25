// pkg/network/cosmos/build_test.go
package cosmos

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/altuslabsxyz/devnet-builder/pkg/network"
	"github.com/stretchr/testify/require"
)

func TestBuildTx_GovVote(t *testing.T) {
	// Mock account query server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Contains(t, r.URL.Path, "/cosmos/auth/v1beta1/accounts/")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"account": {
				"@type": "/cosmos.auth.v1beta1.BaseAccount",
				"address": "cosmos1voter123",
				"account_number": "42",
				"sequence": "7"
			}
		}`))
	}))
	defer server.Close()

	builder := &TxBuilder{
		rpcEndpoint: server.URL,
		chainID:     "test-1",
		client:      &http.Client{},
		sdkVersion: &network.SDKVersion{
			Framework: network.FrameworkCosmosSDK,
			Version:   "v0.50.0",
			Features:  []string{network.FeatureGovV1},
		},
		txConfig: NewTxConfig(),
	}

	// Create request with TxTypeGovVote payload
	payload, err := json.Marshal(map[string]interface{}{
		"proposal_id": 1,
		"option":      "yes",
	})
	require.NoError(t, err)

	req := &network.TxBuildRequest{
		TxType:   network.TxTypeGovVote,
		Sender:   "cosmos1voter123",
		ChainID:  "test-1",
		Payload:  payload,
		GasLimit: 200000,
		GasPrice: "0.025stake",
	}

	unsignedTx, err := builder.BuildTx(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, unsignedTx)

	// Verify the account information was fetched correctly
	require.Equal(t, uint64(42), unsignedTx.AccountNumber)
	require.Equal(t, uint64(7), unsignedTx.Sequence)

	// The tx bytes should contain the message
	require.NotEmpty(t, unsignedTx.TxBytes)
	require.NotEmpty(t, unsignedTx.SignDoc)
}

func TestBuildTx_GovVote_InvalidOption(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"account": {
				"@type": "/cosmos.auth.v1beta1.BaseAccount",
				"address": "cosmos1voter123",
				"account_number": "42",
				"sequence": "7"
			}
		}`))
	}))
	defer server.Close()

	builder := &TxBuilder{
		rpcEndpoint: server.URL,
		chainID:     "test-1",
		client:      &http.Client{},
		sdkVersion: &network.SDKVersion{
			Framework: network.FrameworkCosmosSDK,
			Version:   "v0.50.0",
			Features:  []string{network.FeatureGovV1},
		},
		txConfig: NewTxConfig(),
	}

	payload, err := json.Marshal(map[string]interface{}{
		"proposal_id": 1,
		"option":      "invalid_option",
	})
	require.NoError(t, err)

	req := &network.TxBuildRequest{
		TxType:   network.TxTypeGovVote,
		Sender:   "cosmos1voter123",
		ChainID:  "test-1",
		Payload:  payload,
		GasLimit: 200000,
		GasPrice: "0.025stake",
	}

	unsignedTx, err := builder.BuildTx(context.Background(), req)
	require.Error(t, err)
	require.Nil(t, unsignedTx)
	require.Contains(t, err.Error(), "invalid vote option")
}

func TestBuildTx_BankSend(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"account": {
				"@type": "/cosmos.auth.v1beta1.BaseAccount",
				"address": "cosmos1sender123",
				"account_number": "10",
				"sequence": "3"
			}
		}`))
	}))
	defer server.Close()

	builder := &TxBuilder{
		rpcEndpoint: server.URL,
		chainID:     "test-1",
		client:      &http.Client{},
		sdkVersion: &network.SDKVersion{
			Framework: network.FrameworkCosmosSDK,
			Version:   "v0.50.0",
			Features:  []string{network.FeatureGovV1},
		},
		txConfig: NewTxConfig(),
	}

	payload, err := json.Marshal(map[string]interface{}{
		"to_address": "cosmos1receiver456",
		"amount":     "1000stake",
	})
	require.NoError(t, err)

	req := &network.TxBuildRequest{
		TxType:   network.TxTypeBankSend,
		Sender:   "cosmos1sender123",
		ChainID:  "test-1",
		Payload:  payload,
		GasLimit: 100000,
		GasPrice: "0.025stake",
	}

	unsignedTx, err := builder.BuildTx(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, unsignedTx)
	require.Equal(t, uint64(10), unsignedTx.AccountNumber)
	require.Equal(t, uint64(3), unsignedTx.Sequence)
	require.NotEmpty(t, unsignedTx.TxBytes)
}

func TestBuildTx_StakingDelegate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"account": {
				"@type": "/cosmos.auth.v1beta1.BaseAccount",
				"address": "cosmos1delegator123",
				"account_number": "5",
				"sequence": "1"
			}
		}`))
	}))
	defer server.Close()

	builder := &TxBuilder{
		rpcEndpoint: server.URL,
		chainID:     "test-1",
		client:      &http.Client{},
		sdkVersion: &network.SDKVersion{
			Framework: network.FrameworkCosmosSDK,
			Version:   "v0.50.0",
			Features:  []string{network.FeatureGovV1},
		},
		txConfig: NewTxConfig(),
	}

	payload, err := json.Marshal(map[string]interface{}{
		"validator_address": "cosmosvaloper1validator789",
		"amount":            "5000stake",
	})
	require.NoError(t, err)

	req := &network.TxBuildRequest{
		TxType:   network.TxTypeStakingDelegate,
		Sender:   "cosmos1delegator123",
		ChainID:  "test-1",
		Payload:  payload,
		GasLimit: 250000,
		GasPrice: "0.025stake",
	}

	unsignedTx, err := builder.BuildTx(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, unsignedTx)
	require.Equal(t, uint64(5), unsignedTx.AccountNumber)
	require.Equal(t, uint64(1), unsignedTx.Sequence)
	require.NotEmpty(t, unsignedTx.TxBytes)
}

func TestBuildTx_UnsupportedTxType(t *testing.T) {
	builder := &TxBuilder{
		rpcEndpoint: "http://localhost:1317",
		chainID:     "test-1",
		client:      &http.Client{},
		sdkVersion: &network.SDKVersion{
			Framework: network.FrameworkCosmosSDK,
			Version:   "v0.50.0",
			Features:  []string{network.FeatureGovV1},
		},
		txConfig: NewTxConfig(),
	}

	payload, err := json.Marshal(map[string]interface{}{})
	require.NoError(t, err)

	req := &network.TxBuildRequest{
		TxType:   network.TxType("unknown/type"),
		Sender:   "cosmos1test",
		ChainID:  "test-1",
		Payload:  payload,
		GasLimit: 100000,
		GasPrice: "0.025stake",
	}

	unsignedTx, err := builder.BuildTx(context.Background(), req)
	require.Error(t, err)
	require.Nil(t, unsignedTx)
	require.Contains(t, err.Error(), "unsupported transaction type")
}

func TestBuildTx_AccountQueryFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error": "account not found"}`))
	}))
	defer server.Close()

	builder := &TxBuilder{
		rpcEndpoint: server.URL,
		chainID:     "test-1",
		client:      &http.Client{},
		sdkVersion: &network.SDKVersion{
			Framework: network.FrameworkCosmosSDK,
			Version:   "v0.50.0",
			Features:  []string{network.FeatureGovV1},
		},
		txConfig: NewTxConfig(),
	}

	payload, err := json.Marshal(map[string]interface{}{
		"proposal_id": 1,
		"option":      "yes",
	})
	require.NoError(t, err)

	req := &network.TxBuildRequest{
		TxType:   network.TxTypeGovVote,
		Sender:   "cosmos1notfound",
		ChainID:  "test-1",
		Payload:  payload,
		GasLimit: 200000,
		GasPrice: "0.025stake",
	}

	unsignedTx, err := builder.BuildTx(context.Background(), req)
	require.Error(t, err)
	require.Nil(t, unsignedTx)
	require.Contains(t, err.Error(), "not found")
}

func TestBuildMessage_GovVote(t *testing.T) {
	payload, err := json.Marshal(map[string]interface{}{
		"proposal_id": uint64(1),
		"option":      "yes",
	})
	require.NoError(t, err)

	msg, err := BuildMessage(network.TxTypeGovVote, "cosmos1voter", payload)
	require.NoError(t, err)
	require.NotNil(t, msg)
}

func TestBuildMessage_GovVote_AllOptions(t *testing.T) {
	options := []string{"yes", "no", "abstain", "no_with_veto"}

	for _, opt := range options {
		t.Run(opt, func(t *testing.T) {
			payload, err := json.Marshal(map[string]interface{}{
				"proposal_id": uint64(1),
				"option":      opt,
			})
			require.NoError(t, err)

			msg, err := BuildMessage(network.TxTypeGovVote, "cosmos1voter", payload)
			require.NoError(t, err)
			require.NotNil(t, msg)
		})
	}
}

func TestParseGasPrice(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantAmount  string
		wantDenom   string
		expectError bool
	}{
		{
			name:       "standard gas price",
			input:      "0.025stake",
			wantAmount: "0.025000000000000000", // SDK Dec uses 18 decimal places
			wantDenom:  "stake",
		},
		{
			name:       "integer amount",
			input:      "1uatom",
			wantAmount: "1.000000000000000000",
			wantDenom:  "uatom",
		},
		{
			name:       "large decimal",
			input:      "0.000001ustable",
			wantAmount: "0.000001000000000000",
			wantDenom:  "ustable",
		},
		{
			name:        "empty string",
			input:       "",
			expectError: true,
		},
		{
			name:        "no denom",
			input:       "0.025",
			expectError: true,
		},
		{
			name:        "no amount",
			input:       "stake",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			coin, err := ParseGasPrice(tt.input)
			if tt.expectError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantDenom, coin.Denom)
			require.Equal(t, tt.wantAmount, coin.Amount.String())
		})
	}
}

func TestParseVoteOption(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantOption  int32
		expectError bool
	}{
		{
			name:       "yes",
			input:      "yes",
			wantOption: 1, // VOTE_OPTION_YES
		},
		{
			name:       "YES uppercase",
			input:      "YES",
			wantOption: 1,
		},
		{
			name:       "no",
			input:      "no",
			wantOption: 3, // VOTE_OPTION_NO
		},
		{
			name:       "abstain",
			input:      "abstain",
			wantOption: 2, // VOTE_OPTION_ABSTAIN
		},
		{
			name:       "no_with_veto",
			input:      "no_with_veto",
			wantOption: 4, // VOTE_OPTION_NO_WITH_VETO
		},
		{
			name:       "nowithveto without underscore",
			input:      "nowithveto",
			wantOption: 4, // VOTE_OPTION_NO_WITH_VETO
		},
		{
			name:        "invalid",
			input:       "invalid",
			expectError: true,
		},
		{
			name:        "empty",
			input:       "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			option, err := parseVoteOption(tt.input)
			if tt.expectError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantOption, int32(option))
		})
	}
}

func TestParseAmount(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantAmount  string
		wantDenom   string
		expectError bool
	}{
		{
			name:       "standard coin",
			input:      "1000stake",
			wantAmount: "1000",
			wantDenom:  "stake",
		},
		{
			name:       "large amount",
			input:      "1000000000uatom",
			wantAmount: "1000000000",
			wantDenom:  "uatom",
		},
		{
			name:        "empty",
			input:       "",
			expectError: true,
		},
		{
			name:        "decimal amount rejected",
			input:       "1000.5stake",
			expectError: true,
		},
		{
			name:        "zero amount rejected",
			input:       "0stake",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			coin, err := ParseAmount(tt.input)
			if tt.expectError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantDenom, coin.Denom)
			require.Equal(t, tt.wantAmount, coin.Amount.String())
		})
	}
}
