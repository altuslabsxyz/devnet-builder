package rpc

import (
	"strings"
	"testing"
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/application/ports"
)

// TestValidateGovParams_Valid tests validation of valid parameters.
func TestValidateGovParams_Valid(t *testing.T) {
	testCases := []struct {
		name   string
		params *ports.GovParams
	}{
		{
			name: "valid with expedited",
			params: &ports.GovParams{
				VotingPeriod:          48 * time.Hour,
				ExpeditedVotingPeriod: 24 * time.Hour,
				MinDeposit:            "10000000",
				ExpeditedMinDeposit:   "50000000",
			},
		},
		{
			name: "valid without expedited",
			params: &ports.GovParams{
				VotingPeriod:          48 * time.Hour,
				ExpeditedVotingPeriod: 0,
				MinDeposit:            "10000000",
				ExpeditedMinDeposit:   "",
			},
		},
		{
			name: "valid with denom suffix",
			params: &ports.GovParams{
				VotingPeriod:          60 * time.Second,
				ExpeditedVotingPeriod: 30 * time.Second,
				MinDeposit:            "10000000uatom",
				ExpeditedMinDeposit:   "50000000uatom",
			},
		},
		{
			name: "valid minimal expedited period",
			params: &ports.GovParams{
				VotingPeriod:          2 * time.Second,
				ExpeditedVotingPeriod: 1 * time.Second,
				MinDeposit:            "1",
				ExpeditedMinDeposit:   "5",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateGovParams(tc.params)
			if err != nil {
				t.Errorf("expected no error, got: %v", err)
			}
		})
	}
}

// TestValidateGovParams_Invalid tests validation of invalid parameters.
func TestValidateGovParams_Invalid(t *testing.T) {
	testCases := []struct {
		name        string
		params      *ports.GovParams
		expectedErr string
	}{
		{
			name: "zero voting period",
			params: &ports.GovParams{
				VotingPeriod:          0,
				ExpeditedVotingPeriod: 0,
				MinDeposit:            "10000000",
			},
			expectedErr: "voting_period must be positive",
		},
		{
			name: "negative voting period",
			params: &ports.GovParams{
				VotingPeriod:          -1 * time.Hour,
				ExpeditedVotingPeriod: 0,
				MinDeposit:            "10000000",
			},
			expectedErr: "voting_period must be positive",
		},
		{
			name: "negative expedited voting period",
			params: &ports.GovParams{
				VotingPeriod:          48 * time.Hour,
				ExpeditedVotingPeriod: -1 * time.Hour,
				MinDeposit:            "10000000",
			},
			expectedErr: "expedited_voting_period cannot be negative",
		},
		{
			name: "expedited period equals regular period",
			params: &ports.GovParams{
				VotingPeriod:          48 * time.Hour,
				ExpeditedVotingPeriod: 48 * time.Hour,
				MinDeposit:            "10000000",
			},
			expectedErr: "expedited_voting_period",
		},
		{
			name: "expedited period greater than regular period",
			params: &ports.GovParams{
				VotingPeriod:          24 * time.Hour,
				ExpeditedVotingPeriod: 48 * time.Hour,
				MinDeposit:            "10000000",
			},
			expectedErr: "must be less than voting_period",
		},
		{
			name: "empty min deposit",
			params: &ports.GovParams{
				VotingPeriod:          48 * time.Hour,
				ExpeditedVotingPeriod: 24 * time.Hour,
				MinDeposit:            "",
			},
			expectedErr: "min_deposit is required",
		},
		{
			name: "invalid min deposit (non-numeric)",
			params: &ports.GovParams{
				VotingPeriod:          48 * time.Hour,
				ExpeditedVotingPeriod: 24 * time.Hour,
				MinDeposit:            "abc123",
			},
			expectedErr: "min_deposit must be a numeric string",
		},
		{
			name: "invalid expedited min deposit",
			params: &ports.GovParams{
				VotingPeriod:          48 * time.Hour,
				ExpeditedVotingPeriod: 24 * time.Hour,
				MinDeposit:            "10000000",
				ExpeditedMinDeposit:   "invalid",
			},
			expectedErr: "expedited_min_deposit must be a numeric string",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateGovParams(tc.params)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.expectedErr) {
				t.Errorf("expected error containing %q, got: %v", tc.expectedErr, err)
			}
		})
	}
}

// TestIsValidDepositAmount tests deposit amount validation.
func TestIsValidDepositAmount(t *testing.T) {
	testCases := []struct {
		amount string
		valid  bool
	}{
		// Valid amounts
		{"10000000", true},
		{"1", true},
		{"999999999999", true},
		{"10000000uatom", true},
		{"50000000stake", true},
		{"123456789abcdefg", true}, // Starts with digit, has suffix

		// Invalid amounts
		{"", false},
		{"abc", false},
		{"uatom10000000", false}, // Doesn't start with digit
		{"-10000000", false},     // Negative
		{" 10000000", false},     // Leading space
	}

	for _, tc := range testCases {
		t.Run(tc.amount, func(t *testing.T) {
			result := isValidDepositAmount(tc.amount)
			if result != tc.valid {
				t.Errorf("amount %q: expected valid=%v, got %v", tc.amount, tc.valid, result)
			}
		})
	}
}
