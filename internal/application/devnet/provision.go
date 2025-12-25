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
	"github.com/b-harvest/devnet-builder/internal/infrastructure/stateexport"
	"github.com/b-harvest/devnet-builder/internal/infrastructure/tomlutil"
)

// ProvisionUseCase handles devnet provisioning.
type ProvisionUseCase struct {
	devnetRepo      ports.DevnetRepository
	nodeRepo        ports.NodeRepository
	snapshotSvc     ports.SnapshotFetcher
	genesisSvc      ports.GenesisFetcher
	stateExportSvc  ports.StateExportService
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
	stateExportSvc ports.StateExportService,
	nodeInitializer ports.NodeInitializer,
	networkModule ports.NetworkModule,
	logger ports.Logger,
) *ProvisionUseCase {
	return &ProvisionUseCase{
		devnetRepo:      devnetRepo,
		nodeRepo:        nodeRepo,
		snapshotSvc:     snapshotSvc,
		genesisSvc:      genesisSvc,
		stateExportSvc:  stateExportSvc,
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
	rpcGenesis, err := uc.genesisSvc.FetchFromRPC(ctx, rpcEndpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch genesis from RPC: %w", err)
	}

	// Use snapshot-based export if requested
	var genesis []byte
	if input.UseSnapshot && uc.stateExportSvc != nil {
		uc.logger.Info("Exporting genesis from snapshot state...")
		genesis, err = uc.exportGenesisFromSnapshot(ctx, input, rpcGenesis)
		if err != nil {
			return nil, fmt.Errorf("failed to export genesis from snapshot: %w", err)
		}
	} else {
		genesis = rpcGenesis
	}

	// Determine chain ID to use from genesis
	chainID, _ := extractChainID(genesis)
	metadata.ChainID = chainID

	// Step 1: Create account keys for validators (for transaction signing)
	uc.logger.Info("Creating validator account keys...")
	accountsDir := filepath.Join(input.HomeDir, "devnet", "accounts")
	accountKeys, err := uc.createAccountKeys(ctx, accountsDir, input.NumValidators, input.UseTestMnemonic)
	if err != nil {
		return nil, fmt.Errorf("failed to create account keys: %w", err)
	}

	// Step 2: Initialize nodes to generate consensus keys (for block signing)
	uc.logger.Info("Initializing validator nodes...")
	nodes, err := uc.initializeNodes(ctx, input, chainID)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize nodes: %w", err)
	}

	// Step 2.1: Save validator key information to JSON files for export-keys command
	uc.logger.Debug("Saving validator key information...")
	if err := uc.saveValidatorKeys(input.HomeDir, accountKeys, uc.networkModule.Bech32Prefix()); err != nil {
		return nil, fmt.Errorf("failed to save validator keys: %w", err)
	}

	// Step 2.5: Configure nodes with network-specific settings (config.toml, app.toml)
	uc.logger.Info("Configuring node settings...")
	if err := uc.configureNodes(ctx, nodes, chainID, input.NumValidators); err != nil {
		return nil, fmt.Errorf("failed to configure nodes: %w", err)
	}

	// Step 3: Build validator info combining consensus and account keys
	uc.logger.Info("Building validator info...")
	validators, err := uc.buildValidatorInfo(nodes, accountKeys, uc.networkModule.Bech32Prefix())
	if err != nil {
		return nil, fmt.Errorf("failed to build validator info: %w", err)
	}

	// Step 4: Modify genesis with validators
	uc.logger.Info("Modifying genesis for devnet (chainID: %s)...", chainID)
	if uc.networkModule != nil {
		opts := ports.GenesisModifyOptions{
			ChainID:       chainID,
			NumValidators: input.NumValidators,
			AddValidators: validators,
		}

		// Check genesis size - gRPC has 4MB default limit
		const grpcSizeLimit = 4 * 1024 * 1024 // 4MB
		if len(genesis) > grpcSizeLimit {
			// Use file-based modification for large genesis (e.g., exported mainnet ~90MB)
			uc.logger.Info("Using file-based genesis modification (size: %.1f MB)", float64(len(genesis))/(1024*1024))
			modifiedGenesis, err := uc.modifyGenesisViaFile(ctx, genesis, opts, input.HomeDir)
			if err != nil {
				return nil, fmt.Errorf("failed to modify genesis via file: %w", err)
			}
			genesis = modifiedGenesis
		} else {
			// Use standard in-memory modification for small genesis
			modifiedGenesis, err := uc.networkModule.ModifyGenesis(genesis, opts)
			if err != nil {
				return nil, fmt.Errorf("failed to modify genesis: %w", err)
			}
			genesis = modifiedGenesis
		}
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
// If useTestMnemonic is true, deterministic test mnemonics are used for reproducible testing.
func (uc *ProvisionUseCase) createAccountKeys(ctx context.Context, accountsDir string, numValidators int, useTestMnemonic bool) ([]*ports.AccountKeyInfo, error) {
	keys := make([]*ports.AccountKeyInfo, numValidators)

	for i := 0; i < numValidators; i++ {
		keyName := fmt.Sprintf("validator%d", i)

		var keyInfo *ports.AccountKeyInfo
		var err error

		if useTestMnemonic {
			// Use deterministic test mnemonic for reproducible testing
			mnemonic := uc.nodeInitializer.GetTestMnemonic(i)
			keyInfo, err = uc.nodeInitializer.CreateAccountKeyFromMnemonic(ctx, accountsDir, keyName, mnemonic)
			if err != nil {
				return nil, fmt.Errorf("failed to create account key from test mnemonic for %s: %w", keyName, err)
			}
			uc.logger.Debug("Created account key %s from test mnemonic: %s", keyName, keyInfo.Address)
		} else {
			// Generate new random mnemonic
			keyInfo, err = uc.nodeInitializer.CreateAccountKey(ctx, accountsDir, keyName)
			if err != nil {
				return nil, fmt.Errorf("failed to create account key for %s: %w", keyName, err)
			}
			uc.logger.Debug("Created account key %s: %s", keyName, keyInfo.Address)
		}

		keys[i] = keyInfo
	}

	return keys, nil
}

// saveValidatorKeys saves validator key information to JSON files for export-keys command.
// This creates validator{i}.json in each node directory with account address, valoper address, and mnemonic.
func (uc *ProvisionUseCase) saveValidatorKeys(homeDir string, accountKeys []*ports.AccountKeyInfo, bech32Prefix string) error {
	devnetDir := filepath.Join(homeDir, "devnet")

	for i, key := range accountKeys {
		// Convert account address to valoper address
		valoperAddress, err := convertToValoperAddress(key.Address, bech32Prefix)
		if err != nil {
			return fmt.Errorf("failed to convert address for validator %d: %w", i, err)
		}

		// Create validator key file structure
		keyFile := struct {
			Name       string `json:"name"`
			Address    string `json:"address"`
			ValAddress string `json:"val_address"`
			Mnemonic   string `json:"mnemonic"`
		}{
			Name:       key.Name,
			Address:    key.Address,
			ValAddress: valoperAddress,
			Mnemonic:   key.Mnemonic,
		}

		// Save to node directory
		nodeDir := filepath.Join(devnetDir, fmt.Sprintf("node%d", i))
		keyPath := filepath.Join(nodeDir, fmt.Sprintf("validator%d.json", i))

		data, err := json.MarshalIndent(keyFile, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal validator key %d: %w", i, err)
		}

		if err := os.WriteFile(keyPath, data, 0644); err != nil {
			return fmt.Errorf("failed to write validator key file %d: %w", i, err)
		}

		uc.logger.Debug("Saved validator key %d to %s", i, keyPath)
	}

	return nil
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

// configureNodes configures each node's config.toml and app.toml using the plugin.
// This gets config overrides from the plugin and merges them with the init'd configs.
func (uc *ProvisionUseCase) configureNodes(ctx context.Context, nodes []*ports.NodeMetadata, chainID string, numValidators int) error {
	if uc.networkModule == nil {
		uc.logger.Debug("No network module available, skipping node configuration")
		return nil
	}

	// Build persistent peers string: node_id@127.0.0.1:p2p_port,...
	persistentPeers := uc.buildPersistentPeers(nodes)
	uc.logger.Debug("Built persistent peers: %s", persistentPeers)

	for _, node := range nodes {
		opts := ports.NodeConfigOptions{
			ChainID:         chainID,
			Ports:           node.Ports,
			PersistentPeers: persistentPeers,
			NumValidators:   numValidators,
			IsValidator:     true, // All nodes are validators in devnet
			Moniker:         node.Name,
		}

		// Get config overrides from plugin
		configOverride, appOverride, err := uc.networkModule.GetConfigOverrides(node.Index, opts)
		if err != nil {
			return fmt.Errorf("failed to get config overrides for node %d: %w", node.Index, err)
		}

		uc.logger.Debug("Node %d config overrides: config=%d bytes, app=%d bytes", node.Index, len(configOverride), len(appOverride))

		// Merge config.toml if there are overrides
		if len(configOverride) > 0 {
			configPath := filepath.Join(node.HomeDir, "config", "config.toml")
			if err := uc.mergeConfig(configPath, configOverride); err != nil {
				return fmt.Errorf("failed to merge config.toml for node %d: %w", node.Index, err)
			}
			uc.logger.Debug("Merged config.toml for node %d", node.Index)
		}

		// Merge app.toml if there are overrides
		if len(appOverride) > 0 {
			appPath := filepath.Join(node.HomeDir, "config", "app.toml")
			if err := uc.mergeConfig(appPath, appOverride); err != nil {
				return fmt.Errorf("failed to merge app.toml for node %d: %w", node.Index, err)
			}
			uc.logger.Debug("Merged app.toml for node %d", node.Index)
		}
	}

	uc.logger.Debug("All nodes configured successfully")
	return nil
}

// mergeConfig reads a config file, merges with overrides, and writes back.
func (uc *ProvisionUseCase) mergeConfig(filePath string, override []byte) error {
	base, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	merged, err := tomlutil.MergeTOML(base, override)
	if err != nil {
		return err
	}

	if err := os.WriteFile(filePath, merged, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// buildPersistentPeers builds the persistent peers string from node metadata.
// Format: node_id@127.0.0.1:p2p_port,node_id@127.0.0.1:p2p_port,...
func (uc *ProvisionUseCase) buildPersistentPeers(nodes []*ports.NodeMetadata) string {
	var peers []string
	for _, node := range nodes {
		if node.NodeID != "" {
			peer := fmt.Sprintf("%s@127.0.0.1:%d", node.NodeID, node.Ports.P2P)
			peers = append(peers, peer)
		}
	}

	result := ""
	for i, peer := range peers {
		if i > 0 {
			result += ","
		}
		result += peer
	}
	return result
}

// exportGenesisFromSnapshot exports genesis from snapshot state.
// Flow:
// 1. Get snapshot URL from plugin
// 2. Download snapshot (with 30-minute caching)
// 3. Extract snapshot to temp directory
// 4. Export genesis from snapshot state
// 5. Return exported genesis
func (uc *ProvisionUseCase) exportGenesisFromSnapshot(ctx context.Context, input dto.ProvisionInput, rpcGenesis []byte) ([]byte, error) {
	// Validate binary path
	if input.BinaryPath == "" {
		return nil, fmt.Errorf("binary path is required for snapshot-based export")
	}

	// Get snapshot URL from plugin
	snapshotURL := input.SnapshotURL
	if snapshotURL == "" && uc.networkModule != nil {
		snapshotURL = uc.networkModule.SnapshotURL(input.Network)
	}
	if snapshotURL == "" {
		return nil, fmt.Errorf("no snapshot URL available for network: %s", input.Network)
	}

	// Step 1: Download snapshot with caching
	// Cached snapshots are stored in ~/.devnet-builder/snapshots/<cacheKey>/
	// Cache expires after 30 minutes by default
	// Cache key format: "plugin-network" (e.g., "stable-mainnet", "ault-testnet")
	cacheKey := fmt.Sprintf("%s-%s", input.BlockchainNetwork, input.Network)
	uc.logger.Info("Downloading snapshot from %s...", snapshotURL)
	snapshotPath, fromCache, err := uc.snapshotSvc.DownloadWithCache(ctx, snapshotURL, cacheKey, input.NoCache)
	if err != nil {
		return nil, fmt.Errorf("failed to download snapshot: %w", err)
	}
	if fromCache {
		uc.logger.Success("Using cached snapshot")
	}

	// Step 1.5: Check genesis cache BEFORE extraction
	// If snapshot was cached and genesis cache exists, use it directly
	if fromCache && cacheKey != "" && !input.NoCache {
		cache, err := stateexport.GetValidGenesisCache(input.HomeDir, cacheKey)
		if err == nil && cache != nil {
			// Verify the cached genesis is from the same snapshot
			if cache.SnapshotURL == snapshotURL {
				// Read cached genesis
				genesis, err := os.ReadFile(cache.FilePath)
				if err == nil {
					uc.logger.Info("Using cached genesis export (expires in %s)", cache.TimeUntilExpiry().Round(time.Minute))
					return genesis, nil
				}
				uc.logger.Debug("Failed to read cached genesis: %v", err)
			} else {
				uc.logger.Debug("Cached genesis is from different snapshot, will re-export")
			}
		}
	}

	// Create temp directory for extraction and export
	// The snapshot file itself stays in the cache directory
	exportDir := filepath.Join(input.HomeDir, "tmp", "state-export")
	if err := os.MkdirAll(exportDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create export directory: %w", err)
	}
	defer os.RemoveAll(exportDir) // Clean up extracted data after export

	// Step 2: Extract snapshot to temp directory
	uc.logger.Info("Extracting snapshot...")
	if err := uc.snapshotSvc.Extract(ctx, snapshotPath, exportDir); err != nil {
		return nil, fmt.Errorf("failed to extract snapshot: %w", err)
	}

	// Step 3: Export genesis from snapshot state
	// Pass snapshot information to enable genesis export caching
	uc.logger.Info("Exporting genesis from snapshot state...")
	exportOpts := ports.StateExportOptions{
		HomeDir:           exportDir,
		BinaryPath:        input.BinaryPath,
		RpcGenesis:        rpcGenesis,
		ExportOpts:        uc.stateExportSvc.DefaultExportOptions(),
		Network:           input.Network, // Keep for backward compatibility
		CacheKey:          cacheKey,      // Use composite cache key
		SnapshotURL:       snapshotURL,
		SnapshotFromCache: fromCache,
	}

	genesis, err := uc.stateExportSvc.ExportFromSnapshot(ctx, exportOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to export genesis: %w", err)
	}

	uc.logger.Success("Genesis exported from snapshot (%d bytes)", len(genesis))
	return genesis, nil
}

// modifyGenesisViaFile modifies genesis using file-based approach.
// This is used when genesis exceeds gRPC message size limits (4MB default).
// The method:
// 1. Writes genesis to a temp file
// 2. Calls plugin's ModifyGenesisFile with file paths
// 3. Reads the modified genesis from output file
func (uc *ProvisionUseCase) modifyGenesisViaFile(ctx context.Context, genesis []byte, opts ports.GenesisModifyOptions, homeDir string) ([]byte, error) {
	// Check if network module supports file-based modification
	fileModifier, ok := uc.networkModule.(ports.FileBasedGenesisModifier)
	if !ok {
		// Fallback: try standard modification anyway (may fail with gRPC limit)
		uc.logger.Warn("Network module doesn't support file-based genesis modification, falling back to standard method")
		return uc.networkModule.ModifyGenesis(genesis, opts)
	}

	// Create temp directory for genesis files
	tmpDir := filepath.Join(homeDir, "tmp", "genesis-modify")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Write input genesis
	inputPath := filepath.Join(tmpDir, "genesis_input.json")
	if err := os.WriteFile(inputPath, genesis, 0644); err != nil {
		return nil, fmt.Errorf("failed to write input genesis: %w", err)
	}

	// Define output path
	outputPath := filepath.Join(tmpDir, "genesis_output.json")

	// Call file-based modification via plugin
	outputSize, err := fileModifier.ModifyGenesisFile(inputPath, outputPath, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to modify genesis via file: %w", err)
	}
	uc.logger.Debug("Genesis modified via file (output size: %d bytes)", outputSize)

	// Read modified genesis
	modifiedGenesis, err := os.ReadFile(outputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read modified genesis: %w", err)
	}

	// Cleanup temp files
	_ = os.RemoveAll(tmpDir)

	return modifiedGenesis, nil
}
