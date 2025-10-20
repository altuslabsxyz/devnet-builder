# Stable Devnet Builder

Devnet Builder is a tool to build local development networks from exported genesis files.

## Building

Use the Makefile:

```bash
make build
```

The binary will be created at `./build/devnet-builder`.

Available Makefile targets:
- `make build` - Build devnet-builder binary (default)
- `make clean` - Remove build artifacts
- `make install` - Install devnet-builder to GOPATH/bin
- `make test` - Run tests
- `make help` - Display help message

## How to Use

### Step 1: Export Genesis

First, export the genesis state from your running chain:

```bash
stabled export > genesis-export.json
```

### Step 2: Build Devnet

Then, build the devnet using the exported genesis:

```bash
./build/devnet-builder build genesis-export.json \
  --validators 4 \
  --accounts 10 \
  --account-balance "1000000000000000000000astable,500000000000000000000agasusdt" \
  --validator-balance "1000000000000000000000astable,500000000000000000000agasusdt" \
  --validator-stake "100000000000000000000" \
  --output ./devnet
```

## Parameters

- `--validators`: Number of validators to create (default: 4)
- `--accounts`: Number of dummy accounts to create (default: 10)
- `--account-balance`: Initial balance for each account (supports multiple denoms)
- `--validator-balance`: Initial balance for each validator (supports multiple denoms)
- `--validator-stake`: Staking amount for validators (base denom only)
- `--output`: Output directory (default: ./devnet)

## Output Structure

```
devnet/
├── node0/
│   ├── config/
│   │   ├── genesis.json
│   │   └── priv_validator_key.json
│   ├── data/
│   │   └── priv_validator_state.json
│   └── keyring-test/
├── node1/
├── node2/
├── node3/
└── accounts/
    └── keyring-test/
```
