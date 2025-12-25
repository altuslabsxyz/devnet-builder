package export

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewHashCalculator(t *testing.T) {
	calc := NewHashCalculator()

	if calc == nil {
		t.Fatal("expected non-nil HashCalculator")
	}
}

func TestHashCalculator_CalculateHash_EmptyPath(t *testing.T) {
	calc := NewHashCalculator()

	_, err := calc.CalculateHash("")

	if err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestHashCalculator_CalculateHash_NonexistentFile(t *testing.T) {
	calc := NewHashCalculator()

	_, err := calc.CalculateHash("/nonexistent/file/path")

	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestHashCalculator_CalculateHash_ValidFile(t *testing.T) {
	calc := NewHashCalculator()

	// Create a temporary file with known content
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test-binary")

	testContent := []byte("test content for hashing")
	if err := os.WriteFile(testFile, testContent, 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	hash, err := calc.CalculateHash(testFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify hash is 64 hex characters
	if len(hash) != 64 {
		t.Errorf("expected hash length 64, got %d", len(hash))
	}

	// Verify hash is deterministic (same content = same hash)
	hash2, err := calc.CalculateHash(testFile)
	if err != nil {
		t.Fatalf("unexpected error on second call: %v", err)
	}

	if hash != hash2 {
		t.Errorf("expected deterministic hash, got different values: %s vs %s", hash, hash2)
	}

	// Verify hash is lowercase hex
	for _, c := range hash {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("hash contains invalid character: %c", c)
		}
	}
}

func TestHashCalculator_CalculateHashPrefix_EmptyPath(t *testing.T) {
	calc := NewHashCalculator()

	_, err := calc.CalculateHashPrefix("")

	if err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestHashCalculator_CalculateHashPrefix_ValidFile(t *testing.T) {
	calc := NewHashCalculator()

	// Create a temporary file with known content
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test-binary")

	testContent := []byte("test content for hashing")
	if err := os.WriteFile(testFile, testContent, 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	prefix, err := calc.CalculateHashPrefix(testFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify prefix is exactly 8 characters
	if len(prefix) != 8 {
		t.Errorf("expected prefix length 8, got %d", len(prefix))
	}

	// Verify prefix matches first 8 chars of full hash
	fullHash, _ := calc.CalculateHash(testFile)
	expectedPrefix := fullHash[0:8]

	if prefix != expectedPrefix {
		t.Errorf("expected prefix %s, got %s", expectedPrefix, prefix)
	}

	// Verify prefix is lowercase hex
	for _, c := range prefix {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("prefix contains invalid character: %c", c)
		}
	}
}

func TestHashCalculator_CalculateHashPrefix_NonexistentFile(t *testing.T) {
	calc := NewHashCalculator()

	_, err := calc.CalculateHashPrefix("/nonexistent/file/path")

	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestHashCalculator_CalculateHash_EmptyFile(t *testing.T) {
	calc := NewHashCalculator()

	// Create an empty file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "empty-file")

	if err := os.WriteFile(testFile, []byte{}, 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	hash, err := calc.CalculateHash(testFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// SHA256 hash of empty file
	expectedHash := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	if hash != expectedHash {
		t.Errorf("expected hash %s for empty file, got %s", expectedHash, hash)
	}
}
