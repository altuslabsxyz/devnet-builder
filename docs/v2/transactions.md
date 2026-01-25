# Transaction System Guide

Complete guide to building, signing, and broadcasting transactions with Devnet Builder v2.

## Overview

The transaction system provides a unified interface for submitting transactions across different blockchain platforms (Cosmos SDK, EVM, etc.). Transactions go through a multi-phase lifecycle managed by the TxController.

## Transaction Lifecycle

```
Pending → Building → Signing → Submitted → Confirmed
                                      ↓
                                   Failed
```

### Phases

1. **Pending** - Transaction created, waiting for builder
2. **Building** - Constructing unsigned transaction
3. **Signing** - Signing with validator key
4. **Submitted** - Broadcast to network
5. **Confirmed** - Included in block
6. **Failed** - Error occurred (retryable or permanent)

## Transaction Types

### Cosmos SDK Transactions

#### Bank Send

Transfer tokens between accounts:

```bash
dvb tx submit mydevnet \
  --type bank/send \
  --signer validator:0 \
  --payload '{
    "to_address": "cosmos1abc...",
    "amount": "1000000uatom"
  }'
```

#### Governance Proposal

Submit governance proposal:

```bash
dvb tx submit mydevnet \
  --type gov/proposal \
  --signer validator:0 \
  --payload '{
    "title": "Enable IBC",
    "description": "Proposal to enable IBC transfers",
    "deposit": "10000000uatom",
    "proposal_type": "text"
  }'
```

#### Governance Vote

Vote on proposal:

```bash
dvb tx submit mydevnet \
  --type gov/vote \
  --signer validator:0 \
  --payload '{
    "proposal_id": 1,
    "option": "yes"
  }'
```

#### Staking Delegate

Delegate tokens to validator:

```bash
dvb tx submit mydevnet \
  --type staking/delegate \
  --signer validator:0 \
  --payload '{
    "validator_address": "cosmosvaloper1...",
    "amount": "1000000uatom"
  }'
```

#### Staking Unbond

Unbond tokens from validator:

```bash
dvb tx submit mydevnet \
  --type staking/unbond \
  --signer validator:0 \
  --payload '{
    "validator_address": "cosmosvaloper1...",
    "amount": "500000uatom"
  }'
```

#### IBC Transfer

Transfer tokens via IBC:

```bash
dvb tx submit mydevnet \
  --type ibc/transfer \
  --signer validator:0 \
  --payload '{
    "receiver": "cosmos1...",
    "amount": "1000000uatom",
    "source_channel": "channel-0",
    "timeout_height": {"revision_number": 1, "revision_height": 1000}
  }'
```

### EVM Transactions

#### Native Transfer

Transfer native tokens (ETH, etc.):

```bash
dvb tx submit eth-devnet \
  --type bank/send \
  --signer validator:0 \
  --payload '{
    "to_address": "0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb0",
    "amount": "1000000000000000000"
  }'
```

#### Contract Call

Call smart contract:

```bash
dvb tx submit eth-devnet \
  --type contract/call \
  --signer validator:0 \
  --payload '{
    "contract_address": "0x...",
    "method": "transfer",
    "args": ["0x...", "1000"]
  }'
```

## Payload Formats

### Cosmos SDK Payloads

Payloads match the Cosmos SDK message structure:

```json
{
  // Bank Send (MsgSend)
  "to_address": "cosmos1...",
  "amount": "1000000uatom"
}

{
  // Gov Proposal (MsgSubmitProposal)
  "title": "Title",
  "description": "Description",
  "deposit": "10000000uatom",
  "proposal_type": "text"
}

{
  // Gov Vote (MsgVote)
  "proposal_id": 1,
  "option": "yes"  // yes, no, abstain, no_with_veto
}

{
  // Staking Delegate (MsgDelegate)
  "validator_address": "cosmosvaloper1...",
  "amount": "1000000uatom"
}
```

### EVM Payloads

EVM payloads use hex-encoded data:

```json
{
  // Native transfer
  "to_address": "0x742d35Cc6634C0532925a3b844Bc9e7595f0bEb0",
  "amount": "1000000000000000000"  // 1 ETH in wei
}

{
  // Contract call
  "contract_address": "0x...",
  "method": "transfer",
  "args": ["0x...", "1000"],
  "value": "0"  // Optional ETH to send
}
```

## Signers

Signers identify which key to use for signing:

### Validator Reference

```bash
# By index
--signer validator:0
--signer validator:1

# By validator address
--signer cosmosvaloper1abc...
```

### Full Node Reference

```bash
--signer fullnode:0
```

### Custom Address

```bash
--signer cosmos1abc...
```

## Gas and Fees

### Auto Gas Estimation

Default behavior estimates gas automatically:

```bash
dvb tx submit mydevnet \
  --type bank/send \
  --signer validator:0 \
  --payload @tx.json
# Gas automatically estimated
```

### Manual Gas Limit

Override with explicit limit:

```bash
dvb tx submit mydevnet \
  --type bank/send \
  --signer validator:0 \
  --payload @tx.json \
  --gas-limit 200000
```

### Gas Price

Set custom gas price (Cosmos SDK):

```bash
dvb tx submit mydevnet \
  --type bank/send \
  --signer validator:0 \
  --payload @tx.json \
  --gas-price 0.025uatom
```

## Memos

Transaction memos are useful for tagging or tracking:

```bash
dvb tx submit mydevnet \
  --type bank/send \
  --signer validator:0 \
  --payload @tx.json \
  --memo "reward-tag:pool-alpha"
```

Common uses:
- Validator reward distribution tags
- Exchange deposit identifiers
- Application-specific metadata

## Batch Transactions

Submit multiple transactions:

```bash
dvb tx submit-batch mydevnet @batch.json

# batch.json
[
  {
    "type": "bank/send",
    "signer": "validator:0",
    "payload": {...}
  },
  {
    "type": "gov/vote",
    "signer": "validator:1",
    "payload": {...}
  }
]
```

## Transaction Monitoring

### Real-time Watching

```bash
dvb tx submit mydevnet \
  --type bank/send \
  --signer validator:0 \
  --payload @tx.json \
  --watch

# Output:
# [10:15:30] Phase: Pending
# [10:15:31] Phase: Building
# [10:15:32] Phase: Signing
# [10:15:33] Phase: Submitted (TxHash: 0xABCD...)
# [10:15:35] Phase: Confirmed (Height: 150)
```

### Status Check

```bash
dvb tx status tx-12345

# Output:
# Transaction: tx-12345
# Status: Confirmed
# TxHash: 0xABCD...
# Height: 150
# GasUsed: 85000/100000
```

### List Transactions

```bash
dvb tx list mydevnet --status confirmed --limit 10
```

## Error Handling

### Retryable Errors

TxController automatically retries transient errors:

- Network timeouts
- Sequence mismatch
- Account not found (waits for account)
- Insufficient gas (increases gas)

### Permanent Errors

Non-retryable errors fail immediately:

- Invalid payload
- Insufficient funds
- Invalid signature
- Unauthorized operation

### Error Messages

```bash
dvb tx status tx-failed

# Output:
# Transaction: tx-failed
# Status: Failed
# Error: insufficient funds: 5000uatom < 1000000uatom
# Phase: Building
# Attempts: 3
```

## SDK Version Awareness

TxBuilder automatically adapts to SDK version:

```go
// Controller detects SDK version from devnet
sdkVersion := devnet.Status.SDKVersion

// Creates appropriate TxBuilder
builder, err := runtime.GetTxBuilder(ctx, devnetName)

// Builder knows which SDK version to use
unsignedTx, err := builder.BuildTx(ctx, req)
```

After upgrades, new transactions use the updated SDK version:

```bash
# Before upgrade: SDK v0.47
dvb tx submit mydevnet --type gov/vote ...  # Uses v0.47 builder

# Perform upgrade to v0.50
dvb upgrade create mydevnet --upgrade-name v2 ...

# After upgrade: SDK v0.50
dvb tx submit mydevnet --type gov/vote ...  # Uses v0.50 builder (automatic)
```

## Advanced Usage

### Custom Payload Files

```bash
# proposal.json
{
  "title": "Enable Module X",
  "description": "This proposal enables module X for enhanced functionality",
  "deposit": "10000000uatom",
  "proposal_type": "text"
}

dvb tx submit mydevnet \
  --type gov/proposal \
  --signer validator:0 \
  --payload @proposal.json
```

### Transaction Templates

```bash
# Save as template
dvb tx submit mydevnet \
  --type bank/send \
  --signer validator:0 \
  --payload @tx.json \
  --save-template bank-send-template

# Reuse template
dvb tx submit mydevnet \
  --template bank-send-template \
  --payload '{"to_address":"cosmos1...","amount":"5000000uatom"}'
```

### Programmatic Access (gRPC)

```go
import pb "github.com/altuslabsxyz/devnet-builder/api/proto/v1"

client := pb.NewTransactionServiceClient(conn)

// Submit transaction
resp, err := client.Submit(ctx, &pb.SubmitTransactionRequest{
    Devnet: "mydevnet",
    TxType: "bank/send",
    Signer: "validator:0",
    Payload: []byte(`{"to_address":"cosmos1...","amount":"1000000uatom"}`),
    GasLimit: 200000,
    Memo: "test-tx",
})

// Watch transaction
stream, err := client.Watch(ctx, &pb.WatchTransactionRequest{
    Id: resp.Id,
})

for {
    update, err := stream.Recv()
    if err == io.EOF {
        break
    }
    fmt.Printf("Phase: %s\n", update.Status.Phase)
}
```

## Best Practices

1. **Use payload files** - Easier to review and version control
2. **Set explicit gas limits** - Avoid auto-estimation in production
3. **Add memos** - Track transactions for debugging
4. **Watch important transactions** - Real-time feedback for critical operations
5. **Handle errors gracefully** - Check transaction status before assuming success
6. **Test in devnet first** - Always validate payloads before mainnet
7. **Use batch submissions** - More efficient for multiple transactions
8. **Monitor gas usage** - Track gas consumption patterns

## Troubleshooting

### Transaction Stuck in Building

```bash
# Check builder status
dvb daemon status --verbose

# View controller logs
dvb daemon logs --follow | grep TxController

# Force reconciliation
dvb debug reconcile transaction/tx-12345
```

### Signature Verification Failed

```bash
# Check signer exists
dvb nodes list mydevnet

# Verify signer has balance
dvb query balance mydevnet validator:0

# Re-submit with correct signer
dvb tx submit mydevnet --signer validator:0 ...
```

### Transaction Rejected by Network

```bash
# Get detailed error
dvb tx status tx-12345 --output json | jq .status.error

# Common issues:
# - Insufficient funds: add more tokens
# - Invalid sequence: wait and retry
# - Invalid proposal: fix payload format
```

## Next Steps

- **[Client Reference](client.md)** - Complete dvb command reference
- **[Architecture](architecture.md)** - Understanding the transaction system
- **[Plugin Development](plugins.md)** - Implementing custom transaction types
- **[API Reference](api-reference.md)** - gRPC transaction API
