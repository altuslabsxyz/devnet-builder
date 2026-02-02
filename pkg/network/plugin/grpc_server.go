package plugin

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/altuslabsxyz/devnet-builder/pkg/network"
)

// GRPCServer is the gRPC server that plugins use to implement network.Module.
type GRPCServer struct {
	UnimplementedNetworkModuleServer
	impl network.Module

	// TxBuilder management
	builders   map[string]network.TxBuilder
	buildersMu sync.RWMutex
	builderSeq uint64
}

// NewGRPCServer creates a new GRPCServer for the given network module.
func NewGRPCServer(impl network.Module) *GRPCServer {
	return &GRPCServer{impl: impl}
}

// Identity methods
func (s *GRPCServer) Name(ctx context.Context, req *Empty) (*StringResponse, error) {
	return &StringResponse{Value: s.impl.Name()}, nil
}

func (s *GRPCServer) DisplayName(ctx context.Context, req *Empty) (*StringResponse, error) {
	return &StringResponse{Value: s.impl.DisplayName()}, nil
}

func (s *GRPCServer) Version(ctx context.Context, req *Empty) (*StringResponse, error) {
	return &StringResponse{Value: s.impl.Version()}, nil
}

// Binary methods
func (s *GRPCServer) BinaryName(ctx context.Context, req *Empty) (*StringResponse, error) {
	return &StringResponse{Value: s.impl.BinaryName()}, nil
}

func (s *GRPCServer) BinarySource(ctx context.Context, req *Empty) (*BinarySourceResponse, error) {
	src := s.impl.BinarySource()
	return &BinarySourceResponse{
		Type:      src.Type,
		Owner:     src.Owner,
		Repo:      src.Repo,
		LocalPath: src.LocalPath,
		AssetName: src.AssetName,
		BuildTags: src.BuildTags,
	}, nil
}

func (s *GRPCServer) DefaultBinaryVersion(ctx context.Context, req *Empty) (*StringResponse, error) {
	return &StringResponse{Value: s.impl.DefaultBinaryVersion()}, nil
}

// GetBuildConfig returns network-specific build configuration.
// This implements fail-fast validation by validating the BuildConfig before returning.
func (s *GRPCServer) GetBuildConfig(ctx context.Context, req *BuildConfigRequest) (*BuildConfigResponse, error) {
	// Validate network type parameter
	if req.NetworkType == "" {
		return &BuildConfigResponse{
			Error: "network_type is required",
		}, nil
	}

	// Call the plugin's GetBuildConfig method
	config, err := s.impl.GetBuildConfig(req.NetworkType)
	if err != nil {
		return &BuildConfigResponse{
			Error: err.Error(),
		}, nil
	}

	// Handle nil config (plugin returns no custom configuration)
	if config == nil {
		return &BuildConfigResponse{
			Tags:      []string{},
			Ldflags:   []string{},
			Env:       map[string]string{},
			ExtraArgs: []string{},
		}, nil
	}

	// Validate the build config (fail-fast pattern from R0.5)
	if err := config.Validate(); err != nil {
		return &BuildConfigResponse{
			Error: "invalid build config: " + err.Error(),
		}, nil
	}

	// Convert to protobuf response
	return &BuildConfigResponse{
		Tags:      config.Tags,
		Ldflags:   config.LDFlags,
		Env:       config.Env,
		ExtraArgs: config.ExtraArgs,
	}, nil
}

// Chain methods

// Deprecated: DefaultChainID will be removed in v2.0.0
func (s *GRPCServer) DefaultChainID(ctx context.Context, req *Empty) (*StringResponse, error) {
	return &StringResponse{Value: s.impl.DefaultChainID()}, nil
}

func (s *GRPCServer) Bech32Prefix(ctx context.Context, req *Empty) (*StringResponse, error) {
	return &StringResponse{Value: s.impl.Bech32Prefix()}, nil
}

func (s *GRPCServer) BaseDenom(ctx context.Context, req *Empty) (*StringResponse, error) {
	return &StringResponse{Value: s.impl.BaseDenom()}, nil
}

// Configuration methods
func (s *GRPCServer) GenesisConfig(ctx context.Context, req *Empty) (*GenesisConfigResponse, error) {
	cfg := s.impl.GenesisConfig()
	return &GenesisConfigResponse{
		ChainIdPattern:          cfg.ChainIDPattern,
		EvmChainId:              cfg.EVMChainID,
		BaseDenom:               cfg.BaseDenom,
		DenomExponent:           int32(cfg.DenomExponent),
		DisplayDenom:            cfg.DisplayDenom,
		BondDenom:               cfg.BondDenom,
		MinSelfDelegation:       cfg.MinSelfDelegation,
		UnbondingTimeSeconds:    int64(cfg.UnbondingTime.Seconds()),
		MaxValidators:           cfg.MaxValidators,
		MinDeposit:              cfg.MinDeposit,
		VotingPeriodSeconds:     int64(cfg.VotingPeriod.Seconds()),
		MaxDepositPeriodSeconds: int64(cfg.MaxDepositPeriod.Seconds()),
		CommunityTax:            cfg.CommunityTax,
	}, nil
}

func (s *GRPCServer) DefaultPorts(ctx context.Context, req *Empty) (*PortConfigResponse, error) {
	ports := s.impl.DefaultPorts()
	return &PortConfigResponse{
		Rpc:       int32(ports.RPC),
		P2P:       int32(ports.P2P),
		Grpc:      int32(ports.GRPC),
		GrpcWeb:   int32(ports.GRPCWeb),
		Api:       int32(ports.API),
		EvmRpc:    int32(ports.EVMRPC),
		EvmSocket: int32(ports.EVMSocket),
	}, nil
}

func (s *GRPCServer) DefaultGeneratorConfig(ctx context.Context, req *Empty) (*GeneratorConfigResponse, error) {
	cfg := s.impl.DefaultGeneratorConfig()
	return &GeneratorConfigResponse{
		NumValidators:    int32(cfg.NumValidators),
		NumAccounts:      int32(cfg.NumAccounts),
		AccountBalance:   cfg.AccountBalance,
		ValidatorBalance: cfg.ValidatorBalance,
		ValidatorStake:   cfg.ValidatorStake,
		OutputDir:        cfg.OutputDir,
		ChainId:          cfg.ChainID,
	}, nil
}

// Docker methods
func (s *GRPCServer) DockerImage(ctx context.Context, req *Empty) (*StringResponse, error) {
	return &StringResponse{Value: s.impl.DockerImage()}, nil
}

func (s *GRPCServer) DockerImageTag(ctx context.Context, req *StringRequest) (*StringResponse, error) {
	return &StringResponse{Value: s.impl.DockerImageTag(req.Value)}, nil
}

func (s *GRPCServer) DockerHomeDir(ctx context.Context, req *Empty) (*StringResponse, error) {
	return &StringResponse{Value: s.impl.DockerHomeDir()}, nil
}

// Command methods
func (s *GRPCServer) InitCommand(ctx context.Context, req *InitCommandRequest) (*StringListResponse, error) {
	cmd := s.impl.InitCommand(req.HomeDir, req.ChainId, req.Moniker)
	return &StringListResponse{Values: cmd}, nil
}

func (s *GRPCServer) StartCommand(ctx context.Context, req *StartCommandRequest) (*StringListResponse, error) {
	cmd := s.impl.StartCommand(req.HomeDir, req.NetworkMode)
	return &StringListResponse{Values: cmd}, nil
}

func (s *GRPCServer) ExportCommand(ctx context.Context, req *StringRequest) (*StringListResponse, error) {
	cmd := s.impl.ExportCommand(req.Value)
	return &StringListResponse{Values: cmd}, nil
}

// Path methods
func (s *GRPCServer) DefaultNodeHome(ctx context.Context, req *Empty) (*StringResponse, error) {
	return &StringResponse{Value: s.impl.DefaultNodeHome()}, nil
}

func (s *GRPCServer) PIDFileName(ctx context.Context, req *Empty) (*StringResponse, error) {
	return &StringResponse{Value: s.impl.PIDFileName()}, nil
}

func (s *GRPCServer) LogFileName(ctx context.Context, req *Empty) (*StringResponse, error) {
	return &StringResponse{Value: s.impl.LogFileName()}, nil
}

func (s *GRPCServer) ProcessPattern(ctx context.Context, req *Empty) (*StringResponse, error) {
	return &StringResponse{Value: s.impl.ProcessPattern()}, nil
}

// Operation methods
func (s *GRPCServer) ModifyGenesis(ctx context.Context, req *ModifyGenesisRequest) (*BytesResponse, error) {
	// Convert validators from protobuf to network types
	validators := make([]network.ValidatorInfo, len(req.Validators))
	for i, v := range req.Validators {
		validators[i] = network.ValidatorInfo{
			Moniker:         v.Moniker,
			ConsPubKey:      v.ConsPubKey,
			OperatorAddress: v.OperatorAddress,
			SelfDelegation:  v.SelfDelegation,
		}
	}

	opts := network.GenesisOptions{
		ChainID:       req.ChainId,
		NumValidators: int(req.NumValidators),
		Validators:    validators,
	}
	result, err := s.impl.ModifyGenesis(req.Genesis, opts)
	if err != nil {
		return &BytesResponse{Error: err.Error()}, nil
	}
	return &BytesResponse{Data: result}, nil
}

func (s *GRPCServer) GenerateDevnet(ctx context.Context, req *GenerateDevnetRequest) (*ErrorResponse, error) {
	config := network.GeneratorConfig{
		NumValidators:    int(req.NumValidators),
		NumAccounts:      int(req.NumAccounts),
		AccountBalance:   req.AccountBalance,
		ValidatorBalance: req.ValidatorBalance,
		ValidatorStake:   req.ValidatorStake,
		OutputDir:        req.OutputDir,
		ChainID:          req.ChainId,
	}
	err := s.impl.GenerateDevnet(ctx, config, req.GenesisFile)
	if err != nil {
		return &ErrorResponse{Error: err.Error()}, nil
	}
	return &ErrorResponse{}, nil
}

func (s *GRPCServer) GetCodec(ctx context.Context, req *Empty) (*BytesResponse, error) {
	data, err := s.impl.GetCodec()
	if err != nil {
		return &BytesResponse{Error: err.Error()}, nil
	}
	return &BytesResponse{Data: data}, nil
}

func (s *GRPCServer) Validate(ctx context.Context, req *Empty) (*ErrorResponse, error) {
	err := s.impl.Validate()
	if err != nil {
		return &ErrorResponse{Error: err.Error()}, nil
	}
	return &ErrorResponse{}, nil
}

// Snapshot methods
func (s *GRPCServer) SnapshotURL(ctx context.Context, req *StringRequest) (*StringResponse, error) {
	return &StringResponse{Value: s.impl.SnapshotURL(req.Value)}, nil
}

func (s *GRPCServer) RPCEndpoint(ctx context.Context, req *StringRequest) (*StringResponse, error) {
	return &StringResponse{Value: s.impl.RPCEndpoint(req.Value)}, nil
}

func (s *GRPCServer) AvailableNetworks(ctx context.Context, req *Empty) (*StringListResponse, error) {
	return &StringListResponse{Values: s.impl.AvailableNetworks()}, nil
}

// GetConfigOverrides returns TOML configuration overrides for a node.
func (s *GRPCServer) GetConfigOverrides(ctx context.Context, req *NodeConfigRequest) (*ConfigOverridesResponse, error) {
	opts := network.NodeConfigOptions{
		ChainID:         req.ChainId,
		PersistentPeers: req.PersistentPeers,
		NumValidators:   int(req.NumValidators),
		IsValidator:     req.IsValidator,
		Moniker:         req.Moniker,
	}
	if req.Ports != nil {
		opts.Ports = network.PortConfig{
			RPC:       int(req.Ports.Rpc),
			P2P:       int(req.Ports.P2P),
			GRPC:      int(req.Ports.Grpc),
			GRPCWeb:   int(req.Ports.GrpcWeb),
			API:       int(req.Ports.Api),
			EVMRPC:    int(req.Ports.EvmRpc),
			EVMSocket: int(req.Ports.EvmSocket),
		}
	}

	configToml, appToml, err := s.impl.GetConfigOverrides(int(req.NodeIndex), opts)
	if err != nil {
		return &ConfigOverridesResponse{Error: err.Error()}, nil
	}
	return &ConfigOverridesResponse{
		ConfigToml: configToml,
		AppToml:    appToml,
	}, nil
}

// ModifyGenesisFile handles file-based genesis modification.
// This method avoids gRPC message size limits by using file paths.
func (s *GRPCServer) ModifyGenesisFile(ctx context.Context, req *ModifyGenesisFileRequest) (*ModifyGenesisFileResponse, error) {
	// Convert validators from protobuf to network types
	validators := make([]network.ValidatorInfo, len(req.Validators))
	for i, v := range req.Validators {
		validators[i] = network.ValidatorInfo{
			Moniker:         v.Moniker,
			ConsPubKey:      v.ConsPubKey,
			OperatorAddress: v.OperatorAddress,
			SelfDelegation:  v.SelfDelegation,
		}
	}

	opts := network.GenesisOptions{
		ChainID:       req.ChainId,
		NumValidators: int(req.NumValidators),
		Validators:    validators,
	}

	// Check if the implementation supports file-based modification
	if fbm, ok := s.impl.(network.FileBasedGenesisModifier); ok {
		// Use the optimized file-based method
		outputSize, err := fbm.ModifyGenesisFile(req.InputPath, req.OutputPath, opts)
		if err != nil {
			return &ModifyGenesisFileResponse{Error: err.Error()}, nil
		}
		return &ModifyGenesisFileResponse{OutputSize: outputSize}, nil
	}

	// Fallback: read file, modify in memory, write file
	genesis, err := os.ReadFile(req.InputPath)
	if err != nil {
		return &ModifyGenesisFileResponse{Error: "failed to read input file: " + err.Error()}, nil
	}

	modifiedGenesis, err := s.impl.ModifyGenesis(genesis, opts)
	if err != nil {
		return &ModifyGenesisFileResponse{Error: err.Error()}, nil
	}

	if err := os.WriteFile(req.OutputPath, modifiedGenesis, 0644); err != nil {
		return &ModifyGenesisFileResponse{Error: "failed to write output file: " + err.Error()}, nil
	}

	return &ModifyGenesisFileResponse{OutputSize: int64(len(modifiedGenesis))}, nil
}

// GetGovernanceParams retrieves governance parameters from the blockchain via the plugin.
// If the plugin doesn't implement GetGovernanceParams, returns Unimplemented error.
func (s *GRPCServer) GetGovernanceParams(ctx context.Context, req *GovernanceParamsRequest) (*GovernanceParamsResponse, error) {
	// Check if plugin implements GetGovernanceParams
	// This is a type assertion to see if the underlying network.Module supports this method
	type govParamsProvider interface {
		GetGovernanceParams(rpcEndpoint, networkType string) (*GovernanceParamsResponse, error)
	}

	if gpp, ok := s.impl.(govParamsProvider); ok {
		return gpp.GetGovernanceParams(req.RpcEndpoint, req.NetworkType)
	}

	// Plugin doesn't implement GetGovernanceParams - return Unimplemented
	// This allows backward compatibility with older plugins
	return nil, status.Errorf(codes.Unimplemented, "method GetGovernanceParams not implemented")
}

// RPC Operations - All blockchain RPC operations delegated to plugins.
// These methods use type assertions to check if the plugin implements the optional interface,
// returning Unimplemented error for backward compatibility with older plugins.

// RPCProvider defines the interface for RPC operations that plugins can implement.
type RPCProvider interface {
	GetBlockHeight(ctx context.Context, rpcEndpoint string) (*BlockHeightResponse, error)
	GetBlockTime(ctx context.Context, rpcEndpoint string, sampleSize int) (*BlockTimeResponse, error)
	IsChainRunning(ctx context.Context, rpcEndpoint string) (*ChainStatusResponse, error)
	WaitForBlock(ctx context.Context, rpcEndpoint string, targetHeight int64, timeoutMs int64) (*WaitForBlockResponse, error)
	GetProposal(ctx context.Context, rpcEndpoint string, proposalID uint64) (*ProposalResponse, error)
	GetUpgradePlan(ctx context.Context, rpcEndpoint string) (*UpgradePlanResponse, error)
	GetAppVersion(ctx context.Context, rpcEndpoint string) (*AppVersionResponse, error)
}

// GetBlockHeight retrieves the current block height via the plugin.
func (s *GRPCServer) GetBlockHeight(ctx context.Context, req *BlockHeightRequest) (*BlockHeightResponse, error) {
	if rpc, ok := s.impl.(RPCProvider); ok {
		return rpc.GetBlockHeight(ctx, req.RpcEndpoint)
	}
	return nil, status.Errorf(codes.Unimplemented, "method GetBlockHeight not implemented")
}

// GetBlockTime retrieves the average block time via the plugin.
func (s *GRPCServer) GetBlockTime(ctx context.Context, req *BlockTimeRequest) (*BlockTimeResponse, error) {
	if rpc, ok := s.impl.(RPCProvider); ok {
		return rpc.GetBlockTime(ctx, req.RpcEndpoint, int(req.SampleSize))
	}
	return nil, status.Errorf(codes.Unimplemented, "method GetBlockTime not implemented")
}

// IsChainRunning checks if the chain is responding via the plugin.
func (s *GRPCServer) IsChainRunning(ctx context.Context, req *ChainStatusRequest) (*ChainStatusResponse, error) {
	if rpc, ok := s.impl.(RPCProvider); ok {
		return rpc.IsChainRunning(ctx, req.RpcEndpoint)
	}
	return nil, status.Errorf(codes.Unimplemented, "method IsChainRunning not implemented")
}

// WaitForBlock waits until the chain reaches the specified height via the plugin.
func (s *GRPCServer) WaitForBlock(ctx context.Context, req *WaitForBlockRequest) (*WaitForBlockResponse, error) {
	if rpc, ok := s.impl.(RPCProvider); ok {
		return rpc.WaitForBlock(ctx, req.RpcEndpoint, req.TargetHeight, req.TimeoutMs)
	}
	return nil, status.Errorf(codes.Unimplemented, "method WaitForBlock not implemented")
}

// GetProposal retrieves a governance proposal by ID via the plugin.
func (s *GRPCServer) GetProposal(ctx context.Context, req *ProposalRequest) (*ProposalResponse, error) {
	if rpc, ok := s.impl.(RPCProvider); ok {
		return rpc.GetProposal(ctx, req.RpcEndpoint, req.ProposalId)
	}
	return nil, status.Errorf(codes.Unimplemented, "method GetProposal not implemented")
}

// GetUpgradePlan retrieves the current upgrade plan via the plugin.
func (s *GRPCServer) GetUpgradePlan(ctx context.Context, req *UpgradePlanRequest) (*UpgradePlanResponse, error) {
	if rpc, ok := s.impl.(RPCProvider); ok {
		return rpc.GetUpgradePlan(ctx, req.RpcEndpoint)
	}
	return nil, status.Errorf(codes.Unimplemented, "method GetUpgradePlan not implemented")
}

// GetAppVersion retrieves the application version via the plugin.
func (s *GRPCServer) GetAppVersion(ctx context.Context, req *AppVersionRequest) (*AppVersionResponse, error) {
	if rpc, ok := s.impl.(RPCProvider); ok {
		return rpc.GetAppVersion(ctx, req.RpcEndpoint)
	}
	return nil, status.Errorf(codes.Unimplemented, "method GetAppVersion not implemented")
}

// TxBuilder Operations

// CreateTxBuilder creates a new TxBuilder instance.
func (s *GRPCServer) CreateTxBuilder(ctx context.Context, req *CreateTxBuilderRequest) (*CreateTxBuilderResponse, error) {
	// Check if module implements TxBuilderFactory
	factory, ok := s.impl.(network.TxBuilderFactory)
	if !ok {
		return &CreateTxBuilderResponse{Error: "module does not support TxBuilder"}, nil
	}

	cfg := &network.TxBuilderConfig{
		RPCEndpoint: req.RpcEndpoint,
		ChainID:     req.ChainId,
	}

	if req.SdkVersion != nil {
		cfg.SDKVersion = &network.SDKVersion{
			Framework: req.SdkVersion.Framework,
			Version:   req.SdkVersion.Version,
			Features:  req.SdkVersion.Features,
		}
	}

	builder, err := factory.CreateTxBuilder(ctx, cfg)
	if err != nil {
		return &CreateTxBuilderResponse{Error: err.Error()}, nil
	}

	// Generate unique builder ID
	s.buildersMu.Lock()
	s.builderSeq++
	builderID := fmt.Sprintf("builder-%d", s.builderSeq)
	if s.builders == nil {
		s.builders = make(map[string]network.TxBuilder)
	}
	s.builders[builderID] = builder
	s.buildersMu.Unlock()

	return &CreateTxBuilderResponse{BuilderId: builderID}, nil
}

// BuildTx builds an unsigned transaction.
func (s *GRPCServer) BuildTx(ctx context.Context, req *BuildTxRequest) (*BuildTxResponse, error) {
	builder, err := s.getBuilder(req.BuilderId)
	if err != nil {
		return &BuildTxResponse{Error: err.Error()}, nil
	}

	buildReq := &network.TxBuildRequest{
		TxType:   network.TxType(req.TxType),
		Sender:   req.Sender,
		Payload:  req.Payload,
		ChainID:  req.ChainId,
		GasLimit: req.GasLimit,
		GasPrice: req.GasPrice,
		Memo:     req.Memo,
	}

	unsignedTx, err := builder.BuildTx(ctx, buildReq)
	if err != nil {
		return &BuildTxResponse{Error: err.Error()}, nil
	}

	return &BuildTxResponse{
		TxBytes:       unsignedTx.TxBytes,
		SignDoc:       unsignedTx.SignDoc,
		AccountNumber: unsignedTx.AccountNumber,
		Sequence:      unsignedTx.Sequence,
	}, nil
}

// SignTx signs a transaction.
func (s *GRPCServer) SignTx(ctx context.Context, req *SignTxRequest) (*SignTxResponse, error) {
	builder, err := s.getBuilder(req.BuilderId)
	if err != nil {
		return &SignTxResponse{Error: err.Error()}, nil
	}

	unsignedTx := &network.UnsignedTx{
		TxBytes:       req.TxBytes,
		SignDoc:       req.SignDoc,
		AccountNumber: req.AccountNumber,
		Sequence:      req.Sequence,
	}

	key := &network.SigningKey{
		Address:    req.Key.Address,
		PrivKey:    req.Key.PrivKey,
		KeyringRef: req.Key.KeyringRef,
	}

	signedTx, err := builder.SignTx(ctx, unsignedTx, key)
	if err != nil {
		return &SignTxResponse{Error: err.Error()}, nil
	}

	return &SignTxResponse{
		TxBytes:   signedTx.TxBytes,
		Signature: signedTx.Signature,
		PubKey:    signedTx.PubKey,
	}, nil
}

// BroadcastTx broadcasts a signed transaction.
func (s *GRPCServer) BroadcastTx(ctx context.Context, req *BroadcastTxRequest) (*BroadcastTxResponse, error) {
	builder, err := s.getBuilder(req.BuilderId)
	if err != nil {
		return &BroadcastTxResponse{Error: err.Error()}, nil
	}

	signedTx := &network.SignedTx{
		TxBytes: req.TxBytes,
	}

	result, err := builder.BroadcastTx(ctx, signedTx)
	if err != nil {
		return &BroadcastTxResponse{Error: err.Error()}, nil
	}

	return &BroadcastTxResponse{
		TxHash: result.TxHash,
		Code:   result.Code,
		Log:    result.Log,
		Height: result.Height,
	}, nil
}

// DestroyTxBuilder releases a TxBuilder.
func (s *GRPCServer) DestroyTxBuilder(ctx context.Context, req *DestroyTxBuilderRequest) (*DestroyTxBuilderResponse, error) {
	s.buildersMu.Lock()
	if builder, ok := s.builders[req.BuilderId]; ok {
		if closer, ok := builder.(interface{ Close() }); ok {
			closer.Close()
		}
		delete(s.builders, req.BuilderId)
	}
	s.buildersMu.Unlock()
	return &DestroyTxBuilderResponse{}, nil
}

func (s *GRPCServer) getBuilder(id string) (network.TxBuilder, error) {
	s.buildersMu.RLock()
	defer s.buildersMu.RUnlock()

	builder, ok := s.builders[id]
	if !ok {
		return nil, fmt.Errorf("builder not found: %s", id)
	}
	return builder, nil
}

// Helper to convert Duration
func durationToSeconds(d time.Duration) int64 {
	return int64(d.Seconds())
}
