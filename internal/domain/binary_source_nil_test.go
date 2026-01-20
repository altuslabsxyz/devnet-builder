package domain

import (
	"fmt"
	"testing"
)

func TestBinarySourceNilSafety(t *testing.T) {
	var nilSource *BinarySource

	// Test IsLocal on nil
	if nilSource.IsLocal() {
		t.Error("IsLocal() on nil should return false")
	}

	// Test IsGitHubRelease on nil
	if nilSource.IsGitHubRelease() {
		t.Error("IsGitHubRelease() on nil should return false")
	}

	// Test IsValid on nil
	if nilSource.IsValid() {
		t.Error("IsValid() on nil should return false")
	}

	// Test MarkValid on nil (should not panic)
	nilSource.MarkValid()

	// Test MarkInvalid on nil (should not panic)
	nilSource.MarkInvalid(fmt.Errorf("test error"))

	// Test Validate on nil
	err := nilSource.Validate()
	if err == nil {
		t.Error("Validate() on nil should return an error")
	}
}
