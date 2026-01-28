// internal/daemon/server/ante/ante_test.go
package ante

import (
	"context"
	"testing"

	v1 "github.com/altuslabsxyz/devnet-builder/api/proto/gen/v1"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/types"
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
