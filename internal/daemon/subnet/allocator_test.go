package subnet

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadOrCreate_NewFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "subnets.json")

	a, err := LoadOrCreate(path)
	require.NoError(t, err)
	require.NotNil(t, a)

	// Verify file was created
	_, err = os.Stat(path)
	require.NoError(t, err)

	// Verify empty allocations
	allocations := a.ListAllocations()
	assert.Empty(t, allocations)
}

func TestLoadOrCreate_ExistingFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "subnets.json")

	// Create initial allocator and add allocation
	a1, err := LoadOrCreate(path)
	require.NoError(t, err)

	subnet, err := a1.Allocate("default", "test-devnet")
	require.NoError(t, err)

	// Load from existing file
	a2, err := LoadOrCreate(path)
	require.NoError(t, err)

	// Verify allocation persisted
	existingSubnet, found := a2.GetSubnet("default", "test-devnet")
	assert.True(t, found)
	assert.Equal(t, subnet, existingSubnet)
}

func TestLoadOrCreate_NestedDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "deep", "nested", "subnets.json")

	a, err := LoadOrCreate(path)
	require.NoError(t, err)
	require.NotNil(t, a)

	// Verify file was created
	_, err = os.Stat(path)
	require.NoError(t, err)
}

func TestAllocate_Basic(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "subnets.json")

	a, err := LoadOrCreate(path)
	require.NoError(t, err)

	subnet, err := a.Allocate("default", "my-devnet")
	require.NoError(t, err)

	// Verify valid subnet range
	assert.GreaterOrEqual(t, subnet, uint8(1))
	assert.LessOrEqual(t, subnet, uint8(254))

	// Verify can retrieve
	retrieved, found := a.GetSubnet("default", "my-devnet")
	assert.True(t, found)
	assert.Equal(t, subnet, retrieved)
}

func TestAllocate_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "subnets.json")

	a, err := LoadOrCreate(path)
	require.NoError(t, err)

	subnet1, err := a.Allocate("default", "my-devnet")
	require.NoError(t, err)

	subnet2, err := a.Allocate("default", "my-devnet")
	require.NoError(t, err)

	// Same devnet should get same subnet
	assert.Equal(t, subnet1, subnet2)
}

func TestAllocate_HashDeterminism(t *testing.T) {
	// The hash function should be deterministic
	key1 := "default/my-devnet"
	key2 := "default/my-devnet"

	subnet1 := hashToSubnet(key1)
	subnet2 := hashToSubnet(key2)

	assert.Equal(t, subnet1, subnet2)

	// Different keys should (usually) get different subnets
	subnet3 := hashToSubnet("alice/staging")
	// Note: This could theoretically be equal due to hash collision,
	// but for these specific inputs it shouldn't be
	assert.NotEqual(t, subnet1, subnet3, "different keys should hash to different subnets")
}

func TestAllocate_CollisionHandling(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "subnets.json")

	a, err := LoadOrCreate(path)
	require.NoError(t, err)

	// Allocate many devnets - force some collisions
	subnets := make(map[uint8]string)
	for i := 0; i < 10; i++ {
		subnet, err := a.Allocate("ns", string(rune('a'+i)))
		require.NoError(t, err)

		// Each allocation should be unique
		_, exists := subnets[subnet]
		assert.False(t, exists, "subnet %d already allocated", subnet)
		subnets[subnet] = string(rune('a' + i))
	}
}

func TestAllocate_WrapAround(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "subnets.json")

	a, err := LoadOrCreate(path)
	require.NoError(t, err)

	// Fill up most subnets (except a few near the end)
	for i := uint8(1); i <= 250; i++ {
		a.allocations[i] = "dummy"
	}

	// This allocation should wrap around and find an available slot
	subnet, err := a.Allocate("default", "wrap-test")
	require.NoError(t, err)

	// Should be one of the remaining slots (251-254)
	assert.GreaterOrEqual(t, subnet, uint8(251))
	assert.LessOrEqual(t, subnet, uint8(254))
}

func TestAllocate_NoAvailable(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "subnets.json")

	a, err := LoadOrCreate(path)
	require.NoError(t, err)

	// Fill up all subnets
	for i := uint8(1); i <= 254; i++ {
		a.allocations[i] = "dummy"
	}

	// Should fail when all subnets are taken
	_, err = a.Allocate("default", "no-space")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no available subnets")
}

func TestRelease_Basic(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "subnets.json")

	a, err := LoadOrCreate(path)
	require.NoError(t, err)

	subnet, err := a.Allocate("default", "my-devnet")
	require.NoError(t, err)

	err = a.Release("default", "my-devnet")
	require.NoError(t, err)

	// Should no longer be found
	_, found := a.GetSubnet("default", "my-devnet")
	assert.False(t, found)

	// Subnet should be available for reuse
	newSubnet, err := a.Allocate("other", "new-devnet")
	require.NoError(t, err)
	// With hash-based allocation, may or may not get same subnet
	_ = newSubnet
	_ = subnet
}

func TestRelease_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "subnets.json")

	a, err := LoadOrCreate(path)
	require.NoError(t, err)

	// Release non-existent allocation should not error
	err = a.Release("default", "never-allocated")
	assert.NoError(t, err)

	// Allocate and release twice should not error
	_, err = a.Allocate("default", "my-devnet")
	require.NoError(t, err)

	err = a.Release("default", "my-devnet")
	assert.NoError(t, err)

	err = a.Release("default", "my-devnet")
	assert.NoError(t, err)
}

func TestRelease_Persistence(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "subnets.json")

	a1, err := LoadOrCreate(path)
	require.NoError(t, err)

	_, err = a1.Allocate("default", "my-devnet")
	require.NoError(t, err)

	err = a1.Release("default", "my-devnet")
	require.NoError(t, err)

	// Load fresh and verify release persisted
	a2, err := LoadOrCreate(path)
	require.NoError(t, err)

	_, found := a2.GetSubnet("default", "my-devnet")
	assert.False(t, found)
}

func TestGetSubnet_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "subnets.json")

	a, err := LoadOrCreate(path)
	require.NoError(t, err)

	_, found := a.GetSubnet("default", "nonexistent")
	assert.False(t, found)
}

func TestPersistence_JSONFormat(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "subnets.json")

	a, err := LoadOrCreate(path)
	require.NoError(t, err)

	_, err = a.Allocate("default", "my-devnet")
	require.NoError(t, err)

	_, err = a.Allocate("alice", "staging")
	require.NoError(t, err)

	// Read raw file and verify JSON structure
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var state map[string]map[string]string
	err = json.Unmarshal(data, &state)
	require.NoError(t, err)

	allocations, ok := state["allocations"]
	assert.True(t, ok, "should have 'allocations' key")
	assert.Len(t, allocations, 2)
}

func TestNodeIP(t *testing.T) {
	tests := []struct {
		subnet    uint8
		nodeIndex int
		expected  string
	}{
		{42, 0, "127.0.42.1"},
		{42, 1, "127.0.42.2"},
		{42, 9, "127.0.42.10"},
		{1, 0, "127.0.1.1"},
		{254, 0, "127.0.254.1"},
		{100, 254, "127.0.100.255"},
	}

	for _, tc := range tests {
		t.Run(tc.expected, func(t *testing.T) {
			result := NodeIP(tc.subnet, tc.nodeIndex)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestHashToSubnet_Range(t *testing.T) {
	// Test many different inputs to verify range
	inputs := []string{
		"default/devnet1",
		"alice/staging",
		"bob/production",
		"test/test",
		"a/b",
		"really-long-namespace-name/really-long-devnet-name",
		"",
		"123/456",
		"special!@#$/chars",
	}

	for _, input := range inputs {
		subnet := hashToSubnet(input)
		assert.GreaterOrEqual(t, subnet, uint8(1), "subnet for %q should be >= 1", input)
		assert.LessOrEqual(t, subnet, uint8(254), "subnet for %q should be <= 254", input)
	}
}

func TestConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "subnets.json")

	a, err := LoadOrCreate(path)
	require.NoError(t, err)

	var wg sync.WaitGroup
	errors := make(chan error, 100)

	// Concurrent allocations
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, err := a.Allocate("ns", string(rune('A'+idx)))
			if err != nil {
				errors <- err
			}
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_ = a.ListAllocations()
			_, _ = a.GetSubnet("ns", string(rune('A'+idx)))
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("concurrent operation error: %v", err)
	}
}

func TestListAllocations(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "subnets.json")

	a, err := LoadOrCreate(path)
	require.NoError(t, err)

	// Empty initially
	assert.Empty(t, a.ListAllocations())

	// Add some allocations
	s1, _ := a.Allocate("default", "devnet1")
	s2, _ := a.Allocate("alice", "devnet2")

	allocs := a.ListAllocations()
	assert.Len(t, allocs, 2)
	assert.Equal(t, "default/devnet1", allocs[s1])
	assert.Equal(t, "alice/devnet2", allocs[s2])

	// Verify it's a copy (modifying doesn't affect original)
	delete(allocs, s1)
	assert.Len(t, a.ListAllocations(), 2)
}

func TestReallocationAfterRelease(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "subnets.json")

	a, err := LoadOrCreate(path)
	require.NoError(t, err)

	// Allocate, release, and reallocate same devnet
	subnet1, err := a.Allocate("default", "my-devnet")
	require.NoError(t, err)

	err = a.Release("default", "my-devnet")
	require.NoError(t, err)

	subnet2, err := a.Allocate("default", "my-devnet")
	require.NoError(t, err)

	// Should get same subnet (deterministic hash)
	assert.Equal(t, subnet1, subnet2, "reallocating same devnet should get same subnet")
}

func TestLoadOrCreate_CorruptedFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "subnets.json")

	// Write corrupted JSON
	err := os.WriteFile(path, []byte("not valid json"), 0644)
	require.NoError(t, err)

	_, err = LoadOrCreate(path)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse")
}

func TestLoadOrCreate_InvalidSubnetKey(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "subnets.json")

	// Write JSON with invalid subnet key
	err := os.WriteFile(path, []byte(`{"allocations": {"invalid": "default/devnet"}}`), 0644)
	require.NoError(t, err)

	_, err = LoadOrCreate(path)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid subnet key")
}
