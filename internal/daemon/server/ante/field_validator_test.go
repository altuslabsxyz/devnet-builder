// internal/daemon/server/ante/field_validator_test.go
package ante

import (
	"context"
	"testing"

	v1 "github.com/altuslabsxyz/devnet-builder/api/proto/gen/v1"
)

func TestFieldValidator_CreateDevnetRequest(t *testing.T) {
	v := NewFieldValidator()
	ctx := context.Background()

	tests := []struct {
		name    string
		req     *v1.CreateDevnetRequest
		wantErr bool
		field   string
	}{
		{
			name:    "valid request",
			req:     &v1.CreateDevnetRequest{Name: "test", Spec: &v1.DevnetSpec{Plugin: "stable", Mode: "docker"}},
			wantErr: false,
		},
		{
			name:    "missing name",
			req:     &v1.CreateDevnetRequest{Name: "", Spec: &v1.DevnetSpec{Plugin: "stable", Mode: "docker"}},
			wantErr: true,
			field:   "name",
		},
		{
			name:    "missing spec",
			req:     &v1.CreateDevnetRequest{Name: "test", Spec: nil},
			wantErr: true,
			field:   "spec",
		},
		{
			name:    "missing plugin",
			req:     &v1.CreateDevnetRequest{Name: "test", Spec: &v1.DevnetSpec{Plugin: "", Mode: "docker"}},
			wantErr: true,
			field:   "spec.plugin",
		},
		{
			name:    "missing mode",
			req:     &v1.CreateDevnetRequest{Name: "test", Spec: &v1.DevnetSpec{Plugin: "stable", Mode: ""}},
			wantErr: true,
			field:   "spec.mode",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.ValidateCreateDevnetRequest(ctx, tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateCreateDevnetRequest() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.field != "" {
				ve, ok := err.(*ValidationError)
				if !ok {
					t.Errorf("expected *ValidationError, got %T", err)
					return
				}
				if ve.Field != tt.field {
					t.Errorf("error field = %s, want %s", ve.Field, tt.field)
				}
			}
		})
	}
}

func TestFieldValidator_ApplyDevnetRequest(t *testing.T) {
	v := NewFieldValidator()
	ctx := context.Background()

	tests := []struct {
		name    string
		req     *v1.ApplyDevnetRequest
		wantErr bool
		field   string
	}{
		{
			name:    "valid",
			req:     &v1.ApplyDevnetRequest{Name: "test", Spec: &v1.DevnetSpec{Plugin: "stable", Mode: "docker"}},
			wantErr: false,
		},
		{
			name:    "missing name",
			req:     &v1.ApplyDevnetRequest{Name: "", Spec: &v1.DevnetSpec{Plugin: "stable", Mode: "docker"}},
			wantErr: true,
			field:   "name",
		},
		{
			name:    "missing spec",
			req:     &v1.ApplyDevnetRequest{Name: "test", Spec: nil},
			wantErr: true,
			field:   "spec",
		},
		{
			name:    "missing plugin",
			req:     &v1.ApplyDevnetRequest{Name: "test", Spec: &v1.DevnetSpec{Plugin: "", Mode: "docker"}},
			wantErr: true,
			field:   "spec.plugin",
		},
		{
			name:    "missing mode",
			req:     &v1.ApplyDevnetRequest{Name: "test", Spec: &v1.DevnetSpec{Plugin: "stable", Mode: ""}},
			wantErr: true,
			field:   "spec.mode",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.ValidateApplyDevnetRequest(ctx, tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateApplyDevnetRequest() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.field != "" {
				ve, ok := err.(*ValidationError)
				if !ok {
					t.Errorf("expected *ValidationError, got %T", err)
					return
				}
				if ve.Field != tt.field {
					t.Errorf("error field = %s, want %s", ve.Field, tt.field)
				}
			}
		})
	}
}

func TestFieldValidator_UpdateDevnetRequest(t *testing.T) {
	v := NewFieldValidator()
	ctx := context.Background()

	tests := []struct {
		name    string
		req     *v1.UpdateDevnetRequest
		wantErr bool
		field   string
	}{
		{
			name:    "valid with spec",
			req:     &v1.UpdateDevnetRequest{Name: "test", Spec: &v1.DevnetSpec{Plugin: "stable"}},
			wantErr: false,
		},
		{
			name:    "valid without spec (partial update)",
			req:     &v1.UpdateDevnetRequest{Name: "test"},
			wantErr: false,
		},
		{
			name:    "missing name",
			req:     &v1.UpdateDevnetRequest{Name: ""},
			wantErr: true,
			field:   "name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.ValidateUpdateDevnetRequest(ctx, tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateUpdateDevnetRequest() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.field != "" {
				ve, ok := err.(*ValidationError)
				if !ok {
					t.Errorf("expected *ValidationError, got %T", err)
					return
				}
				if ve.Field != tt.field {
					t.Errorf("error field = %s, want %s", ve.Field, tt.field)
				}
			}
		})
	}
}

func TestFieldValidator_CreateUpgradeRequest(t *testing.T) {
	v := NewFieldValidator()
	ctx := context.Background()

	tests := []struct {
		name    string
		req     *v1.CreateUpgradeRequest
		wantErr bool
		field   string
	}{
		{
			name: "valid",
			req: &v1.CreateUpgradeRequest{
				Name: "upgrade-1",
				Spec: &v1.UpgradeSpec{DevnetRef: "my-devnet", UpgradeName: "v2"},
			},
			wantErr: false,
		},
		{
			name:    "missing name",
			req:     &v1.CreateUpgradeRequest{Name: "", Spec: &v1.UpgradeSpec{DevnetRef: "my-devnet", UpgradeName: "v2"}},
			wantErr: true,
			field:   "name",
		},
		{
			name:    "missing spec",
			req:     &v1.CreateUpgradeRequest{Name: "upgrade-1", Spec: nil},
			wantErr: true,
			field:   "spec",
		},
		{
			name:    "missing devnet_ref",
			req:     &v1.CreateUpgradeRequest{Name: "upgrade-1", Spec: &v1.UpgradeSpec{DevnetRef: "", UpgradeName: "v2"}},
			wantErr: true,
			field:   "spec.devnet_ref",
		},
		{
			name:    "missing upgrade_name",
			req:     &v1.CreateUpgradeRequest{Name: "upgrade-1", Spec: &v1.UpgradeSpec{DevnetRef: "my-devnet", UpgradeName: ""}},
			wantErr: true,
			field:   "spec.upgrade_name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.ValidateCreateUpgradeRequest(ctx, tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateCreateUpgradeRequest() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.field != "" {
				ve, ok := err.(*ValidationError)
				if !ok {
					t.Errorf("expected *ValidationError, got %T", err)
					return
				}
				if ve.Field != tt.field {
					t.Errorf("error field = %s, want %s", ve.Field, tt.field)
				}
			}
		})
	}
}

func TestFieldValidator_StartNodeRequest(t *testing.T) {
	v := NewFieldValidator()
	ctx := context.Background()

	tests := []struct {
		name    string
		req     *v1.StartNodeRequest
		wantErr bool
		field   string
	}{
		{
			name:    "valid",
			req:     &v1.StartNodeRequest{DevnetName: "my-devnet", Index: 0},
			wantErr: false,
		},
		{
			name:    "valid with index > 0",
			req:     &v1.StartNodeRequest{DevnetName: "my-devnet", Index: 5},
			wantErr: false,
		},
		{
			name:    "missing devnet_name",
			req:     &v1.StartNodeRequest{DevnetName: "", Index: 0},
			wantErr: true,
			field:   "devnet_name",
		},
		{
			name:    "negative index",
			req:     &v1.StartNodeRequest{DevnetName: "my-devnet", Index: -1},
			wantErr: true,
			field:   "index",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.ValidateStartNodeRequest(ctx, tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateStartNodeRequest() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.field != "" {
				ve, ok := err.(*ValidationError)
				if !ok {
					t.Errorf("expected *ValidationError, got %T", err)
					return
				}
				if ve.Field != tt.field {
					t.Errorf("error field = %s, want %s", ve.Field, tt.field)
				}
			}
		})
	}
}

func TestFieldValidator_GetNodeRequest(t *testing.T) {
	v := NewFieldValidator()
	ctx := context.Background()

	tests := []struct {
		name    string
		req     *v1.GetNodeRequest
		wantErr bool
		field   string
	}{
		{
			name:    "valid",
			req:     &v1.GetNodeRequest{DevnetName: "my-devnet", Index: 0},
			wantErr: false,
		},
		{
			name:    "valid with index > 0",
			req:     &v1.GetNodeRequest{DevnetName: "my-devnet", Index: 3},
			wantErr: false,
		},
		{
			name:    "missing devnet_name",
			req:     &v1.GetNodeRequest{DevnetName: "", Index: 0},
			wantErr: true,
			field:   "devnet_name",
		},
		{
			name:    "negative index",
			req:     &v1.GetNodeRequest{DevnetName: "my-devnet", Index: -1},
			wantErr: true,
			field:   "index",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.ValidateGetNodeRequest(ctx, tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateGetNodeRequest() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.field != "" {
				ve, ok := err.(*ValidationError)
				if !ok {
					t.Errorf("expected *ValidationError, got %T", err)
					return
				}
				if ve.Field != tt.field {
					t.Errorf("error field = %s, want %s", ve.Field, tt.field)
				}
			}
		})
	}
}

func TestFieldValidator_MultipleErrors(t *testing.T) {
	v := NewFieldValidator()
	ctx := context.Background()

	// Test that multiple validation errors are collected
	req := &v1.CreateDevnetRequest{
		Name: "",  // Missing name
		Spec: nil, // Missing spec
	}

	err := v.ValidateCreateDevnetRequest(ctx, req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Should be a MultiValidationError when multiple fields fail
	mve, ok := err.(*MultiValidationError)
	if !ok {
		t.Errorf("expected *MultiValidationError for multiple errors, got %T", err)
		return
	}

	if len(mve.Errors) < 2 {
		t.Errorf("expected at least 2 errors, got %d", len(mve.Errors))
	}
}
