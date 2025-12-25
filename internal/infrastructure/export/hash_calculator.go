// Package export provides infrastructure implementations for state export operations.
package export

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
)

// HashCalculator calculates SHA256 hashes of binary files.
type HashCalculator struct{}

// NewHashCalculator creates a new HashCalculator instance.
func NewHashCalculator() *HashCalculator {
	return &HashCalculator{}
}

// CalculateHash computes the SHA256 hash of a binary file.
// Returns the full 64-character hex string.
func (h *HashCalculator) CalculateHash(binaryPath string) (string, error) {
	if binaryPath == "" {
		return "", fmt.Errorf("binary path cannot be empty")
	}

	file, err := os.Open(binaryPath)
	if err != nil {
		return "", fmt.Errorf("failed to open binary: %w", err)
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", fmt.Errorf("failed to read binary: %w", err)
	}

	hashBytes := hasher.Sum(nil)
	return fmt.Sprintf("%x", hashBytes), nil
}

// CalculateHashPrefix computes the first 8 characters of the SHA256 hash.
func (h *HashCalculator) CalculateHashPrefix(binaryPath string) (string, error) {
	fullHash, err := h.CalculateHash(binaryPath)
	if err != nil {
		return "", err
	}

	if len(fullHash) < 8 {
		return "", fmt.Errorf("hash too short: %s", fullHash)
	}

	return fullHash[0:8], nil
}
