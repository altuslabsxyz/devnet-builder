package plugin

import (
	"context"
	"errors"
	"time"

	"google.golang.org/grpc"

	"github.com/b-harvest/devnet-builder/pkg/network"
)

// GRPCClient wraps the generated gRPC client and implements network.Module.
// This allows the host to use plugins as if they were native implementations.
type GRPCClient struct {
	client NetworkModuleClient
}

// NewGRPCClient creates a new GRPCClient from a gRPC connection.
func NewGRPCClient(conn *grpc.ClientConn) *GRPCClient {
	return &GRPCClient{
		client: NewNetworkModuleClient(conn),
	}
}

// Ensure GRPCClient implements network.Module
var _ network.Module = (*GRPCClient)(nil)

// Identity methods

func (c *GRPCClient) Name() string {
	resp, err := c.client.Name(context.Background(), &Empty{})
	if err != nil {
		return ""
	}
	return resp.Value
}

func (c *GRPCClient) DisplayName() string {
	resp, err := c.client.DisplayName(context.Background(), &Empty{})
	if err != nil {
		return ""
	}
	return resp.Value
}

func (c *GRPCClient) Version() string {
	resp, err := c.client.Version(context.Background(), &Empty{})
	if err != nil {
		return ""
	}
	return resp.Value
}

// Binary methods

func (c *GRPCClient) BinaryName() string {
	resp, err := c.client.BinaryName(context.Background(), &Empty{})
	if err != nil {
		return ""
	}
	return resp.Value
}

func (c *GRPCClient) BinarySource() network.BinarySource {
	resp, err := c.client.BinarySource(context.Background(), &Empty{})
	if err != nil {
		return network.BinarySource{}
	}
	return network.BinarySource{
		Type:      resp.Type,
		Owner:     resp.Owner,
		Repo:      resp.Repo,
		LocalPath: resp.LocalPath,
		AssetName: resp.AssetName,
		BuildTags: resp.BuildTags,
	}
}

func (c *GRPCClient) DefaultBinaryVersion() string {
	resp, err := c.client.DefaultBinaryVersion(context.Background(), &Empty{})
	if err != nil {
		return ""
	}
	return resp.Value
}

// GetBuildConfig returns network-specific build configuration from the plugin.
func (c *GRPCClient) GetBuildConfig(networkType string) (*network.BuildConfig, error) {
	resp, err := c.client.GetBuildConfig(context.Background(), &BuildConfigRequest{
		NetworkType: networkType,
	})
	if err != nil {
		return nil, err
	}

	// Check for error in response
	if resp.Error != "" {
		return nil, errors.New(resp.Error)
	}

	// Convert protobuf response to BuildConfig
	config := &network.BuildConfig{
		Tags:      resp.Tags,
		LDFlags:   resp.Ldflags,
		Env:       resp.Env,
		ExtraArgs: resp.ExtraArgs,
	}

	// Return empty config if all fields are empty
	if config.IsEmpty() {
		return &network.BuildConfig{}, nil
	}

	return config, nil
}

// Chain methods

// DEPRECATED: DefaultChainID will be removed in v2.0.0
func (c *GRPCClient) DefaultChainID() string {
	resp, err := c.client.DefaultChainID(context.Background(), &Empty{})
	if err != nil {
		return ""
	}
	return resp.Value
}

func (c *GRPCClient) Bech32Prefix() string {
	resp, err := c.client.Bech32Prefix(context.Background(), &Empty{})
	if err != nil {
		return ""
	}
	return resp.Value
}

func (c *GRPCClient) BaseDenom() string {
	resp, err := c.client.BaseDenom(context.Background(), &Empty{})
	if err != nil {
		return ""
	}
	return resp.Value
}

// Configuration methods

func (c *GRPCClient) GenesisConfig() network.GenesisConfig {
	resp, err := c.client.GenesisConfig(context.Background(), &Empty{})
	if err != nil {
		return network.GenesisConfig{}
	}
	return network.GenesisConfig{
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

func (c *GRPCClient) DefaultPorts() network.PortConfig {
	resp, err := c.client.DefaultPorts(context.Background(), &Empty{})
	if err != nil {
		return network.PortConfig{}
	}
	return network.PortConfig{
		RPC:       int(resp.Rpc),
		P2P:       int(resp.P2P),
		GRPC:      int(resp.Grpc),
		GRPCWeb:   int(resp.GrpcWeb),
		API:       int(resp.Api),
		EVMRPC:    int(resp.EvmRpc),
		EVMSocket: int(resp.EvmSocket),
	}
}

func (c *GRPCClient) DefaultGeneratorConfig() network.GeneratorConfig {
	resp, err := c.client.DefaultGeneratorConfig(context.Background(), &Empty{})
	if err != nil {
		return network.GeneratorConfig{}
	}
	return network.GeneratorConfig{
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
	resp, err := c.client.DockerImage(context.Background(), &Empty{})
	if err != nil {
		return ""
	}
	return resp.Value
}

func (c *GRPCClient) DockerImageTag(version string) string {
	resp, err := c.client.DockerImageTag(context.Background(), &StringRequest{Value: version})
	if err != nil {
		return ""
	}
	return resp.Value
}

func (c *GRPCClient) DockerHomeDir() string {
	resp, err := c.client.DockerHomeDir(context.Background(), &Empty{})
	if err != nil {
		return ""
	}
	return resp.Value
}

// Path methods

func (c *GRPCClient) DefaultNodeHome() string {
	resp, err := c.client.DefaultNodeHome(context.Background(), &Empty{})
	if err != nil {
		return ""
	}
	return resp.Value
}

func (c *GRPCClient) PIDFileName() string {
	resp, err := c.client.PIDFileName(context.Background(), &Empty{})
	if err != nil {
		return ""
	}
	return resp.Value
}

func (c *GRPCClient) LogFileName() string {
	resp, err := c.client.LogFileName(context.Background(), &Empty{})
	if err != nil {
		return ""
	}
	return resp.Value
}

func (c *GRPCClient) ProcessPattern() string {
	resp, err := c.client.ProcessPattern(context.Background(), &Empty{})
	if err != nil {
		return ""
	}
	return resp.Value
}

// Command methods

func (c *GRPCClient) InitCommand(homeDir, chainID, moniker string) []string {
	resp, err := c.client.InitCommand(context.Background(), &InitCommandRequest{
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
	resp, err := c.client.StartCommand(context.Background(), &StringRequest{Value: homeDir})
	if err != nil {
		return nil
	}
	return resp.Values
}

func (c *GRPCClient) ExportCommand(homeDir string) []string {
	resp, err := c.client.ExportCommand(context.Background(), &StringRequest{Value: homeDir})
	if err != nil {
		return nil
	}
	return resp.Values
}

// Operation methods

func (c *GRPCClient) ModifyGenesis(genesis []byte, opts network.GenesisOptions) ([]byte, error) {
	// Convert validators to protobuf format
	validators := make([]*ValidatorInfo, len(opts.Validators))
	for i, v := range opts.Validators {
		validators[i] = &ValidatorInfo{
			Moniker:         v.Moniker,
			ConsPubKey:      v.ConsPubKey,
			OperatorAddress: v.OperatorAddress,
			SelfDelegation:  v.SelfDelegation,
		}
	}

	resp, err := c.client.ModifyGenesis(context.Background(), &ModifyGenesisRequest{
		Genesis:       genesis,
		ChainId:       opts.ChainID,
		NumValidators: int32(opts.NumValidators),
		Validators:    validators,
	})
	if err != nil {
		return nil, err
	}
	if resp.Error != "" {
		return nil, errors.New(resp.Error)
	}
	return resp.Data, nil
}

func (c *GRPCClient) GenerateDevnet(ctx context.Context, config network.GeneratorConfig, genesisFile string) error {
	resp, err := c.client.GenerateDevnet(ctx, &GenerateDevnetRequest{
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
	resp, err := c.client.GetCodec(context.Background(), &Empty{})
	if err != nil {
		return nil, err
	}
	if resp.Error != "" {
		return nil, errors.New(resp.Error)
	}
	return resp.Data, nil
}

func (c *GRPCClient) Validate() error {
	resp, err := c.client.Validate(context.Background(), &Empty{})
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
	resp, err := c.client.SnapshotURL(context.Background(), &StringRequest{Value: networkType})
	if err != nil {
		return ""
	}
	return resp.Value
}

func (c *GRPCClient) RPCEndpoint(networkType string) string {
	resp, err := c.client.RPCEndpoint(context.Background(), &StringRequest{Value: networkType})
	if err != nil {
		return ""
	}
	return resp.Value
}

func (c *GRPCClient) AvailableNetworks() []string {
	resp, err := c.client.AvailableNetworks(context.Background(), &Empty{})
	if err != nil {
		return nil
	}
	return resp.Values
}

// GetConfigOverrides returns TOML configuration overrides for a node.
func (c *GRPCClient) GetConfigOverrides(nodeIndex int, opts network.NodeConfigOptions) ([]byte, []byte, error) {
	resp, err := c.client.GetConfigOverrides(context.Background(), &NodeConfigRequest{
		NodeIndex:       int32(nodeIndex),
		ChainId:         opts.ChainID,
		PersistentPeers: opts.PersistentPeers,
		NumValidators:   int32(opts.NumValidators),
		IsValidator:     opts.IsValidator,
		Moniker:         opts.Moniker,
		Ports: &PortConfigResponse{
			Rpc:       int32(opts.Ports.RPC),
			P2P:       int32(opts.Ports.P2P),
			Grpc:      int32(opts.Ports.GRPC),
			GrpcWeb:   int32(opts.Ports.GRPCWeb),
			Api:       int32(opts.Ports.API),
			EvmRpc:    int32(opts.Ports.EVMRPC),
			EvmSocket: int32(opts.Ports.EVMSocket),
		},
	})
	if err != nil {
		return nil, nil, err
	}
	if resp.Error != "" {
		return nil, nil, errors.New(resp.Error)
	}
	return resp.ConfigToml, resp.AppToml, nil
}

// Ensure GRPCClient implements FileBasedGenesisModifier
var _ network.FileBasedGenesisModifier = (*GRPCClient)(nil)

// ModifyGenesisFile implements network.FileBasedGenesisModifier.
// This method uses file paths instead of raw bytes to avoid gRPC message size limits.
func (c *GRPCClient) ModifyGenesisFile(inputPath, outputPath string, opts network.GenesisOptions) (int64, error) {
	// Convert validators to protobuf format
	validators := make([]*ValidatorInfo, len(opts.Validators))
	for i, v := range opts.Validators {
		validators[i] = &ValidatorInfo{
			Moniker:         v.Moniker,
			ConsPubKey:      v.ConsPubKey,
			OperatorAddress: v.OperatorAddress,
			SelfDelegation:  v.SelfDelegation,
		}
	}

	resp, err := c.client.ModifyGenesisFile(context.Background(), &ModifyGenesisFileRequest{
		InputPath:     inputPath,
		OutputPath:    outputPath,
		ChainId:       opts.ChainID,
		NumValidators: int32(opts.NumValidators),
		Validators:    validators,
	})
	if err != nil {
		return 0, err
	}
	if resp.Error != "" {
		return 0, errors.New(resp.Error)
	}
	return resp.OutputSize, nil
}

// GetGovernanceParams retrieves governance parameters from the plugin.
// This allows each network plugin to implement chain-specific parameter query logic.
func (c *GRPCClient) GetGovernanceParams(rpcEndpoint, networkType string) (*GovernanceParamsResponse, error) {
	// Use 5-second timeout for governance parameter queries
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := c.client.GetGovernanceParams(ctx, &GovernanceParamsRequest{
		RpcEndpoint: rpcEndpoint,
		NetworkType: networkType,
	})
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// RPC Operations - All blockchain RPC operations delegated to plugins

// GetBlockHeight retrieves the current block height from the plugin.
func (c *GRPCClient) GetBlockHeight(ctx context.Context, rpcEndpoint string) (*BlockHeightResponse, error) {
	resp, err := c.client.GetBlockHeight(ctx, &BlockHeightRequest{
		RpcEndpoint: rpcEndpoint,
	})
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// GetBlockTime retrieves the average block time from the plugin.
func (c *GRPCClient) GetBlockTime(ctx context.Context, rpcEndpoint string, sampleSize int) (*BlockTimeResponse, error) {
	resp, err := c.client.GetBlockTime(ctx, &BlockTimeRequest{
		RpcEndpoint: rpcEndpoint,
		SampleSize:  int32(sampleSize),
	})
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// IsChainRunning checks if the chain is responding via the plugin.
func (c *GRPCClient) IsChainRunning(ctx context.Context, rpcEndpoint string) (*ChainStatusResponse, error) {
	resp, err := c.client.IsChainRunning(ctx, &ChainStatusRequest{
		RpcEndpoint: rpcEndpoint,
	})
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// WaitForBlock waits until the chain reaches the specified height via the plugin.
func (c *GRPCClient) WaitForBlock(ctx context.Context, rpcEndpoint string, targetHeight int64, timeoutMs int64) (*WaitForBlockResponse, error) {
	resp, err := c.client.WaitForBlock(ctx, &WaitForBlockRequest{
		RpcEndpoint:  rpcEndpoint,
		TargetHeight: targetHeight,
		TimeoutMs:    timeoutMs,
	})
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// GetProposal retrieves a governance proposal by ID via the plugin.
func (c *GRPCClient) GetProposal(ctx context.Context, rpcEndpoint string, proposalID uint64) (*ProposalResponse, error) {
	resp, err := c.client.GetProposal(ctx, &ProposalRequest{
		RpcEndpoint: rpcEndpoint,
		ProposalId:  proposalID,
	})
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// GetUpgradePlan retrieves the current upgrade plan via the plugin.
func (c *GRPCClient) GetUpgradePlan(ctx context.Context, rpcEndpoint string) (*UpgradePlanResponse, error) {
	resp, err := c.client.GetUpgradePlan(ctx, &UpgradePlanRequest{
		RpcEndpoint: rpcEndpoint,
	})
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// GetAppVersion retrieves the application version via the plugin.
func (c *GRPCClient) GetAppVersion(ctx context.Context, rpcEndpoint string) (*AppVersionResponse, error) {
	resp, err := c.client.GetAppVersion(ctx, &AppVersionRequest{
		RpcEndpoint: rpcEndpoint,
	})
	if err != nil {
		return nil, err
	}
	return resp, nil
}
