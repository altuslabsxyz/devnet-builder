package interactive

import (
	"context"
	"testing"

	"github.com/altuslabsxyz/devnet-builder/internal/infrastructure/cache"
	"github.com/manifoldco/promptui"
)

// mockPrompter is a test double for the Prompter interface.
// It allows simulating user interactions without requiring a real terminal.
type mockPrompter struct {
	// SelectFromList behavior
	selectIndex  int      // Index to return from SelectFromList
	selectValue  string   // Value to return from SelectFromList
	selectErr    error    // Error to return from SelectFromList
	selectCalled bool     // Tracks if SelectFromList was called
	selectLabel  string   // Captures the label passed to SelectFromList
	selectItems  []string // Captures the items passed to SelectFromList

	// InputText behavior
	inputText   string // Text to return from InputText
	inputErr    error  // Error to return from InputText
	inputCalled bool   // Tracks if InputText was called
	inputLabel  string // Captures the label passed to InputText
}

func (m *mockPrompter) SelectFromList(label string, items []string, cursorPos *int) (int, string, error) {
	m.selectCalled = true
	m.selectLabel = label
	m.selectItems = items

	if m.selectErr != nil {
		return 0, "", m.selectErr
	}

	// Return the configured index and value
	if m.selectIndex >= 0 && m.selectIndex < len(items) {
		m.selectValue = items[m.selectIndex]
	}

	return m.selectIndex, m.selectValue, nil
}

func (m *mockPrompter) InputText(label string) (string, error) {
	m.inputCalled = true
	m.inputLabel = label

	if m.inputErr != nil {
		return "", m.inputErr
	}

	return m.inputText, nil
}

// createTestBinaries is a helper to generate test binary metadata.
func createTestBinaries(count int) []cache.CachedBinaryMetadata {
	binaries := make([]cache.CachedBinaryMetadata, count)
	for i := 0; i < count; i++ {
		binaries[i] = cache.CachedBinaryMetadata{
			Path:            "/cache/mainnet/abc123-empty/stabled",
			Name:            "stabled",
			NetworkType:     "mainnet",
			CommitHashShort: "abc123d",
			CommitHash:      "abc123def456789",
			ConfigHash:      "empty",
			CacheKey:        "mainnet/abc123d-empty",
			Version:         "v1.0.0",
			Size:            47394201,
			SizeHuman:       "45.2 MB",
			ModTimeRelative: "2 hours ago",
			IsValid:         true,
			ValidationError: "",
		}
	}
	return binaries
}

// TestBinarySelector_RunBinarySelectionFlow_EC001_ZeroBinaries tests EC-001: Empty cache
func TestBinarySelector_RunBinarySelectionFlow_EC001_ZeroBinaries(t *testing.T) {
	// Setup: Mock prompter (should not be called)
	prompter := &mockPrompter{}
	selector := NewBinarySelector(prompter)

	opts := BinarySelectionOptions{
		AllowBuildFromSource: true,
		AutoSelectSingle:     true,
		IsInteractive:        true,
	}

	// Execute: Zero binaries
	result, err := selector.RunBinarySelectionFlow(context.Background(), []cache.CachedBinaryMetadata{}, opts)

	// Verify: No error, empty result
	if err != nil {
		t.Fatalf("Expected no error for zero binaries, got: %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	// EC-001: Should return empty result (caller decides to build)
	if result.ShouldBuild {
		t.Error("Expected ShouldBuild=false for zero binaries")
	}
	if result.SelectedBinary != nil {
		t.Error("Expected SelectedBinary=nil for zero binaries")
	}
	if result.BinaryPath != "" {
		t.Error("Expected empty BinaryPath for zero binaries")
	}
	if result.WasCancelled {
		t.Error("Expected WasCancelled=false for zero binaries")
	}

	// Verify: Prompter NOT called (early return)
	if prompter.selectCalled {
		t.Error("Expected prompter not to be called for zero binaries")
	}
}

// TestBinarySelector_RunBinarySelectionFlow_EC002_SingleBinary_AutoSelect tests EC-002: Single binary auto-selection
func TestBinarySelector_RunBinarySelectionFlow_EC002_SingleBinary_AutoSelect(t *testing.T) {
	// Setup
	prompter := &mockPrompter{}
	selector := NewBinarySelector(prompter)

	binaries := createTestBinaries(1)

	opts := BinarySelectionOptions{
		AllowBuildFromSource: true,
		AutoSelectSingle:     true, // Auto-select enabled (CLARIFICATION 1: Option A)
		IsInteractive:        true,
	}

	// Execute: Single binary with auto-select
	result, err := selector.RunBinarySelectionFlow(context.Background(), binaries, opts)

	// Verify: No error
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify: Binary auto-selected
	if result.SelectedBinary == nil {
		t.Fatal("Expected SelectedBinary to be non-nil")
	}
	if result.BinaryPath != binaries[0].Path {
		t.Errorf("Expected BinaryPath=%s, got: %s", binaries[0].Path, result.BinaryPath)
	}
	if result.ShouldBuild {
		t.Error("Expected ShouldBuild=false")
	}
	if result.WasCancelled {
		t.Error("Expected WasCancelled=false")
	}

	// Verify: Prompter NOT called (auto-selected)
	if prompter.selectCalled {
		t.Error("Expected prompter not to be called for auto-select single binary")
	}
}

// TestBinarySelector_RunBinarySelectionFlow_EC002_SingleBinary_NoAutoSelect tests single binary without auto-select
func TestBinarySelector_RunBinarySelectionFlow_EC002_SingleBinary_NoAutoSelect(t *testing.T) {
	// Setup: Mock user selecting the single binary (index 0)
	prompter := &mockPrompter{
		selectIndex: 0,
	}
	selector := NewBinarySelector(prompter)

	binaries := createTestBinaries(1)

	opts := BinarySelectionOptions{
		AllowBuildFromSource: false, // No build option
		AutoSelectSingle:     false, // No auto-select
		IsInteractive:        true,
	}

	// Execute: Single binary, no auto-select
	result, err := selector.RunBinarySelectionFlow(context.Background(), binaries, opts)

	// Verify: No error
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify: User prompted and selected binary
	if !prompter.selectCalled {
		t.Fatal("Expected prompter to be called")
	}
	if result.SelectedBinary == nil {
		t.Fatal("Expected SelectedBinary to be non-nil")
	}
	if result.BinaryPath != binaries[0].Path {
		t.Errorf("Expected BinaryPath=%s, got: %s", binaries[0].Path, result.BinaryPath)
	}
}

// TestBinarySelector_RunBinarySelectionFlow_EC004_NonTTY tests EC-004: Non-interactive environment
func TestBinarySelector_RunBinarySelectionFlow_EC004_NonTTY(t *testing.T) {
	// Setup
	prompter := &mockPrompter{}
	selector := NewBinarySelector(prompter)

	binaries := createTestBinaries(3)

	opts := BinarySelectionOptions{
		AllowBuildFromSource: true,
		AutoSelectSingle:     true,
		IsInteractive:        false, // Non-TTY environment (CI/CD)
	}

	// Execute: Non-interactive mode
	result, err := selector.RunBinarySelectionFlow(context.Background(), binaries, opts)

	// Verify: No error
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// EC-004: Should auto-select first binary (most recent)
	if result.SelectedBinary == nil {
		t.Fatal("Expected SelectedBinary to be non-nil")
	}
	if result.BinaryPath != binaries[0].Path {
		t.Errorf("Expected BinaryPath=%s, got: %s", binaries[0].Path, result.BinaryPath)
	}
	if result.ShouldBuild {
		t.Error("Expected ShouldBuild=false")
	}
	if result.WasCancelled {
		t.Error("Expected WasCancelled=false")
	}

	// Verify: Prompter NOT called (non-interactive)
	if prompter.selectCalled {
		t.Error("Expected prompter not to be called in non-interactive mode")
	}
}

// TestBinarySelector_RunBinarySelectionFlow_MultipleBinaries_SelectFirst tests normal selection flow
func TestBinarySelector_RunBinarySelectionFlow_MultipleBinaries_SelectFirst(t *testing.T) {
	// Setup: Mock user selecting first binary (index 0)
	prompter := &mockPrompter{
		selectIndex: 0,
	}
	selector := NewBinarySelector(prompter)

	binaries := createTestBinaries(3)

	opts := BinarySelectionOptions{
		AllowBuildFromSource: true,
		AutoSelectSingle:     true,
		IsInteractive:        true,
	}

	// Execute: User selects first binary
	result, err := selector.RunBinarySelectionFlow(context.Background(), binaries, opts)

	// Verify: No error
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify: Prompter called with correct items
	if !prompter.selectCalled {
		t.Fatal("Expected prompter to be called")
	}
	// Should have 3 binaries + "Build from source" option
	if len(prompter.selectItems) != 4 {
		t.Errorf("Expected 4 items (3 binaries + build), got: %d", len(prompter.selectItems))
	}

	// Verify: First binary selected
	if result.SelectedBinary == nil {
		t.Fatal("Expected SelectedBinary to be non-nil")
	}
	if result.BinaryPath != binaries[0].Path {
		t.Errorf("Expected BinaryPath=%s, got: %s", binaries[0].Path, result.BinaryPath)
	}
	if result.ShouldBuild {
		t.Error("Expected ShouldBuild=false")
	}
	if result.WasCancelled {
		t.Error("Expected WasCancelled=false")
	}
}

// TestBinarySelector_RunBinarySelectionFlow_MultipleBinaries_SelectLast tests selecting last binary
func TestBinarySelector_RunBinarySelectionFlow_MultipleBinaries_SelectLast(t *testing.T) {
	// Setup: Mock user selecting last binary (index 2)
	prompter := &mockPrompter{
		selectIndex: 2,
	}
	selector := NewBinarySelector(prompter)

	binaries := createTestBinaries(3)

	opts := BinarySelectionOptions{
		AllowBuildFromSource: false, // No build option
		AutoSelectSingle:     true,
		IsInteractive:        true,
	}

	// Execute: User selects last binary
	result, err := selector.RunBinarySelectionFlow(context.Background(), binaries, opts)

	// Verify: No error
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify: Last binary selected
	if result.SelectedBinary == nil {
		t.Fatal("Expected SelectedBinary to be non-nil")
	}
	if result.BinaryPath != binaries[2].Path {
		t.Errorf("Expected BinaryPath=%s, got: %s", binaries[2].Path, result.BinaryPath)
	}
}

// TestBinarySelector_RunBinarySelectionFlow_BuildFromSource tests FR-011: Build from source option
func TestBinarySelector_RunBinarySelectionFlow_BuildFromSource(t *testing.T) {
	// Setup: Mock user selecting "Build from source" (last item, index 3)
	prompter := &mockPrompter{
		selectIndex: 3, // 3 binaries (0,1,2) + build option (3)
		inputText:   "v2.0.0",
	}
	selector := NewBinarySelector(prompter)

	binaries := createTestBinaries(3)

	opts := BinarySelectionOptions{
		AllowBuildFromSource: true,
		AutoSelectSingle:     true,
		IsInteractive:        true,
	}

	// Execute: User selects "Build from source"
	result, err := selector.RunBinarySelectionFlow(context.Background(), binaries, opts)

	// Verify: No error
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify: Build requested
	if !result.ShouldBuild {
		t.Error("Expected ShouldBuild=true")
	}
	if result.BuildVersion != "v2.0.0" {
		t.Errorf("Expected BuildVersion=v2.0.0, got: %s", result.BuildVersion)
	}
	if result.SelectedBinary != nil {
		t.Error("Expected SelectedBinary=nil when building")
	}
	if result.BinaryPath != "" {
		t.Error("Expected empty BinaryPath when building")
	}
	if result.WasCancelled {
		t.Error("Expected WasCancelled=false")
	}

	// Verify: Both select and input prompts called
	if !prompter.selectCalled {
		t.Error("Expected selectCalled=true")
	}
	if !prompter.inputCalled {
		t.Error("Expected inputCalled=true")
	}
}

// TestBinarySelector_RunBinarySelectionFlow_EC005_UserCancelledSelection tests EC-005: User cancellation during selection
func TestBinarySelector_RunBinarySelectionFlow_EC005_UserCancelledSelection(t *testing.T) {
	// Setup: Mock user pressing Ctrl+C during selection
	prompter := &mockPrompter{
		selectErr: promptui.ErrInterrupt, // User pressed Ctrl+C
	}
	selector := NewBinarySelector(prompter)

	binaries := createTestBinaries(3)

	opts := BinarySelectionOptions{
		AllowBuildFromSource: true,
		AutoSelectSingle:     true,
		IsInteractive:        true,
	}

	// Execute: User cancels selection
	result, err := selector.RunBinarySelectionFlow(context.Background(), binaries, opts)

	// Verify: No error (cancellation is not an error)
	if err != nil {
		t.Fatalf("Expected no error for user cancellation, got: %v", err)
	}

	// Verify: WasCancelled flag set
	if !result.WasCancelled {
		t.Error("Expected WasCancelled=true")
	}
	if result.SelectedBinary != nil {
		t.Error("Expected SelectedBinary=nil when cancelled")
	}
	if result.ShouldBuild {
		t.Error("Expected ShouldBuild=false when cancelled")
	}
}

// TestBinarySelector_RunBinarySelectionFlow_EC005_UserCancelledVersionInput tests EC-005: User cancellation during version input
func TestBinarySelector_RunBinarySelectionFlow_EC005_UserCancelledVersionInput(t *testing.T) {
	// Setup: Mock user selecting build, then cancelling version input
	prompter := &mockPrompter{
		selectIndex: 3,                     // Select "Build from source"
		inputErr:    promptui.ErrInterrupt, // Cancel version input
	}
	selector := NewBinarySelector(prompter)

	binaries := createTestBinaries(3)

	opts := BinarySelectionOptions{
		AllowBuildFromSource: true,
		AutoSelectSingle:     true,
		IsInteractive:        true,
	}

	// Execute: User cancels version input
	result, err := selector.RunBinarySelectionFlow(context.Background(), binaries, opts)

	// Verify: No error (cancellation is not an error)
	if err != nil {
		t.Fatalf("Expected no error for user cancellation, got: %v", err)
	}

	// Verify: WasCancelled flag set
	if !result.WasCancelled {
		t.Error("Expected WasCancelled=true")
	}
	if result.ShouldBuild {
		t.Error("Expected ShouldBuild=false when cancelled")
	}

	// Verify: Both prompts were called
	if !prompter.selectCalled {
		t.Error("Expected selectCalled=true")
	}
	if !prompter.inputCalled {
		t.Error("Expected inputCalled=true")
	}
}

// TestBinarySelector_RunBinarySelectionFlow_EC008_LargeCache tests EC-008: Large cache (>50 binaries)
func TestBinarySelector_RunBinarySelectionFlow_EC008_LargeCache(t *testing.T) {
	// Setup: Mock user selecting a binary from large cache
	prompter := &mockPrompter{
		selectIndex: 25, // Select binary in middle of large list
	}
	selector := NewBinarySelector(prompter)

	// Create 60 binaries (exceeds EC-008 threshold of 50)
	binaries := createTestBinaries(60)

	opts := BinarySelectionOptions{
		AllowBuildFromSource: true,
		AutoSelectSingle:     true,
		IsInteractive:        true,
	}

	// Execute: Large cache
	result, err := selector.RunBinarySelectionFlow(context.Background(), binaries, opts)

	// Verify: No error
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify: All binaries displayed (promptui handles scrolling)
	if len(prompter.selectItems) != 61 { // 60 binaries + build option
		t.Errorf("Expected 61 items, got: %d", len(prompter.selectItems))
	}

	// Verify: Correct binary selected
	if result.SelectedBinary == nil {
		t.Fatal("Expected SelectedBinary to be non-nil")
	}
	if result.BinaryPath != binaries[25].Path {
		t.Errorf("Expected BinaryPath=%s, got: %s", binaries[25].Path, result.BinaryPath)
	}
}

// TestBinarySelector_RunBinarySelectionFlow_NoBuildOption tests behavior without build option
func TestBinarySelector_RunBinarySelectionFlow_NoBuildOption(t *testing.T) {
	// Setup: Mock user selecting first binary
	prompter := &mockPrompter{
		selectIndex: 0,
	}
	selector := NewBinarySelector(prompter)

	binaries := createTestBinaries(3)

	opts := BinarySelectionOptions{
		AllowBuildFromSource: false, // No build option
		AutoSelectSingle:     true,
		IsInteractive:        true,
	}

	// Execute: No build option
	result, err := selector.RunBinarySelectionFlow(context.Background(), binaries, opts)

	// Verify: No error
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify: Only 3 items (no build option)
	if len(prompter.selectItems) != 3 {
		t.Errorf("Expected 3 items (no build option), got: %d", len(prompter.selectItems))
	}

	// Verify: Binary selected
	if result.SelectedBinary == nil {
		t.Fatal("Expected SelectedBinary to be non-nil")
	}
	if result.ShouldBuild {
		t.Error("Expected ShouldBuild=false")
	}
}

// TestIsTerminalInteractive tests the TTY detection helper
func TestIsTerminalInteractive(t *testing.T) {
	// Note: This test is environment-dependent
	// In test environments (go test), stdout is usually NOT a TTY
	isInteractive := IsTerminalInteractive()

	// We can't assert a specific value since it depends on test execution context
	// Just verify the function doesn't panic
	t.Logf("IsTerminalInteractive() returned: %v", isInteractive)

	// In most CI/CD environments and when running `go test`, this should be false
	// But we don't assert to avoid flakiness
}

// TestBinarySelector_RunBinarySelectionFlow_PromptLabels tests that correct labels are passed to prompter
func TestBinarySelector_RunBinarySelectionFlow_PromptLabels(t *testing.T) {
	// Setup: Mock selecting build option and providing version
	prompter := &mockPrompter{
		selectIndex: 2, // Select build option (2 binaries + build = index 2)
		inputText:   "main",
	}
	selector := NewBinarySelector(prompter)

	binaries := createTestBinaries(2)

	opts := BinarySelectionOptions{
		AllowBuildFromSource: true,
		AutoSelectSingle:     false,
		IsInteractive:        true,
	}

	// Execute
	_, err := selector.RunBinarySelectionFlow(context.Background(), binaries, opts)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify: Correct prompt labels
	expectedSelectLabel := "Select binary for deployment:"
	if prompter.selectLabel != expectedSelectLabel {
		t.Errorf("Expected select label '%s', got: '%s'", expectedSelectLabel, prompter.selectLabel)
	}

	expectedInputLabel := "Enter version to build (tag/branch/commit):"
	if prompter.inputLabel != expectedInputLabel {
		t.Errorf("Expected input label '%s', got: '%s'", expectedInputLabel, prompter.inputLabel)
	}
}
