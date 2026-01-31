package server

import (
	"context"
	"strings"

	v1 "github.com/altuslabsxyz/devnet-builder/api/proto/gen/v1"
	"github.com/altuslabsxyz/devnet-builder/internal/auth"
	"google.golang.org/grpc/peer"
)

// AuthService implements the AuthService gRPC service.
type AuthService struct {
	v1.UnimplementedAuthServiceServer
}

// NewAuthService creates a new AuthService.
func NewAuthService() *AuthService {
	return &AuthService{}
}

// Ping tests connectivity to the server.
func (s *AuthService) Ping(ctx context.Context, req *v1.PingRequest) (*v1.PingResponse, error) {
	return &v1.PingResponse{
		ServerVersion: "1.0.0", // TODO: inject actual version
	}, nil
}

// WhoAmI returns information about the authenticated user.
func (s *AuthService) WhoAmI(ctx context.Context, req *v1.WhoAmIRequest) (*v1.WhoAmIResponse, error) {
	// Check if user info is in context (remote connection)
	userInfo := auth.GetUserInfo(ctx)
	if userInfo != nil {
		return &v1.WhoAmIResponse{
			Name:       userInfo.Name,
			Namespaces: userInfo.Namespaces,
		}, nil
	}

	// Local connection - return local user info
	return &v1.WhoAmIResponse{
		Name:       "local",
		Namespaces: []string{"*"},
	}, nil
}

// IsLocalConnection determines if the gRPC request came from a Unix socket.
// This is used by the auth interceptor to skip authentication for local connections.
func IsLocalConnection(ctx context.Context) bool {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return false
	}

	// Unix socket addresses have the format "@" or a file path
	// TCP addresses have the format "ip:port"
	addr := p.Addr.String()

	// Unix socket addresses start with @ or / or are empty network addresses
	if strings.HasPrefix(addr, "@") || strings.HasPrefix(addr, "/") {
		return true
	}

	// Check the network type
	if p.Addr.Network() == "unix" {
		return true
	}

	return false
}
