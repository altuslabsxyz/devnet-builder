package auth

import (
	"context"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	// AuthorizationHeader is the metadata key for the authorization header.
	AuthorizationHeader = "authorization"
	// BearerPrefix is the prefix for Bearer token authentication.
	BearerPrefix = "Bearer "
)

// IsLocalConnFunc is a function that determines if the current request
// is from a local connection (e.g., Unix socket).
type IsLocalConnFunc func(ctx context.Context) bool

// NewAuthInterceptor creates a gRPC unary server interceptor for API key authentication.
//
// The interceptor:
// - Allows requests from local connections (Unix socket) without authentication
// - Requires a valid API key in the "authorization" metadata header for remote connections
// - Extracts user info from the API key and injects it into the context
// - Returns codes.Unauthenticated if authentication fails
//
// SECURITY NOTE: This interceptor does not implement rate limiting. For production
// deployments, rate limiting should be handled at the network layer (e.g., via a
// reverse proxy like nginx or envoy) to protect against brute-force attacks.
func NewAuthInterceptor(keyStore KeyStore, isLocalConn IsLocalConnFunc) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// Check if this is a local connection
		if isLocalConn != nil && isLocalConn(ctx) {
			// Local connections are trusted, no auth required
			return handler(ctx, req)
		}

		// Remote connection - require authentication
		userInfo, err := authenticateRequest(ctx, keyStore)
		if err != nil {
			return nil, err
		}

		// Inject user info into context
		ctx = WithUserInfo(ctx, userInfo)
		return handler(ctx, req)
	}
}

// NewStreamAuthInterceptor creates a gRPC stream server interceptor for API key authentication.
//
// The interceptor follows the same logic as the unary interceptor:
// - Allows local connections without authentication
// - Requires valid API key for remote connections
// - Injects user info into the stream context
func NewStreamAuthInterceptor(keyStore KeyStore, isLocalConn IsLocalConnFunc) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx := ss.Context()

		// Check if this is a local connection
		if isLocalConn != nil && isLocalConn(ctx) {
			// Local connections are trusted, no auth required
			return handler(srv, ss)
		}

		// Remote connection - require authentication
		userInfo, err := authenticateRequest(ctx, keyStore)
		if err != nil {
			return err
		}

		// Wrap the stream with authenticated context
		wrapped := &wrappedServerStream{
			ServerStream: ss,
			ctx:          WithUserInfo(ctx, userInfo),
		}
		return handler(srv, wrapped)
	}
}

// authenticateRequest extracts and validates the API key from the request metadata.
func authenticateRequest(ctx context.Context, keyStore KeyStore) (*UserInfo, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing metadata")
	}

	// Get authorization header
	authHeaders := md.Get(AuthorizationHeader)
	if len(authHeaders) == 0 {
		return nil, status.Error(codes.Unauthenticated, "missing authorization header")
	}

	// Parse Bearer token
	authHeader := authHeaders[0]
	if !strings.HasPrefix(authHeader, BearerPrefix) {
		return nil, status.Error(codes.Unauthenticated, "invalid authorization format, expected 'Bearer <token>'")
	}

	token := strings.TrimPrefix(authHeader, BearerPrefix)
	if token == "" {
		return nil, status.Error(codes.Unauthenticated, "empty authorization token")
	}

	// Validate key format and look up the key.
	// SECURITY: Use identical error messages for all key-related failures
	// to prevent timing attacks that could leak information about key structure.
	if !IsValidKeyFormat(token) {
		return nil, status.Error(codes.Unauthenticated, "authentication failed")
	}

	apiKey, ok := keyStore.Get(token)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "authentication failed")
	}

	// Create user info from API key
	return &UserInfo{
		Name:       apiKey.Name,
		Namespaces: apiKey.Namespaces,
	}, nil
}

// wrappedServerStream wraps a grpc.ServerStream to override the context.
type wrappedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

// Context returns the wrapped context with user info.
func (w *wrappedServerStream) Context() context.Context {
	return w.ctx
}

// ExtractBearerToken extracts the API key from a Bearer token string.
// Returns an empty string if the format is invalid.
func ExtractBearerToken(authHeader string) string {
	if !strings.HasPrefix(authHeader, BearerPrefix) {
		return ""
	}
	return strings.TrimPrefix(authHeader, BearerPrefix)
}

// FormatBearerToken formats an API key as a Bearer token.
func FormatBearerToken(apiKey string) string {
	return BearerPrefix + apiKey
}
