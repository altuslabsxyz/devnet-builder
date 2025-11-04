package main

import (
	"bufio"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	govtypesv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	"os"
	"path/filepath"
	"time"

	"cosmossdk.io/log"
	"cosmossdk.io/math"

	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdkkeyring "github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/cosmos/cosmos-sdk/testutil"
	"github.com/cosmos/cosmos-sdk/testutil/sims"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	evmhd "github.com/cosmos/evm/crypto/hd"
	evmostypes "github.com/cosmos/evm/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"github.com/stablelabs/stable/app"
	appcfg "github.com/stablelabs/stable/app/config"
	"github.com/stablelabs/stable/crypto/keyring"
	precompiletypes "github.com/stablelabs/stable/x/precompile/types"
	restrictiontypes "github.com/stablelabs/stable/x/restriction/types"
)

type DevnetGenerator struct {
	config     *DevnetConfig
	cdc        codec.Codec
	txConfig   client.TxConfig
	tempApp    *app.App
	validators []ValidatorInfo
	accounts   []AccountInfo
	logger     log.Logger
}

type ValidatorInfo struct {
	Moniker         string
	OperatorAddress sdk.ValAddress
	ConsensusKey    ValidatorKeyPair
	AccountAddress  sdk.AccAddress
	Tokens          math.Int
}

type ValidatorKeyPair struct {
	PrivKey cryptotypes.PrivKey
	PubKey  cryptotypes.PubKey
}

type AccountInfo struct {
	Name    string
	Address sdk.AccAddress
}

func NewDevnetGenerator(config *DevnetConfig, logger log.Logger) *DevnetGenerator {
	// Create a temporary minimal app to access BasicModuleManager and properly registered codec
	db := dbm.NewMemDB()
	appLogger := log.NewNopLogger() // Use NopLogger for internal app to reduce noise
	appOpts := sims.NewAppOptionsWithFlagHome(app.DefaultNodeHome)
	tempApp := app.NewApp(
		appLogger,
		db,
		nil,                      // traceStore
		false,                    // loadLatest
		appOpts,                  // appOpts
		appcfg.TestnetEVMChainID, // evmChainID
		appcfg.EvmAppOptions,     // evmAppOptions
	)

	return &DevnetGenerator{
		config:     config,
		cdc:        tempApp.AppCodec(),
		txConfig:   tempApp.TxConfig(),
		tempApp:    tempApp,
		validators: make([]ValidatorInfo, 0),
		accounts:   make([]AccountInfo, 0),
		logger:     logger,
	}
}

func (g *DevnetGenerator) Build(genesisFile string) error {
	appState, genDoc, err := genutiltypes.GenesisStateFromGenFile(genesisFile)
	if err != nil {
		return fmt.Errorf("failed to load genesis: %w", err)
	}

	// Use chain ID from genesis if not specified
	if g.config.ChainID == "" {
		g.config.ChainID = genDoc.ChainID
	} else {
		genDoc.ChainID = g.config.ChainID
	}

	// Create output directories first
	if err := os.MkdirAll(g.config.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}
	for i := 0; i < g.config.NumValidators; i++ {
		nodeDir := filepath.Join(g.config.OutputDir, fmt.Sprintf("node%d", i))
		if err := os.MkdirAll(nodeDir, 0755); err != nil {
			return fmt.Errorf("failed to create node%d directory: %w", i, err)
		}
	}
	accountsDir := filepath.Join(g.config.OutputDir, "accounts")
	if err := os.MkdirAll(accountsDir, 0755); err != nil {
		return fmt.Errorf("failed to create accounts directory: %w", err)
	}

	// Generate validators (creates keyrings in node directories)
	if err := g.generateValidators(); err != nil {
		return fmt.Errorf("failed to generate validators: %w", err)
	}

	// Generate accounts (creates keyring in accounts directory)
	if err := g.generateAccounts(); err != nil {
		return fmt.Errorf("failed to generate accounts: %w", err)
	}

	// Add validator keys to accounts keyring
	if err := g.addValidatorKeysToAccountsKeyring(); err != nil {
		return fmt.Errorf("failed to add validator keys: %w", err)
	}

	// Update genesis (only add accounts, keep validators from export)
	if err := g.updateGenesisAccounts(appState); err != nil {
		return fmt.Errorf("failed to update genesis: %w", err)
	}

	// Update consensus validators to nil (like testnet.go)
	if err := g.updateConsensusValidators(genDoc); err != nil {
		return fmt.Errorf("failed to update consensus validators: %w", err)
	}

	// Marshal app state back to genDoc
	appStateBytes, err := json.Marshal(appState)
	if err != nil {
		return fmt.Errorf("failed to marshal app state: %w", err)
	}
	genDoc.AppState = appStateBytes

	// Initialize node directories with validator keys
	if err := g.initNodeDirectories(); err != nil {
		return fmt.Errorf("failed to initialize node directories: %w", err)
	}

	// Validate genesis before saving
	if err := g.tempApp.BasicModuleManager.ValidateGenesis(g.cdc, g.txConfig, appState); err != nil {
		return fmt.Errorf("failed to validate genesis: %w", err)
	}

	// Collect and distribute final genesis to all nodes (like testnet.go:389)
	if err := g.collectGenFiles(genDoc); err != nil {
		return fmt.Errorf("failed to distribute genesis files: %w", err)
	}

	return nil
}

func (g *DevnetGenerator) generateValidators() error {
	inBuf := bufio.NewReader(os.Stdin)

	for i := 0; i < g.config.NumValidators; i++ {
		// Generate consensus key (ed25519)
		consPrivKey := ed25519.GenPrivKey()
		consPubKey := consPrivKey.PubKey()

		// Create keyring for this validator node
		nodeDir := filepath.Join(g.config.OutputDir, fmt.Sprintf("node%d", i))
		kr, err := sdkkeyring.New(
			sdk.KeyringServiceName(),
			sdkkeyring.BackendTest,
			nodeDir,
			inBuf,
			g.cdc,
			keyring.Option(),
		)
		if err != nil {
			return fmt.Errorf("failed to create keyring for node%d: %w", i, err)
		}

		// Get supported algorithms
		keyringAlgos, _ := kr.SupportedAlgorithms()
		algo, err := sdkkeyring.NewSigningAlgoFromString(string(evmhd.EthSecp256k1Type), keyringAlgos)
		if err != nil {
			return fmt.Errorf("failed to get signing algo: %w", err)
		}

		// Generate and save account key using testutil (similar to testnet.go:294)
		valName := fmt.Sprintf("validator%d", i)
		accAddr, _, err := testutil.GenerateSaveCoinKey(kr, valName, "", true, algo)
		if err != nil {
			return fmt.Errorf("failed to generate key for %s: %w", valName, err)
		}

		// Operator address derived from account address
		operatorAddr := sdk.ValAddress(accAddr)

		validator := ValidatorInfo{
			Moniker:         valName,
			OperatorAddress: operatorAddr,
			ConsensusKey: ValidatorKeyPair{
				PrivKey: consPrivKey,
				PubKey:  consPubKey,
			},
			AccountAddress: accAddr,
			Tokens:         g.config.ValidatorStake,
		}

		g.validators = append(g.validators, validator)
	}

	return nil
}

func (g *DevnetGenerator) generateAccounts() error {
	// Create keyring for accounts directory
	accountsDir := filepath.Join(g.config.OutputDir, "accounts")
	inBuf := bufio.NewReader(os.Stdin)

	kr, err := sdkkeyring.New(
		sdk.KeyringServiceName(),
		sdkkeyring.BackendTest,
		accountsDir,
		inBuf,
		g.cdc,
		keyring.Option(),
	)
	if err != nil {
		return fmt.Errorf("failed to create accounts keyring: %w", err)
	}

	// Get supported algorithms
	keyringAlgos, _ := kr.SupportedAlgorithms()
	algo, err := sdkkeyring.NewSigningAlgoFromString(string(evmhd.EthSecp256k1Type), keyringAlgos)
	if err != nil {
		return fmt.Errorf("failed to get signing algo: %w", err)
	}

	for i := 0; i < g.config.NumAccounts; i++ {
		accName := fmt.Sprintf("account%d", i)

		// Generate and save account key using testutil (similar to testnet.go:294)
		addr, _, err := testutil.GenerateSaveCoinKey(kr, accName, "", true, algo)
		if err != nil {
			return fmt.Errorf("failed to generate key for %s: %w", accName, err)
		}

		account := AccountInfo{
			Name:    accName,
			Address: addr,
		}

		g.accounts = append(g.accounts, account)
	}

	return nil
}

func (g *DevnetGenerator) updateGenesisAccounts(appState map[string]json.RawMessage) error {
	// Clear exported genesis and add new validators and dummy accounts

	var authState authtypes.GenesisState
	if err := g.cdc.UnmarshalJSON(appState[authtypes.ModuleName], &authState); err != nil {
		return fmt.Errorf("failed to unmarshal auth state: %w", err)
	}

	// Clear exported accounts
	// => if clear exported accounts, it throws "panic: account not found for address 0x00000000000001e541f0D090868FBe24b59Fbe06" error
	//authState.Accounts = make([]*codectypes.Any, 0)

	// Add validator accounts
	for _, val := range g.validators {
		baseAcc := &authtypes.BaseAccount{
			Address:       val.AccountAddress.String(),
			PubKey:        nil,
			AccountNumber: 0,
			Sequence:      0,
		}
		accAny, err := codectypes.NewAnyWithValue(baseAcc)
		if err != nil {
			return fmt.Errorf("failed to pack validator account: %w", err)
		}
		authState.Accounts = append(authState.Accounts, accAny)
	}

	// Add dummy accounts
	for _, acc := range g.accounts {
		baseAcc := &authtypes.BaseAccount{
			Address:       acc.Address.String(),
			PubKey:        nil,
			AccountNumber: 0,
			Sequence:      0,
		}
		accAny, err := codectypes.NewAnyWithValue(baseAcc)
		if err != nil {
			return fmt.Errorf("failed to pack account: %w", err)
		}
		authState.Accounts = append(authState.Accounts, accAny)
	}

	// Marshal auth state
	authStateBz, err := g.cdc.MarshalJSON(&authState)
	if err != nil {
		return fmt.Errorf("failed to marshal auth state: %w", err)
	}
	appState[authtypes.ModuleName] = authStateBz

	// Update bank balances for dummy accounts
	if err := g.updateBankBalances(appState); err != nil {
		return fmt.Errorf("failed to update bank balances: %w", err)
	}

	if err := g.updateGovState(appState); err != nil {
		return fmt.Errorf("failed to update gov state: %w", err)
	}

	// Update staking state with new validators
	if err := g.updateStakingState(appState); err != nil {
		return fmt.Errorf("failed to update staking state: %w", err)
	}

	// Update slashing state for new validators
	if err := g.updateSlashingState(appState); err != nil {
		return fmt.Errorf("failed to update slashing state: %w", err)
	}

	// Clear EVM state (accounts from exported genesis)
	//if err := g.clearEVMState(appState); err != nil {
	//	return fmt.Errorf("failed to clear EVM state: %w", err)
	//}

	// Clear restriction state (blocked addresses from exported genesis)
	//if err := g.clearRestrictionState(appState); err != nil {
	//	return fmt.Errorf("failed to clear restriction state: %w", err)
	//}

	// Clear precompile state (USDT0 address from exported genesis)
	//if err := g.clearPrecompileState(appState); err != nil {
	//	return fmt.Errorf("failed to clear precompile state: %w", err)
	//}

	return nil
}

func (g *DevnetGenerator) updateGovState(appState map[string]json.RawMessage) error {
	var govState govtypesv1.GenesisState
	if err := g.cdc.UnmarshalJSON(appState[govtypes.ModuleName], &govState); err != nil {
		return fmt.Errorf("failed to unmarshal gov state: %w", err)
	}

	votingPeriod := 6 * time.Minute
	govState.Params.VotingPeriod = &votingPeriod

	appState[govtypes.ModuleName] = g.cdc.MustMarshalJSON(&govState)
	return nil
}

func (g *DevnetGenerator) updateBankBalances(appState map[string]json.RawMessage) error {
	// Unmarshal using proper banktypes.GenesisState
	var bankState banktypes.GenesisState
	if err := g.cdc.UnmarshalJSON(appState[banktypes.ModuleName], &bankState); err != nil {
		return fmt.Errorf("failed to unmarshal bank state: %w", err)
	}

	// Calculate total staked amount (goes to bonded pool)
	totalBondedAmount := math.ZeroInt()
	for _, val := range g.validators {
		totalBondedAmount = totalBondedAmount.Add(val.Tokens)
	}

	// Use a map to ensure unique addresses (later entries overwrite earlier ones)
	balanceMap := make(map[string]banktypes.Balance)

	// Start with existing balances from exported genesis
	for _, balance := range bankState.Balances {
		balanceMap[balance.Address] = balance
	}

	// Add bonded pool module account balance (only astable, not agasusdt)
	// This will overwrite if it already exists
	bondedPoolAddr := authtypes.NewModuleAddress(stakingtypes.BondedPoolName)
	balanceMap[bondedPoolAddr.String()] = banktypes.Balance{
		Address: bondedPoolAddr.String(),
		Coins:   sdk.NewCoins(sdk.NewCoin(appcfg.GovAttoDenom, totalBondedAmount)),
	}

	notBondedPoolAddr := authtypes.NewModuleAddress(stakingtypes.NotBondedPoolName).String()

	var blockedAddrs = []string{notBondedPoolAddr}

	for _, blockedAddr := range blockedAddrs {
		delete(balanceMap, blockedAddr)
	}

	// Add balances for validators (will overwrite if addresses already exist)
	for _, val := range g.validators {
		balanceMap[val.AccountAddress.String()] = banktypes.Balance{
			Address: val.AccountAddress.String(),
			Coins:   g.config.ValidatorBalance, // Already sdk.Coins
		}
	}

	// Add balances for dummy accounts (will overwrite if addresses already exist)
	for _, acc := range g.accounts {
		balanceMap[acc.Address.String()] = banktypes.Balance{
			Address: acc.Address.String(),
			Coins:   g.config.AccountBalance, // Already sdk.Coins
		}
	}

	// Convert map back to slice
	bankState.Balances = make([]banktypes.Balance, 0, len(balanceMap))
	for _, balance := range balanceMap {
		bankState.Balances = append(bankState.Balances, balance)
	}

	supplyCoins := sdk.NewCoins()
	for _, balance := range balanceMap {
		for _, coin := range balance.Coins {
			supplyCoins = supplyCoins.Add(coin)
		}
	}

	// Replace supply with new totals
	bankState.Supply = supplyCoins

	// Marshal bank state back
	bankStateBz, err := g.cdc.MarshalJSON(&bankState)
	if err != nil {
		return fmt.Errorf("failed to marshal bank state: %w", err)
	}
	appState[banktypes.ModuleName] = bankStateBz

	return nil
}

func (g *DevnetGenerator) updateStakingState(appState map[string]json.RawMessage) error {
	// Unmarshal staking genesis state
	var stakingState stakingtypes.GenesisState
	if err := g.cdc.UnmarshalJSON(appState[stakingtypes.ModuleName], &stakingState); err != nil {
		return fmt.Errorf("failed to unmarshal staking state: %w", err)
	}

	// Clear exported validators and replace with new ones
	stakingState.Validators = make([]stakingtypes.Validator, 0)
	stakingState.LastValidatorPowers = make([]stakingtypes.LastValidatorPower, 0)
	stakingState.Delegations = make([]stakingtypes.Delegation, 0)
	stakingState.UnbondingDelegations = make([]stakingtypes.UnbondingDelegation, 0)
	stakingState.Redelegations = make([]stakingtypes.Redelegation, 0)
	stakingState.Exported = false

	// Create new validators with self-delegations
	for _, val := range g.validators {
		// Pack consensus pubkey
		pkAny, err := codectypes.NewAnyWithValue(val.ConsensusKey.PubKey)
		if err != nil {
			return fmt.Errorf("failed to pack consensus pubkey: %w", err)
		}

		// Create validator
		validator := stakingtypes.Validator{
			OperatorAddress: val.OperatorAddress.String(),
			ConsensusPubkey: pkAny,
			Jailed:          false,
			Status:          stakingtypes.Bonded,
			Tokens:          val.Tokens,
			DelegatorShares: math.LegacyNewDecFromInt(val.Tokens),
			Description: stakingtypes.Description{
				Moniker: val.Moniker,
			},
			UnbondingHeight: 0,
			UnbondingTime:   time.Unix(0, 0).UTC(),
			Commission: stakingtypes.Commission{
				CommissionRates: stakingtypes.CommissionRates{
					Rate:          math.LegacyZeroDec(),
					MaxRate:       math.LegacyOneDec(),
					MaxChangeRate: math.LegacyOneDec(),
				},
				UpdateTime: time.Unix(0, 0).UTC(),
			},
			MinSelfDelegation: math.OneInt(),
		}

		stakingState.Validators = append(stakingState.Validators, validator)

		// Create self-delegation
		delegation := stakingtypes.Delegation{
			DelegatorAddress: val.AccountAddress.String(),
			ValidatorAddress: val.OperatorAddress.String(),
			Shares:           math.LegacyNewDecFromInt(val.Tokens),
		}
		stakingState.Delegations = append(stakingState.Delegations, delegation)

		// Create LastValidatorPower entry
		// Calculate consensus power from tokens using AttoPowerReduction
		consensusPower := sdk.TokensToConsensusPower(val.Tokens, evmostypes.AttoPowerReduction)
		lastValPower := stakingtypes.LastValidatorPower{
			Address: val.OperatorAddress.String(),
			Power:   consensusPower,
		}
		stakingState.LastValidatorPowers = append(stakingState.LastValidatorPowers, lastValPower)
	}

	// Marshal staking state back
	stakingStateBz, err := g.cdc.MarshalJSON(&stakingState)
	if err != nil {
		return fmt.Errorf("failed to marshal staking state: %w", err)
	}
	appState[stakingtypes.ModuleName] = stakingStateBz

	return nil
}

func (g *DevnetGenerator) updateSlashingState(appState map[string]json.RawMessage) error {
	// Unmarshal slashing genesis state
	var slashingState slashingtypes.GenesisState
	if err := g.cdc.UnmarshalJSON(appState[slashingtypes.ModuleName], &slashingState); err != nil {
		return fmt.Errorf("failed to unmarshal slashing state: %w", err)
	}

	// Clear exported signing infos and missed blocks
	slashingState.SigningInfos = make([]slashingtypes.SigningInfo, 0)
	slashingState.MissedBlocks = make([]slashingtypes.ValidatorMissedBlocks, 0)

	// Create signing info for each new validator
	for _, val := range g.validators {
		// Get consensus address from consensus pubkey
		consAddr := sdk.ConsAddress(val.ConsensusKey.PubKey.Address())

		// Create ValidatorSigningInfo with initial values
		signingInfo := slashingtypes.SigningInfo{
			Address: consAddr.String(),
			ValidatorSigningInfo: slashingtypes.ValidatorSigningInfo{
				Address:             consAddr.String(),
				StartHeight:         0,
				IndexOffset:         0,
				JailedUntil:         time.Unix(0, 0).UTC(),
				Tombstoned:          false,
				MissedBlocksCounter: 0,
			},
		}

		slashingState.SigningInfos = append(slashingState.SigningInfos, signingInfo)

		// Create empty missed blocks entry for this validator
		missedBlocks := slashingtypes.ValidatorMissedBlocks{
			Address:      consAddr.String(),
			MissedBlocks: make([]slashingtypes.MissedBlock, 0),
		}

		slashingState.MissedBlocks = append(slashingState.MissedBlocks, missedBlocks)
	}

	// Marshal slashing state back
	slashingStateBz, err := g.cdc.MarshalJSON(&slashingState)
	if err != nil {
		return fmt.Errorf("failed to marshal slashing state: %w", err)
	}
	appState[slashingtypes.ModuleName] = slashingStateBz

	return nil
}

func (g *DevnetGenerator) clearEVMState(appState map[string]json.RawMessage) error {
	// Unmarshal EVM genesis state
	var evmState evmtypes.GenesisState
	if err := g.cdc.UnmarshalJSON(appState[evmtypes.ModuleName], &evmState); err != nil {
		return fmt.Errorf("failed to unmarshal EVM state: %w", err)
	}

	// Clear exported EVM accounts
	evmState.Accounts = make([]evmtypes.GenesisAccount, 0)

	// Marshal EVM state back
	evmStateBz, err := g.cdc.MarshalJSON(&evmState)
	if err != nil {
		return fmt.Errorf("failed to marshal EVM state: %w", err)
	}
	appState[evmtypes.ModuleName] = evmStateBz

	return nil
}

func (g *DevnetGenerator) clearRestrictionState(appState map[string]json.RawMessage) error {
	// Unmarshal restriction genesis state
	var restrictionState restrictiontypes.GenesisState
	if err := g.cdc.UnmarshalJSON(appState[restrictiontypes.ModuleName], &restrictionState); err != nil {
		return fmt.Errorf("failed to unmarshal restriction state: %w", err)
	}

	// Clear exported restriction lists
	restrictionState.SanctionsList = make([]string, 0)
	restrictionState.WhiteList = make([]restrictiontypes.ListedAddress, 0)
	restrictionState.BlockList = make([]restrictiontypes.ListedAddress, 0)

	// Clear contract addresses in params to prevent USDT restriction checks
	restrictionState.Params.SanctionsListContractAddr = ""
	restrictionState.Params.BlocklistWhitelistContractAddrs = make([]string, 0)

	// Marshal restriction state back
	restrictionStateBz, err := g.cdc.MarshalJSON(&restrictionState)
	if err != nil {
		return fmt.Errorf("failed to marshal restriction state: %w", err)
	}
	appState[restrictiontypes.ModuleName] = restrictionStateBz

	return nil
}

func (g *DevnetGenerator) clearPrecompileState(appState map[string]json.RawMessage) error {
	// Unmarshal precompile genesis state
	var precompileState precompiletypes.GenesisState
	if err := g.cdc.UnmarshalJSON(appState[precompiletypes.ModuleName], &precompileState); err != nil {
		return fmt.Errorf("failed to unmarshal precompile state: %w", err)
	}

	// Clear USDT0 token address to prevent USDT restriction checks
	precompileState.Usdt0TokenAddress = ""

	// Marshal precompile state back
	precompileStateBz, err := g.cdc.MarshalJSON(&precompileState)
	if err != nil {
		return fmt.Errorf("failed to marshal precompile state: %w", err)
	}
	appState[precompiletypes.ModuleName] = precompileStateBz

	return nil
}

func (g *DevnetGenerator) updateConsensusValidators(genDoc *genutiltypes.AppGenesis) error {
	// Set validators to nil (like testnet.go:516)
	// This allows the chain to initialize validators from staking module genesis state
	if genDoc.Consensus != nil {
		genDoc.Consensus.Validators = nil
	}
	return nil
}

// initNodeDirectories initializes node directories and validator keys (but not genesis)
func (g *DevnetGenerator) initNodeDirectories() error {
	// Create main devnet directory
	if err := os.MkdirAll(g.config.OutputDir, 0755); err != nil {
		return err
	}

	// Create node directories and save validator keys
	for i, val := range g.validators {
		nodeDir := filepath.Join(g.config.OutputDir, fmt.Sprintf("node%d", i))
		configDir := filepath.Join(nodeDir, "config")
		dataDir := filepath.Join(nodeDir, "data")

		if err := os.MkdirAll(configDir, 0755); err != nil {
			return err
		}
		if err := os.MkdirAll(dataDir, 0755); err != nil {
			return err
		}

		// Save priv_validator_key.json
		if err := g.savePrivValidatorKey(configDir, val.ConsensusKey); err != nil {
			return err
		}

		// Save priv_validator_state.json
		if err := g.savePrivValidatorState(dataDir); err != nil {
			return err
		}

		// Note: genesis.json is NOT saved here - it will be saved by collectGenFiles
	}

	// Create accounts directory
	accountsDir := filepath.Join(g.config.OutputDir, "accounts")
	if err := os.MkdirAll(accountsDir, 0755); err != nil {
		return err
	}

	return nil
}

// collectGenFiles distributes the final genesis file to all validator nodes
// Similar to testnet.go:389, but without gentx processing since we use exported genesis
func (g *DevnetGenerator) collectGenFiles(genDoc *genutiltypes.AppGenesis) error {
	// Save the final genesis to each validator node
	for i := range g.validators {
		nodeDir := filepath.Join(g.config.OutputDir, fmt.Sprintf("node%d", i))
		configDir := filepath.Join(nodeDir, "config")

		// Save genesis.json
		if err := g.saveGenesis(configDir, genDoc); err != nil {
			return fmt.Errorf("failed to save genesis for node%d: %w", i, err)
		}
	}

	return nil
}

func (g *DevnetGenerator) savePrivValidatorKey(configDir string, keyPair ValidatorKeyPair) error {
	address := keyPair.PubKey.Address()

	privValKey := map[string]interface{}{
		"address": hex.EncodeToString(address),
		"pub_key": map[string]string{
			"type":  "tendermint/PubKeyEd25519",
			"value": base64.StdEncoding.EncodeToString(keyPair.PubKey.Bytes()),
		},
		"priv_key": map[string]string{
			"type":  "tendermint/PrivKeyEd25519",
			"value": base64.StdEncoding.EncodeToString(keyPair.PrivKey.Bytes()),
		},
	}

	data, err := json.MarshalIndent(privValKey, "", "  ")
	if err != nil {
		return err
	}

	filename := filepath.Join(configDir, "priv_validator_key.json")
	return os.WriteFile(filename, data, 0600)
}

func (g *DevnetGenerator) savePrivValidatorState(dataDir string) error {
	state := map[string]interface{}{
		"height": "0",
		"round":  0,
		"step":   0,
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	filename := filepath.Join(dataDir, "priv_validator_state.json")
	return os.WriteFile(filename, data, 0644)
}

func (g *DevnetGenerator) saveGenesis(configDir string, genDoc *genutiltypes.AppGenesis) error {
	// Marshal consensus directly
	consensusBytes, err := json.Marshal(genDoc.Consensus)
	if err != nil {
		return err
	}

	// Build final genesis structure
	finalGenesis := map[string]interface{}{
		"genesis_time":   genDoc.GenesisTime.Format("2006-01-02T15:04:05.999999999Z07:00"),
		"chain_id":       genDoc.ChainID,
		"initial_height": fmt.Sprintf("%d", genDoc.InitialHeight),
		"consensus":      json.RawMessage(consensusBytes),
		"app_hash":       "",
		"app_state":      json.RawMessage(genDoc.AppState),
	}

	data, err := json.MarshalIndent(finalGenesis, "", "  ")
	if err != nil {
		return err
	}

	filename := filepath.Join(configDir, "genesis.json")
	return os.WriteFile(filename, data, 0644)
}

func (g *DevnetGenerator) addValidatorKeysToAccountsKeyring() error {
	// Open keyring for accounts directory
	accountsDir := filepath.Join(g.config.OutputDir, "accounts")
	inBuf := bufio.NewReader(os.Stdin)

	accountsKr, err := sdkkeyring.New(
		sdk.KeyringServiceName(),
		sdkkeyring.BackendTest,
		accountsDir,
		inBuf,
		g.cdc,
		keyring.Option(),
	)
	if err != nil {
		return fmt.Errorf("failed to open accounts keyring: %w", err)
	}

	// Export each validator key from its node keyring and import to accounts keyring
	for i, val := range g.validators {
		nodeDir := filepath.Join(g.config.OutputDir, fmt.Sprintf("node%d", i))

		// Open the validator's node keyring
		nodeKr, err := sdkkeyring.New(
			sdk.KeyringServiceName(),
			sdkkeyring.BackendTest,
			nodeDir,
			inBuf,
			g.cdc,
			keyring.Option(),
		)
		if err != nil {
			return fmt.Errorf("failed to open keyring for node%d: %w", i, err)
		}

		// Export the key in armor format
		armoredKey, err := nodeKr.ExportPrivKeyArmor(val.Moniker, "")
		if err != nil {
			return fmt.Errorf("failed to export key %s: %w", val.Moniker, err)
		}

		// Delete existing key if it exists (ignore error if key doesn't exist)
		_ = accountsKr.Delete(val.Moniker)

		// Import to accounts keyring
		if err := accountsKr.ImportPrivKey(val.Moniker, armoredKey, ""); err != nil {
			return fmt.Errorf("failed to import key %s to accounts keyring: %w", val.Moniker, err)
		}
	}

	return nil
}
