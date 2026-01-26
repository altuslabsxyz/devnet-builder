// internal/daemon/builder/git.go
package builder

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// CloneOptions specifies options for git clone
type CloneOptions struct {
	Repo    string // repository URL
	DestDir string // destination directory
	Depth   int    // shallow clone depth (0 = full clone)
}

// CheckoutOptions specifies options for git checkout
type CheckoutOptions struct {
	RepoDir string // repository directory
	Ref     string // branch, tag, or commit hash
}

// GitOperations handles git operations
type GitOperations struct{}

// Clone clones a git repository
func (g *GitOperations) Clone(ctx context.Context, opts CloneOptions) error {
	args := []string{"clone"}

	if opts.Depth > 0 {
		args = append(args, "--depth", fmt.Sprintf("%d", opts.Depth))
	}

	args = append(args, opts.Repo, opts.DestDir)

	cmd := exec.CommandContext(ctx, "git", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone failed: %w: %s", err, stderr.String())
	}

	return nil
}

// Fetch fetches updates from remote
func (g *GitOperations) Fetch(ctx context.Context, repoDir string, ref string) error {
	cmd := exec.CommandContext(ctx, "git", "fetch", "origin", ref)
	cmd.Dir = repoDir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git fetch failed: %w: %s", err, stderr.String())
	}

	return nil
}

// Checkout checks out a specific ref and returns the resolved commit hash
func (g *GitOperations) Checkout(ctx context.Context, opts CheckoutOptions) (string, error) {
	// Try to fetch the ref first (in case it's a remote branch/tag)
	_ = g.Fetch(ctx, opts.RepoDir, opts.Ref)

	// Checkout the ref
	cmd := exec.CommandContext(ctx, "git", "checkout", opts.Ref)
	cmd.Dir = opts.RepoDir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// Try with FETCH_HEAD for remote refs
		cmd = exec.CommandContext(ctx, "git", "checkout", "FETCH_HEAD")
		cmd.Dir = opts.RepoDir
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("git checkout failed: %w: %s", err, stderr.String())
		}
	}

	// Get the current commit hash
	return g.ResolveRef(ctx, opts.RepoDir, "HEAD")
}

// ResolveRef resolves a ref to a commit hash
func (g *GitOperations) ResolveRef(ctx context.Context, repoDir string, ref string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", ref)
	cmd.Dir = repoDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git rev-parse failed: %w: %s", err, stderr.String())
	}

	return strings.TrimSpace(stdout.String()), nil
}

// GetRemoteDefaultBranch gets the default branch name from remote
func (g *GitOperations) GetRemoteDefaultBranch(ctx context.Context, repoDir string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "symbolic-ref", "refs/remotes/origin/HEAD")
	cmd.Dir = repoDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// Fallback to main/master
		return "main", nil
	}

	// Output is like "refs/remotes/origin/main"
	ref := strings.TrimSpace(stdout.String())
	parts := strings.Split(ref, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1], nil
	}

	return "main", nil
}
