package auth

import (
	"context"
)

// contextKey is a private type for context keys to avoid collisions.
type contextKey int

const (
	// userInfoKey is the context key for UserInfo.
	userInfoKey contextKey = iota
)

// UserInfo contains information about the authenticated user.
type UserInfo struct {
	// Name is the human-readable identifier for the user (from the API key).
	Name string
	// Namespaces is the list of namespaces this user can access.
	// A value of ["*"] grants access to all namespaces.
	Namespaces []string
}

// HasAllNamespaceAccess returns true if the user has access to all namespaces.
func (u *UserInfo) HasAllNamespaceAccess() bool {
	for _, ns := range u.Namespaces {
		if ns == "*" {
			return true
		}
	}
	return false
}

// CanAccessNamespace returns true if the user can access the given namespace.
func (u *UserInfo) CanAccessNamespace(namespace string) bool {
	if u.HasAllNamespaceAccess() {
		return true
	}
	for _, ns := range u.Namespaces {
		if ns == namespace {
			return true
		}
	}
	return false
}

// WithUserInfo returns a new context with the UserInfo attached.
func WithUserInfo(ctx context.Context, info *UserInfo) context.Context {
	return context.WithValue(ctx, userInfoKey, info)
}

// GetUserInfo extracts the UserInfo from the context.
// Returns nil if no UserInfo is present (e.g., local connection without auth).
func GetUserInfo(ctx context.Context) *UserInfo {
	info, _ := ctx.Value(userInfoKey).(*UserInfo)
	return info
}

// HasNamespaceAccess checks if the user in the context can access the given namespace.
// Returns true if:
// - No UserInfo is present (local connection, trusted)
// - UserInfo has ["*"] in namespaces (all access)
// - UserInfo has the specific namespace in its list
func HasNamespaceAccess(ctx context.Context, namespace string) bool {
	info := GetUserInfo(ctx)
	if info == nil {
		// No user info means local connection (trusted)
		return true
	}
	return info.CanAccessNamespace(namespace)
}

// MustGetUserInfo extracts the UserInfo from the context or panics.
// Use this only when you're certain the user info exists.
func MustGetUserInfo(ctx context.Context) *UserInfo {
	info := GetUserInfo(ctx)
	if info == nil {
		panic("auth: no user info in context")
	}
	return info
}

// IsAuthenticated returns true if there is a UserInfo in the context.
func IsAuthenticated(ctx context.Context) bool {
	return GetUserInfo(ctx) != nil
}

// IsLocalConnection returns true if there is no UserInfo in the context,
// indicating a local (trusted) connection via Unix socket.
func IsLocalConnection(ctx context.Context) bool {
	return GetUserInfo(ctx) == nil
}

// LocalUserInfo returns a UserInfo representing a local trusted user
// with full namespace access.
func LocalUserInfo() *UserInfo {
	return &UserInfo{
		Name:       "local",
		Namespaces: []string{"*"},
	}
}
