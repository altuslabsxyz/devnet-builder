// Package devnet contains UseCases for devnet lifecycle management.
package devnet

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/cosmos/cosmos-sdk/types/bech32"

	"github.com/b-harvest/devnet-builder/internal/application/dto"
	"github.com/b-harvest/devnet-builder/internal/application/ports"
)

// ProvisionUseCase handles devnet provisioning.
type ProvisionUseCase struct {
	devnetRepo      ports.DevnetRepository
	nodeRepo        ports.NodeRepository
	snapshotSvc     ports.SnapshotFetcher
	genesisSvc      ports.GenesisFetcher
	nodeInitializer ports.NodeInitializer
	networkModule   ports.NetworkModule
	logger          ports.Logger
}

// NewProvisionUseCase creates a new ProvisionUseCase.
func NewProvisionUseCase(
	devnetRepo ports.DevnetRepository,
	nodeRepo ports.NodeRepository,
	snapshotSvc ports.SnapshotFetcher,
	genesisSvc ports.GenesisFetcher,
	nodeInitializer ports.NodeInitializer,
	networkModule ports.NetworkModule,
	logger ports.Logger,
) *ProvisionUseCase {
	return &ProvisionUseCase{
		devnetRepo:      devnetRepo,
		nodeRepo:        nodeRepo,
		snapshotSvc:     snapshotSvc,
		genesisSvc:      genesisSvc,
		nodeInitializer: nodeInitializer,
		networkModule:   networkModule,
		logger:          logger,
	}
}

// Execute provisions a new devnet.
func (uc *ProvisionUseCase) Execute(ctx context.Context, input dto.ProvisionInput) (*dto.ProvisionOutput, error) {
	uc.logger.Info("Provisioning devnet...")

	// Check if devnet already exists
	if uc.devnetRepo.Exists(input.HomeDir) {
		return nil, fmt.Errorf("devnet already exists at %s", input.HomeDir)
	}

	// Determine execution mode
	var execMode ports.ExecutionMode
	if input.Mode == "docker" {
		execMode = ports.ModeDocker
	} else {
		execMode = ports.ModeLocal
	}

	// Create metadata
	metadata := &ports.DevnetMetadata{
		HomeDir:           input.HomeDir,
		NetworkName:       input.Network,
		BlockchainNetwork: input.BlockchainNetwork,
		NetworkVersion:    input.NetworkVersion,
		NumValidators:     input.NumValidators,
		NumAccounts:       input.NumAccounts,
		ExecutionMode:     execMode,
		Status:            ports.StateCreated,
		DockerImage:       input.DockerImage,
		CustomBinaryPath:  input.CustomBinaryPath,
		CreatedAt:         time.Now(),
	}

	// Get RPC endpoint for fetching genesis
	rpcEndpoint := ""
	if uc.networkModule != nil {
		rpcEndpoint = uc.networkModule.RPCEndpoint(input.Network)
	}

	// Fetch genesis from RPC (required for initial provisioning)
	if rpcEndpoint == "" {
		return nil, fmt.Errorf("no RPC endpoint available for network: %s", input.Network)
	}

	uc.logger.Info("Fetching genesis from RPC %s...", rpcEndpoint)
	genesis, err := uc.genesisSvc.FetchFromRPC(ctx, rpcEndpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch genesis from RPC: %w", err)
	}

	// Determine chain ID to use
	originalChainID, _ := extractChainID(genesis)
	pluginChainID := ""
	if uc.networkModule != nil {
		pluginChainID = uc.networkModule.DefaultChainID()
	}
	chainIDToUse := originalChainID
	if pluginChainID != "" {
		chainIDToUse = pluginChainID
	}
	metadata.ChainID = chainIDToUse

	// Step 1: Create account keys for validators (for transaction signing)
	uc.logger.Info("Creating validator account keys...")
	accountsDir := filepath.Join(input.HomeDir, "devnet", "accounts")
	accountKeys, err := uc.createAccountKeys(ctx, accountsDir, input.NumValidators)
	if err != nil {
		return nil, fmt.Errorf("failed to create account keys: %w", err)
	}

	// Step 2: Initialize nodes to generate consensus keys (for block signing)
	uc.logger.Info("Initializing validator nodes...")
	nodes, err := uc.initializeNodes(ctx, input, chainIDToUse)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize nodes: %w", err)
	}

	// Step 3: Build validator info combining consensus and account keys
	uc.logger.Info("Building validator info...")
	validators, err := uc.buildValidatorInfo(nodes, accountKeys, uc.networkModule.Bech32Prefix())
	if err != nil {
		return nil, fmt.Errorf("failed to build validator info: %w", err)
	}

	// Step 4: Modify genesis with validators
	uc.logger.Info("Modifying genesis for devnet (chainID: %s)...", chainIDToUse)
	if uc.networkModule != nil {
		modifiedGenesis, err := uc.networkModule.ModifyGenesis(genesis, ports.GenesisModifyOptions{
			ChainID:       chainIDToUse,
			NumValidators: input.NumValidators,
			AddValidators: validators,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to modify genesis: %w", err)
		}
		genesis = modifiedGenesis
		uc.logger.Debug("Genesis modified with %d validators", len(validators))
	}

	// Step 4: Write modified genesis to all nodes
	for _, node := range nodes {
		genesisPath := filepath.Join(node.HomeDir, "config", "genesis.json")
		if err := os.WriteFile(genesisPath, genesis, 0644); err != nil {
			return nil, fmt.Errorf("failed to write genesis to node %d: %w", node.Index, err)
		}
	}

	// Set genesis path in metadata
	if len(nodes) > 0 {
		metadata.GenesisPath = filepath.Join(nodes[0].HomeDir, "config", "genesis.json")
	}

	// Update metadata
	metadata.Status = ports.StateProvisioned
	now := time.Now()
	metadata.LastProvisioned = &now

	// Save metadata
	if err := uc.devnetRepo.Save(ctx, metadata); err != nil {
		return nil, fmt.Errorf("failed to save metadata: %w", err)
	}

	// Save nodes
	for _, node := range nodes {
		if err := uc.nodeRepo.Save(ctx, node); err != nil {
			uc.logger.Warn("Failed to save node %d: %v", node.Index, err)
		}
	}

	// Build output
	output := &dto.ProvisionOutput{
		HomeDir:       input.HomeDir,
		ChainID:       metadata.ChainID,
		GenesisPath:   metadata.GenesisPath,
		NumValidators: input.NumValidators,
		NumAccounts:   input.NumAccounts,
		Nodes:         make([]dto.NodeInfo, len(nodes)),
	}

	for i, node := range nodes {
		output.Nodes[i] = dto.NodeInfo{
			Index:   node.Index,
			Name:    node.Name,
			HomeDir: node.HomeDir,
			NodeID:  node.NodeID,
			Ports:   node.Ports,
		}
	}

	uc.logger.Success("Provision complete!")
	return output, nil
}

func (uc *ProvisionUseCase) downloadAndExtractSnapshot(ctx context.Context, input dto.ProvisionInput, snapshotURL string) error {
	// Download snapshot
	snapshotPath := fmt.Sprintf("%s/snapshot.tar.gz", input.HomeDir)
	if err := uc.snapshotSvc.Download(ctx, snapshotURL, snapshotPath); err != nil {
		return err
	}

	// Extract snapshot
	return uc.snapshotSvc.Extract(ctx, snapshotPath, input.HomeDir)
}


func (uc *ProvisionUseCase) generateValidators(ctx context.Context, input dto.ProvisionInput, metadata *ports.DevnetMetadata, genesis []byte) ([]*ports.NodeMetadata, error) {
	// Get chain ID from plugin first (devnet-specific), fallback to genesis
	var chainID string
	if uc.networkModule != nil {
		chainID = uc.networkModule.DefaultChainID()
		if chainID != "" {
			uc.logger.Debug("Using chain ID from plugin: %s", chainID)
		}
	}

	// If plugin doesn't provide a chain ID, extract from genesis
	if chainID == "" {
		var err error
		chainID, err = extractChainID(genesis)
		if err != nil {
			return nil, fmt.Errorf("failed to extract chain_id from genesis: %w", err)
		}
		uc.logger.Debug("Extracted chain ID from genesis: %s", chainID)
	}
	metadata.ChainID = chainID

	nodes := make([]*ports.NodeMetadata, input.NumValidators)
	defaultPorts := uc.networkModule.DefaultPorts()

	for i := 0; i < input.NumValidators; i++ {
		nodeDir := filepath.Join(input.HomeDir, "devnet", fmt.Sprintf("node%d", i))
		moniker := fmt.Sprintf("node%d", i)

		// Create node directory
		if err := os.MkdirAll(nodeDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create node directory %s: %w", nodeDir, err)
		}

		// Initialize the node
		uc.logger.Debug("Initializing node %d at %s", i, nodeDir)
		if err := uc.nodeInitializer.Initialize(ctx, nodeDir, moniker, chainID); err != nil {
			return nil, fmt.Errorf("failed to initialize node %d: %w", i, err)
		}

		// Write genesis to node's config
		genesisPath := filepath.Join(nodeDir, "config", "genesis.json")
		if err := os.WriteFile(genesisPath, genesis, 0644); err != nil {
			return nil, fmt.Errorf("failed to write genesis for node %d: %w", i, err)
		}

		// Get node ID
		nodeID, err := uc.nodeInitializer.GetNodeID(ctx, nodeDir)
		if err != nil {
			uc.logger.Warn("Failed to get node ID for node %d: %v", i, err)
		}

		nodes[i] = &ports.NodeMetadata{
			Index:   i,
			Name:    moniker,
			HomeDir: nodeDir,
			ChainID: chainID,
			NodeID:  nodeID,
			Ports:   calculateNodePorts(defaultPorts, i),
		}
	}

	// Set genesis path in metadata
	if len(nodes) > 0 {
		metadata.GenesisPath = filepath.Join(nodes[0].HomeDir, "config", "genesis.json")
	}

	return nodes, nil
}

// extractChainID extracts the chain_id from genesis JSON.
func extractChainID(genesis []byte) (string, error) {
	var g struct {
		ChainID string `json:"chain_id"`
	}
	if err := json.Unmarshal(genesis, &g); err != nil {
		return "", err
	}
	if g.ChainID == "" {
		return "", fmt.Errorf("chain_id is empty in genesis")
	}
	return g.ChainID, nil
}

// calculateNodePorts calculates port assignments for a node.
func calculateNodePorts(basePorts ports.PortConfig, index int) ports.PortConfig {
	offset := index * 100
	return ports.PortConfig{
		RPC:     basePorts.RPC + offset,
		P2P:     basePorts.P2P + offset,
		GRPC:    basePorts.GRPC + offset,
		GRPCWeb: basePorts.GRPCWeb + offset,
		API:     basePorts.API + offset,
		EVM:     basePorts.EVM + offset,
		EVMWS:   basePorts.EVMWS + offset,
		PProf:   basePorts.PProf + offset,
		Rosetta: basePorts.Rosetta + offset,
	}
}

// initializeNodes initializes validator nodes and returns their metadata.
// This creates node directories, runs init command, and generates priv_validator_key.json.
func (uc *ProvisionUseCase) initializeNodes(ctx context.Context, input dto.ProvisionInput, chainID string) ([]*ports.NodeMetadata, error) {
	nodes := make([]*ports.NodeMetadata, input.NumValidators)
	defaultPorts := uc.networkModule.DefaultPorts()

	for i := 0; i < input.NumValidators; i++ {
		nodeDir := filepath.Join(input.HomeDir, "devnet", fmt.Sprintf("node%d", i))
		moniker := fmt.Sprintf("node%d", i)

		// Create node directory
		if err := os.MkdirAll(nodeDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create node directory %s: %w", nodeDir, err)
		}

		// Initialize the node (creates priv_validator_key.json)
		uc.logger.Debug("Initializing node %d at %s with chainID %s", i, nodeDir, chainID)
		if err := uc.nodeInitializer.Initialize(ctx, nodeDir, moniker, chainID); err != nil {
			return nil, fmt.Errorf("failed to initialize node %d: %w", i, err)
		}

		// Get node ID
		nodeID, err := uc.nodeInitializer.GetNodeID(ctx, nodeDir)
		if err != nil {
			uc.logger.Warn("Failed to get node ID for node %d: %v", i, err)
		}

		nodes[i] = &ports.NodeMetadata{
			Index:   i,
			Name:    moniker,
			HomeDir: nodeDir,
			ChainID: chainID,
			NodeID:  nodeID,
			Ports:   calculateNodePorts(defaultPorts, i),
		}
	}

	return nodes, nil
}

// extractValidatorInfo extracts validator information from initialized nodes.
// It reads priv_validator_key.json and derives the operator address.
func (uc *ProvisionUseCase) extractValidatorInfo(nodes []*ports.NodeMetadata, bech32Prefix string) ([]ports.ValidatorInfo, error) {
	validators := make([]ports.ValidatorInfo, len(nodes))

	for i, node := range nodes {
		// Read priv_validator_key.json
		keyPath := filepath.Join(node.HomeDir, "config", "priv_validator_key.json")
		keyData, err := os.ReadFile(keyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read validator key for node %d: %w", i, err)
		}

		// Parse the key file
		var keyFile struct {
			Address string `json:"address"`
			PubKey  struct {
				Type  string `json:"type"`
				Value string `json:"value"`
			} `json:"pub_key"`
		}
		if err := json.Unmarshal(keyData, &keyFile); err != nil {
			return nil, fmt.Errorf("failed to parse validator key for node %d: %w", i, err)
		}

		// The pubkey value is already base64 encoded
		consPubKey := keyFile.PubKey.Value

		// Derive operator address from consensus pubkey
		// The address in priv_validator_key.json is hex-encoded
		// We need to convert it to bech32 valoper address
		addressBytes, err := hexToBytes(keyFile.Address)
		if err != nil {
			return nil, fmt.Errorf("failed to decode address for node %d: %w", i, err)
		}

		// Create valoper address (validator operator address)
		valoperPrefix := bech32Prefix + "valoper"
		operatorAddress, err := bech32.ConvertAndEncode(valoperPrefix, addressBytes)
		if err != nil {
			return nil, fmt.Errorf("failed to encode operator address for node %d: %w", i, err)
		}

		validators[i] = ports.ValidatorInfo{
			Moniker:         node.Name,
			ConsPubKey:      consPubKey,
			OperatorAddress: operatorAddress,
			SelfDelegation:  "1000000000000000000000", // 1000 tokens (18 decimals)
		}

		uc.logger.Debug("Extracted validator %d: moniker=%s, operator=%s", i, node.Name, operatorAddress)
	}

	return validators, nil
}

// hexToBytes converts a hex string to bytes.
func hexToBytes(hexStr string) ([]byte, error) {
	// Remove 0x prefix if present
	if len(hexStr) >= 2 && hexStr[:2] == "0x" {
		hexStr = hexStr[2:]
	}

	bytes := make([]byte, len(hexStr)/2)
	for i := 0; i < len(bytes); i++ {
		var b byte
		_, err := fmt.Sscanf(hexStr[i*2:i*2+2], "%02x", &b)
		if err != nil {
			return nil, err
		}
		bytes[i] = b
	}
	return bytes, nil
}

// createAccountKeys creates secp256k1 account keys for validators.
// These keys are used for signing transactions (proposals, votes, etc.).
// The keys are stored in the accounts directory keyring.
func (uc *ProvisionUseCase) createAccountKeys(ctx context.Context, accountsDir string, numValidators int) ([]*ports.AccountKeyInfo, error) {
	keys := make([]*ports.AccountKeyInfo, numValidators)

	for i := 0; i < numValidators; i++ {
		keyName := fmt.Sprintf("validator%d", i)

		keyInfo, err := uc.nodeInitializer.CreateAccountKey(ctx, accountsDir, keyName)
		if err != nil {
			return nil, fmt.Errorf("failed to create account key for %s: %w", keyName, err)
		}

		keys[i] = keyInfo
		uc.logger.Debug("Created account key %s: %s", keyName, keyInfo.Address)
	}

	return keys, nil
}

// buildValidatorInfo combines consensus keys from nodes with account addresses.
// - ConsPubKey: from priv_validator_key.json (ed25519) for block signing
// - OperatorAddress: from account key (secp256k1) for transaction signing
func (uc *ProvisionUseCase) buildValidatorInfo(nodes []*ports.NodeMetadata, accountKeys []*ports.AccountKeyInfo, bech32Prefix string) ([]ports.ValidatorInfo, error) {
	if len(nodes) != len(accountKeys) {
		return nil, fmt.Errorf("mismatch: %d nodes but %d account keys", len(nodes), len(accountKeys))
	}

	validators := make([]ports.ValidatorInfo, len(nodes))

	for i, node := range nodes {
		// Read consensus pubkey from priv_validator_key.json
		keyPath := filepath.Join(node.HomeDir, "config", "priv_validator_key.json")
		keyData, err := os.ReadFile(keyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read validator key for node %d: %w", i, err)
		}

		var keyFile struct {
			PubKey struct {
				Value string `json:"value"`
			} `json:"pub_key"`
		}
		if err := json.Unmarshal(keyData, &keyFile); err != nil {
			return nil, fmt.Errorf("failed to parse validator key for node %d: %w", i, err)
		}

		consPubKey := keyFile.PubKey.Value

		// Get operator address from account key
		// Account address format: stable1xxx... -> need to convert to stablevaloper1xxx...
		accountAddr := accountKeys[i].Address
		valoperAddr, err := convertToValoperAddress(accountAddr, bech32Prefix)
		if err != nil {
			return nil, fmt.Errorf("failed to convert account address to valoper for node %d: %w", i, err)
		}

		validators[i] = ports.ValidatorInfo{
			Moniker:         node.Name,
			ConsPubKey:      consPubKey,
			OperatorAddress: valoperAddr,
			SelfDelegation:  "1000000000000000000000", // 1000 tokens (18 decimals)
		}

		uc.logger.Debug("Built validator %d: moniker=%s, operator=%s", i, node.Name, valoperAddr)
	}

	return validators, nil
}

// convertToValoperAddress converts an account address to a validator operator address.
// e.g., stable1xxx... -> stablevaloper1xxx...
func convertToValoperAddress(accountAddr, bech32Prefix string) (string, error) {
	// Decode the account address
	hrp, data, err := bech32.DecodeAndConvert(accountAddr)
	if err != nil {
		return "", fmt.Errorf("failed to decode address: %w", err)
	}

	// Verify the prefix matches
	if hrp != bech32Prefix {
		return "", fmt.Errorf("unexpected address prefix: got %s, expected %s", hrp, bech32Prefix)
	}

	// Encode with valoper prefix
	valoperPrefix := bech32Prefix + "valoper"
	valoperAddr, err := bech32.ConvertAndEncode(valoperPrefix, data)
	if err != nil {
		return "", fmt.Errorf("failed to encode valoper address: %w", err)
	}

	return valoperAddr, nil
}
