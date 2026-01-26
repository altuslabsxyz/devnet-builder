// internal/daemon/types/types_test.go
package types

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResourceMeta_JSON(t *testing.T) {
	meta := ResourceMeta{
		Name:       "test-devnet",
		Generation: 1,
		CreatedAt:  time.Date(2026, 1, 23, 10, 0, 0, 0, time.UTC),
		UpdatedAt:  time.Date(2026, 1, 23, 11, 0, 0, 0, time.UTC),
		Labels:     map[string]string{"env": "test"},
	}

	data, err := json.Marshal(meta)
	require.NoError(t, err)

	var decoded ResourceMeta
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, meta.Name, decoded.Name)
	assert.Equal(t, meta.Generation, decoded.Generation)
	assert.Equal(t, meta.Labels["env"], decoded.Labels["env"])
}

func TestDevnet_DefaultPhase(t *testing.T) {
	devnet := &Devnet{
		Metadata: ResourceMeta{Name: "test"},
		Spec:     DevnetSpec{Plugin: "stable", Validators: 4},
	}

	assert.Empty(t, devnet.Status.Phase)
}

func TestDevnetStatus_SDKVersionHistory(t *testing.T) {
	status := DevnetStatus{
		Phase:      PhaseRunning,
		SDKVersion: "0.53.4",
		SDKVersionHistory: []SDKVersionChange{
			{
				FromVersion: "0.50.9",
				ToVersion:   "0.53.4",
				Height:      15000,
				UpgradeRef:  "v2-upgrade",
			},
		},
	}

	assert.Equal(t, PhaseRunning, status.Phase)
	assert.Equal(t, "0.53.4", status.SDKVersion)
	assert.Len(t, status.SDKVersionHistory, 1)
	assert.Equal(t, "0.50.9", status.SDKVersionHistory[0].FromVersion)
}

func TestResourceMetaNamespace(t *testing.T) {
	meta := ResourceMeta{
		Namespace: "production",
	}
	if meta.Namespace != "production" {
		t.Errorf("expected namespace 'production', got %q", meta.Namespace)
	}
}

func TestResourceMetaFullName(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		want      string
	}{
		{"devnet", "prod", "prod/devnet"},
		{"devnet", "", "default/devnet"},
		{"devnet", "default", "default/devnet"},
	}
	for _, tt := range tests {
		meta := ResourceMeta{Name: tt.name, Namespace: tt.namespace}
		if got := meta.FullName(); got != tt.want {
			t.Errorf("FullName() = %q, want %q", got, tt.want)
		}
	}
}

func TestResourceMetaEnsureNamespace(t *testing.T) {
	// Test that empty namespace gets set to default
	meta := ResourceMeta{Name: "test", Namespace: ""}
	meta.EnsureNamespace()
	assert.Equal(t, DefaultNamespace, meta.Namespace)

	// Test that non-empty namespace is preserved
	meta2 := ResourceMeta{Name: "test", Namespace: "custom"}
	meta2.EnsureNamespace()
	assert.Equal(t, "custom", meta2.Namespace)
}
