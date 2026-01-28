// internal/daemon/server/ante/reference_validator_test.go
package ante

import (
	"context"
	"errors"
	"testing"

	v1 "github.com/altuslabsxyz/devnet-builder/api/proto/gen/v1"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/types"
)

// mockStore implements the Store interface for testing.
type mockStore struct {
	devnets map[string]*types.Devnet
	nodes   map[string]*types.Node
}

func newMockStore() *mockStore {
	return &mockStore{
		devnets: make(map[string]*types.Devnet),
		nodes:   make(map[string]*types.Node),
	}
}

func (m *mockStore) GetDevnet(ctx context.Context, namespace, name string) (*types.Devnet, error) {
	key := namespace + "/" + name
	if d, ok := m.devnets[key]; ok {
		return d, nil
	}
	return nil, errors.New("not found")
}

func (m *mockStore) GetNode(ctx context.Context, namespace, devnetName string, index int) (*types.Node, error) {
	return nil, errors.New("not found")
}

// mockNetworkService implements NetworkService for testing.
type mockNetworkService struct {
	networks map[string]bool
}

func newMockNetworkService() *mockNetworkService {
	return &mockNetworkService{networks: map[string]bool{"stable": true, "osmosis": true}}
}

func (m *mockNetworkService) GetNetworkInfo(ctx context.Context, req *v1.GetNetworkInfoRequest) (*v1.GetNetworkInfoResponse, error) {
	if m.networks[req.Name] {
		return &v1.GetNetworkInfoResponse{Network: &v1.NetworkInfo{Name: req.Name}}, nil
	}
	return nil, errors.New("not found")
}

func (m *mockNetworkService) ListBinaryVersions(ctx context.Context, req *v1.ListBinaryVersionsRequest) (*v1.ListBinaryVersionsResponse, error) {
	return &v1.ListBinaryVersionsResponse{}, nil
}

func TestReferenceValidator_DevnetReferences(t *testing.T) {
	store := newMockStore()
	networkSvc := newMockNetworkService()
	v := NewReferenceValidator(store, networkSvc)
	ctx := context.Background()

	tests := []struct {
		name      string
		namespace string
		spec      *v1.DevnetSpec
		wantErr   bool
		field     string
	}{
		{
			name:      "valid network",
			namespace: "default",
			spec:      &v1.DevnetSpec{Plugin: "stable"},
			wantErr:   false,
		},
		{
			name:      "nil spec is ok",
			namespace: "default",
			spec:      nil,
			wantErr:   false,
		},
		{
			name:      "unknown network",
			namespace: "default",
			spec:      &v1.DevnetSpec{Plugin: "unknown-network"},
			wantErr:   true,
			field:     "spec.plugin",
		},
		{
			name:      "empty plugin is ok",
			namespace: "default",
			spec:      &v1.DevnetSpec{Plugin: ""},
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.ValidateDevnetReferences(ctx, tt.namespace, tt.spec)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateDevnetReferences() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.field != "" {
				if ve, ok := err.(*ValidationError); ok {
					if ve.Field != tt.field {
						t.Errorf("ValidateDevnetReferences() field = %v, want %v", ve.Field, tt.field)
					}
				}
			}
		})
	}
}

func TestReferenceValidator_UpgradeReferences(t *testing.T) {
	store := newMockStore()
	store.devnets["default/my-devnet"] = &types.Devnet{
		Metadata: types.ResourceMeta{Name: "my-devnet", Namespace: "default"},
		Spec:     types.DevnetSpec{Plugin: "stable"},
	}
	networkSvc := newMockNetworkService()
	v := NewReferenceValidator(store, networkSvc)
	ctx := context.Background()

	tests := []struct {
		name      string
		namespace string
		devnetRef string
		spec      *v1.UpgradeSpec
		wantErr   bool
		field     string
	}{
		{
			name:      "valid devnet ref",
			namespace: "default",
			devnetRef: "my-devnet",
			spec:      &v1.UpgradeSpec{DevnetRef: "my-devnet", UpgradeName: "v2"},
			wantErr:   false,
		},
		{
			name:      "nil spec is ok",
			namespace: "default",
			devnetRef: "",
			spec:      nil,
			wantErr:   false,
		},
		{
			name:      "unknown devnet",
			namespace: "default",
			devnetRef: "nonexistent",
			spec:      &v1.UpgradeSpec{DevnetRef: "nonexistent", UpgradeName: "v2"},
			wantErr:   true,
			field:     "spec.devnet_ref",
		},
		{
			name:      "empty devnet ref is ok",
			namespace: "default",
			devnetRef: "",
			spec:      &v1.UpgradeSpec{DevnetRef: "", UpgradeName: "v2"},
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.ValidateUpgradeReferences(ctx, tt.namespace, tt.spec)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateUpgradeReferences() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.field != "" {
				if ve, ok := err.(*ValidationError); ok {
					if ve.Field != tt.field {
						t.Errorf("ValidateUpgradeReferences() field = %v, want %v", ve.Field, tt.field)
					}
				}
			}
		})
	}
}

func TestReferenceValidator_NodeReferences(t *testing.T) {
	store := newMockStore()
	store.devnets["default/my-devnet"] = &types.Devnet{
		Metadata: types.ResourceMeta{Name: "my-devnet", Namespace: "default"},
		Spec:     types.DevnetSpec{Plugin: "stable"},
	}
	networkSvc := newMockNetworkService()
	v := NewReferenceValidator(store, networkSvc)
	ctx := context.Background()

	tests := []struct {
		name       string
		namespace  string
		devnetName string
		index      int
		wantErr    bool
		field      string
	}{
		{
			name:       "valid devnet ref",
			namespace:  "default",
			devnetName: "my-devnet",
			index:      0,
			wantErr:    false,
		},
		{
			name:       "unknown devnet",
			namespace:  "default",
			devnetName: "nonexistent",
			index:      0,
			wantErr:    true,
			field:      "devnet_name",
		},
		{
			name:       "empty devnet name is ok",
			namespace:  "default",
			devnetName: "",
			index:      0,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.ValidateNodeReferences(ctx, tt.namespace, tt.devnetName, tt.index)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateNodeReferences() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.field != "" {
				if ve, ok := err.(*ValidationError); ok {
					if ve.Field != tt.field {
						t.Errorf("ValidateNodeReferences() field = %v, want %v", ve.Field, tt.field)
					}
				}
			}
		})
	}
}
