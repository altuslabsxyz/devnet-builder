package binary

import (
	"context"
	"errors"
	"testing"

	"github.com/altuslabsxyz/devnet-builder/internal/application/ports"
)

// Mock implementations for testing

type mockBinaryResolver struct {
	resolveBinaryFunc        func(ctx context.Context, pluginName string) (string, error)
	getActiveBinaryFunc      func(ctx context.Context) (string, string, error)
	getBinaryNameFunc        func(ctx context.Context, pluginName string) (string, error)
	listAvailablePluginsFunc func(ctx context.Context) ([]string, error)
}

func (m *mockBinaryResolver) ResolveBinary(ctx context.Context, pluginName string) (string, error) {
	if m.resolveBinaryFunc != nil {
		return m.resolveBinaryFunc(ctx, pluginName)
	}
	return "/path/to/" + pluginName + "d", nil
}

func (m *mockBinaryResolver) GetActiveBinary(ctx context.Context) (string, string, error) {
	if m.getActiveBinaryFunc != nil {
		return m.getActiveBinaryFunc(ctx)
	}
	return "/path/to/stabled", "stable", nil
}

func (m *mockBinaryResolver) GetBinaryName(ctx context.Context, pluginName string) (string, error) {
	if m.getBinaryNameFunc != nil {
		return m.getBinaryNameFunc(ctx, pluginName)
	}
	return pluginName + "d", nil
}

func (m *mockBinaryResolver) ListAvailablePlugins(ctx context.Context) ([]string, error) {
	if m.listAvailablePluginsFunc != nil {
		return m.listAvailablePluginsFunc(ctx)
	}
	return []string{"stable", "ault"}, nil
}

type mockBinaryExecutor struct {
	executeFunc            func(ctx context.Context, cmd ports.BinaryPassthroughCommand) (int, error)
	executeInteractiveFunc func(ctx context.Context, cmd ports.BinaryPassthroughCommand) (int, error)
}

func (m *mockBinaryExecutor) Execute(ctx context.Context, cmd ports.BinaryPassthroughCommand) (int, error) {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, cmd)
	}
	return 0, nil
}

func (m *mockBinaryExecutor) ExecuteInteractive(ctx context.Context, cmd ports.BinaryPassthroughCommand) (int, error) {
	if m.executeInteractiveFunc != nil {
		return m.executeInteractiveFunc(ctx, cmd)
	}
	return 0, nil
}

// Tests

func TestPassthroughUseCase_Execute_Success(t *testing.T) {
	resolver := &mockBinaryResolver{
		resolveBinaryFunc: func(ctx context.Context, pluginName string) (string, error) {
			return "/path/to/stabled", nil
		},
	}

	executor := &mockBinaryExecutor{
		executeFunc: func(ctx context.Context, cmd ports.BinaryPassthroughCommand) (int, error) {
			if cmd.PluginName != "/path/to/stabled" {
				t.Errorf("Expected binary path '/path/to/stabled', got '%s'", cmd.PluginName)
			}
			return 0, nil
		},
	}

	uc := NewPassthroughUseCase(resolver, executor)

	req := ExecuteRequest{
		PluginName:  "stable",
		Args:        []string{"status"},
		Interactive: false,
	}

	resp, err := uc.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if resp.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", resp.ExitCode)
	}

	if resp.PluginName != "stable" {
		t.Errorf("Expected plugin name 'stable', got '%s'", resp.PluginName)
	}

	if resp.BinaryPath != "/path/to/stabled" {
		t.Errorf("Expected binary path '/path/to/stabled', got '%s'", resp.BinaryPath)
	}
}

func TestPassthroughUseCase_Execute_WithActivePlugin(t *testing.T) {
	resolver := &mockBinaryResolver{
		getActiveBinaryFunc: func(ctx context.Context) (string, string, error) {
			return "/path/to/stabled", "stable", nil
		},
	}

	executor := &mockBinaryExecutor{
		executeFunc: func(ctx context.Context, cmd ports.BinaryPassthroughCommand) (int, error) {
			if cmd.PluginName != "/path/to/stabled" {
				t.Errorf("Expected binary path '/path/to/stabled', got '%s'", cmd.PluginName)
			}
			return 0, nil
		},
	}

	uc := NewPassthroughUseCase(resolver, executor)

	// Empty plugin name should use active plugin
	req := ExecuteRequest{
		PluginName:  "",
		Args:        []string{"status"},
		Interactive: false,
	}

	resp, err := uc.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if resp.PluginName != "stable" {
		t.Errorf("Expected plugin name 'stable', got '%s'", resp.PluginName)
	}
}

func TestPassthroughUseCase_Execute_InteractiveMode(t *testing.T) {
	resolver := &mockBinaryResolver{}

	executorCalled := false
	executor := &mockBinaryExecutor{
		executeInteractiveFunc: func(ctx context.Context, cmd ports.BinaryPassthroughCommand) (int, error) {
			executorCalled = true
			return 0, nil
		},
	}

	uc := NewPassthroughUseCase(resolver, executor)

	req := ExecuteRequest{
		PluginName:  "stable",
		Args:        []string{"tx", "bank", "send"},
		Interactive: true,
	}

	_, err := uc.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if !executorCalled {
		t.Error("Expected ExecuteInteractive to be called, but it wasn't")
	}
}

func TestPassthroughUseCase_Execute_PluginNotFound(t *testing.T) {
	resolver := &mockBinaryResolver{
		resolveBinaryFunc: func(ctx context.Context, pluginName string) (string, error) {
			return "", errors.New("plugin not found")
		},
	}

	executor := &mockBinaryExecutor{}
	uc := NewPassthroughUseCase(resolver, executor)

	req := ExecuteRequest{
		PluginName: "nonexistent",
		Args:       []string{"status"},
	}

	_, err := uc.Execute(context.Background(), req)
	if err == nil {
		t.Fatal("Expected error for nonexistent plugin, got nil")
	}
}

func TestPassthroughUseCase_Execute_NoActiveBinary(t *testing.T) {
	resolver := &mockBinaryResolver{
		getActiveBinaryFunc: func(ctx context.Context) (string, string, error) {
			return "", "", errors.New("no active binary set")
		},
	}

	executor := &mockBinaryExecutor{}
	uc := NewPassthroughUseCase(resolver, executor)

	req := ExecuteRequest{
		PluginName: "", // Empty means use active
		Args:       []string{"status"},
	}

	_, err := uc.Execute(context.Background(), req)
	if err == nil {
		t.Fatal("Expected error for no active binary, got nil")
	}

	if !containsString(err.Error(), "set active binary") {
		t.Errorf("Expected error message to mention setting active binary, got: %v", err)
	}
}

func TestPassthroughUseCase_Execute_ExecutionFails(t *testing.T) {
	resolver := &mockBinaryResolver{}

	executor := &mockBinaryExecutor{
		executeFunc: func(ctx context.Context, cmd ports.BinaryPassthroughCommand) (int, error) {
			return 0, errors.New("execution failed")
		},
	}

	uc := NewPassthroughUseCase(resolver, executor)

	req := ExecuteRequest{
		PluginName: "stable",
		Args:       []string{"status"},
	}

	_, err := uc.Execute(context.Background(), req)
	if err == nil {
		t.Fatal("Expected error for execution failure, got nil")
	}
}

func TestPassthroughUseCase_Execute_NonZeroExitCode(t *testing.T) {
	resolver := &mockBinaryResolver{}

	executor := &mockBinaryExecutor{
		executeFunc: func(ctx context.Context, cmd ports.BinaryPassthroughCommand) (int, error) {
			return 42, nil
		},
	}

	uc := NewPassthroughUseCase(resolver, executor)

	req := ExecuteRequest{
		PluginName: "stable",
		Args:       []string{"status"},
	}

	resp, err := uc.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if resp.ExitCode != 42 {
		t.Errorf("Expected exit code 42, got %d", resp.ExitCode)
	}
}

func TestPassthroughUseCase_Execute_WithWorkDir(t *testing.T) {
	resolver := &mockBinaryResolver{}

	expectedWorkDir := "/custom/work/dir"
	executor := &mockBinaryExecutor{
		executeFunc: func(ctx context.Context, cmd ports.BinaryPassthroughCommand) (int, error) {
			if cmd.WorkDir != expectedWorkDir {
				t.Errorf("Expected work dir '%s', got '%s'", expectedWorkDir, cmd.WorkDir)
			}
			return 0, nil
		},
	}

	uc := NewPassthroughUseCase(resolver, executor)

	req := ExecuteRequest{
		PluginName: "stable",
		Args:       []string{"status"},
		WorkDir:    expectedWorkDir,
	}

	_, err := uc.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
}

func TestPassthroughUseCase_GetBinaryName_Success(t *testing.T) {
	resolver := &mockBinaryResolver{
		getBinaryNameFunc: func(ctx context.Context, pluginName string) (string, error) {
			return pluginName + "d", nil
		},
	}

	executor := &mockBinaryExecutor{}
	uc := NewPassthroughUseCase(resolver, executor)

	binaryName, err := uc.GetBinaryName(context.Background(), "stable")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if binaryName != "stabled" {
		t.Errorf("Expected 'stabled', got '%s'", binaryName)
	}
}

func TestPassthroughUseCase_GetBinaryName_EmptyPluginName(t *testing.T) {
	resolver := &mockBinaryResolver{}
	executor := &mockBinaryExecutor{}
	uc := NewPassthroughUseCase(resolver, executor)

	_, err := uc.GetBinaryName(context.Background(), "")
	if err == nil {
		t.Fatal("Expected error for empty plugin name, got nil")
	}
}

func TestPassthroughUseCase_GetBinaryName_PluginNotFound(t *testing.T) {
	resolver := &mockBinaryResolver{
		getBinaryNameFunc: func(ctx context.Context, pluginName string) (string, error) {
			return "", errors.New("plugin not found")
		},
	}

	executor := &mockBinaryExecutor{}
	uc := NewPassthroughUseCase(resolver, executor)

	_, err := uc.GetBinaryName(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("Expected error for nonexistent plugin, got nil")
	}
}

func TestPassthroughUseCase_ListAvailablePlugins_Success(t *testing.T) {
	expectedPlugins := []string{"stable", "ault", "osmosis"}
	resolver := &mockBinaryResolver{
		listAvailablePluginsFunc: func(ctx context.Context) ([]string, error) {
			return expectedPlugins, nil
		},
	}

	executor := &mockBinaryExecutor{}
	uc := NewPassthroughUseCase(resolver, executor)

	plugins, err := uc.ListAvailablePlugins(context.Background())
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(plugins) != len(expectedPlugins) {
		t.Errorf("Expected %d plugins, got %d", len(expectedPlugins), len(plugins))
	}

	for i, plugin := range plugins {
		if plugin != expectedPlugins[i] {
			t.Errorf("Expected plugin '%s' at index %d, got '%s'", expectedPlugins[i], i, plugin)
		}
	}
}

func TestPassthroughUseCase_ListAvailablePlugins_Error(t *testing.T) {
	resolver := &mockBinaryResolver{
		listAvailablePluginsFunc: func(ctx context.Context) ([]string, error) {
			return nil, errors.New("discovery failed")
		},
	}

	executor := &mockBinaryExecutor{}
	uc := NewPassthroughUseCase(resolver, executor)

	_, err := uc.ListAvailablePlugins(context.Background())
	if err == nil {
		t.Fatal("Expected error for discovery failure, got nil")
	}
}

func TestPassthroughUseCase_Execute_ArgsPassthrough(t *testing.T) {
	resolver := &mockBinaryResolver{}

	expectedArgs := []string{"tx", "bank", "send", "cosmos1...", "1000stake"}
	executor := &mockBinaryExecutor{
		executeFunc: func(ctx context.Context, cmd ports.BinaryPassthroughCommand) (int, error) {
			if len(cmd.Args) != len(expectedArgs) {
				t.Errorf("Expected %d args, got %d", len(expectedArgs), len(cmd.Args))
			}
			for i, arg := range cmd.Args {
				if arg != expectedArgs[i] {
					t.Errorf("Expected arg '%s' at index %d, got '%s'", expectedArgs[i], i, arg)
				}
			}
			return 0, nil
		},
	}

	uc := NewPassthroughUseCase(resolver, executor)

	req := ExecuteRequest{
		PluginName: "stable",
		Args:       expectedArgs,
	}

	_, err := uc.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
}

func TestPassthroughUseCase_Execute_EmptyArgs(t *testing.T) {
	resolver := &mockBinaryResolver{}

	executor := &mockBinaryExecutor{
		executeFunc: func(ctx context.Context, cmd ports.BinaryPassthroughCommand) (int, error) {
			if len(cmd.Args) != 0 {
				t.Errorf("Expected 0 args, got %d", len(cmd.Args))
			}
			return 0, nil
		},
	}

	uc := NewPassthroughUseCase(resolver, executor)

	req := ExecuteRequest{
		PluginName: "stable",
		Args:       []string{},
	}

	_, err := uc.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
}

// Helper function
func containsString(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) >= len(substr) && s[:len(substr)] == substr || len(s) > len(substr) && s[len(s)-len(substr):] == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
