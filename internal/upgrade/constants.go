package upgrade

import "time"

const (
	// GovPrecompileAddress is the EVM address of the governance precompile.
	GovPrecompileAddress = "0x0000000000000000000000000000000000000805"

	// GovAuthority is the governance module account address.
	GovAuthority = "stable10d07y265gmmuvt4z0w9aw880jnsr700jjjzdw5"

	// DefaultVotingPeriod is the default expedited voting period.
	DefaultVotingPeriod = 60 * time.Second

	// MinVotingPeriod is the minimum allowed voting period.
	MinVotingPeriod = 30 * time.Second

	// DefaultHeightBuffer is the default number of blocks to add after voting.
	DefaultHeightBuffer = 10

	// MinHeightBuffer is the minimum allowed height buffer.
	MinHeightBuffer = 5

	// DefaultDepositAmount is the deposit amount in astable (50001 STABLE).
	DefaultDepositAmount = "50000000000000000000001"

	// DefaultDepositDenom is the denomination for deposit.
	DefaultDepositDenom = "astable"

	// BlockPollInterval is how often to poll for block height.
	BlockPollInterval = 2 * time.Second

	// UpgradeTimeout is the maximum time to wait for upgrade height.
	UpgradeTimeout = 30 * time.Minute

	// PostUpgradeTimeout is the maximum time to wait for chain to resume.
	PostUpgradeTimeout = 5 * time.Minute

	// ChainHaltDetectionInterval is how long to wait between halt checks.
	ChainHaltDetectionInterval = 5 * time.Second

	// ChainHaltThreshold is how many consecutive unchanged heights indicate halt.
	ChainHaltThreshold = 3

	// VoteOptionYes is the vote option for YES.
	VoteOptionYes = 1

	// DefaultGasLimit is the default gas limit for EVM transactions.
	DefaultGasLimit = uint64(500000)

	// DefaultRPCPort is the default CometBFT RPC port.
	DefaultRPCPort = 26657

	// DefaultEVMPort is the default EVM JSON-RPC port.
	DefaultEVMPort = 8545
)
