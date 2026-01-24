package server

import (
	"context"
	"testing"

	v1 "github.com/altuslabsxyz/devnet-builder/api/proto/gen/v1"
	"github.com/altuslabsxyz/devnet-builder/internal/daemon/store"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestDevnetService_Create(t *testing.T) {
	s := store.NewMemoryStore()
	svc := NewDevnetService(s, nil)

	req := &v1.CreateDevnetRequest{
		Name: "test-devnet",
		Spec: &v1.DevnetSpec{
			Plugin:     "stable",
			Validators: 4,
			Mode:       "docker",
		},
		Labels: map[string]string{"env": "test"},
	}

	resp, err := svc.CreateDevnet(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateDevnet failed: %v", err)
	}

	if resp.Devnet.Metadata.Name != "test-devnet" {
		t.Errorf("expected name test-devnet, got %s", resp.Devnet.Metadata.Name)
	}
	if resp.Devnet.Spec.Plugin != "stable" {
		t.Errorf("expected plugin stable, got %s", resp.Devnet.Spec.Plugin)
	}
	if resp.Devnet.Status.Phase != "Pending" {
		t.Errorf("expected phase Pending, got %s", resp.Devnet.Status.Phase)
	}

	// Verify it's in the store
	devnet, err := s.GetDevnet(context.Background(), "test-devnet")
	if err != nil {
		t.Fatalf("GetDevnet from store failed: %v", err)
	}
	if devnet.Metadata.Name != "test-devnet" {
		t.Errorf("store has wrong name: %s", devnet.Metadata.Name)
	}
}

func TestDevnetService_CreateAlreadyExists(t *testing.T) {
	s := store.NewMemoryStore()
	svc := NewDevnetService(s, nil)

	req := &v1.CreateDevnetRequest{
		Name: "duplicate",
		Spec: &v1.DevnetSpec{
			Plugin:     "stable",
			Validators: 4,
		},
	}

	// Create first
	_, err := svc.CreateDevnet(context.Background(), req)
	if err != nil {
		t.Fatalf("first CreateDevnet failed: %v", err)
	}

	// Try to create again
	_, err = svc.CreateDevnet(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for duplicate create")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %v", err)
	}
	if st.Code() != codes.AlreadyExists {
		t.Errorf("expected AlreadyExists, got %v", st.Code())
	}
}

func TestDevnetService_CreateMissingName(t *testing.T) {
	s := store.NewMemoryStore()
	svc := NewDevnetService(s, nil)

	req := &v1.CreateDevnetRequest{
		Name: "", // Missing name
		Spec: &v1.DevnetSpec{
			Plugin: "stable",
		},
	}

	_, err := svc.CreateDevnet(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for missing name")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %v", err)
	}
	if st.Code() != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %v", st.Code())
	}
}

func TestDevnetService_Get(t *testing.T) {
	s := store.NewMemoryStore()
	svc := NewDevnetService(s, nil)

	// Create a devnet
	createReq := &v1.CreateDevnetRequest{
		Name: "get-test",
		Spec: &v1.DevnetSpec{
			Plugin:     "stable",
			Validators: 4,
		},
	}
	_, err := svc.CreateDevnet(context.Background(), createReq)
	if err != nil {
		t.Fatalf("CreateDevnet failed: %v", err)
	}

	// Get it
	getReq := &v1.GetDevnetRequest{Name: "get-test"}
	resp, err := svc.GetDevnet(context.Background(), getReq)
	if err != nil {
		t.Fatalf("GetDevnet failed: %v", err)
	}

	if resp.Devnet.Metadata.Name != "get-test" {
		t.Errorf("expected name get-test, got %s", resp.Devnet.Metadata.Name)
	}
}

func TestDevnetService_GetNotFound(t *testing.T) {
	s := store.NewMemoryStore()
	svc := NewDevnetService(s, nil)

	req := &v1.GetDevnetRequest{Name: "non-existent"}
	_, err := svc.GetDevnet(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for non-existent devnet")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %v", err)
	}
	if st.Code() != codes.NotFound {
		t.Errorf("expected NotFound, got %v", st.Code())
	}
}

func TestDevnetService_List(t *testing.T) {
	s := store.NewMemoryStore()
	svc := NewDevnetService(s, nil)

	// Create multiple devnets
	for _, name := range []string{"devnet-1", "devnet-2", "devnet-3"} {
		req := &v1.CreateDevnetRequest{
			Name: name,
			Spec: &v1.DevnetSpec{
				Plugin:     "stable",
				Validators: 4,
			},
		}
		_, err := svc.CreateDevnet(context.Background(), req)
		if err != nil {
			t.Fatalf("CreateDevnet %s failed: %v", name, err)
		}
	}

	// List all
	resp, err := svc.ListDevnets(context.Background(), &v1.ListDevnetsRequest{})
	if err != nil {
		t.Fatalf("ListDevnets failed: %v", err)
	}

	if len(resp.Devnets) != 3 {
		t.Errorf("expected 3 devnets, got %d", len(resp.Devnets))
	}
}

func TestDevnetService_Delete(t *testing.T) {
	s := store.NewMemoryStore()
	svc := NewDevnetService(s, nil)

	// Create a devnet
	createReq := &v1.CreateDevnetRequest{
		Name: "to-delete",
		Spec: &v1.DevnetSpec{
			Plugin:     "stable",
			Validators: 4,
		},
	}
	_, err := svc.CreateDevnet(context.Background(), createReq)
	if err != nil {
		t.Fatalf("CreateDevnet failed: %v", err)
	}

	// Delete it
	deleteReq := &v1.DeleteDevnetRequest{Name: "to-delete"}
	resp, err := svc.DeleteDevnet(context.Background(), deleteReq)
	if err != nil {
		t.Fatalf("DeleteDevnet failed: %v", err)
	}

	if !resp.Deleted {
		t.Error("expected deleted=true")
	}

	// Verify it's gone
	_, err = svc.GetDevnet(context.Background(), &v1.GetDevnetRequest{Name: "to-delete"})
	if err == nil {
		t.Error("expected error getting deleted devnet")
	}
}

func TestDevnetService_DeleteNotFound(t *testing.T) {
	s := store.NewMemoryStore()
	svc := NewDevnetService(s, nil)

	req := &v1.DeleteDevnetRequest{Name: "non-existent"}
	_, err := svc.DeleteDevnet(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for non-existent devnet")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %v", err)
	}
	if st.Code() != codes.NotFound {
		t.Errorf("expected NotFound, got %v", st.Code())
	}
}

func TestDevnetService_StartDevnet(t *testing.T) {
	s := store.NewMemoryStore()
	svc := NewDevnetService(s, nil)

	// Create and simulate it being stopped
	createReq := &v1.CreateDevnetRequest{
		Name: "stopped-devnet",
		Spec: &v1.DevnetSpec{
			Plugin:     "stable",
			Validators: 4,
		},
	}
	_, err := svc.CreateDevnet(context.Background(), createReq)
	if err != nil {
		t.Fatalf("CreateDevnet failed: %v", err)
	}

	// Manually set to stopped
	devnet, _ := s.GetDevnet(context.Background(), "stopped-devnet")
	devnet.Status.Phase = "Stopped"
	s.UpdateDevnet(context.Background(), devnet)

	// Start it
	resp, err := svc.StartDevnet(context.Background(), &v1.StartDevnetRequest{Name: "stopped-devnet"})
	if err != nil {
		t.Fatalf("StartDevnet failed: %v", err)
	}

	// Should transition to Pending (to be reconciled)
	if resp.Devnet.Status.Phase != "Pending" {
		t.Errorf("expected phase Pending after start, got %s", resp.Devnet.Status.Phase)
	}
}

func TestDevnetService_StopDevnet(t *testing.T) {
	s := store.NewMemoryStore()
	svc := NewDevnetService(s, nil)

	// Create a devnet
	createReq := &v1.CreateDevnetRequest{
		Name: "running-devnet",
		Spec: &v1.DevnetSpec{
			Plugin:     "stable",
			Validators: 4,
		},
	}
	_, err := svc.CreateDevnet(context.Background(), createReq)
	if err != nil {
		t.Fatalf("CreateDevnet failed: %v", err)
	}

	// Manually set to running
	devnet, _ := s.GetDevnet(context.Background(), "running-devnet")
	devnet.Status.Phase = "Running"
	s.UpdateDevnet(context.Background(), devnet)

	// Stop it
	resp, err := svc.StopDevnet(context.Background(), &v1.StopDevnetRequest{Name: "running-devnet"})
	if err != nil {
		t.Fatalf("StopDevnet failed: %v", err)
	}

	if resp.Devnet.Status.Phase != "Stopped" {
		t.Errorf("expected phase Stopped, got %s", resp.Devnet.Status.Phase)
	}
}
