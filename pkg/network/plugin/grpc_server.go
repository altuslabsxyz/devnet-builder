package plugin

import (
	"context"
	"time"

	"github.com/b-harvest/devnet-builder/pkg/network"
)

// GRPCServer is the gRPC server that plugins use to implement network.Module.
type GRPCServer struct {
	UnimplementedNetworkModuleServer
	impl network.Module
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

// Chain methods
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

func (s *GRPCServer) StartCommand(ctx context.Context, req *StringRequest) (*StringListResponse, error) {
	cmd := s.impl.StartCommand(req.Value)
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

// Helper to convert Duration
func durationToSeconds(d time.Duration) int64 {
	return int64(d.Seconds())
}
