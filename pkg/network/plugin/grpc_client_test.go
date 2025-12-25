package plugin

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// mockNetworkModuleClient is a mock implementation of NetworkModuleClient for testing.
type mockNetworkModuleClient struct {
	NetworkModuleClient
	getGovernanceParamsFn func(ctx context.Context, in *GovernanceParamsRequest, opts ...grpc.CallOption) (*GovernanceParamsResponse, error)
}

func (m *mockNetworkModuleClient) GetGovernanceParams(ctx context.Context, in *GovernanceParamsRequest, opts ...grpc.CallOption) (*GovernanceParamsResponse, error) {
	if m.getGovernanceParamsFn != nil {
		return m.getGovernanceParamsFn(ctx, in, opts...)
	}
	return nil, status.Errorf(codes.Unimplemented, "method GetGovernanceParams not implemented")
}

// TestGRPCClient_GetGovernanceParams_Success tests successful parameter query.
func TestGRPCClient_GetGovernanceParams_Success(t *testing.T) {
	// Create mock client with successful response
	mockClient := &mockNetworkModuleClient{
		getGovernanceParamsFn: func(ctx context.Context, in *GovernanceParamsRequest, opts ...grpc.CallOption) (*GovernanceParamsResponse, error) {
			// Verify request parameters
			if in.RpcEndpoint != "http://localhost:26657" {
				t.Errorf("unexpected rpc_endpoint: got %s, want http://localhost:26657", in.RpcEndpoint)
			}
			if in.NetworkType != "devnet" {
				t.Errorf("unexpected network_type: got %s, want devnet", in.NetworkType)
			}

			// Return successful response
			return &GovernanceParamsResponse{
				VotingPeriodNs:          172800000000000, // 48 hours in nanoseconds
				ExpeditedVotingPeriodNs: 86400000000000,  // 24 hours in nanoseconds
				MinDeposit:              "10000000",
				ExpeditedMinDeposit:     "50000000",
				Error:                   "",
			}, nil
		},
	}

	client := &GRPCClient{client: mockClient}

	// Execute
	resp, err := client.GetGovernanceParams("http://localhost:26657", "devnet")

	// Verify
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("response is nil")
	}
	if resp.VotingPeriodNs != 172800000000000 {
		t.Errorf("unexpected voting_period_ns: got %d, want 172800000000000", resp.VotingPeriodNs)
	}
	if resp.ExpeditedVotingPeriodNs != 86400000000000 {
		t.Errorf("unexpected expedited_voting_period_ns: got %d, want 86400000000000", resp.ExpeditedVotingPeriodNs)
	}
	if resp.MinDeposit != "10000000" {
		t.Errorf("unexpected min_deposit: got %s, want 10000000", resp.MinDeposit)
	}
	if resp.Error != "" {
		t.Errorf("unexpected error field: got %s, want empty", resp.Error)
	}
}

// TestGRPCClient_GetGovernanceParams_NetworkUnreachable tests network error handling.
func TestGRPCClient_GetGovernanceParams_NetworkUnreachable(t *testing.T) {
	// Create mock client that returns network error
	mockClient := &mockNetworkModuleClient{
		getGovernanceParamsFn: func(ctx context.Context, in *GovernanceParamsRequest, opts ...grpc.CallOption) (*GovernanceParamsResponse, error) {
			// Return response with error field populated
			return &GovernanceParamsResponse{
				Error: "connection refused: http://localhost:26657",
			}, nil
		},
	}

	client := &GRPCClient{client: mockClient}

	// Execute
	resp, err := client.GetGovernanceParams("http://localhost:26657", "devnet")

	// Verify
	if err != nil {
		t.Fatalf("unexpected error from client: %v (error should be in response.Error field)", err)
	}
	if resp == nil {
		t.Fatal("response is nil")
	}
	if resp.Error == "" {
		t.Error("expected error field to be populated")
	}
	if resp.Error != "connection refused: http://localhost:26657" {
		t.Errorf("unexpected error message: got %s", resp.Error)
	}
}

// TestGRPCClient_GetGovernanceParams_Unimplemented tests backward compatibility.
func TestGRPCClient_GetGovernanceParams_Unimplemented(t *testing.T) {
	// Create mock client that returns Unimplemented error (old plugin)
	mockClient := &mockNetworkModuleClient{
		getGovernanceParamsFn: func(ctx context.Context, in *GovernanceParamsRequest, opts ...grpc.CallOption) (*GovernanceParamsResponse, error) {
			return nil, status.Errorf(codes.Unimplemented, "method GetGovernanceParams not implemented")
		},
	}

	client := &GRPCClient{client: mockClient}

	// Execute
	resp, err := client.GetGovernanceParams("http://localhost:26657", "devnet")

	// Verify
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if resp != nil {
		t.Errorf("expected nil response, got %+v", resp)
	}

	// Check that error is Unimplemented
	if s, ok := status.FromError(err); ok {
		if s.Code() != codes.Unimplemented {
			t.Errorf("unexpected error code: got %v, want %v", s.Code(), codes.Unimplemented)
		}
	} else {
		t.Error("error is not a gRPC status error")
	}
}

// TestGRPCClient_GetGovernanceParams_Timeout tests context timeout.
func TestGRPCClient_GetGovernanceParams_Timeout(t *testing.T) {
	// Create mock client that blocks until context is cancelled
	mockClient := &mockNetworkModuleClient{
		getGovernanceParamsFn: func(ctx context.Context, in *GovernanceParamsRequest, opts ...grpc.CallOption) (*GovernanceParamsResponse, error) {
			// Block until context is cancelled (simulating slow network)
			<-ctx.Done()
			return nil, ctx.Err()
		},
	}

	client := &GRPCClient{client: mockClient}

	// Execute (will timeout after 5 seconds based on implementation)
	start := time.Now()
	resp, err := client.GetGovernanceParams("http://localhost:26657", "devnet")
	elapsed := time.Since(start)

	// Verify
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if resp != nil {
		t.Errorf("expected nil response on timeout, got %+v", resp)
	}

	// Verify timeout happened around 5 seconds (±1s tolerance)
	if elapsed < 4*time.Second || elapsed > 6*time.Second {
		t.Errorf("unexpected timeout duration: got %v, want ~5s", elapsed)
	}
}

// TestGRPCClient_GetGovernanceParams_EmptyEndpoint tests validation.
func TestGRPCClient_GetGovernanceParams_EmptyEndpoint(t *testing.T) {
	// Create mock client that validates empty endpoint
	mockClient := &mockNetworkModuleClient{
		getGovernanceParamsFn: func(ctx context.Context, in *GovernanceParamsRequest, opts ...grpc.CallOption) (*GovernanceParamsResponse, error) {
			// This test verifies that the client passes through empty endpoints
			// The actual validation should happen in the plugin implementation
			if in.RpcEndpoint != "" {
				t.Errorf("expected empty endpoint, got %s", in.RpcEndpoint)
			}
			return &GovernanceParamsResponse{
				Error: "rpc_endpoint is required",
			}, nil
		},
	}

	client := &GRPCClient{client: mockClient}

	// Execute
	resp, err := client.GetGovernanceParams("", "devnet")

	// Verify - client should pass request through, validation is plugin's responsibility
	if err != nil {
		t.Fatalf("unexpected client error: %v", err)
	}
	if resp.Error == "" {
		t.Error("expected plugin to return error for empty endpoint")
	}
}
