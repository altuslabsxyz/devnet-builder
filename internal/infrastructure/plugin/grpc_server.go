// Package plugin provides HashiCorp go-plugin based network module plugin system.
package plugin

import (
	"context"
	"time"

	pb "github.com/b-harvest/devnet-builder/pkg/network/plugin"
)

// GRPCServer is the gRPC server that plugins use to implement NetworkModule.
type GRPCServer struct {
	pb.UnimplementedNetworkModuleServer
	Impl NetworkModule
}

// NetworkModule is the interface that plugins must implement.
type NetworkModule interface {
	Name() string
	DisplayName() string
	Version() string
	BinaryName() string
	BinarySource() BinarySource
	DefaultBinaryVersion() string
	DefaultChainID() string
	Bech32Prefix() string
	BaseDenom() string
	GenesisConfig() GenesisConfig
	DefaultPorts() PortConfig
	DefaultGeneratorConfig() GeneratorConfig
	DockerImage() string
	DockerImageTag(version string) string
	DockerHomeDir() string
	InitCommand(homeDir, chainID, moniker string) []string
	StartCommand(homeDir string) []string
	ExportCommand(homeDir string) []string
	DefaultNodeHome() string
	PIDFileName() string
	LogFileName() string
	ProcessPattern() string
	ModifyGenesis(genesis []byte, opts GenesisOptions) ([]byte, error)
	GenerateDevnet(ctx context.Context, config GeneratorConfig, genesisFile string) error
	GetCodec() ([]byte, error)
	Validate() error
	// Snapshot methods
	SnapshotURL(networkType string) string
	RPCEndpoint(networkType string) string
	AvailableNetworks() []string
}

// BinarySource defines how to acquire the network binary.
type BinarySource struct {
	Type      string
	Owner     string
	Repo      string
	LocalPath string
	AssetName string
}

// PortConfig contains port configuration.
type PortConfig struct {
	RPC       int
	P2P       int
	GRPC      int
	GRPCWeb   int
	API       int
	EVMRPC    int
	EVMSocket int
}

// GenesisConfig contains genesis parameters.
type GenesisConfig struct {
	ChainIDPattern    string
	EVMChainID        int64
	BaseDenom         string
	DenomExponent     int
	DisplayDenom      string
	BondDenom         string
	MinSelfDelegation string
	UnbondingTime     time.Duration
	MaxValidators     uint32
	MinDeposit        string
	VotingPeriod      time.Duration
	MaxDepositPeriod  time.Duration
	CommunityTax      string
}

// GeneratorConfig contains devnet generator configuration.
type GeneratorConfig struct {
	NumValidators    int
	NumAccounts      int
	AccountBalance   string
	ValidatorBalance string
	ValidatorStake   string
	OutputDir        string
	ChainID          string
}

// GenesisOptions contains genesis modification options.
type GenesisOptions struct {
	ChainID       string
	NumValidators int
}

// Identity methods
func (s *GRPCServer) Name(ctx context.Context, req *pb.Empty) (*pb.StringResponse, error) {
	return &pb.StringResponse{Value: s.Impl.Name()}, nil
}

func (s *GRPCServer) DisplayName(ctx context.Context, req *pb.Empty) (*pb.StringResponse, error) {
	return &pb.StringResponse{Value: s.Impl.DisplayName()}, nil
}

func (s *GRPCServer) Version(ctx context.Context, req *pb.Empty) (*pb.StringResponse, error) {
	return &pb.StringResponse{Value: s.Impl.Version()}, nil
}

// Binary methods
func (s *GRPCServer) BinaryName(ctx context.Context, req *pb.Empty) (*pb.StringResponse, error) {
	return &pb.StringResponse{Value: s.Impl.BinaryName()}, nil
}

func (s *GRPCServer) BinarySource(ctx context.Context, req *pb.Empty) (*pb.BinarySourceResponse, error) {
	src := s.Impl.BinarySource()
	return &pb.BinarySourceResponse{
		Type:      src.Type,
		Owner:     src.Owner,
		Repo:      src.Repo,
		LocalPath: src.LocalPath,
		AssetName: src.AssetName,
	}, nil
}

func (s *GRPCServer) DefaultBinaryVersion(ctx context.Context, req *pb.Empty) (*pb.StringResponse, error) {
	return &pb.StringResponse{Value: s.Impl.DefaultBinaryVersion()}, nil
}

// Chain methods

// Deprecated: DefaultChainID will be removed in v2.0.0
func (s *GRPCServer) DefaultChainID(ctx context.Context, req *pb.Empty) (*pb.StringResponse, error) {
	return &pb.StringResponse{Value: s.Impl.DefaultChainID()}, nil
}

func (s *GRPCServer) Bech32Prefix(ctx context.Context, req *pb.Empty) (*pb.StringResponse, error) {
	return &pb.StringResponse{Value: s.Impl.Bech32Prefix()}, nil
}

func (s *GRPCServer) BaseDenom(ctx context.Context, req *pb.Empty) (*pb.StringResponse, error) {
	return &pb.StringResponse{Value: s.Impl.BaseDenom()}, nil
}

// Configuration methods
func (s *GRPCServer) GenesisConfig(ctx context.Context, req *pb.Empty) (*pb.GenesisConfigResponse, error) {
	cfg := s.Impl.GenesisConfig()
	return &pb.GenesisConfigResponse{
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

func (s *GRPCServer) DefaultPorts(ctx context.Context, req *pb.Empty) (*pb.PortConfigResponse, error) {
	ports := s.Impl.DefaultPorts()
	return &pb.PortConfigResponse{
		Rpc:       int32(ports.RPC),
		P2P:       int32(ports.P2P),
		Grpc:      int32(ports.GRPC),
		GrpcWeb:   int32(ports.GRPCWeb),
		Api:       int32(ports.API),
		EvmRpc:    int32(ports.EVMRPC),
		EvmSocket: int32(ports.EVMSocket),
	}, nil
}

func (s *GRPCServer) DefaultGeneratorConfig(ctx context.Context, req *pb.Empty) (*pb.GeneratorConfigResponse, error) {
	cfg := s.Impl.DefaultGeneratorConfig()
	return &pb.GeneratorConfigResponse{
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
func (s *GRPCServer) DockerImage(ctx context.Context, req *pb.Empty) (*pb.StringResponse, error) {
	return &pb.StringResponse{Value: s.Impl.DockerImage()}, nil
}

func (s *GRPCServer) DockerImageTag(ctx context.Context, req *pb.StringRequest) (*pb.StringResponse, error) {
	return &pb.StringResponse{Value: s.Impl.DockerImageTag(req.Value)}, nil
}

func (s *GRPCServer) DockerHomeDir(ctx context.Context, req *pb.Empty) (*pb.StringResponse, error) {
	return &pb.StringResponse{Value: s.Impl.DockerHomeDir()}, nil
}

// Command methods
func (s *GRPCServer) InitCommand(ctx context.Context, req *pb.InitCommandRequest) (*pb.StringListResponse, error) {
	cmd := s.Impl.InitCommand(req.HomeDir, req.ChainId, req.Moniker)
	return &pb.StringListResponse{Values: cmd}, nil
}

func (s *GRPCServer) StartCommand(ctx context.Context, req *pb.StringRequest) (*pb.StringListResponse, error) {
	cmd := s.Impl.StartCommand(req.Value)
	return &pb.StringListResponse{Values: cmd}, nil
}

func (s *GRPCServer) ExportCommand(ctx context.Context, req *pb.StringRequest) (*pb.StringListResponse, error) {
	cmd := s.Impl.ExportCommand(req.Value)
	return &pb.StringListResponse{Values: cmd}, nil
}

// Path methods
func (s *GRPCServer) DefaultNodeHome(ctx context.Context, req *pb.Empty) (*pb.StringResponse, error) {
	return &pb.StringResponse{Value: s.Impl.DefaultNodeHome()}, nil
}

func (s *GRPCServer) PIDFileName(ctx context.Context, req *pb.Empty) (*pb.StringResponse, error) {
	return &pb.StringResponse{Value: s.Impl.PIDFileName()}, nil
}

func (s *GRPCServer) LogFileName(ctx context.Context, req *pb.Empty) (*pb.StringResponse, error) {
	return &pb.StringResponse{Value: s.Impl.LogFileName()}, nil
}

func (s *GRPCServer) ProcessPattern(ctx context.Context, req *pb.Empty) (*pb.StringResponse, error) {
	return &pb.StringResponse{Value: s.Impl.ProcessPattern()}, nil
}

// Operation methods
func (s *GRPCServer) ModifyGenesis(ctx context.Context, req *pb.ModifyGenesisRequest) (*pb.BytesResponse, error) {
	opts := GenesisOptions{
		ChainID:       req.ChainId,
		NumValidators: int(req.NumValidators),
	}
	result, err := s.Impl.ModifyGenesis(req.Genesis, opts)
	if err != nil {
		return &pb.BytesResponse{Error: err.Error()}, nil
	}
	return &pb.BytesResponse{Data: result}, nil
}

func (s *GRPCServer) GenerateDevnet(ctx context.Context, req *pb.GenerateDevnetRequest) (*pb.ErrorResponse, error) {
	config := GeneratorConfig{
		NumValidators:    int(req.NumValidators),
		NumAccounts:      int(req.NumAccounts),
		AccountBalance:   req.AccountBalance,
		ValidatorBalance: req.ValidatorBalance,
		ValidatorStake:   req.ValidatorStake,
		OutputDir:        req.OutputDir,
		ChainID:          req.ChainId,
	}
	err := s.Impl.GenerateDevnet(ctx, config, req.GenesisFile)
	if err != nil {
		return &pb.ErrorResponse{Error: err.Error()}, nil
	}
	return &pb.ErrorResponse{}, nil
}

func (s *GRPCServer) GetCodec(ctx context.Context, req *pb.Empty) (*pb.BytesResponse, error) {
	data, err := s.Impl.GetCodec()
	if err != nil {
		return &pb.BytesResponse{Error: err.Error()}, nil
	}
	return &pb.BytesResponse{Data: data}, nil
}

func (s *GRPCServer) Validate(ctx context.Context, req *pb.Empty) (*pb.ErrorResponse, error) {
	err := s.Impl.Validate()
	if err != nil {
		return &pb.ErrorResponse{Error: err.Error()}, nil
	}
	return &pb.ErrorResponse{}, nil
}

// Snapshot methods
func (s *GRPCServer) SnapshotURL(ctx context.Context, req *pb.StringRequest) (*pb.StringResponse, error) {
	return &pb.StringResponse{Value: s.Impl.SnapshotURL(req.Value)}, nil
}

func (s *GRPCServer) RPCEndpoint(ctx context.Context, req *pb.StringRequest) (*pb.StringResponse, error) {
	return &pb.StringResponse{Value: s.Impl.RPCEndpoint(req.Value)}, nil
}

func (s *GRPCServer) AvailableNetworks(ctx context.Context, req *pb.Empty) (*pb.StringListResponse, error) {
	return &pb.StringListResponse{Values: s.Impl.AvailableNetworks()}, nil
}
