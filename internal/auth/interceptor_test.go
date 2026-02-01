package auth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// mockKeyStore implements KeyStore for testing.
type mockKeyStore struct {
	keys map[string]*APIKey
}

func newMockKeyStore() *mockKeyStore {
	return &mockKeyStore{
		keys: make(map[string]*APIKey),
	}
}

func (m *mockKeyStore) Load() error             { return nil }
func (m *mockKeyStore) Save() error             { return nil }
func (m *mockKeyStore) List() []*APIKey         { return nil }
func (m *mockKeyStore) Revoke(key string) error { return nil }
func (m *mockKeyStore) Create(name string, ns []string) (*APIKey, error) {
	key, _ := GenerateAPIKey()
	apiKey := &APIKey{Key: key, Name: name, Namespaces: ns}
	m.keys[key] = apiKey
	return apiKey, nil
}
func (m *mockKeyStore) Get(key string) (*APIKey, bool) {
	k, ok := m.keys[key]
	return k, ok
}

func (m *mockKeyStore) addKey(key, name string, namespaces []string) {
	m.keys[key] = &APIKey{
		Key:        key,
		Name:       name,
		Namespaces: namespaces,
	}
}

// mockHandler is a test gRPC handler.
func mockHandler(ctx context.Context, req interface{}) (interface{}, error) {
	// Return the user info if present (for testing)
	info := GetUserInfo(ctx)
	if info != nil {
		return info.Name, nil
	}
	return "no-user", nil
}

func TestNewAuthInterceptor_LocalConnection(t *testing.T) {
	store := newMockKeyStore()
	isLocal := func(ctx context.Context) bool { return true }

	interceptor := NewAuthInterceptor(store, isLocal)

	ctx := context.Background()
	result, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{}, mockHandler)

	require.NoError(t, err)
	assert.Equal(t, "no-user", result) // No user injected for local
}

func TestNewAuthInterceptor_RemoteWithValidKey(t *testing.T) {
	store := newMockKeyStore()
	testKey := "devnet_0123456789abcdef0123456789abcdef"
	store.addKey(testKey, "alice", []string{"team-a"})

	isLocal := func(ctx context.Context) bool { return false }
	interceptor := NewAuthInterceptor(store, isLocal)

	// Create context with authorization metadata
	md := metadata.New(map[string]string{
		AuthorizationHeader: FormatBearerToken(testKey),
	})
	ctx := metadata.NewIncomingContext(context.Background(), md)

	result, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{}, mockHandler)

	require.NoError(t, err)
	assert.Equal(t, "alice", result) // User was injected
}

func TestNewAuthInterceptor_RemoteWithoutMetadata(t *testing.T) {
	store := newMockKeyStore()
	isLocal := func(ctx context.Context) bool { return false }

	interceptor := NewAuthInterceptor(store, isLocal)

	ctx := context.Background()
	_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{}, mockHandler)

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, st.Code())
	assert.Contains(t, st.Message(), "missing metadata")
}

func TestNewAuthInterceptor_RemoteWithoutAuthHeader(t *testing.T) {
	store := newMockKeyStore()
	isLocal := func(ctx context.Context) bool { return false }

	interceptor := NewAuthInterceptor(store, isLocal)

	md := metadata.New(map[string]string{})
	ctx := metadata.NewIncomingContext(context.Background(), md)

	_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{}, mockHandler)

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, st.Code())
	assert.Contains(t, st.Message(), "missing authorization header")
}

func TestNewAuthInterceptor_RemoteWithInvalidFormat(t *testing.T) {
	store := newMockKeyStore()
	isLocal := func(ctx context.Context) bool { return false }

	interceptor := NewAuthInterceptor(store, isLocal)

	// Wrong format (not Bearer)
	md := metadata.New(map[string]string{
		AuthorizationHeader: "Basic dXNlcjpwYXNz",
	})
	ctx := metadata.NewIncomingContext(context.Background(), md)

	_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{}, mockHandler)

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, st.Code())
	assert.Contains(t, st.Message(), "invalid authorization format")
}

func TestNewAuthInterceptor_RemoteWithInvalidKeyFormat(t *testing.T) {
	store := newMockKeyStore()
	isLocal := func(ctx context.Context) bool { return false }

	interceptor := NewAuthInterceptor(store, isLocal)

	// Invalid key format
	md := metadata.New(map[string]string{
		AuthorizationHeader: "Bearer invalid_key_format",
	})
	ctx := metadata.NewIncomingContext(context.Background(), md)

	_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{}, mockHandler)

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, st.Code())
	// Use generic error message to prevent timing attacks
	assert.Equal(t, "authentication failed", st.Message())
}

func TestNewAuthInterceptor_RemoteWithUnknownKey(t *testing.T) {
	store := newMockKeyStore()
	isLocal := func(ctx context.Context) bool { return false }

	interceptor := NewAuthInterceptor(store, isLocal)

	// Valid format but unknown key
	md := metadata.New(map[string]string{
		AuthorizationHeader: "Bearer devnet_0123456789abcdef0123456789abcdef",
	})
	ctx := metadata.NewIncomingContext(context.Background(), md)

	_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{}, mockHandler)

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, st.Code())
	// Use generic error message to prevent timing attacks
	assert.Equal(t, "authentication failed", st.Message())
}

func TestNewAuthInterceptor_NilIsLocalFunc(t *testing.T) {
	store := newMockKeyStore()
	testKey := "devnet_0123456789abcdef0123456789abcdef"
	store.addKey(testKey, "alice", []string{"*"})

	// nil isLocalConn function should treat all as remote
	interceptor := NewAuthInterceptor(store, nil)

	// Without auth should fail
	ctx := context.Background()
	_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{}, mockHandler)
	require.Error(t, err)

	// With auth should succeed
	md := metadata.New(map[string]string{
		AuthorizationHeader: FormatBearerToken(testKey),
	})
	ctx = metadata.NewIncomingContext(context.Background(), md)
	result, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{}, mockHandler)
	require.NoError(t, err)
	assert.Equal(t, "alice", result)
}

func TestExtractBearerToken(t *testing.T) {
	tests := []struct {
		header string
		token  string
	}{
		{"Bearer devnet_abc123", "devnet_abc123"},
		{"Bearer ", ""},
		{"bearer devnet_abc123", ""}, // case-sensitive
		{"Basic abc123", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.header, func(t *testing.T) {
			assert.Equal(t, tt.token, ExtractBearerToken(tt.header))
		})
	}
}

func TestFormatBearerToken(t *testing.T) {
	assert.Equal(t, "Bearer devnet_abc123", FormatBearerToken("devnet_abc123"))
}

// mockServerStream implements grpc.ServerStream for testing.
type mockServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (m *mockServerStream) Context() context.Context {
	return m.ctx
}

func TestNewStreamAuthInterceptor_LocalConnection(t *testing.T) {
	store := newMockKeyStore()
	isLocal := func(ctx context.Context) bool { return true }

	interceptor := NewStreamAuthInterceptor(store, isLocal)

	stream := &mockServerStream{ctx: context.Background()}
	handlerCalled := false
	handler := func(srv interface{}, stream grpc.ServerStream) error {
		handlerCalled = true
		// No user should be in context for local
		assert.Nil(t, GetUserInfo(stream.Context()))
		return nil
	}

	err := interceptor(nil, stream, &grpc.StreamServerInfo{}, handler)

	require.NoError(t, err)
	assert.True(t, handlerCalled)
}

func TestNewStreamAuthInterceptor_RemoteWithValidKey(t *testing.T) {
	store := newMockKeyStore()
	testKey := "devnet_0123456789abcdef0123456789abcdef"
	store.addKey(testKey, "bob", []string{"team-b"})

	isLocal := func(ctx context.Context) bool { return false }
	interceptor := NewStreamAuthInterceptor(store, isLocal)

	md := metadata.New(map[string]string{
		AuthorizationHeader: FormatBearerToken(testKey),
	})
	ctx := metadata.NewIncomingContext(context.Background(), md)
	stream := &mockServerStream{ctx: ctx}

	handlerCalled := false
	handler := func(srv interface{}, stream grpc.ServerStream) error {
		handlerCalled = true
		info := GetUserInfo(stream.Context())
		assert.NotNil(t, info)
		assert.Equal(t, "bob", info.Name)
		return nil
	}

	err := interceptor(nil, stream, &grpc.StreamServerInfo{}, handler)

	require.NoError(t, err)
	assert.True(t, handlerCalled)
}

func TestNewStreamAuthInterceptor_RemoteWithoutAuth(t *testing.T) {
	store := newMockKeyStore()
	isLocal := func(ctx context.Context) bool { return false }

	interceptor := NewStreamAuthInterceptor(store, isLocal)

	stream := &mockServerStream{ctx: context.Background()}
	handler := func(srv interface{}, stream grpc.ServerStream) error {
		t.Fatal("handler should not be called")
		return nil
	}

	err := interceptor(nil, stream, &grpc.StreamServerInfo{}, handler)

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, st.Code())
}

// TestAuthErrorMessagesAreIdentical verifies that all key-related auth failures
// return the SAME error message to prevent timing attacks.
// An attacker should not be able to distinguish between:
// - Invalid key format
// - Unknown key
// - Any other key validation failure
func TestAuthErrorMessagesAreIdentical(t *testing.T) {
	store := newMockKeyStore()
	// Add a valid key so we can compare errors
	store.addKey("devnet_0123456789abcdef0123456789abcdef", "alice", []string{"*"})
	isLocal := func(ctx context.Context) bool { return false }
	interceptor := NewAuthInterceptor(store, isLocal)

	// Collect error messages for different failure scenarios
	var errorMessages []string

	// Case 1: Invalid key format (not devnet_ prefix)
	md := metadata.New(map[string]string{
		AuthorizationHeader: "Bearer invalid_key_format_123",
	})
	ctx := metadata.NewIncomingContext(context.Background(), md)
	_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{}, mockHandler)
	require.Error(t, err)
	st, _ := status.FromError(err)
	errorMessages = append(errorMessages, st.Message())

	// Case 2: Valid format but unknown key
	md = metadata.New(map[string]string{
		AuthorizationHeader: "Bearer devnet_ffffffffffffffffffffffffffffffff",
	})
	ctx = metadata.NewIncomingContext(context.Background(), md)
	_, err = interceptor(ctx, nil, &grpc.UnaryServerInfo{}, mockHandler)
	require.Error(t, err)
	st, _ = status.FromError(err)
	errorMessages = append(errorMessages, st.Message())

	// Case 3: Too short key (wrong length)
	md = metadata.New(map[string]string{
		AuthorizationHeader: "Bearer devnet_tooshort",
	})
	ctx = metadata.NewIncomingContext(context.Background(), md)
	_, err = interceptor(ctx, nil, &grpc.UnaryServerInfo{}, mockHandler)
	require.Error(t, err)
	st, _ = status.FromError(err)
	errorMessages = append(errorMessages, st.Message())

	// All error messages MUST be identical to prevent timing attacks
	expected := "authentication failed"
	for i, msg := range errorMessages {
		assert.Equal(t, expected, msg, "Case %d: error message should be generic", i+1)
	}
}
