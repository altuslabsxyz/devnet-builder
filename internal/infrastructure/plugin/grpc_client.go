package plugin

import (
	"context"
	"errors"
	"time"

	pb "github.com/b-harvest/devnet-builder/pkg/network/plugin"
)

// GRPCClient is the gRPC client that the host uses to communicate with plugins.
type GRPCClient struct {
	client pb.NetworkModuleClient
}

// Identity methods
func (c *GRPCClient) Name() string {
	resp, err := c.client.Name(context.Background(), &pb.Empty{})
	if err != nil {
		return ""
	}
	return resp.Value
}

func (c *GRPCClient) DisplayName() string {
	resp, err := c.client.DisplayName(context.Background(), &pb.Empty{})
	if err != nil {
		return ""
	}
	return resp.Value
}

func (c *GRPCClient) Version() string {
	resp, err := c.client.Version(context.Background(), &pb.Empty{})
	if err != nil {
		return ""
	}
	return resp.Value
}

// Binary methods
func (c *GRPCClient) BinaryName() string {
	resp, err := c.client.BinaryName(context.Background(), &pb.Empty{})
	if err != nil {
		return ""
	}
	return resp.Value
}

func (c *GRPCClient) BinarySource() BinarySource {
	resp, err := c.client.BinarySource(context.Background(), &pb.Empty{})
	if err != nil {
		return BinarySource{}
	}
	return BinarySource{
		Type:      resp.Type,
		Owner:     resp.Owner,
		Repo:      resp.Repo,
		LocalPath: resp.LocalPath,
		AssetName: resp.AssetName,
	}
}

func (c *GRPCClient) DefaultBinaryVersion() string {
	resp, err := c.client.DefaultBinaryVersion(context.Background(), &pb.Empty{})
	if err != nil {
		return ""
	}
	return resp.Value
}

// Chain methods

// Deprecated: DefaultChainID will be removed in v2.0.0
func (c *GRPCClient) DefaultChainID() string {
	resp, err := c.client.DefaultChainID(context.Background(), &pb.Empty{})
	if err != nil {
		return ""
	}
	return resp.Value
}

func (c *GRPCClient) Bech32Prefix() string {
	resp, err := c.client.Bech32Prefix(context.Background(), &pb.Empty{})
	if err != nil {
		return ""
	}
	return resp.Value
}

func (c *GRPCClient) BaseDenom() string {
	resp, err := c.client.BaseDenom(context.Background(), &pb.Empty{})
	if err != nil {
		return ""
	}
	return resp.Value
}

// Configuration methods
func (c *GRPCClient) GenesisConfig() GenesisConfig {
	resp, err := c.client.GenesisConfig(context.Background(), &pb.Empty{})
	if err != nil {
		return GenesisConfig{}
	}
	return GenesisConfig{
		ChainIDPattern:    resp.ChainIdPattern,
		EVMChainID:        resp.EvmChainId,
		BaseDenom:         resp.BaseDenom,
		DenomExponent:     int(resp.DenomExponent),
		DisplayDenom:      resp.DisplayDenom,
		BondDenom:         resp.BondDenom,
		MinSelfDelegation: resp.MinSelfDelegation,
		UnbondingTime:     time.Duration(resp.UnbondingTimeSeconds) * time.Second,
		MaxValidators:     resp.MaxValidators,
		MinDeposit:        resp.MinDeposit,
		VotingPeriod:      time.Duration(resp.VotingPeriodSeconds) * time.Second,
		MaxDepositPeriod:  time.Duration(resp.MaxDepositPeriodSeconds) * time.Second,
		CommunityTax:      resp.CommunityTax,
	}
}

func (c *GRPCClient) DefaultPorts() PortConfig {
	resp, err := c.client.DefaultPorts(context.Background(), &pb.Empty{})
	if err != nil {
		return PortConfig{}
	}
	return PortConfig{
		RPC:       int(resp.Rpc),
		P2P:       int(resp.P2P),
		GRPC:      int(resp.Grpc),
		GRPCWeb:   int(resp.GrpcWeb),
		API:       int(resp.Api),
		EVMRPC:    int(resp.EvmRpc),
		EVMSocket: int(resp.EvmSocket),
	}
}

func (c *GRPCClient) DefaultGeneratorConfig() GeneratorConfig {
	resp, err := c.client.DefaultGeneratorConfig(context.Background(), &pb.Empty{})
	if err != nil {
		return GeneratorConfig{}
	}
	return GeneratorConfig{
		NumValidators:    int(resp.NumValidators),
		NumAccounts:      int(resp.NumAccounts),
		AccountBalance:   resp.AccountBalance,
		ValidatorBalance: resp.ValidatorBalance,
		ValidatorStake:   resp.ValidatorStake,
		OutputDir:        resp.OutputDir,
		ChainID:          resp.ChainId,
	}
}

// Docker methods
func (c *GRPCClient) DockerImage() string {
	resp, err := c.client.DockerImage(context.Background(), &pb.Empty{})
	if err != nil {
		return ""
	}
	return resp.Value
}

func (c *GRPCClient) DockerImageTag(version string) string {
	resp, err := c.client.DockerImageTag(context.Background(), &pb.StringRequest{Value: version})
	if err != nil {
		return ""
	}
	return resp.Value
}

func (c *GRPCClient) DockerHomeDir() string {
	resp, err := c.client.DockerHomeDir(context.Background(), &pb.Empty{})
	if err != nil {
		return ""
	}
	return resp.Value
}

// Command methods
func (c *GRPCClient) InitCommand(homeDir, chainID, moniker string) []string {
	resp, err := c.client.InitCommand(context.Background(), &pb.InitCommandRequest{
		HomeDir: homeDir,
		ChainId: chainID,
		Moniker: moniker,
	})
	if err != nil {
		return nil
	}
	return resp.Values
}

func (c *GRPCClient) StartCommand(homeDir string) []string {
	resp, err := c.client.StartCommand(context.Background(), &pb.StringRequest{Value: homeDir})
	if err != nil {
		return nil
	}
	return resp.Values
}

func (c *GRPCClient) ExportCommand(homeDir string) []string {
	resp, err := c.client.ExportCommand(context.Background(), &pb.StringRequest{Value: homeDir})
	if err != nil {
		return nil
	}
	return resp.Values
}

// Path methods
func (c *GRPCClient) DefaultNodeHome() string {
	resp, err := c.client.DefaultNodeHome(context.Background(), &pb.Empty{})
	if err != nil {
		return ""
	}
	return resp.Value
}

func (c *GRPCClient) PIDFileName() string {
	resp, err := c.client.PIDFileName(context.Background(), &pb.Empty{})
	if err != nil {
		return ""
	}
	return resp.Value
}

func (c *GRPCClient) LogFileName() string {
	resp, err := c.client.LogFileName(context.Background(), &pb.Empty{})
	if err != nil {
		return ""
	}
	return resp.Value
}

func (c *GRPCClient) ProcessPattern() string {
	resp, err := c.client.ProcessPattern(context.Background(), &pb.Empty{})
	if err != nil {
		return ""
	}
	return resp.Value
}

// Operation methods
func (c *GRPCClient) ModifyGenesis(genesis []byte, opts GenesisOptions) ([]byte, error) {
	resp, err := c.client.ModifyGenesis(context.Background(), &pb.ModifyGenesisRequest{
		Genesis:       genesis,
		ChainId:       opts.ChainID,
		NumValidators: int32(opts.NumValidators),
	})
	if err != nil {
		return nil, err
	}
	if resp.Error != "" {
		return nil, errors.New(resp.Error)
	}
	return resp.Data, nil
}

func (c *GRPCClient) GenerateDevnet(ctx context.Context, config GeneratorConfig, genesisFile string) error {
	resp, err := c.client.GenerateDevnet(ctx, &pb.GenerateDevnetRequest{
		NumValidators:    int32(config.NumValidators),
		NumAccounts:      int32(config.NumAccounts),
		AccountBalance:   config.AccountBalance,
		ValidatorBalance: config.ValidatorBalance,
		ValidatorStake:   config.ValidatorStake,
		OutputDir:        config.OutputDir,
		ChainId:          config.ChainID,
		GenesisFile:      genesisFile,
	})
	if err != nil {
		return err
	}
	if resp.Error != "" {
		return errors.New(resp.Error)
	}
	return nil
}

func (c *GRPCClient) GetCodec() ([]byte, error) {
	resp, err := c.client.GetCodec(context.Background(), &pb.Empty{})
	if err != nil {
		return nil, err
	}
	if resp.Error != "" {
		return nil, errors.New(resp.Error)
	}
	return resp.Data, nil
}

func (c *GRPCClient) Validate() error {
	resp, err := c.client.Validate(context.Background(), &pb.Empty{})
	if err != nil {
		return err
	}
	if resp.Error != "" {
		return errors.New(resp.Error)
	}
	return nil
}

// Snapshot methods
func (c *GRPCClient) SnapshotURL(networkType string) string {
	resp, err := c.client.SnapshotURL(context.Background(), &pb.StringRequest{Value: networkType})
	if err != nil {
		return ""
	}
	return resp.Value
}

func (c *GRPCClient) RPCEndpoint(networkType string) string {
	resp, err := c.client.RPCEndpoint(context.Background(), &pb.StringRequest{Value: networkType})
	if err != nil {
		return ""
	}
	return resp.Value
}

func (c *GRPCClient) AvailableNetworks() []string {
	resp, err := c.client.AvailableNetworks(context.Background(), &pb.Empty{})
	if err != nil {
		return nil
	}
	return resp.Values
}
