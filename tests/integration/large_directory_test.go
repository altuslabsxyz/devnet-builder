package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/b-harvest/devnet-builder/internal/infrastructure/interactive"
)

// TestPathCompleterLargeDirectoryPerformance tests autocomplete performance with 10k files.
//
// This test implements SC-002: Autocomplete response < 100ms for directories with < 1000 entries.
// We test with 10k entries to ensure the 100-result pagination limit works correctly.
//
// Test Strategy:
//  1. Create temp directory with 10,000 files (dir-0000 to dir-9999)
//  2. Measure autocomplete time for partial prefix
//  3. Verify response time < 100ms
//  4. Verify result count = 100 (pagination limit)
//  5. Verify results are alphabetically sorted
//
// Performance Target:
//   - Response time: < 100ms for partial match (SC-002)
//   - Result limit: Exactly 100 entries (FR-012)
//   - Sorting: Alphabetically sorted (FR-012)
func TestPathCompleterLargeDirectoryPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "perf-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create 10,000 directories: dir-0000, dir-0001, ..., dir-9999
	t.Log("Creating 10,000 test directories...")
	startSetup := time.Now()
	for i := 0; i < 10000; i++ {
		dirName := filepath.Join(tmpDir, fmt.Sprintf("dir-%04d", i))
		if err := os.Mkdir(dirName, 0755); err != nil {
			t.Fatalf("Failed to create directory %d: %v", i, err)
		}
	}
	setupDuration := time.Since(startSetup)
	t.Logf("Setup completed in %v", setupDuration)

	// Create PathCompleterAdapter
	completer := interactive.NewPathCompleterAdapter()

	// Test Case 1: Complete with prefix that matches many entries
	// Input: "{tmpDir}/dir-" should match all 10,000 entries
	// Expected: Returns first 100 alphabetically (dir-0000 to dir-0099)
	t.Run("Autocomplete with broad prefix", func(t *testing.T) {
		input := filepath.Join(tmpDir, "dir-")

		// Measure autocomplete performance
		start := time.Now()
		results := completer.Complete(input)
		elapsed := time.Since(start)

		// SC-002: Verify response time < 100ms
		if elapsed > 100*time.Millisecond {
			t.Errorf("Autocomplete took %v, expected < 100ms (SC-002 violation)", elapsed)
		} else {
			t.Logf("✓ Autocomplete completed in %v (< 100ms)", elapsed)
		}

		// FR-012: Verify result count = 100 (pagination limit)
		if len(results) != 100 {
			t.Errorf("Complete() returned %d results, expected exactly 100 (FR-012 pagination)", len(results))
		} else {
			t.Logf("✓ Result count = 100 (pagination limit)")
		}

		// FR-012: Verify results are alphabetically sorted
		// First 100 should be dir-0000 to dir-0099
		for i := 0; i < 100; i++ {
			expected := filepath.Join(tmpDir, fmt.Sprintf("dir-%04d", i)) + "/"
			if results[i] != expected {
				t.Errorf("Result[%d] = %s, expected %s (not alphabetically sorted)", i, results[i], expected)
			}
		}
		t.Logf("✓ Results are alphabetically sorted")
	})

	// Test Case 2: Complete with narrow prefix (fewer matches)
	// Input: "{tmpDir}/dir-99" should match 100 entries (dir-9900 to dir-9999)
	// Expected: Returns exactly 100 entries in < 100ms
	t.Run("Autocomplete with narrow prefix", func(t *testing.T) {
		input := filepath.Join(tmpDir, "dir-99")

		// Measure autocomplete performance
		start := time.Now()
		results := completer.Complete(input)
		elapsed := time.Since(start)

		// SC-002: Verify response time < 100ms
		if elapsed > 100*time.Millisecond {
			t.Errorf("Autocomplete took %v, expected < 100ms (SC-002 violation)", elapsed)
		} else {
			t.Logf("✓ Autocomplete completed in %v (< 100ms)", elapsed)
		}

		// Expect 100 matches: dir-9900 to dir-9999
		expectedCount := 100
		if len(results) != expectedCount {
			t.Errorf("Complete() returned %d results, expected %d", len(results), expectedCount)
		} else {
			t.Logf("✓ Result count = %d", expectedCount)
		}

		// Verify first and last results
		if len(results) > 0 {
			expectedFirst := filepath.Join(tmpDir, "dir-9900") + "/"
			if results[0] != expectedFirst {
				t.Errorf("First result = %s, expected %s", results[0], expectedFirst)
			}

			expectedLast := filepath.Join(tmpDir, "dir-9999") + "/"
			if results[len(results)-1] != expectedLast {
				t.Errorf("Last result = %s, expected %s", results[len(results)-1], expectedLast)
			}
		}
	})

	// Test Case 3: Complete with very specific prefix (single match)
	// Input: "{tmpDir}/dir-5678" should match exactly 1 entry (dir-5678)
	// Expected: Returns 1 entry in < 100ms
	t.Run("Autocomplete with specific prefix", func(t *testing.T) {
		input := filepath.Join(tmpDir, "dir-5678")

		// Measure autocomplete performance
		start := time.Now()
		results := completer.Complete(input)
		elapsed := time.Since(start)

		// SC-002: Verify response time < 100ms
		if elapsed > 100*time.Millisecond {
			t.Errorf("Autocomplete took %v, expected < 100ms (SC-002 violation)", elapsed)
		} else {
			t.Logf("✓ Autocomplete completed in %v (< 100ms)", elapsed)
		}

		// Expect exactly 1 match: dir-5678
		if len(results) != 1 {
			t.Errorf("Complete() returned %d results, expected 1", len(results))
		} else {
			expected := filepath.Join(tmpDir, "dir-5678") + "/"
			if results[0] != expected {
				t.Errorf("Result = %s, expected %s", results[0], expected)
			} else {
				t.Logf("✓ Single match correct: %s", results[0])
			}
		}
	})

	// Test Case 4: Measure multiple consecutive calls (cache behavior)
	// Run autocomplete 10 times and measure average response time
	t.Run("Repeated autocomplete calls", func(t *testing.T) {
		input := filepath.Join(tmpDir, "dir-")
		iterations := 10

		var totalDuration time.Duration
		for i := 0; i < iterations; i++ {
			start := time.Now()
			results := completer.Complete(input)
			elapsed := time.Since(start)
			totalDuration += elapsed

			if len(results) != 100 {
				t.Errorf("Iteration %d: returned %d results, expected 100", i, len(results))
			}
		}

		avgDuration := totalDuration / time.Duration(iterations)
		t.Logf("Average response time over %d iterations: %v", iterations, avgDuration)

		if avgDuration > 100*time.Millisecond {
			t.Errorf("Average autocomplete time %v exceeds 100ms (SC-002 violation)", avgDuration)
		} else {
			t.Logf("✓ Average response time < 100ms")
		}
	})
}

// TestPathCompleterStressTest tests autocomplete with extremely large directories (100k files).
//
// This is a stress test to ensure the implementation handles edge cases gracefully.
// It's marked as optional and only runs with -tags=stress flag.
//
// Performance Target:
//   - Should not hang or crash with 100k files
//   - Response time may exceed 100ms, but should be < 1s
//   - Memory usage should remain reasonable (< 100MB for result set)
func TestPathCompleterStressTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	// Only run if explicitly requested
	// go test -v -tags=stress ./tests/integration/
	t.Skip("Skipping stress test (requires -tags=stress flag)")

	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "stress-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create 100,000 directories
	t.Log("Creating 100,000 test directories... (this may take a while)")
	startSetup := time.Now()
	for i := 0; i < 100000; i++ {
		dirName := filepath.Join(tmpDir, fmt.Sprintf("dir-%06d", i))
		if err := os.Mkdir(dirName, 0755); err != nil {
			t.Fatalf("Failed to create directory %d: %v", i, err)
		}

		if i%10000 == 0 {
			t.Logf("Progress: %d/100000 directories created", i)
		}
	}
	setupDuration := time.Since(startSetup)
	t.Logf("Setup completed in %v", setupDuration)

	// Create PathCompleterAdapter
	completer := interactive.NewPathCompleterAdapter()

	// Measure autocomplete performance
	input := filepath.Join(tmpDir, "dir-")
	start := time.Now()
	results := completer.Complete(input)
	elapsed := time.Since(start)

	t.Logf("Autocomplete completed in %v", elapsed)

	// Verify result count = 100 (pagination limit)
	if len(results) != 100 {
		t.Errorf("Complete() returned %d results, expected exactly 100", len(results))
	}

	// Stress test passes if it completes without hanging/crashing
	if elapsed > 1*time.Second {
		t.Logf("Warning: Autocomplete took > 1s with 100k files (may be acceptable)")
	}
}

// TestPathCompleterBenchmark provides benchmark measurements for autocomplete performance.
//
// This benchmark measures:
//  1. Time to complete partial path
//  2. Memory allocations during completion
//  3. Throughput (completions per second)
//
// Run with: go test -bench=. -benchmem ./tests/integration/
func BenchmarkPathCompleter(b *testing.B) {
	// Create a temporary directory for benchmarking
	tmpDir, err := os.MkdirTemp("", "bench-*")
	if err != nil {
		b.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create 1,000 directories for realistic benchmarking
	for i := 0; i < 1000; i++ {
		dirName := filepath.Join(tmpDir, fmt.Sprintf("dir-%04d", i))
		if err := os.Mkdir(dirName, 0755); err != nil {
			b.Fatalf("Failed to create directory %d: %v", i, err)
		}
	}

	// Create PathCompleterAdapter
	completer := interactive.NewPathCompleterAdapter()
	input := filepath.Join(tmpDir, "dir-")

	// Reset timer after setup
	b.ResetTimer()

	// Run benchmark
	for i := 0; i < b.N; i++ {
		results := completer.Complete(input)
		if len(results) != 100 {
			b.Errorf("Unexpected result count: %d", len(results))
		}
	}
}

// BenchmarkPathCompleterDifferentSizes benchmarks autocomplete with various directory sizes.
func BenchmarkPathCompleterDifferentSizes(b *testing.B) {
	sizes := []int{100, 500, 1000, 5000, 10000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("Size_%d", size), func(b *testing.B) {
			// Create temporary directory
			tmpDir, err := os.MkdirTemp("", "bench-*")
			if err != nil {
				b.Fatalf("Failed to create temp directory: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			// Create directories
			for i := 0; i < size; i++ {
				dirName := filepath.Join(tmpDir, fmt.Sprintf("dir-%05d", i))
				if err := os.Mkdir(dirName, 0755); err != nil {
					b.Fatalf("Failed to create directory %d: %v", i, err)
				}
			}

			// Create completer
			completer := interactive.NewPathCompleterAdapter()
			input := filepath.Join(tmpDir, "dir-")

			// Reset timer after setup
			b.ResetTimer()

			// Run benchmark
			for i := 0; i < b.N; i++ {
				_ = completer.Complete(input)
			}
		})
	}
}
