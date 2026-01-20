package rpc

import (
	"context"
	"testing"
	"time"

	pb "github.com/altuslabsxyz/devnet-builder/pkg/network/plugin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// mockPluginModule is a mock implementation of NetworkPluginModule for testing.
// Implements all methods with default Unimplemented behavior for backward compatibility testing.
type mockPluginModule struct {
	getGovernanceParamsFn func(rpcEndpoint, networkType string) (*pb.GovernanceParamsResponse, error)
	getBlockHeightFn      func(ctx context.Context, rpcEndpoint string) (*pb.BlockHeightResponse, error)
	getBlockTimeFn        func(ctx context.Context, rpcEndpoint string, sampleSize int) (*pb.BlockTimeResponse, error)
	isChainRunningFn      func(ctx context.Context, rpcEndpoint string) (*pb.ChainStatusResponse, error)
	waitForBlockFn        func(ctx context.Context, rpcEndpoint string, targetHeight int64, timeoutMs int64) (*pb.WaitForBlockResponse, error)
	getProposalFn         func(ctx context.Context, rpcEndpoint string, proposalID uint64) (*pb.ProposalResponse, error)
	getUpgradePlanFn      func(ctx context.Context, rpcEndpoint string) (*pb.UpgradePlanResponse, error)
	getAppVersionFn       func(ctx context.Context, rpcEndpoint string) (*pb.AppVersionResponse, error)
}

func (m *mockPluginModule) GetGovernanceParams(rpcEndpoint, networkType string) (*pb.GovernanceParamsResponse, error) {
	if m.getGovernanceParamsFn != nil {
		return m.getGovernanceParamsFn(rpcEndpoint, networkType)
	}
	return nil, status.Errorf(codes.Unimplemented, "method GetGovernanceParams not implemented")
}

func (m *mockPluginModule) GetBlockHeight(ctx context.Context, rpcEndpoint string) (*pb.BlockHeightResponse, error) {
	if m.getBlockHeightFn != nil {
		return m.getBlockHeightFn(ctx, rpcEndpoint)
	}
	return nil, status.Errorf(codes.Unimplemented, "method GetBlockHeight not implemented")
}

func (m *mockPluginModule) GetBlockTime(ctx context.Context, rpcEndpoint string, sampleSize int) (*pb.BlockTimeResponse, error) {
	if m.getBlockTimeFn != nil {
		return m.getBlockTimeFn(ctx, rpcEndpoint, sampleSize)
	}
	return nil, status.Errorf(codes.Unimplemented, "method GetBlockTime not implemented")
}

func (m *mockPluginModule) IsChainRunning(ctx context.Context, rpcEndpoint string) (*pb.ChainStatusResponse, error) {
	if m.isChainRunningFn != nil {
		return m.isChainRunningFn(ctx, rpcEndpoint)
	}
	return nil, status.Errorf(codes.Unimplemented, "method IsChainRunning not implemented")
}

func (m *mockPluginModule) WaitForBlock(ctx context.Context, rpcEndpoint string, targetHeight int64, timeoutMs int64) (*pb.WaitForBlockResponse, error) {
	if m.waitForBlockFn != nil {
		return m.waitForBlockFn(ctx, rpcEndpoint, targetHeight, timeoutMs)
	}
	return nil, status.Errorf(codes.Unimplemented, "method WaitForBlock not implemented")
}

func (m *mockPluginModule) GetProposal(ctx context.Context, rpcEndpoint string, proposalID uint64) (*pb.ProposalResponse, error) {
	if m.getProposalFn != nil {
		return m.getProposalFn(ctx, rpcEndpoint, proposalID)
	}
	return nil, status.Errorf(codes.Unimplemented, "method GetProposal not implemented")
}

func (m *mockPluginModule) GetUpgradePlan(ctx context.Context, rpcEndpoint string) (*pb.UpgradePlanResponse, error) {
	if m.getUpgradePlanFn != nil {
		return m.getUpgradePlanFn(ctx, rpcEndpoint)
	}
	return nil, status.Errorf(codes.Unimplemented, "method GetUpgradePlan not implemented")
}

func (m *mockPluginModule) GetAppVersion(ctx context.Context, rpcEndpoint string) (*pb.AppVersionResponse, error) {
	if m.getAppVersionFn != nil {
		return m.getAppVersionFn(ctx, rpcEndpoint)
	}
	return nil, status.Errorf(codes.Unimplemented, "method GetAppVersion not implemented")
}

// TestCosmosRPCClient_PluginDelegation_Success tests successful plugin delegation.
func TestCosmosRPCClient_PluginDelegation_Success(t *testing.T) {
	// Create mock plugin that returns governance parameters
	mockPlugin := &mockPluginModule{
		getGovernanceParamsFn: func(rpcEndpoint, networkType string) (*pb.GovernanceParamsResponse, error) {
			// Verify parameters passed correctly
			if rpcEndpoint != "http://localhost:1317" {
				t.Errorf("unexpected rpcEndpoint: got %s, want http://localhost:1317", rpcEndpoint)
			}
			if networkType != "devnet" {
				t.Errorf("unexpected networkType: got %s, want devnet", networkType)
			}

			return &pb.GovernanceParamsResponse{
				VotingPeriodNs:          int64(48 * time.Hour),
				ExpeditedVotingPeriodNs: int64(24 * time.Hour),
				MinDeposit:              "10000000",
				ExpeditedMinDeposit:     "50000000",
				Error:                   "",
			}, nil
		},
	}

	// Create client with plugin configured
	client := NewCosmosRPCClient("localhost", 26657).
		WithPlugin(mockPlugin, "devnet")

	// Call GetGovParams - should delegate to plugin
	ctx := context.Background()
	params, err := client.GetGovParams(ctx)

	// Verify success
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if params == nil {
		t.Fatal("params is nil")
	}

	// Verify parameters
	if params.VotingPeriod != 48*time.Hour {
		t.Errorf("unexpected VotingPeriod: got %v, want 48h", params.VotingPeriod)
	}
	if params.ExpeditedVotingPeriod != 24*time.Hour {
		t.Errorf("unexpected ExpeditedVotingPeriod: got %v, want 24h", params.ExpeditedVotingPeriod)
	}
	if params.MinDeposit != "10000000" {
		t.Errorf("unexpected MinDeposit: got %s, want 10000000", params.MinDeposit)
	}
	if params.ExpeditedMinDeposit != "50000000" {
		t.Errorf("unexpected ExpeditedMinDeposit: got %s, want 50000000", params.ExpeditedMinDeposit)
	}
}

// TestCosmosRPCClient_PluginDelegation_ErrorResponse tests plugin error handling.
func TestCosmosRPCClient_PluginDelegation_ErrorResponse(t *testing.T) {
	// Create mock plugin that returns error in response
	mockPlugin := &mockPluginModule{
		getGovernanceParamsFn: func(rpcEndpoint, networkType string) (*pb.GovernanceParamsResponse, error) {
			return &pb.GovernanceParamsResponse{
				Error: "connection refused: failed to connect to chain",
			}, nil
		},
	}

	// Create client with plugin configured
	client := NewCosmosRPCClient("localhost", 26657).
		WithPlugin(mockPlugin, "devnet")

	// Call GetGovParams - should return error
	ctx := context.Background()
	params, err := client.GetGovParams(ctx)

	// Verify error
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if params != nil {
		t.Errorf("expected nil params on error, got %+v", params)
	}

	// Check error message contains plugin error
	rpcErr, ok := err.(*RPCError)
	if !ok {
		t.Fatalf("expected *RPCError, got %T", err)
	}
	if rpcErr.Operation != "gov_params_plugin" {
		t.Errorf("unexpected operation: got %s, want gov_params_plugin", rpcErr.Operation)
	}
}

// TestCosmosRPCClient_PluginDelegation_Unimplemented tests backward compatibility.
// When plugin doesn't implement GetGovernanceParams, client should fall back to REST.
func TestCosmosRPCClient_PluginDelegation_Unimplemented(t *testing.T) {
	// Create mock plugin that returns Unimplemented error
	mockPlugin := &mockPluginModule{
		getGovernanceParamsFn: func(rpcEndpoint, networkType string) (*pb.GovernanceParamsResponse, error) {
			return nil, status.Errorf(codes.Unimplemented, "method GetGovernanceParams not implemented")
		},
	}

	// Create client with plugin configured
	client := NewCosmosRPCClient("localhost", 26657).
		WithPlugin(mockPlugin, "devnet")

	// Call GetGovParams - should attempt plugin, then fall back to REST
	ctx := context.Background()
	params, err := client.GetGovParams(ctx)

	// Since we don't have a real REST API running, this will fail with REST error
	// The important thing is that it tried to fall back (not return Unimplemented error)
	if err == nil {
		t.Fatal("expected REST error (no real server), got nil")
	}

	// Error should be REST operation error, not plugin error
	rpcErr, ok := err.(*RPCError)
	if !ok {
		t.Fatalf("expected *RPCError, got %T", err)
	}
	if rpcErr.Operation == "gov_params_plugin" {
		t.Error("should have fallen back to REST, not return plugin error")
	}
	if rpcErr.Operation != "gov_params" {
		t.Logf("operation: %s (expected REST fallback)", rpcErr.Operation)
	}

	// params should be nil since REST also failed
	if params != nil {
		t.Errorf("expected nil params, got %+v", params)
	}
}

// TestCosmosRPCClient_WithoutPlugin tests REST fallback when no plugin configured.
func TestCosmosRPCClient_WithoutPlugin(t *testing.T) {
	// Create client WITHOUT plugin configured
	client := NewCosmosRPCClient("localhost", 26657)
	// Explicitly NOT calling WithPlugin()

	// Call GetGovParams - should go directly to REST
	ctx := context.Background()
	params, err := client.GetGovParams(ctx)

	// Since we don't have a real REST API running, this will fail
	// But the error should be a REST error, not a plugin error
	if err == nil {
		t.Fatal("expected REST error (no real server), got nil")
	}

	rpcErr, ok := err.(*RPCError)
	if !ok {
		t.Fatalf("expected *RPCError, got %T", err)
	}
	if rpcErr.Operation == "gov_params_plugin" {
		t.Error("should not have attempted plugin delegation")
	}

	if params != nil {
		t.Errorf("expected nil params on error, got %+v", params)
	}
}

// TestCosmosRPCClient_PluginDelegation_NetworkError tests network errors from plugin.
func TestCosmosRPCClient_PluginDelegation_NetworkError(t *testing.T) {
	// Create mock plugin that returns a network error
	mockPlugin := &mockPluginModule{
		getGovernanceParamsFn: func(rpcEndpoint, networkType string) (*pb.GovernanceParamsResponse, error) {
			// Simulate network error (not Unimplemented - real error)
			return nil, status.Errorf(codes.Unavailable, "network timeout")
		},
	}

	// Create client with plugin configured
	client := NewCosmosRPCClient("localhost", 26657).
		WithPlugin(mockPlugin, "devnet")

	// Call GetGovParams - should return error (not fall back to REST)
	ctx := context.Background()
	params, err := client.GetGovParams(ctx)

	// Verify error is returned (not fallback)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if params != nil {
		t.Errorf("expected nil params on error, got %+v", params)
	}

	// Error should be plugin error
	rpcErr, ok := err.(*RPCError)
	if !ok {
		t.Fatalf("expected *RPCError, got %T", err)
	}
	if rpcErr.Operation != "gov_params_plugin" {
		t.Errorf("unexpected operation: got %s, want gov_params_plugin", rpcErr.Operation)
	}
}
