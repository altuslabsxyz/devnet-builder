// internal/daemon/server/ante/spec_validator_test.go
package ante

import (
	"context"
	"testing"

	v1 "github.com/altuslabsxyz/devnet-builder/api/proto/gen/v1"
)

func TestSpecValidator_DevnetSpec(t *testing.T) {
	v := NewSpecValidator()
	ctx := context.Background()

	tests := []struct {
		name    string
		spec    *v1.DevnetSpec
		wantErr bool
		field   string
	}{
		{
			name:    "valid spec",
			spec:    &v1.DevnetSpec{Plugin: "stable", Mode: "docker", Validators: 2},
			wantErr: false,
		},
		{
			name:    "nil spec is ok (field validator handles)",
			spec:    nil,
			wantErr: false,
		},
		{
			name:    "invalid mode",
			spec:    &v1.DevnetSpec{Plugin: "stable", Mode: "invalid"},
			wantErr: true,
			field:   "spec.mode",
		},
		{
			name:    "validators too high",
			spec:    &v1.DevnetSpec{Plugin: "stable", Mode: "docker", Validators: 10},
			wantErr: true,
			field:   "spec.validators",
		},
		{
			name:    "fullnodes too high",
			spec:    &v1.DevnetSpec{Plugin: "stable", Mode: "docker", FullNodes: 20},
			wantErr: true,
			field:   "spec.full_nodes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.ValidateDevnetSpec(ctx, tt.spec)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateDevnetSpec() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.field != "" {
				ve, ok := err.(*ValidationError)
				if !ok {
					me, ok := err.(*MultiValidationError)
					if ok && len(me.Errors) > 0 {
						ve = me.Errors[0]
					} else {
						t.Errorf("expected *ValidationError, got %T", err)
						return
					}
				}
				if ve.Field != tt.field {
					t.Errorf("error field = %s, want %s", ve.Field, tt.field)
				}
			}
		})
	}
}

func TestSpecValidator_UpgradeSpec(t *testing.T) {
	v := NewSpecValidator()
	ctx := context.Background()

	tests := []struct {
		name    string
		spec    *v1.UpgradeSpec
		wantErr bool
	}{
		{
			name:    "valid spec",
			spec:    &v1.UpgradeSpec{DevnetRef: "my-devnet", UpgradeName: "v2", TargetHeight: 100},
			wantErr: false,
		},
		{
			name:    "nil spec is ok",
			spec:    nil,
			wantErr: false,
		},
		{
			name:    "negative height",
			spec:    &v1.UpgradeSpec{DevnetRef: "my-devnet", UpgradeName: "v2", TargetHeight: -1},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.ValidateUpgradeSpec(ctx, tt.spec)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateUpgradeSpec() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
