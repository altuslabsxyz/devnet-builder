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

	assert.Len(t, status.SDKVersionHistory, 1)
	assert.Equal(t, "0.50.9", status.SDKVersionHistory[0].FromVersion)
}
