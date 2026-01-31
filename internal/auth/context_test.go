package auth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWithUserInfo_GetUserInfo(t *testing.T) {
	ctx := context.Background()

	// No user info initially
	assert.Nil(t, GetUserInfo(ctx))

	// Add user info
	info := &UserInfo{
		Name:       "alice",
		Namespaces: []string{"team-a", "team-b"},
	}
	ctx = WithUserInfo(ctx, info)

	// Should be retrievable
	retrieved := GetUserInfo(ctx)
	assert.NotNil(t, retrieved)
	assert.Equal(t, "alice", retrieved.Name)
	assert.Equal(t, []string{"team-a", "team-b"}, retrieved.Namespaces)
}

func TestUserInfo_CanAccessNamespace(t *testing.T) {
	tests := []struct {
		name       string
		namespaces []string
		target     string
		canAccess  bool
	}{
		{"wildcard access", []string{"*"}, "any-namespace", true},
		{"exact match", []string{"team-a", "team-b"}, "team-a", true},
		{"no match", []string{"team-a", "team-b"}, "team-c", false},
		{"empty namespaces", []string{}, "any-namespace", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &UserInfo{Namespaces: tt.namespaces}
			assert.Equal(t, tt.canAccess, info.CanAccessNamespace(tt.target))
		})
	}
}

func TestUserInfo_HasAllNamespaceAccess(t *testing.T) {
	tests := []struct {
		name       string
		namespaces []string
		hasAll     bool
	}{
		{"wildcard only", []string{"*"}, true},
		{"wildcard with others", []string{"team-a", "*"}, true},
		{"no wildcard", []string{"team-a", "team-b"}, false},
		{"empty", []string{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &UserInfo{Namespaces: tt.namespaces}
			assert.Equal(t, tt.hasAll, info.HasAllNamespaceAccess())
		})
	}
}

func TestHasNamespaceAccess(t *testing.T) {
	tests := []struct {
		name      string
		userInfo  *UserInfo
		namespace string
		hasAccess bool
	}{
		{
			name:      "no user info (local connection)",
			userInfo:  nil,
			namespace: "any-namespace",
			hasAccess: true, // Local is trusted
		},
		{
			name:      "wildcard access",
			userInfo:  &UserInfo{Namespaces: []string{"*"}},
			namespace: "any-namespace",
			hasAccess: true,
		},
		{
			name:      "matching namespace",
			userInfo:  &UserInfo{Namespaces: []string{"team-a"}},
			namespace: "team-a",
			hasAccess: true,
		},
		{
			name:      "no matching namespace",
			userInfo:  &UserInfo{Namespaces: []string{"team-a"}},
			namespace: "team-b",
			hasAccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.userInfo != nil {
				ctx = WithUserInfo(ctx, tt.userInfo)
			}
			assert.Equal(t, tt.hasAccess, HasNamespaceAccess(ctx, tt.namespace))
		})
	}
}

func TestIsAuthenticated(t *testing.T) {
	ctx := context.Background()
	assert.False(t, IsAuthenticated(ctx))

	ctx = WithUserInfo(ctx, &UserInfo{Name: "alice"})
	assert.True(t, IsAuthenticated(ctx))
}

func TestIsLocalConnection(t *testing.T) {
	ctx := context.Background()
	assert.True(t, IsLocalConnection(ctx)) // No user info = local

	ctx = WithUserInfo(ctx, &UserInfo{Name: "alice"})
	assert.False(t, IsLocalConnection(ctx)) // Has user info = remote
}

func TestLocalUserInfo(t *testing.T) {
	info := LocalUserInfo()
	assert.Equal(t, "local", info.Name)
	assert.Equal(t, []string{"*"}, info.Namespaces)
	assert.True(t, info.HasAllNamespaceAccess())
}

func TestMustGetUserInfo_Panics(t *testing.T) {
	ctx := context.Background()

	assert.Panics(t, func() {
		MustGetUserInfo(ctx)
	})
}

func TestMustGetUserInfo_Success(t *testing.T) {
	ctx := context.Background()
	info := &UserInfo{Name: "alice"}
	ctx = WithUserInfo(ctx, info)

	assert.NotPanics(t, func() {
		retrieved := MustGetUserInfo(ctx)
		assert.Equal(t, "alice", retrieved.Name)
	})
}
