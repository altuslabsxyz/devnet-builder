// pkg/network/cosmos/msgs.go
package cosmos

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	"github.com/altuslabsxyz/devnet-builder/pkg/network"
)

// GovVotePayload contains the fields for a governance vote transaction.
type GovVotePayload struct {
	// ProposalID is the unique identifier of the proposal.
	ProposalID uint64 `json:"proposal_id"`
	// Option is the vote option: "yes", "no", "abstain", or "no_with_veto".
	Option string `json:"option"`
}

// BankSendPayload contains the fields for a bank send transaction.
type BankSendPayload struct {
	// ToAddress is the recipient's bech32 address.
	ToAddress string `json:"to_address"`
	// Amount is the amount to send (e.g., "1000stake").
	Amount string `json:"amount"`
}

// StakingDelegatePayload contains the fields for a staking delegate transaction.
type StakingDelegatePayload struct {
	// ValidatorAddress is the validator's bech32 operator address.
	ValidatorAddress string `json:"validator_address"`
	// Amount is the amount to delegate (e.g., "1000stake").
	Amount string `json:"amount"`
}

// BuildMessage creates an SDK message from the given transaction type and payload.
// It returns the appropriate message type based on TxType.
func BuildMessage(txType network.TxType, sender string, payload json.RawMessage) (sdk.Msg, error) {
	switch txType {
	case network.TxTypeGovVote:
		return buildGovVoteMsg(sender, payload)
	case network.TxTypeBankSend:
		return buildBankSendMsg(sender, payload)
	case network.TxTypeStakingDelegate:
		return buildStakingDelegateMsg(sender, payload)
	default:
		return nil, fmt.Errorf("unsupported transaction type: %s", txType)
	}
}

// buildGovVoteMsg creates a governance vote message.
func buildGovVoteMsg(voter string, payload json.RawMessage) (sdk.Msg, error) {
	var p GovVotePayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, fmt.Errorf("failed to unmarshal gov vote payload: %w", err)
	}

	option, err := parseVoteOption(p.Option)
	if err != nil {
		return nil, err
	}

	msg := &govtypes.MsgVote{
		ProposalId: p.ProposalID,
		Voter:      voter,
		Option:     option,
		Metadata:   "",
	}

	return msg, nil
}

// buildBankSendMsg creates a bank send message.
func buildBankSendMsg(sender string, payload json.RawMessage) (sdk.Msg, error) {
	var p BankSendPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, fmt.Errorf("failed to unmarshal bank send payload: %w", err)
	}

	coin, err := ParseAmount(p.Amount)
	if err != nil {
		return nil, fmt.Errorf("failed to parse amount: %w", err)
	}

	msg := &banktypes.MsgSend{
		FromAddress: sender,
		ToAddress:   p.ToAddress,
		Amount:      sdk.Coins{coin},
	}

	return msg, nil
}

// buildStakingDelegateMsg creates a staking delegate message.
func buildStakingDelegateMsg(delegator string, payload json.RawMessage) (sdk.Msg, error) {
	var p StakingDelegatePayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil, fmt.Errorf("failed to unmarshal staking delegate payload: %w", err)
	}

	coin, err := ParseAmount(p.Amount)
	if err != nil {
		return nil, fmt.Errorf("failed to parse amount: %w", err)
	}

	msg := &stakingtypes.MsgDelegate{
		DelegatorAddress: delegator,
		ValidatorAddress: p.ValidatorAddress,
		Amount:           coin,
	}

	return msg, nil
}

// parseVoteOption converts a string vote option to the governance VoteOption type.
func parseVoteOption(opt string) (govtypes.VoteOption, error) {
	switch strings.ToLower(opt) {
	case "yes":
		return govtypes.OptionYes, nil
	case "abstain":
		return govtypes.OptionAbstain, nil
	case "no":
		return govtypes.OptionNo, nil
	case "no_with_veto", "nowithveto":
		return govtypes.OptionNoWithVeto, nil
	default:
		return govtypes.OptionEmpty, fmt.Errorf("invalid vote option: %s (valid options: yes, no, abstain, no_with_veto)", opt)
	}
}

// ParseGasPrice parses a gas price string like "0.025stake" into a DecCoin.
func ParseGasPrice(s string) (sdk.DecCoin, error) {
	if s == "" {
		return sdk.DecCoin{}, fmt.Errorf("gas price cannot be empty")
	}

	// Parse the gas price string (e.g., "0.025stake")
	// The format is <amount><denom> where amount can be decimal
	re := regexp.MustCompile(`^(\d+\.?\d*|\.\d+)([a-zA-Z][a-zA-Z0-9/]*)$`)
	matches := re.FindStringSubmatch(s)
	if len(matches) != 3 {
		return sdk.DecCoin{}, fmt.Errorf("invalid gas price format: %s (expected format like '0.025stake')", s)
	}

	amountStr := matches[1]
	denom := matches[2]

	amount, err := sdkmath.LegacyNewDecFromStr(amountStr)
	if err != nil {
		return sdk.DecCoin{}, fmt.Errorf("failed to parse gas price amount: %w", err)
	}

	return sdk.NewDecCoinFromDec(denom, amount), nil
}

// ParseAmount parses an amount string like "1000stake" into a Coin.
func ParseAmount(s string) (sdk.Coin, error) {
	if s == "" {
		return sdk.Coin{}, fmt.Errorf("amount cannot be empty")
	}

	// Parse the amount string (e.g., "1000stake")
	// The format is <amount><denom> where amount is an integer
	re := regexp.MustCompile(`^(\d+)([a-zA-Z][a-zA-Z0-9/]*)$`)
	matches := re.FindStringSubmatch(s)
	if len(matches) != 3 {
		return sdk.Coin{}, fmt.Errorf("invalid amount format: %s (expected format like '1000stake')", s)
	}

	amountStr := matches[1]
	denom := matches[2]

	amount, ok := sdkmath.NewIntFromString(amountStr)
	if !ok {
		return sdk.Coin{}, fmt.Errorf("failed to parse amount: %s", amountStr)
	}

	return sdk.NewCoin(denom, amount), nil
}
