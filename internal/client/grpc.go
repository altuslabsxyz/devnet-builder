// internal/client/grpc.go
package client

import (
	"context"
	"fmt"

	v1 "github.com/altuslabsxyz/devnet-builder/api/proto/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// GRPCClient wraps the gRPC DevnetServiceClient.
type GRPCClient struct {
	conn   *grpc.ClientConn
	devnet v1.DevnetServiceClient
}

// NewGRPCClient creates a new gRPC client connected to the daemon.
func NewGRPCClient(socketPath string) (*GRPCClient, error) {
	// Connect via Unix socket
	target := "unix://" + socketPath
	conn, err := grpc.NewClient(target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to daemon: %w", err)
	}

	return &GRPCClient{
		conn:   conn,
		devnet: v1.NewDevnetServiceClient(conn),
	}, nil
}

// Close closes the gRPC connection.
func (c *GRPCClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// CreateDevnet creates a new devnet.
func (c *GRPCClient) CreateDevnet(ctx context.Context, name string, spec *v1.DevnetSpec, labels map[string]string) (*v1.Devnet, error) {
	req := &v1.CreateDevnetRequest{
		Name:   name,
		Spec:   spec,
		Labels: labels,
	}

	resp, err := c.devnet.CreateDevnet(ctx, req)
	if err != nil {
		return nil, wrapGRPCError(err)
	}

	return resp, nil
}

// GetDevnet retrieves a devnet by name.
func (c *GRPCClient) GetDevnet(ctx context.Context, name string) (*v1.Devnet, error) {
	resp, err := c.devnet.GetDevnet(ctx, &v1.GetDevnetRequest{Name: name})
	if err != nil {
		return nil, wrapGRPCError(err)
	}
	return resp, nil
}

// ListDevnets lists all devnets.
func (c *GRPCClient) ListDevnets(ctx context.Context) ([]*v1.Devnet, error) {
	resp, err := c.devnet.ListDevnets(ctx, &v1.ListDevnetsRequest{})
	if err != nil {
		return nil, wrapGRPCError(err)
	}
	return resp.Devnets, nil
}

// DeleteDevnet deletes a devnet.
func (c *GRPCClient) DeleteDevnet(ctx context.Context, name string) error {
	_, err := c.devnet.DeleteDevnet(ctx, &v1.DeleteDevnetRequest{Name: name})
	if err != nil {
		return wrapGRPCError(err)
	}
	return nil
}

// StartDevnet starts a stopped devnet.
func (c *GRPCClient) StartDevnet(ctx context.Context, name string) (*v1.Devnet, error) {
	resp, err := c.devnet.StartDevnet(ctx, &v1.StartDevnetRequest{Name: name})
	if err != nil {
		return nil, wrapGRPCError(err)
	}
	return resp, nil
}

// StopDevnet stops a running devnet.
func (c *GRPCClient) StopDevnet(ctx context.Context, name string) (*v1.Devnet, error) {
	resp, err := c.devnet.StopDevnet(ctx, &v1.StopDevnetRequest{Name: name})
	if err != nil {
		return nil, wrapGRPCError(err)
	}
	return resp, nil
}

// wrapGRPCError converts gRPC errors to user-friendly messages.
func wrapGRPCError(err error) error {
	st, ok := status.FromError(err)
	if !ok {
		return err
	}

	switch st.Code() {
	case codes.NotFound:
		return fmt.Errorf("not found: %s", st.Message())
	case codes.AlreadyExists:
		return fmt.Errorf("already exists: %s", st.Message())
	case codes.InvalidArgument:
		return fmt.Errorf("invalid argument: %s", st.Message())
	case codes.Unavailable:
		return fmt.Errorf("daemon unavailable: %s", st.Message())
	default:
		return fmt.Errorf("%s: %s", st.Code(), st.Message())
	}
}
