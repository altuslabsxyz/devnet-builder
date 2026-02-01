// internal/daemon/server/ante/ante_test.go
package ante

import (
	"context"
	"testing"

	v1 "github.com/altuslabsxyz/devnet-builder/api/proto/gen/v1"
	"github.com/altuslabsxyz/devnet-builder/internal/auth"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestAnteHandler_ValidateCreateDevnet(t *testing.T) {
	store := newMockStore()
	networkSvc := newMockNetworkService()
	handler := New(store, networkSvc)
	ctx := context.Background()

	tests := []struct {
		name    string
		req     *v1.CreateDevnetRequest
		wantErr bool
	}{
		{
			name: "valid request",
			req: &v1.CreateDevnetRequest{
				Name: "test",
				Spec: &v1.DevnetSpec{Plugin: "stable", Mode: "docker", Validators: 2},
			},
			wantErr: false,
		},
		{
			name: "missing name (field validation)",
			req: &v1.CreateDevnetRequest{
				Name: "",
				Spec: &v1.DevnetSpec{Plugin: "stable", Mode: "docker"},
			},
			wantErr: true,
		},
		{
			name: "invalid mode (spec validation)",
			req: &v1.CreateDevnetRequest{
				Name: "test",
				Spec: &v1.DevnetSpec{Plugin: "stable", Mode: "invalid"},
			},
			wantErr: true,
		},
		{
			name: "unknown network (reference validation)",
			req: &v1.CreateDevnetRequest{
				Name: "test",
				Spec: &v1.DevnetSpec{Plugin: "unknown", Mode: "docker"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := handler.ValidateCreateDevnet(ctx, tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateCreateDevnet() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAnteHandler_ValidateApplyDevnet(t *testing.T) {
	store := newMockStore()
	networkSvc := newMockNetworkService()
	handler := New(store, networkSvc)
	ctx := context.Background()

	tests := []struct {
		name    string
		req     *v1.ApplyDevnetRequest
		wantErr bool
	}{
		{
			name: "valid request",
			req: &v1.ApplyDevnetRequest{
				Name: "test",
				Spec: &v1.DevnetSpec{Plugin: "stable", Mode: "docker"},
			},
			wantErr: false,
		},
		{
			name: "missing name",
			req: &v1.ApplyDevnetRequest{
				Name: "",
				Spec: &v1.DevnetSpec{Plugin: "stable", Mode: "docker"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := handler.ValidateApplyDevnet(ctx, tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateApplyDevnet() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAnteHandler_ValidateCreateUpgrade(t *testing.T) {
	store := newMockStore()
	store.devnets["default/my-devnet"] = &types.Devnet{
		Metadata: types.ResourceMeta{Name: "my-devnet", Namespace: "default"},
	}
	networkSvc := newMockNetworkService()
	handler := New(store, networkSvc)
	ctx := context.Background()

	tests := []struct {
		name    string
		req     *v1.CreateUpgradeRequest
		wantErr bool
	}{
		{
			name: "valid request",
			req: &v1.CreateUpgradeRequest{
				Name:      "upgrade-1",
				Namespace: "default",
				Spec:      &v1.UpgradeSpec{DevnetRef: "my-devnet", UpgradeName: "v2", TargetHeight: 100},
			},
			wantErr: false,
		},
		{
			name: "missing devnet_ref (field validation)",
			req: &v1.CreateUpgradeRequest{
				Name:      "upgrade-1",
				Namespace: "default",
				Spec:      &v1.UpgradeSpec{DevnetRef: "", UpgradeName: "v2"},
			},
			wantErr: true,
		},
		{
			name: "devnet not found (reference validation)",
			req: &v1.CreateUpgradeRequest{
				Name:      "upgrade-1",
				Namespace: "default",
				Spec:      &v1.UpgradeSpec{DevnetRef: "nonexistent", UpgradeName: "v2"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := handler.ValidateCreateUpgrade(ctx, tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateCreateUpgrade() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestAnteHandler_ValidateGetNodeHealth_Authorization verifies that GetNodeHealth
// checks namespace authorization. This is a security-critical test.
func TestAnteHandler_ValidateGetNodeHealth_Authorization(t *testing.T) {
	store := newMockStore()
	networkSvc := newMockNetworkService()
	handler := New(store, networkSvc)

	tests := []struct {
		name      string
		namespace string
		userNs    []string // namespaces the user has access to
		wantCode  codes.Code
	}{
		{
			name:      "user has access to namespace",
			namespace: "team-a",
			userNs:    []string{"team-a", "team-b"},
			wantCode:  codes.OK,
		},
		{
			name:      "user does not have access to namespace",
			namespace: "team-a",
			userNs:    []string{"team-b"},
			wantCode:  codes.PermissionDenied,
		},
		{
			name:      "wildcard access grants all namespaces",
			namespace: "any-namespace",
			userNs:    []string{"*"},
			wantCode:  codes.OK,
		},
		{
			name:      "default namespace access",
			namespace: "", // empty defaults to "default"
			userNs:    []string{"default"},
			wantCode:  codes.OK,
		},
		{
			name:      "no access to default namespace",
			namespace: "", // empty defaults to "default"
			userNs:    []string{"other"},
			wantCode:  codes.PermissionDenied,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create context with user info (simulating remote connection)
			userInfo := &auth.UserInfo{
				Name:       "test-user",
				Namespaces: tt.userNs,
			}
			ctx := auth.WithUserInfo(context.Background(), userInfo)

			req := &v1.GetNodeHealthRequest{
				DevnetName: "my-devnet",
				Index:      0,
				Namespace:  tt.namespace,
			}

			err := handler.ValidateGetNodeHealth(ctx, req)

			if tt.wantCode == codes.OK {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			} else {
				if err == nil {
					t.Errorf("expected error with code %v, got nil", tt.wantCode)
					return
				}
				st, ok := status.FromError(err)
				if !ok {
					t.Errorf("expected gRPC status error, got %v", err)
					return
				}
				if st.Code() != tt.wantCode {
					t.Errorf("expected code %v, got %v", tt.wantCode, st.Code())
				}
			}
		})
	}
}
