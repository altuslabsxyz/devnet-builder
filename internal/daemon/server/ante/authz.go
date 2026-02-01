package ante

import (
	"context"

	"github.com/altuslabsxyz/devnet-builder/internal/auth"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// AuthzValidator validates authorization for requests.
// It checks if the authenticated user has access to the requested namespace.
type AuthzValidator struct{}

// NewAuthzValidator creates a new authorization validator.
func NewAuthzValidator() *AuthzValidator {
	return &AuthzValidator{}
}

// ValidateNamespaceAccess checks if the user in context has access to the namespace.
// For local connections (no user info in context), access is always granted.
// For remote connections, the user's allowed namespaces are checked.
func (v *AuthzValidator) ValidateNamespaceAccess(ctx context.Context, namespace string) error {
	// Normalize empty namespace to default
	if namespace == "" {
		namespace = types.DefaultNamespace
	}

	// Check if user has access to this namespace
	if !auth.HasNamespaceAccess(ctx, namespace) {
		userInfo := auth.GetUserInfo(ctx)
		if userInfo != nil {
			return status.Errorf(codes.PermissionDenied,
				"user %q does not have access to namespace %q", userInfo.Name, namespace)
		}
		return status.Errorf(codes.PermissionDenied,
			"access denied to namespace %q", namespace)
	}

	return nil
}
