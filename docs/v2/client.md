# DVB Client Reference

Complete reference for the `dvb` command-line client.

## Overview

`dvb` is the primary interface for interacting with the devnetd daemon. It provides commands for deploying devnets, managing nodes, submitting transactions, and orchestrating upgrades.

## Global Flags

```bash
dvb [global-flags] <command> [command-flags]
```

Global flags apply to all commands:

```
--config string      Config file (default: ~/.devnet-builder/config.toml)
--socket string      Daemon socket path (default: ~/.devnet-builder/devnetd.sock)
--address string     Daemon address for remote access (e.g., localhost:50051)
--tls                Enable TLS (for remote daemon)
--ca-cert string     CA certificate file for TLS
--token string       Authentication token
--output string      Output format: text, json, yaml (default: text)
--no-color           Disable colored output
--verbose            Verbose output
--debug              Debug logging
```

## Devnet Commands

### deploy

Deploy a new devnet:

```bash
dvb deploy <plugin> [flags]

Flags:
  --name string             Devnet name (default: auto-generated)
  --validators int          Number of validators (default: 4)
  --full-nodes int          Number of full nodes (default: 0)
  --mode string             Mode: docker, local (default: docker)
  --version string          Binary version (default: latest)
  --binary string           Path to custom binary
  --genesis string          Path to genesis file
  --chain-id string         Custom chain ID
  --home-base-dir string    Base directory for node data
  --snapshot-url string     Snapshot URL for fast sync
  --config string           JSON config overrides

Examples:
  # Deploy 4-validator Osmosis devnet
  dvb deploy osmosisd --name osmosis-test

  # Deploy with custom version
  dvb deploy gaiad --version v14.1.0 --validators 7

  # Deploy from local binary
  dvb deploy osmosisd --binary /path/to/osmosisd

  # Deploy from snapshot
  dvb deploy osmosisd --snapshot-url https://snapshots.polkachu.com/osmosis.tar

  # Deploy with JSON config
  dvb deploy osmosisd --config '{
    "chainId": "my-chain-1",
    "ports": {"rpcPortStart": 30000}
  }'
```

### list

List all devnets:

```bash
dvb list [flags]

Flags:
  --status string    Filter by status: running, stopped, degraded
  --label string     Filter by label (key=value)
  --output string    Output format: text, json, yaml, wide

Output Columns (text):
  NAME       TYPE     STATUS    NODES  HEIGHT  SDK VERSION
  osmosis    cosmos   Running   4/4    1245    v0.50.3
  hub        cosmos   Stopped   4/4    -       v0.47.10

Output (json):
  [
    {
      "name": "osmosis",
      "type": "cosmos",
      "status": "Running",
      "nodes": 4,
      "readyNodes": 4,
      "height": 1245,
      "sdkVersion": "v0.50.3"
    }
  ]
```

### status

Get devnet status:

```bash
dvb status <devnet> [flags]

Flags:
  --watch           Watch for changes
  --refresh-interval duration   Refresh interval (default: 5s)

Example:
  dvb status osmosis-test

Output:
  Devnet: osmosis-test
  Status: Running
  Created: 2026-01-25 10:00:00
  Uptime: 2h 15m

  Network:
    Type: cosmos
    SDK Version: v0.50.3
    Chain ID: osmosis-test-1

  Nodes: 4/4 ready
    validator-0  Running  Height: 1245  Peers: 3  Restarts: 0
    validator-1  Running  Height: 1245  Peers: 3  Restarts: 0
    validator-2  Running  Height: 1245  Peers: 3  Restarts: 0
    validator-3  Running  Height: 1245  Peers: 3  Restarts: 0

  Endpoints:
    RPC:    http://localhost:26657
    REST:   http://localhost:1317
    gRPC:   localhost:9090
```

### start

Start a stopped devnet:

```bash
dvb start <devnet>

Example:
  dvb start osmosis-test
```

### stop

Stop a running devnet:

```bash
dvb stop <devnet> [flags]

Flags:
  --graceful-timeout duration  Timeout for graceful shutdown (default: 30s)
  --force                      Force stop immediately

Example:
  dvb stop osmosis-test
  dvb stop osmosis-test --force
```

### destroy

Permanently delete a devnet:

```bash
dvb destroy <devnet> [flags]

Flags:
  --confirm   Skip confirmation prompt
  --keep-data Keep data directory

Example:
  dvb destroy osmosis-test --confirm
```

## Node Commands

### nodes list

List nodes in a devnet:

```bash
dvb nodes list <devnet> [flags]

Flags:
  --role string   Filter by role: validator, fullnode
  --status string Filter by status: running, stopped, crashed

Example:
  dvb nodes list osmosis-test

Output:
  NAME          ROLE       STATUS   HEIGHT  PEERS  RESTARTS
  validator-0   validator  Running  1245    3      0
  validator-1   validator  Running  1245    3      0
  validator-2   validator  Running  1245    3      0
  validator-3   validator  Running  1245    3      0
```

### nodes logs

Stream node logs:

```bash
dvb nodes logs <devnet> <node> [flags]

Flags:
  --follow, -f       Follow log output
  --tail int         Number of lines to show (default: 100)
  --since duration   Show logs since duration (e.g., 5m, 1h)
  --timestamps       Include timestamps

Examples:
  # Follow logs
  dvb nodes logs osmosis-test validator:0 --follow

  # Last 500 lines
  dvb nodes logs osmosis-test validator:0 --tail 500

  # Logs from last hour
  dvb nodes logs osmosis-test validator:0 --since 1h
```

### nodes restart

Restart a node:

```bash
dvb nodes restart <devnet> <node>

Example:
  dvb nodes restart osmosis-test validator:0
```

### nodes health

Check node health:

```bash
dvb nodes health <devnet> [node] [flags]

Flags:
  --all    Check all devnets

Examples:
  # Single node
  dvb nodes health osmosis-test validator:0

  # All nodes in devnet
  dvb nodes health osmosis-test

  # All devnets
  dvb nodes health --all

Output:
  Node Health Check:
    validator-0  Running  Height: 1245  Peers: 3  ✓ Healthy
    validator-1  Running  Height: 1245  Peers: 3  ✓ Healthy
    validator-2  Crashed  Height: 1200  Peers: 0  ✗ Unhealthy
    validator-3  Running  Height: 1245  Peers: 3  ✓ Healthy

  Overall: Degraded (1/4 nodes unhealthy)
```

## Transaction Commands

### tx submit

Submit a transaction:

```bash
dvb tx submit <devnet> [flags]

Flags:
  --type string        Transaction type (required)
  --signer string      Signer address or ref (required)
  --payload string     JSON payload or @file
  --gas-limit uint     Gas limit (default: auto)
  --memo string        Transaction memo
  --wait               Wait for confirmation
  --timeout duration   Confirmation timeout (default: 30s)

Examples:
  # Bank send
  dvb tx submit osmosis-test \
    --type bank/send \
    --signer validator:0 \
    --payload '{"to_address":"osmo1...","amount":"1000000uosmo"}'

  # From file
  dvb tx submit osmosis-test \
    --type gov/proposal \
    --signer validator:0 \
    --payload @proposal.json

  # With wait
  dvb tx submit osmosis-test \
    --type gov/vote \
    --signer validator:0 \
    --payload '{"proposal_id":1,"option":"yes"}' \
    --wait

Output:
  ✓ Transaction submitted: tx-12345
  ✓ Confirmed at height 150
    TxHash: 0xABCD...
    GasUsed: 85000
```

### tx status

Get transaction status:

```bash
dvb tx status <tx-id>

Example:
  dvb tx status tx-12345

Output:
  Transaction: tx-12345
  Status: Confirmed
  TxHash: 0xABCD...
  Height: 150
  GasUsed: 85000/100000
  Submitted: 2026-01-25 10:15:30
  Confirmed: 2026-01-25 10:15:35
```

### tx watch

Watch transaction progress:

```bash
dvb tx watch <tx-id>

Example:
  dvb tx watch tx-12345

Output (streaming):
  [10:15:30] Phase: Pending
  [10:15:31] Phase: Building
  [10:15:32] Phase: Signing
  [10:15:33] Phase: Submitted (TxHash: 0xABCD...)
  [10:15:35] Phase: Confirmed (Height: 150)
```

### tx list

List transactions:

```bash
dvb tx list <devnet> [flags]

Flags:
  --status string  Filter by status: pending, confirmed, failed
  --type string    Filter by type
  --limit int      Max results (default: 20)

Example:
  dvb tx list osmosis-test --status confirmed --limit 10
```

## Governance Commands

### gov propose

Submit governance proposal:

```bash
dvb gov propose <devnet> [flags]

Flags:
  --title string         Proposal title (required)
  --description string   Proposal description (required)
  --deposit string       Initial deposit (required)
  --type string          Proposal type (default: text)
  --signer string        Signer (default: validator:0)
  --wait                 Wait for proposal creation

Example:
  dvb gov propose osmosis-test \
    --title "Enable IBC" \
    --description "Proposal to enable IBC module" \
    --deposit "10000000uosmo"

Output:
  ✓ Proposal submitted: 1
  ✓ Voting period: 2026-01-25 10:00:00 - 2026-01-27 10:00:00
```

### gov vote

Vote on proposal:

```bash
dvb gov vote <devnet> <proposal-id> <option> [flags]

Flags:
  --signer string      Signer (default: validator:0)
  --all-validators     Vote with all validators
  --wait               Wait for vote confirmation

Options: yes, no, abstain, no_with_veto

Examples:
  # Single validator
  dvb gov vote osmosis-test 1 yes

  # All validators
  dvb gov vote osmosis-test 1 yes --all-validators
```

### gov list

List governance proposals:

```bash
dvb gov list <devnet> [flags]

Flags:
  --status string  Filter by status: voting, passed, rejected

Example:
  dvb gov list osmosis-test

Output:
  ID  TITLE         STATUS   SUBMIT TIME         VOTING END
  1   Enable IBC    Voting   2026-01-25 10:00   2026-01-27 10:00
  2   Param Change  Passed   2026-01-20 10:00   2026-01-22 10:00
```

### gov status

Get proposal status:

```bash
dvb gov status <devnet> <proposal-id>

Example:
  dvb gov status osmosis-test 1

Output:
  Proposal: 1
  Title: Enable IBC
  Status: Voting

  Voting Period:
    Start: 2026-01-25 10:00:00
    End:   2026-01-27 10:00:00

  Votes:
    Yes: 75%  (3/4 validators)
    No: 0%
    Abstain: 0%
    NoWithVeto: 0%

  Turnout: 75% (3/4 voted)
```

## Upgrade Commands

### upgrade create

Create chain upgrade:

```bash
dvb upgrade create <devnet> [flags]

Flags:
  --upgrade-name string  Upgrade name (required)
  --height int64         Upgrade height (required)
  --binary string        New binary path (required)
  --version string       New version (alternative to --binary)
  --auto-vote            Auto-vote yes on proposal
  --with-export          Export state before/after upgrade

Example:
  dvb upgrade create osmosis-test \
    --upgrade-name v25 \
    --height 1000 \
    --binary /path/to/osmosisd-v25 \
    --auto-vote

Output:
  ✓ Upgrade created: v25
  ✓ Proposal submitted: 2
  ⏳ Waiting for votes...
```

### upgrade status

Get upgrade status:

```bash
dvb upgrade status <name>

Example:
  dvb upgrade status v25

Output:
  Upgrade: v25
  Status: Completed
  Devnet: osmosis-test

  Timeline:
    ✓ Proposed at height 250
    ✓ Voting completed (4/4 yes)
    ✓ Upgrade height 1000 reached
    ✓ Nodes stopped
    ✓ Binaries switched
    ✓ Nodes restarted
    ✓ Chain producing blocks (height: 1005)

  Old SDK Version: v0.50.3
  New SDK Version: v0.50.4
```

### upgrade list

List upgrades:

```bash
dvb upgrade list <devnet> [flags]

Flags:
  --status string  Filter by status: pending, completed, failed

Example:
  dvb upgrade list osmosis-test

Output:
  NAME  TARGET HEIGHT  STATUS     CREATED
  v25   1000           Completed  2026-01-25 10:00
  v24   500            Completed  2026-01-24 10:00
```

### upgrade cancel

Cancel pending upgrade:

```bash
dvb upgrade cancel <name>

Example:
  dvb upgrade cancel v25
```

## Daemon Commands

### daemon status

Get daemon status:

```bash
dvb daemon status

Output:
  Status: Running
  PID: 12345
  Uptime: 2h 15m 30s
  Version: v2.0.0
  gRPC Socket: /home/user/.devnet-builder/devnetd.sock
  Active Devnets: 3
  Total Nodes: 12
  Memory Usage: 245 MB
```

### daemon shutdown

Shutdown daemon:

```bash
dvb daemon shutdown [flags]

Flags:
  --force  Force shutdown immediately

Example:
  dvb daemon shutdown
```

### daemon logs

View daemon logs:

```bash
dvb daemon logs [flags]

Flags:
  --follow, -f      Follow log output
  --tail int        Number of lines (default: 100)
  --level string    Filter by level: debug, info, warn, error

Example:
  dvb daemon logs --follow --level error
```

### daemon config

Manage daemon configuration:

```bash
# Show config
dvb daemon config show

# Set value
dvb daemon config set <key>=<value>

# Get value
dvb daemon config get <key>

Examples:
  dvb daemon config show
  dvb daemon config set log_level=debug
  dvb daemon config get controller.reconcile_interval
```

## Plugin Commands

### plugins list

List installed plugins:

```bash
dvb plugins list

Output:
  NAME             VERSION  STATUS   SUPPORTED SDK VERSIONS
  cosmos-v047      v1.0.0   Active   v0.47.x
  cosmos-v050      v1.0.0   Active   v0.50.x
  cosmos-v053      v1.0.0   Active   v0.53.x
  evm-geth         v1.0.0   Active   -
```

### plugins info

Get plugin info:

```bash
dvb plugins info <plugin>

Example:
  dvb plugins info cosmos-v050

Output:
  Plugin: cosmos-v050
  Version: v1.0.0
  Type: Network Plugin
  Supported SDK Versions: v0.50.0 - v0.50.99

  Supported Transaction Types:
    - gov/proposal
    - gov/vote
    - bank/send
    - staking/delegate
    - staking/unbond
```

## Output Formats

All commands support multiple output formats:

```bash
# Human-readable text (default)
dvb list

# JSON
dvb list --output json

# YAML
dvb list --output yaml

# Wide (more columns)
dvb list --output wide
```

## Shell Completion

Generate shell completion:

```bash
# Bash
dvb completion bash > /etc/bash_completion.d/dvb

# Zsh
dvb completion zsh > ~/.zsh/completions/_dvb

# Fish
dvb completion fish > ~/.config/fish/completions/dvb.fish
```

## Environment Variables

Configure client behavior:

```bash
# Socket path
export DVB_SOCKET=/tmp/devnetd.sock

# Output format
export DVB_OUTPUT=json

# Disable colors
export DVB_NO_COLOR=1

# Token
export DVB_TOKEN=secret-token-1234
```

## Next Steps

- **[Transaction Guide](transactions.md)** - Deep dive into transactions
- **[Daemon Operations](daemon.md)** - Managing the daemon
- **[API Reference](api-reference.md)** - gRPC API documentation
