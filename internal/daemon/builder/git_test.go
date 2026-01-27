// internal/daemon/builder/git_test.go
package builder

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestGitClone(t *testing.T) {
	// Use a small, fast-cloning repo for tests
	tempDir := t.TempDir()

	g := &GitOperations{}

	err := g.Clone(context.Background(), CloneOptions{
		Repo:    "https://github.com/golang/example",
		DestDir: tempDir,
	})
	if err != nil {
		t.Fatalf("Clone failed: %v", err)
	}

	// Verify .git directory exists
	if _, err := os.Stat(filepath.Join(tempDir, ".git")); os.IsNotExist(err) {
		t.Error(".git directory not created")
	}
}

func TestGitCheckout(t *testing.T) {
	tempDir := t.TempDir()

	g := &GitOperations{}

	// Clone first
	err := g.Clone(context.Background(), CloneOptions{
		Repo:    "https://github.com/golang/example",
		DestDir: tempDir,
	})
	if err != nil {
		t.Fatalf("Clone failed: %v", err)
	}

	// Checkout a specific commit (first commit of golang/example)
	commit, err := g.Checkout(context.Background(), CheckoutOptions{
		RepoDir: tempDir,
		Ref:     "master", // use branch for speed
	})
	if err != nil {
		t.Fatalf("Checkout failed: %v", err)
	}

	if commit == "" {
		t.Error("Expected commit hash, got empty string")
	}

	t.Logf("Checked out commit: %s", commit)
}

func TestResolveCommit(t *testing.T) {
	tempDir := t.TempDir()

	g := &GitOperations{}

	err := g.Clone(context.Background(), CloneOptions{
		Repo:    "https://github.com/golang/example",
		DestDir: tempDir,
	})
	if err != nil {
		t.Fatalf("Clone failed: %v", err)
	}

	// Resolve branch to commit
	commit, err := g.ResolveRef(context.Background(), tempDir, "master")
	if err != nil {
		t.Fatalf("ResolveRef failed: %v", err)
	}

	// Should be a 40-char hex string
	if len(commit) != 40 {
		t.Errorf("Expected 40-char commit hash, got %d chars: %s", len(commit), commit)
	}
}
