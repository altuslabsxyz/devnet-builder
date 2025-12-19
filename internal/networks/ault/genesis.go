package ault

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/stablelabs/stable-devnet/internal/network"
)

// DefaultGenesisConfig returns the default genesis configuration for Ault.
func DefaultGenesisConfig() network.GenesisConfig {
	return network.GenesisConfig{
		// Chain identity
		ChainIDPattern: "ault_{evmid}-1",
		EVMChainID:     EVMChainIDDevnet,

		// Token configuration
		BaseDenom:     DefaultBaseDenom,
		DenomExponent: 18,
		DisplayDenom:  DefaultDisplayDenom,

		// Staking parameters (using fast devnet settings)
		BondDenom:         DefaultBaseDenom,
		MinSelfDelegation: "1",
		UnbondingTime:     10 * time.Second, // Fast for devnet
		MaxValidators:     100,

		// Governance parameters (using fast devnet settings)
		MinDeposit:       "10000000" + DefaultBaseDenom, // 10 AULT (in aault)
		VotingPeriod:     10 * time.Second,              // Fast for devnet
		MaxDepositPeriod: 10 * time.Second,              // Fast for devnet

		// Bank/distribution
		CommunityTax: "0.02",
	}
}

// ModifyGenesis applies Ault-specific modifications to a genesis file.
func ModifyGenesis(genesis []byte, opts network.GenesisOptions) ([]byte, error) {
	var genDoc map[string]interface{}
	if err := json.Unmarshal(genesis, &genDoc); err != nil {
		return nil, fmt.Errorf("failed to parse genesis: %w", err)
	}

	// Override chain ID if specified
	if opts.ChainID != "" {
		genDoc["chain_id"] = opts.ChainID
	}

	appState, ok := genDoc["app_state"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid genesis: missing or invalid app_state")
	}

	// Apply governance parameter overrides
	if opts.GovParams != nil {
		if err := applyGovParams(appState, opts.GovParams); err != nil {
			return nil, fmt.Errorf("failed to apply gov params: %w", err)
		}
	}

	// Apply staking parameter overrides
	if opts.StakingParams != nil {
		if err := applyStakingParams(appState, opts.StakingParams); err != nil {
			return nil, fmt.Errorf("failed to apply staking params: %w", err)
		}
	}

	// Apply Ault-specific module modifications (miner, license, etc.)
	// These modules are specific to Ault and may need special handling
	if err := applyAultModules(appState, opts); err != nil {
		return nil, fmt.Errorf("failed to apply ault modules: %w", err)
	}

	return json.Marshal(genDoc)
}

// applyGovParams applies governance parameter overrides to app_state.
func applyGovParams(appState map[string]interface{}, params *network.GovParamsOverride) error {
	gov, ok := appState["gov"].(map[string]interface{})
	if !ok {
		return nil // gov module not present
	}

	govParams, ok := gov["params"].(map[string]interface{})
	if !ok {
		return nil // params not present
	}

	if params.VotingPeriod != nil {
		govParams["voting_period"] = fmt.Sprintf("%dns", params.VotingPeriod.Nanoseconds())
	}

	if params.MaxDepositPeriod != nil {
		govParams["max_deposit_period"] = fmt.Sprintf("%dns", params.MaxDepositPeriod.Nanoseconds())
	}

	if params.MinDeposit != nil {
		// MinDeposit is a coin array
		govParams["min_deposit"] = []map[string]string{
			{"denom": DefaultBaseDenom, "amount": *params.MinDeposit},
		}
	}

	return nil
}

// applyStakingParams applies staking parameter overrides to app_state.
func applyStakingParams(appState map[string]interface{}, params *network.StakingParamsOverride) error {
	staking, ok := appState["staking"].(map[string]interface{})
	if !ok {
		return nil // staking module not present
	}

	stakingParams, ok := staking["params"].(map[string]interface{})
	if !ok {
		return nil // params not present
	}

	if params.UnbondingTime != nil {
		stakingParams["unbonding_time"] = fmt.Sprintf("%dns", params.UnbondingTime.Nanoseconds())
	}

	if params.MaxValidators != nil {
		stakingParams["max_validators"] = *params.MaxValidators
	}

	if params.MinSelfDelegation != nil {
		stakingParams["min_self_delegation"] = *params.MinSelfDelegation
	}

	return nil
}

// applyAultModules applies Ault-specific module configurations.
// Ault has custom modules like miner and license that may need special handling.
func applyAultModules(appState map[string]interface{}, opts network.GenesisOptions) error {
	// The miner module handles mining rewards and hash submissions
	// For devnet, we may want to configure specific parameters

	// Currently, we don't modify these modules in devnet mode
	// Add specific modifications here as needed for Ault's custom modules

	return nil
}
