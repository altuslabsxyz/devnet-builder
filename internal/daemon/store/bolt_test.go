// internal/daemon/store/bolt_test.go
package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/altuslabsxyz/devnet-builder/internal/daemon/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBoltStore_DevnetCRUD(t *testing.T) {
	// Setup temp database
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	store, err := NewBoltStore(dbPath)
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()

	// Create
	devnet := &types.Devnet{
		Metadata: types.ResourceMeta{Name: "test-devnet"},
		Spec: types.DevnetSpec{
			Plugin:     "stable",
			Validators: 4,
			Mode:       "docker",
		},
	}

	err = store.CreateDevnet(ctx, devnet)
	require.NoError(t, err)
	assert.Equal(t, int64(1), devnet.Metadata.Generation)
	assert.False(t, devnet.Metadata.CreatedAt.IsZero())

	// Get
	got, err := store.GetDevnet(ctx, "", "test-devnet")
	require.NoError(t, err)
	assert.Equal(t, "test-devnet", got.Metadata.Name)
	assert.Equal(t, "stable", got.Spec.Plugin)

	// Update
	got.Spec.Validators = 8
	err = store.UpdateDevnet(ctx, got)
	require.NoError(t, err)
	assert.Equal(t, int64(2), got.Metadata.Generation)

	// Verify update
	updated, err := store.GetDevnet(ctx, "", "test-devnet")
	require.NoError(t, err)
	assert.Equal(t, 8, updated.Spec.Validators)

	// List
	list, err := store.ListDevnets(ctx, "")
	require.NoError(t, err)
	assert.Len(t, list, 1)

	// Delete
	err = store.DeleteDevnet(ctx, "", "test-devnet")
	require.NoError(t, err)

	// Verify delete
	_, err = store.GetDevnet(ctx, "", "test-devnet")
	assert.True(t, IsNotFound(err))
}

func TestBoltStore_ConflictDetection(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	store, err := NewBoltStore(dbPath)
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()

	// Create devnet
	devnet := &types.Devnet{
		Metadata: types.ResourceMeta{Name: "conflict-test"},
		Spec:     types.DevnetSpec{Plugin: "stable", Validators: 4},
	}
	err = store.CreateDevnet(ctx, devnet)
	require.NoError(t, err)

	// Get two copies
	copy1, _ := store.GetDevnet(ctx, "", "conflict-test")
	copy2, _ := store.GetDevnet(ctx, "", "conflict-test")

	// Update first copy
	copy1.Spec.Validators = 8
	err = store.UpdateDevnet(ctx, copy1)
	require.NoError(t, err)

	// Try to update second copy (should conflict)
	copy2.Spec.Validators = 16
	err = store.UpdateDevnet(ctx, copy2)
	assert.True(t, IsConflict(err))
}

func TestBoltStore_AlreadyExists(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	store, err := NewBoltStore(dbPath)
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()

	devnet := &types.Devnet{
		Metadata: types.ResourceMeta{Name: "duplicate"},
		Spec:     types.DevnetSpec{Plugin: "stable"},
	}

	err = store.CreateDevnet(ctx, devnet)
	require.NoError(t, err)

	// Try to create again
	err = store.CreateDevnet(ctx, devnet)
	assert.True(t, IsAlreadyExists(err))
}

func TestBoltStore_Watch(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	store, err := NewBoltStore(dbPath)
	require.NoError(t, err)
	defer store.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	events := make(chan string, 10)

	// Start watching in goroutine
	go func() {
		store.Watch(ctx, "devnets", func(eventType string, resource interface{}) {
			events <- eventType
		})
	}()

	// Give watcher time to start
	time.Sleep(100 * time.Millisecond)

	// Create devnet
	devnet := &types.Devnet{
		Metadata: types.ResourceMeta{Name: "watch-test"},
		Spec:     types.DevnetSpec{Plugin: "stable"},
	}
	err = store.CreateDevnet(ctx, devnet)
	require.NoError(t, err)

	// Wait for event
	select {
	case event := <-events:
		assert.Equal(t, "ADDED", event)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for watch event")
	}
}
