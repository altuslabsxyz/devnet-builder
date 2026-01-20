package unit

import (
	"fmt"
	"testing"

	"github.com/altuslabsxyz/devnet-builder/internal/domain"
)

// TestBinarySourceCreation tests the NewBinarySource constructor.
func TestBinarySourceCreation(t *testing.T) {
	tests := []struct {
		name       string
		sourceType domain.SourceType
		wantType   domain.SourceType
		wantValid  bool
	}{
		{
			name:       "Create local binary source",
			sourceType: domain.SourceTypeLocal,
			wantType:   domain.SourceTypeLocal,
			wantValid:  false, // Not valid until path is set and validated
		},
		{
			name:       "Create GitHub release source",
			sourceType: domain.SourceTypeGitHubRelease,
			wantType:   domain.SourceTypeGitHubRelease,
			wantValid:  false, // Not valid until download completes
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := domain.NewBinarySource(tt.sourceType)

			if source.SourceType != tt.wantType {
				t.Errorf("NewBinarySource().SourceType = %v, want %v", source.SourceType, tt.wantType)
			}

			if source.IsValid() != tt.wantValid {
				t.Errorf("NewBinarySource().IsValid() = %v, want %v", source.IsValid(), tt.wantValid)
			}

			if source.SelectedPath != "" {
				t.Errorf("NewBinarySource().SelectedPath = %q, want empty string", source.SelectedPath)
			}
		})
	}
}

// TestBinarySourceValidation tests the Validate method with various scenarios.
func TestBinarySourceValidation(t *testing.T) {
	tests := []struct {
		name        string
		source      *domain.BinarySource
		wantErr     bool
		errContains string
	}{
		{
			name: "Valid local binary with absolute path",
			source: &domain.BinarySource{
				SourceType:       domain.SourceTypeLocal,
				SelectedPath:     "/usr/local/bin/stabled",
				ValidationStatus: true,
			},
			wantErr: false,
		},
		{
			name: "Valid local binary with Windows absolute path",
			source: &domain.BinarySource{
				SourceType:       domain.SourceTypeLocal,
				SelectedPath:     "C:\\Program Files\\stabled.exe",
				ValidationStatus: true,
			},
			wantErr: false,
		},
		{
			name: "Invalid local binary - no path",
			source: &domain.BinarySource{
				SourceType:   domain.SourceTypeLocal,
				SelectedPath: "",
			},
			wantErr:     true,
			errContains: "requires a selected path",
		},
		{
			name: "Invalid local binary - relative path",
			source: &domain.BinarySource{
				SourceType:   domain.SourceTypeLocal,
				SelectedPath: "bin/stabled",
			},
			wantErr:     true,
			errContains: "must be absolute",
		},
		{
			name: "Valid GitHub release source",
			source: &domain.BinarySource{
				SourceType:       domain.SourceTypeGitHubRelease,
				SelectedPath:     "", // Empty is OK for GitHub releases
				ValidationStatus: false,
			},
			wantErr: false,
		},
		{
			name: "Invalid state - error but status valid",
			source: &domain.BinarySource{
				SourceType:       domain.SourceTypeLocal,
				SelectedPath:     "/usr/local/bin/stabled",
				ValidationStatus: true,
				ValidationError:  "some error", // Inconsistent: error set but status is valid
			},
			wantErr:     true,
			errContains: "inconsistent state",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.source.Validate()

			if tt.wantErr {
				if err == nil {
					t.Errorf("Validate() error = nil, want error containing %q", tt.errContains)
					return
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("Validate() error = %q, want error containing %q", err.Error(), tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("Validate() error = %v, want nil", err)
				}
			}
		})
	}
}

// TestBinarySourceHelperMethods tests IsLocal, IsGitHubRelease, IsValid methods.
func TestBinarySourceHelperMethods(t *testing.T) {
	t.Run("IsLocal returns true for local source", func(t *testing.T) {
		source := domain.NewBinarySource(domain.SourceTypeLocal)
		if !source.IsLocal() {
			t.Error("IsLocal() = false, want true for SourceTypeLocal")
		}
		if source.IsGitHubRelease() {
			t.Error("IsGitHubRelease() = true, want false for SourceTypeLocal")
		}
	})

	t.Run("IsGitHubRelease returns true for GitHub source", func(t *testing.T) {
		source := domain.NewBinarySource(domain.SourceTypeGitHubRelease)
		if source.IsLocal() {
			t.Error("IsLocal() = true, want false for SourceTypeGitHubRelease")
		}
		if !source.IsGitHubRelease() {
			t.Error("IsGitHubRelease() = false, want true for SourceTypeGitHubRelease")
		}
	})

	t.Run("IsValid returns true only when validated", func(t *testing.T) {
		source := domain.NewBinarySource(domain.SourceTypeLocal)
		source.SelectedPath = "/usr/local/bin/stabled"

		// Initially not valid
		if source.IsValid() {
			t.Error("IsValid() = true, want false before validation")
		}

		// After marking valid
		source.MarkValid()
		if !source.IsValid() {
			t.Error("IsValid() = false, want true after MarkValid()")
		}
		if source.ValidationError != "" {
			t.Errorf("ValidationError = %q, want empty after MarkValid()", source.ValidationError)
		}
	})
}

// TestBinarySourceMarkInvalid tests the MarkInvalid method.
func TestBinarySourceMarkInvalid(t *testing.T) {
	source := domain.NewBinarySource(domain.SourceTypeLocal)
	source.SelectedPath = "/usr/local/bin/stabled"
	source.MarkValid() // Start as valid

	// Mark invalid with error
	testErr := "file not found"
	source.MarkInvalid(fmt.Errorf("%s", testErr))

	if source.IsValid() {
		t.Error("IsValid() = true, want false after MarkInvalid()")
	}

	if !contains(source.ValidationError, testErr) {
		t.Errorf("ValidationError = %q, want error containing %q", source.ValidationError, testErr)
	}

	if source.ValidationStatus {
		t.Error("ValidationStatus = true, want false after MarkInvalid()")
	}
}

// TestSourceTypeString tests the String() method for SourceType enum.
func TestSourceTypeString(t *testing.T) {
	tests := []struct {
		sourceType domain.SourceType
		want       string
	}{
		{domain.SourceTypeLocal, "local"},
		{domain.SourceTypeGitHubRelease, "github-release"},
		{domain.SourceType(999), "unknown"}, // Invalid value
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := tt.sourceType.String()
			if got != tt.want {
				t.Errorf("SourceType.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestBinarySourceWorkflow tests the full workflow of binary source selection.
func TestBinarySourceWorkflow(t *testing.T) {
	t.Run("Local binary workflow", func(t *testing.T) {
		// Step 1: User selects local binary source
		source := domain.NewBinarySource(domain.SourceTypeLocal)

		// Step 2: User browses and selects path
		source.SelectedPath = "/usr/local/bin/stabled"

		// Step 3: Validate business rules
		if err := source.Validate(); err != nil {
			t.Fatalf("Validate() failed: %v", err)
		}

		// Step 4: Infrastructure validates binary file (success)
		source.MarkValid()

		// Step 5: Binary source is ready to use
		if !source.IsValid() {
			t.Error("Expected source to be valid after successful validation")
		}
	})

	t.Run("Local binary workflow with validation failure", func(t *testing.T) {
		// Step 1-2: User selects local binary and path
		source := domain.NewBinarySource(domain.SourceTypeLocal)
		source.SelectedPath = "/usr/local/bin/invalid"

		// Step 3: Validate business rules (passes)
		if err := source.Validate(); err != nil {
			t.Fatalf("Validate() failed: %v", err)
		}

		// Step 4: Infrastructure validates binary file (fails)
		source.MarkInvalid(fmt.Errorf("binary is not executable"))

		// Step 5: User retries with different path
		if source.IsValid() {
			t.Error("Expected source to be invalid after failed validation")
		}

		// Step 6: User selects valid path
		source.SelectedPath = "/usr/local/bin/stabled"
		source.MarkValid()

		// Step 7: Binary source is ready to use
		if !source.IsValid() {
			t.Error("Expected source to be valid after retry")
		}
	})

	t.Run("GitHub release workflow", func(t *testing.T) {
		// Step 1: User selects GitHub release source
		source := domain.NewBinarySource(domain.SourceTypeGitHubRelease)

		// Step 2: Validate business rules (no path needed)
		if err := source.Validate(); err != nil {
			t.Fatalf("Validate() failed for GitHub source: %v", err)
		}

		// Step 3: System will handle download (not tested here)
		// SelectedPath will be set after download completes
	})
}

// contains is a helper function to check if a string contains a substring.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || indexAny(s, substr) >= 0)
}

// indexAny is a simple helper to find substring.
func indexAny(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
