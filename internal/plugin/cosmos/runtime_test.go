// internal/plugin/cosmos/runtime_test.go
package cosmos

import (
	"syscall"
	"testing"
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/daemon/runtime"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCosmosRuntimeImplementsInterface(t *testing.T) {
	var _ runtime.PluginRuntime = (*CosmosRuntime)(nil)
}

func TestCosmosRuntime_StartCommand(t *testing.T) {
	tests := []struct {
		name       string
		binaryName string
		node       *types.Node
		wantHome   string
	}{
		{
			name:       "stabled binary",
			binaryName: "stabled",
			node: &types.Node{
				Spec: types.NodeSpec{Index: 0},
			},
			wantHome: "/root/.stabled",
		},
		{
			name:       "gaiad binary",
			binaryName: "gaiad",
			node: &types.Node{
				Spec: types.NodeSpec{Index: 1},
			},
			wantHome: "/root/.gaiad",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewCosmosRuntime(tt.binaryName)
			cmd := r.StartCommand(tt.node)

			// Should start with "start"
			require.Greater(t, len(cmd), 0)
			assert.Equal(t, "start", cmd[0])

			// Should contain --home with correct path
			foundHome := false
			for i, arg := range cmd {
				if arg == "--home" && i+1 < len(cmd) {
					assert.Equal(t, tt.wantHome, cmd[i+1])
					foundHome = true
					break
				}
			}
			assert.True(t, foundHome, "expected --home flag in command")

			// Should enable API
			assert.Contains(t, cmd, "--api.enable=true")
			// Should enable gRPC
			assert.Contains(t, cmd, "--grpc.enable=true")
		})
	}
}

func TestCosmosRuntime_StartEnv(t *testing.T) {
	r := NewCosmosRuntime("stabled")
	node := &types.Node{Spec: types.NodeSpec{Index: 0}}

	env := r.StartEnv(node)

	assert.Equal(t, "/root/.stabled", env["TMHOME"])
	assert.Equal(t, "1", env["NO_COLOR"])
}

func TestCosmosRuntime_ContainerHomePath(t *testing.T) {
	tests := []struct {
		binaryName string
		want       string
	}{
		{"stabled", "/root/.stabled"},
		{"gaiad", "/root/.gaiad"},
		{"simd", "/root/.simd"},
	}

	for _, tt := range tests {
		t.Run(tt.binaryName, func(t *testing.T) {
			r := NewCosmosRuntime(tt.binaryName)
			assert.Equal(t, tt.want, r.ContainerHomePath())
		})
	}
}

func TestCosmosRuntime_StopSignal(t *testing.T) {
	r := NewCosmosRuntime("stabled")
	assert.Equal(t, syscall.SIGTERM, r.StopSignal())
}

func TestCosmosRuntime_GracePeriod(t *testing.T) {
	r := NewCosmosRuntime("stabled")
	assert.Equal(t, 30*time.Second, r.GracePeriod())
}

func TestCosmosRuntime_HealthEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		index    int
		wantPort int
	}{
		{"node 0", 0, 26657},
		{"node 1", 1, 26757},
		{"node 2", 2, 26857},
		{"node 5", 5, 27157},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewCosmosRuntime("stabled")
			node := &types.Node{Spec: types.NodeSpec{Index: tt.index}}

			endpoint := r.HealthEndpoint(node)

			assert.Contains(t, endpoint, "localhost")
			assert.Contains(t, endpoint, "/status")
			assert.Contains(t, endpoint, string(rune('0'+tt.wantPort/10000)))
		})
	}
}

func TestCosmosRuntime_BinaryName(t *testing.T) {
	r := NewCosmosRuntime("stabled")
	assert.Equal(t, "stabled", r.BinaryName())

	r2 := NewCosmosRuntime("gaiad")
	assert.Equal(t, "gaiad", r2.BinaryName())
}
