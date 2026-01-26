// internal/daemon/store/bolt_node_test.go
package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/altuslabsxyz/devnet-builder/internal/daemon/types"
)

func TestBoltStore_CreateNode(t *testing.T) {
	dir := t.TempDir()
	s, err := NewBoltStore(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("NewBoltStore: %v", err)
	}
	defer s.Close()

	node := &types.Node{
		Metadata: types.ResourceMeta{Name: "test-node"},
		Spec:     types.NodeSpec{DevnetRef: "mydevnet", Index: 0, Role: "validator"},
	}

	err = s.CreateNode(context.Background(), node)
	if err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	// Verify metadata was set
	if node.Metadata.Generation != 1 {
		t.Errorf("Generation = %d, want 1", node.Metadata.Generation)
	}
	if node.Metadata.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}

	// Verify we can get it back
	got, err := s.GetNode(context.Background(), "", "mydevnet", 0)
	if err != nil {
		t.Fatalf("GetNode: %v", err)
	}
	if got.Spec.Role != "validator" {
		t.Errorf("Role = %q, want %q", got.Spec.Role, "validator")
	}
	if got.Spec.DevnetRef != "mydevnet" {
		t.Errorf("DevnetRef = %q, want %q", got.Spec.DevnetRef, "mydevnet")
	}
}

func TestBoltStore_CreateNode_AlreadyExists(t *testing.T) {
	dir := t.TempDir()
	s, err := NewBoltStore(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("NewBoltStore: %v", err)
	}
	defer s.Close()

	node := &types.Node{
		Metadata: types.ResourceMeta{Name: "test-node"},
		Spec:     types.NodeSpec{DevnetRef: "mydevnet", Index: 0, Role: "validator"},
	}

	// First create should succeed
	if err := s.CreateNode(context.Background(), node); err != nil {
		t.Fatalf("First CreateNode: %v", err)
	}

	// Second create should fail
	node2 := &types.Node{
		Metadata: types.ResourceMeta{Name: "test-node-2"},
		Spec:     types.NodeSpec{DevnetRef: "mydevnet", Index: 0, Role: "validator"},
	}
	err = s.CreateNode(context.Background(), node2)
	if err == nil {
		t.Fatal("Expected error for duplicate node")
	}

	var alreadyExists *AlreadyExistsError
	if !IsAlreadyExists(err) {
		t.Errorf("Expected AlreadyExistsError, got %T: %v", err, err)
	}
	_ = alreadyExists
}

func TestBoltStore_GetNode_NotFound(t *testing.T) {
	dir := t.TempDir()
	s, err := NewBoltStore(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("NewBoltStore: %v", err)
	}
	defer s.Close()

	_, err = s.GetNode(context.Background(), "", "nonexistent", 0)
	if err == nil {
		t.Fatal("Expected error for nonexistent node")
	}
	if !IsNotFound(err) {
		t.Errorf("Expected NotFoundError, got %T: %v", err, err)
	}
}

func TestBoltStore_UpdateNode(t *testing.T) {
	dir := t.TempDir()
	s, err := NewBoltStore(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("NewBoltStore: %v", err)
	}
	defer s.Close()

	// Create a node
	node := &types.Node{
		Metadata: types.ResourceMeta{Name: "test-node"},
		Spec:     types.NodeSpec{DevnetRef: "mydevnet", Index: 0, Role: "validator"},
		Status:   types.NodeStatus{Phase: "Pending"},
	}
	if err := s.CreateNode(context.Background(), node); err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	// Get fresh copy
	node, err = s.GetNode(context.Background(), "", "mydevnet", 0)
	if err != nil {
		t.Fatalf("GetNode: %v", err)
	}

	// Update it
	node.Status.Phase = "Running"
	if err := s.UpdateNode(context.Background(), node); err != nil {
		t.Fatalf("UpdateNode: %v", err)
	}

	// Verify update
	got, err := s.GetNode(context.Background(), "", "mydevnet", 0)
	if err != nil {
		t.Fatalf("GetNode after update: %v", err)
	}
	if got.Status.Phase != "Running" {
		t.Errorf("Phase = %q, want %q", got.Status.Phase, "Running")
	}
	if got.Metadata.Generation != 2 {
		t.Errorf("Generation = %d, want 2", got.Metadata.Generation)
	}
}

func TestBoltStore_UpdateNode_ConflictError(t *testing.T) {
	dir := t.TempDir()
	s, err := NewBoltStore(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("NewBoltStore: %v", err)
	}
	defer s.Close()

	// Create a node
	node := &types.Node{
		Metadata: types.ResourceMeta{Name: "test-node"},
		Spec:     types.NodeSpec{DevnetRef: "mydevnet", Index: 0, Role: "validator"},
	}
	if err := s.CreateNode(context.Background(), node); err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	// Simulate concurrent update by using stale generation
	staleNode := &types.Node{
		Metadata: types.ResourceMeta{Name: "test-node", Generation: 0}, // Wrong generation
		Spec:     types.NodeSpec{DevnetRef: "mydevnet", Index: 0, Role: "validator"},
	}
	err = s.UpdateNode(context.Background(), staleNode)
	if err == nil {
		t.Fatal("Expected conflict error")
	}
	if !IsConflict(err) {
		t.Errorf("Expected ConflictError, got %T: %v", err, err)
	}
}

func TestBoltStore_DeleteNode(t *testing.T) {
	dir := t.TempDir()
	s, err := NewBoltStore(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("NewBoltStore: %v", err)
	}
	defer s.Close()

	// Create a node
	node := &types.Node{
		Metadata: types.ResourceMeta{Name: "test-node"},
		Spec:     types.NodeSpec{DevnetRef: "mydevnet", Index: 0, Role: "validator"},
	}
	if err := s.CreateNode(context.Background(), node); err != nil {
		t.Fatalf("CreateNode: %v", err)
	}

	// Delete it
	if err := s.DeleteNode(context.Background(), "", "mydevnet", 0); err != nil {
		t.Fatalf("DeleteNode: %v", err)
	}

	// Verify it's gone
	_, err = s.GetNode(context.Background(), "", "mydevnet", 0)
	if !IsNotFound(err) {
		t.Errorf("Expected NotFoundError after delete, got %v", err)
	}
}

func TestBoltStore_DeleteNode_NotFound(t *testing.T) {
	dir := t.TempDir()
	s, err := NewBoltStore(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("NewBoltStore: %v", err)
	}
	defer s.Close()

	err = s.DeleteNode(context.Background(), "", "nonexistent", 0)
	if err == nil {
		t.Fatal("Expected error for nonexistent node")
	}
	if !IsNotFound(err) {
		t.Errorf("Expected NotFoundError, got %T: %v", err, err)
	}
}

func TestBoltStore_ListNodes(t *testing.T) {
	dir := t.TempDir()
	s, err := NewBoltStore(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("NewBoltStore: %v", err)
	}
	defer s.Close()

	// Create nodes for two different devnets
	nodes := []*types.Node{
		{Spec: types.NodeSpec{DevnetRef: "devnet-a", Index: 0, Role: "validator"}},
		{Spec: types.NodeSpec{DevnetRef: "devnet-a", Index: 1, Role: "validator"}},
		{Spec: types.NodeSpec{DevnetRef: "devnet-a", Index: 2, Role: "fullnode"}},
		{Spec: types.NodeSpec{DevnetRef: "devnet-b", Index: 0, Role: "validator"}},
	}
	for _, n := range nodes {
		if err := s.CreateNode(context.Background(), n); err != nil {
			t.Fatalf("CreateNode: %v", err)
		}
	}

	// List nodes for devnet-a
	gotA, err := s.ListNodes(context.Background(), "", "devnet-a")
	if err != nil {
		t.Fatalf("ListNodes devnet-a: %v", err)
	}
	if len(gotA) != 3 {
		t.Errorf("ListNodes devnet-a: got %d nodes, want 3", len(gotA))
	}

	// List nodes for devnet-b
	gotB, err := s.ListNodes(context.Background(), "", "devnet-b")
	if err != nil {
		t.Fatalf("ListNodes devnet-b: %v", err)
	}
	if len(gotB) != 1 {
		t.Errorf("ListNodes devnet-b: got %d nodes, want 1", len(gotB))
	}

	// List nodes for nonexistent devnet
	gotC, err := s.ListNodes(context.Background(), "", "devnet-c")
	if err != nil {
		t.Fatalf("ListNodes devnet-c: %v", err)
	}
	if len(gotC) != 0 {
		t.Errorf("ListNodes devnet-c: got %d nodes, want 0", len(gotC))
	}
}

func TestBoltStore_DeleteNodesByDevnet(t *testing.T) {
	dir := t.TempDir()
	s, err := NewBoltStore(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("NewBoltStore: %v", err)
	}
	defer s.Close()

	// Create nodes for two different devnets
	nodes := []*types.Node{
		{Spec: types.NodeSpec{DevnetRef: "devnet-a", Index: 0, Role: "validator"}},
		{Spec: types.NodeSpec{DevnetRef: "devnet-a", Index: 1, Role: "validator"}},
		{Spec: types.NodeSpec{DevnetRef: "devnet-b", Index: 0, Role: "validator"}},
	}
	for _, n := range nodes {
		if err := s.CreateNode(context.Background(), n); err != nil {
			t.Fatalf("CreateNode: %v", err)
		}
	}

	// Delete all nodes for devnet-a
	if err := s.DeleteNodesByDevnet(context.Background(), "", "devnet-a"); err != nil {
		t.Fatalf("DeleteNodesByDevnet: %v", err)
	}

	// Verify devnet-a nodes are gone
	gotA, err := s.ListNodes(context.Background(), "", "devnet-a")
	if err != nil {
		t.Fatalf("ListNodes devnet-a: %v", err)
	}
	if len(gotA) != 0 {
		t.Errorf("Expected 0 nodes for devnet-a, got %d", len(gotA))
	}

	// Verify devnet-b nodes are still there
	gotB, err := s.ListNodes(context.Background(), "", "devnet-b")
	if err != nil {
		t.Fatalf("ListNodes devnet-b: %v", err)
	}
	if len(gotB) != 1 {
		t.Errorf("Expected 1 node for devnet-b, got %d", len(gotB))
	}
}

func TestBoltStore_NodeWithLargeIndex(t *testing.T) {
	// Test that nodes with index > 9 work correctly
	dir := t.TempDir()
	s, err := NewBoltStore(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("NewBoltStore: %v", err)
	}
	defer s.Close()

	// Create nodes with various indices
	indices := []int{0, 9, 10, 15, 99, 100}
	for _, idx := range indices {
		node := &types.Node{
			Spec: types.NodeSpec{DevnetRef: "mydevnet", Index: idx, Role: "validator"},
		}
		if err := s.CreateNode(context.Background(), node); err != nil {
			t.Fatalf("CreateNode index %d: %v", idx, err)
		}
	}

	// Verify we can retrieve each one
	for _, idx := range indices {
		got, err := s.GetNode(context.Background(), "", "mydevnet", idx)
		if err != nil {
			t.Errorf("GetNode index %d: %v", idx, err)
			continue
		}
		if got.Spec.Index != idx {
			t.Errorf("Index = %d, want %d", got.Spec.Index, idx)
		}
	}

	// Verify list returns all
	nodes, err := s.ListNodes(context.Background(), "", "mydevnet")
	if err != nil {
		t.Fatalf("ListNodes: %v", err)
	}
	if len(nodes) != len(indices) {
		t.Errorf("ListNodes: got %d nodes, want %d", len(nodes), len(indices))
	}
}
